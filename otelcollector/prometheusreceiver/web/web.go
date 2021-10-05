package web

import (
	"bytes"
	"context"
	"fmt"
	"io"
	//"io/ioutil"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	//"path"
	"regexp"
	"sort"
	"sync"
	html_template "html/template"
	template_text "text/template"
	"time"
	"os"
	"embed"
	"errors"
	"strings"
	"math"
	"strconv"
	"path"
	"runtime"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	conntrack "github.com/mwitkow/go-conntrack"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	opentracing "github.com/opentracing/opentracing-go"
	//"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/route"
	//"github.com/prometheus/common/server"
	//toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"go.uber.org/atomic"
	"golang.org/x/net/netutil"

	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/rules"
	"github.com/prometheus/prometheus/scrape"
	//"github.com/prometheus/prometheus/template"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	//"github.com/prometheus/prometheus/web/ui"
)

//go:embed templates
var targetHTML embed.FS

//go:embed static
var staticFiles embed.FS

// PrometheusVersion contains build information about Prometheus.
type PrometheusVersion = api_v1.PrometheusVersion

// Handler serves various HTTP endpoints of the Prometheus server
type Handler struct {
	logger log.Logger

	scrapeManager   *scrape.Manager
	context         context.Context

	router      *route.Router
	quitCh      chan struct{}
	quitOnce    sync.Once
	reloadCh    chan chan error
	options     *Options
	config      *config.Config
	versionInfo *PrometheusVersion
	birth       time.Time
	cwd         string
	flagsMap    map[string]string

	mtx sync.RWMutex
	now func() model.Time

	ready atomic.Uint32 // ready is uint32 rather than boolean to be able to use atomic functions.
}

// Options for the web Handler.
type Options struct {
	Context               context.Context
	LookbackDelta         time.Duration
	ScrapeManager         *scrape.Manager
	Version               *PrometheusVersion
	Flags                 map[string]string

	ListenAddress              string
	CORSOrigin                 *regexp.Regexp
	ReadTimeout                time.Duration
	MaxConnections             int
	ExternalURL                *url.URL
	RoutePrefix                string
	UseLocalAssets             bool
	UserAssetsPath             string
	EnableLifecycle            bool
	EnableAdminAPI             bool
	PageTitle                  string
}

// New initializes a new web Handler.
func New(logger log.Logger, o *Options) *Handler {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	router := route.New()

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "<error retrieving current working directory>"
	}
	level.Info(logger).Log("msg", "Getting cwd", "cwd", cwd)


	h := &Handler{
		logger: logger,
		router:      router,
		quitCh:      make(chan struct{}),
		reloadCh:    make(chan chan error),
		options:     o,
		versionInfo: o.Version,
		birth:       time.Now().UTC(),
		cwd:         cwd,
		flagsMap:    o.Flags,

		context:         o.Context,
		scrapeManager:   o.ScrapeManager,

		now: model.Now,
	}

	h.ready.Store(1)
	readyf := h.testReady
	
	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path.Join(o.ExternalURL.Path, "/config"), http.StatusFound)
	})
	router.Get("/test", testHandler)
	router.Get("/config", readyf(h.serveConfig))
	router.Get("/targets", readyf(h.targets))
	router.Get("/service-discovery", readyf(h.serviceDiscovery))

	router.Get("/static/*filepath", func(w http.ResponseWriter, r *http.Request) {
		level.Info(h.logger).Log("msg", "handling static routing", "url", r.URL.Path)
		var staticFS = http.FS(staticFiles)
		fs := http.FileServer(staticFS)
		fs.ServeHTTP(w, r)
	})

	return h
}

// Listener creates the TCP listener for web requests.
func (h *Handler) Listener() (net.Listener, error) {
	level.Info(h.logger).Log("msg", "Start listening for connections", "address", h.options.ListenAddress)
	level.Info(h.logger).Log("msg", "Using custom logging")

	listener, err := net.Listen("tcp", h.options.ListenAddress)
	if err != nil {
		return listener, err
	}
	listener = netutil.LimitListener(listener, h.options.MaxConnections)

	// Monitor incoming connections with conntrack.
	listener = conntrack.NewListener(listener,
		conntrack.TrackWithName("http"),
		conntrack.TrackWithTracing(),
		conntrack.TrackWithTcpKeepAlive(5 * time.Minute))

	return listener, nil
}


