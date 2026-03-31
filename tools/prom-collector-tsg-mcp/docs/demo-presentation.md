# Azure Managed Prometheus — ICM Troubleshooting with AI

## Copilot CLI + MCP Server + TSG Skill

**Grace Wehner** · Container Insights / AzureManagedPrometheusAgent

---

## The Problem

**ICM triage for prometheus-collector is slow and painful:**

- 🕐 Manually running KQL queries across 6+ Kusto clusters
- 🔀 Switching between ADX dashboards, Azure Portal, ICM, Grafana
- 📋 Copy-pasting cluster ARM IDs, MDM account names, subscription GUIDs
- 🧠 Remembering which query goes to which data source
- 📖 Looking up the right TSG for each symptom pattern
- ⏱️ A typical investigation takes **30-60 minutes** of manual query work

---

## The Bigger Problem: Combined On-Call and Knowledge Gaps

We recently moved to **combined on-call across the team** — everyone rotates through all areas, not just their specialty. This means:

- The person on-call for a **private link / DCE issue** might be an expert in **remote write** — they've never debugged a missing DCE before
- An **OOMKill on the replicaset** might land on someone who primarily works on **control plane metrics** — they don't know that system pool VM size matters, not user pools
- A **cardinality spike** investigation requires understanding MDM internals that only 1-2 people on the team have ever touched

**The SME knowledge problem:** Every experienced engineer has built up intuition from dozens of investigations — they know what to check first, which errors are red herrings, which queries to run for which symptoms, and what the fix usually is. **That knowledge lives in their heads, not in a system.**

When a non-SME gets an ICM outside their area:
1. They read the TSG doc, but it's generic — doesn't tell them *which specific queries* to run
2. They try the ADX dashboard, but don't know how to interpret the results
3. They Slack the SME (who may be asleep / on vacation / in a different timezone)
4. **Mean time to resolution goes up. Customer experience suffers.**

### The Vision: Encode SME Knowledge Into the Tooling

**What if we could capture the intuition and investigation patterns of every SME — and make it available to everyone on the team, 24/7?**

That's the core idea behind this skill + MCP server approach:

| What SMEs Know | How We Capture It |
|---------------|-------------------|
| "If it's a private cluster, check DCE first" | **Skill routing**: Private link symptom → `tsg_triage` runs Private Cluster Check (definitive) → Missing DCE check — automatically, in the right order |
| "OOMKills create an HPA feedback loop — check minshards" | **TSG document**: `tsgs/pod-restarts-oomkills.md` has the exact diagnostic workflow + fix |
| "That MDSD error means AMCS can't serve config over private link" | **Error pattern matching**: `tsg_errors` detects the specific MDSD error string → skill routes to private link TSG |
| "Check ARM for whether the DCR was recently deleted" | **Built-in queries**: `tsg_triage` runs DCRA Operations, DCE Operations, Failed Operations against 3 regional ARM clusters automatically |
| "The CCP cluster ID resolution is flaky — use subscription+name instead" | **Fallback queries**: MCP server has CCP-independent queries using `ManagedClusterSnapshot` directly |
| "For cardinality, look at the per-dimension breakdown, not just total TS count" | **MetricInsights queries**: Per-Dimension Cardinality Breakdown, Risk-Rated Value Counts — built from real investigation patterns |

### How We Avoid Context Overload

The skill is designed in **layers** to avoid overloading the initial context:

```
┌─────────────────────────────────────────────┐
│  SKILL.md (loaded first)                    │  ← Workflow + routing table
│  • 5-step investigation workflow             │     (~300 lines)
│  • Symptom → Tool → TSG routing table        │
│  • Tool descriptions + parameters            │
│  • Escalation contacts                       │
└──────────────────┬──────────────────────────┘
                   │ References (loaded on demand)
         ┌─────────┼──────────┐
         ▼         ▼          ▼
   ┌──────────┐ ┌─────┐ ┌──────────┐
   │ tsgs/    │ │ref  │ │ MCP      │
   │ 16 files │ │.md  │ │ Server   │
   │ (on      │ │(on  │ │ (175     │
   │ demand)  │ │need)│ │ queries) │
   └──────────┘ └─────┘ └──────────┘
```

- **SKILL.md** is the entry point — small enough to fit in context, has the routing logic to know *which* TSG to pull in
- **Individual TSGs** (`tsgs/*.md`) are loaded only when the symptom matches — not all 16 at once
- **reference.md** has deep technical details (data sources, version checking, ME deep-dive) — loaded only for specific investigation needs
- **MCP server queries** run server-side — the 175+ KQL queries never need to be in the LLM context at all

**Result:** A non-SME on-call can type `investigate ICM 12345678` and get the same diagnostic path that the most experienced engineer on the team would follow — including the right queries, the right interpretation, and the right TSG, without needing to know any of it upfront.

---

## Why Is This So Hard?

**We don't have access to the customer's cluster — we can only see what our telemetry collects.**

The prometheus-collector addon runs as a container on the customer's AKS cluster. When something goes wrong, the issue could be in **any of 6+ independent data sources**, and we have to piece together the story from scattered telemetry:

| Data Source | What It Tells Us | Example Questions |
|-------------|-----------------|-------------------|
| **App Insights** (our container telemetry) | Logs, errors, and metrics emitted by our collector containers running on the cluster | Is the OTel collector crashing? Are there MDSD auth errors? Is MetricsExtension dropping samples? |
| **AKS Kusto** (Kubernetes platform) | Pod CPU/memory, restarts, OOMKills, cluster health, node pool capacity, upgrade history | Is the pod being OOMKilled? Is the node pool out of resources? Did an AKS upgrade break things? |
| **AMCS / AMWInfo** (control plane config) | Azure Monitor Workspace, DCR, DCE, DCRA associations — the "wiring" between cluster and AMW | Does a DCR exist? Is there a DCE for this private cluster? Was the DCRA deleted? |
| **ARM** (resource operations) | Creation, deletion, and modification of AMW/DCR/DCE/DCRA resources — the deployment history | Was the DCR recently deleted? Did a DCE creation fail? When was the AMW provisioned? |
| **Geneva MDM** (metrics pipeline) | AMW account throttling, ingestion rate, time series counts, cardinality, metric names ingested | Is the account being throttled? Which metrics are causing cardinality explosion? Are samples being dropped at ingestion? |
| **Our Config Telemetry** | Default scrape targets enabled, custom scrape configs, keep lists, scrape intervals, pod/service monitors | What targets are they scraping? Is their custom config valid? Did they enable control plane metrics? |

