# Azure Monitor Metrics for AKS clusters

## Release 03-22-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.5.0-main-03-24-2023-7eb3f5c7`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.5.0-main-03-24-2023-7eb3f5c7-win`
* Change log -
  * feat: Support for ARC-A
  * fix:  Match ME setings between DS & RS
  * feat: Simplify Pod annotation based scraping by adding it as a target in the configmap
  * fix:  Add golang race detector during builds (SDL requirement)
  * fix:  Reduce telemetry volume
  * feat: Make deployment progress deadline configurable as a param (controbution from @OriYosefiMSFT)
  * feat: Enable workload identity for valur store (1p only) (contribution from @lnr0626)
  * fix:  Bump client-go and lumberjack.v2 packages for fluentbit
  * fix:  Upgrades
          Node exporter - image from: `v1.3.1` to:`v1.5.0`; chart from:`3.1.1` to:`4.14.0`
          Kube state    - image from: `v2.6.0` to:`v2.8.1`; chart from:`4.23.0` to `4.32.0`
          ME            - from:`2.2022.1201.1140` to:`2.2023.224.2214`
          MDSD          - from:`1.23.4` to:`1.23.5`
          MA            - from:`46.2.3` to: `46.4.1`
          Telegraf(linux) - from `1.23.0` to `1.25.2`
  * fix: CVEs (many)

## Release 02-22-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.4.0-main-02-22-2023-3ee44b9e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.4.0-main-02-22-2023-3ee44b9e-win`
* Change log -
  * feat: Allow setting a priority class for the daemonset and deployment objects
  * fix:  Truncate the tag to 128 characters (docker requirement)
  * fix:  Bump github.com/prometheus/client_golang from 1.9.0 to 1.11.1 in /internal/referenceapp/golang
  * feat: HTTP Proxy for distroless and ARC
  * fix:  Wait for token adapter to be healthy before starting dependencies
  * feat: Add azure policy templates for metrics addon
  * feat: enable network monitoring metrics (kappie)
  * feat: AKS addon HTTP Proxy Support
  * fix:  certificate import for windows ME startup

## Release 01-31-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.2.0-main-01-31-2023-e1e3858b`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.2.0-main-01-31-2023-e1e3858b-win`
* Change log -
  * Adding Bicep template to monitoring addon
  * Added custom DCR and DCE arm templates for Remote Write
  * Adding monitoring reader role to Azure Monitor Workspace in ARM and Bicep templates
  * Fix fluent-bit daemonset tailing path and mariner docs
  * Liveness probe update for NON-MAC mode (windows)
  * Adds windows daemonset support with MSI (only in deprecated chart mode)

## Release 01-11-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.1.0-main-01-11-2023-5bf41607`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.1.0-main-01-11-2023-5bf41607-win`
* Change log -
  * Upgrade otel (from 0.58 to 0.66)
  * Upgrade ME (from 2.2022.1021.1309 to 2.2022.1201.1140)
  * Upgrade mdsd (from azure-mdsd_1.19.3-build.master.428 to azure-mdsd_1.23.4-build.master.28)

## Release 12-14-2022 (This version is being released only internally due to deployment freeze during holidays)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3-win`
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