// Run serves the HTTP endpoints.
func (h *Handler) Run(ctx context.Context, listener net.Listener, webConfig string) error {
	level.Info(h.logger).Log("msg", "Running")
	if listener == nil {
		var err error
		listener, err = h.Listener()
		if err != nil {
			return err
		}
	}
	operationName := nethttp.OperationNameFunc(func(r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	mux := http.NewServeMux()
	mux.Handle("/", h.router)

	errlog := stdlog.New(log.NewStdlibAdapter(level.Error(h.logger)), "", 0)

	level.Info(h.logger).Log("msg", "About to create server")

	httpSrv := &http.Server{
		Handler:     withStackTracer(nethttp.Middleware(opentracing.GlobalTracer(), mux, operationName), h.logger)/*mux*/,
		ErrorLog:    errlog,
		ReadTimeout: h.options.ReadTimeout,
		Addr: h.options.ListenAddress,
	}

	level.Info(h.logger).Log("msg", "Making channel for server")

	errCh := make(chan error)
	go func() {
		level.Info(h.logger).Log("msg", "About to serve")
		errCh <- httpSrv.Serve(listener)
		level.Info(h.logger).Log("msg", "Serve returned")
	}()

	select {
	case e := <-errCh:
		level.Info(h.logger).Log("msg", "Returning error")
		return e
	case <-ctx.Done():
		level.Info(h.logger).Log("msg", "Shutting down server")
		httpSrv.Shutdown(ctx)
		return nil
	}
}

// ApplyConfig updates the config field of the Handler struct
func (h *Handler) ApplyConfig(conf *config.Config) error {
	h.mtx.Lock()
	defer h.mtx.Unlock()

	h.config = conf

	return nil
}

// Verifies whether the server is ready or not.
func (h *Handler) isReady() bool {
	return h.ready.Load() > 0
}

// Checks if server is ready, calls f if it is, returns 503 if it is not.
func (h *Handler) testReady(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.isReady() {
			f(w, r)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Service Unavailable")
		}
	}
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, world!")
}

func (h *Handler) serveConfig(w http.ResponseWriter, r *http.Request) {
	h.mtx.RLock()
	defer h.mtx.RUnlock()

	h.executeTemplate(w, "config.html", h.config.String())
}

func (h *Handler) serviceDiscovery(w http.ResponseWriter, r *http.Request) {
	level.Info(h.logger).Log("msg", "In service discovery handler")
	var index []string
	targets := h.scrapeManager.TargetsAll()
	for job := range targets {
		index = append(index, job)
	}
	sort.Strings(index)
	scrapeConfigData := struct {
		Index   []string
		Targets map[string][]*scrape.Target
		Active  []int
		Dropped []int
		Total   []int
	}{
		Index:   index,
		Targets: make(map[string][]*scrape.Target),
		Active:  make([]int, len(index)),
		Dropped: make([]int, len(index)),
		Total:   make([]int, len(index)),
	}
	for i, job := range scrapeConfigData.Index {
		scrapeConfigData.Targets[job] = make([]*scrape.Target, 0, len(targets[job]))
		scrapeConfigData.Total[i] = len(targets[job])
		for _, target := range targets[job] {
			// Do not display more than 100 dropped targets per job to avoid
			// returning too much data to the clients.
			if target.Labels().Len() == 0 {
				scrapeConfigData.Dropped[i]++
				if scrapeConfigData.Dropped[i] > 100 {
					continue
				}
			} else {
				scrapeConfigData.Active[i]++
			}
			scrapeConfigData.Targets[job] = append(scrapeConfigData.Targets[job], target)
		}
	}

	level.Info(h.logger).Log("msg", "About to execute template")

	h.executeTemplate(w, "service-discovery.html", scrapeConfigData)
}

func (h *Handler) targets(w http.ResponseWriter, r *http.Request) {
	level.Info(h.logger).Log("msg", "In target handler")

	tps := h.scrapeManager.TargetsActive()
	for _, targets := range tps {
		sort.Slice(targets, func(i, j int) bool {
			iJobLabel := targets[i].Labels().Get(model.JobLabel)
			jJobLabel := targets[j].Labels().Get(model.JobLabel)
			if iJobLabel == jJobLabel {
				return targets[i].Labels().Get(model.InstanceLabel) < targets[j].Labels().Get(model.InstanceLabel)
			}
			return iJobLabel < jJobLabel
		})
	}

	level.Info(h.logger).Log("msg", "About to execute template")

	h.executeTemplate(w, "targets.html", struct {
		TargetPools map[string][]*scrape.Target
	}{
		TargetPools: tps,
	})
}

// A version of vector that's easier to use from templates.
type sample struct {
	Labels map[string]string
	Value  float64
}
type queryResult []*sample

type queryResultByLabelSorter struct {
	results queryResult
	by      string
}

func (q queryResultByLabelSorter) Len() int {
	return len(q.results)
}

func (q queryResultByLabelSorter) Less(i, j int) bool {
	return q.results[i].Labels[q.by] < q.results[j].Labels[q.by]
}

func (q queryResultByLabelSorter) Swap(i, j int) {
	q.results[i], q.results[j] = q.results[j], q.results[i]
}

func convertToFloat(i interface{}) (float64, error) {
	switch v := i.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("can't convert %T to float", v)
	}
}