**A single ICM can require queries across ALL of these** — and the data lives in 11 different Kusto clusters, App Insights, and Geneva MDM. Without tooling, you're manually:
- Resolving the CCP cluster ID from the ARM resource ID
- Finding the right MDM account name for the AMW
- Copy-pasting the subscription GUID into ARM queries across 3 regional clusters
- Remembering which KQL table has the column you need
- Cross-referencing timestamps across data sources to build a timeline

**This is the problem the MCP server solves** — one command, all data sources, correlated results.

---

## What We Had Before: The ADX Dashboard

We built a **TSG ADX Dashboard** to consolidate our most common queries. You paste in the cluster ARM resource ID and it runs everything:

```
ARM Resource ID ──► ADX Dashboard
                        │
                        ├─► Resolves AKS internal CCP cluster ID
                        ├─► Finds the AMW associated with that cluster
                        ├─► Looks up the internal MDM account ID for that AMW
                        ├─► Runs 50+ diagnostic queries across data sources
                        └─► Displays results in panels
```

**The key insight:** every data source uses a **different identifier** for the same cluster:

| Data Source | Key It Needs | How We Get It |
|-------------|-------------|---------------|
| App Insights | ARM resource ID | Given by the customer / ICM |
| AKS Kusto (CCP) | Internal hex cluster ID | Resolved from `ManagedClusterMonitoring` (sparse — ~6h apart) |
| AKS Kusto (Infra) | ARM resource ID or subscription + cluster name | Extracted from ARM ID |
| AMWInfo | Subscription GUID | Extracted from ARM ID |
| MetricInsights | MDM account name (e.g. `mac_0d8947c8...`) | Looked up from AMW association |
| ARM Kusto | Subscription GUID + target URI patterns | Extracted from ARM ID, queries 3 regional clusters |
| Geneva MDM | MDM monitoring account name | Same as MetricInsights account |

The dashboard helped — it handles the ID resolution chain automatically. **But it still has significant gaps:**

### Dashboard Limitations

| Problem | Impact |
|---------|--------|
| **ADX dashboards can't query Geneva MDM** | For throttling, drops, and cardinality — you have to open **Jarvis** (Geneva portal) separately and paste in the MDM account name |
| **ADX dashboards can't query ARM deployment history** | For DCR/DCE/DCRA creation and deletion — you have to open **ARMProd Kusto** in a separate ADX tab across 3 regional clusters |
| **No access to AKS Service Insights** | AKS uses the **Azure Service Insights** portal for deep cluster diagnostics (node health, upgrade status, control plane events) — completely separate from ADX |
| **No ICM context integration** | You still have to read the ICM, extract ARM IDs, understand the symptom — then switch to the dashboard |
| **Static panels, no intelligence** | The dashboard shows data but doesn't interpret it — you still have to visually scan every panel, correlate timestamps yourself, and know which TSG to follow |
| **No custom query support** | When the pre-built panels don't answer your question, you have to open yet another ADX tab for ad-hoc queries |

**Result: you're juggling 3-4 web pages during every investigation:**

```
┌──────────┐  ┌──────────────┐  ┌─────────────┐  ┌───────────────────┐
│   ICM    │  │ ADX Dashboard│  │   Jarvis    │  │ Service Insights  │
│  Portal  │  │  (our KQL)   │  │  (MDM QoS)  │  │   (AKS portal)    │
└────┬─────┘  └──────┬───────┘  └──────┬──────┘  └────────┬──────────┘
     │               │                 │                   │
     └───────────────┴─────────────────┴───────────────────┘
                    Manual context switching
              Copy-paste IDs between every page
           No correlation — you build the timeline in your head
```

**The MCP server eliminates all of this** — every data source, every ID resolution, every TSG, in one terminal.

---

## The Solution

**An MCP server that gives Copilot CLI direct access to all our diagnostic data sources.**

```
┌─────────────────────────────────────┐
│         Copilot CLI (Terminal)       │
│                                     │
│  "Investigate ICM 12345678"         │
│  "Why are pods restarting on        │
│   cluster /subscriptions/..."       │
└──────────────┬──────────────────────┘
               │
    ┌──────────▼──────────┐
    │   TSG Skill (SKILL.md)   │  ← Routing logic + TSG knowledge
    │   16 TSGs + Reference    │
    └──────────┬──────────┘
               │
    ┌──────────▼──────────┐
    │  MCP Server (Node.js)    │  ← 14 tools, 175 KQL queries
    │  prom-collector-tsg-mcp  │
    └──────────┬──────────┘
               │
    ┌──────────▼──────────────────────────────────┐
    │  Data Sources                                │
    │                                              │
    │  PrometheusAppInsights  (collector telemetry)│
    │  AKS / AKS CCP / Infra  (cluster state)     │
    │  AMWInfo                 (DCR/AMW mapping)   │
    │  MetricInsights          (cardinality/volume)│
    │  ARMProd (3 regions)     (deployment history)│
    │  Geneva MDM              (QoS/throttling)    │
    │  ICM Portal              (CDP browser scrape)│
    └──────────────────────────────────────────────┘
```

---

## What's in the MCP Server

### 14 Diagnostic Tools

| Tool | What it does |
|------|-------------|
| **`tsg_triage`** | Initial triage: addon version, region, AMW/DCR/DCE config, token adapter health, CCP cluster ID, node pool capacity & autoscaling, AKS upgrade history, Missing DCE check for private clusters |
| **`tsg_errors`** | Scans ALL error categories: ContainerLog, OtelCollector, MetricsExtension, MDSD, AddonTokenAdapter, TargetAllocator, ConfigReader, DNS resolution, Private Link, DCR/AMCS config, Liveness Probe |
| **`tsg_config`** | Scrape configs (RS/DS/Win), custom config validation + errors, keep lists, scrape intervals, HPA, pod/service monitors, recording rules, addon enabled check, OTLP metrics, cluster alias/label, KSM allow lists |
| **`tsg_workload`** | Replica count, samples/min, drops (OTel & ME), P95 CPU/memory per container, queue sizes, export failures, HPA oscillation analysis, pod resource limits, TA distribution, ME ingestion success rate, event timeline, per-job scrape samples, node exporter trends |
| **`tsg_pods`** | Pod restarts & reasons (OOM, liveness), per-pod detail, DaemonSet pod status, pod-to-node mapping, system pool node resources, node status timeline, pod scheduling events, cluster autoscaler events |
| **`tsg_logs`** | Raw logs from replicaset, linux-daemonset, windows-daemonset, configreader |
| **`tsg_control_plane`** | Control plane metrics enabled status, jobs config, metrics keep list, configmap watcher logs, container restarts, max CPU by container |
| **`tsg_query`** | Ad-hoc KQL against any of 11 data sources (including 3 regional ARM clusters) — supports token replacement, write results to CSV/JSON |
| **`tsg_metric_insights`** | Top metrics by TS count & sample rate, full volume summary, top 20 cardinality, high-dimension detection, volume by category (Istio/Envoy/Container/NodeExporter/KSM), all metric names (180-day lookback) |
| **`tsg_mdm_throttling`** | Geneva MDM QoS: ThrottledClientMetricCount, DroppedClientMetricCount, ThrottledTimeSeriesCount, MStoreDroppedSamplesCount, active TS vs limits, throttled queries |
| **`tsg_scrape_health`** | Per-job scrape target health from MDM — `up` metric success/failure by bucket, relabeling drop rate, all common jobs probe |
| **`tsg_icm_page`** | CDP browser scrape of ICM page — extracts authored summary, discussion entries, ARM resource IDs (works Windows + WSL2) |
| **`tsg_dashboard_link`** | Direct link to ADX dashboard pre-filtered for cluster |
| **`tsg_auth_check`** | Validates credentials + connectivity to all data sources, auto-fixes token issues, detects ARMProd CAP blocks |

