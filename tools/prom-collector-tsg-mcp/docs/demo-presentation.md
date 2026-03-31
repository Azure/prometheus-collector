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
| **`tsg_triage`** | Version, region, AMW config, DCR/DCE, token adapter, node pools, AKS upgrade history |
| **`tsg_errors`** | Scans ALL error categories: container, OTel, ME, MDSD, token adapter, TA, DNS, private link, liveness, DCR/AMCS |
| **`tsg_config`** | Scrape configs, custom config validation, keep lists, HPA, pod/service monitors |
| **`tsg_workload`** | CPU, memory, samples/min, drops, queue sizes, export failures, HPA status |
| **`tsg_pods`** | Pod restarts, restart reasons (OOM, liveness), per-pod detail |
| **`tsg_logs`** | Raw logs from replicaset, daemonset, windows, configreader |
| **`tsg_control_plane`** | Control plane metrics config and health |
| **`tsg_query`** | Ad-hoc KQL against any of 11 data sources — write results to CSV/JSON |
| **`tsg_metric_insights`** | Top metrics by time series count, sample rate, cardinality |
| **`tsg_mdm_throttling`** | Geneva MDM QoS: throttling, drops, time series limits |
| **`tsg_scrape_health`** | Per-job scrape target health from MDM (`up` metric analysis) |
| **`tsg_icm_page`** | Browser-scrape ICM for authored summary + discussion (not in API) |
| **`tsg_dashboard_link`** | Direct link to ADX dashboard pre-filtered for cluster |
| **`tsg_auth_check`** | Validate credentials + connectivity to all data sources |

### By the Numbers

| Metric | Count |
|--------|-------|
| MCP tools | 14 |
| KQL queries | 175 |
| Query categories | 9 |
| Kusto clusters | 12 |
| TSG documents | 16 |
| Symptom→Tool mappings | 21 |
| Lines of TypeScript | ~5,500 |

---

## The Skill Layer

The **TSG Skill** (`.github/skills/prom-collector-tsg/`) teaches Copilot *how* to investigate:

### 5-Step Workflow
1. **Gather Context** — Scrape ICM page + call ICM API in parallel, extract cluster ARM ID
2. **Run Triage** — `tsg_triage` → version, region, DCR/AMW, node pools, private cluster check
3. **Identify Symptoms** — Match error patterns to one of 16 TSGs
4. **Deep Dive** — Follow TSG-specific tool sequence (errors → workload → logs → config)
5. **Summarize** — Root cause, error counts, fix steps, escalation path, dashboard link

### 16 Troubleshooting Guides

| Category | TSGs |
|----------|------|
| **Core** | Missing Metrics, Pod Restarts/OOM, Spike in Metrics |
| **Network** | Firewall/Private Link/AMPLS, Proxy, DNS |
| **Config** | Duplicate Labels (KSM), DCR/DCE Region Mismatch, Control Plane Metrics |
| **Platform** | Windows Pod Restarts, Remote Write, Pods Not Created, Node Exporter ARM64 |
| **Operations** | AMW Usage Optimization, Vulnerabilities/CVEs, Known Issues & FAQ |
| **NEW** | Missing DCE for Private Clusters (from ICM 964000 investigation) |

### Symptom → Tool Quick Reference (sample)

| Symptom | Run these tools |
|---------|----------------|
| No metrics flowing | `tsg_triage` → `tsg_errors` → `tsg_mdm_throttling` |
| Pod CrashLoopBackOff | `tsg_errors` → `tsg_workload` → `tsg_pods` |
| Spike in ingestion | `tsg_workload` → `tsg_metric_insights` → `tsg_mdm_throttling` |
| Private link errors | `tsg_triage` (⚠️ Missing DCE check) → `tsg_errors` |
| Config not applied | `tsg_config` (validation errors + custom job names) |

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
- [ ] Add more ARM investigation queries (DCRA verification)
- [ ] Improve MetricInsights queries for cardinality root cause
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
