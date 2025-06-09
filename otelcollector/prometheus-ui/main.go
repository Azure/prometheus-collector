package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/server"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Paths handled by the React router that should all serve the main React app's index.html,
// no matter if agent mode is enabled or not.
var oldUIReactRouterPaths = []string{
	"/config",
	"/flags",
	"/service-discovery",
	"/status",
	"/targets",
}

var newUIReactRouterPaths = []string{
	"/config",
	"/flags",
	"/service-discovery",
	"/alertmanager-discovery",
	"/status",
	"/targets",
}

// Paths that are handled by the React router when the Agent mode is set.
var reactRouterAgentPaths = []string{
	"/agent",
}

// Paths that are handled by the React router when the Agent mode is not set.
var oldUIReactRouterServerPaths = []string{
	"/alerts",
	"/graph",
	"/rules",
	"/tsdb-status",
}

var newUIReactRouterServerPaths = []string{
	"/alerts",
	"/query", // The old /graph redirects to /query on the server side.
	"/rules",
	"/tsdb-status",
}

var apiRouterPaths = []string{
	"/scrape_pools",
	"/targets",
	"/targets/metadata",
	"/metadata",
	"/status/config",
	"/status/runtimeinfo",
	"/status/buildinfo",
	"/status/flags",
}

func main() {
	UseOldUI := true
	IsAgent := true
	ExternalURL := &url.URL{
		Scheme: "http",
		Host:   "localhost:9090",
		Path:   "",
	}
	address := ":9090"

	ctx := context.Background()
	logger := promslog.NewNopLogger()
	router := route.New()

	homePage := "/query"
	if UseOldUI {
		homePage = "/graph"
	}
	if IsAgent {
		homePage = "/agent"
	}

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path.Join(ExternalURL.Path, homePage), http.StatusFound)
	})

	if !UseOldUI {
		router.Get("/graph", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, path.Join(ExternalURL.Path, "/query?"+r.URL.RawQuery), http.StatusFound)
		})
	}

	reactAssetsRoot := "/static/mantine-ui"
	if UseOldUI {
		reactAssetsRoot = "/static/react-app"
	}

	// The console library examples at 'console_libraries/prom.lib' still depend on old asset files being served under `classic`.
	router.Get("/classic/static/*filepath", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = path.Join("/static", route.Param(r.Context(), "filepath"))
		fs := server.StaticFileServer(Assets)
		fs.ServeHTTP(w, r)
	})

	//router.Get("/version", Version)
	// router.Get("/metrics", promhttp.Handler().ServeHTTP)

	serveReactApp := func(w http.ResponseWriter, _ *http.Request) {
		indexPath := reactAssetsRoot + "/index.html"
		f, err := Assets.Open(indexPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error opening React index.html: %v", err)
			return
		}
		defer func() { _ = f.Close() }()
		idx, err := io.ReadAll(f)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error reading React index.html: %v", err)
			return
		}
		replacedIdx := bytes.ReplaceAll(idx, []byte("CONSOLES_LINK_PLACEHOLDER"), []byte(""))
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("TITLE_PLACEHOLDER"), []byte("Prometheus Receiver"))
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("AGENT_MODE_PLACEHOLDER"), []byte(strconv.FormatBool(IsAgent)))
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("READY_PLACEHOLDER"), []byte(strconv.FormatBool(true)))
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("LOOKBACKDELTA_PLACEHOLDER"), []byte(model.Duration(time.Minute*5).String()))
		w.Write(replacedIdx)
	}

	// Serve the React app.
	reactRouterPaths := newUIReactRouterPaths
	reactRouterServerPaths := newUIReactRouterServerPaths
	if UseOldUI {
		reactRouterPaths = oldUIReactRouterPaths
		reactRouterServerPaths = oldUIReactRouterServerPaths
	}

	for _, p := range reactRouterPaths {
		router.Get(p, serveReactApp)
	}

	if IsAgent {
		for _, p := range reactRouterAgentPaths {
			router.Get(p, serveReactApp)
		}
	} else {
		for _, p := range reactRouterServerPaths {
			router.Get(p, serveReactApp)
		}
	}

	// The favicon and manifest are bundled as part of the React app, but we want to serve
	// them on the root.
	for _, p := range []string{"/favicon.svg", "/favicon.ico", "/manifest.json"} {
		assetPath := reactAssetsRoot + p
		router.Get(p, func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = assetPath
			fs := server.StaticFileServer(Assets)
			fs.ServeHTTP(w, r)
		})
	}

	reactStaticAssetsDir := "/assets"
	if UseOldUI {
		reactStaticAssetsDir = "/static"
	}
	// Static files required by the React app.
	router.Get(reactStaticAssetsDir+"/*filepath", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = path.Join(reactAssetsRoot+reactStaticAssetsDir, route.Param(r.Context(), "filepath"))
		fs := server.StaticFileServer(Assets)
		fs.ServeHTTP(w, r)
	})

	// ---

	// Route API calls to the port that's hosted by the otelcollector
	// We need a reverse proxy because norm
	api, err := url.Parse("http://localhost:9092")
	if err != nil {
		panic(err)
	}
	proxyHandler := func(p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Host = api.Host
			p.ServeHTTP(w, r)
		}
	}
	proxy := httputil.NewSingleHostReverseProxy(api)

	apiPath := "/api/v1"
	for _, path := range apiRouterPaths {
		router.Get(apiPath+path, proxyHandler(proxy))
	}
	router.Get("/metrics", proxyHandler(proxy))

	mux := http.NewServeMux()
	mux.Handle("/", router)

	errlog := slog.NewLogLogger(logger.Handler(), slog.LevelError)
	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	httpSrv := &http.Server{
		Handler:     withStackTracer(otelhttp.NewHandler(mux, "", spanNameFormatter), logger),
		ErrorLog:    errlog,
		ReadTimeout: time.Duration(model.Duration(time.Minute * 5)),
	}

	logger.Info("Start listening for connections", "address", address)

	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(err)
	}

	errCh := make(chan error, 1)
	webConfig := ""
	go func() {
		errCh <- toolkit_web.Serve(listener, httpSrv, &toolkit_web.FlagConfig{WebConfigFile: &webConfig}, logger)
	}()

	select {
	case e := <-errCh:
		fmt.Errorf("error serving web: %w", e)
		return
	case <-ctx.Done():
		httpSrv.Shutdown(ctx)
		return
	}
}

// withStackTracer logs the stack trace in case the request panics. The function
// will re-raise the error which will then be handled by the net/http package.
// It is needed because the go-kit log package doesn't manage properly the
// panics from net/http (see https://github.com/go-kit/kit/issues/233).
func withStackTracer(h http.Handler, l *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				const size = 64 << 10
				buf := make([]byte, size)
				buf = buf[:runtime.Stack(buf, false)]
				l.Error("panic while serving request", "client", r.RemoteAddr, "url", r.URL, "err", err, "stack", buf)
				panic(err)
			}
		}()
		h.ServeHTTP(w, r)
	})
}
