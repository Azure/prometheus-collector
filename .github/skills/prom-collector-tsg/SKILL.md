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
| `tsg_triage` | Initial triage: version, region, AMW config, token adapter, DCR/DCE. **Includes ⚠️ Private Cluster Check (definitive — from `ManagedClusterSnapshot.privateLinkProfile.enablePrivateCluster` boolean, NOT the FQDN) and ⚠️ Missing DCE check.** Also resolves CCP cluster_id, node pool capacity, autoscaling history, and AKS upgrade history (version timeline with resource_id fallback). |
| `tsg_errors` | Scan all error categories: container, OtelCollector, ME, MDSD, token adapter, TA, DNS, private link |
| `tsg_config` | Configuration: scrape configs, keep lists, intervals, custom config validation status/errors, custom job names from startup logs, HPA, pod/service monitors, **addon enabled check, recording rules** |
| `tsg_workload` | Workload health: CPU, memory, samples/min, drops, queue sizes, export failures. **Also includes HPA status, HPA scaling metric and oscillation analysis, HPA metric configuration, pod resource limits, target allocator distribution, exporter send failures, ME ingestion success rate, event timeline correlation, scrape samples per job over time, ME throughput by pod type, and node exporter sample count trend.** |
| `tsg_pods` | Pod restarts and health. **Includes per-pod restart detail, DaemonSet pod count by status, node status timeline, pod scheduling events, cluster autoscaler events, and cluster autoscaler scale decisions (scale-up/down with unschedulable pod detection).** |
| `tsg_logs` | Raw logs from specific component (replicaset, linux-daemonset, windows-daemonset, configreader) |
| `tsg_control_plane` | Control plane metrics config and health |
| `tsg_query` | Run arbitrary KQL against any data source (including `ARMProd` for ARM deployment logs). Accepts optional `cluster`, `timeRange`, `outputFile` (write ALL rows to CSV/JSON), `outputFormat`, and `maxRows` params |
| `tsg_dashboard_link` | Get a link to the ADX dashboard pre-filtered for the cluster |
| `tsg_metric_insights` | Analyze metric volume and cardinality — top metrics by time series count, sample rate, high-dimension cardinality, and **View All Metric Names** (full list of metric names ever ingested into the account). Requires `mdmAccountId` from `tsg_triage`. **Note:** "View All Metric Names" uses a 180-day lookback — it shows metrics that were ingested at some point, NOT that they are currently flowing. Use it to confirm a metric name exists in the account, then check `tsg_workload` or Grafana to verify current ingestion. |
| `tsg_mdm_throttling` | Check Geneva MDM QoS throttling, drops, and utilization for a monitoring account. **Requires Geneva MDM MCP server running on localhost:5050.** |
| `tsg_auth_check` | Validate credentials and connectivity to all data sources before running queries. Tests Azure credential, App Insights, Kusto clusters, and MDM MCP server. **Run this first if queries fail with 403 or connection errors.** |
| `tsg_icm_page` | Scrape ICM incident page via Edge CDP. Works on both **Windows** (localhost:9222) and **WSL2** (port proxy on 9223). Intercepts `GetIncidentDetails` and `getdescriptionentries` API responses on reload to extract the **authored summary** (`Summary` field), **discussion entries**, and **ARM resource IDs**. Requires Edge with `--remote-debugging-port=9222`. On WSL2, also needs Windows port proxy on 9223. |

All tools take `cluster` (ARM resource ID) and `timeRange` (e.g. "24h").

**Timeframe best practices:**
- **Always use the incident timeframe**, not defaults. If the ICM started 2 days ago and was mitigated yesterday, use `"48h"` or `"24h"` — not `"7d"`
- **Start narrow, widen if needed.** Use `"1h"` or `"6h"` first. Queries on busy clusters with `"7d"` frequently timeout
- **If a query times out**, retry with a shorter `timeRange` (e.g. `"1h"` instead of `"7d"`)
- **Past incidents** (>30 days old): App Insights data is gone. Only AKS/CCP Kusto queries may have data. Focus diagnosis on ICM metadata (`howFixed`, `mitigateData`, `customFields`)

**⚠️ ARM Resource ID pitfalls:**
- **Use the cluster resource group, NOT the node resource group.** ICMs often mention both. The cluster RG (e.g. `rg-myteam-prod-eus`) is what the addon telemetry uses. The node/MC RG (e.g. `MC_rg-myteam-prod-eus_mycluster_eastus`) will return **all queries empty** with no error
- **Symptom of wrong ARM ID**: every `PrometheusAppInsights` query returns "No data returned" but `AKS` queries still work (or vice versa). If you see this, check the ICM authored summary for a different ARM ID
- **Multiple ARM IDs in ICM**: ICMs often list multiple clusters or both the cluster RG and node RG. Always use the one with `Microsoft.ContainerService/managedClusters` in the path — NOT `Microsoft.Compute` or node resource groups

**⚠️ When ALL queries return empty or "No data":**
This is almost always one of these causes (check in order):
1. **Wrong ARM resource ID** — verify you're using the cluster RG, not the node RG (see above)
2. **Wrong timeframe** — incident was days/weeks ago but you're querying last 24h. Use `startTime`/`endTime` to target the incident window
3. **Addon not installed** — the cluster may not have the monitoring addon enabled (no telemetry sent at all). Check with `tsg_config` → "Addon Enabled in AKS Profile"
4. **App Insights data expired** — data older than ~30 days is purged. Only Kusto-backed data sources (AKS, AKS CCP) retain longer history
5. **VPN/auth issue** — if ALL queries across ALL data sources fail, you're likely not connected to corpnet or credentials expired


