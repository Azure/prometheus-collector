// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/mwitkow/go-conntrack"

	"github.com/go-kit/log"
	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	promHTTP "github.com/prometheus/prometheus/discovery/http"
	"github.com/prometheus/prometheus/scrape"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/gracewehner/prometheusreceiver/internal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
	"github.com/prometheus/prometheus/web"

	"github.com/prometheus/common/route"
	toolkit_web "github.com/prometheus/exporter-toolkit/web"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/net/netutil"

	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/httputil"
	api_v1 "github.com/prometheus/prometheus/web/api/v1"
)

const (
	defaultGCInterval = 2 * time.Minute
	gcIntervalDelta   = 1 * time.Minute
	// Use same settings as Prometheus web server
	maxConnections     = 512
	readTimeoutMinutes = 10
)

// pReceiver is the type that provides Prometheus scraper/receiver functionality.
type pReceiver struct {
	cfg                 *Config
	consumer            consumer.Metrics
	cancelFunc          context.CancelFunc
	targetAllocatorStop chan struct{}
	configLoaded        chan struct{}
	loadConfigOnce      sync.Once

	settings         receiver.CreateSettings
	scrapeManager    *scrape.Manager
	discoveryManager *discovery.Manager
	webHandler       *web.Handler
}

func setPathWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			handler(w, r.WithContext(httputil.ContextWithPath(r.Context(), prefix+r.URL.Path)))
		}
	}
}

// New creates a new prometheus.Receiver reference.
func newPrometheusReceiver(set receiver.CreateSettings, cfg *Config, next consumer.Metrics) *pReceiver {
	pr := &pReceiver{
		cfg:                 cfg,
		consumer:            next,
		settings:            set,
		configLoaded:        make(chan struct{}),
		targetAllocatorStop: make(chan struct{}),
	}
	return pr
}

// Start is the method that starts Prometheus scraping. It
// is controlled by having previously defined a Configuration using perhaps New.
func (r *pReceiver) Start(_ context.Context, host component.Host) error {
	discoveryCtx, cancel := context.WithCancel(context.Background())
	r.cancelFunc = cancel

	logger := internal.NewZapToGokitLogAdapter(r.settings.Logger)

	// add scrape configs defined by the collector configs
	baseCfg := r.cfg.PrometheusConfig

	err := r.initPrometheusComponents(discoveryCtx, host, logger)
	if err != nil {
		r.settings.Logger.Error("Failed to initPrometheusComponents Prometheus components", zap.Error(err))
		return err
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return err
	}

	allocConf := r.cfg.TargetAllocator
	if allocConf != nil {
		err = r.startTargetAllocator(allocConf, baseCfg)
		if err != nil {
			return err
		}
	}

	r.loadConfigOnce.Do(func() {
		close(r.configLoaded)
	})

	return nil
}

func (r *pReceiver) startTargetAllocator(allocConf *targetAllocator, baseCfg *config.Config) error {
	r.settings.Logger.Info("Starting target allocator discovery")
	// immediately sync jobs, not waiting for the first tick
	savedHash, err := r.syncTargetAllocator(nil, allocConf, baseCfg)
	if err != nil {
		return err
	}
	go func() {
		targetAllocatorIntervalTicker := time.NewTicker(allocConf.Interval)
		for {
			select {
			case <-targetAllocatorIntervalTicker.C:
				hash, newErr := r.syncTargetAllocator(savedHash, allocConf, baseCfg)
				if newErr != nil {
					r.settings.Logger.Error(newErr.Error())
					continue
				}
				savedHash = hash
			case <-r.targetAllocatorStop:
				targetAllocatorIntervalTicker.Stop()
				r.settings.Logger.Info("Stopping target allocator")
				return
			}
		}
	}()
	return nil
}

