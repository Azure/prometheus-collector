# Azure Monitor Metrics for AKS clusters

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