# Reference Guide

Detailed reference information for the prom-collector-tsg MCP tools, data sources,
version checking, MetricsExtension deep-dives, and escalation procedures.

---

#### Using `tsg_query` for Ad-Hoc Investigation

Use `tsg_query` when the built-in tools don't cover your specific symptom. It accepts optional `cluster`, `timeRange`, `outputFile`, `outputFormat`, and `maxRows` parameters:

```
tsg_query(datasource: "PrometheusAppInsights", kql: "traces | where ...", cluster: "/subscriptions/.../managedClusters/name", timeRange: "7d")
```

When `cluster` is provided, any `_cluster` placeholder in the KQL is auto-replaced with the cluster ARM ID.

**Writing full results to a file** (bypasses the default 100-row truncation):
```
tsg_query(datasource: "PrometheusAppInsights", kql: "traces | where ...", outputFile: "/tmp/results.csv")
tsg_query(datasource: "AKS", kql: "...", outputFile: "/tmp/data.json", outputFormat: "json")
```
- `outputFile`: absolute path to write results. Extension determines format (`.csv` or `.json`), or use `outputFormat` to override
- `maxRows`: override inline truncation limit (default 100) without writing to file

**Common KQL patterns:**
- **Find config errors**: `traces | where tostring(customDimensions.cluster) =~ _cluster | where message has "unmarshal" | ...`
- **Check pod memory over time**: `customMetrics | where tostring(customDimensions.cluster) =~ _cluster | where name == "otelcollector_memory_rss" | ...`
- **Search logs by keyword**: `traces | where tostring(customDimensions.cluster) =~ _cluster | where message has "YOUR_KEYWORD" | take 20`

**Data sources available:**
| Data Source | Tables | Use For |
|---|---|---|
| PrometheusAppInsights | `traces`, `customMetrics` | Collector logs, configs, telemetry metrics |
| MetricInsights | `GetPreaggUsageSummary*` | Time series counts, ingestion rates |
| AMWInfo | `AzureMonitorMetricsDCRDaily`, `AzureMonitorWorkspaceStatsDaily` | AMW/DCR/MDM account mapping |
| AKS | `ManagedClusterSnapshot`, `AKSprod` tables | Cluster state, settings, addon config |
| AKS CCP | `AKSccplogs` tables | Control plane logs, AMA metrics status |
| AKS Infra | `AKSinfra` tables | Control plane pod CPU, container restarts |
| Vulnerabilities | `ShaVulnMgmt` tables | Image CVE scanning |
| ARMProd | `HttpIncomingRequests` | ARM deployment logs — what was deployed/enabled and when |

#### Checking Scrape Target Health via Geneva MDM

When investigating **intermittent missing metrics** for a specific target (e.g. kube-state-metrics), use the Geneva MDM MCP server to query the `up` metric:

1. Run `tsg_triage` → extract the `MDMAccountName` (e.g. `mac_0d8947c8_...`)
2. Use the `geneva-mdm` MCP tools to query the `up` metric:
   - Namespace: `customdefault` (or `prometheus` depending on configuration)
   - Metric: `up`
   - Filter by dimension `job` = target name (e.g. `kube-state-metrics`)
   - Look at the `Sum` field (not `Min` — gauge metrics without pre-agg always show NaN for Min)
3. **Interpreting results:**
   - Typical Sum = N × scrapes_per_minute (e.g. Sum=45 means 3 replicas × 15 scrapes/min at 4s interval)
   - Sum dips below typical → some scrapes returned `up=0` (target unreachable)
   - Calculate failure rate: `(typical_sum - actual_sum) / typical_sum × 100`
   - Failure rate < 1% → transient scrape timeouts, usually self-healing
   - Failure rate > 5% → persistent target health issue, check target pod logs
4. **Correlate with App Insights logs** — search for target-specific log tags:
   - `prometheus.log.kubestatemetricscontainer` — KSM pod logs
   - `prometheus.log.targetallocator.tacontainer` — target allocator logs
   - `prometheus.log.prometheuscollectorcontainer` — otelcollector scrape logs (ReplicaSet)
   - If a log tag has zero entries, that component isn't sending telemetry (may be crash-looping)

