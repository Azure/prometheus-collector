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
- **ME account routing errors** (multi-AMW): `traces | where message has "AggregatedMetricsPublisher" and message has "publication failed" | project timestamp, message | order by timestamp desc | take 10` — look for `_Default[wrong-endpoint]` to identify which AMW endpoint ME is trying to reach
- **Cross-cluster AMW lookup** (AMWInfo): `search '<amw-name>' | where $table == 'AzureMonitorMetricsDCRDaily' | project ParentResourceId | distinct ParentResourceId` — finds which cluster an AMW is associated with globally
- **All AMWs for a subscription** (AMWInfo): `search '<subscription-id>' | where $table == 'AzureMonitorWorkspaceStatsDaily' | distinct AMWAccountResourceId, MDMAccountName`

**Tips:**
- Use `outputFile` parameter to write full results to JSON/CSV — avoids truncation of long ARM resource IDs in tabular output
- AMWInfo tables are not directly queryable by name; use `search 'keyword'` with `$table` filters for ad-hoc exploration

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
| ARMProd | `HttpIncomingRequests` | ARM global proxy — prefer regional clusters below |
| ARMPRODSEA | `HttpIncomingRequests`, `HttpOutgoingRequests` | ARM regional: Asia/Pacific/UK/Africa |
| ARMPRODEUS | `HttpIncomingRequests`, `HttpOutgoingRequests` | ARM regional: Americas |
| ARMPRODWEU | `HttpIncomingRequests`, `HttpOutgoingRequests` | ARM regional: Europe |

#### Checking Scrape Target Health via Geneva MDM