// Calculate a hash for a scrape config map.
// This is done by marshaling to YAML because it's the most straightforward and doesn't run into problems with unexported fields.
func getScrapeConfigHash(jobToScrapeConfig map[string]*config.ScrapeConfig) (hash.Hash64, error) {
	var err error
	hash := fnv.New64()
	yamlEncoder := yaml.NewEncoder(hash)
	for jobName, scrapeConfig := range jobToScrapeConfig {
		_, err = hash.Write([]byte(jobName))
		if err != nil {
			return nil, err
		}
		err = yamlEncoder.Encode(scrapeConfig)
		if err != nil {
			return nil, err
		}
	}
	yamlEncoder.Close()
	return hash, err
}

// syncTargetAllocator request jobs from targetAllocator and update underlying receiver, if the response does not match the provided compareHash.
// baseDiscoveryCfg can be used to provide additional ScrapeConfigs which will be added to the retrieved jobs.
func (r *pReceiver) syncTargetAllocator(compareHash hash.Hash64, allocConf *targetAllocator, baseCfg *config.Config) (hash.Hash64, error) {
	r.settings.Logger.Debug("Syncing target allocator jobs")
	scrapeConfigsResponse, err := r.getScrapeConfigsResponse(allocConf.Endpoint)
	if err != nil {
		r.settings.Logger.Error("Failed to retrieve job list", zap.Error(err))
		return nil, err
	}

	hash, err := getScrapeConfigHash(scrapeConfigsResponse)
	if err != nil {
		r.settings.Logger.Error("Failed to hash job list", zap.Error(err))
		return nil, err
	}
	if hash == compareHash {
		// no update needed
		return hash, nil
	}

	// Clear out the current configurations
	baseCfg.ScrapeConfigs = []*config.ScrapeConfig{}

	for jobName, scrapeConfig := range scrapeConfigsResponse {
		var httpSD promHTTP.SDConfig
		if allocConf.HTTPSDConfig == nil {
			httpSD = promHTTP.SDConfig{
				RefreshInterval: model.Duration(30 * time.Second),
			}
		} else {
			httpSD = *allocConf.HTTPSDConfig
		}
		escapedJob := url.QueryEscape(jobName)
		httpSD.URL = fmt.Sprintf("%s/jobs/%s/targets?collector_id=%s", allocConf.Endpoint, escapedJob, allocConf.CollectorID)
		httpSD.HTTPClientConfig.FollowRedirects = false
		scrapeConfig.ServiceDiscoveryConfigs = discovery.Configs{
			&httpSD,
		}

		baseCfg.ScrapeConfigs = append(baseCfg.ScrapeConfigs, scrapeConfig)
	}

	err = r.applyCfg(baseCfg)
	if err != nil {
		r.settings.Logger.Error("Failed to apply new scrape configuration", zap.Error(err))
		return nil, err
	}

	return hash, nil
}

// instantiateShard inserts the SHARD environment variable in the returned configuration
func (r *pReceiver) instantiateShard(body []byte) []byte {
	shard, ok := os.LookupEnv("SHARD")
	if !ok {
		shard = "0"
	}
	return bytes.ReplaceAll(body, []byte("$(SHARD)"), []byte(shard))
}

func (r *pReceiver) getScrapeConfigsResponse(baseURL string) (map[string]*config.ScrapeConfig, error) {
	scrapeConfigsURL := fmt.Sprintf("%s/scrape_configs", baseURL)
	_, err := url.Parse(scrapeConfigsURL) // check if valid
	if err != nil {
		return nil, err
	}

	resp, err := http.Get(scrapeConfigsURL) //nolint
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	jobToScrapeConfig := map[string]*config.ScrapeConfig{}
	envReplacedBody := r.instantiateShard(body)
	err = yaml.Unmarshal(envReplacedBody, &jobToScrapeConfig)
	if err != nil {
		return nil, err
	}
	err = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	return jobToScrapeConfig, nil
}

