# Azure Monitor Metrics for AKS clusters

## Release 09-29-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-cfg`
* AKS and Arc Container Images :
  + fix: increase tokenconfig download wait time to 45 sec for windows (<https://github.com/Azure/prometheus-collector/pull/1301>)
  + feat: Ability to update kube-state-metrics startup parameters using configmap (<https://github.com/Azure/prometheus-collector/pull/1292>)
  + fix: add missing data plane metrics(<https://github.com/Azure/prometheus-collector/pull/1293>)
  + chart: Match AKS RP chart for values and daemonset yaml (<https://github.com/Azure/prometheus-collector/pull/1298>)
  + build(deps): Upgrade otelcollector to v0.135.0 (<https://github.com/Azure/prometheus-collector/pull/1303>)
* Pipeline/Docs/Templates Updates:
  + fix: KSM on Arc, CI/CD retina version, release prod cluster deployment(<https://github.com/Azure/prometheus-collector/pull/1306>)
  + test: add two new error exclusions (<https://github.com/Azure/prometheus-collector/pull/1309>)
  + fix: Values context for Arc chart (<https://github.com/Azure/prometheus-collector/pull/1312>)

## Release 09-04-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.22.0-main-09-05-2025-e40947f3`

* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.22.0-main-09-05-2025-e40947f3-win`

* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.22.0-main-09-05-2025-e40947f3-targetallocator`

* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.22.0-main-09-05-2025-e40947f3-cfg`

* AKS and Arc Container Images :
  + fix: bleu cloud for linux and windows (<https://github.com/Azure/prometheus-collector/pull/1280>)
  + build(deps): Upgrade otelcollector to v0.132.0(<https://github.com/Azure/prometheus-collector/pull/1283>)
* Pipeline/Docs/Templates Updates:
  + infra: CI/CD and test fixes (<https://github.com/Azure/prometheus-collector/pull/1279>)

## Release 08-13-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:<tbd>-cfg`
* AKS and Arc Container Images:
  + feat: Add support for Container Storage's storage operator (<https://github.com/Azure/prometheus-collector/pull/1224>)
  + feat: Upgrade Pod and Service Monitor CRD (<https://github.com/Azure/prometheus-collector/pull/1223>)
  + feat: Upgrade otelcollector to v0.131.0 (<https://github.com/Azure/prometheus-collector/pull/1257>)
  + fix: Remove fatal error on fluent bit startup (<https://github.com/Azure/prometheus-collector/pull/1256>)
* Pipeline/Docs/Templates Updates:
  + fix: update recommended alert arm template to use clustername variable (<https://github.com/Azure/prometheus-collector/pull/1249>)
  + fix: improve upgrade bot (<https://github.com/Azure/prometheus-collector/pull/1225>)
  + feat: use managed sdp and region agnostic for release (<https://github.com/Azure/prometheus-collector/pull/1259>)

## Release 07-24-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.20.0-main-07-24-2025-756981f2`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.20.0-main-07-24-2025-756981f2-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.20.0-main-07-24-2025-756981f2-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.20.0-main-07-24-2025-756981f2-cfg`
* AKS and Arc Container Images:
  + feat: otlp support (<https://github.com/Azure/prometheus-collector/pull/1216>)
  + fix: handle utf-16 BOM (LE/BE) encoding for certificates in air gapped clouds (<https://github.com/Azure/prometheus-collector/pull/1244>)
  + fix: me upgrade+otlp+mariner3 fixes (<https://github.com/Azure/prometheus-collector/pull/1232>)
  + fix: update to allow partial configmap settings (<https://github.com/Azure/prometheus-collector/pull/1217>)
  + fix: for watching secret updates for CRs (<https://github.com/Azure/prometheus-collector/pull/1218>)
  + fix: sign prometheusui.exe, main.exe and remove telegraf.exe code (<https://github.com/Azure/prometheus-collector/pull/1213>)
  + fix: respect debug mode setting in configmap v1 (<https://github.com/Azure/prometheus-collector/pull/1191>)
  + fix: Remove secret create permissions (<https://github.com/Azure/prometheus-collector/pull/1190>)
* Pipeline/Docs/Templates Updates:
  + fix: remove uneeded terneray operator in tf template (<https://github.com/Azure/prometheus-collector/pull/1236>)
  + release: separate pipeline for arc release (<https://github.com/Azure/prometheus-collector/pull/1212>)
  + docs: update configmap to v1 as its pointed from docs (<https://github.com/Azure/prometheus-collector/pull/1188>)
  + build: add reference app to dependabot for OSS vulnerability remediation (<https://github.com/Azure/prometheus-collector/pull/1201>)
  + docs: add documentation for AMW limit upgrade api (<https://github.com/Azure/prometheus-collector/pull/1199>)
  + build: Add Github action for otelcollector upgrade bot (<https://github.com/Azure/prometheus-collector/pull/1197>)
  + build: account for patch version differences between TA and otelcollector (<https://github.com/Azure/prometheus-collector/pull/1206>)

## Release 07-10-2025 (CCP only release)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.19.0-main-07-10-2025-36d292f8`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.19.0-main-07-10-2025-36d292f8-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.19.0-main-07-10-2025-36d292f8-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.19.0-main-07-10-2025-36d292f8-cfg`
* AKS and Arc Container Images:
  + feat: Scrape metrics for node autoprovisioning (aks control plane) (<https://github.com/Azure/prometheus-collector/pull/1169>)

## Release 06-19-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.18.0-main-06-19-2025-TBD`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.18.0-main-06-19-2025-TBD-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.18.0-main-06-19-2025-TBD-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.18.0-main-06-19-2025-TBD-cfg`
* AKS and Arc Container Images:
  + feat: Update max shards to 24 (<https://github.com/Azure/prometheus-collector/pull/1167>)
  + feat: Secure communication between ReplicaSet and TargetAllocator (<https://github.com/Azure/prometheus-collector/pull/1133>)
  + fix: Respect minimal ingestion profile when no configmap is present (<https://github.com/Azure/prometheus-collector/pull/1184>)
  + fix: Set minimal ingestion environment variable with value (<https://github.com/Azure/prometheus-collector/pull/1185>)
  + feat: add support for bleu cloud (<https://github.com/Azure/prometheus-collector/pull/1150>)
  + feat: Upgrade OpenTelemetry Collector components from 0.123.0 to 0.127.0 (<https://github.com/Azure/prometheus-collector/pull/1182>)
  + fix: Update code for win support in sovereign clouds, ca cert bootstrap (<https://github.com/Azure/prometheus-collector/pull/1174>)

## Release 05-29-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.17.0-main-05-29-2025-1a3ab39b`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.17.0-main-05-29-2025-1a3ab39b-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.17.0-main-05-29-2025-1a3ab39b-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.17.0-main-05-29-2025-1a3ab39b-cfg`
* AKS and Arc Container Images:
  + feat: Configmap update for CCP (v2 + v1 schema support) (<https://github.com/Azure/prometheus-collector/pull/1056>)
  + feat: add support for bleu cloud (<https://github.com/Azure/prometheus-collector/pull/1150>)
  + feat: Upgrade OpenTelemetry Collector components from 0.121.0 to 0.123.0 (<https://github.com/Azure/prometheus-collector/pull/1139>)
  + fix: Update CA Cert volume mounts for Azure Linux (<https://github.com/Azure/prometheus-collector/pull/1132>)
  + fix: Downgrade Target Allocator to 0.120.0 to fix missing target when it's included in two different jobs (<https://github.com/Azure/prometheus-collector/pull/1163>)
  + fix: Very high CPU usage in OpenTelemetry Collector with openmetrics protocol (<https://github.com/Azure/prometheus-collector/pull/1162>)
  + fix: Empty scrape job in custom configmap (<https://github.com/Azure/prometheus-collector/pull/1161>)
* Pipeline/Docs/Templates Updates:
  + feat: enable ux recording rules arm, bicep, terraform, policy for aks and arc (<https://github.com/Azure/prometheus-collector/pull/1140>)
  + fix: move filesystemwatcher to hash based golang appoach for windows (<https://github.com/Azure/prometheus-collector/pull/1144>)
  + feat: move to 1ES build pipeline (<https://github.com/Azure/prometheus-collector/pull/1135>)
  + fix: KubePodReadyStateLow alert query (<https://github.com/Azure/prometheus-collector/pull/1141>)

## Release 04-15-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.16.0-main-04-15-2025-d78050c6`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.16.0-main-04-15-2025-d78050c6-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.16.0-main-04-15-2025-d78050c6-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.16.0-main-04-15-2025-d78050c6-cfg`
* AKS and Arc Container Images:
  + Update metrics extension (from: metricsext2-2.2024.419.1535 to:metricsext2-2.2025.123.2222 ) [applies tp widows, Linux & CCP images]
  + Deprecate windows metric `windows_system_system_up_time` and replace it with `windows_system_boot_time_timestamp_seconds` in all dashboards & rec.rules (arm, terraform, bicep, policy templates)
  + Scope ACSTOR discoveries to `acstor` namespace to aviod Target allocator discovering all pods across all namespaces in larger clusters
  + update version to 6.16.0
  + upgrade : upgrade otelcollector and targetallocator to v0.121.0 (<https://github.com/Azure/prometheus-collector/pull/1110>)
  + upgrade : KSM to 2.15 (<https://github.com/Azure/prometheus-collector/pull/1117>)
  + sync dashboards and few fixes for managed prometheus dashboards (<https://github.com/Azure/prometheus-collector/pull/1113/files>)
  + test : add tests for config (<https://github.com/Azure/prometheus-collector/pull/1114/files>)

* Pipeline/Docs/Templates Updates:
  + ci/cd: update remote write sidecar to write to eastus2 workspace (<https://github.com/Azure/prometheus-collector/pull/1090>)
  + release : new governed pipeline for release (<https://github.com/Azure/prometheus-collector/pull/1102>)
  + update esrp to use AME MSI ID (<https://github.com/Azure/prometheus-collector/pull/1101>)

## Release 02-21-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.15.0-main-02-21-2025-4acb2b4c`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.15.0-main-02-21-2025-4acb2b4c-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.15.0-main-02-21-2025-4acb2b4c-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.15.0-main-02-21-2025-4acb2b4c-cfg`
* AKS and Arc Container Images:
  + Add scrape_samples_scraped metric to telemetry when debug mode is enabled (<https://github.com/Azure/prometheus-collector/pull/1055>)
  + fix: set proxy the old way for mdsd in golang (<https://github.com/Azure/prometheus-collector/pull/1062>)
  + fix: add cluster scope to recording rules in policy (<https://github.com/Azure/prometheus-collector/pull/1064>)
  + upgrade: Upgrade otelcollector and targetallocator to 0.117.0 (<https://github.com/Azure/prometheus-collector/pull/1063>)

* Pipeline/Docs/Templates Updates:
  + fix: add cluster scope to recording rules in policy (<https://github.com/Azure/prometheus-collector/pull/1064>)
  + Add job labels to monitoring alerts (<https://github.com/Azure/prometheus-collector/pull/1065>)
  + fix: arc conformance build in pipeline (<https://github.com/Azure/prometheus-collector/pull/1066>)

## Release 01-16-2025

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.14.0-main-01-16-2025-8d52acfe`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.14.0-main-01-16-2025-8d52acfe-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.14.0-main-01-16-2025-8d52acfe-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.14.0-main-01-16-2025-8d52acfe-cfg`
* AKS and Arc Container Images:
  + Add support for global settings (<https://github.com/Azure/prometheus-collector/pull/1003>)
  + Sign flagged binaries for windows containers (<https://github.com/Azure/prometheus-collector/pull/1001>)
  + fix hubble & cilium regex-es for minimal ingestion profile (<https://github.com/Azure/prometheus-collector/pull/1017>)
  + Fix for CCP configmap processing issue (<https://github.com/Azure/prometheus-collector/pull/1023>)
  + fix keeplist in ccp, when minimalingestionprofile=false (<https://github.com/Azure/prometheus-collector/pull/1024>)
  + Fix dashboard links in Azure managed Prometheus dashboards (<https://github.com/Azure/prometheus-collector/pull/1025>)
  + Upgrade otelcollector and TA from 0.109 to 0.116 (<https://github.com/Azure/prometheus-collector/pull/1034>)
  + CCP scrape config + adding metrics_relabel_config to end of the config (<https://github.com/Azure/prometheus-collector/pull/1035>)
  + build: use new azure linux images for building the reference app (<https://github.com/Azure/prometheus-collector/pull/1036>)
  + Handle SIGTERM and exit when encountered restarting the pod (<https://github.com/Azure/prometheus-collector/pull/1037>)
  + Print fluent-bit and telegraf version for windows (<https://github.com/Azure/prometheus-collector/pull/1043>)
  + fix troubleshooting script for hpa and remove uneeded login check (<https://github.com/Azure/prometheus-collector/pull/1044>)
  + fix telegraf version print to be only for windows (linux already done) (<https://github.com/Azure/prometheus-collector/pull/1045>)
  + Log only log with level panic and above for TA (<https://github.com/Azure/prometheus-collector/pull/1046>)
* Arc Extension Chart:
  + Arc: configuration settings for GA (<https://github.com/Azure/prometheus-collector/pull/1016>)
* Pipeline/Docs/Templates Updates
  + doc: change example for pod annotations namespace regex filter (<https://github.com/Azure/prometheus-collector/pull/1042>)
  + docs: Add explicit step to check configmap applies for controlPlane to buildandrelease docs (<https://github.com/Azure/prometheus-collector/pull/1022>)
  + Update HPA doc to add min=max behavior (<https://github.com/Azure/prometheus-collector/pull/1009>)
  + doc: readme update for backdoor deployment (<https://github.com/Azure/prometheus-collector/pull/1006>)

## Release 12-05-2024 (hot-fix for ccp config map issue ) - CCP release only -

* CCP image -

* Changelog -
  + Fix for CCP Config map processing issue - (<https://github.com/Azure/prometheus-collector/pull/1017>)
  + Fix a bug where by with miminal ingestion profile is false, keep list wasn't effective - (<https://github.com/Azure/prometheus-collector/pull/1024>)

## Release 10-21-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.11.0-main-10-21-2024-91ec49e3`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.11.0-main-10-21-2024-91ec49e3-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.11.0-main-10-21-2024-91ec49e3-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.11.0-main-10-21-2024-91ec49e3-cfg`
* AKS and Arc Container Images:
  + Upgrades for CVE fixes (<https://github.com/Azure/prometheus-collector/pull/979>)
    - Golang: 1.21.5 -> 1.22.7
    - OtelCollector/Operator: 0.99.0 -> 0.109.0
    - Telegraf: 1.28.5 -> 1.29.4
  + Add AcStor scrape config support (<https://github.com/Azure/prometheus-collector/pull/976>)
* Arc Extension Chart:
  + Enable operator (<https://github.com/Azure/prometheus-collector/pull/977>)
  + Upgrade node-exporter chart 4.26.0 -> 4.39.0 (<https://github.com/Azure/prometheus-collector/pull/982>)
  + Arc-A: Add support to override image registry for custom environment (<https://github.com/Azure/prometheus-collector/pull/983>)
* Pipeline/Docs/Templates Updates
  + Pipeline reliability and test fixes (<https://github.com/Azure/prometheus-collector/pull/998>)
  + Terraform private link support (<https://github.com/Azure/prometheus-collector/pull/991>)
  + Doc Prometheus Equivalent metrics for CI Custom metrics (<https://github.com/Azure/prometheus-collector/pull/978>)

## Release 10-15-2024 (CCP release only)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.2-main-10-15-2024-06b20de5`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.2-main-10-15-2024-06b20de5-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.2-main-10-15-2024-06b20de5-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.2-main-10-15-2024-06b20de5-cfg`
* Change log -
* fix: Control Plane fixes (call proper executable in dockerfile, set env correctly) - <https://github.com/Azure/prometheus-collector/pull/993>
* feat : Onboard ACStor targets - <https://github.com/Azure/prometheus-collector/pull/976>

## Release 09-16-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.0-main-09-16-2024-85a71678`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.0-main-09-16-2024-85a71678-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.0-main-09-16-2024-85a71678-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.10.0-main-09-16-2024-85a71678-cfg`
* Change log -
* feat: Changes to enable HPA for ama-metrics deployment - <https://github.com/Azure/prometheus-collector/pull/968>

## Release 08-28-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.1-main-08-28-2024-f33aacb5`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.1-main-08-28-2024-f33aacb5-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.1-main-08-28-2024-f33aacb5-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.1-main-08-28-2024-f33aacb5-cfg`
* Change log -
* fix: ama-metrics-operator-targets Config Reader sidecar memory leak fix - <https://github.com/Azure/prometheus-collector/pull/962>
* fix: Adding back log mount for tailing logs(telemetry fix) - <https://github.com/Azure/prometheus-collector/pull/966>

## Release 07-22-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.0-main-07-22-2024-2e3dfb56`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.0-main-07-22-2024-2e3dfb56-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.0-main-07-22-2024-2e3dfb56-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.9.0-main-07-22-2024-2e3dfb56-cfg`
* Change log -
* fix: Truncate DCE/DCR to 44/64 chars in ARM, Bicep, Terraform... - <https://github.com/Azure/prometheus-collector/pull/922>
* fix: add single quotes to pod annotations for multiple namespace scenario - <https://github.com/Azure/prometheus-collector/pull/927>
* feat: Upgrade components to 0.99 and use golang for config processing - <https://github.com/Azure/prometheus-collector/pull/891>
* fix: move to single quote for telegraf - <https://github.com/Azure/prometheus-collector/pull/946>
* compliance: add codeql to build pipeline - <https://github.com/Azure/prometheus-collector/pull/939>
* Updating deployment specs for ama-metrics-operator-targets as this pod was not getting cleaned up by GC on low memory evictions due to memory pressure - <https://github.com/Azure/prometheus-collector/pull/931>
* Test: unit tests and some small fixes for configmap processing - <https://github.com/Azure/prometheus-collector/pull/930>

## Release 06-12-2024

* Target Allocator Chart update to add Tls secret to the config reader sidecar container  

## Release 06-10-2024 (CCP release only)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.14-main-06-10-2024-b20600b3`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.14-main-06-10-2024-b20600b3-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.14-main-06-10-2024-b20600b3-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.14-main-06-10-2024-b20600b3-cfg`
* Change log -
  + fix: honor minimal ingestion profile setting for ccp - <https://github.com/Azure/prometheus-collector/pull/911>

## Release 05-29-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.13-main-05-29-2024-3e72c0e4-cfg`
* Change log -
  + fix: update kube-state-metrics from: `v2.9.2` to: `v2.12.0` - <https://github.com/Azure/prometheus-collector/pull/887>
  + fix: switch to Managed Identity for ARC release - <https://github.com/Azure/prometheus-collector/pull/895>
  + fix: move PV metrics to correct job (from kubelet to k-s-m) - <https://github.com/Azure/prometheus-collector/pull/898>
    - `kube_persistentvolumeclaim_access_mode`
    - `kube_persistentvolumeclaim_labels`
    - `kube_persistentvolume_status_phase`
  + fix: signature artifacts drop issue - <https://github.com/Azure/prometheus-collector/pull/885/files>
  + fix: revert Telegraf removal (i.e revert PRs #766 & #841) - <https://github.com/Azure/prometheus-collector/pull/899>

## Release 05-20-2024 (CCP release only)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.12-main-05-21-2024-56bc7e3d-cfg`
* Change log -
  + fix: ccp ignore minimal ingestion profile setting and respecting the keep list regex values - <https://github.com/Azure/prometheus-collector/pull/886>
  + fix: signature artifacts drop issue - <https://github.com/Azure/prometheus-collector/pull/885/files>
  + fix: Remove histograms from minimal ingestion list - ccp metrics collector - <https://github.com/Azure/prometheus-collector/pull/884>

## Release 05-07-2024 (CCP release only)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.11-main-6.8.11-main-05-07-2024-fcfa51bd-cfg`
* Change log -
  + fix: ccp metrics missing cluster value

## Release 05-03-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.10-main-05-06-2024-079dabbc-cfg`
* Change log -
  + fix: update to use older proxy setup for mdsd in aks - <https://github.com/Azure/prometheus-collector/pull/864>
  + add remaining sdl scans similar to onebranch default - <https://github.com/Azure/prometheus-collector/pull/858>
  + Add life cycle metadata to container image - <https://github.com/Azure/prometheus-collector/pull/842>
  + Migrate to ESRP version 5 from version 3
  + [fix] Fix the fluent bit error when collector health is enabled - <https://github.com/Azure/prometheus-collector/pull/841>
  + Test
    - add Windows tests for Prometheus Target UX - <https://github.com/Azure/prometheus-collector/pull/839>
    - small fixes to CRs and running tests - <https://github.com/Azure/prometheus-collector/pull/835>
  + Various build fixes
    - <https://github.com/Azure/prometheus-collector/pull/834>
    - <https://github.com/Azure/prometheus-collector/pull/831>
    - <https://github.com/Azure/prometheus-collector/pull/827>
  + fix (ccp) : Relabel host for Control Plane Metrics - (<https://github.com/Azure/prometheus-collector/pull/833>)
  + Adding filter strategy - <https://github.com/Azure/prometheus-collector/pull/832/files>
  + fix: set hubble minimal ingestion profile - <https://github.com/Azure/prometheus-collector/pull/829>
  + [fix] Minor fix in onboarding templates - <https://github.com/Azure/prometheus-collector/pull/828>
  + Remove telegraf for telemetry and only use fluent-bit
  + perf: add namespace selector to default jobs to improve perf - <https://github.com/Azure/prometheus-collector/pull/867>
  + set hubble minimal ingestion profile - <https://github.com/Azure/prometheus-collector/pull/860>
  + Upgrade Metrics Extension (Linux & windows) from metricsext2-2.2024.328.1744 --> metricsext2-2.2024.419.1535 (This fixes the HDInsights bug (OOM) on flint clusters)

## Release 04-08-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.7-main-04-09-2024-82adbf97-cfg`
* Change log -
  + fix: Fix: For Arc, use a default value CloudEnvironment that customers can change for other clouds- <https://github.com/Azure/prometheus-collector/pull/753>
  + Upgrade: fluent-bit from 2.0.9 to 2.1.10 - <https://github.com/Azure/prometheus-collector/pull/809>
  + Update mdsd, MA, MetricsExtension
    - Linux
      * mdsd 1.27.4 --> 1.30.3
      * MetricsExtension 2.2023.928.2134 --> 2.2024.328.1744
    - Windows
      * MA 46.4.1 --> 46.15.4
      * MetricsExtension 2.2023.224.2214 -> 2.2024.328.1744
  + Upgrading telegraf from 1.27.3 to 1.28.5 for linux
  + fix: Change logging from error to info for missing configmap settings to not have errors for older configmaps - <https://github.com/Azure/prometheus-collector/pull/804>
  + feat : add support for per cloud AI instance - <https://github.com/Azure/prometheus-collector/pull/798>
  + Step 0 : Merge CCP changes to main with a separate image - <https://github.com/Azure/prometheus-collector/pull/653>

## Release 03-08-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.6-main-03-08-2024-fd4f13cb-cfg`
* Change log -
  + fix: Fix for yaml unmarshal bug for keepequal/dropequal - <https://github.com/Azure/prometheus-collector/pull/753>
  + fix: dollar fix for TA - <https://github.com/Azure/prometheus-collector/pull/769>
  + ENable operator with CRD support globally - (This will be rolled out once the image roll out is complete)
  + Add new regions for Azure Monitor Workspace ('brazilsoutheast', 'francecentral', 'ukwest', 'koreasouth', 'switzerlandwest', 'japaneast', 'swedencentral', 'canadaeast', 'norwaywest', 'southindia', 'australiaeast', 'swedensouth')

## Release 02-14-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.4-main-02-14-2024-90d01292-cfg`
* Change log -
  + feat: add ccp config map settings for public preview
  + feat: Enable MTLS authentication
  + fix: add some metrics for civ2 ux
  + fix: Add telemetry for collector and addon token adaptor
  + fix: Set autoresolve to true for new agent version alert
  + fix: SDL Requirment : add policheck
  + fix: [infra] Fix commented out ARC deploy chart condition
  + fix: stop copying libssl.so.1.1 & libcrypto.so.1.1 as they are already available with openssl in distroless and copying them over causes FIPS HMAC verification failures
  + fix: update windows liveness timeoutSeconds, periodSeconds to 60 and reduce tasklist usage in liveness probe
  + toggle: toggle internal clusters for FIPS fix

## Release 01-09-2024

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.3-main-01-09-2024-a192d342-cfg`
* Change log -
  + Network Observability metrics update - <https://github.com/Azure/prometheus-collector/pull/666>
  + Windows powershell startup script bug fix - <https://github.com/Azure/prometheus-collector/pull/694>
  + Upgrade collector (0.90), collector-operator (0.90) and prometheus-operator (0.69.1)
  + Remove request values for windows ama-metrics daemonset (old behavior) - <https://github.com/Azure/prometheus-collector/pull/707>
  + Build and release improvements

## Release 11-16-2023

* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.2-main-11-17-2023-19f08915-win
* Change log -
  + Fix Windows liveness probe for error level evaluation

## Release 11-03-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.1-main-11-03-2023-c061beb4-cfg`
* Change log -
  + Add new regions for Azure Monitor Workspace - <https://github.com/Azure/prometheus-collector/pull/609>
  + Add telemetry for target allocator & config side-car image tags - <https://github.com/Azure/prometheus-collector/pull/661>
  + Add more metrics as default metrics (to enable Insights Portal Ux) - <https://github.com/Azure/prometheus-collector/pull/667>
    - kube-state-metrics
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
    - node-exporter (Linux)
      * node_boot_time_seconds
  + Adding telemetry for ta and cfg reader img versions - <https://github.com/Azure/prometheus-collector/pull/661>

## Release 10-20-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-win`
* TA image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-targetallocator`
* cfg sidecar image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.8.0-main-10-20-2023-182f67d4-cfg`
* Change log -
  + Update telegraf jitter & disable exemplar for rs - <https://github.com/Azure/prometheus-collector/pull/634>
  + Add Operator support for prometheus-collector - <https://github.com/Azure/prometheus-collector/pull/554>

## Release 10-05-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.7-main-10-06-2023-b75a076c`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.7-main-10-06-2023-b75a076c-win`
* Change log -
  + Update k8s.io/clientgo in fluentbit plugin from `0.28.0` to `0.28.2` - <https://github.com/Azure/prometheus-collector/pull/595>
  + fix: ARC fixes (already released to ARC as hotfix) - <https://github.com/Azure/prometheus-collector/pull/605>
    - Update ARC regions (add Sweden South)
    - Fix registry for node exporter
    - Add `clusterDistribution` override parameter for AKS EE
  + Update CPU requests for Daemonset (linux & windows) - <https://github.com/Azure/prometheus-collector/pull/606>
  + Add telemetry for per target scrape interval - <https://github.com/Azure/prometheus-collector/pull/614>
  + Update dependencies, Disable exemplars on ME (Linux only), Update CVE exemptions, Stop windows ingestion from replicaset, Fix try scans to fail on CVEs - <https://github.com/Azure/prometheus-collector/pull/616>
    - Linux
      * mdsd = azure-mdsd-1.23.5 --> 1.27.4
      * ME = 2.2023.224.2214 --> 2.2023.928.2134
      * telegraf = 1.25.2 --> 1.27.3
      * golang = 1.18 --> 1.20
    - Windows
      * golang = 1.18 --> 1.20
    - Upgrade addon token adapter for back door deployments (Linux only)
      * master.221118.2 --> master.230804.1
  + Fix $ substitution issue in relabel and metric relabel config - <https://github.com/Azure/prometheus-collector/pull/618>
  + update github.com/prometheus/client_golang from `1.16.0` to `1.17.0` in fluentbit plugin - <https://github.com/Azure/prometheus-collector/pull/608>

## Release 9-11-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.5-main-09-12-2023-8fbde9ca`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.5-main-09-12-2023-8fbde9ca-win`
* Change log -
  + Add container image signing - <https://github.com/Azure/prometheus-collector/pull/570>
  + fix: windows liveness probe bug fix - <https://github.com/Azure/prometheus-collector/pull/568>
  + fix: proxy liveness probe timing fix - <https://github.com/Azure/prometheus-collector/pull/591>
  + update : add trouble shooting script - <https://github.com/Azure/prometheus-collector/pull/572>
  + Add following metrics from below targets to be collected by default when with ingestion profile - This is for future alerting improvements - <https://github.com/Azure/prometheus-collector/pull/571>
    - `job=Kubelet` - kubelet_certificate_manager_client_ttl_seconds, kubelet_certificate_manager_client_expiration_renew_errors, kubelet_server_expiration_renew_errors, kubelet_certificate_manager_server_ttl_seconds, kubelet_volume_stats_inodes_free, kube_persistentvolumeclaim_access_mode, kube_persistentvolumeclaim_labels, kube_persistentvolume_status_phase
    - `job=kube-state-metrics` - kube_daemonset_status_current_number_scheduled, kube_daemonset_status_number_misscheduled

## Release 08-11-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.4-main-08-11-2023-6de2ec55`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.4-main-08-11-2023-6de2ec55-win`
* Change log -
  + fix: revert commit `e7254187` to remove $$-->$ issue in config processing
  + fix: ARC extension : fixes for selectively not mounting for k3s & k8s edge distros (Edge distros)

## Release 07-28-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-07-28-2023-0efd3e4e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-07-28-2023-0efd3e4e-win`
* Change log -
  + fix: Add unfair semaphore wait to windows container for better initial CPU performance
  + fix:  Upgrades
      Node exporter - image from: `v1.5.0` to: `v1.6.0` ; chart from: `4.14.0` to: `4.21.0`

      Kube state    - image from: `v2.8.1` to: `v2.9.2` ; chart from: `4.32.0` to: `5.10.1`

  + Arc extension: do not mount ubuntu ca-certs if k8s distro is AKS Edge Essentials

## Release 06-26-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-06-26-2023-6ee07896`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.2-main-06-26-2023-6ee07896-win`
* Change log -
  + fix: Bicep template warnings
  + fix: Adding kube_**labels and kube**_annotations to the default list of metrics
  + fix: Bump github.com/prometheus/client_golang from 1.15.1 to 1.16.0 in /otelcollector/fluent-bit/src
  + fix: Bump k8s.io/apimachinery from 0.27.2 to 0.27.3 in /otelcollector/fluent-bit/src
  + fix: Bump k8s.io/client-go from 0.27.1 to 0.27.3 in /otelcollector/fluent-bit/src

## Release 06-02-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.1-main-06-02-2023-d384b035`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.1-main-06-02-2023-d384b035-win`
* Change log -
  + fix: Terraform template fixes for Azure Monitor Metrics addon
  + fix: Reduce image tag length to docker limit of 128 characters
  + fix: Various ARC release script fixes
  + fix: Bicep template fix for adding role assingment for new grafana instance + allow different RGs for Grafana and AMW

## Release 05-04-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.0-main-05-04-2023-4450ad10`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.7.0-main-05-04-2023-4450ad10-win`
* Change log -
  + feat: Add release and CI/CD support for Arc extension
  + fix:  Allowlist all metrics used in alerting
  + fix:  Update CPU and memory limits for Windows pods

## Release 04-25-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.6.0-main-04-25-2023-2eb2a81c`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.6.0-main-04-25-2023-2eb2a81c-win`
* Change log -
  + feat: Add priorityclass system node critical for RS, DS & KSM pods
  + fix:  Upgrades
          Fluent bit           - from: `v1.9.6` to: `v2.0.9`

          Telegraf(windows)    - from: `v1.23.4` to: `v1.24.2`

          Otelcol              - from: `v0.66.0` to: `v0.73.0`

  + fix:  pod annotations bug

## Release 03-24-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.5.0-main-03-24-2023-7eb3f5c7`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.5.0-main-03-24-2023-7eb3f5c7-win`
* Change log -
  + feat: Support for ARC-A
  + fix:  Match ME setings between DS & RS
  + feat: Simplify Pod annotation based scraping by adding it as a target in the configmap
  + fix:  Add golang race detector during builds (SDL requirement)
  + fix:  Reduce telemetry volume
  + feat: Make deployment progress deadline configurable as a param (controbution from @OriYosefiMSFT)
  + feat: Enable workload identity for valur store (1p only) (contribution from @lnr0626)
  + fix:  Bump client-go and lumberjack.v2 packages for fluentbit
  + fix:  Upgrades
          Node exporter - image from: `v1.3.1` to: `v1.5.0` ; chart from: `3.1.1` to: `4.14.0`

          Kube state    - image from: `v2.6.0` to: `v2.8.1` ; chart from: `4.23.0` to `4.32.0`

          ME            - from: `2.2022.1201.1140` to: `2.2023.224.2214`

          MDSD          - from: `1.23.4` to: `1.23.5`

          MA            - from: `46.2.3` to: `46.4.1`

          Telegraf(linux) - from `1.23.0` to `1.25.2`

  + fix: CVEs (many)

## Release 02-22-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.4.0-main-02-22-2023-3ee44b9e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.4.0-main-02-22-2023-3ee44b9e-win`
* Change log -
  + feat: Allow setting a priority class for the daemonset and deployment objects
  + fix:  Truncate the tag to 128 characters (docker requirement)
  + fix:  Bump github.com/prometheus/client_golang from 1.9.0 to 1.11.1 in /internal/referenceapp/golang
  + feat: HTTP Proxy for distroless and ARC
  + fix:  Wait for token adapter to be healthy before starting dependencies
  + feat: Add azure policy templates for metrics addon
  + feat: enable network monitoring metrics (kappie)
  + feat: AKS addon HTTP Proxy Support
  + fix:  certificate import for windows ME startup

## Release 01-31-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.2.0-main-01-31-2023-e1e3858b`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.2.0-main-01-31-2023-e1e3858b-win`
* Change log -
  + Adding Bicep template to monitoring addon
  + Added custom DCR and DCE arm templates for Remote Write
  + Adding monitoring reader role to Azure Monitor Workspace in ARM and Bicep templates
  + Fix fluent-bit daemonset tailing path and mariner docs
  + Liveness probe update for NON-MAC mode (windows)
  + Adds windows daemonset support with MSI (only in deprecated chart mode)

## Release 01-11-2023

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.1.0-main-01-11-2023-5bf41607`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.1.0-main-01-11-2023-5bf41607-win`
* Change log -
  + Upgrade otel (from 0.58 to 0.66)
  + Upgrade ME (from 2.2022.1021.1309 to 2.2022.1201.1140)
  + Upgrade mdsd (from azure-mdsd_1.19.3-build.master.428 to azure-mdsd_1.23.4-build.master.28)

## Release 12-14-2022 (This version is being released only internally due to deployment freeze during holidays)

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.2-main-12-14-2022-e0364da3-win`
* Change Log -
  + Update addon token adapter (from master.220916.1 to master.221118.2)
  + Enable non-default dashboards & their recording rules (apiserver, kube-proxy, kubedns and kubernetes*)
  + Fix for excluding windows nodes in the node dropdown for k8s computer (nodes) dashboard

## Release 11-29-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.1-main-11-29-2022-97e2122e`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:6.0.1-main-11-29-2022-97e2122e-win`
* Change Log -
  + Move to mariner base for Linux image
  + Enable ARM64 support (for addon based datacollection only) - Includes both Daemonset & Replicaset
  + Update Kube-state-metrics chart (from 4.18.0 to 4.23.0) [chart only upgrade]
  + Update Prometheus node exporter chart (from 3.1.1 to 4.5.2) and image (from 1.3.1 to 1.4.0) [Remove selector label changes in 1.4.x chart that breaks upgrade]

## Release 10-27-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.4.0-main-10-26-2022-16f02b39`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.4.0-main-10-26-2022-16f02b39-win`
* Change Log -
  + Release custom prometheus config global settings to apply to the default targets in AKS-Addon
  + Rebuild with latest base image for security patches

## Release 10-06-22

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.3.0-main-10-06-2022-c0c49872`
* Change Log -
  + Add capability for the custom prometheus config global settings to apply to the default targets
  + Bug fix - Rollback from otelcollector version 0.59.0 to 0.58.0 due to external labels bug
  + Bug fix - Fix race condition for internal production build

## Release 09-30-2022

* Linux image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.2.0-main-09-29-2022-ca064de1`
* Windows image - `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/prometheus-collector/images:5.2.0-main-09-29-2022-ca064de1-win`
* Change Log -
  + Public preview release image for Azure Monitor Metrics on AKS clusters
  + Add NOTICE file for OSS code and Component Governance generated notice for container
  + Bug fix - Add missing region dimension for all telemetry collected thru telegraf
  + Bug fix - Fix memory usage alert which hits multiple matches for labels issue
  + Bug fix - Remove virtual node core capacity from telemetry total
  + Bug fix - Update alert group names for default and CI alerts
  + Bug fix - Update prometheus custom config for Azure Monitor Metrics Addon