Use internal service telemetry for incident work: `PrometheusAppInsights`,
`MetricInsights`, internal AKS telemetry, and Geneva MDM. A cluster ARM ID is a
filter for internal telemetry, not permission to query the customer's cluster,
ARM endpoint, workspace, or Grafana. Resolve `MDMAccountName` with `tsg_triage` or
the internal AMWInfo mapping, then use the [verified Windows helper startup](../troubleshooting-setup/SKILL.md#start-the-server-windows).

Start with a short incident window. `tsg_workload` accepts explicit `startTime` and
`endTime` for historical investigation; do not infer retention expiry from one
empty ad-hoc query. Use `tsg_query` for bounded App Insights logs, preserving all
returned rows with `outputFile` when necessary.

##### Discovering scrape-metric preaggregates

1. Verify the helper's MCP initialization and tool catalog, then run
   `tsg_mdm_throttling` against the authorized account. Real `MdmQos` values verify
   that path only, not KSM scrape health or `MetricsExtension2` availability.
2. Use `tsg_mdm_metadata` with `operation: "dimensions"` or `"preaggregates"` and
   the account, namespace and metric. It dispatches `GetDimensions` or
   `GetPreAggregateConfigurations`; `"namespaces"` and `"metrics"` enumerate names.
   Empty metadata is inconclusive. Use MetricInsights below when that happens.
   Individual dimension names do not establish that their combination and
   requested sampling types are queryable.
3. Use `tsg_query(datasource: "MetricInsights", kql: ...)` with explicitly typed
   arguments to discover observed scrape-metric preaggregates:

```kusto
let _mdmAccount = '<MDM_ACCOUNT>';
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| where MetricName in ('up','scrape_samples_scraped','scrape_samples_post_metric_relabeling','scrape_duration_seconds')
| project MetricName, Dimensions, DailyTSAcrossAccounts, AvgEventRate
| order by MetricName asc
```

The 180-day usage view is historical/account-wide evidence, not proof of current
ingestion or a usable MDM query. Use the discovered combinations when constructing
a query, then verify its actual response. Adding `instance` alone does not
establish that a dimension/sampling combination works. Successful family Count
queries do not establish that scrape-total queries work.

`InvalidSeries` with "Metric doesn't have preaggregate defined ... requested
dimensions and requested sampling types" is a query/preaggregate failure, not
evidence that the metric is absent. Inspect the inner response even when the
outer wrapper says `Success`. `tsg_scrape_health` uses `cluster` + `job`;
`tsg_mdm_query` adds `cluster` to the supplied dimensions. Neither can establish
scrape health when that query fails. Preserve the error, requested dimensions,
sampling types, and time window for the owning team rather than repeatedly
widening the window or reporting zero samples.

Only interpret numeric `up` series after validating the query. A bucket needs
finite Sum and positive finite Count to count as observed. Sum=0 with Count>0
is an observed zero; Count=0 or NaN is unobserved, not a healthy scrape or proof
that metric data exists. A no-failures result applies only to observed scrapes;
health during missing buckets remains unknown.

`tsg_mdm_query` rejects malformed dimension JSON or shapes other than an object
mapping dimension names to arrays of strings. It never silently drops invalid
job/instance filters to run a broader cluster query.

`up=0` with observed samples indicates a failed scrape, not which team caused
it. Missing data is inconclusive, not proof that a collector stopped.
Correlate with App Insights target, collector, and allocator logs
(`prometheus.log.kubestatemetricscontainer`,
`prometheus.log.prometheuscollectorcontainer`,
`prometheus.log.targetallocator.tacontainer`). Missing log rows alone do not prove
a crash.

##### Flexible target counts, family comparisons, and label breakdowns

Use `tsg_mdm_kqlm` with `monitoringAccount: "<MDM_ACCOUNT>"`,
`metricNamespace: "customdefault"`, explicit UTC `startTime`/`endTime`, and a
`queries` array. Use the configured Prometheus **cluster label**, not an assumed
AKS resource name. Replace placeholders before submitting; the account is not
the AMW ARM ID.

This one-minute Count shape succeeded for individual KSM metric families:

```kusto
metric("<METRIC>")
.dimensions("cluster","job","instance")
.resolution(1m)
.samplingTypes("Count")
| where cluster == "<CLUSTER_LABEL>" and job == "<JOB>" and instance == "<INSTANCE>"
| summarize Observed=sum(Count) by cluster,job,instance
```

For **per-family comparison**, submit one copy per discovered metric in
`queries`, using identical filters, sampling type, resolution and window.
Responses retain each query and every returned value; `finiteValueCount` counts
finite numeric values, **not events**. `missingValueCount` counts NaN/null values.
A finite zero stays zero; no series, all-NaN arrays and an error are distinct.
Only `Status: "Success", Results: []` confirms a no-series response; a missing
helper payload (`Result: null` or `[]`) is an inconclusive query error.
The batch runs serially and any failed query fails the call rather than being
silently treated as absent data.

For a **condition/status breakdown**, this shape also succeeded:

```kusto
metric("<METRIC>")
.dimensions("cluster","job","instance","condition","status")
.resolution(1m)
.samplingTypes("Count")
| where cluster == "<CLUSTER_LABEL>" and job == "<JOB>" and instance == "<INSTANCE>"
| summarize Observed=sum(Count) by condition,status
```

Choose grouping dimensions from discovered metadata; a dimension's existence is
not proof that every combination is queryable. Do not assume every condition
exists on every node, or count every status series as a distinct node.

For **raw-series/identity cardinality**, this unsummarized query succeeded for
`kube_pod_status_phase`, with either `Count` or `Sum`:

```kusto
metric("<METRIC>").dimensions("cluster","job","instance").samplingTypes("Count")
| where cluster == "<CLUSTER_LABEL>" and job == "<JOB>"
```

Use `outputFile` to preserve the entire JSON response. Unlike the summarized
queries above, this verified raw query did not specify a resolution or filter
`instance`; it returned instance and the full namespace/pod/UID/phase labels.
Filter the complete `results[i].result.Results` locally to the required instance
before counting distinct complete `Dimensions` maps, distinct
`(namespace,pod,uid)` tuples, and namespace/phase values separately.
These are returned label-set/identity cardinalities across the requested window,
not concurrent pod counts or scrape payload size. Keep series with no finite
values distinguishable from measured values when reporting cardinality.

**Interpretation limits:** Count here measures ingested MDM events for the queried
family, not total scrape samples, unique samples, payload bytes, or an endpoint
inventory. Per-family maxima may occur in different buckets; do not sum them as
a simultaneous total. The verified helper returned arrays without timestamps.
`resolution(1m)` requests a resolution but does not establish bucket timestamps
or prove a gap from a NaN array position. The tool preserves any actual timestamp
fields but never derives them from the request or array index. `requestedWindow`
is only the requested window. Successful family reads establish neither
scrape-total availability nor collector ownership.

##### Target counts, ownership, and job load

`target_allocator_opentelemetry_allocator_targets` is a discovered-target gauge
by job, not a local collector's assignment (see
`otelcollector/otel-allocator/internal/target/discovery.go`).
A job-labeled global allocator metric emitted by multiple replicas does not
identify that job's owner. `targets_per_collector` records assigned target counts using
`collector_name` and `strategy`, not per-job sample volume (see
`otelcollector/otel-allocator/internal/allocation/allocator.go`).

