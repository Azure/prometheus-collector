---
name: prom-collector-tsg
description: >
  Troubleshoot Azure Managed Prometheus (prometheus-collector / AMA Metrics addon) ICMs
  by running diagnostic KQL queries against ADX and App Insights.
  USE FOR: ICM investigation, customer escalation, prometheus-collector TSG,
  AMA metrics troubleshooting, managed prometheus debugging, scrape config issues,
  metrics not flowing, pod crashes, OOM, high CPU, token adapter errors,
  DCR/DCE errors, MDSD errors, ME errors, target allocator issues,
  control plane metrics, private link issues, DNS errors, liveness probe failures,
  sample drops, queue backup, metric volume analysis.
  DO NOT USE FOR: code changes, build fixes, EV2 artifacts, load testing.
argument-hint: 'Provide the ICM number or cluster ARM resource ID — e.g. "investigate ICM 12345678" or "troubleshoot cluster /subscriptions/.../managedClusters/mycluster"'
---

# Azure Managed Prometheus Troubleshooting Skill

Investigate ICMs for Azure Managed Prometheus (prometheus-collector / AMA Metrics addon)
customers by running diagnostic KQL queries from the TSG dashboard.

## Workflow

When invoked, follow this workflow:

### Step 1: Gather Context

If an **ICM number** is provided (or an ICM URL like `https://portal.microsofticm.com/imp/v5/incidents/details/{id}/summary`):

#### 1a. Use ICM MCP tools AND browser scrape (run in parallel)

Call the ICM API tools AND the browser scrape simultaneously. The API tools give structured metadata; the browser scrape gives the **authored summary** which is the richest source of context (problem description, ARM IDs, PromQL queries, evidence).

| Tool | What it returns | Reliability |
|------|----------------|-------------|
| `icm-get_incident_details_by_id` | Severity, state, owning team, custom fields, howFixed, mitigation steps, tags, time range | ✅ Always works |
| `icm-get_ai_summary` | AI-generated summary — **often contains the cluster ARM ID** quoted from the authored summary | ✅ Usually works (may say "No AI summary available") |
| `icm-get_incident_context` | AI-generated `Description`, `BriefSummary`, `DiscussionSection`, `DescriptionEntriesSummary`, symptoms, causes, mitigation, similar incidents, Kusto queries, bridge info | ⚠️ Works for ~60% of incidents; returns "Error fetching context" for others |
| `icm-get_incident_location` | Region, cluster, datacenter info | ✅ Usually works |
| `icm-get_support_requests_crisit` | Linked support requests and CritSits | ✅ Usually works |
| **`tsg_icm_page`** | **Authored summary** (full problem description, ARM IDs, AMW IDs), **discussion entries** (full thread with all context) | ✅ Works on both Windows and WSL2 when Edge is running with `--remote-debugging-port=9222` |

**⚠️ CRITICAL:** The `icm-get_incident_context` `Description` and `DescriptionEntriesSummary` fields are **AI-generated paraphrases**, NOT the original authored text. They often omit ARM IDs, specific metric names, PromQL queries, and reproduction details. Always use the browser scrape to get the real authored summary.

**When `get_incident_context` succeeds, parse these fields for the cluster ARM ID:**
- `SummarySection.Description` — AI-synthesized description (may quote the ARM ID)
- `SummarySection.DescriptionEntriesSummary` — summary of the authored description entries
- `SummarySection.Symptoms[]` — symptom descriptions
- `SummarySection.Causes` — root cause if known
- `SummarySection.MitigationSolutions` — mitigation steps taken
- `SummarySection.KustoQueries[]` — any Kusto queries referenced (may contain cluster ID)
- `DiscussionSection[]` — discussion thread entries
- `BasicInfoSection` — incident metadata

**When `get_incident_context` fails, fall back to:**
- `icm-get_ai_summary` response (AI often quotes the cluster ARM ID from the authored summary)
- `icm-get_incident_details_by_id` → scan `customFields[].StringValue`, `mitigateData`, `title`, `tags`, `howFixed`
- `icm-get_incident_location` → cluster field

