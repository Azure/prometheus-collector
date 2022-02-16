# FAQ
## How do I check the prometheus-collector logs?
The prometheus-collector container prints logs at startup and errors from Metrics Extension.
```
kubectl logs $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector) -n <release-namespace>
```
This will have info about:
- What configmap settings were used.
- The result from running the promconfigvalidator check on a custom config:
  ```
  prom-config-validator::Config file provided - /etc/config/settings/prometheus/prometheus-config
  prom-config-validator::Successfully generated otel config
  prom-config-validator::Loading configuration...
  prom-config-validator::Successfully loaded and validated custom prometheus config
  ```
  This means the custom prometheus config is valid. Otherwise, the errors will be printed.
- The metric account names and results of decoding their certificates. 
- The following processes starting up: otelcollector, metricsextension, telegraf, and fluent-bit.
- Any Metrics Extension errors, including authentication, certificate, and ingestion issues.

## How do I check the Metrics Extension logs?
ME logs are located at the root: `/MetricsExtensionConsoleDebugLog.log`. These are logs at the `INFO` level and include information about metrics received, processed, published, and dropped, as well as any errors. Access either by copying the file from the container:
```
kubectl cp $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector):MetricsExtensionConsoleDebugLog
.log MetricsExtensionConsoleDebugLog.log -n <release-namespace>
```
or exec-ing into the container:
```
kubectl exec -it $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector) -n <release-namespace> -- bash
```