Establish ownership only with time-aligned assignment evidence containing both
job/target and collector identity. Establish load with actual per-job sample
telemetry and verified units. Do not assign a replica's excess volume to KSM or
sum counter snapshots as events: verify rate/delta semantics, resets, aggregation,
and bucket width first. If internal telemetry lacks those labels, report that
ownership or job volume remains unproven; do not query customer endpoints to fill
the gap.

#### MDM Account Resolution and Throttling Check

The `tsg_triage` tool includes the **"MDM Account ID"** query which resolves the cluster ARM resource ID to the Geneva MDM monitoring account name(s) via `AzureMonitorMetricsDCRDaily` → `AzureMonitorWorkspaceStatsDaily`.

After running `tsg_triage`, extract the `MDMAccountName` from the "MDM Account ID" result and pass it to `tsg_mdm_throttling` to check for throttling:

1. Run `tsg_triage` → Look at "MDM Account ID" row → get `MDMAccountName` value (e.g. `cirruspl_promws_at52044_neu1`)
2. Run `tsg_mdm_throttling` with `monitoringAccount` = that MDMAccountName
3. If the customer has multiple AMWs, repeat for each `MDMAccountName`

The throttling check queries the **MdmQos** namespace for: `ThrottledClientMetricCount`, `DroppedClientMetricCount`, `ThrottledTimeSeriesCount`, `MStoreDroppedSamplesCount`, `ClientAggregatedMetricCount` vs Limit, `MStoreActiveTimeSeriesCount` vs Limit, and `ThrottledQueriesCount`.

Treat drops and throttling separately: `Duplicated`, `TooManyDimensions`, and
`NotSupportedDoubleValue` are not evidence of quota throttling. Inspect the
reason breakdown and explicit throttling series before attributing quota
pressure. Do not assume active-series counts or limit values are per-minute
rates. Empty, NaN, failed, or partially missing ME/QoS panels are inconclusive;
only numeric zero drop/error panels support "no drops detected in queried data."
Even zero panels do not establish successful end-to-end ingestion.

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
10. If HPA is at limit (`atLimit == true`) → customer can increase `maxReplicas` by patching the `ama-metrics-hpa` HPA object in `kube-system`: `kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"maxReplicas": 30}}'` — see [Autoscaling docs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-autoscaling). BUT only if system pool nodes can accommodate more pods
11. If system pool is at max node count (`isFull == true`) → customer needs to increase maxCount on the system pool or use bigger VMs
12. **Most common root cause:** Customer has high Istio/Envoy metric volume (millions of time series) but system pool uses small VMs (32GB). The HPA scales out replicas to handle volume, but each replica needs up to 14Gi memory. Small system pool nodes cannot fit enough replicas → constant OOMKill cycle. **Solution: reduce metric volume via metric_relabel_configs (drop histogram _bucket metrics) AND/OR upgrade system pool VM size**


