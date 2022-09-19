
***Additional to current 1P troubleshooting.md, covered by Soham***

* Check Azure Monitor Workspace throttling
* Check kubectl get pods -n kube-system | grep ama-metrics
* Check kubectl logs ama-metrics -n kube-system -c addon-token-adapter
* Check kubectl logs ama-metrics -n kube-system -c prometheus-collector
  * At startup, any initial errors will be printed in red. Warnings will be printed in yellow.
    * To view color, use powershell version >= 7 or a linux distribution
  * Could be an issue getting the IMDS auth token
    * Will log every 5 minutes: No configuration present for the AKS resource
    * Will restart pod every 15 minutes to try again with the error: No configuration present for the AKS resource
* Check kubectl describe pod ama-metrics -n kube-system
  * Will have reason for restarts
  * If otelcollector is not running, the container may have been OOM-killed. See the scale recommendations for the volume of metrics.
* Check that all custom configs are correct, the targets have been discovered for the job, and there are no errors scraping specific targets
  * Example: I am missing metrics from a certain pod.
    * Go to /config to check if scrape job is present with correct settings
    * Go to /service-discovery to find the url of the discovered pod
    * Go to /targets to see if there is an issue scraping that url
    * If there is no issue, follow debug-mode instructions and see if metrics expected are there
    * If metrics are not there, it could be an issue with the name length or number of labels. See these limitations


