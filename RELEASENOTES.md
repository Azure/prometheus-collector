# Azure Monitor Metrics for AKS clusters

## Release 06-12-2024
* Target Allocator Chart update to add Tls secret to the config reader sidecar container  

## Release 06-10-2024 (CCP release only)
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-cfg`
* Change log -
  * fix: honor minimal ingestion profile setting for ccp - https://github.com/Azure/prometheus-collector/pull/911

## Release 05-29-2024
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-cfg`
* Change log -
  * fix: update kube-state-metrics from: `v2.9.2` to: `v2.12.0` - https://github.com/Azure/prometheus-collector/pull/887
  * fix: switch to Managed Identity for ARC release - https://github.com/Azure/prometheus-collector/pull/895
  * fix: move PV metrics to correct job (from kubelet to k-s-m) - https://github.com/Azure/prometheus-collector/pull/898
    * `kube_persistentvolumeclaim_access_mode`
    * `kube_persistentvolumeclaim_labels`
    * `kube_persistentvolume_status_phase`
  * fix: signature artifacts drop issue - https://github.com/Azure/prometheus-collector/pull/885/files
  * fix: revert Telegraf removal (i.e revert PRs #766 & #841) - https://github.com/Azure/prometheus-collector/pull/899

## Release 05-20-2024 (CCP release only)
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-cfg`
* Change log -
  * fix: ccp ignore minimal ingestion profile setting and respecting the keep list regex values - https://github.com/Azure/prometheus-collector/pull/886
  * fix: signature artifacts drop issue - https://github.com/Azure/prometheus-collector/pull/885/files
  * fix: Remove histograms from minimal ingestion list - ccp metrics collector - https://github.com/Azure/prometheus-collector/pull/884

## Release 05-07-2024 (CCP release only)
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-cfg`
* Change log -
  * fix: ccp metrics missing cluster value

## Release 05-03-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-cfg`
* Change log -
  * fix: update to use older proxy setup for mdsd in aks - https://github.com/Azure/prometheus-collector/pull/864
  * add remaining sdl scans similar to onebranch default - https://github.com/Azure/prometheus-collector/pull/858
  * Add life cycle metadata to container image - https://github.com/Azure/prometheus-collector/pull/842
  * Migrate to ESRP version 5 from version 3
  * [fix] Fix the fluent bit error when collector health is enabled - https://github.com/Azure/prometheus-collector/pull/841
  * Test
    * add Windows tests for Prometheus Target UX - https://github.com/Azure/prometheus-collector/pull/839
    * small fixes to CRs and running tests - https://github.com/Azure/prometheus-collector/pull/835
  * Various build fixes
    * https://github.com/Azure/prometheus-collector/pull/834
    * https://github.com/Azure/prometheus-collector/pull/831
    * https://github.com/Azure/prometheus-collector/pull/827
  * fix (ccp) : Relabel host for Control Plane Metrics - (https://github.com/Azure/prometheus-collector/pull/833)
  * Adding filter strategy - https://github.com/Azure/prometheus-collector/pull/832/files
  * fix: set hubble minimal ingestion profile - https://github.com/Azure/prometheus-collector/pull/829
  * [fix] Minor fix in onboarding templates - https://github.com/Azure/prometheus-collector/pull/828
  * Remove telegraf for telemetry and only use fluent-bit
  * perf: add namespace selector to default jobs to improve perf - https://github.com/Azure/prometheus-collector/pull/867
  * set hubble minimal ingestion profile - https://github.com/Azure/prometheus-collector/pull/860
  * Upgrade Metrics Extension (Linux & windows) from metricsext2-2.2024.328.1744 --> metricsext2-2.2024.419.1535 (This fixes the HDInsights bug (OOM) on flint clusters)

## Release 04-08-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-cfg`
* Change log -
  * fix: Fix: For Arc, use a default value CloudEnvironment that customers can change for other clouds- https://github.com/Azure/prometheus-collector/pull/753
  * Upgrade: fluent-bit from 2.0.9 to 2.1.10 - https://github.com/Azure/prometheus-collector/pull/809
  * Update mdsd, MA, MetricsExtension
    * Linux
      * mdsd 1.27.4 --> 1.30.3
      * MetricsExtension 2.2023.928.2134 --> 2.2024.328.1744
    * Windows
      * MA 46.4.1 --> 46.15.4
      * MetricsExtension 2.2023.224.2214 -> 2.2024.328.1744
  * Upgrading telegraf from 1.27.3 to 1.28.5 for linux
  * fix: Change logging from error to info for missing configmap settings to not have errors for older configmaps - https://github.com/Azure/prometheus-collector/pull/804
  * feat : add support for per cloud AI instance - https://github.com/Azure/prometheus-collector/pull/798
  * Step 0 : Merge CCP changes to main with a separate image - https://github.com/Azure/prometheus-collector/pull/653


## Release 03-08-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-cfg`
* Change log -
  * fix: Fix for yaml unmarshal bug for keepequal/dropequal - https://github.com/Azure/prometheus-collector/pull/753
  * fix: dollar fix for TA - https://github.com/Azure/prometheus-collector/pull/769
  * ENable operator with CRD support globally - (This will be rolled out once the image roll out is complete)
  * Add new regions for Azure Monitor Workspace ('brazilsoutheast', 'francecentral', 'ukwest', 'koreasouth', 'switzerlandwest', 'japaneast', 'swedencentral', 'canadaeast', 'norwaywest', 'southindia', 'australiaeast', 'swedensouth')

## Release 02-14-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-cfg`
* Change log -
  * feat: add ccp config map settings for public preview
  * feat: Enable MTLS authentication
  * fix: add some metrics for civ2 ux
  * fix: Add telemetry for collector and addon token adaptor
  * fix: Set autoresolve to true for new agent version alert
  * fix: SDL Requirment : add policheck
  * fix: [infra] Fix commented out ARC deploy chart condition
  * fix: stop copying libssl.so.1.1 & libcrypto.so.1.1 as they are already available with openssl in distroless and copying them over causes FIPS HMAC verification failures
  * fix: update windows liveness timeoutSeconds, periodSeconds to 60 and reduce tasklist usage in liveness probe
  * toggle: toggle internal clusters for FIPS fix


## Release 01-09-2024
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-cfg`
* Change log -
  * Network Observability metrics update - https://github.com/Azure/prometheus-collector/pull/666
  * Windows powershell startup script bug fix - https://github.com/Azure/prometheus-collector/pull/694
  * Upgrade collector (0.90), collector-operator (0.90) and prometheus-operator (0.69.1)
  * Remove request values for windows ama-metrics daemonset (old behavior) - https://github.com/Azure/prometheus-collector/pull/707
  * Build and release improvements

## Release 11-16-2023
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.2-main-11-17-2023-19f08915-win
* Change log -
  * Fix Windows liveness probe for error level evaluation

## Release 11-03-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-cfg`
* Change log -
  * Add new regions for Azure Monitor Workspace - https://github.com/Azure/prometheus-collector/pull/609
  * Add telemetry for target allocator & config side-car image tags - https://github.com/Azure/prometheus-collector/pull/661
  * Add more metrics as default metrics (to enable Insights Portal Ux) - https://github.com/Azure/prometheus-collector/pull/667
    * kube-state-metrics
      * kube_service_info
      * kube_pod_container_status_running
      * kube_pod_container_status_waiting
      * kube_pod_container_status_terminated
      * kube_pod_container_state_started
      * kube_pod_created
      * kube_pod_start_time
      * kube_pod_init_container_info
      * kube_pod_init_container_status_terminated
      * kube_pod_init_container_status_terminated_reason
      * kube_pod_init_container_status_ready
      * kube_pod_init_container_resource_limits
      * kube_pod_init_container_status_running
      * kube_pod_init_container_status_waiting
      * kube_pod_init_container_status_restarts_total
    * node-exporter (Linux)
      * node_boot_time_seconds
  * Adding telemetry for ta and cfg reader img versions - https://github.com/Azure/prometheus-collector/pull/661

## Release 10-20-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-cfg`
* Change log -
  * Update telegraf jitter & disable exemplar for rs - https://github.com/Azure/prometheus-collector/pull/634
  * Add Operator support for prometheus-collector - https://github.com/Azure/prometheus-collector/pull/554

## Release 10-05-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.7-main-10-06-2023-b75a076c`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.7-main-10-06-2023-b75a076c-win`
* Change log -
  * Update k8s.io/clientgo in fluentbit plugin from `0.28.0` to `0.28.2` - https://github.com/Azure/prometheus-collector/pull/595
  * fix: ARC fixes (already released to ARC as hotfix) - https://github.com/Azure/prometheus-collector/pull/605
    * Update ARC regions (add Sweden South)
    * Fix registry for node exporter
    * Add `clusterDistribution` override parameter for AKS EE
  * Update CPU requests for Daemonset (linux & windows) - https://github.com/Azure/prometheus-collector/pull/606
  * Add telemetry for per target scrape interval - https://github.com/Azure/prometheus-collector/pull/614
  * Update dependencies, Disable exemplars on ME (Linux only), Update CVE exemptions, Stop windows ingestion from replicaset, Fix try scans to fail on CVEs - https://github.com/Azure/prometheus-collector/pull/616
    * Linux
      * mdsd = azure-mdsd-1.23.5 --> 1.27.4
      * ME = 2.2023.224.2214 --> 2.2023.928.2134
      * telegraf = 1.25.2 --> 1.27.3
      * golang = 1.18 --> 1.20
    * Windows
      * golang = 1.18 --> 1.20
    * Upgrade addon token adapter for back door deployments (Linux only)
      * master.221118.2 --> master.230804.1
  * Fix $ substitution issue in relabel and metric relabel config - https://github.com/Azure/prometheus-collector/pull/618
  * update github.com/prometheus/client_golang from `1.16.0` to `1.17.0` in fluentbit plugin - https://github.com/Azure/prometheus-collector/pull/608

## Release 9-11-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.5-main-09-12-2023-8fbde9ca`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.5-main-09-12-2023-8fbde9ca-win`
* Change log -
  * Add container image signing - https://github.com/Azure/prometheus-collector/pull/570
  * fix: windows liveness probe bug fix - https://github.com/Azure/prometheus-collector/pull/568
  * fix: proxy liveness probe timing fix - https://github.com/Azure/prometheus-collector/pull/591
  * update : add trouble shooting script - https://github.com/Azure/prometheus-collector/pull/572
  * Add following metrics from below targets to be collected by default when with ingestion profile - This is for future alerting improvements - https://github.com/Azure/prometheus-collector/pull/571
    * `job=Kubelet` - kubelet_certificate_manager_client_ttl_seconds, kubelet_certificate_manager_client_expiration_renew_errors, kubelet_server_expiration_renew_errors, kubelet_certificate_manager_server_ttl_seconds, kubelet_volume_stats_inodes_free, kube_persistentvolumeclaim_access_mode, kube_persistentvolumeclaim_labels, kube_persistentvolume_status_phase
    * `job=kube-state-metrics` - kube_daemonset_status_current_number_scheduled, kube_daemonset_status_number_misscheduled

## Release 08-11-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.4-main-08-11-2023-6de2ec55`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.4-main-08-11-2023-6de2ec55-win`
* Change log -
  * fix: revert commit `e7254187` to remove $$-->$ issue in config processing
  * fix: ARC extension : fixes for selectively not mounting for k3s & k8s edge distros (Edge distros)

## Release 07-28-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-07-28-2023-0efd3e4e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-07-28-2023-0efd3e4e-win`
* Change log -
  * fix: Add unfair semaphore wait to windows container for better initial CPU performance
  * fix:  Upgrades
      Node exporter - image from: `v1.5.0` to:`v1.6.0`; chart from:`4.14.0` to:`4.21.0`
      Kube state    - image from: `v2.8.1` to:`v2.9.2`; chart from:`4.32.0` to:`5.10.1`
  * Arc extension: do not mount ubuntu ca-certs if k8s distro is AKS Edge Essentials

## Release 06-26-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-06-26-2023-6ee07896`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-06-26-2023-6ee07896-win`
* Change log -
  * fix: Bicep template warnings
  * fix: Adding kube_*_labels and kube_*_annotations to the default list of metrics
  * fix: Bump github.com/prometheus/client_golang from 1.15.1 to 1.16.0 in /otelcollector/fluent-bit/src
  * fix: Bump k8s.io/apimachinery from 0.27.2 to 0.27.3 in /otelcollector/fluent-bit/src
  * fix: Bump k8s.io/client-go from 0.27.1 to 0.27.3 in /otelcollector/fluent-bit/src

## Release 06-02-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.1-main-06-02-2023-d384b035`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.1-main-06-02-2023-d384b035-win`
* Change log -
  * fix: Terraform template fixes for Azure Monitor Metrics addon
  * fix: Reduce image tag length to docker limit of 128 characters
  * fix: Various ARC release script fixes
  * fix: Bicep template fix for adding role assingment for new grafana instance + allow different RGs for Grafana and AMW

## Release 05-04-2023
* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.0-main-05-04-2023-4450ad10`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.0-main-05-04-2023-4450ad10-win`
* Change log -
  * feat: Add release and CI/CD support for Arc extension
  * fix:  Allowlist all metrics used in alerting
  * fix:  Update CPU and memory limits for Windows pods

## Release 04-25-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.6.0-main-04-25-2023-2eb2a81c`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.6.0-main-04-25-2023-2eb2a81c-win`
* Change log -
  * feat: Add priorityclass system node critical for RS, DS & KSM pods
  * fix:  Upgrades
          Fluent bit           - from: `v1.9.6` to:`v2.0.9`
          Telegraf(windows)    - from: `v1.23.4` to:`v1.24.2`
          Otelcol              - from:`v0.66.0` to:`v0.73.0`
  * fix:  pod annotations bug

## Release 03-24-2023

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
