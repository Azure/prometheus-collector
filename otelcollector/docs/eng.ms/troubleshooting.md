# All or Some of my Metrics are not in Grafana
### 1. Pods Status
* Run `kubectl get pods -n <namespace>` and check the status of the `prometheus-collector` pod(s)
* If the pod is in ContainerCreating state for more than a couple minutes:
  * There is an issue with the `secrets-store-csi-driver` being able to pull from the Azure Key Vault. Check the logs of this pod that is on the same node as the `prometheus-collector` pod for more details. The values given in the helm chart may not exactly match the information for the cert and key vault
* If pod state is `Running` but has restarts:
  * Run `kubectl describe pod <prometheus-collector pod> -n <namespace>`
  * If the reason for the restart is `OOMKilled`, the pod cannot keep up with the volume of metrics. The memory limit can be increased using the values in the helm chart for both the replicaset and the daemonset
  * Pod restarts are expected if configmap changes have been made
### 2. Container Logs
* Run `kubectl get logs <prometheus-collector pod> -n <namespace>`
* Check there are no errors with parsing the Prometheus config, merging with any default scrape targets enabled, and validating the full config
* Check if there are errors from MetricsExtension for authenticating wtih the MDM account
* Check if there are errors from the OpenTelemetry Collector for scraping
### 3. Prometheus UI
* Run `kubectl port-forward <prometheus-collector pod> -n <namespace> 9090`
* Go to `127.0.0.1:9090/config` in a browser. This will have the full scrape configs. Check that the job is there
* `127.0.0.1:9090/service-discovery` will have targets discovered by the service discovery object specified and what the relabel_configs have filtered the targets to
* `127.0.0.1:9090/targets` will have all jobs, the last time the endpoint for that job was scraped, and any errors
### 4. Prometheus-Collector Health Metrics
* Metrics available in a dashboard in Grafana
* Also available locally by running `kubectl port-forward <prometheus-collector pod> -n <namespace> 2234` and going to `127.0.0.1:2234/metrics` in a browser
* Check if issues with volume and too many metrics sending
* Check if the config validation failed
### 5. Debug Mode
* Enable Debug Mode through the Helm Chart values or the `<release-name>-prometheus-collector-settings` configmap by setting `debug-mode.enabled="true"`
* An extra server is created that hosts all the metrics scraped. Run `kubectl port-forward <prometheus-collector pod> -n <namespace> 9091` and go to `127.0.0.1:9091/metrics` in a browser to see if the metrics were scraped by the OpenTelemetry Collector. This can be done for both the replicaset and daemonset pods if advanced mode is enabled
* This mode can affect performance and should only be enabled for a short time for debugging purposes