### By the Numbers

| Metric | Count |
|--------|-------|
| MCP tools | 14 |
| KQL queries | 171+ |
| Query categories | 9 |
| Data sources | 11 Kusto clusters + App Insights + Geneva MDM |
| TSG documents | 16 |
| Symptom→Tool mappings | 30+ |
| Lines of TypeScript | ~5,500 |

---

## All 165 Queries Across 9 Categories

### Triage (27 queries)
Version • Component Versions (ME, OTel, Golang, Prometheus) • Cluster Region • AKS Cluster ID • Azure Monitor Workspace • MDM Account ID • MDM Stamp • AMW Region • Internal DCE/DCR Ids • Missing DCE for Private Cluster (AMCS 403) • Token Adapter Health • DCRs Associated with Cluster • AMW(s) from Scrape Config Routing • AMW(s) • AMW(s) in Subscription (fallback) • AKS Network Settings • AKS Addons Enabled • AKS Cluster Settings • AKS Cluster State • CCP Cluster ID • CCP Cluster ID (AgentPoolSnapshot fallback) • Node Pool Capacity • Node Conditions (Memory/Disk/PID Pressure) • Node Allocatable Resources • AgentPool Autoscaling History • AKS Upgrade History • Node Pool Versions (resource_id fallback)

### Errors (12 queries)
DCR/DCE/AMCS Configuration Errors • ContainerLog Errors • OtelCollector Errors • MetricsExtension Errors • MDSD Errors • AddonTokenAdapter Errors • TargetAllocator Errors • ConfigReader Errors • DNS Resolution Issues • Private Link Issues • Private Link Issues by Nodepool/Node/Pod • Liveness Probe Logs

### Config (28 queries)
Invalid Custom Prometheus Config • RS/DS/Win Scrape Configs Enabled • HPA Enabled • Debug Mode • HTTP Proxy • RS ConfigMap Jobs • Custom Config Validation Status/Errors/YAML Error Lines/OTel Loading Errors • Custom Scrape Jobs from Startup Logs • RS PodMonitors/ServiceMonitors • Default Targets KeepList/Scrape Interval • Minimal Ingestion Profile • OTLP Metrics Enabled • Cluster Alias/Label • ConfigMap Version • Pod Annotations Namespace Regex • RS Targets Discovered per Job • KSM Labels/Annotations Allow Lists • Recording Rules • Addon Enabled in AKS Profile

### Workload (58 queries)
Replica/DaemonSet Count • Samples/Min (total, per-replica, per-pod, per-account) • Samples Dropped (RS ME, DS) • P95 CPU/Memory per Container (OTel, ME, ConfigReloader, TargetAllocator) • ME Queue Size • OTel Queue Size (RS/DS) • OTel Export Failures (RS/DS) • OTel Receiver Metrics Refused (RS/DS) • Collectors Discovered • Scrape Jobs • Targets Per Replica • Unassigned Targets • SD HTTP Failures • TA Error Count • KSM Version • HPA Status/Scaling Metric/Oscillation/Metric Config • Cluster Autoscaler Scale Decisions/Unschedulable Count • Pod Resource Limits • TA Distribution • Exporter Send Failures • ME Ingestion Success Rate • Event Timeline (Config/Restarts/Errors) • DaemonSet Per-Pod Sample Variance/Distribution • Scrape Samples Per Job Over Time • ME Throughput by Pod Type • Node Exporter Sample Count Trend • Node Pools • System Nodepool Nodes Status • Total P95 CPU/Memory per Replica

### Pods (10 queries)
Latest Pod Restarts • Pod Restarts During Interval • AKS Addon Pod Restart Count/Reason • Pod Restart Detail by Pod • DaemonSet Pod Count by Status • Pod to Node Mapping • System Pool Node Resources • Node Status Timeline • Pod Schedule Events • Cluster Autoscaler Events

### Logs (4 queries)
All ReplicaSet Logs • All Linux DaemonSet Logs • All Windows DaemonSet Logs • All ConfigReader Logs

### Control Plane (8 queries)
Enabled • Jobs Enabled • Metrics KeepList • Minimal Ingestion Profile • Configmap Watcher Logs • Prometheus-Collector Stdout Logs • Container Restarts • Max CPU by Container

### Metric Insights (11 queries)
Top Metrics by TS Count • Top Metrics by Sample Rate • Full Metric Volume Summary • Total TS and Events Summary • Top 20 Highest Cardinality Metrics • Metrics with High Dimension Cardinality • Volume by Category (Istio/Envoy/Container/NodeExporter/KSM/ScrapeHealth) • View All Metric Names (180-day lookback) • **Per-Dimension Cardinality Breakdown (Top 10 Metrics)** • **Cardinality Trend Over Time (Top 5 Metrics, 30d)** • **Metric Dimension Names and Risk-Rated Value Counts**

### ARM Investigation (13 queries)
ARM PUT Operations by Resource Provider • Managed Clusters PUT Operations (Addon Enablement) • Microsoft.Insights PUT/DELETE (DCR/DCE/DCRA) • Microsoft.Insights DELETE Details • ContainerService Operations Breakdown • ARM Outgoing Requests to Insights RP • All Operations on Specific Cluster • All Subscription DELETEs on Microsoft.Insights • AMW All Operations • AMW PUT/DELETE Operations • **DCRA Operations for Cluster (dataCollectionRuleAssociations)** • **DCRA Failed Operations (4xx/5xx errors)** • **DCE Operations in Subscription (dataCollectionEndpoints)**

---

## 11 Data Sources

