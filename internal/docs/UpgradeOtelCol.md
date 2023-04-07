The following are the steps to upgrade otel collector.

Example PR - https://github.com/Azure/prometheus-collector/pull/431

** opentelemetry-collector-builder **
* update go.mod to new collector version for all components
update line 18 in main.go with the new collector version

** prometheus-receiver **

Get latest release version and latest prometheusreceiver code:
1. Check for the latest release here: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases
2. git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git
3. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. You can name it whatever you want

* copy over new folder for prometheusreceiver from [here](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver) and create a new copy named prometheusreceiver_copy
* in the new copy folder, rename go.mod module to "github.com/gracewehner/prometheusreceiver" from the old go.mod file
* go.mod - keep the first require block and remove the other blocks
* Delete go.sum file and then run `go mod tidy` to regenerate the go.sum file and other blocks in go.mod with the updated versions

Find new version of github.com/prometheus/prometheus. Put this version in the file /otelcollector/opentelemetry-collector-builder/PROMETHEUS_VERSION
* delete testdata directory
* metrics_receiver.go: rename internal package "github.com/gracewehner/prometheusreceiver/internal"
* metrics_receiver.go: add webhandler code(shown below) in initPrometheusComponents() or Start() function
* metrics_receiver.go: add extra import packages at the top
* metrics_receiver.go: add constants at the top
* run `go mod tidy`

-- Code --
module github.com/gracewehner/prometheusreceiver
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/common/version"
    "github.com/prometheus/prometheus/web"
    // Use same settings as Prometheus web server
    maxConnections = 512
    readTimeoutMinutes = 10
    // Setup settings and logger and create Prometheus web handler
    webOptions := web.Options{
        ScrapeManager: r.scrapeManager,
        Context: ctx,
        ListenAddress: ":9090",
        ExternalURL: &url.URL{
            Scheme: "http",
            Host:   "localhost:9090",
            Path:   "",
        },
        RoutePrefix:    "/",
        ReadTimeout: time.Minute * readTimeoutMinutes,
        PageTitle: "Prometheus Receiver",
        Version: &web.PrometheusVersion{
            Version:   version.Version,
            Revision:  version.Revision,
            Branch:    version.Branch,
            BuildUser: version.BuildUser,
            BuildDate: version.BuildDate,
            GoVersion: version.GoVersion,
        },
        Flags: make(map[string]string),
        MaxConnections: maxConnections,
        IsAgent: true,
        Gatherer:   prometheus.DefaultGatherer,
    }
    go_kit_logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
    webHandler := web.New(go_kit_logger, &webOptions)
    listener, err := webHandler.Listener()
    if err != nil {
        return err
    }
    // Pass config and let the web handler know the config is ready.
    // These are needed because Prometheus allows reloading the config without restarting.
    webHandler.ApplyConfig(r.cfg.PrometheusConfig)
    webHandler.SetReady(true)
    // Uses the same context as the discovery and scrape managers for shutting down
    go func() {
        if err := webHandler.Run(ctx, listener, ""); err != nil {
            r.settings.Logger.Error("Web handler failed", zap.Error(err))
            host.ReportFatalError(err)
        }
    }()
-- End Code ---
prom-config-validator-builder
* update go.mod to new collector version for all components
* try to build to check for any breaking changes to the interfaces used: run make
* For any breaking changes , refer to this page https://github.com/open-telemetry/opentelemetry-collector/releases and update the components in the components.go file.
* For testing, remove go.sum and run `go mod tidy` with the updated changes.