---

#### Querying ARM Deployment Logs

ARM has **3 regional Kusto v2 clusters** in Public Cloud — you must query the correct one based on the AKS cluster's region:

| Data Source | Kusto Cluster | Database | Regions Covered |
|---|---|---|---|
| `ARMPRODSEA` | `armprodsea.southeastasia.kusto.windows.net` | `Requests` | East Asia, Southeast Asia, Australia, Japan, Korea, India, Israel, UAE, UK, South Africa |
| `ARMPRODEUS` | `armprodeus.eastus.kusto.windows.net` | `Requests` | East US, West US, Central US, Canada, Brazil, all Americas |
| `ARMPRODWEU` | `armprodweu.westeurope.kusto.windows.net` | `Requests` | West Europe, North Europe, Germany, France, Switzerland, Norway, Sweden |

The global endpoint (`armprod.kusto.windows.net` / `ARMProd`) is a query proxy only — it often fails with CAP or empty results. **Always use the regional cluster.**

**Tables:** `HttpIncomingRequests`, `HttpOutgoingRequests`

**⚠️ Key gotchas:**
- **Column name:** Use `targetUri` (NOT `resourceUri`) for filtering ARM request paths
- **Provider names are UPPERCASE:** Use `toupper()` for case-insensitive matching (e.g., `toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'`)
- **30-day retention only:** Queries MUST include `TIMESTAMP > ago(30d)` — omitting the time filter returns HTTP 400
- **DCR/DCE/DCRA can be in any resource group** — don't filter by the AKS cluster's resource group when searching for Insights resources; search subscription-wide

**⚠️ Connectivity note:** The ARM Kusto clusters have a **Conditional Access Policy** that blocks device-code flow. `az login --use-device-code` will fail. **Workaround:** Use `azureauth --scope https://kusto.kusto.windows.net/.default --output token` which uses the Windows WAM broker through WSL interop and satisfies CAP requirements.

**Built-in ARM investigation queries:** The MCP server has an `armInvestigation` query category with pre-built queries for:
- PUT operations by resource provider (subscription health check)
- managedClusters PUT operations (addon enablement)
- Microsoft.Insights PUT/DELETE operations (DCR/DCE/DCRA creation/deletion)
- DELETE details with resource group and resource name extraction
- ContainerService operations breakdown by method and type
- ARM outgoing requests to Insights RP (AKS RP → Monitor RP calls)

**Ad-hoc query examples:**

Find all ARM operations on a specific cluster (last 30d):
```kql
tsg_query(datasource: "ARMPRODSEA", cluster: "/subscriptions/.../managedClusters/mycluster",
  kql: "HttpIncomingRequests | where TIMESTAMP > ago(30d) | where subscriptionId == '_subscriptionId' | where targetUri has '_clusterName' | project TIMESTAMP, httpMethod, httpStatusCode, targetUri, userAgent | order by TIMESTAMP desc")
```

Check if DCR/DCE/DCRA were ever created or deleted (subscription-wide):
```kql
tsg_query(datasource: "ARMPRODEUS", cluster: "/subscriptions/.../managedClusters/mycluster",
  kql: "HttpIncomingRequests | where TIMESTAMP > ago(30d) | where subscriptionId == '_subscriptionId' | where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS' | where httpMethod in ('PUT', 'DELETE') | project TIMESTAMP, httpMethod, httpStatusCode, targetUri, userAgent | order by TIMESTAMP desc")
```

Source: [ARM Data Consumer Onboarding — Kusto v2 Overview](https://eng.ms/docs/cloud-ai-platform/azure-core/azure-cloud-native-and-management-platform/control-plane-bburns/azure-resource-reporting/azure-resource-reporting/dataconsumeronboarding/armdata/kustov2/overview_prod)

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
