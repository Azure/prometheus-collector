# Copilot Instructions for prometheus-collector

This repository is the **Azure Managed Prometheus** (prometheus-collector / AMA Metrics addon) codebase. It contains the addon that runs in AKS clusters to scrape Prometheus metrics and send them to Azure Monitor Workspace (AMW) via Geneva MDM.

## Troubleshooting Skills

This repo includes two Copilot skills for ICM investigation and troubleshooting. They are auto-discovered from `.github/skills/` when the working directory is inside this repo.

### 1. `prom-collector-tsg` — ICM Investigation Skill

**When to use:** Investigating ICM incidents, customer escalations, or debugging metrics issues on AKS clusters running the Managed Prometheus addon.

**Invoke by saying:** "Investigate ICM 12345678" or "Troubleshoot cluster /subscriptions/.../managedClusters/my-cluster"

**What it does:** Runs diagnostic KQL queries against 7+ internal data sources (App Insights, AKS Kusto, AKS CCP, MetricInsights, AMWInfo, ARM) and follows structured TSG workflows to diagnose:
- Missing metrics, pod crashes, OOM kills
- Scrape config issues, KSM timeouts
- MDM throttling, dimension limits
- Node drain / eviction loops
- Private link, DNS, proxy issues
- Control plane metrics, DCR/DCE misconfigurations

**Requires:** Local setup of MCP servers (see setup skill below).

### 2. `troubleshooting-setup` — Environment Setup Skill

**When to use:** Setting up the local troubleshooting environment for the first time, or when MCP tools aren't working.

**Invoke by saying:** "Help me set up the troubleshooting environment" or "My MCP tools aren't loading"

**What it does:** Walks through complete setup of:
- Copilot CLI (Agency) installation
- TSG MCP server build and configuration (`~/.copilot/mcp-config.json`)
- Geneva MDM MCP server setup (optional, for `tsg_mdm_throttling` / `tsg_scrape_health`)
- Edge CDP for ICM browser scraping (`tsg_icm_page`)
- Authentication (Azure CLI, azureauth shim for WSL2, EngHub keyring)
- VS Code MCP configuration

**Quick start (if you already have Agency CLI and the repo cloned):**
```bash
cd ~/go/src/prometheus-collector
bash tools/prom-collector-tsg-mcp/setup.sh
```

Then:
```bash
agency copilot
> "Investigate ICM 12345678"
```

## For SREs Setting Up Local Investigation

If you want to do **deep local investigations** (running diagnostic queries, checking scrape health, analyzing MDM throttling), you need the local MCP server setup. Ask Copilot:

> "Set up the troubleshooting environment"

Or follow the full guide at: `.github/skills/troubleshooting-setup/SKILL.md`

### Minimum Setup (5 minutes)

1. **Install Agency CLI:** `curl -fsSL https://aka.ms/install-agency | bash`
2. **Clone repo:** `git clone https://github.com/Azure/prometheus-collector.git`
3. **Run setup:** `cd prometheus-collector && bash tools/prom-collector-tsg-mcp/setup.sh`
4. **Start Copilot:** `agency copilot` → "Investigate ICM 12345678"

### What the Setup Provides

| Tool | What it enables |
|------|----------------|
| `tsg_triage` | Cluster version, AMW config, node pools, DCR/DCE, MDM account |
| `tsg_errors` | All error categories (ME, MDSD, token adapter, DNS, TA) |
| `tsg_workload` | CPU/memory, samples/min, drops, HPA, KSM health |
| `tsg_pods` | Pod restarts, node drains, evictions, autoscaler events |
| `tsg_config` | Scrape configs, keep lists, custom config validation |
| `tsg_logs` | Raw component logs (RS, DS, Windows, configreader) |
| `tsg_query` | Ad-hoc KQL against any data source |
| `tsg_icm_page` | Scrape ICM authored summary via Edge CDP |
| `tsg_mdm_throttling` | Geneva MDM QoS (requires MDM MCP server) |
| `tsg_scrape_health` | Per-job scrape target health from MDM |
| `tsg_me_diagnostics` | MetricsExtension internal drop analysis |

## Code Conventions

- **Go** code is in `otelcollector/` — the main addon binary
- **TypeScript** tooling is in `tools/` — MCP servers and utilities
- **Helm charts** are in `otelcollector/deploy/` — addon deployment
- **Skills and TSGs** are in `.github/skills/` — troubleshooting guides
- Do NOT commit customer-specific data (ARM IDs, cluster names, subscription IDs, ICM numbers)