**Known API limitations:**
- `icm-get_incident_details_by_id` does NOT return the `Summary` field (which IS the authored summary in the ICM portal). The raw ICM API at `prod.microsofticm.com` exposes `Summary` in `GetIncidentDetails`, but the ICM MCP tool strips it. The authored summary often has the cluster ARM ID and AMW ID right at the top
- `icm-get_incident_context` returns an AI-generated `Description` and `DescriptionEntriesSummary`, NOT the raw authored text. These may paraphrase or omit the ARM IDs
- `icm-get_ai_summary` also returns AI-generated content — it sometimes includes the ARM ID but often doesn't
- **Restricted ICMs** — some incidents have restricted access. When restricted, `get_ai_summary` returns "No AI summary available" and `get_incident_context` returns empty. Only `get_incident_details_by_id` reliably works for restricted ICMs
- **The browser scrape (step 5) is the most reliable way to get the ARM ID.** `tsg_icm_page` intercepts the raw `GetIncidentDetails` and `getdescriptionentries` API responses via CDP Network capture during page reload, extracting the full authored summary and all discussion entries

#### 1b. Finding the Cluster ARM ID

The cluster ARM resource ID is critical for running TSG queries. It looks like:
`/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ContainerService/managedClusters/{name}`

**Search order (check ALL responses from 1a):**
1. **`icm-get_incident_context`** → search `Description`, `DescriptionEntriesSummary`, `Symptoms`, `KustoQueries`, `DiscussionSection` for `/subscriptions/.../managedClusters/...` pattern
2. **`icm-get_ai_summary`** — AI summary often quotes the cluster ARM ID from the authored summary
3. **`icm-get_incident_details_by_id`** — scan ALL fields: `customFields[].StringValue`, `mitigateData`, `title`, `tags`, `howFixed`
4. **`icm-get_incident_location`** — may have cluster info
5. **Browser scrape (CRITICAL for full context)** — The ICM authored summary (`Summary` field in the raw API) is **the most important source of information** for understanding the incident. It typically contains:
   - The **cluster ARM ID** and **AMW resource ID** at the top
   - A **detailed problem description** written by the reporter — often with PromQL queries, specific metric names, timestamps, and screenshots that the AI summary omits or paraphrases
   - **Reproduction steps** and evidence (e.g. "only cAdvisor is affected, node-exporter is fine")
   
   The ICM MCP API tools (`get_incident_details_by_id`, `get_ai_summary`, `get_incident_context`) do NOT return this field. The AI-generated descriptions are often too vague to understand the real issue. **Always scrape the ICM page to get the full authored summary before starting diagnosis.**

   **Use the `tsg_icm_page` MCP tool** (works on both Windows and WSL2):
   - Call `tsg_icm_page` with the incident ID
   - This connects to a running Edge instance via CDP (Chrome DevTools Protocol)
   - **On Windows:** connects directly to `localhost:9222` — just launch Edge with `--remote-debugging-port=9222` and a unique `--user-data-dir` (see below)
   - **On WSL2:** connects via port proxy on `9223` — requires Edge on the Windows host with `--remote-debugging-port=9222` and a netsh port proxy from `0.0.0.0:9223` → `127.0.0.1:9222`
   - If Edge is not running, the tool will show platform-appropriate launch instructions
   - If sign-in is needed, the tool will prompt you to tell the user to sign in
   - The tool returns: authored summary, extracted ARM IDs, and all discussion entries

   **Windows Edge CDP quick start** (if `tsg_icm_page` fails with connection error):
   ```powershell
   # Launch a separate Edge with CDP enabled (don't reuse your main Edge profile!)
   Start-Process "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" `
     -ArgumentList "--remote-debugging-port=9222","--user-data-dir=C:\Users\$env:USERNAME\.edge-cdp-debug","--no-first-run","--disable-sync"
   # Then sign in to ICM in that Edge window
   ```
   > ⚠️ Use a **dedicated** `--user-data-dir` (not your main Edge profile). If another Edge instance already uses that profile, the new one merges into it and CDP becomes inaccessible.

   **Do NOT use Playwright MCP for ICM scraping.** The ICM portal SPA is extremely heavy — `browser_snapshot` hangs on the DOM, and the authored summary is loaded via XHR API calls (not visible in `innerText`). Playwright is fine for Grafana, Azure portal, etc. — just not ICM.

   **Important:** Read the full authored summary AND discussion entries carefully — they contain the reporter's actual problem description, specific metric names, PromQL queries, and evidence that the AI summary loses. This context is essential for targeted diagnosis.
6. **Ask the user** — if none of the above have it, ask: "What is the cluster ARM resource ID? It's usually in the ICM authored summary at: https://portal.microsofticm.com/imp/v5/incidents/details/{ICM_ID}/summary"

**STOP and ask the user if you cannot find the ARM ID after checking steps 1–5.**
Do NOT proceed to triage queries without it — every query requires the cluster ARM ID.
Do NOT guess, fabricate, or skip this step. Ask the user immediately:
> "I couldn't find the cluster ARM resource ID in the ICM details. Can you provide it?
> It looks like: `/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ContainerService/managedClusters/{name}`
> You can find it in the ICM authored summary at: https://portal.microsofticm.com/imp/v5/incidents/details/{ICM_ID}/summary"

#### 1c. Determine the Incident Time Range

**CRITICAL:** All TSG queries must target the time window when the incident was active, NOT the current time. Querying current data for a past incident will return irrelevant or empty results.

1. Extract the incident time range from `icm-get_incident_details_by_id`:
   - `impactStartTime` — when the issue started
   - `mitigateTime` (in `mitigateData`) — when the issue was mitigated
   - `createdDate` — when the ICM was filed (use as fallback start time)
2. Calculate the appropriate `timeRange` parameter:
   - For active/recent incidents (still ongoing or within last 24h): use `"24h"` or `"6h"`
   - For past incidents: the TSG tools query relative to "now", so data may no longer be available if the incident is old (App Insights retains ~30 days, Kusto clusters vary)
   - **If the incident is older than 30 days**, App Insights queries will return empty. Only Kusto-backed queries (AKS, AKS CCP, MetricInsights, AMWInfo) may still have data
3. **Start with a shorter timeRange** (e.g. `"1h"` or `"6h"`) and widen if needed. Queries with `"7d"` timeRange can timeout on busy clusters. If a query times out, retry with `"1h"` or `"24h"`

**Connection issues:** If MCP server queries fail with connection errors, timeouts, or authentication failures, **STOP and ask the user**: "Are you connected to the corp VPN? The MCP server queries require VPN access to reach internal data sources (App Insights, Kusto, AKS CCP)."

### Step 2: Run Triage Queries

**Use the `prom-collector-tsg` MCP server** which provides these tools:

| Tool | Description |
|------|-------------|
| `tsg_triage` | Initial triage: version, region, AMW config, token adapter, DCR/DCE. **Also resolves CCP cluster_id, node pool capacity, and autoscaling history.** |
| `tsg_errors` | Scan all error categories: container, OtelCollector, ME, MDSD, token adapter, TA, DNS, private link |
| `tsg_config` | Configuration: scrape configs, keep lists, intervals, custom config validation status/errors, custom job names from startup logs, HPA, pod/service monitors, **addon enabled check, recording rules** |
| `tsg_workload` | Workload health: CPU, memory, samples/min, drops, queue sizes, export failures. **Also includes HPA status, pod resource limits, target allocator distribution, exporter send failures, ME ingestion success rate, and event timeline correlation.** |
| `tsg_pods` | Pod restarts and health. **Includes per-pod restart detail, DaemonSet pod count by status, node status timeline, pod scheduling events, and cluster autoscaler events.** |
| `tsg_logs` | Raw logs from specific component (replicaset, linux-daemonset, windows-daemonset, configreader) |
| `tsg_control_plane` | Control plane metrics config and health |
| `tsg_query` | Run arbitrary KQL against any data source |
| `tsg_dashboard_link` | Get a link to the ADX dashboard pre-filtered for the cluster |
| `tsg_metric_insights` | Analyze metric volume and cardinality — top metrics by time series count, sample rate, high-dimension cardinality, and **View All Metric Names** (full list of metric names ever ingested into the account). Requires `mdmAccountId` from `tsg_triage`. **Note:** "View All Metric Names" uses a 180-day lookback — it shows metrics that were ingested at some point, NOT that they are currently flowing. Use it to confirm a metric name exists in the account, then check `tsg_workload` or Grafana to verify current ingestion. |
| `tsg_mdm_throttling` | Check Geneva MDM QoS throttling, drops, and utilization for a monitoring account. **Requires Geneva MDM MCP server running on localhost:5050.** |
| `tsg_icm_page` | Scrape ICM incident page via Edge CDP. Works on both **Windows** (localhost:9222) and **WSL2** (port proxy on 9223). Intercepts `GetIncidentDetails` and `getdescriptionentries` API responses on reload to extract the **authored summary** (`Summary` field), **discussion entries**, and **ARM resource IDs**. Requires Edge with `--remote-debugging-port=9222`. On WSL2, also needs Windows port proxy on 9223. |

All tools take `cluster` (ARM resource ID) and `timeRange` (e.g. "24h").

**Timeframe best practices:**
- **Always use the incident timeframe**, not defaults. If the ICM started 2 days ago and was mitigated yesterday, use `"48h"` or `"24h"` — not `"7d"`
- **Start narrow, widen if needed.** Use `"1h"` or `"6h"` first. Queries on busy clusters with `"7d"` frequently timeout
- **If a query times out**, retry with a shorter `timeRange` (e.g. `"1h"` instead of `"7d"`)
- **Past incidents** (>30 days old): App Insights data is gone. Only AKS/CCP Kusto queries may have data. Focus diagnosis on ICM metadata (`howFixed`, `mitigateData`, `customFields`)

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

### Step 3: Identify Symptom Category and Follow TSG

Based on triage results, identify the primary symptom category and follow the corresponding TSG.
TSG source: https://dev.azure.com/msazure/InfrastructureInsights/_wiki/wikis/InfrastructureInsights.wiki?pagePath=/ManagedPrometheus/OnCall/TSGs

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

---

### Step 4: Summarize Findings

Present findings as:
1. **Cluster Info** — version, region, state
2. **Root Cause** — what the queries revealed, linked to TSG category
3. **Errors Found** — list of error categories with counts
4. **Configuration Issues** — any misconfigurations detected
5. **Resource Health** — CPU/memory/queue status
6. **Recommended Actions** — specific steps from the relevant TSG
7. **Escalation Path** — if issue requires another team (see below)
8. **Dashboard Link** — provide the direct link:
   `https://dataexplorer.azure.com/dashboards/94da59c1-df12-4134-96bb-82c6b32e6199?p-_cluster=v-{CLUSTER_ARM_ID_URL_ENCODED}`
