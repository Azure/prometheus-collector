# I don't see metrics flowing in Grafana
## Current Troubleshooting Resources
### Kubectl Get Pods
* Are pods running with no restarts? If not, what is the reason for the restart?
* OOMKilled -> cannot keep up with volume
* MDSD not running -> issues with auth
### Container Logs
* Issues with the Prometheus config or default targets will log there.
* Issues with authenticating with MAC/MDM account will log there.
### Prometheus UI
Port-forward 9090 for the Prometheus-Collector pod.
* localhost:9090/config will have the full scrape configs. Check that the job is there.
* localhost:9090/service-discovery will have targets discovered by the service discovery object specified and what the relabel_configs have filtered the targets to.
* localhost:9090/targets will have all jobs, the last time the endpoint for that job was scraped, and any errors
### Prometheus-Collector Health
* Check if issues with volume/too many metrics sending.
### QoS Dashboard (1P)
* Check if the account is throttled.
* For 3P, this will be handled by Platform Metrics and is being worked on.

## Metrics can still be missing even if everything above looks good
* Three 1P private preview customers have asked for more visibility into what metrics are scraped and sent
* Ask has been for customer to be able to see what metrics are being sent out from the agent. All we can currently see is if there are issues with the target. But if the job and the target report that it's scraping, the investigation for the customer stops there
* One feedback was it's ok to not see every metric that is dropping because of the metric name length, etc. but would at least want to see which metrics are being sent
* Log metrics collected by otelcollector to a log file
  * Adding to the otelcollector pipeline an extra exporter
  * The OTLP 10 switch is blocking this due to issues with the fileexporter in the version we are using now
  * The amount of data can be very large
  * Customer can disable default scrape configs and remove other jobs from their custom one if they just want to debug for one job and disruption is ok
* Verbose logging for otelcollector
* Log otelcollector errors to stdout (in general, not just for debugging mode)
  * Has been blocker because of double-sending to our telemetry because we are sending stdout
* ME can print out all the timeseries and data for a given metric name through ME config [transformation rules](https://eng.ms/docs/products/geneva/metrics/howdoi/transformationrules)
  * Can be compared with what the otelcollector scrapes to see if ME is dropping anything
  * Can be one metric, a list of metrics, or a list of metric prefixes to narrow down. This is unlike printing out all the scrape data of the otelcollector
* Can always add more info like if ME needs to drop metrics due to the metric name size and add this to logging

### Debug/Verbose Mode
* Configmap setting to turn on, restarts container with adding in the above features
* Include in configmap the metric name if they want ME to print that out specifically

```
  debug-mode: |-
    enabled = true
    allowed_metric_names = ["myapp_temperature", "myapp_temperature_count"]
```

* When parsing, environment variable is set and this is used in the various files to add the fileexporter, turn on verbose mode for the otelcollector, change the ME config to include the metric names to log

### Script
* Can script turn on debug mode, sleep for a couple minutes, collect all the logs, zip them up, and then turn off debug mode?
* Or have customer enable/disable debug mode and have them run the script
* Could script also look for errors and print out suggestions of what is wrong?
* Script could also curl the prometheus-collector health endpoint and list out volume and if it is very high
* Script could also grep for if ME cannot keep up or if otelcollector has the errors that it can't keep up
* Not as intuitive for customers to use, simpler to implement

### Port-Forward UI
* Lists info for volume, ME logs/errors, otelcollector errors in different sections
* Would only run when debug mode is turned on?
* Nicer for customers, much more effort to implement
* Might make our image larger, more packages needed for Mariner depending on programming language
* Log files are huge, would need to find a way to trim the info to the latest, but how to decide on that?
* Probably just a GoLang web sever with basic html and css
* Getting the info would similar, could always build this later depending on feedback
* Could have this in addition to the script?

### Perf and Scale
* Only enabled by customer, no impact to running the agent as normal
* Depending on the number of metrics or errors, the log files can fill up very quickly. We have logrotate in place but need to make sure it will work for this scenario. Need to communicate to customers the log files can get large very quickly if they leave this running
* If customer is sending a lot of metrics and wants to see what the otelcollector is scraping, will need to change their configs to only have the job they are looking for or will have very large files

### Troubleshooting Doc
* More details than what we currently have, all in one place. Step-by-step of things to check

### Must be Done
* Debug mode configmap
* OtelCollector error logs to stdout
* ME prints out metric specified
* Troubleshooting doc with a flow of things to check