| Data Source | What it provides |
|-------------|-----------------|
| **PrometheusAppInsights** | Collector telemetry — logs, configs, error messages, scrape validation, version info, samples/min. **Primary source for most queries.** |
| **AKS** | Cluster state — version, addon status, network settings, node pools, VM sizes, autoscaler config |
| **AKS CCP** | Control plane — configmap watcher logs, control plane metrics, jobs, keep lists, container restarts |
| **AKS Infra** | Infrastructure — control plane pod CPU, container restart counts |
| **AMWInfo** | AMW/DCR mapping — cluster→AMW→DCR→MDM account resolution, subscription-level AMW discovery |
| **MetricInsights** | Cardinality — time series counts, sample rates, metric names, volume by category (180-day lookback) |
| **ARMPRODSEA** | ARM ops (Asia/Pacific/UK/Africa) — DCR/DCE/DCRA creation/deletion, addon enablement logs |
| **ARMPRODEUS** | ARM ops (Americas) — same as above for US regions |
| **ARMPRODWEU** | ARM ops (Europe) — same as above for EU regions |
| **Vulnerabilities** | CVE scanning — container image vulnerability information |
| **Geneva MDM** | QoS metrics — throttling, drops, time series limits, active TS vs account limits |

---

## The Skill Layer

The **TSG Skill** (`.github/skills/prom-collector-tsg/`) teaches Copilot *how* to investigate:

### 5-Step Workflow
1. **Gather Context** — Scrape ICM page + call ICM API in parallel, extract cluster ARM ID and incident time range
2. **Run Triage** — `tsg_triage` → version, region, DCR/AMW, node pools, private cluster check, Missing DCE
3. **Identify Symptoms** — Match error patterns to one of 16 TSGs using symptom→tool routing table
4. **Deep Dive** — Follow TSG-specific tool sequence (errors → workload → logs → config → ARM)
5. **Summarize** — Root cause, error counts, fix steps, escalation path, dashboard link, customer doc link

### 16 Troubleshooting Guides

| # | TSG | What it covers |
|---|-----|---------------|
| 1 | **Missing Metrics** | Metrics fail to flow — scrape failures, token/auth errors, config issues, ME ingestion, MDM throttling, multi-AMW routing |
| 2 | **Pod Restarts / OOMKills** | Crash loops, OOM kills, HPA feedback loops, system node pool capacity, memory pressure |
| 3 | **Spike in Metrics Ingested** | Sudden volume increase, cardinality explosion, label churn, Istio/Envoy proliferation, cost impact |
| 4 | **Firewall / Network / Private Link / AMPLS** | AMCS access blocked, DNS failures, private link config, AMPLS, Missing DCE for private clusters |
| 5 | **Proxy / Authenticated Proxy** | HTTP/HTTPS proxy config, auth proxy errors, bypass rules, env var validation |
| 6 | **DCR/DCE Region Mismatch** | DCR/DCE region validation, missing DCE, multi-region config, ARM deployment errors |
| 7 | **Duplicate Labels (KSM)** | kube-state-metrics label conflicts, allow list config, label deduplication |
| 8 | **Liveness Probe Failures (503)** | ME health check failures, startup delays, service availability |
| 9 | **Pods Not Created / Addon Not Deploying** | Deployment failures, pod scheduling, quota/capacity, node pool constraints |
| 10 | **Remote Write Issues** | Endpoint connectivity, auth failures, ingestion gateway errors (500/4xx), batch config |
| 11 | **Control Plane Metrics** | Control plane collection status, job config (apiserver, etcd, scheduler, controller-manager) |
| 12 | **Node Exporter ARM64** | ARM64 label scraping, node exporter version compat, metric filtering |
| 13 | **Windows Pod Restarts** | Windows-specific failures, Windows DaemonSet, OS-specific logging |
| 14 | **Vulnerabilities / CVEs** | Security scanning, image CVEs, patch availability, remediation |
| 15 | **AMW Usage Optimization** | Cost optimization, volume reduction, cardinality management, ingestion profile tuning, metric relabeling |
| 16 | **Known Issues & FAQ** | Post-rollout regressions, expected behaviors (HPA scale-down), AKS upgrade compat, common misconceptions |

### Complete Symptom → Tool Routing Table

| Symptom | Tools | TSG |
|---------|-------|-----|
| No metrics flowing | `tsg_triage` → `tsg_errors` → `tsg_mdm_throttling` | Missing Metrics |
| Account throttling / drops | `tsg_mdm_throttling` | Missing Metrics (MDM quota) |
| Pod CrashLoopBackOff / OOM | `tsg_errors` → `tsg_workload` → `tsg_pods` | Pod Restarts / OOM |
| High CPU / Memory | `tsg_workload` | Pod Restarts / Resources |
| Partial metrics / drops | `tsg_workload` → `tsg_mdm_throttling` | Missing Metrics (ME queue or MDM) |
| Config not applied / invalid | `tsg_config` | Missing Metrics (custom config) |
| Config validation failed | `tsg_config` | Missing Metrics (validation errors) |
| Private link errors | `tsg_triage` (DCE check) → `tsg_errors` | Firewall / Private Link |
| TokenConfig.json missing / ME won't start | `tsg_errors` → `tsg_logs` | Firewall (AMCS blocked) |
| ARC cluster pod restarts | `tsg_errors` → `tsg_logs` | Firewall (ARC/Azure Local) |
| Proxy / auth proxy issues | `tsg_errors` → `tsg_config` | Proxy |
| Target allocator errors | `tsg_errors` | Pod Restarts (operator-targets) |
| Token / auth errors | `tsg_errors` | Missing Metrics (auth) |
| Liveness probe 503 | `tsg_errors` | Liveness Probe Failures |
| Control plane metrics missing | `tsg_control_plane` | Control Plane |
| Spike in ingestion | `tsg_workload` → `tsg_config` → `tsg_metric_insights` → `tsg_mdm_throttling` | Spike in Metrics |
| High cardinality / volume | `tsg_metric_insights` | Spike (cardinality) |
| AMW cost optimization | `tsg_metric_insights` → `tsg_config` | AMW Optimization |
| Pods not created | `tsg_triage` | Pods Not Created |
| Duplicate label errors | `tsg_config` | Duplicate Labels (KSM) |
| DCR/DCE wrong region or missing | `tsg_triage` → `tsg_query` (ARM) | DCR/DCE Mismatch |
| Windows pod restarts | `tsg_errors` → `tsg_logs` | Windows |
| Remote write failures | `tsg_errors` | Remote Write |
| Metrics missing in non-default AMW | `tsg_triage` → `tsg_config` | Missing Metrics (Multi-AMW) |
| CVE reported | N/A | Vulnerabilities |
| ARM64 missing labels | `tsg_config` | Node Exporter ARM64 |
| HPA scaled down unexpectedly | `tsg_workload` | Known Issues |
| HPA oscillating / OOMKill loop | `tsg_workload` → `tsg_errors` → `tsg_pods` | Pod Restarts / OOM |
| Inconsistent scrape intervals | `tsg_config` → `tsg_workload` | Known Issues (cAdvisor) |
| Regression after addon update | `tsg_triage` → `tsg_config` | Known Issues (post-rollout) |
| Metrics missing after AKS upgrade | `tsg_triage` → `tsg_scrape_health` → `tsg_workload` | Missing Metrics (upgrade) |
| TS explosion / cardinality spike | `tsg_workload` → `tsg_mdm_throttling` → `tsg_metric_insights` | Spike (label churn) |
| Node exporter down (up=0) | `tsg_scrape_health` → `tsg_triage` | Missing Metrics (NE version) |