9. **Reference Documentation** — search the learn.microsoft.com doc trees below for the most relevant page based on the customer's specific issue. Use `web_search` or `web_fetch` to find the right sub-page (e.g., custom scrape config, remote write, troubleshooting). Do NOT just link the overview — find and link the specific doc page that addresses the customer's problem:
   - TOC root: [Azure Managed Prometheus](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/prometheus-metrics-overview) — covers configuration, collection, scrape configs, remote write
   - TOC root: [Kubernetes monitoring](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-overview) — covers AKS addon setup, troubleshooting, managed Grafana

### Step 5: Improve the Tooling

After each investigation, if you wrote any **ad-hoc KQL queries via `tsg_query`** that were useful for diagnosis, **add them to the MCP server** so future investigations benefit:

1. Identify which ad-hoc queries produced actionable results during the investigation
2. Add the query to the appropriate category in `tools/prom-collector-tsg-mcp/src/queries.ts`
3. Wire it into the relevant tool in `tools/prom-collector-tsg-mcp/src/index.ts`
4. Rebuild: `cd tools/prom-collector-tsg-mcp && npx tsc`

This ensures the tooling continuously improves — every investigation makes the next one faster.

## Escalation Contacts

| Issue/Area | ICM Team |
|------------|----------|
| AMW Quota increases | Geneva Monitoring/MDM-Support-Manageability-Tier2 |
| Query throttling (429 in Grafana) | Azure Monitor Essentials/Sev3 and 4 CRI – Metrics |
| Remote-write errors (500, 4xx) | Geneva Monitoring/Ingestion Gateway Support - Tier 2 |
| ARC Kubernetes ingestion | Container Insights/AzureManagedPrometheusAgent |
| Prometheus Recording rules & alerts | Azure Log Search Alerts/Prometheus Alerts |
| Grafana service issues | Azure Managed Grafana/Triage |
| AMW RP issues | Azure Monitor Control Service/Triage |
| AMCS (DCR/DCE/DCRA) | Azure Monitor Control Service/Triage |
| MDM Store | Geneva Monitoring/MDM-Support-Core-IngestionAndStorage-Tier2 |
| AKS addon/ARM/Policy/Bicep/Terraform | Container Insights/AzureManagedPrometheusAgent |

