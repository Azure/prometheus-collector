# Troubleshooting Guides (TSGs)

Symptom-specific troubleshooting workflows for Azure Managed Prometheus.

TSG source: <https://dev.azure.com/msazure/InfrastructureInsights/_wiki/wikis/InfrastructureInsights.wiki?pagePath=/ManagedPrometheus/OnCall/TSGs>

---

#### TSG: Pod Restarts and OOMKills

Run `tsg_errors` and `tsg_workload`. Then:

**ama-metrics ReplicaSet:**
1. Check if restarts are due to **authentication/connectivity issues** — run `tsg_errors`, look for `DCR/DCE/AMCS Configuration Errors`, `Liveness Probe Logs` with "No configuration present". Also run `tsg_logs` and check for repeated `TokenConfig.json does not exist`. If present, this is the **firewall/blocked endpoints** pattern — see TSG: Firewall / Network Connectivity below
2. Check if restarts are due to **OOMKilled** — run `tsg_workload`, check P95 CPU/Memory. If OtelCollector + MetricsExtension CPU/Memory is near container limits, pods are resource-starved
3. **Check system pool VM size** — run `tsg_triage`, look at "Node Pool Capacity" for the **System** mode pool. Note the `vmSize` (e.g., Standard_E4s_v5 = 32GB). ReplicaSet pods run exclusively on system pool nodes as a managed addon. Small system pool VMs are the most common cause of OOMKill with high metric volumes
4. **Check HPA status** — run `tsg_workload`, check "HPA Status" for `currentReplicas`, `maxReplicas`, and `atLimit` flag. The HPA automatically scales ReplicaSet pods to handle high metric volumes. If `atLimit == true`, HPA cannot scale further. Max is adjustable up to 30 via `ama-metrics-settings-configmap` → `minshards`
5. **Calculate if system pool can fit HPA replicas** — each ReplicaSet pod has a 14Gi memory limit (check "Pod Resource Limits" to confirm). Calculate: system pool nodes × node memory ÷ 14Gi = max pods. If HPA wants more replicas than the system pool can fit, pods will OOMKill. Example: 4 nodes × Standard_E4s_v5 (32GB) = 128GB → ~9 pods max at 14Gi each
6. **Check pod-to-node placement** — run `tsg_pods`, check "Pod to Node Mapping" and "System Pool Node Resources". Verify ReplicaSet pods are distributed across system pool nodes and that nodes aren't under MemoryPressure
7. **Check metric volume** — run `tsg_metric_insights`. If Istio/Envoy histogram `_bucket` metrics dominate (common: 50-90% of total volume), recommend dropping them via `metric_relabel_configs`. This is the most impactful mitigation
8. **Check pod resource limits** — run `tsg_workload`, check "Pod Resource Limits". ReplicaSet default: 500Mi req / 14Gi limit memory, 150m req / 7 CPU limit
9. **Check scrape interval** — aggressive intervals (e.g. 1s) in `ama-metrics-prometheus-config` configmap cause excessive load
10. **Check for double collection** — customer may have `podannotationnamespaceregex` set in `ama-metrics-settings-configmap` AND custom jobs scraping the same pod annotations
11. **Check relabelings** — ensure customer is using `relabel_configs` and `metric_relabel_configs` to scope scraping
12. **Resolution summary for OOMKills:**
    - **If system pool VMs are small (≤32GB)** → upgrade to larger VM size (Standard_E8s_v5 or larger)
    - **If metric volume is very high (>5M daily TS)** → reduce volume via `metric_relabel_configs` (drop `_bucket` histograms, reduce label cardinality)
    - **If HPA is at limit** → increase `minshards` in settings configmap (up to 30), but ONLY if system pool can accommodate more pods
    - **If system pool is at max nodes** → increase `maxCount` for the system pool autoscaler

