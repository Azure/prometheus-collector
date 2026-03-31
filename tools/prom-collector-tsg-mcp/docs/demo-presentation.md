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
| **`tsg_triage`** | Initial triage: addon version, region, AMW/DCR/DCE config, token adapter health, CCP cluster ID, node pool capacity & autoscaling, AKS upgrade history, ⚠️ Missing DCE check for private clusters |
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
Version • Component Versions (ME, OTel, Golang, Prometheus) • Cluster Region • AKS Cluster ID • Azure Monitor Workspace • MDM Account ID • MDM Stamp • AMW Region • Internal DCE/DCR Ids • ⚠️ Missing DCE for Private Cluster (AMCS 403) • Token Adapter Health • DCRs Associated with Cluster • AMW(s) from Scrape Config Routing • AMW(s) • AMW(s) in Subscription (fallback) • AKS Network Settings • AKS Addons Enabled • AKS Cluster Settings • AKS Cluster State • CCP Cluster ID • CCP Cluster ID (AgentPoolSnapshot fallback) • Node Pool Capacity • Node Conditions (Memory/Disk/PID Pressure) • Node Allocatable Resources • AgentPool Autoscaling History • AKS Upgrade History • Node Pool Versions (resource_id fallback)

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
2. **Run Triage** — `tsg_triage` → version, region, DCR/AMW, node pools, private cluster check, ⚠️ Missing DCE
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
| Private link errors | `tsg_triage` (⚠️ DCE check) → `tsg_errors` | Firewall / Private Link |
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
- ⚠️ **Private cluster** = requires DCE

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

---

## How to Set Up

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

## What's Next

- [ ] Merge `grwehner/tsg-tooling-and-devbox` → main
- [ ] Integrate with SRE Agent for automated ICM triage
- [ ] Auto-generate ICM summary from investigation results

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