### Escalation Contacts

| Issue | ICM Team |
|-------|----------|
| AMW Quota increases | Geneva Monitoring / MDM-Support-Manageability-Tier2 |
| Query throttling (429 in Grafana) | Azure Monitor Essentials / Sev3 and 4 CRI – Metrics |
| Remote-write errors (500, 4xx) | Geneva Monitoring / Ingestion Gateway Support - Tier 2 |
| ARC Kubernetes ingestion | Container Insights / AzureManagedPrometheusAgent |
| Prometheus Recording rules & alerts | Azure Log Search Alerts / Prometheus Alerts |
| Grafana service issues | Azure Managed Grafana / Triage |
| AMW RP / AMCS (DCR/DCE/DCRA) | Azure Monitor Control Service / Triage |
| MDM Store | Geneva Monitoring / MDM-Support-Core-IngestionAndStorage-Tier2 |
| AKS addon / ARM / Policy / Bicep / Terraform | Container Insights / AzureManagedPrometheusAgent |

---

## Live Demo: ICM 964000

**Scenario:** Pods restarting on cluster `kda1fb58033esos`

### What Copilot found in ~5 minutes:

**Step 1 — Triage:**
- ✅ Addon v6.26.0, switzerlandnorth, K8s 1.33.7
- ❌ Internal DCE/DCR Ids: **empty**
- ❌ `defaultmetricaccountname: ""` — no AMW linked
- **Private cluster** = requires DCE

**Step 2 — Errors:**
- 281,461 DCR/AMCS config errors
- 5,459 MDSD 403s: `"Data collection endpoint must be used to access configuration over private link"`
- 2,172 liveness probe 503s
- 22M OTel export failures per 6h window

**Step 3 — Root Cause Chain:**
```
Private cluster + No DCE/DCR/DCRA provisioned
  → MDSD calls AMCS → 403 "DCE must be used over private link"
  → TokenConfig.json never created
  → MetricsExtension never starts
  → OTel exports fail (connection refused to ME)
  → Liveness probe 503
  → Pod restarts every 5-8 min
```

**Step 4 — AMW Investigation (new subscription-level fallback query):**
- AMW `at58033-azmws` exists ✅ (created 2025-03-04)
- DCR: ❌ None ever created (90-day lookback)
- Ingestion: ❌ Zero data ever flowed

**Diagnosis:** Incomplete onboarding — AMW created but DCR+DCE+DCRA never deployed.

### What would have taken 30-60 min of manual KQL was done conversationally.

---

## Live Demo: ICM 770972482

**Scenario:** Partner team's pod monitor metrics not flowing to their AMW on a shared multi-tenant dev cluster

**Context:** An infrastructure platform team hosts a shared AKS cluster (`mshapisg2-dev-k8s-westus2-03`) with **18 different AMW associations** — a highly multi-tenant setup. A partner (PAS team) attached a pod monitor with `microsoft_metrics_account` relabeling to route metrics to their own AMW, following the [multi-AMW documentation](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-multiple-workspaces). Metrics aren't flowing.

### What Copilot found:

**Step 1 — Triage:**
- ✅ Addon v6.26.0, westus2, NOT private
- ✅ 18 AMWs associated — shared infrastructure cluster
- ✅ 38 pod monitors discovered, including partner's `pas-drc-1-podmonitor` (10 targets)
- ✅ 11 different account names in scrape config routing

**Step 2 — The Mismatch:**
The ICM attached a pod monitor with:
```yaml
relabelings:
  - action: replace
    replacement: ue2-prod-pas-1-amw      # ← WRONG: prod East US 2
    targetLabel: microsoft_metrics_account
```
But the partner's actual AMW is `wus2-dev-pas-1-amw` (dev West US 2). Different environment entirely.

**Step 3 — Verification via MDM:**
- `wus2-dev-pas-1-amw` **IS in the routing list** and **IS receiving metrics** — 238 metrics, 246K daily time series ✅
- `ue2-prod-pas-1-amw` is **NOT in routing**, **no ME logs**, no matching DCRA ❌
- No MetricsExtension errors for the partner's account — the correctly-configured pod monitor works fine

**Step 4 — ARM Forensics (the deeper investigation):**

Queried ARM telemetry for all DCRA operations on the cluster filtered to the partner. Discovered **extensive DCRA churn** over 30 days:

| DCRA Name | PUT ✅ | PUT 403 | DELETE ✅ | DELETE 403 |
|-----------|--------|---------|-----------|------------|
| `wus2-dev-pas-1-amw-association` | — | — | 9 | — |
| `wus2-dev-pas-1-dcr-association` | — | 2 | 4 | 16 |
| `pas-dcr-amw-association` | 1 | — | 1 (2 min later!) | — |
| `pas-dcrTestdcr-association` | 1 | — | — | — |
| `amw-mshapisg2-dev-uswest2-pas-association` | — | — | 2 | — |

Timeline revealed: partner tried **5 different DCRA naming conventions**, hit **403 permission failures** repeatedly, created associations then **deleted them minutes later**, and may not have a stable DCRA in place at all.

**Step 5 — Root Cause:**
```
Two independent issues:

1. Pod monitor account name mismatch:
   Pod monitor says "ue2-prod-pas-1-amw" (prod/EUS2)
   but AMW is "wus2-dev-pas-1-amw" (dev/WUS2)
   → Metrics with wrong label are silently dropped or go to default AMW

2. DCRA instability:
   Dozens of create/delete cycles over 30 days
   Multiple 403 permission failures
   → Partner may not have a stable DCRA currently in place
```

**Step 6 — Tooling Improvement:**

The ad-hoc ARM DCRA forensics queries were so useful they were immediately added to the MCP server as two new permanent queries:
- **DCRA History Timeline** — summarized view with success/failure counts per association
- **DCRA Detailed Timeline** — chronological trace for exact create/delete sequences

### This investigation shows what's hard without the tooling:
- Correlating across **6 different data sources** (ICM, App Insights, ARM, AMWInfo, MDM, AKS) to build the full picture
- The ARM forensics query alone required querying `ARMPRODEUS` with a 30-day lookback across 158 operations
- A human would need to: check the triage dashboard, then open Jarvis to check MDM, then open ARM Kusto to check DCRA history, then cross-reference account names — all while juggling 18 AMW associations on a shared cluster