**ama-metrics-node DaemonSet (OOM is uncommon but has a specific root cause pattern):**
1. Check for aggressive scrape interval in `ama-metrics-prometheus-config-node`
2. Check if **Advanced Network Observability** is enabled — this can cause high memory usage. Mitigation: increase memory limits via AKS RP toggle
3. **Most common DaemonSet OOM cause: wrong configmap.** Check if the customer put cluster-wide scrape jobs in `ama-metrics-prometheus-config-node` instead of `ama-metrics-prometheus-config`. The node configmap (`-node` suffix) runs on every DaemonSet pod, so cluster-wide targets get scraped N times (once per node) instead of once. This causes massive duplication and OOMKills. **Fix:** move cluster-wide jobs to `ama-metrics-prometheus-config` (ReplicaSet configmap), keep only node-local targets (e.g. kubelet, node-exporter) in the `-node` configmap
4. Check `tsg_config` → look at "Configmaps", "Scrape Configs", and "Custom Scrape Jobs from Startup Logs" to see what jobs are in each configmap. The startup logs query shows which jobs were loaded at pod startup — if DaemonSet shows cluster-wide jobs like `kubernetes-pods` or `kube-state-metrics`, that confirms the wrong-configmap pattern. **Note:** startup logs only appear if pods restarted within the timeRange — use `timeRange='30d'` if needed
5. If DaemonSet pods are OOMing but ReplicaSet pods are healthy, the wrong-configmap pattern is almost certainly the cause

**ama-metrics-operator-targets:**
- Rare. Check if service discovery is not scoped to specific namespaces (e.g. kube-api-server endpoints should be scoped to `default` namespace)

---

#### TSG: Missing Metrics

Run `tsg_triage`, `tsg_config`, `tsg_workload`. Then:

1. **Check addon is enabled** — run `tsg_config`, check "Addon Enabled in AKS Profile". If `metricsEnabled == false`, the monitoring addon isn't enabled. Customer needs `az aks update --enable-azure-monitor-metrics`
2. **Check for OOMKill / pod crashes first** — missing metrics are often a SYMPTOM of pod OOMKills. If App Insights queries return "path does not exist", the addon is crash-looping and not sending telemetry at all. Go directly to `tsg_pods` for pod restart analysis and follow the Pod Restarts TSG below
3. **Check if the metric name exists in the account** — run `tsg_metric_insights` with the `MDMAccountName` from triage. The "View All Metric Names" panel returns every metric name ever ingested (180-day lookback). If the missing metric is NOT in this list, it was never successfully ingested — focus on scrape config, keep list regex, or config validation errors. If it IS in the list, the metric was ingested at some point — the issue may be throttling, intermittent scrape failures, or dimension fragmentation (see step 4)
4. **Check AMW quota and MDM throttling** — this is the most common cause of missing metrics at scale:
   - Run `tsg_triage` → extract `MDMAccountName` from "MDM Account ID" result
   - Run `tsg_mdm_throttling` with that `monitoringAccount` to check for throttling
   - If `ThrottledClientMetricCount > 0` → incoming events are being rejected by Geneva. Customer is hitting their account ingestion rate limit
   - If `ThrottledTimeSeriesCount > 0` → MStore is throttling time series. Customer has too many unique metric+label combinations
   - If `DroppedClientMetricCount > 0` → events are being dropped before ingestion
   - If `MStoreDroppedSamplesCount > 0` → samples are being lost in MStore
   - If event volume utilization is > 80% → approaching limit, will start throttling soon
   - If time series utilization is > 80% → approaching limit, need to reduce cardinality
   - **Resolution for throttling**: escalate to `Geneva Monitoring/MDM-Support-Manageability-Tier2` for quota increase, or help customer reduce cardinality via `metric_relabel_configs`
