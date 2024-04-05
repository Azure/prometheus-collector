Here is the shareable screenshare video link for how to upgrade Otel Collector -> https://microsoft-my.sharepoint.com/:v:/p/sohdasgupta/EYk_qxXtMEtGvz7nfK87N70BVrea5psydVKMO2p4PDsVjA?e=UnotGp

Below are details for steps to upgrade Otel Collector.

Get latest release version and latest prometheusreceiver code:
1. Check for the latest release here: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases
2. git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git
3. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. You can name it whatever you want

> **opentelemetry-collector-builder**
* update go.mod to new collector version for all components
update line 18 in main.go with the new collector version
> **prometheus-receiver**
* copy over new folder
* go.mod rename module
go.mod remove replacements at the end
Find new version of github.com/prometheus/prometheus. Put this version in the file /otelcollector/opentelemetry-collector-builder/PROMETHEUS_VERSION
* delete testdata directory
* metrics_receiver.go: rename internal package "github.com/gracewehner/prometheusreceiver/internal"
<!-- * metrics_receiver.go: add webhandler code in initPrometheusComponents() or Start() function
* metrics_receiver.go: add extra import packages at the top
* metrics_receiver.go: add constants at the top
internal/otlp_transaction.go: in Append() function before if len(t.externalLabels) != 0 (currently line 92) add labels = labels.Copy() -->
prom-config-validator-builder
* update go.mod to new collector version for all components
* try to build to check for any breaking changes to the interfaces used: run make


 ## web handler changes to be added 

**opentelemetry-collector-builder** - 
go mod tidy

<!-- Code block for web handler (This will be moved to extension)
```
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
``` -->

### TargetAllocator Update
Get latest release version and latest prometheusreceiver code:
1. Check for the latest release here: https://github.com/open-telemetry/opentelemetry-operator/releases
2. git clone https://github.com/open-telemetry/opentelemetry-operator.git
3. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. 
4. Copy the folder otel-allocator
5. Update Dockerfile with the existing Dockerfile changes accordingly(Make sure to include prometheus-operators' api group customization for build command like below)
```
go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com'
```
6. Update main.go to include ARC EULA
7. Update go.mod file in the otel-allocator folder with the go.mod of the opentelemetry-operator file.
8. Run go mod tidy from the otel-allocator directory.
9. Update any dependencies in go.mod of configuration-reader-builder to match versions in go.mod of otel-allocator
10. Run go mod tidy from configuration-reader-builder directory