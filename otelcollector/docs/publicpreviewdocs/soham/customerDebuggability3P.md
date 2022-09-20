# All or Some of my Metrics are not in Grafana
### 1. Pods Status
* Check Azure Monitor Workspace throttling
* Check kubectl get pods -n kube-system | grep ama-metrics and check the status of the pod(s) ![Pods status screenshot](./podstatus.png)  
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
* Check kubectl logs ama-metrics -n kube-system -c addon-token-adapter ![addon token log screenshot](./addontokenadapter.png)  
* Check kubectl logs ama-metrics -n kube-system -c prometheus-collector ![collector log screenshot](./collector%20logs.png)  
  * At startup, any initial errors will be printed in red. Warnings will be printed in yellow.
    * To view color, use powershell version >= 7 or a linux distribution
  * Could be an issue getting the IMDS auth token
    * Will log every 5 minutes: No configuration present for the AKS resource
    * Will restart pod every 15 minutes to try again with the error: No configuration present for the AKS resource
* Check kubectl describe pod ama-metrics -n kube-system
  * Will have reason for restarts
  * If otelcollector is not running, the container may have been OOM-killed. See the scale recommendations for the volume of metrics.
### 3. Prometheus UI
* Run `kubectl port-forward <prometheus-collector pod> -n <namespace> 9090` ![Portforward screenshot](Port-forward.png/)  
* Go to `127.0.0.1:9090/config` in a browser. This will have the full scrape configs. Check that the job is there ![Config ui screenshot](./config-ui.png)  
* `127.0.0.1:9090/service-discovery` will have targets discovered by the service discovery object specified and what the relabel_configs have filtered the targets to ![Service discovery screenshot](./service-discovery.png)  
* `127.0.0.1:9090/targets` will have all jobs, the last time the endpoint for that job was scraped, and any errors ![Targets screenshot](./targets.png)  
*    Check that all custom configs are correct, the targets have been discovered for the job, and there are no errors scraping specific targets
  * Example: I am missing metrics from a certain pod.
    * Go to /config to check if scrape job is present with correct settings
    * Go to /service-discovery to find the url of the discovered pod
    * Go to /targets to see if there is an issue scraping that url
    * If there is no issue, follow debug-mode instructions and see if metrics expected are there
    * If metrics are not there, it could be an issue with the name length or number of labels. See the limitations below
### 4. Debug Mode
* Enable Debug Mode through the Helm Chart values or the `<release-name>-prometheus-collector-settings` configmap by setting `debug-mode.enabled="true"` ![Debug Mode](./debugMode.png)  
* An extra server is created that hosts all the metrics scraped. Run `kubectl port-forward <prometheus-collector pod> -n <namespace> 9091` and go to `127.0.0.1:9091/metrics` in a browser to see if the metrics were scraped by the OpenTelemetry Collector. This can be done for both the replicaset and daemonset pods if advanced mode is enabled ![Debug Mode](./debugModeMetrics.png)  
* This mode can affect performance and should only be enabled for a short time for debugging purposes
### 5. Metric names, label names & label values
* We currently enforce the below limits for agent based scraping
  * Label name length - less than or equal to 511 characters. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
  * Label value length  - less than or equal to 1023 characters. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
  * Number of labels per timeseries - less than or equal to 63. When this limit is exceeded for any time-series in a job, the entire scrape job will be failed and metrics will be dropped from that job before ingestion. You can see up=0 for that job and also target Ux will show the reason for up=0.
  * Metric name length - less than or equal to 511 characters. When this limit is exceeded for any time-series in a job, only that particular series will be dropped. MetricextensionConsoleDebugLog will have traces for the dropped metric.