4. **Check ME ingestion success rate** — run `tsg_workload`, check "ME Ingestion Success Rate". If `successRate < 99%`, ME is dropping significant metrics. Cross-reference with ME queue sizes and drops
5. **Check auth issues** — look for `DCR/DCE/AMCS Configuration Errors`, `Liveness Probe Logs` with "No configuration present", `MDSD Errors`, `MetricsExtension Errors`. If `tsg_logs` shows repeated `TokenConfig.json does not exist`, this is the firewall/blocked endpoints pattern — see TSG: Firewall / Network Connectivity. If errors mention "private link is needed", also see that TSG
6. **Check CPU/memory** — if resources are very high, pods may be overwhelmed. Check if samples per minute per ReplicaSet exceed ~3.5 million
7. **Check ME queue/drops** — run `tsg_workload`, look at `ReplicaSet Samples Dropped` and queue sizes. If growing, need HPA or more shards
8. **Default metrics missing** — run `tsg_config`, check if default scrape config is enabled and metric is in `Default Targets KeepListRegex`. Customer may need to add metric to keep list
9. **Custom metrics missing** — run `tsg_config` and check these queries in order:
   - **"Invalid Custom Prometheus Config"** — if `true`, the customer's configmap has errors. Check **"Custom Config Validation Errors"** for the specific error (common: `not a valid duration string: "30"` — missing unit suffix; `found multiple scrape configs with job name` — duplicate job names; `unsupported features: rule_files` — rule_files not supported in configmap)
   - **"Custom Config YAML Error Lines"** — when the error is `yaml: unmarshal errors`, this query extracts every individual line number and field error (e.g. `line 136: field action not found in type kubernetes.plain`). This pinpoints exactly where the YAML is malformed — typically `metric_relabel_configs` fields (`action`, `regex`, `source_labels`, `target_label`, `replacement`) placed at the wrong indentation level inside `kubernetes_sd_configs`
   - **"Custom Config OTel Loading Errors"** — captures the full OTel collector configuration loading error chain (`Cannot load configuration: cannot unmarshal the configuration: decoding failed...`). This shows the higher-level error when the Prometheus config can't be loaded into the OTel receiver, complementing the line-level errors above
   - **"Custom Config Validation Status"** — shows per-pod whether config was loaded (`OK`), rejected (`INVALID`), or absent (`NO_CUSTOM_CONFIG`). If DaemonSet shows `NO_CUSTOM_CONFIG` but ReplicaSet shows `OK`, that's expected (DaemonSet uses separate `-node` configmap)
   - **"ReplicaSet ConfigMap Jobs"** and **"Custom Scrape Jobs from Startup Logs"** — verify the customer's job names appear. The startup logs query uses a wider time window since it only captures pod restarts. If empty, retry with `timeRange='30d'`
   - Check job also appears in PodMonitors or ServiceMonitors if using operator-based discovery
10. **Recording rule metrics missing** — run `tsg_config`, check "Recording Rules Configured" to confirm rules exist. Check scrape frequency vs recording rules evaluation interval (e.g. 1m rule interval with 2m scrape interval causes gaps). Transfer to `Azure Log Search Alerts/Prometheus Alerts` if needed
11. **Target distribution imbalance** — run `tsg_workload`, check "Target Allocator Distribution". If `targets_per_collector` is uneven or very high, some collectors may be overloaded. Check "Exporter Send Failures" for `send_failed_metric_points > 0` — indicates ME/MDM ingestion failures
12. **Check event timeline** — run `tsg_workload`, check "Event Timeline" to correlate config changes, restarts, and error spikes. Look for patterns like "config change → error spike → restart"
13. **Multi-AMW routing** — if the cluster has multiple AMWs associated (check `tsg_triage` → "AMW Configuration"):
    - All scrape jobs route to the **default AMW** unless explicitly configured otherwise
    - To send metrics to a non-default AMW, the customer must set `metricsAccountName` on the scrape job config or PodMonitor/ServiceMonitor
    - Cross-subscription ingestion IS supported without additional RBAC configuration — the DCRA handles auth automatically
    - If customer says metrics are missing, first ask **which AMW** they are querying. If they're checking a non-default AMW but haven't configured `metricsAccountName` routing, that is the root cause
    - Example PodMonitor annotation: `prometheus.io/metricsAccountName: <amw-name>`
    - Example scrape config: add `metricsAccountName: <amw-name>` under the job definition