## Quick Reference

| Symptom | MCP Tool | TSG Category |
|---------|----------|--------------|
| No metrics flowing | `tsg_triage` + `tsg_errors` + `tsg_mdm_throttling` | Missing Metrics |
| Account throttling / drops | `tsg_mdm_throttling` | Missing Metrics (MDM quota) |
| Pod CrashLoopBackOff / OOM | `tsg_errors` + `tsg_workload` | Pod Restarts and OOMKills |
| High CPU/Memory | `tsg_workload` | Pod Restarts / Resource Consumption |
| Partial metrics / drops | `tsg_workload` + `tsg_mdm_throttling` | Missing Metrics (ME queue or MDM throttle) |
| Config not applied / invalid | `tsg_config` | Missing Metrics (custom config) |
| Config validation failed | `tsg_config` | Missing Metrics (check "Custom Config Validation Errors") |
| Private link errors | `tsg_errors` | Firewall / Network / Private Link |
| TokenConfig.json missing / ME won't start | `tsg_errors` + `tsg_logs` | Firewall / Network (AMCS blocked) |
| ARC cluster pod restarts | `tsg_errors` + `tsg_logs` | Firewall / Network (ARC/Azure Local) |
| Proxy / auth proxy issues | `tsg_errors` + `tsg_config` | Proxy / Authenticated Proxy |
| Target allocator errors | `tsg_errors` | Pod Restarts (operator-targets) |
| Token/auth errors | `tsg_errors` | Missing Metrics (auth issues) |
| Liveness probe 503 | `tsg_errors` | Liveness Probe Failures |
| Control plane metrics missing | `tsg_control_plane` | Control Plane Metrics |
| Spike in ingestion | `tsg_workload` + `tsg_config` + `tsg_metric_insights` + `tsg_mdm_throttling` | Spike in Metrics Ingested |
| High cardinality / volume | `tsg_metric_insights` | Spike in Metrics (cardinality) |
| AMW cost optimization | `tsg_metric_insights` + `tsg_config` | AMW Usage Optimization |
| Pods not created | `tsg_triage` | Pods Not Created / Addon Not Deploying |
| Duplicate label errors | `tsg_config` | Duplicate Labels (kube-state-metrics) |
| DCR/DCE wrong region | `tsg_triage` | DCR/DCE Region Mismatch |
| Windows pod restarts | `tsg_errors` + `tsg_logs` | Windows Pod Restart |
| Remote write failures | `tsg_errors` | Remote Write |
| Metrics missing in non-default AMW | `tsg_triage` + `tsg_config` | Missing Metrics (Multi-AMW routing) |
| CVE reported | N/A | Vulnerabilities |
| ARM64 missing labels | `tsg_config` | Node Exporter Missing Labels on ARM64 |
| HPA scaled down | `tsg_workload` | Known Issues (expected behavior) |
| Inconsistent scrape intervals | `tsg_config` + `tsg_workload` | Known Issues (cAdvisor timeout) |
| Regression after addon update | `tsg_triage` + `tsg_config` | Known Issues (post-rollout) |
| Node drain blocked | N/A | Known Issues (tolerations — fixed) |

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
