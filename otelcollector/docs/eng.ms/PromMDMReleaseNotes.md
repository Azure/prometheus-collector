# Working with Prometheus metrics in MDM


## Release 02-15-2022 [Breaking changes]

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector:2.0.0-main-03-17-2022-dfef2a5d`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:2.0.0-main-03-17-2022-dfef2a5d`
* Change Log -
  * BREAKING CHANGE -- Ingest Prometheus metrics to a new mdm namespace (move from `prometheus` namespace to `_azure_managed_prometheus` namespace). See more details [here](./PrometheusNamespace.md)
  * Bug fix - Target windows kubelets from replica in advanved mode when windowsDaemonset=false
  * Bug fix - Fix NODE_NAME (to be windows node's name rather than replica/linux node's name) for windows-exporter & wndows-kube-proxy targets when scraped from replica (i.e mode.advanced=false)
  * Bug fix - Do not scrape cadvisor target for windows nodes, when mode.advanced=false
  * Bug fix - Telemetry metrics (Telegraf) for windows daemonset
  * Use seperate (new) config map for Windows targets that will be picked up only by windows daemonset (see docs for more details)
  * Build improvements [ faster/parallel builds :) ]
  

## Release 03-07-2022

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector:1.1.2-main-03-07-2022-df71b65a`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:1.1.2-main-03-07-2022-df71b65a`
* Change Log -
  * Fixes to patch CVEs


## Release 02-15-2022

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector:1.1.1-main-02-17-2022-d225f7bf`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:1.1.1-main-02-17-2022-d225f7bf`
* Change Log -
  * Fix for CSI driver's secret missing label [Bug](https://msazure.visualstudio.com/InfrastructureInsights/_workitems/edit/13386952)
  * Fix for passing INT FLAG to ME for ingestion to INT environments (internal product team use)

## Release 02-08-2022

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector:1.1.0-main-02-08-2022-573d3086`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:1.1.0-main-02-08-2022-573d3086`
* Change Log -
  * [Support for MSI for accessing Azure Key Vault]((~/metrics/Prometheus/PromMDMTutorial2DeployAgentHELM.md)
    * Both User Assigned & System MSIs are supported
  * [Run collector natively on Windows nodes](~/metrics/Prometheus/windows.md)
    * Optionally run collector on Windows nodes as a Daemonset.
  * Replace IP address with node name for nodeexporter scrapes
  

## Release 11-01-2021 

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector:1.0.0-main-11-01-2021-e86fc50d`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:1.0.0-main-11-01-2021-e86fc50d`
* Change Log -
  * Support for HELM versions >= 3.7.0 
    * This is a breaking change, this chart and image versions only work with helm versions >= 3.7.0
  * Target UX to visualize the config, targets and service discovery  
  * Validate otel config after default and custom scrape configurations are merged
  * Move to MCR for dependent charts
  * Fix telemetry image tag to have registry info
  * Remove ruby-full which reduces image size by 30MB
  * Trivy scan issue fix for skip-dirs
  * Update mongo-driver and containerd to use non vulnerable versions

## Release 09-25-2021 

* chart - `mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-main-0.0.5-09-25-2021-e1c22c83`
* image - `mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-main-0.0.5-09-25-2021-e1c22c83`
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