14. **Pod restarts causing gaps** — see Pod Restarts TSG above

**Ask customer to check Prometheus UI:**
- `kubectl port-forward <ama-metrics-pod> 9090` then check `/config` (scrape config present?), `/targets` (targets up?)
- If target is down: error message has details. If `node-exporter` down → transfer to AKS team. If `kube-state-metrics` down → check `ama-metrics-ksm` pod logs

---

#### TSG: Spike in Metrics Ingested

Run `tsg_workload`, `tsg_config`, `tsg_mdm_throttling`, and `tsg_metric_insights`. Then:

1. **Check MDM throttling first** — extract `MDMAccountName` from `tsg_triage`, then run `tsg_mdm_throttling`. If event volume or time series utilization is > 80%, the spike may be causing throttling and metric loss
2. **Identify top offending metrics** — run `tsg_metric_insights` with the same `MDMAccountName`. Check "Top 20 Highest Cardinality Metrics" and "Metrics with High Dimension Cardinality" to find which metrics/jobs are causing the spike
3. Customer can run PromQL: `sum_over_time(scrape_samples_post_metric_relabeling) by (job)` to see if new jobs were added or existing jobs increased
4. Most common cause: **Network Observability** metrics increase with cluster traffic
5. **Reduction options:**
   - Default metrics: use `ama-metrics-settings-configmap` to change targets, metrics, scrape frequency
   - Custom metrics: use `relabel_configs`/`metric_relabel_configs` to filter, increase `scrape_interval`
   - Reducing labels reduces time series count; reducing scrape interval reduces sample count

---

#### TSG: Firewall / Network Connectivity / Private Link / AMPLS Issues

**Applies to:** Any cluster where outbound connectivity to Azure Monitor endpoints is blocked — including **ARC/Azure Local clusters behind customer firewalls**, AKS clusters with restrictive NSGs, private-link-enabled clusters, and AMPLS configurations.

**How to detect — the "TokenConfig.json" error chain:**

This is one of the most common patterns. When AMCS endpoints are unreachable (firewall, network policy, private link misconfiguration), the pod enters a restart loop with this characteristic error chain visible in `tsg_errors` and `tsg_logs`:

1. **`TokenConfig.json does not exist`** — logged every 15-30s in ReplicaSet/DaemonSet logs. MDSD/MA cannot download this file from AMCS because the endpoint is unreachable
2. **`AmcsTokenStore.cpp(54): Token config file is not...`** — MetricsExtension (ME) cannot initialize because the AMCS token store was never populated
3. **`MetricsExtensionService.cpp(213): Failed...`** — ME fails to start entirely because it has no authentication tokens
4. **Liveness probe HTTP 503: `"No configuration present for the AKS resource"`** — since ME never starts, the health endpoint returns 503
5. **Container killed & restarted** by kubelet after 3 consecutive failed probes (period=15s, failure=3). Restart count climbs to hundreds/thousands over days
6. **OtelCollector: `Exporting failed... connection refused on 127.0.0.1:55680`** — OtelCollector tries to export scraped metrics to ME's local OTLP endpoint, but ME is not listening. Data is dropped silently
7. **DCR/DCE/AMCS Configuration Errors: thousands per 6-hour window** — massive error volume in `tsg_errors` confirms persistent auth/config failure

**Key insight:** The OtelCollector "connection refused" errors look like an OtelCollector bug but are actually a SYMPTOM of ME not running. Always check the ME and MDSD errors first — they reveal the true root cause (missing TokenConfig.json → blocked endpoints).

**Investigation steps:**

Run `tsg_errors`, look for the error chain above, private link errors, and DNS errors. Then:

1. **Check for the TokenConfig.json error chain** — if `tsg_logs` shows repeated `TokenConfig.json does not exist` and `tsg_errors` shows `AmcsTokenStore` + `MetricsExtensionService` failures, the issue is blocked AMCS endpoints. Proceed to firewall rules below
2. **Check if cluster is ARC / Azure Local** — ARM resource ID containing `Microsoft.Kubernetes/connectedclusters` means this is an ARC cluster. ARC clusters run on-premises behind customer-managed firewalls, making blocked endpoints the most common root cause for pod restart issues
3. **DCE region mismatch** — DCE must be in same region as AKS cluster. If AKS and AMW are in different regions, create a new DCE in the AKS cluster's region
4. **DCE not linked to AMPLS** — check DCE Network Isolation settings, ensure correct Azure Monitor Private Link Scope is selected
5. **Firewall rules** — ensure outbound on port 443 is allowed to:
   - `*.ods.opinsights.azure.com`, `*.oms.opinsights.azure.com`
   - `*.monitoring.azure.com`, `*.metrics.ingest.monitor.azure.com`
   - `*.ingest.monitor.azure.com`, `login.microsoftonline.com`
   - `global.handler.control.monitor.azure.com`
   - `<cluster-region>.handler.control.monitor.azure.com`
6. **Validate connectivity** from a pod: `curl -sv https://global.handler.control.monitor.azure.com`
7. **After fixing** — delete the ama-metrics pods to force fresh config download. TokenConfig.json should appear within 2-3 minutes if endpoints are reachable

---

#### TSG: Control Plane Metrics

Run `tsg_control_plane`. Then:

1. Check AMW quota and OOM issues first
2. Check ASI page (requires VPN): `https://azureserviceinsights.trafficmanager.net/search/services/AKS?searchText={_cluster}` → Addons → Monitoring. If `ama-metrics-ccp` pod OOMing → transfer to AKS RP team
3. Verify ConfigMap formatting: `default-targets-metrics-keep-list`, `minimal-ingestion-profile`, `default-scrape-settings-enabled`
4. Isolate: set some node metrics to `true` and confirm they flow — determines if issue is control-plane-specific
5. Check Metrics Explorer for ingestion rate changes after config changes

---

#### TSG: Windows Pod Restarts (ama-metrics-win)

1. Check pod logs for `TokenConfig.json not found`
2. If liveness probe shows `MetricsExtension not running (configuration exists)` — MA/MDSD was slow downloading TokenConfig.json from AMCS
3. **Resolution:** escalate to AMCS team with the DCR ID (get from `tsg_triage` → `Internal DCE and DCR Ids`)

---

#### TSG: Remote Write Issues

1. Check if Prometheus version ≥ v2.45 (managed identity) or ≥ v2.48 (Entra ID app auth)
2. HTTP 403 → check `Monitoring Metrics Publisher` role on DCR (takes ~30 min to propagate)
3. No data flowing → `kubectl describe pod <prometheus-pod>`, check MSI assignment
4. Container restart loop → verify `AZURE_CLIENT_ID` and `IDENTITY_TYPE` env vars
5. If MDM ingestion issue → transfer to `Geneva Monitoring/Observability T1 Support (Not Live Site)`

---

#### TSG: Vulnerabilities / CVEs

1. Run trivy scan via GitHub action: https://github.com/Azure/prometheus-collector/actions/workflows/scan.yml
2. If CVEs are in base image → create release with new image build (Mariner base auto-upgrades)
3. If CVEs are in packages → check version against Mariner CVE database at aka.ms/astrolabe
4. If we have same or higher version → false positive

---

#### TSG: Node Exporter Missing Labels on ARM64

- ARM64 nodes expose fewer `/proc/cpuinfo` fields than x86_64
- `node_exporter` labels like CPU model/family may be absent — this is by design
- Update dashboards/alerts to not assume architecture-specific labels
- Consider metric relabeling to add stable labels (e.g. `node_architecture`)

---

#### TSG: Pods Not Created / Addon Not Deploying

When `ama-metrics` pods don't exist at all:

1. **Check if monitoring addon is enabled** — run `tsg_config`, check "Addon Enabled in AKS Profile". If `metricsEnabled == false`, the addon isn't enabled. Customer needs to enable via `az aks update --enable-azure-monitor-metrics`
2. **Check cluster PUT failures** — if addon is enabled but pods don't exist, cluster PUT calls may be timing out. Transfer to `Azure Kubernetes Service/RP Triage` for cluster provisioning issues
3. **Check for DCRA (Data Collection Rule Association)** — the DCRA links the DCR to the cluster. If missing, metrics won't flow. Check via Azure Portal → AKS cluster → Monitoring → Data Collection Rules
4. **Check webhook/admission controller** — if the cluster has restrictive admission policies (OPA Gatekeeper, Kyverno), they may block ama-metrics pod creation. Check for denied admission events

---

#### TSG: Proxy / Authenticated Proxy Issues

Run `tsg_errors`, look for HTTP proxy and AMCS connection errors. Then:

1. **Basic proxy** — ama-metrics supports unauthenticated HTTP proxies via AKS outbound proxy config. Check `tsg_config` → "HTTP Proxy Enabled"
2. **Authenticated proxy (NOT supported)** — ama-metrics does NOT currently support proxies that require authentication (username/password). If customer reports `ama-metrics cannot connect to AMCS when proxy has authentication`, confirm this is a known unsupported scenario
3. **Proxy bypass** — customer can configure `NO_PROXY` to bypass proxy for specific endpoints. AMCS and MDM endpoints should be in the bypass list if possible
4. **Escalation** — if this is a hard requirement for the customer, file a feature request on the prometheus-collector GitHub repo

---

#### TSG: Liveness Probe Failures (503)

Run `tsg_errors`, check "Liveness Probe Logs". Then:

1. **HTTP 503 from liveness probe** — this means ME (MetricsExtension) is not ready. Common causes:
   - TokenConfig.json not yet downloaded from AMCS (slow AMCS response, especially on cold start)
   - DCR/DCE misconfiguration preventing config download
   - Network policy blocking egress to AMCS endpoints
2. **Check auth errors** — run `tsg_errors`, look for `DCR/DCE/AMCS Configuration Errors`. If "Configuration not found", the DCR may be deleted or DCE endpoint is wrong
3. **Transient vs persistent** — if liveness probes fail only during pod startup (first 30-60s) then succeed, this is normal cold-start behavior. If persistent, there's a config or network issue
4. **Gov cloud / sovereign** — gov cloud clusters (`*.cx.aks.containerservice.azure.us`) have different AMCS endpoints. Verify the DCE region matches the cluster region

---

#### TSG: Duplicate Label Errors (kube-state-metrics)

When `kube-state-metrics` scraping produces duplicate label errors:

1. **Check `Kube-State-Metrics Labels Allow List`** — run `tsg_config`. If customer set `metricLabelsAllowlist` to `[*]` (all labels), Kubernetes labels may conflict with Prometheus labels (e.g., `pod`, `namespace` exist as both KSM metric labels and Kubernetes object labels)
2. **Check for double-scraping** — customer may have both default KSM scraping enabled AND a custom job scraping the same KSM endpoint with different label handling
3. **Resolution** — either narrow the `metricLabelsAllowlist` to specific needed labels, or use `metric_relabel_configs` to rename/drop conflicting labels

---

#### TSG: DCR/DCE Region Mismatch

Run `tsg_triage`, check DCR and DCE configuration. Then:

1. **Random region name in DCR/DCE** — when AKS and AMW are in different regions, the system may create DCR/DCE resources in an unexpected region. The DCE MUST be in the same region as the AKS cluster
2. **Fix** — customer should create a new DCE in the AKS cluster's region and update the DCRA to point to it
3. **Validation** — check `tsg_triage` → "Internal DCE and DCR Ids" to see which DCE region is being used

---

#### TSG: AMW Usage Optimization

When customer asks about reducing Azure Monitor Workspace costs:

1. **Identify top volume drivers** — run `tsg_metric_insights` to see which metrics have the most time series and highest sample rates
2. **Reduce default metrics** — use `ama-metrics-settings-configmap` to disable unused default targets (e.g., `kube-proxy`, `core-dns` if not needed)
3. **Set keep lists** — configure `default-targets-metrics-keep-list` to only ingest needed metrics from each target
4. **Increase scrape intervals** — change from default 15s/30s to 60s for non-critical targets via `default-scrape-settings-enabled`
5. **Reduce cardinality** — use `metric_relabel_configs` to drop high-cardinality labels. Check `tsg_metric_insights` → "Metrics with High Dimension Cardinality"
6. **Enable minimal ingestion profile** — set `minimal-ingestion-profile: true` in settings configmap to only ingest a curated set of metrics
7. **Reference** — search the [Prometheus cost optimization docs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-overview) for the latest guidance

---

## Known Issues & FAQ

These are specific known behaviors and past incidents — not troubleshooting workflows, but useful context when a customer reports one of these patterns.

**HPA scaling down unexpectedly** — HPA scaling down is expected behavior when metric volume decreases (e.g., customer deployed a new app version that exposes fewer metrics). Check `tsg_workload` → "HPA Status". Customer can set `minshards` in `ama-metrics-settings-configmap` to prevent scaling below a minimum.

**Inconsistent cAdvisor scrape intervals** — cAdvisor scraping has known inconsistent intervals due to kubelet `/metrics/cadvisor` endpoint latency. Key investigation steps:
1. **Check scrape interval** — run `tsg_config`, look at "Default Targets Scrape Interval". cAdvisor defaults to **15s** — the most aggressive default target (others are 30-60s). This is the primary contributor to timeouts.
2. **Check per-pod sample variance** — run `tsg_workload`, look at "DaemonSet Per-Pod Sample Rate Variance". If `highVariance == true` (>100% difference between min/max pod rates), nodes have very different container counts. Nodes with more containers produce slower cadvisor responses.
3. **Check DaemonSet resource usage** — run `tsg_workload`, look at DaemonSet CPU/memory. If near limits (default: 500m CPU / 1Gi memory), the collector may not have enough resources to maintain consistent scrape timing.
4. **Root cause**: Kubelet's `/metrics/cadvisor` endpoint enumerates cgroup stats for ALL containers on the node — inherently slower than node-exporter (which reads static `/proc` files). When response time exceeds `scrape_timeout` (default 10s), the sample is silently dropped, creating gaps.
5. **Why node-exporter is unaffected**: Node-exporter reads static `/proc` and `/sys` files — near-instant. Kubelet cadvisor queries cgroups for every container — can take seconds on busy nodes.
6. **`scrape_duration_seconds` is NOT in our App Insights telemetry** — customer must verify via `kubectl port-forward <ama-metrics-node-pod> 9090` → query `scrape_duration_seconds{job="cadvisor"}` or check `/targets` page for "Last Scrape Duration".
7. **Recommendations**: Increase `scrape_timeout` for cadvisor to match interval (e.g. 15s), or increase cadvisor `scrape_interval` to 30-60s via `ama-metrics-settings-configmap`. This reduces kubelet load by 2-4x and eliminates most timeouts.
8. **This is a systemic kubelet behavior**, not a collector bug. Affects all clusters but is more pronounced on nodes with many containers (60+ pods/node).

**Post-rollout minimal ingestion profile regression (Aug 2025)** — A past addon release broke minimal ingestion profile logic, causing clusters without ConfigMaps to ingest ALL metrics. Symptoms: sudden CPU spike + ingestion increase after addon update. Workaround: deploy `ama-metrics-settings-configmap` with explicit `minimal-ingestion-profile: true`. If a new version causes similar regression, file Sev2 to `Container Insights/AzureManagedPrometheusAgent`.

**Tolerations blocking node drain** — Older addon versions had tolerations that prevented pod eviction during node drains/cluster upgrades. Fixed in recent releases. Workaround: manually delete the pod before draining. Fix: upgrade addon to latest.

