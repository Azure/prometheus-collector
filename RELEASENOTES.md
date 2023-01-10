# Azure Monitor Metrics for AKS clusters

## Release 12-14-2022 (This version is being released only internally due to deployment freeze during holidays)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3`
* Change Log -
  * Update addon token adapter (from master.220916.1 to master.221118.2)
  * Enable non-default dashboards & their recording rules (apiserver, kube-proxy, kubedns and kubernetes*)
  * Fix for excluding windows nodes in the node dropdown for k8s computer (nodes) dashboard

## Release 11-29-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.1-main-11-29-2022-97e2122e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.1-main-11-29-2022-97e2122e-win`
* Change Log -
  * Move to mariner base for Linux image
  * Enable ARM64 support (for addon based datacollection only) - Includes both Daemonset & Replicaset
  * Update Kube-state-metrics chart (from 4.18.0 to 4.23.0) [chart only upgrade]
  * Update Prometheus node exporter chart (from 3.1.1 to 4.5.2) and image (from 1.3.1 to 1.4.0) [Remove selector label changes in 1.4.x chart that breaks upgrade]

## Release 10-27-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.4.0-main-10-26-2022-16f02b39`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.4.0-main-10-26-2022-16f02b39-win`
* Change Log -
  * Release custom prometheus config global settings to apply to the default targets in AKS-Addon
  * Rebuild with latest base image for security patches

## Release 10-06-22

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.3.0-main-10-06-2022-c0c49872`
* Change Log -
  * Add capability for the custom prometheus config global settings to apply to the default targets
  * Bug fix - Rollback from otelcollector version 0.59.0 to 0.58.0 due to external labels bug
  * Bug fix - Fix race condition for internal production build

## Release 09-30-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.2.0-main-09-29-2022-ca064de1`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.2.0-main-09-29-2022-ca064de1-win`
* Change Log -
  * Public preview release image for Azure Monitor Metrics on AKS clusters
  * Add NOTICE file for OSS code and Component Governance generated notice for container
  * Bug fix - Add missing region dimension for all telemetry collected thru telegraf
  * Bug fix - Fix memory usage alert which hits multiple matches for labels issue
  * Bug fix - Remove virtual node core capacity from telemetry total
  * Bug fix - Update alert group names for default and CI alerts
  * Bug fix - Update prometheus custom config for Azure Monitor Metrics Addon
  