func (r *pReceiver) applyCfg(cfg *config.Config) error {
	if err := r.scrapeManager.ApplyConfig(cfg); err != nil {
		return err
	}

	discoveryCfg := make(map[string]discovery.Configs)
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		discoveryCfg[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		r.settings.Logger.Info("Scrape job added", zap.String("jobName", scrapeConfig.JobName))
	}
	if err := r.discoveryManager.ApplyConfig(discoveryCfg); err != nil {
		return err
	}

	//r.webHandler.ApplyConfig(cfg)

	return nil
}

func (r *pReceiver) initPrometheusComponents(ctx context.Context, host component.Host, logger log.Logger) error {
	r.discoveryManager = discovery.NewManager(ctx, logger)

	go func() {
		r.settings.Logger.Info("Starting discovery manager")
		if err := r.discoveryManager.Run(); err != nil && !errors.Is(err, context.Canceled) {
			r.settings.Logger.Error("Discovery manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	var startTimeMetricRegex *regexp.Regexp
	if r.cfg.StartTimeMetricRegex != "" {
		var err error
		startTimeMetricRegex, err = regexp.Compile(r.cfg.StartTimeMetricRegex)
		if err != nil {
			return err
		}
	}

	store, err := internal.NewAppendable(
		r.consumer,
		r.settings,
		gcInterval(r.cfg.PrometheusConfig),
		r.cfg.UseStartTimeMetric,
		startTimeMetricRegex,
		useCreatedMetricGate.IsEnabled(),
		r.cfg.PrometheusConfig.GlobalConfig.ExternalLabels,
		r.cfg.TrimMetricSuffixes,
	)
	if err != nil {
		return err
	}

	r.scrapeManager = scrape.NewManager(&scrape.Options{
		PassMetadataInContext:     true,
		EnableProtobufNegotiation: r.cfg.EnableProtobufNegotiation,
		ExtraMetrics:              r.cfg.ReportExtraScrapeMetrics,
		HTTPClientOptions: []commonconfig.HTTPClientOption{
			commonconfig.WithUserAgent(r.settings.BuildInfo.Command + "/" + r.settings.BuildInfo.Version),
		},
	}, logger, store)

	go func() {
		// The scrape manager needs to wait for the configuration to be loaded before beginning
		<-r.configLoaded
		r.settings.Logger.Info("Starting scrape manager")
		if err := r.scrapeManager.Run(r.discoveryManager.SyncCh()); err != nil {
			r.settings.Logger.Error("Scrape manager failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()

	// Create Options just for easy readability for creating the API object.
	// These settings are more applicable for what we want to expose for configuration for the Prometheus Receiver.
	o := &web.Options{
		ScrapeManager: r.scrapeManager,
		Context:       ctx,
		ListenAddress: ":9090",
		ExternalURL: &url.URL{
			Scheme: "http",
			Host:   "localhost:9090",
			Path:   "",
		},
		RoutePrefix: "/",
		ReadTimeout: time.Minute * readTimeoutMinutes,
		PageTitle:   "Prometheus Receiver",
		Version: &web.PrometheusVersion{
			Version:   version.Version,
			Revision:  version.Revision,
			Branch:    version.Branch,
			BuildUser: version.BuildUser,
			BuildDate: version.BuildDate,
			GoVersion: version.GoVersion,
		},
		Flags:          make(map[string]string),
		MaxConnections: maxConnections,
		IsAgent:        true,
		Gatherer:       prometheus.DefaultGatherer,
	}

	// Creates the API object in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L314-L354
	// Anything not defined by the options above will be nil, such as o.QueryEngine, o.Storage, etc. IsAgent=true, so these being nil is expected by Prometheus.
	factorySPr := func(_ context.Context) api_v1.ScrapePoolsRetriever { return r.scrapeManager }
	factoryTr := func(_ context.Context) api_v1.TargetRetriever { return r.scrapeManager }
	factoryAr := func(_ context.Context) api_v1.AlertmanagerRetriever { return nil }
	FactoryRr := func(_ context.Context) api_v1.RulesRetriever { return nil }
	var app storage.Appendable
	logger = log.NewNopLogger()

	apiV1 := api_v1.NewAPI(o.QueryEngine, o.Storage, app, o.ExemplarStorage, factorySPr, factoryTr, factoryAr,
		func() config.Config {
			return *r.cfg.PrometheusConfig
		},
		o.Flags,
		api_v1.GlobalURLOptions{
			ListenAddress: o.ListenAddress,
			Host:          o.ExternalURL.Host,
			Scheme:        o.ExternalURL.Scheme,
		},
		func(f http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				f(w, r)
			}
		},
		o.LocalStorage,
		o.TSDBDir,
		o.EnableAdminAPI,
		logger,
		FactoryRr,
		o.RemoteReadSampleLimit,
		o.RemoteReadConcurrencyLimit,
		o.RemoteReadBytesInFrame,
		o.IsAgent,
		o.CORSOrigin,
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
		nil,
		o.Gatherer,
		o.Registerer,
		nil,
		o.EnableRemoteWriteReceiver,
		o.EnableOTLPWriteReceiver,
	)

	// Create listener and monitor with conntrack in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L564-L579
	level.Info(logger).Log("msg", "Start listening for connections", "address", o.ListenAddress)
	listener, err := net.Listen("tcp", o.ListenAddress)
	if err != nil {
		return err
	}
	listener = netutil.LimitListener(listener, o.MaxConnections)
	listener = conntrack.NewListener(listener,
		conntrack.TrackWithName("http"),
		conntrack.TrackWithTracing())

	// Run the API server in the same way as the Prometheus web package: https://github.com/prometheus/prometheus/blob/6150e1ca0ede508e56414363cc9062ef522db518/web/web.go#L582-L630
	mux := http.NewServeMux()
	router := route.New().WithInstrumentation(setPathWithPrefix(""))
	mux.Handle("/", router)

	// This is the path the web package uses, but the router above with no prefix can also be Registered by apiV1 instead.
	apiPath := "/api"
	if o.RoutePrefix != "/" {
		apiPath = o.RoutePrefix + apiPath
		level.Info(logger).Log("msg", "Router prefix", "prefix", o.RoutePrefix)
	}
	av1 := route.New().
		WithInstrumentation(setPathWithPrefix(apiPath + "/v1"))
	apiV1.Register(av1)
	mux.Handle(apiPath+"/v1/", http.StripPrefix(apiPath+"/v1", av1))

	errlog := stdlog.New(log.NewStdlibAdapter(level.Error(logger)), "", 0)
	spanNameFormatter := otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	})
	httpSrv := &http.Server{
		Handler:     otelhttp.NewHandler(mux, "", spanNameFormatter),
		ErrorLog:    errlog,
		ReadTimeout: o.ReadTimeout,
	}
	webconfig := ""

	// An error channel will be needed for graceful shutdown in the Shutdown() method for the receiver
	go func() {
		toolkit_web.Serve(listener, httpSrv, &toolkit_web.FlagConfig{WebConfigFile: &webconfig}, logger)
	}()
	
	return nil
}

// gcInterval returns the longest scrape interval used by a scrape config,
// plus a delta to prevent race conditions.
// This ensures jobs are not garbage collected between scrapes.
func gcInterval(cfg *config.Config) time.Duration {
	gcInterval := defaultGCInterval
	if time.Duration(cfg.GlobalConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
		gcInterval = time.Duration(cfg.GlobalConfig.ScrapeInterval) + gcIntervalDelta
	}
	for _, scrapeConfig := range cfg.ScrapeConfigs {
		if time.Duration(scrapeConfig.ScrapeInterval)+gcIntervalDelta > gcInterval {
			gcInterval = time.Duration(scrapeConfig.ScrapeInterval) + gcIntervalDelta
		}
	}
	return gcInterval
}

// Shutdown stops and cancels the underlying Prometheus scrapers.
func (r *pReceiver) Shutdown(context.Context) error {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
	if r.scrapeManager != nil {
		r.scrapeManager.Stop()
	}
	close(r.targetAllocatorStop)
	return nil
}
