Here is the shareable screenshare video link for how to upgrade Otel Collector -> https://microsoft-my.sharepoint.com/:v:/p/sohdasgupta/EYk_qxXtMEtGvz7nfK87N70BVrea5psydVKMO2p4PDsVjA?e=UnotGp

Below are details for steps to upgrade Otel Collector.

## Release version
Get latest release version and latest prometheusreceiver code:
1. Check for the latest release here: https://github.com/open-telemetry/opentelemetry-collector-contrib/releases
2. git clone https://github.com/open-telemetry/opentelemetry-collector-contrib.git
3. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. You can name it whatever you want

## opentelemetry-collector-builder
* update go.mod to new collector version for all components
* If there is a later version than the one you are upgrading to, you may need to run below to force download an earlier version
	```
	 go get <package>@<version>
	```
* update the `Version` field in `main.go` with the new collector version

## prometheus-receiver
* copy over new folder
* delete testdata directory

#### go.mod 
* remove replacements at the end of the file

### Prometheus version
* Find new version of github.com/prometheus/prometheus. Put this version in the file /otelcollector/opentelemetry-collector-builder/PROMETHEUS_VERSION

## opentelemetry-collector-builder - 
* go mod tidy
* make

## prom-config-validator-builder
* update go.mod to new collector version for all components
* copy the second block of go.mod from the latest of go.mod of opentelemetry-collector-builder 
* try to build to check for any breaking changes to the interfaces used: 
* Run - go mod tidy
* Run - make

## golang version
* Update the `GOLANG_VERSION` variable in `azure-pipeline-build.yaml` to match the golang version used by the otelcollector (see the go.mod files)

## TargetAllocator Update
Get latest release version and latest prometheusreceiver code:
1. Check for the latest release here: https://github.com/open-telemetry/opentelemetry-operator/releases (Pick the same version as opentelemetry-collector)
2. git clone https://github.com/open-telemetry/opentelemetry-operator.git
3. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. 
4. Copy the folder otel-allocator
5. Update Dockerfile with the existing Dockerfile changes accordingly(Make sure to include prometheus-operators' api group customization for build command like below)
```
go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com'
```
6. Update main.go to include ARC EULA (lines 69-73)
7. In the file - otelcollector/otel-allocator/internal/config/flags.go add the below in the import section.
```
uberzap "go.uber.org/zap"
```
and add the below after *zapCmdLineOpts.BindFlags(zapFlagSet)* in the getFlagSet method.
```
	lvl := uberzap.NewAtomicLevelAt(uberzap.PanicLevel)
	zapCmdLineOpts.Level = &lvl
```
8. In the file otelcollector/otel-allocator/internal/watcher/promOperator.go,
	- add the import `"k8s.io/client-go/metadata"`
	- Replace in the function with:
		```
			mdClient, err := metadata.NewForConfig(cfg.ClusterConfig)
			if err != nil {
				return nil, err
			}

			allowList, denyList := cfg.PrometheusCR.GetAllowDenyLists()

			monitoringInformerFactory := informers.NewMonitoringInformerFactories(allowList, denyList, mClient, allocatorconfig.DefaultResyncTime, nil)
			metaDataInformerFactory := informers.NewMetadataInformerFactory(allowList, denyList, mdClient, allocatorconfig.DefaultResyncTime, nil)
			monitoringInformers, err := getInformers(monitoringInformerFactory, metaDataInformerFactory)
		```
	- Change the getInformers function to:
		```
			func getInformers(factory informers.FactoriesForNamespaces, metaDataInformerFactory informers.FactoriesForNamespaces) (map[string]*informers.ForResource, error) {
		```

9. Update go.mod file in the otel-allocator folder with the go.mod of the opentelemetry-operator file.
10. Replace the module as module `github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator`
11. Add the section `replace github.com/prometheus-operator/prometheus-operator => github.com/rashmichandrashekar/prometheus-operator v0.0.0-20250715221118-b55ea6d3c138` to the go.mod file.
12. Run go mod tidy from the otel-allocator directory and then run make.

## Configuration Reader Builder
1. Update the version of prometheus/common in go.mod of configuration-reader-builder to match versions in go.mod of otel-allocator
2. Run go mod tidy from configuration-reader-builder directory and then run make


## Note about $ $$ changes that we need to test
During upgrades make sure that the environment variable substitution works for the daemonset and shows the substituted value in the prometheus UX as well as in the metrics in Grafana, whereas for the replicaset the environment variable substitution is not expected to work as shows up as ${env:env_var_name} at all places.