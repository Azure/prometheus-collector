# Working with Prometheus metrics in MDM

## Release 09-25-2021 

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-main-0.0.5-09-25-2021-e1c22c8`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-main-0.0.5-09-25-2021-e1c22c8`
* Change Log -
  * Add support for sending staleness markers (MDM store has this support added as well to co-ordinate with this release)
  * Configuration Parsing -
    * Added new tool 'promconfigvalidator' to do stricter config validation and tighten prometheus schema validation for custom scrape config provided thru configmap (see documentation for more details)
  * Collector Health -
    * Added 2 metrics (number of samples scraped and size in bytes ingested by the collector agent)
    * Added `Prometheus-Collector Health` dashboard as part of default Grafana dashboards, showing the above health metrics
  * Windows kubernetes support (phase-1) -
    * Added windows kube-proxy and windows-exporter as default targets (which is turned OFF by default, but can be turned ON  for windows clusters as needed)
      * Note : Windows-exporter needs to be manually setup on windows host (see documentation for more details)
    * Added 2 windows dashboards as part of default Grafana dashboards for windows node metrics
      * USE Methos / Cluster(Windows)
      * USE Method / Node(Windows)
  * Add `maxUnavailable` chart parameter for daemonset
  * Dashboard fixes -
    * Remove 'All' option from cluster picker for all default dashboards
    * Fix cross links between dashboards
    * Remove 'All' option for instance variable in node xporter dashboard
    * Expand 2 panels in node exporter dashboard as graphs are squashed due to long legends
    * Include regex =~ for cluster filter
  * Release chart through our brand new release process & automation :)