#### MDM Account Resolution and Throttling Check

The `tsg_triage` tool includes the **"MDM Account ID"** query which resolves the cluster ARM resource ID to the Geneva MDM monitoring account name(s) via `AzureMonitorMetricsDCRDaily` → `AzureMonitorWorkspaceStatsDaily`.

After running `tsg_triage`, extract the `MDMAccountName` from the "MDM Account ID" result and pass it to `tsg_mdm_throttling` to check for throttling:

1. Run `tsg_triage` → Look at "MDM Account ID" row → get `MDMAccountName` value (e.g. `cirruspl_promws_at52044_neu1`)
2. Run `tsg_mdm_throttling` with `monitoringAccount` = that MDMAccountName
3. If the customer has multiple AMWs, repeat for each `MDMAccountName`

The throttling check queries the **MdmQos** namespace for: `ThrottledClientMetricCount`, `DroppedClientMetricCount`, `ThrottledTimeSeriesCount`, `MStoreDroppedSamplesCount`, `ClientAggregatedMetricCount` vs Limit, `MStoreActiveTimeSeriesCount` vs Limit, and `ThrottledQueriesCount`.

Run `tsg_triage` first, then based on findings, run the relevant deep-dive tools.

#### CCP Cluster ID Resolution

The `tsg_triage` tool includes a "CCP Cluster ID" query that resolves the ARM resource ID to the CCP namespace
(e.g. `6604ae19e8805300010dae5e`). This ID is required by all AKS/CCP queries. The tool passes it automatically
via the `AKSClusterID` parameter.

If App Insights queries return "path does not exist", it means the addon is crash-looping and not sending telemetry.
Go directly to AKS CCP data via `tsg_pods` for pod restart analysis.

#### Node Pool Capacity Check

The `tsg_triage` tool includes these node health queries:

- **Node Pool Capacity** — shows current node count vs autoscaler max with `isFull` flag, plus **vmSize** and **mode** (System vs User). Focus on the **System** mode pool since ReplicaSet pods run there.
- **Node Conditions (Memory/Disk/PID Pressure)** — shows per-node conditions. If `MemoryPressure == True`, the node is running out of memory and the scheduler won't place new pods on it.
- **Node Allocatable Resources** — shows allocatable vs capacity memory/CPU/pods per node. Helps identify if nodes have room for more ama-metrics pods.

**IMPORTANT:** ama-metrics ReplicaSet pods run on **system node pools** (not user pools) because they are a managed AKS addon. User pool node counts and VM sizes are **irrelevant** for ReplicaSet OOMKill analysis. Always check the system pool VM size and capacity.

**Check workflow for OOMKills:**
1. `tsg_triage` → Check "Node Pool Capacity" — find the **System** mode pool. Note the **vmSize** (e.g. Standard_E4s_v5 = 32GB) and **currentNodes** count. This determines total memory available for ReplicaSet pods
2. `tsg_workload` → Check "HPA Status" for `currentReplicas`, `maxReplicas`, and `atLimit` flag. The HPA scales ReplicaSet pods to handle high metric volumes — it WILL scale if the system pool supports it
3. **Calculate capacity:** Each ReplicaSet pod has a 14Gi memory limit. If the system pool has N nodes × M GB each, then max pods ≈ (N × M) / 14. For example: 4 × Standard_E4s_v5 (32GB) = 128GB → ~9 pods max. If HPA wants 15 replicas but the system pool only fits 9, pods will OOMKill
4. `tsg_workload` → Check "Pod Resource Limits" for actual memory/CPU limits on prometheus-collector container
5. `tsg_pods` → Check "Pod to Node Mapping" — confirms which system pool nodes have ama-metrics pods and how many per node
6. `tsg_pods` → Check "System Pool Node Resources" — shows allocatable memory and MemoryPressure per system node
7. `tsg_triage` → Check "Node Conditions" for `MemoryPressure == True` on system pool nodes
8. `tsg_pods` → Check "Node Status Timeline" — shows when nodes transitioned to NotReady/Unknown, which may correlate with OOMKill waves
9. If system pool VM size is too small for the metric volume → customer needs **bigger system pool VMs** (e.g., upgrade from Standard_E4s_v5 to Standard_E8s_v5)
10. If HPA is at limit (`atLimit == true`) → customer can increase `maxReplicas` up to 30 via `ama-metrics-settings-configmap` → `minshards`, BUT only if system pool nodes can accommodate more pods
11. If system pool is at max node count (`isFull == true`) → customer needs to increase maxCount on the system pool or use bigger VMs
12. **Most common root cause:** Customer has high Istio/Envoy metric volume (millions of time series) but system pool uses small VMs (32GB). The HPA scales out replicas to handle volume, but each replica needs up to 14Gi memory. Small system pool nodes cannot fit enough replicas → constant OOMKill cycle. **Solution: reduce metric volume via metric_relabel_configs (drop histogram _bucket metrics) AND/OR upgrade system pool VM size**


