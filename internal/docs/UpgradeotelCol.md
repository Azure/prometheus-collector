Here is the shareable screenshare video link for how to upgrade Otel Collector -> <https://microsoft-my.sharepoint.com/:v:/p/sohdasgupta/EYk_qxXtMEtGvz7nfK87N70BVrea5psydVKMO2p4PDsVjA?e=UnotGp>

Below are details for steps to upgrade Otel Collector.

## Release version

Get latest release version and latest prometheusreceiver code:

01. Check for the latest release here: <https://github.com/open-telemetry/opentelemetry-collector-contrib/releases>
02. git clone <https://github.com/open-telemetry/opentelemetry-collector-contrib.git>
03. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name. You can name it whatever you want

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

01. Check for the latest release here: <https://github.com/open-telemetry/opentelemetry-operator/releases> (Pick the same version as opentelemetry-collector)
02. git clone <https://github.com/open-telemetry/opentelemetry-operator.git>
03. git checkout tags/<tag_name> -b <branch_name>   tag name will be in the format of v0.x.x and branch name is your local branch name.
04. Copy the folder otel-allocator
05. Update Dockerfile with the existing Dockerfile changes accordingly(Make sure to include prometheus-operators' api group customization for build command like below)

```
go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com'
```

06. Update main.go to include ARC EULA (lines 69-73)
07. Update go.mod file in the otel-allocator folder with the go.mod of the opentelemetry-operator file.
08. In the file otelcollector/otel-allocator/go.mod add this before imports

```

// pointing to this fork for prometheus-operator since we need fixes for asset store which is only available from v0.84.0 of prometheus-operator
// targetallocator cannot upgrade to v0.84.0 because of this issue - https://github.com/open-telemetry/opentelemetry-operator/issues/4196
// this commit is from this repository -https://github.com/rashmichandrashekar/prometheus-operator/tree/rashmi/v0.81.0-patch-assetstore - which only has the asset store fixes on top of v0.81.0 of prometheus-operator
replace github.com/prometheus-operator/prometheus-operator => github.com/rashmichandrashekar/prometheus-operator v0.0.0-20250715221118-b55ea6d3c138

```

09. Replace the module as module `github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator`
10. In file otelcollector/otel-allocator/internal/watcher/promOperator.go -

* Add imports for the below -

```
promMonitoring "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring"
"k8s.io/client-go/metadata"
```

* In NewPrometheusCRWatcher method add this before GetAllowDenyLists() -

```
mdClient, err := metadata.NewForConfig(cfg.ClusterConfig)
 if err != nil {
  return nil, err
 }
```

Update the below lines with the following lines -

```
factory := informers.NewMonitoringInformerFactories(allowList, denyList, monitoringclient, allocatorconfig.DefaultResyncTime, nil)

 monitoringInformers, err := getInformers(factory, cfg.ClusterConfig, promLogger)
```

```
monitoringInformerFactory := informers.NewMonitoringInformerFactories(allowList, denyList, monitoringclient, allocatorconfig.DefaultResyncTime, nil)
 metaDataInformerFactory := informers.NewMetadataInformerFactory(allowList, denyList, mdClient, allocatorconfig.DefaultResyncTime, nil)
 monitoringInformers, err := getInformers(monitoringInformerFactory, cfg.ClusterConfig, promLogger, metaDataInformerFactory)
```

* Update group. Name value from "monitoring.coreos.com" to promMonitoring. GroupName in the method checkCRDAvailability

* Add additional function parameters in the function getInformers - metaDataInformerFactory informers. FactoriesForNamespaces

* Add the below code before return statement of the method getInformers

```
 secretInformers, err := informers.NewInformersForResourceWithTransform(metaDataInformerFactory, v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets)), informers.PartialObjectMetadataStrip)
 if err != nil {
  return nil, err
 }
 if secretInformers != nil {
  informersMap[string(v1.ResourceSecrets)] = secretInformers
 }
```

* In the function Watch(), replace this code -

```
// only send an event notification if there isn't one already
  resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
   // these functions only write to the notification channel if it's empty to avoid blocking
   // if scrape config updates are being rate-limited
   AddFunc: func(obj interface{}) {
    select {
    case notifyEvents <- struct{}{}:
    default:
    }
   },
   UpdateFunc: func(oldObj, newObj interface{}) {
    select {
    case notifyEvents <- struct{}{}:
    default:
    }
   },
   DeleteFunc: func(obj interface{}) {
    select {
    case notifyEvents <- struct{}{}:
    default:
    }
   },
  })
```

with the below code -

```
// Use a custom event handler for secrets since secret update requires asset store to be updated so that CRs can pick up updated secrets.
  if name == string(v1.ResourceSecrets) {
   w.logger.Info("Using custom event handler for secrets informer", "informer", name)
   // only send an event notification if there isn't one already
   resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
    // these functions only write to the notification channel if it's empty to avoid blocking
    // if scrape config updates are being rate-limited
    AddFunc: func(obj interface{}) {
     select {
     case notifyEvents <- struct{}{}:
     default:
     }
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
     oldMeta, _ := oldObj.(metav1.ObjectMetaAccessor)
     newMeta, _ := newObj.(metav1.ObjectMetaAccessor)
     secretName := newMeta.GetObjectMeta().GetName()
     secretNamespace := newMeta.GetObjectMeta().GetNamespace()
     _, exists, err := w.store.GetObject(&v1.Secret{
      ObjectMeta: metav1.ObjectMeta{
       Name:      secretName,
       Namespace: secretNamespace,
      },
     })
     if !exists || err != nil {
      if err != nil {
       w.logger.Error("unexpected store error when checking if secret exists, skipping update", secretName, "error", err)
       return
      }
      // if the secret does not exist in the store, we skip the update
      return
     }

     newSecret, err := w.store.GetSecretClient().Secrets(secretNamespace).Get(context.Background(), secretName, metav1.GetOptions{})

     if err != nil {
      w.logger.Error("unexpected store error when getting updated secret - ", secretName, "error", err)
      return
     }

     w.logger.Info("Updating secret in store", "newObjName", newMeta.GetObjectMeta().GetName(), "newobjnamespace", newMeta.GetObjectMeta().GetNamespace())
     if err := w.store.UpdateObject(newSecret); err != nil {
      w.logger.Error("unexpected store error when updating secret  - ", newMeta.GetObjectMeta().GetName(), "error", err)
     } else {
      w.logger.Info(
       "Successfully updated store, sending update event to notifyEvents channel",
       "oldObjName", oldMeta.GetObjectMeta().GetName(),
       "oldobjnamespace", oldMeta.GetObjectMeta().GetNamespace(),
       "newObjName", newMeta.GetObjectMeta().GetName(),
       "newobjnamespace", newMeta.GetObjectMeta().GetNamespace(),
      )
      select {
      case notifyEvents <- struct{}{}:
      default:
      }
     }
    },
    DeleteFunc: func(obj interface{}) {
     secretMeta, _ := obj.(metav1.ObjectMetaAccessor)

     secretName := secretMeta.GetObjectMeta().GetName()
     secretNamespace := secretMeta.GetObjectMeta().GetNamespace()

     // check if the secret exists in the store
     secretObj := &v1.Secret{
      ObjectMeta: metav1.ObjectMeta{
       Name:      secretName,
       Namespace: secretNamespace,
      },
     }
     _, exists, err := w.store.GetObject(secretObj)
     // if the secret does not exist in the store, we skip the delete
     if !exists || err != nil {
      if err != nil {
       w.logger.Error("unexpected store error when checking if secret exists, skipping delete", secretMeta.GetObjectMeta().GetName(), "error", err)
       return
      }
      // if the secret does not exist in the store, we skip the delete
      return
     }
     w.logger.Info("Deleting secret from store", "objName", secretMeta.GetObjectMeta().GetName(), "objnamespace", secretMeta.GetObjectMeta().GetNamespace())
     // if the secret exists in the store, we delete it
     // and send an event notification to the notifyEvents channel
     if err := w.store.DeleteObject(secretObj); err != nil {
      w.logger.Error("unexpected store error when deleting secret - ", secretMeta.GetObjectMeta().GetName(), "error", err)
      //return
     } else {
      w.logger.Info(
       "Successfully removed secret from store, sending update event to notifyEvents channel",
       "objName", secretMeta.GetObjectMeta().GetName(),
       "objnamespace", secretMeta.GetObjectMeta().GetNamespace(),
      )
      select {
      case notifyEvents <- struct{}{}:
      default:
      }
     }
    },
   })
  } else {
   w.logger.Info("Using default event handler for informer", "informer", name)
   // only send an event notification if there isn't one already
   resource.AddEventHandler(cache.ResourceEventHandlerFuncs{
    // these functions only write to the notification channel if it's empty to avoid blocking
    // if scrape config updates are being rate-limited
    AddFunc: func(obj interface{}) {
     select {
     case notifyEvents <- struct{}{}:
     default:
     }
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
     select {
     case notifyEvents <- struct{}{}:
     default:
     }
    },
    DeleteFunc: func(obj interface{}) {
     select {
     case notifyEvents <- struct{}{}:
     default:
     }
    },
   })
  }

```

10. Run go mod tidy from the otel-allocator directory and then run make.

## Configuration Reader Builder

01. Update the version of prometheus/common in go.mod of configuration-reader-builder to match versions in go.mod of otel-allocator
02. Run go mod tidy from configuration-reader-builder directory and then run make

## Note about $ $$ changes that we need to test

During upgrades make sure that the environment variable substitution works for the daemonset and shows the substituted value in the prometheus UX as well as in the metrics in Grafana, whereas for the replicaset the environment variable substitution is not expected to work as shows up as ${env:env_var_name} at all places.

---

## Web Handler Refactoring (Custom Changes)
These changes are based on commit 49202c2 pattern.

### New Files Added

Create a new folder at `otelcollector/prometheusreceiver/apiserver/` containing:

1. **config.go**
2. **manager.go**.

Copy over these files from the previous changes in main.

### Key Files Modified

#### 1. `otelcollector/prometheusreceiver/config.go`

**Changed**:
```go
// Before:
APIServer APIServer `mapstructure:"api_server"`

// After:
APIServer configoptional.Optional[apiserver.Config] `mapstructure:"api_server"`
```

**Removed**: Old `APIServer` struct with `Enabled` boolean field

**Added import**: 
```go
"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/apiserver"
```

#### 2. `otelcollector/prometheusreceiver/metrics_receiver.go`

**Removed** (~20 imports related to web/HTTP handling):
- `net/http`, `net/url`, `strings`, `runtime`, `runtime/debug`, `os`
- `github.com/grafana/regexp`
- `github.com/mwitkow/go-conntrack`
- Prometheus web/API packages
- OpenTelemetry HTTP packages

**Changed struct field**:
```go
// Before:
apiServer *http.Server

// After:
apiServerManager *apiserver.Manager
```

**Modified functions**:

- `newPrometheusReceiver()` - Now uses `cfg.APIServer.Get()` and creates Manager instance
- `initPrometheusComponents()` - Calls `apiServerManager.Start()` instead of inline initialization
- `Shutdown()` - Uses `apiServerManager.Shutdown()`

**Removed functions** (~150 lines):
- `initAPIServer()` - All functionality moved to `apiserver.Manager.Start()`
- `setPathWithPrefix()` - Moved to apiserver package

**Type casting fix** in `gcInterval()`:
```go
promCfg := (*promconfig.Config)(cfg)
// Use promCfg instead of cfg for accessing Prometheus config fields
```

#### 3. Deleted File

- `otelcollector/prometheusreceiver/metricsreceiver_api_server_test.go` - Removed (old test file)