> **Detailed reference** for tsg_query, MDM account resolution, CCP cluster ID, node pool capacity, historical time ranges, versions, MetricsExtension deep-dives, and escalation contacts: see **`reference.md`** in this directory.

---

### Step 3: Identify Symptom Category and Follow TSG

Based on triage results, identify the primary symptom category and follow the corresponding TSG file in the `tsgs/` directory.

**Always check versions first** — run `tsg_triage` → "Version" (addon image tag) and "Component Versions" (ME, OTel, Golang, Prometheus). See `reference.md` → "Checking Versions and Release Notes" for details.

**TSG categories available** (each is a separate file in `tsgs/`):

| TSG Category | File |
|-------------|------|
| Pod Restarts and OOMKills | `tsgs/pod-restarts-oom.md` |
| Missing Metrics | `tsgs/missing-metrics.md` |
| Spike in Metrics Ingested | `tsgs/spike-in-metrics.md` |
| Firewall / Network / Private Link / AMPLS | `tsgs/firewall-network-private-link.md` |
| Control Plane Metrics | `tsgs/control-plane-metrics.md` |
| Windows Pod Restarts | `tsgs/windows-pod-restarts.md` |
| Remote Write Issues | `tsgs/remote-write.md` |
| Vulnerabilities / CVEs | `tsgs/vulnerabilities.md` |
| Node Exporter Missing Labels on ARM64 | `tsgs/node-exporter-arm64.md` |
| Pods Not Created / Addon Not Deploying | `tsgs/pods-not-created.md` |
| Proxy / Authenticated Proxy | `tsgs/proxy-authenticated.md` |
| Liveness Probe Failures (503) | `tsgs/liveness-probe-503.md` |
| Duplicate Label Errors (kube-state-metrics) | `tsgs/duplicate-labels-ksm.md` |
| DCR/DCE Region Mismatch or Missing | `tsgs/dcr-dce-region-mismatch.md` |
| AMW Usage Optimization | `tsgs/amw-usage-optimization.md` |
| Known Issues & FAQ | `tsgs/known-issues-faq.md` |

**For MetricsExtension (ME) deep-dives** — see `reference.md` → "Deep-Diving into MetricsExtension (ME) Issues".

**For ad-hoc KQL queries** — see `reference.md` → "Using tsg_query for Ad-Hoc Investigation".

---

### Step 4: Summarize Findings

**⚠️ Do NOT speculate.** Only state what the query data shows. If you don't know the origin or purpose of a Kubernetes resource (namespace, PodMonitor, ServiceMonitor, ConfigMap, etc.), say "I don't have data on this" rather than guessing. Common traps:
- Don't claim resources are "auto-generated" unless you have documentation or code proving it
- Don't infer intent from naming patterns (e.g., a GUID-prefixed namespace could be customer-created or platform-created — you don't know without evidence)
- If a field or behavior is ambiguous, flag the uncertainty explicitly

Present findings as:
1. **Cluster Info** — version, region, state
2. **Root Cause** — what the queries revealed, linked to TSG category
3. **Errors Found** — list of error categories with counts
4. **Configuration Issues** — any misconfigurations detected
5. **Resource Health** — CPU/memory/queue status
6. **Recommended Actions** — specific steps from the relevant TSG
7. **Escalation Path** — if issue requires another team (see Escalation Contacts below)
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

---

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
| Private link errors | `tsg_triage` (⚠️ Private Cluster Check + Missing DCE) → `tsg_errors` | Firewall / Network / Private Link |
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
| DCR/DCE wrong region or missing | `tsg_triage` + `tsg_query` (ARM) | DCR/DCE Region Mismatch or Missing (**check private link first!**) |
| Windows pod restarts | `tsg_errors` + `tsg_logs` | Windows Pod Restart |
| Remote write failures | `tsg_errors` | Remote Write |
| Metrics missing in non-default AMW | `tsg_triage` + `tsg_config` | Missing Metrics (Multi-AMW routing) |
| CVE reported | N/A | Vulnerabilities |
| ARM64 missing labels | `tsg_config` | Node Exporter Missing Labels on ARM64 |
| HPA scaled down | `tsg_workload` | Known Issues (expected behavior) |
| HPA oscillating / OOMKill feedback loop | `tsg_workload` + `tsg_errors` + `tsg_pods` | Pod Restarts and OOMKills (HPA can't scale when OOM resets memory signal) |
| Inconsistent scrape intervals | `tsg_config` + `tsg_workload` | Known Issues (cAdvisor timeout) |
| Regression after addon update | `tsg_triage` + `tsg_config` | Known Issues (post-rollout) |
| Node drain blocked | N/A | Known Issues (tolerations — fixed) |
| Metrics missing after AKS upgrade | `tsg_triage` + `tsg_scrape_health` + `tsg_workload` | Missing Metrics (AKS upgrade / node exporter) |
| TS explosion / cardinality spike | `tsg_workload` + `tsg_mdm_throttling` + `tsg_metric_insights` | Spike in Metrics (label churn / floodgate) |
| Node exporter down (up=0) | `tsg_scrape_health` + `tsg_triage` | Missing Metrics (node image / NE version) |

---

## Companion Files

| File | Contents |
|------|----------|
| `tsgs/` | 16 individual TSG files — one per symptom category + Known Issues & FAQ |
| `reference.md` | tsg_query guide, data sources, MDM/CCP resolution, versions, ME deep-dive, customer docs |
