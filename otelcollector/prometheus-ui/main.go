package main

import (
	"bytes"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"runtime"
	"strconv"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/server"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Paths that are handled by the React / Reach router that should all be served the main React app's index.html.
var reactRouterPaths = []string{
	"/config",
	"/flags",
	"/service-discovery",
	"/status",
	"/targets",
	"/starting",
}

// Paths that are handled by the React router when the Agent mode is set.
var reactRouterAgentPaths = []string{
	"/agent",
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
	router := route.New()

	serveReactApp := func(w http.ResponseWriter, r *http.Request) {
		f, err := Assets.Open("/static/react/index.html")
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
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("AGENT_MODE_PLACEHOLDER"), []byte(strconv.FormatBool(true)))
		replacedIdx = bytes.ReplaceAll(replacedIdx, []byte("READY_PLACEHOLDER"), []byte(strconv.FormatBool(true)))
		w.Write(replacedIdx)
	}

	// Serve the React app.
	for _, p := range reactRouterPaths {
		router.Get(p, serveReactApp)
	}
	for _, p := range reactRouterAgentPaths {
		router.Get(p, serveReactApp)
	}
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	// The favicon and manifest are bundled as part of the React app, but we want to serve
	// them on the root.
	for _, p := range []string{"/favicon.ico", "/manifest.json"} {
		assetPath := "/static/react" + p
		router.Get(p, func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = assetPath
			fs := server.StaticFileServer(Assets)
			fs.ServeHTTP(w, r)
		})
	}

	// Static files required by the React app.
	router.Get("/static/*filepath", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = path.Join("/static/react/static", route.Param(r.Context(), "filepath"))
		fs := server.StaticFileServer(Assets)
		fs.ServeHTTP(w, r)
	})

	// Route API calls to the port that's hosted by the otelcollector
	// We need a reverse proxy because norm
	api, err := url.Parse("http://localhost:9090")
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
		router.Get(apiPath + path, proxyHandler(proxy))
	}

	mux := http.NewServeMux()
	mux.Handle("/", router)

	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	errlog := stdlog.New(log.NewStdlibAdapter(level.Error(logger)), "", 0)
	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})

	httpSrv := &http.Server{
		Handler:      withStackTracer(otelhttp.NewHandler(mux, "", spanNameFormatter), logger),
		ReadTimeout: 100000,
		ErrorLog:    errlog,
	}

	listener, err := net.Listen("tcp", ":9091")
	if err != nil {
		panic(err)
	}

	errCh := make(chan error, 1)
	webconfig := ""
	go func() {
	  errCh <- toolkit_web.Serve(listener, httpSrv, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger)
	}()

	select {
	case e := <-errCh:
		fmt.Println(e.Error())
	}
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