// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	grafanaRegexp "github.com/grafana/regexp"
	"github.com/mwitkow/go-conntrack"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/route"
	"github.com/prometheus/common/version"
	toolkitweb "github.com/prometheus/exporter-toolkit/web"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	"github.com/prometheus/prometheus/web"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"golang.org/x/net/netutil"
)

const (
	maxConnections       = 512
	defaultReadTimeout   = 10 * time.Minute
	apiRoutePrefix       = "/api"
	lookbackDeltaDefault = 5 * time.Minute
)

// Manager owns the lifecycle of the optional embedded Prometheus API server.
type Manager struct {
	settings   receiver.Settings
	cfg        *Config
	registerer prometheus.Registerer
	registry   *prometheus.Registry
	server     *http.Server
}

// NewManager constructs a Manager from the supplied configuration. Returns nil when cfg is nil.
func NewManager(settings receiver.Settings, cfg *Config, registerer prometheus.Registerer, registry *prometheus.Registry) *Manager {
	if cfg == nil {
		return nil
	}
	return &Manager{
		settings:   settings,
		cfg:        cfg,
		registerer: registerer,
		registry:   registry,
	}
}

// Start bootstraps the Prometheus web handler and begins serving requests.
func (m *Manager) Start(ctx context.Context, host component.Host, scrapeManager *scrape.Manager, cfgProvider func() promconfig.Config) error {
	if m == nil {
		return nil
	}
	if scrapeManager == nil {
		return fmt.Errorf("scrape manager must be provided to start API server")
	}
	if cfgProvider == nil {
		return fmt.Errorf("config provider must be supplied to start API server")
	}
	if m.server != nil {
		return fmt.Errorf("API server already started")
	}

	m.settings.Logger.Info("Starting Prometheus API server")

	var corsOriginRegexp *grafanaRegexp.Regexp
	if m.cfg.ServerConfig.CORS.HasValue() {
		allowedOrigins := m.cfg.ServerConfig.CORS.Get().AllowedOrigins
		if len(allowedOrigins) > 0 {
			var b strings.Builder
			b.WriteString(allowedOrigins[0])
			for _, origin := range allowedOrigins[1:] {
				b.WriteString("|")
				b.WriteString(origin)
			}
			combined, err := grafanaRegexp.Compile(b.String())
			if err != nil {
				return fmt.Errorf("failed to compile combined CORS allowed origins into regex: %w", err)
			}
			corsOriginRegexp = combined
		}
	}

	readTimeout := m.cfg.ServerConfig.ReadTimeout
	if readTimeout == 0 {
		readTimeout = defaultReadTimeout
	}

	options := &web.Options{
		ScrapeManager:   scrapeManager,
		Context:         ctx,
		ListenAddresses: []string{m.cfg.ServerConfig.Endpoint},
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   m.cfg.ServerConfig.Endpoint,
			Path:   "",
		},
		RoutePrefix:    "/",
		ReadTimeout:    readTimeout,
		PageTitle:      "Prometheus Receiver",
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Registerer:     m.registerer,
		Gatherer:       m.registry,
		CORSOrigin:     corsOriginRegexp,
	}

	promLogger := promslog.NewNopLogger()
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return options.ScrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return options.ScrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	factoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable

	apiV1 := api_v1.NewAPI(options.QueryEngine, options.Storage, app, options.ExemplarStorage, factorySPr, factoryTr, factoryAr,
		cfgProvider,
		options.Flags,
		api_v1.GlobalURLOptions{
			ListenAddress: options.ListenAddresses[0],
			Host:          options.ExternalURL.Host,
			Scheme:        options.ExternalURL.Scheme,
		},
		func(f http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				f(w, r)
			}
		},
		options.LocalStorage,
		options.TSDBDir,
		options.EnableAdminAPI,
		promLogger,
		factoryRr,
		options.RemoteReadSampleLimit,
		options.RemoteReadConcurrencyLimit,
		options.RemoteReadBytesInFrame,
		options.IsAgent,
		options.CORSOrigin,
		func() (api_v1.RuntimeInfo, error) {
			status := api_v1.RuntimeInfo{
				GoroutineCount: runtime.NumGoroutine(),
				GOMAXPROCS:     runtime.GOMAXPROCS(0),
				GOMEMLIMIT:     debug.SetMemoryLimit(-1),
				GOGC:           os.Getenv("GOGC"),
				GODEBUG:        os.Getenv("GODEBUG"),
			}
			return status, nil
		},
		&web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		options.NotificationsGetter,
		options.NotificationsSub,
		options.Gatherer,
		options.Registerer,
		nil,
		options.EnableRemoteWriteReceiver,
		options.AcceptRemoteWriteProtoMsgs,
		options.EnableOTLPWriteReceiver,
		options.ConvertOTLPDelta,
		options.NativeOTLPDeltaIngestion,
		options.CTZeroIngestionEnabled,
		lookbackDeltaDefault,
		options.EnableTypeAndUnitLabels,
		nil,
	)

	listener, err := m.cfg.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	listener = netutil.LimitListener(listener, options.MaxConnections)
	listener = conntrack.NewListener(listener,
		conntrack.TrackWithName("http"),
		conntrack.TrackWithTracing(),
	)

	mux := http.NewServeMux()
	promHandler := promhttp.HandlerFor(options.Gatherer, promhttp.HandlerOpts{Registry: options.Registerer})
	mux.Handle("/metrics", promHandler)

	apiPath := apiRoutePrefix
	if options.RoutePrefix != "/" {
		apiPath = options.RoutePrefix + apiPath
		m.settings.Logger.Info("Router prefix", zap.String("prefix", options.RoutePrefix))
	}
	router := route.New().WithInstrumentation(setPathWithPrefix(apiPath + "/v1"))
	apiV1.Register(router)
	mux.Handle(apiPath+"/v1/", http.StripPrefix(apiPath+"/v1", router))

	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	m.server, err = m.cfg.ServerConfig.ToServer(ctx, host.GetExtensions(), m.settings.TelemetrySettings, otelhttp.NewHandler(mux, "", spanNameFormatter))
	if err != nil {
		return fmt.Errorf("failed to create API server: %w", err)
	}

	webConfig := ""
	go func() {
		if err := toolkitweb.Serve(listener, m.server, &toolkitweb.FlagConfig{WebConfigFile: &webConfig}, promLogger); err != nil {
			m.settings.Logger.Error("API server failed", zap.Error(err))
		}
	}()

	return nil
}

// Shutdown stops the underlying HTTP server.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil || m.server == nil {
		return nil
	}
	return m.server.Shutdown(ctx)
}

func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(_ string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(httputil.ContextWithPath(r.Context(), prefix+r.URL.Path)))
		}
	}
}