---

#### Querying ARM Deployment Logs

Use the `ARMProd` data source (`armprod.kusto.windows.net`) to investigate **what was deployed to a cluster and when**. This is useful for:
- Determining when managed prometheus was enabled
- Checking if DCR/DCE/DCRA creation succeeded or failed during addon enablement
- Finding what ARM operations were performed on the cluster

**⚠️ Connectivity note:** The ARMProd Kusto cluster (`armprod.kusto.windows.net`) has a **Conditional Access Policy** that blocks device-code flow and non-compliant device auth. `az login --use-device-code` will fail with "does not meet the criteria to access this resource". The MCP server's `DefaultAzureCredential` may also fail for the same reason. **Workaround:** Use `azureauth --scope https://kusto.kusto.windows.net/.default --output token` which uses the Windows WAM broker through WSL interop and satisfies CAP requirements. If the MCP server can't reach ARMProd, provide the user with KQL queries to run manually in [Kusto Web Explorer](https://dataexplorer.azure.com) connected to `armprod.kusto.windows.net` / `ARMProd`.

**Example queries:**

Find all ARM operations around addon enablement time:
```kql
tsg_query(datasource: "ARMProd", kql: "HttpIncomingRequests | where PreciseTimeStamp between (datetime(2026-03-25T15:00:00Z) .. datetime(2026-03-25T18:00:00Z)) | where subscriptionId == \"<SUB_ID>\" | where resourceUri has \"<CLUSTER_NAME>\" or resourceUri has \"datacollectionrule\" or resourceUri has \"microsoft.monitor\" or resourceUri has \"datacollectionendpoint\" | project PreciseTimeStamp, httpMethod, resourceUri, httpStatusCode | order by PreciseTimeStamp asc | take 30")
```

Check for failed DCR/DCE/DCRA creation (non-200 status codes indicate failures):
```kql
tsg_query(datasource: "ARMProd", kql: "HttpIncomingRequests | where PreciseTimeStamp > ago(7d) | where subscriptionId == \"<SUB_ID>\" | where resourceUri has \"datacollectionrule\" or resourceUri has \"datacollectionendpoint\" or resourceUri has \"microsoft.monitor\" | where httpStatusCode != 200 and httpStatusCode != 201 | project PreciseTimeStamp, httpMethod, resourceUri, httpStatusCode | order by PreciseTimeStamp desc | take 20")
```

---

#### Checking Versions and Release Notes

When investigating an ICM, **always check the addon and component versions** as part of triage:

1. **`tsg_triage` → "Version"** — shows the `agentversion` (addon image tag like `6.26.0`)
2. **`tsg_triage` → "Component Versions (ME, OtelCollector, Golang, Prometheus)"** — shows MetricsExtension version (`ME_VERSION`), OTel Collector version, Golang version, and Prometheus version from startup logs. These are logged at pod startup via `FmtVar()` calls

**Checking release notes:**
- **Addon (prometheus-collector) release notes**: `RELEASENOTES.md` in the repo root — lists each release with image tags, changes, bug fixes, and dependency bumps. Map the customer's `agentversion` to a release date to see what changed
- **MetricsExtension release notes**: ME is a closed-source binary bundled inside the container. Its version (e.g. `2.2024.328.1744`) is logged at startup as `ME_VERSION`. ME versions are updated in our releases — search `RELEASENOTES.md` for "MetricsExtension" to find version bumps. For ME-specific bugs or behavior, check with the Geneva Metrics team
- **Remote Write release notes**: `REMOTE-WRITE-RELEASENOTES.md` — separate changelog for remote write functionality

**Common version-related investigation patterns:**
- **Post-upgrade regression**: Compare the customer's `agentversion` with `RELEASENOTES.md` to see if a recent addon upgrade introduced the issue. Check "Known Issues" section of the skill for post-rollout regressions
- **Old addon version**: If the customer is running an old version (e.g. `6.20.x`), check if the issue was already fixed in a newer release before deep-diving
- **ME version mismatch**: If `ME_VERSION` shows an unexpected version, it may indicate the container image wasn't properly rebuilt

#### Deep-Diving into MetricsExtension (ME) Issues

MetricsExtension is a closed-source C++ binary (owned by the Geneva Metrics team) that handles metric aggregation, batching, and ingestion into Geneva/MDM. It runs as a sidecar process inside the prometheus-collector container. When ME-specific issues arise (crashes, ingestion failures, throttling, queue backup, token errors), the TSG tools above may not be enough.

**When to deep-dive into ME:**
- `tsg_errors` → "MetricsExtension Errors" shows persistent `MetricsExtensionConsoleDebugLog` errors
- `tsg_workload` → ME CPU/Memory is abnormally high but OtelCollector is fine
- `tsg_errors` → "MDSD Errors" shows `AmcsTokenStore` or `MetricsExtensionService` failures
- Liveness probe shows `MetricsExtension not running` or HTTP 503
- `tsg_mdm_throttling` shows throttled/dropped events originating from ME pipeline
- Customer sees metric gaps but OtelCollector is scraping successfully (ME ingestion failure)

**Tools for ME investigation:**
1. **`enghub-search`** — Search Engineering Hub for ME TSGs, onboarding guides, and known issues. Example queries:
   - `enghub-search(query: "MetricsExtension crash")` — find ME crash TSGs
   - `enghub-search(query: "MetricsExtension token adapter")` — find token/auth debugging
   - `enghub-search(query: "Geneva Metrics ingestion failure")` — find ingestion pipeline docs
   - `enghub-search(query: "MDSD MetricsExtension configuration")` — find config/setup docs
2. **`enghub-fetch`** — Read full content of any Engineering Hub page found via search
3. **`es-chat-es_ask`** / **`es-chat-es_search`** — Ask Engineering Systems chat for ME-specific questions or search across internal knowledge bases. Examples:
   - `es_ask(question: "How does MetricsExtension handle token refresh failures?")` 
   - `es_search(keywords: "MetricsExtension OOM memory leak", question: "What causes MetricsExtension memory leaks?")`
4. **ME source code** — For detailed code-level investigation, the MetricsExtension ADO repository is at:
   **https://msazure.visualstudio.com/One/_git/EngSys-MDA-MetricsExtension?version=GBmaster**
   Use `ado-tracing-search_code` or browse the repo directly to understand ME behavior, error handling, config parsing, and ingestion pipeline internals

**Key ME components to understand:**
- **AmcsTokenStore** — manages authentication tokens from AMCS. Failures here mean ME can't authenticate to Geneva/MDM
- **MetricsExtensionService** — main service entry point. If this fails to start, no metrics are ingested
- **MStore** — the metric store/queue inside ME. `MStoreDroppedSamplesCount` and `MStoreActiveTimeSeriesCount` in MDM QoS indicate ME-side issues
- **TokenConfig.json** — configuration file downloaded by MDSD from AMCS endpoints. If blocked by firewall, ME never initializes

---

## Investigating AKS Upgrades and Node Exporter Version Changes

AKS cluster upgrades change the node image, which bundles a specific `node_exporter` version. A new node exporter version can:
- Expose **new metrics** (increasing cardinality and TS count)
- **Remove or rename metrics** (breaking dashboards/alerts)
- **Break scraping entirely** (up=0 for the `node` job)

### How to detect an AKS upgrade from telemetry

1. **`tsg_triage` → "AKS Upgrade History"** — queries `AgentPoolSnapshot` by `resource_id` (works even when CCP cluster ID resolution fails). Shows `min_ts` and `max_ts` per version per pool — the version transition timestamps reveal exactly when the upgrade happened

2. **`tsg_triage` → "Node Pool Versions"** — shows current `orchestratorVersion`, `osSku`, `distroVersion`, `imageRef` per pool

3. **`tsg_scrape_health`** with `job="node"` — shows `up` metric success rate. If the new node exporter broke, success rate drops (e.g. 43% → 0%)

4. **`tsg_workload` → "Node Exporter Sample Count Trend"** — tracks `max_samples` for the `node` job over time. A version change shows as a step change:
   - `max_samples` 2028 → 2065 = new version exposing 37 additional metrics passing the keep-list
   - `max_samples` drops to 0 = new version broke scraping entirely

5. **`tsg_workload` → "Scrape Samples Per Job Over Time"** — shows all jobs trending. If total samples are flat but TS is growing, it's label churn, not new metrics

### Example: AKS 1.32.5 → 1.34.2 upgrade pattern

```
AgentPoolSnapshot timeline:
  1.32.5: Mar 24 20:15 → Mar 26 03:09 (stable)
  1.33.6: Mar 26 03:14 → Mar 26 04:09 (transitional, ~1 hour)
  1.34.2: Mar 26 04:14 → present

Node exporter impact:
  max_samples changed 2028 → 2065 (new NE version in 1.34 node image)
  Success rate degraded from ~100% to 43%, then to 0%
  TS count exploded as quota auto-scaled to accommodate new cardinality
```

### AKS Kusto column reference

The `AgentPoolSnapshot` table in the AKS data source has two cluster identifier columns:
- `cluster_id` — CCP hex ID (e.g. `69c58df659d077000103a651`). Used by most existing queries via `AKSClusterID` token resolution. **May fail** if the CCP cluster ID cannot be resolved (returns empty results with no error)
- `resource_id` — ARM resource ID path. **Always works** as a fallback: `where resource_id =~ _cluster`

If all AKS/CCP queries return empty or 400 errors but auth checks pass, try `tsg_query` with `resource_id =~ _cluster` instead of `cluster_id == AKSClusterID`.

---

## Querying Historical Time Ranges

All tools support optional `startTime` and `endTime` parameters (ISO 8601 format) for querying specific past time windows instead of relative ranges:

```
tsg_triage(cluster="...", startTime="2026-03-10T00:00:00Z", endTime="2026-03-10T12:00:00Z")
```

When `startTime`/`endTime` are provided, they override the `timeRange` parameter for both KQL token replacement and App Insights query timespan. Use this when investigating incidents that occurred days or weeks ago.

## Customer Reference Links

When summarizing findings for ICM or customer communication, **search** these documentation trees for the specific page relevant to the customer's issue — do not just link the overview page. Use `web_search` to find the right sub-page:

- **Azure Managed Prometheus** (TOC root): https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/prometheus-metrics-overview
  - Sub-pages cover: custom scrape config, remote write, recording rules, default targets, metric keep lists, troubleshooting
- **Kubernetes monitoring** (TOC root): https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-overview
  - Sub-pages cover: AKS addon install, managed Grafana, cost optimization, data collection rules, troubleshooting
- **TSG wiki (internal)**: https://dev.azure.com/msazure/InfrastructureInsights/_wiki/wikis/InfrastructureInsights.wiki?pagePath=/ManagedPrometheus/OnCall/TSGs