func tmplFuncs(consolesPath string, opts *Options) template_text.FuncMap {
	return template_text.FuncMap{
		"first": func(v queryResult) (*sample, error) {
			if len(v) > 0 {
				return v[0], nil
			}
			return nil, errors.New("first() called on vector with no elements")
		},
		"label": func(label string, s *sample) string {
			return s.Labels[label]
		},
		"value": func(s *sample) float64 {
			return s.Value
		},
		"strvalue": func(s *sample) string {
			return s.Labels["__value__"]
		},
		"args": func(args ...interface{}) map[string]interface{} {
			result := make(map[string]interface{})
			for i, a := range args {
				result[fmt.Sprintf("arg%d", i)] = a
			}
			return result
		},
		"reReplaceAll": func(pattern, repl, text string) string {
			re := regexp.MustCompile(pattern)
			return re.ReplaceAllString(text, repl)
		},
		"safeHtml": func(text string) html_template.HTML {
			return html_template.HTML(text)
		},
		"match":     regexp.MatchString,
		"title":     strings.Title,
		"toUpper":   strings.ToUpper,
		"toLower":   strings.ToLower,
		"sortByLabel": func(label string, v queryResult) queryResult {
			sorter := queryResultByLabelSorter{v[:], label}
			sort.Stable(sorter)
			return v
		},
		"humanize": func(i interface{}) (string, error) {
			v, err := convertToFloat(i)
			if err != nil {
				return "", err
			}
			if v == 0 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			if math.Abs(v) >= 1 {
				prefix := ""
				for _, p := range []string{"k", "M", "G", "T", "P", "E", "Z", "Y"} {
					if math.Abs(v) < 1000 {
						break
					}
					prefix = p
					v /= 1000
				}
				return fmt.Sprintf("%.4g%s", v, prefix), nil
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%s", v, prefix), nil
		},
		"humanize1024": func(i interface{}) (string, error) {
			v, err := convertToFloat(i)
			if err != nil {
				return "", err
			}
			if math.Abs(v) <= 1 || math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			prefix := ""
			for _, p := range []string{"ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"} {
				if math.Abs(v) < 1024 {
					break
				}
				prefix = p
				v /= 1024
			}
			return fmt.Sprintf("%.4g%s", v, prefix), nil
		},
		"humanizeDuration": func(i interface{}) (string, error) {
			v, err := convertToFloat(i)
			if err != nil {
				return "", err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			if v == 0 {
				return fmt.Sprintf("%.4gs", v), nil
			}
			if math.Abs(v) >= 1 {
				sign := ""
				if v < 0 {
					sign = "-"
					v = -v
				}
				seconds := int64(v) % 60
				minutes := (int64(v) / 60) % 60
				hours := (int64(v) / 60 / 60) % 24
				days := int64(v) / 60 / 60 / 24
				// For days to minutes, we display seconds as an integer.
				if days != 0 {
					return fmt.Sprintf("%s%dd %dh %dm %ds", sign, days, hours, minutes, seconds), nil
				}
				if hours != 0 {
					return fmt.Sprintf("%s%dh %dm %ds", sign, hours, minutes, seconds), nil
				}
				if minutes != 0 {
					return fmt.Sprintf("%s%dm %ds", sign, minutes, seconds), nil
				}
				// For seconds, we display 4 significant digits.
				return fmt.Sprintf("%s%.4gs", sign, v), nil
			}
			prefix := ""
			for _, p := range []string{"m", "u", "n", "p", "f", "a", "z", "y"} {
				if math.Abs(v) >= 1 {
					break
				}
				prefix = p
				v *= 1000
			}
			return fmt.Sprintf("%.4g%ss", v, prefix), nil
		},
		"humanizePercentage": func(i interface{}) (string, error) {
			v, err := convertToFloat(i)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%.4g%%", v*100), nil
		},
		"humanizeTimestamp": func(i interface{}) (string, error) {
			v, err := convertToFloat(i)
			if err != nil {
				return "", err
			}
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Sprintf("%.4g", v), nil
			}
			t := model.TimeFromUnixNano(int64(v * 1e9)).Time().UTC()
			return fmt.Sprint(t), nil
		},
		"since": func(t time.Time) time.Duration {
			return time.Since(t) / time.Millisecond * time.Millisecond
		},
		"unixToTime": func(i int64) time.Time {
			t := time.Unix(i/int64(time.Microsecond), 0).UTC()
			return t
		},
		"consolesPath": func() string { return consolesPath },
		"pathPrefix":   func() string { return opts.ExternalURL.Path },
		"pageTitle":    func() string { return opts.PageTitle },
		"buildVersion": func() string { return opts.Version.Revision },
		"globalURL": func(u *url.URL) *url.URL {
			host, port, err := net.SplitHostPort(u.Host)
			if err != nil {
				return u
			}
			for _, lhr := range api_v1.LocalhostRepresentations {
				if host == lhr {
					_, ownPort, err := net.SplitHostPort(opts.ListenAddress)
					if err != nil {
						return u
					}

					if port == ownPort {
						// Only in the case where the target is on localhost and its port is
						// the same as the one we're listening on, we know for sure that
						// we're monitoring our own process and that we need to change the
						// scheme, hostname, and port to the externally reachable ones as
						// well. We shouldn't need to touch the path at all, since if a
						// path prefix is defined, the path under which we scrape ourselves
						// should already contain the prefix.
						u.Scheme = opts.ExternalURL.Scheme
						u.Host = opts.ExternalURL.Host
					} else {
						// Otherwise, we only know that localhost is not reachable
						// externally, so we replace only the hostname by the one in the
						// external URL. It could be the wrong hostname for the service on
						// this port, but it's still the best possible guess.
						host, _, err := net.SplitHostPort(opts.ExternalURL.Host)
						if err != nil {
							return u
						}
						u.Host = host + ":" + port
					}
					break
				}
			}
			return u
		},
		"numHealthy": func(pool []*scrape.Target) int {
			alive := len(pool)
			for _, p := range pool {
				if p.Health() != scrape.HealthGood {
					alive--
				}
			}

			return alive
		},
		"targetHealthToClass": func(th scrape.TargetHealth) string {
			switch th {
			case scrape.HealthUnknown:
				return "warning"
			case scrape.HealthGood:
				return "success"
			default:
				return "danger"
			}
		},
		"ruleHealthToClass": func(rh rules.RuleHealth) string {
			switch rh {
			case rules.HealthUnknown:
				return "warning"
			case rules.HealthGood:
				return "success"
			default:
				return "danger"
			}
		},
		"alertStateToClass": func(as rules.AlertState) string {
			switch as {
			case rules.StateInactive:
				return "success"
			case rules.StatePending:
				return "warning"
			case rules.StateFiring:
				return "danger"
			default:
				panic("unknown alert state")
			}
		},
	}
}

func (h *Handler) executeTemplate(w http.ResponseWriter, name string, data interface{}) { 
	level.Info(h.logger).Log("msg", "executeTemplate", "data", data)

	template, _ := targetHTML.ReadFile("templates/" + name)
	level.Info(h.logger).Log("msg", "executeTemplate", "template", string(template))

	funcMap := tmplFuncs("", h.options)

	level.Info(h.logger).Log("msg", "Got funcMap")

	tmpl := html_template.New(name).Funcs(html_template.FuncMap(funcMap))

	level.Info(h.logger).Log("msg", "In executeTemplate", "name", tmpl.Name())

	level.Info(h.logger).Log("msg", "Made template")

	tmpl.Option("missingkey=zero")
	/*tmpl.Funcs(html_template.FuncMap{
		"tmpl": func(name string, data interface{}) (html_template.HTML, error) {
			var buffer bytes.Buffer
			err := tmpl.ExecuteTemplate(&buffer, name, data)
			return html_template.HTML(buffer.String()), err
		},
	})*/

	level.Info(h.logger).Log("msg", "About to parse template")

	tmpl, err := tmpl.Parse(string(template))
	if err != nil {
		level.Info(h.logger).Log("msg", "Error parsing template", "err", err)
	}

	level.Info(h.logger).Log("msg", "In executeTemplate", "name", tmpl.Name())

	level.Info(h.logger).Log("msg", "Parsed template")

	var buffer bytes.Buffer
	err = tmpl.ExecuteTemplate(&buffer, name, data)
	if err != nil {
		level.Info(h.logger).Log("msg", "Error executing template", "err", err)
	}

	level.Info(h.logger).Log("msg", "About to write back", "buffer", buffer.String())

	io.WriteString(w, buffer.String())
}

// withStackTrace logs the stack trace in case the request panics. The function
// will re-raise the error which will then be handled by the net/http package.
// It is needed because the go-kit log package doesn't manage properly the
// panics from net/http (see https://github.com/go-kit/kit/issues/233).
func withStackTracer(h http.Handler, l log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				level.Error(l).Log("msg", "panic while serving request", "client", r.RemoteAddr, "url", r.URL, "err", err, "stack", buf)
				panic(err)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