---

## How the Process Works: Learning From Every ICM

### The Investigation Loop

For every new ICM that comes in, I have the agent investigate and see what it outputs. Then I dig deeper conversationally — asking follow-up questions, pointing it at specific data sources, guiding it toward the root cause. **The conversation IS the investigation.**

```
ICM comes in
  └─► "Investigate ICM 12345678"
        └─► Agent runs triage, errors, workload, config
        └─► Shows initial findings
              └─► "Dig deeper into the MDSD errors"
              └─► "Is this a private cluster?"
              └─► "Check ARM for whether the DCR was deleted"
              └─► "Compare this cluster to the working one they gave us"
                    └─► Root cause identified
                          └─► Agent adds any new ad-hoc queries to MCP server
                          └─► Skill gets smarter for next time
```

### Things It Can Do That the Dashboard Can't

The conversational approach unlocks investigation patterns that are impossible with a static dashboard:

**Cluster comparison** — Customers often give us a working cluster and a non-working cluster. The agent can run triage on both, diff the results, and pinpoint exactly what's different:
- "Both are private V1, but the broken one has no DCE and the working one does"
- "Same addon version, but the broken cluster's HPA is oscillating between 5 and 15 replicas"
- "Working cluster has a DCRA to the AMW, broken one's DCRA was deleted 3 days ago"

**ARM forensics** — It can quickly determine if resources were partially provisioned or deleted after enabling:
- "AMW exists in the subscription, DCR exists, but there was never a DCRA created — the onboarding was incomplete"
- "DCRA existed until 2 days ago — someone deleted it. Here's the ARM operation with the caller identity and timestamp"
- "DCE creation failed with a 409 conflict — region mismatch between the DCE and the AMW"

**Cross-referencing release notes** — When something changed after an addon update, it can look up our version in `RELEASENOTES.md`, find the MetricsExtension version we ship, then search EngHub for the ME release notes to see if a known issue was introduced:
- "You're on ME 2.2024.517.1714 which introduced a change to the queue overflow behavior — that matches the symptom of dropped samples after the upgrade"

**Arbitrary KQL** — When the built-in queries don't cover an edge case, it writes ad-hoc KQL on the fly against any of the 11 data sources. No context switching, no copy-pasting IDs.

### The SME Training Loop

At first, the agent had misconceptions — it would check the wrong columns, misinterpret error messages, or follow the wrong diagnostic path. But **every ICM has made it better:**

| ICM | What the Agent Learned |
|-----|----------------------|
| Early investigations | FQDN `-priv` pattern is NOT authoritative for private cluster detection → added definitive `privateLinkProfile` boolean check |
| ICM 770972482 | Multi-AMW routing: metrics go to the AMW whose DCR matches the scrape job, not necessarily the "default" AMW → added multi-AMW diagnostic queries and TSG |
| ICM 964000 | CCP cluster ID resolution is flaky (~6h sparse data) → added CCP-independent queries using `subscription` + `clusterName` directly |
| ICM 964000 | `ManagedClusterSnapshot` has no `resourceId` column (unlike what you'd expect) → fixed all AKS queries to use correct columns |
| ARM investigation | DCR/DCE/DCRA can be in any RG in the subscription, not just the cluster's RG → broadened ARM queries to subscription-level |
| Cardinality spike | Total TS count isn't enough — need per-dimension breakdown to find which label is exploding → added risk-rated cardinality queries |

**The key insight:** I'm the SME guiding the agent through each investigation. Every correction, every "no, check this instead", every "that column doesn't exist" gets permanently encoded into the queries, the TSGs, and the skill routing. The agent is accumulating the team's collective investigation experience — one ICM at a time.

---

## Technical Highlights

### WSL2 Reliability Fix
- Node.js `fetch` has **~80% TLS failure rate** to Kusto in WSL2
- Replaced with `curl` subprocess — **100% reliable**
- System OpenSSL handles WSL2 virtual networking correctly

### Multi-Source Query Engine
- Queries run in parallel (configurable concurrency, default 5)
- Retry with exponential backoff for transient failures
- 3-minute timeout per query
- Progress notifications via MCP protocol

### ICM Browser Scraper
- Chrome DevTools Protocol (CDP) via Edge
- Works on both Windows native and WSL2
- Intercepts raw API responses during page reload
- Extracts authored summary that ICM API tools don't return

### Auth Check
- Tests all data sources before investigation starts
- Auto-detects ARMProd Conditional Access Policy issues
- Suggests `azureauth` CLI for WAM-based auth

### Self-Improving: Every Investigation Makes the Next One Faster

The skill includes a **Step 5: Improve the Tooling** — after every investigation, the agent automatically adds any ad-hoc KQL queries it wrote back into the MCP server and skill:

```
Investigation #1: "Is this a private cluster?"
  └─► Agent writes ad-hoc KQL against ManagedClusterSnapshot
  └─► Adds "Private Cluster Check (definitive)" to queries.ts
  └─► Rebuilds MCP server, commits, pushes

Investigation #2: Same question on a different cluster
  └─► Query already exists in tsg_triage — runs automatically
  └─► No ad-hoc work needed
```

**This means:**
- The **175+ queries** in the MCP server today were built up investigation by investigation
- Every new ICM pattern that requires a custom query gets **permanently captured**
- The skill's TSG routing and symptom mappings grow with each new case
- The team benefits from every investigation — not just the person who ran it

This is fundamentally different from a static dashboard — **the tooling learns from usage**.

---

## How to Set Up

### There's a Skill for That Too

You don't even need to follow the manual steps below. We built a **`troubleshooting-setup` skill** that walks Copilot through the entire setup process:

```
> @troubleshooting-setup Set up the prometheus-collector troubleshooting environment
```

Copilot will:
1. Check if Node.js is installed (and tell you how to get it if not)
2. Build the MCP server (`npm install` + `npx tsc`)
3. Configure `mcp.json` with the right paths
4. Run `tsg_auth_check` to validate credentials to all data sources
5. **If auth fails** — diagnose why (expired token? missing `az login`? ARMProd CAP issue? VPN not connected?) and walk you through the fix

This means nobody on the team has to spend time figuring out setup. They ask Copilot, and Copilot handles it — including troubleshooting auth issues that would otherwise be a Slack message to the person who built the tooling.

### Manual Setup (if you prefer)

### Prerequisites
- Copilot CLI installed
- Azure CLI logged in (`az login`)
- Corp VPN connected
- Node.js 22+ (comes with Copilot CLI)

### Quick Start
```bash
# Clone and checkout the branch
git checkout grwehner/tsg-tooling-and-devbox

# Build the MCP server
cd tools/prom-collector-tsg-mcp
npm install
npx tsc

# Add to ~/.copilot/mcp.json
{
  "mcpServers": {
    "prom-collector-tsg": {
      "command": "node",
      "args": ["tools/prom-collector-tsg-mcp/dist/index.js"]
    }
  }
}

# Start Copilot CLI and verify
copilot
> tsg_auth_check
```

### Usage
```
> Investigate ICM 12345678

> Troubleshoot cluster /subscriptions/.../managedClusters/mycluster

> Why are pods restarting? Check errors for the last 6 hours

> What's the metric volume for MDM account mac_12345?

> Run this KQL against AMWInfo: AzureMonitorMetricsDCRDaily | where ...
```

---

## What's Next: The SRE Agent

### "Isn't this what the SRE Agent should be doing?"

**Yes — and that's exactly the plan.** Everything we've built here is preparation for the SRE Agent.

The [Azure SRE Agent](https://eng.ms/docs/coreai/devdiv/serverless-paas-balam/serverless-paas-vikr/sre-agent/sre-agent-documentation) accepts:
- **Custom skills** — our `SKILL.md` + 16 TSGs + `reference.md`
- **Your code repo** — `Azure/prometheus-collector` for context on the addon
- **Data sources to query** — Kusto clusters, App Insights
- **Extra MCP servers** — our `prom-collector-tsg-mcp` with 175+ queries

All of this plugs directly into the SRE Agent's sub-agent framework. The skill, MCP server, and TSGs we're building locally today become the SRE Agent's brain for prometheus-collector incidents tomorrow.

### Why Not Just Use the SRE Agent Today?

**The identity and access problem.** The SRE Agent operates off a **managed identity** — and our investigation requires read access to data sources we don't own:

| Data Source | Owner | Access Problem |
|-------------|-------|---------------|
| **AKS Kusto** (5 clusters) | AKS team | We'd need to grant the SRE Agent's managed identity reader access to clusters owned by another team |
| **AKS CCP Kusto** | AKS Control Plane team | Same — different team's cluster, separate access request |
| **ARMProd** (3 regional clusters) | ARM team | Has a **Conditional Access Policy** that blocks non-compliant auth — managed identities may not satisfy CAP |
| **AMCS / AMWInfo** | Azure Monitor Control Service | Yet another team's Kusto cluster to onboard |
| **Geneva MDM** (per-account) | Customer's AMW | Read access for throttling/metric names/drops must be assigned **per MDM account individually** — there's no blanket "read all accounts" role |
| **App Insights** | Our team (we own this one) | ✅ This one we can grant easily |

**With our own identity** (logged in via `az login`), we already have:
- Reader access to all these Kusto clusters (granted to our team/alias)
- MDM read access for customer accounts (inherited from our team's on-call permissions)
- CAP-compliant auth via `azureauth` WAM tokens

**With the SRE Agent's managed identity**, we'd need to:
1. File access requests with 5+ different teams for their Kusto clusters
2. Get each team to add the managed identity to their cluster's reader role
3. Figure out ARMProd CAP compliance for a managed identity
4. Assign MDM read access for every individual customer AMW account (not scalable)

### The Path Forward

```
Today (Local Copilot CLI)          Tomorrow (SRE Agent)
─────────────────────────          ─────────────────────
Your identity (az login)    →→→    SRE Agent managed identity
  ✅ All access already              ⬜ Access requests needed
                                     ⬜ Per-team Kusto onboarding
Skill + MCP server          →→→    Same skill + MCP server
  ✅ Built and tested                ✅ Plugs in directly

Manual trigger              →→→    Auto-trigger on ICM
  "investigate ICM 123"              SRE Agent picks up ICM
                                     runs skill automatically
```

**We're building the skill and MCP server now** because:
1. They work locally today — immediate value for on-call
2. They're the same artifacts the SRE Agent will use — no throwaway work
3. Every investigation improves them — by the time we onboard the SRE Agent, the tooling will be battle-tested with dozens of real ICMs
4. The identity/access problem is solvable — it's just logistics, not architecture

---

## Adopting This for Your Team: A Step-by-Step Guide

This approach isn't specific to prometheus-collector — **any team with ICM on-call can build the same thing for their service.** Here's how.

### What You Need to Start

| Component | What It Is | Effort |
|-----------|-----------|--------|
| **MCP Server** | A Node.js/TypeScript server that wraps your diagnostic KQL queries as callable tools | 1-2 days for a basic version |
| **Skill** (`SKILL.md`) | A markdown file that teaches the LLM your investigation workflow, symptom routing, and escalation paths | 1 day — start with your existing TSG docs |
| **TSG docs** | Your existing TSGs, split into individual files so only the relevant one loads into context | Already have these (just restructure) |
| **Copilot CLI** | The runtime that connects skill + MCP server + LLM | Already available |

### Step 1: Identify Your Data Sources

Map out every place you look during an ICM investigation:

```
Your Service
  ├─► Where are your container/service logs?     (App Insights? Kusto? Geneva Logs?)
  ├─► Where is your platform telemetry?           (AKS Kusto? Service Fabric? VM metrics?)
  ├─► Where is your control plane config?         (ARM? AMCS? Service-specific RP?)
  ├─► Where are your customer-facing metrics?     (Geneva MDM? Azure Monitor? Custom?)
  ├─► Where is your deployment/change history?    (ARM? EV2? Kubernetes?)
  └─► What external data sources do you query?    (Dependent services' Kusto clusters?)
```

For each, note:
- The Kusto cluster URL or App Insights resource
- What auth is needed (your team alias? specific role? CAP?)
- The key identifier to look up data (ARM ID? subscription? internal ID?)

### Step 2: Track Your Queries Before You Build Anything

**Before writing any code, start keeping a list.** For the next few ICMs you investigate, write down every KQL query you run, every dashboard you open, every ID you copy-paste. This is your query backlog.

```markdown
## Query Backlog

### Queries I run on every ICM (triage):
- [ ] Check addon version (App Insights: `customDimensions.version`)
- [ ] Check cluster region (App Insights: `customDimensions.region`)
- [ ] Check if addon is enabled (AKS Kusto: `ManagedClusterSnapshot`)
- [ ] Check AMW association (AMWInfo: `AzureMonitorMetricsDCRDaily`)
- [ ] Check pod restart count (AKS Kusto: `ContainerLastStatus`)

### Queries I run for specific symptoms:
- [ ] OOM investigation: pod memory vs node capacity vs HPA replicas
- [ ] Missing metrics: ME errors, scrape config, target health
- [ ] Auth failures: token adapter logs, MDSD errors, DCR status

### Queries I wish I had:
- [ ] "Was the DCR deleted recently?" (need ARM query)
- [ ] "What changed between the last working version and now?"
- [ ] "Compare this cluster to the working cluster the customer gave us"

### Dashboards / portals I open:
- [ ] ADX dashboard (our KQL) — for what queries?
- [ ] Jarvis (MDM) — for what metrics?
- [ ] Service Insights (AKS) — for what views?
- [ ] Azure Portal — for what resources?
```

**Why this matters:** This list becomes your MCP server's query backlog. The queries you run on every ICM become `tsg_triage`. The symptom-specific ones become `tsg_errors`, `tsg_workload`, etc. The "queries I wish I had" become your roadmap. You don't need to automate everything on day one — just know what you're aiming for.

### Step 3: Build the MCP Server

Start minimal — you can always add more queries later.

```
tools/your-service-tsg-mcp/
├── src/
│   ├── index.ts          ← Tool definitions + query execution
│   ├── queries.ts        ← All your KQL queries, organized by category
│   └── datasources.ts    ← Kusto cluster URLs + App Insights config
├── package.json
└── tsconfig.json
```

**Start with 3-4 tools:**

| Tool | Purpose |
|------|---------|
| `tsg_triage` | The queries you run first for every ICM — version, region, config, health |
| `tsg_errors` | Scan all error categories in your logs |
| `tsg_query` | Ad-hoc KQL against any of your data sources (the escape hatch) |
| `tsg_auth_check` | Validate credentials to all data sources before investigation |

**Key design pattern:** Each tool runs a *category* of queries (an array), not a single query. This means you can add new queries to an existing tool just by appending to the array — no tool definition changes needed.

```typescript
// queries.ts — start with your most common triage queries
export const queries = {
  triage: [
    { name: "Service Version", datasource: "YourAppInsights", kql: `...` },
    { name: "Region", datasource: "YourAppInsights", kql: `...` },
    { name: "Health Check", datasource: "YourKusto", kql: `...` },
    // Add more over time — every investigation adds queries here
  ],
  errors: [
    { name: "Container Errors", datasource: "YourAppInsights", kql: `...` },
    // ...
  ]
};
```

### Step 4: Write the Skill

Your `SKILL.md` is the investigation playbook. It should have:

**1. A workflow** — what to do step by step:
```markdown
### Step 1: Gather Context (extract ARM ID from ICM)
### Step 2: Run Triage (tsg_triage → identify symptom category)
### Step 3: Follow the TSG (route to the right diagnostic doc)
### Step 4: Summarize Findings (structured output)
### Step 5: Improve the Tooling (add ad-hoc queries back to MCP server)
```

**2. A symptom → tool routing table:**
```markdown
| Symptom | Tool | TSG |
|---------|------|-----|
| Pod crashing | tsg_errors + tsg_triage | pod-restarts.md |
| Metrics missing | tsg_triage + tsg_config | missing-metrics.md |
| High latency | tsg_workload | performance.md |
```

**3. Tool descriptions** — so the LLM knows what each tool does and when to use it.

**4. Escalation contacts** — who to hand off to when it's not your problem.

### Step 5: Split Your TSGs

Don't put all your TSGs in one file. Split them so only the relevant one loads:

```
.github/skills/your-service-tsg/
├── SKILL.md                      ← Entry point (small, always loaded)
├── reference.md                  ← Deep technical details (loaded on demand)
└── tsgs/
    ├── pod-restarts.md           ← Loaded when symptom matches
    ├── missing-metrics.md
    ├── performance.md
    ├── auth-failures.md
    └── deployment-issues.md
```

This keeps the initial context small while having deep knowledge available when needed.

### Step 6: Iterate With Real ICMs

**This is the most important step.** The tooling gets good by using it on real incidents:

```
Week 1:  Basic MCP server (10 queries) + skeleton skill
         └─► Run it on your first ICM
         └─► It will be wrong about some things — that's expected
         └─► Fix the queries, update the skill

Week 2:  20-30 queries, better routing
         └─► Run it on 2-3 more ICMs
         └─► Add the ad-hoc queries you wrote during investigation
         └─► Add edge cases to TSGs

Week 4:  50+ queries, solid skill
         └─► Non-SMEs on the team can start using it
         └─► Most common ICM patterns are covered

Week 8+: 100+ queries, comprehensive coverage
         └─► Ready for SRE Agent integration
         └─► Every ICM type has a diagnostic path
```

**The key insight:** Don't try to build it all upfront. Start with the 5-10 queries you run on every ICM, then let real investigations drive what gets added next. The SME guides the agent through each new case, and the corrections become permanent improvements.

**Pro tip: Test it on past ICMs.** You don't have to wait for new incidents. Pull up a closed ICM where you already know the root cause, give it to the agent, and see if it reaches the same conclusion. This is the fastest way to find gaps:

- If it **gets the same root cause** → your queries and routing are solid for that symptom pattern
- If it **misses the root cause** → you've found a gap. What query would have caught it? Add it
- If it **finds the root cause but takes a wrong path first** → your skill routing needs a better symptom→tool mapping
- If the **data is expired** (>30 days) → you'll discover which data sources have retention limits and can document that in the skill

This is especially useful early on — you probably have dozens of resolved ICMs with known root causes. Run 5-10 of them through the agent in the first week and you'll quickly build coverage for your most common incident patterns.

### What You'll Get

| Before | After |
|--------|-------|
| 30-60 min manual KQL per ICM | 5-10 min conversational investigation |
| SME knowledge in people's heads | SME knowledge encoded in skill + queries |
| New team members struggle on-call | Non-SMEs get the same diagnostic path as experts |
| Static dashboard, multiple web pages | One terminal, all data sources |
| Each investigation starts from scratch | Each investigation builds on all previous ones |

---

## Commits on This Branch

| Commit | Description |
|--------|-------------|
| `a6f97ac` | feat: add Missing DCE triage check and subscription-level AMW fallback |
| `759acaf` | fix: replace Node.js fetch with curl for Kusto queries (WSL2 TLS fix) |
| `04ce2a5` | fix: use data queries for auth check, expand tested data sources |
| `8a3585e` | docs: expand multi-AMW routing TSG from ICM 770972482 learnings |
| `162aa88` | Split TSGs into individual files, remove public doc gaps |
| `c0ae73e` | Add ARM regional data sources and investigation queries |
| `2d02741` | Fix AKS/CCP query failures: token replacement and CCP ID resolver |
| `91cd7bc` | feat: add ARMProd data source and improve retry logic |
| ... | 20+ commits total |

---

## Questions?

**Repo:** `Azure/prometheus-collector` branch `grwehner/tsg-tooling-and-devbox`
**MCP Server:** `tools/prom-collector-tsg-mcp/`
**Skill:** `.github/skills/prom-collector-tsg/`
