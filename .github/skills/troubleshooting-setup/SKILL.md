---
name: troubleshooting-setup
description: "Complete onboarding guide for setting up the Azure Managed Prometheus troubleshooting environment with Copilot CLI, MCP servers, and diagnostic tooling. USE FOR: onboarding, new developer setup, troubleshooting setup, environment setup, getting started, ICM triage, TSG setup, MCP setup, install tools, configure copilot, prometheus-collector setup, setup failed, setup errors, MCP not working, tools not loading, azureauth errors, Node.js not found, npm install failed, skills not loading, agency config."
---

# Prometheus-Collector Troubleshooting Environment — Onboarding Guide

This guide walks through setting up the complete troubleshooting environment for Azure Managed Prometheus (prometheus-collector) ICM investigation using Copilot CLI, MCP servers, and diagnostic tools.

## Quick Start (Automated Setup)

If you already have **Agency CLI** installed and the **repo cloned**, run the setup script:

```bash
cd ~/go/src/prometheus-collector
bash tools/prom-collector-tsg-mcp/setup.sh
```

This builds the MCP server, writes `~/.copilot/mcp.json` with correct paths, sets up auth workarounds (WSL2 only), and configures Edge CDP for ICM browser scraping (WSL2 only). Everything else is already in the repo:

| What | Where | Auto-discovered? |
|------|-------|-------------------|
| Built-in MCPs (ado, icm, es-chat) | `agency.toml` (repo root) | ✅ Yes — Agency reads it when you run `agency copilot` from the repo |
| Skills (prom-collector-tsg, troubleshooting-setup) | `.github/skills/` | ✅ Yes — Copilot loads them when cwd is in the repo |
| Repo context | `.github/copilot-instructions.md` | ✅ Yes — Copilot reads it automatically |
| VS Code MCP config | `.vscode/mcp.json` | ✅ Yes — VS Code reads it automatically |
| Custom MCP (prom-collector-tsg binary) | `~/.copilot/mcp.json` | ⚠️ Needs setup.sh — requires absolute path to built `dist/index.js` |
| Edge CDP (WSL2 only) | Windows port proxy + Edge launch | ⚠️ Needs setup.sh — launches Edge with `--remote-debugging-port=9222` and configures port proxy |

After setup:
```bash
cd ~/go/src/prometheus-collector
agency copilot
> "Investigate ICM 12345678"
```

> **First time?** If you don't have Agency or the repo yet, follow the full guide below starting at Section 1.

> **New Dev Box?** Use `tools/prom-collector-tsg-mcp/devbox.yaml` as the Dev Box customization file — it installs everything (Git, Azure CLI, Node.js, .NET 8, WSL2, Agency CLI, VS Code), clones the repo, builds the MCP server, and configures Edge CDP for WSL2.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Clone the Repository](#2-clone-the-repository)
3. [Install Agency CLI + Copilot CLI](#3-install-agency-cli--copilot-cli)
4. [Set Up the TSG MCP Server](#4-set-up-the-tsg-mcp-server)
5. [Set Up the Geneva MDM MCP Server](#5-set-up-the-geneva-mdm-mcp-server)
6. [Configure Copilot CLI MCP Servers](#6-configure-copilot-cli-mcp-servers)
7. [Configure VS Code MCP Servers](#7-configure-vs-code-mcp-servers)
8. [Authentication](#8-authentication)
9. [Verify Everything Works](#9-verify-everything-works)
10. [Available Tools Reference](#10-available-tools-reference)
11. [Troubleshooting Workflow](#11-troubleshooting-workflow)
12. [Quick Reference](#12-quick-reference)
13. [Setup Troubleshooting](#13-setup-troubleshooting)

---

## 1. Prerequisites

### Determine your platform

The setup differs depending on whether you're running on **native Windows** or **WSL2 (Windows Subsystem for Linux)**. Windows is simpler — fewer workarounds are needed.

| Concern | Windows (native) | WSL2 (Linux) |
|---------|-------------------|--------------|
| **ICM browsing** | ✅ `tsg_icm_page` connects to Edge on localhost:9222 | ⚠️ `tsg_icm_page` connects via port proxy on 9223 — needs netsh setup |
| **Browser automation** | ✅ Playwright MCP works natively | ⚠️ Needs Google Chrome installed + `--disable-gpu` flag |
| **EngHub auth** | ✅ Keytar/keychain works natively | ⚠️ Needs gnome-keyring-daemon shim (see Section 8) |
| **azureauth** | ✅ Usually pre-installed on corp machines | ⚠️ Often missing — needs shim script (see Section 8) |
| **Node.js / .NET / az CLI** | Install via winget or official installers | Install via apt / curl scripts |

**How to tell which you're on:**
- If `uname -a` shows `Linux ... microsoft ... WSL2` → you're in WSL2
- If you're in a Windows terminal (PowerShell/cmd) → native Windows
- If you're in a VS Code terminal and the shell is bash inside WSL → WSL2

---

### Windows (native) prerequisites

- **Git** — with Azure DevOps / GitHub credential manager
- **Node.js 18+** — `winget install OpenJS.NodeJS.LTS` or from https://nodejs.org
- **Azure CLI** — `winget install Microsoft.AzureCLI` or from https://aka.ms/installazurecliwindows
- **.NET 8+ SDK** — `winget install Microsoft.DotNet.SDK.8` or from https://dot.net
- **Docker** (optional) — only if running local Aspire workloads

#### Edge CDP for ICM browsing (Windows)

The `tsg_icm_page` tool connects directly to Edge on `localhost:9222`. Launch a **separate** Edge instance with a dedicated profile:

```powershell
# Launch Edge with remote debugging (use a dedicated profile to avoid conflicts with your main Edge)
Start-Process "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" `
  -ArgumentList "--remote-debugging-port=9222","--user-data-dir=C:\Users\$env:USERNAME\.edge-cdp-debug","--no-first-run","--disable-sync"
```

**Verify:**
```powershell
(Invoke-WebRequest -Uri "http://localhost:9222/json/version" -UseBasicParsing).Content | ConvertFrom-Json | Select-Object Browser
# Should show: Edg/xxx.x.xxxx.xx
```

Then sign in to ICM in that Edge window (navigate to `https://portal.microsofticm.com` and authenticate). After that, `tsg_icm_page` can scrape any incident.

> **⚠️ IMPORTANT — Edge profile locking:** You **must** use a `--user-data-dir` that is different from your main Edge profile (`%LOCALAPPDATA%\Microsoft\Edge\User Data`). If you point to your existing profile while Edge is already running, the new instance will merge into the existing session and close — the CDP port won't work. Always use a dedicated directory like `C:\Users\<user>\.edge-cdp-debug`.

> **Why not Playwright for ICM?** Playwright MCP can browse most sites, but the ICM portal SPA is extremely heavy (~150 console warnings, lazy-rendered React UI). `browser_snapshot` hangs on the complex DOM, and the authored summary is never in the visible `innerText` — ICM loads it via XHR API calls. `tsg_icm_page` is purpose-built for this: it intercepts the raw `GetIncidentDetails` and `getdescriptionentries` API responses via CDP Network capture during page reload, reliably extracting the full authored summary and discussion entries.

---

### WSL2 (Linux) prerequisites

You need a WSL2 distro (Ubuntu/Debian) with:

- **Git** — with Azure DevOps / GitHub credential manager
- **Node.js 18+** — for the TSG MCP server (`node --version`)
- **Azure CLI** — for authentication (`az --version`)
- **.NET 8+ SDK** — for the Geneva MDM MCP server (`dotnet --version`)
- **Docker** (optional) — only if running local Aspire workloads

#### Install Node.js (if not present)

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs
```

#### Install Azure CLI (if not present)

```bash
curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
```

#### Install .NET SDK (if not present)

```bash
# Install .NET 8 (required for Geneva MDM MCP)
curl -fsSL https://dot.net/v1/dotnet-install.sh | bash -s -- --channel 8.0

# Add to PATH
echo 'export DOTNET_ROOT="$HOME/.dotnet"' >> ~/.bashrc
echo 'export PATH="$DOTNET_ROOT:$DOTNET_ROOT/tools:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

#### Install Google Chrome (WSL2 only — for Playwright browser automation)

```bash
# Chrome is needed for Playwright MCP to work on Linux/WSL
# The bundled Chromium often has GPU rendering issues
wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb
sudo apt-get install -f  # fix any dependency issues
```

#### ICM Browser Access — Edge CDP setup

The `tsg_icm_page` tool connects to Edge via the Chrome DevTools Protocol (CDP) to scrape ICM incident pages. This works on both **Windows (native)** and **WSL2**.

**Windows (native) — one-time setup:**

```powershell
# Launch Edge with remote debugging port (add to a .bat file or PowerShell profile)
Start-Process "C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe" `
  -ArgumentList "--remote-debugging-port=9222","--user-data-dir=C:\Users\$env:USERNAME\.edge-cdp-debug","--no-first-run"
```

**Verify:**
```powershell
(Invoke-WebRequest -Uri "http://localhost:9222/json/version" -UseBasicParsing).Content | ConvertFrom-Json | Select-Object Browser
# Should return Edge version info
```

No port proxy or firewall rules needed — the tool connects directly to `localhost:9222`.

**WSL2 (Linux) — one-time setup on the Windows host (run in PowerShell as Administrator):**

**Automated setup (recommended):** `setup.sh` handles this automatically — it launches Edge with `--remote-debugging-port=9222`, sets up the Windows port proxy via admin-elevated PowerShell, and auto-detects the WSL gateway IP.

```bash
# setup.sh does all of this for you, but if you need to do it manually:

# 1. From WSL, launch Edge with CDP debugging port
powershell.exe -Command "Start-Process 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' -ArgumentList '--remote-debugging-port=9222','--user-data-dir=C:\Users\<user>\.playwright-mcp-edge3','--no-first-run'"

# 2. Set up port proxy with admin elevation (from WSL)
powershell.exe -Command "Start-Process powershell -Verb RunAs -ArgumentList '-Command','netsh interface portproxy add v4tov4 listenport=9223 listenaddress=0.0.0.0 connectport=9222 connectaddress=127.0.0.1; netsh advfirewall firewall add rule name=\"\"\"WSL Edge CDP\"\"\" dir=in action=allow protocol=TCP localport=9223'"
```

> **Note:** The admin elevation will trigger a Windows UAC prompt — you must click "Yes" on the Windows side.

**Verify from WSL:**
```bash
WSL_GATEWAY=$(ip route show default | awk '{print $3}')
curl -s "http://${WSL_GATEWAY}:9223/json/version" | python3 -m json.tool
# Should return Edge version info with "Browser": "Edg/..."
```

> **Key detail:** The `tsg_icm_page` tool auto-detects the WSL gateway IP at runtime (via `ip route show default`). On Windows, it connects directly to `localhost:9222`. You can also override with `CDP_ENDPOINT=http://host:port`.

Then `tsg_icm_page` will work from both platforms to scrape the authored summary from the ICM portal.

---

## 2. Clone the Repository

```bash
# GitHub (primary)
git clone https://github.com/Azure/prometheus-collector.git ~/go/src/prometheus-collector

# Or if already cloned, ensure you're up to date
cd ~/go/src/prometheus-collector && git pull
```

---

## 3. Install Agency CLI + Copilot CLI

Agency is the Microsoft internal CLI platform. The **Copilot CLI** (`copilot`) comes bundled with Agency.

```bash
# Install Agency (includes Copilot CLI)
curl -fsSL https://aka.ms/install-agency | bash

# Verify — must be version 2026.3.13+ for MCP compatibility
agency --version
copilot --version

# Update if needed
agency update
```

> **Note:** `copilot` is the CLI that provides the AI-assisted terminal. MCP servers, skills, and tools are all configured for this CLI.

> **Note:** Agency bundles its own Node.js at `~/.agency/nodejs/node-v22.21.0-linux-x64/bin/node`. If system Node.js is not installed, you can use this for building the TSG MCP server:
> ```bash
> export PATH="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin:$PATH"
> ```

---

## 4. Set Up the TSG MCP Server

The TSG MCP server (`prom-collector-tsg`) runs diagnostic KQL queries against 7 Kusto clusters and App Insights to troubleshoot prometheus-collector issues.

### Build from source

```bash
cd ~/go/src/prometheus-collector/tools/prom-collector-tsg-mcp

# Install dependencies
npm install

# Build TypeScript (use agency's Node.js if system node isn't installed)
# export PATH="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin:$PATH"
npx tsc

# Verify it starts
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | timeout 5 node dist/index.js 2>/dev/null
# Should print a JSON response with serverInfo
```

### What it connects to

| Data Source | Kusto Cluster | Database | Purpose |
|-------------|---------------|----------|---------|
| PrometheusAppInsights | App Insights REST API | ContainerInsightsPrometheusCollector-Prod | Collector logs, metrics, configs |
| MetricInsights | metricsinsights.westus2.kusto.windows.net | metricsinsightsUX | Time series counts, ingestion rates |
| AMWInfo | appinsightstlm.kusto.windows.net | azuremonitorattach | Azure Monitor Workspace, DCR, MDM mapping |
| AKS | akshuba.centralus.kusto.windows.net | AKSprod | AKS cluster state, pod restarts, settings |
| AKS CCP | akshuba.centralus.kusto.windows.net | AKSccplogs | Control plane metrics config and logs |
| AKS Infra | akshuba.centralus.kusto.windows.net | AKSinfra | Control plane pod CPU, container restarts |
| Vulnerabilities | shavulnmgmtprdwus.kusto.windows.net | ShaVulnMgmt | Image CVE vulnerability scanning |

---

## 5. Set Up the Geneva MDM MCP Server

The Geneva MDM MCP server provides tools to query MDM time-series metrics — used for checking account throttling, dropped events, QoS health, and per-metric scrape target health (e.g., the `up` metric filtered by `job` and `cluster` dimensions).

### One-time setup (Windows)

```powershell
# Clone the MDM MCP repo
cd $env:USERPROFILE\go\src
git clone https://msazure@dev.azure.com/msazure/One/_git/Networking-MadariExt-MdmMCP mdm-mcp

# Get a token for the private NuGet feed
$token = az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv

# Add credentials for the private NuGet feed
dotnet nuget update source networking-madari-Consumption `
  --username az --password $token --store-password-in-clear-text `
  --configfile $env:USERPROFILE\go\src\mdm-mcp\nuget.config

# Enable nuget.org — edit nuget.config:
#   1. Remove the <clear /> line under <packageSources>
#   2. Remove the <disabledPackageSources> section
#   3. Add:  <add key="nuget.org" value="https://api.nuget.org/v3/index.json" />

# Restore and build
dotnet restore $env:USERPROFILE\go\src\mdm-mcp\src\MdmMcp\GenevaMDM-MCP.csproj
dotnet build $env:USERPROFILE\go\src\mdm-mcp\src\MdmMcp\GenevaMDM-MCP.csproj --no-restore
```

### Start the server (Windows)

```powershell
$env:DOTNET_EnableDiagnostics = "0"
$env:ASPNETCORE_URLS = "http://localhost:5050"
$env:ASPNETCORE_ENVIRONMENT = "Local"

# Start in a new window so it persists
Start-Process dotnet -ArgumentList "run","--project","$env:USERPROFILE\go\src\mdm-mcp\src\MdmMcp\GenevaMDM-MCP.csproj","--no-build","--","--urls","http://localhost:5050" `
  -WindowStyle Minimized

# Verify (wait ~10 seconds for startup)
Start-Sleep -Seconds 10
(Invoke-WebRequest -Uri "http://localhost:5050/mcp" -Method POST -ContentType "application/json" `
  -Body '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' `
  -UseBasicParsing).Content | ConvertFrom-Json | ConvertTo-Json -Depth 3
# Should return serverInfo with name "GenevaMDM-MCP"
```

### One-time setup (WSL2 / Linux)

```bash
# Clone the MDM MCP repo
cd /tmp
git clone https://msazure@dev.azure.com/msazure/One/_git/Networking-MadariExt-MdmMCP mdm-mcp

# Add the private NuGet feed with auth
TOKEN=$(az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv)
dotnet nuget add source \
  "https://msazure.pkgs.visualstudio.com/One/_packaging/networking-madari-Consumption/nuget/v3/index.json" \
  --name networking-madari-Consumption \
  --username az --password "$TOKEN" --store-password-in-clear-text \
  --configfile /tmp/mdm-mcp/nuget.config

# Enable nuget.org in the repo's nuget.config (disabled by default)
# Edit /tmp/mdm-mcp/nuget.config and remove or comment out the <clear/> line
# that disables nuget.org, or add:
#   <add key="nuget.org" value="https://api.nuget.org/v3/index.json" />

# Restore and build
dotnet restore /tmp/mdm-mcp/src/MdmMcp/GenevaMDM-MCP.csproj
dotnet build /tmp/mdm-mcp/src/MdmMcp/GenevaMDM-MCP.csproj --no-restore
```

### Start the server (WSL2 / Linux)

```bash
export DOTNET_ROOT="$HOME/.dotnet"
export PATH="$DOTNET_ROOT:$DOTNET_ROOT/tools:$PATH"

# IMPORTANT: DOTNET_EnableDiagnostics=0 prevents a startup hang
DOTNET_EnableDiagnostics=0 \
ASPNETCORE_URLS="http://localhost:5050" \
ASPNETCORE_ENVIRONMENT=Local \
nohup dotnet run --project /tmp/mdm-mcp/src/MdmMcp/GenevaMDM-MCP.csproj --no-build \
  -- --urls "http://localhost:5050" > /tmp/mdm-mcp.log 2>&1 &
echo "MDM MCP PID: $!"

# Verify it's running (wait ~10 seconds for startup)
sleep 10
curl -s http://localhost:5050/mcp \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
# Should return a JSON-RPC response
```

> **Key details:**
> - Port 5050 avoids conflicts with other services
> - `DOTNET_EnableDiagnostics=0` is **required** — without it the server hangs on startup
> - This is an HTTP MCP server (not stdio) — it must be running before Copilot can use it
> - Auth uses `DefaultAzureCredential` — ensure you're logged in with `az login`
> - **Windows:** clone to `%USERPROFILE%\go\src\mdm-mcp`; **WSL2:** clone to `/tmp/mdm-mcp`

---

## 6. Configure Copilot CLI MCP Servers

Copilot CLI reads MCP server config from **two files** (either works, both are checked):
- `~/.copilot/mcp.json` — primary config
- `~/.copilot/mcp-config.json` — alternative (same format)

Create or update `~/.copilot/mcp.json`:

> **IMPORTANT:** Replace `/home/yourname` with your actual home directory (Linux/WSL) or `C:\\Users\\yourname` (Windows). JSON doesn't expand `$HOME` or `~`.

### Windows (native) config

```json
{
  "mcpServers": {
    "prom-collector-tsg": {
      "command": "node",
      "args": ["C:\\Users\\yourname\\go\\src\\prometheus-collector\\tools\\prom-collector-tsg-mcp\\dist\\index.js"]
    },
    "icm": {
      "command": "agency",
      "args": ["mcp", "remote", "--url", "https://icmmcpprod-centralus.azurewebsites.net/"]
    },
    "azure-devops": {
      "command": "npx",
      "args": ["-y", "@azure-devops/mcp", "msazure"]
    },
    "ado-tracing": {
      "command": "agency",
      "args": ["mcp", "ado", "--organization", "msazure"]
    },
    "enghub": {
      "command": "enghub-mcp",
      "args": ["start"]
    },
    "es-chat": {
      "command": "agency",
      "args": ["mcp", "es-chat"]
    },
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"]
    }
  }
}
```

> **Windows notes:**
> - For ICM portal browsing, use `tsg_icm_page` (not Playwright). It connects to Edge on `localhost:9222` and reliably extracts the authored summary via CDP network interception. See Section 1 for Edge CDP setup.
> - Playwright MCP works for general web browsing but **hangs on the ICM portal** (heavy SPA with lazy-rendered content). Use it for dashboards, Grafana, etc. — not ICM.
> - If Playwright fails to launch, run `npx playwright install msedge` to install the Edge browser. Chrome is not installed by default on Windows — use `--browser msedge`.
> - EngHub MCP works natively with Windows keychain (no gnome-keyring needed)
> - `azureauth` is typically pre-installed on corp Windows machines (no shim needed)

### WSL2 (Linux) config

```json
{
  "mcpServers": {
    "prom-collector-tsg": {
      "command": "node",
      "args": ["/home/yourname/go/src/prometheus-collector/tools/prom-collector-tsg-mcp/dist/index.js"]
    },
    "icm": {
      "command": "agency",
      "args": ["mcp", "remote", "--url", "https://icmmcpprod-centralus.azurewebsites.net/"]
    },
    "azure-devops": {
      "command": "npx",
      "args": ["-y", "@azure-devops/mcp", "msazure"]
    },
    "ado-tracing": {
      "command": "agency",
      "args": ["mcp", "ado", "--organization", "msazure"]
    },
    "enghub": {
      "command": "enghub-mcp",
      "args": ["start"],
      "env": {
        "LD_LIBRARY_PATH": "/home/yourname/.local/libsecret/usr/lib/x86_64-linux-gnu"
      }
    },
    "es-chat": {
      "command": "agency",
      "args": ["mcp", "es-chat"]
    },
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest", "--browser", "chrome", "--executable-path", "/usr/bin/google-chrome"],
      "env": {
        "CHROMIUM_FLAGS": "--disable-gpu"
      }
    }
  }
}
```

> **WSL2 notes:**
> - Playwright needs explicit `--browser chrome --executable-path /usr/bin/google-chrome` and `CHROMIUM_FLAGS: --disable-gpu` (see Section 1 for Chrome install)
> - Playwright in WSL can browse most sites, but **ICM portal auth is unreliable** from WSL Playwright. Use `tsg_icm_page` instead — it works on both Windows and WSL2 (see Section 1 for Edge CDP setup)
> - EngHub MCP needs `LD_LIBRARY_PATH` pointing to libsecret (see Section 8)
> - `azureauth` usually needs a shim script (see Section 8)

### ICM MCP Connection Gotcha

> **CRITICAL:** The ICM MCP server must use the **direct backend URL**, NOT the APIM gateway:
> - ✅ `https://icmmcpprod-centralus.azurewebsites.net/` — works
> - ❌ `https://icmmcpprod.azurewebsites.net/` — fails (APIM gateway incompatible with agency's HTTP client)
>
> If you see connection errors with ICM, this is almost always the cause.

### ICM MCP API Limitations

The ICM MCP server provides several tools for querying incidents. Be aware of these limitations:

| Tool | Returns | Limitations |
|------|---------|-------------|
| `icm-get_incident_details_by_id` | Severity, state, owning team, howFixed, mitigation, tags, time range | ❌ Does NOT return the authored summary / description body |
| `icm-get_ai_summary` | AI-generated summary (often quotes cluster ARM ID) | May return "No AI summary available" for some incidents |
| `icm-get_incident_context` | Rich structured data: description, symptoms, discussion, Kusto queries | ⚠️ Works ~60% of the time; returns "Error fetching context" for others |
| `icm-get_incident_location` | Region, cluster, datacenter | Usually works |

**Key limitation:** The raw HTML authored summary visible in the ICM portal is NOT returned by any ICM MCP API tool. To get the full authored description (which usually contains the cluster ARM resource ID), you must either:
1. **Use `tsg_icm_page`** (works on both Windows and WSL2) — connects to Edge via CDP, intercepts the raw API responses on page reload, and extracts the authored summary + discussion entries. Requires Edge running with `--remote-debugging-port=9222` (see Section 1).
2. Rely on `icm-get_ai_summary` which often quotes the ARM ID from the authored summary
3. Check `icm-get_incident_context` → `DescriptionEntriesSummary` field (when available)
4. Ask the user to paste it from the ICM portal

### What each server provides

| Server | Type | Purpose |
|--------|------|---------|
| **prom-collector-tsg** | stdio | TSG diagnostic queries — triage, errors, config, workload, pods, logs, control plane (124 KQL queries across 7 Kusto clusters) |
| **icm** | stdio | IcM incident management — query incident details, get AI summaries, check customer impact |
| **azure-devops** | stdio | ADO repos, PRs, work items, pipelines, wikis, search |
| **ado-tracing** | stdio | ADO with tracing org context — repos, PRs, work items for msazure org |
| **enghub** | stdio | Engineering Hub — docs, TSGs, ServiceTree |
| **es-chat** | stdio | Engineering Systems Chat — internal knowledge search |
| **playwright** | stdio | Browser automation — navigate, click, screenshot dashboards. For ICM portal browsing, prefer `tsg_icm_page` (faster and more reliable via CDP network interception) |

### Optional servers

```bash
agency mcp --help  # See full list of available agency MCP servers
```

Other useful servers:
- `geneva-mdm` (HTTP, port 5050) — Geneva MDM metrics, requires separate setup (see Section 5)
- `kusto` — query Kusto/ADX directly
- `workiq` — M365 Copilot integration
- `aspire` — .NET Aspire integration

### Copilot Skills

The troubleshooting skill at `.github/skills/prom-collector-tsg/SKILL.md` provides a complete investigation workflow including:
- TSG decision trees for all symptom categories (OOMKill, missing metrics, spikes, private link, etc.)
- Node pool capacity and autoscaler analysis workflow
- Pod-to-node placement checks for system pool bottlenecks
- Escalation contacts for each issue type

---

## 7. Configure VS Code MCP Servers

Create `.vscode/mcp.json` in the repo for VS Code Copilot integration:

### Windows (native) VS Code config

```json
{
  "servers": {
    "prom-collector-tsg": {
      "type": "stdio",
      "command": "node",
      "args": ["${workspaceFolder}/tools/prom-collector-tsg-mcp/dist/index.js"]
    },
    "azure-devops": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@azure-devops/mcp", "msazure"]
    },
    "icm": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "remote", "--url", "https://icmmcpprod-centralus.azurewebsites.net/"]
    },
    "ado-tracing": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "ado", "--organization", "msazure"]
    },
    "enghub": {
      "type": "stdio",
      "command": "enghub-mcp",
      "args": ["start"]
    },
    "es-chat": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "es-chat"]
    },
    "playwright": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"]
    }
  }
}
```

### WSL2 (Linux) VS Code config

```json
{
  "servers": {
    "prom-collector-tsg": {
      "type": "stdio",
      "command": "node",
      "args": ["${workspaceFolder}/tools/prom-collector-tsg-mcp/dist/index.js"]
    },
    "azure-devops": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@azure-devops/mcp", "msazure"]
    },
    "icm": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "remote", "--url", "https://icmmcpprod-centralus.azurewebsites.net/"]
    },
    "ado-tracing": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "ado", "--organization", "msazure"]
    },
    "enghub": {
      "type": "stdio",
      "command": "enghub-mcp",
      "args": ["start"]
    },
    "es-chat": {
      "type": "stdio",
      "command": "agency",
      "args": ["mcp", "es-chat"]
    },
    "playwright": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest", "--browser", "chrome", "--executable-path", "/usr/bin/google-chrome"],
      "env": {
        "CHROMIUM_FLAGS": "--disable-gpu"
      }
    }
  }
}
```

---

## 8. Authentication

### Azure CLI

```bash
# Login (opens browser for auth)
az login

# Verify
az account show --query "{name:name, id:id}" -o table
```

### Kusto cluster access

The TSG MCP server uses `DefaultAzureCredential` which chains:
1. Environment variables → 2. Managed Identity → 3. Azure CLI → 4. VS Code

For local dev, `az login` is sufficient. The credential is used to query:
- App Insights (ContainerInsightsPrometheusCollector-Prod)
- Kusto clusters (metricsinsights, appinsightstlm, akshuba, shavulnmgmtprdwus)

If you get `403 Forbidden` on a Kusto query, you need to request access to that cluster via [JIT/MyAccess](https://myaccess.microsoft.com/).

### Geneva MDM access

The MDM MCP server also uses `DefaultAzureCredential`. It authenticates against the Geneva metrics API. If you see `Unauthorized` errors, ensure:

```bash
# You're in the right tenant
az account set --subscription "your-subscription-id"

# Refresh token
az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv
```

### ICM MCP Authentication (azureauth)

The ICM MCP server (`agency mcp remote`) uses the `azureauth` CLI to acquire Entra ID tokens.

**On Windows:** `azureauth` is typically pre-installed on corp machines. No extra setup needed.

**On WSL2/Linux:** `azureauth` is often **not installed** because it's distributed via internal NuGet feeds that require pre-existing auth.

**Symptoms of missing azureauth (WSL2):**
```
MCP error -32603: Internal error: Failed to get access token: Failed to get EntraID token: Failed to run AzureAuth: azureauth failed with exit status exit status: 1
```

**Fix: Create an azureauth shim that delegates to Azure CLI:**

```bash
mkdir -p ~/.local/bin

cat > ~/.local/bin/azureauth << 'SCRIPT'
#!/bin/bash
# azureauth shim — delegates to az CLI for token acquisition
# agency mcp remote calls: azureauth aad --scope <scope> --client <id> --tenant <id> --mode Web --output token

SCOPE=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        aad) shift ;;
        --scope) SCOPE="$2"; shift 2 ;;
        --client|--tenant|--mode|--output) shift 2 ;;
        *) shift ;;
    esac
done

if [[ -z "$SCOPE" ]]; then
    echo "Error: --scope is required" >&2
    exit 1
fi

# Convert scope to resource (strip /.default suffix)
RESOURCE="${SCOPE%/.default}"
RESOURCE="${RESOURCE%/}"

TOKEN=$(az account get-access-token --resource "$RESOURCE" --query accessToken --output tsv 2>/dev/null)
if [[ -z "$TOKEN" ]]; then
    TOKEN=$(az account get-access-token --resource "$SCOPE" --query accessToken --output tsv 2>/dev/null)
fi

if [[ -z "$TOKEN" ]]; then
    echo "Error: Failed to get token for scope $SCOPE via az CLI" >&2
    exit 1
fi

echo "$TOKEN"
SCRIPT

chmod +x ~/.local/bin/azureauth
```

Ensure `~/.local/bin` is in your PATH (add to `~/.bashrc` if needed):
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Verify:** After creating the shim, test ICM MCP connectivity in Copilot CLI:
```
"Get details for ICM 12345678"
```

### EngHub MCP Authentication

**On Windows:** EngHub MCP works natively — Windows Credential Manager handles token caching. No extra setup needed.

**On WSL2/Linux:** The EngHub MCP (`enghub-mcp start`) uses MSAL device code flow + keytar for token caching. On headless Linux/WSL, it fails because keytar requires a D-Bus secret service.

**Symptoms (WSL2 only):**
```
Access denied to EngineeringHub. You may not have permission, or your token may have expired.
```

**Fix: Install gnome-keyring-daemon for the D-Bus secret service:**

```bash
# Download and extract gnome-keyring + dependencies (no sudo required)
mkdir -p ~/.local/gnome-keyring
cd /tmp

# Download packages for your distro (Ubuntu 22.04 example)
apt download gnome-keyring libgcr-base-3-1 libgck-1-0 2>/dev/null || \
  apt-get download gnome-keyring libgcr-base-3-1 libgck-1-0

for deb in gnome-keyring*.deb libgcr-base*.deb libgck*.deb; do
    dpkg-deb -x "$deb" ~/.local/gnome-keyring
done

# Start the keyring daemon
export XDG_RUNTIME_DIR=/tmp/keyring-$USER
mkdir -p "$XDG_RUNTIME_DIR"
LD_LIBRARY_PATH=~/.local/gnome-keyring/usr/lib/x86_64-linux-gnu \
  ~/.local/gnome-keyring/usr/bin/gnome-keyring-daemon \
  --start --components=secrets

# Also ensure libsecret is available for keytar
mkdir -p ~/.local/libsecret
apt download libsecret-1-0 2>/dev/null || apt-get download libsecret-1-0
dpkg-deb -x libsecret-1-0*.deb ~/.local/libsecret
```

Then update your EngHub MCP config to include the library path:
```json
"enghub": {
    "command": "enghub-mcp",
    "args": ["start"],
    "env": {
        "LD_LIBRARY_PATH": "/home/yourname/.local/libsecret/usr/lib/x86_64-linux-gnu"
    }
}
```

**First-time authentication:** EngHub uses device code flow. Watch the terminal output for a URL and code to enter at https://login.microsoft.com/device.

---

## 9. Verify Everything Works

### Step 1: Verify TSG MCP server

**Windows (PowerShell):**
```powershell
Set-Location $env:USERPROFILE\go\src\prometheus-collector\tools\prom-collector-tsg-mcp

# Send initialize + tools/list via JSON-RPC
$init = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
$list = '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
"$init`n$list" | node dist/index.js 2>$null | Select-String '"name"' | ForEach-Object { $_.Line.Trim() }
```

**WSL2 / Linux (bash):**
```bash
cd ~/go/src/prometheus-collector/tools/prom-collector-tsg-mcp

cat << 'EOF' | timeout 10 node dist/index.js 2>/dev/null | python3 -c "
import sys, json
for line in sys.stdin:
    try:
        d = json.loads(line.strip())
        if d.get('id') == 2:
            tools = d['result']['tools']
            print(f'{len(tools)} tools available:')
            for t in tools:
                print(f'  ✅ {t[\"name\"]}')
    except: pass
"
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
EOF
```

Expected: **11 tools** including `tsg_triage`, `tsg_errors`, `tsg_config`, `tsg_workload`, `tsg_pods`, `tsg_logs`, `tsg_control_plane`, `tsg_query`, `tsg_dashboard_link`, `tsg_mdm_throttling`, `tsg_icm_page`.

### Step 2: Verify Edge CDP for ICM browsing

**Windows (PowerShell):**
```powershell
# Check that Edge CDP is running
try {
    $ver = (Invoke-WebRequest -Uri "http://localhost:9222/json/version" -UseBasicParsing).Content | ConvertFrom-Json
    Write-Host "✅ Edge CDP: $($ver.Browser)"
} catch {
    Write-Host "❌ Edge CDP not running. Launch Edge with --remote-debugging-port=9222 (see Section 1)"
}

# Test tsg_icm_page scrape (replace 12345678 with a real ICM ID)
Set-Location $env:USERPROFILE\go\src\prometheus-collector\tools\prom-collector-tsg-mcp
node -e "require('./dist/icm-browser.js').scrapeICMIncident(12345678).then(r => console.log(r.substring(0, 200)))"
```

**WSL2 (bash):**
```bash
# Check that Edge CDP is reachable via port proxy
curl -s http://$(hostname).local:9223/json/version | python3 -m json.tool
# Should return Edge version info
```

### Step 3: Verify Geneva MDM MCP server

```bash
# Check it's running
ss -tlnp | grep 5050

# Test a query
curl -s -X POST http://localhost:5050/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | python3 -m json.tool | head -20
```

Expected: 7 MDM tools including `QueryDimensionMDM`, `GetNameSpaces`, `GetMetrics`, etc.

### Step 3: Test from Copilot CLI

```bash
cd ~/go/src/prometheus-collector
copilot

# Then ask:
# "Run triage for cluster /subscriptions/.../managedClusters/my-cluster"
# "Check MDM throttling for account GenevaQos"
```

---

## 10. Available Tools Reference

### TSG Tools (prom-collector-tsg) — 124 KQL queries

| Tool | Description | Key Params |
|------|-------------|------------|
| `tsg_triage` | Initial triage — version, region, AMW config, token adapter, DCR/DCE, CCP cluster ID, MDM account ID, node pool capacity, autoscaling history | `cluster`, `timeRange` |
| `tsg_errors` | Scan all error categories — container, OtelCollector, ME, MDSD, DNS, private link | `cluster`, `timeRange` |
| `tsg_config` | Check scrape configs, default targets, keep list, intervals, HPA, pod/service monitors | `cluster`, `timeRange` |
| `tsg_workload` | Workload health — replicas, samples/min, drops, CPU/mem, queues, HPA status, pod resource limits | `cluster`, `timeRange` |
| `tsg_pods` | Pod health — restarts, restart reasons, pod-to-node mapping, node status timeline, autoscaler events | `cluster`, `timeRange` |
| `tsg_logs` | Raw component logs (replicaset, linux-daemonset, windows-daemonset, configreader) | `cluster`, `component` |
| `tsg_control_plane` | Control plane — enabled jobs, keep list, minimal ingestion profile | `cluster`, `timeRange` |
| `tsg_query` | Run arbitrary KQL against any data source | `datasource`, `kql` |
| `tsg_dashboard_link` | Generate TSG ADX dashboard link for a cluster | `cluster` |
| `tsg_mdm_throttling` | Check Geneva MDM QoS throttling/drops/utilization (requires Geneva MDM MCP on port 5050) | `monitoringAccount`, `timeRangeHours` |

> **MDM Throttling Workflow:** Run `tsg_triage` first → extract `MDMAccountName` from the "MDM Account ID" result → pass it to `tsg_mdm_throttling` to check for Geneva account throttling.

### MDM Tools (geneva-mdm)

| Tool | Description |
|------|-------------|
| `GetNameSpaces` | List namespaces in a monitoring account |
| `GetMetrics` | List metrics in a namespace |
| `GetDimensions` | Get dimensions for a metric |
| `GetPreAggregateConfigurations` | Get pre-aggregation configs |
| `QueryDimensionMDM` | Query metric data with dimension filters |
| `KqlmQuery` | Execute KQLM queries against MDM time-series |
| `ReadMonitorInfo` | Get comprehensive monitor information |

### Other MCP Tools

| Server | Key Tools |
|--------|-----------|
| **azure-devops** | Search code/work items, manage PRs, query pipelines, read wikis |
| **icm** | Query incidents, create incidents, update severity/status |
| **enghub** | Search EngHub docs, fetch TSG pages, resolve services |
| **es-chat** | Ask engineering systems questions, search internal KB |
| **playwright** | Navigate to dashboards, take screenshots, fill forms |

---

## 11. Troubleshooting Workflow

### ICM Investigation Flow

When an ICM incident comes in for prometheus-collector:

1. **Get the cluster ARM resource ID** from the ICM incident
2. **Run triage**: `tsg_triage` — identifies version, region, AMW config, MDM account, CCP cluster ID
3. **Check MDM throttling**: extract `MDMAccountName` from triage → `tsg_mdm_throttling` — verifies no account-level throttling
4. **Check errors**: `tsg_errors` — scans all error categories
5. **Check workload health**: `tsg_workload` — samples/min, drops, CPU/memory, HPA status
6. **Check pod health**: `tsg_pods` — restarts, reasons, pod-to-node mapping, autoscaler events
7. **Dive into logs**: `tsg_logs` — raw logs from specific components
8. **Check configs**: `tsg_config` — scrape configs, intervals, targets
9. **Generate dashboard link**: `tsg_dashboard_link` — for visual investigation

### MDM QoS Metrics Checked by `tsg_mdm_throttling`

These match the [Geneva QoS Dashboard](https://portal.microsoftgeneva.com/dashboard/mac_91c1e6c2-bcdf-4650-9f80-179b245c2533/GenevaQos/%E2%86%90%20MdmQos):

| Check | Metric | What it means |
|-------|--------|---------------|
| Incoming Events Throttled | `ThrottledClientMetricCount` | Events rejected due to rate limits |
| Incoming Events Dropped | `DroppedClientMetricCount` | Events lost before ingestion |
| MStore Time Series Throttled | `ThrottledTimeSeriesCount` | Time series exceeding account limit |
| MStore Samples Dropped | `MStoreDroppedSamplesCount` | Samples lost in MStore |
| Queries Throttled | `ThrottledQueriesCount` | Read queries being rate limited |
| Event Volume vs Limit | `ClientAggregatedMetricCount` / `Limit` | % of ingestion capacity used |
| Time Series vs Limit | `MStoreActiveTimeSeriesCount` / `Limit` | % of time series capacity used |

---

## 12. Quick Reference

### Start everything

```bash
# 1. Login
az login

# 2. Start Copilot CLI from repo root
cd ~/go/src/prometheus-collector
copilot
```

> **Note:** The TSG MCP server starts automatically as a stdio process when Copilot calls it. No manual server start needed.
> If you also want the Geneva MDM MCP server, see Section 5 for one-time setup.

### Rebuild TSG MCP after changes

```bash
cd ~/go/src/prometheus-collector/tools/prom-collector-tsg-mcp

# If system Node.js isn't installed, use agency's:
# export PATH="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin:$PATH"

npx tsc
# Copilot CLI automatically picks up the new dist/index.js on next tool call
```

### Check MDM MCP server status

```bash
# Is it running?
ss -tlnp | grep 5050

# View logs
tail -50 /tmp/mdm-mcp.log

# Restart if needed
kill $(ss -tlnp | grep 5050 | grep -oP 'pid=\K\d+') 2>/dev/null
# Then re-run the nohup command from above
```

### Common cluster ID format

```
/subscriptions/{sub-id}/resourceGroups/{rg}/providers/Microsoft.ContainerService/managedClusters/{cluster-name}
```

### Useful Copilot prompts

```
"Triage cluster /subscriptions/xxx/resourceGroups/yyy/providers/Microsoft.ContainerService/managedClusters/zzz"
"Check MDM throttling for account GenevaQos over the last 24 hours"
"What errors are happening on cluster X in the last 6 hours?"
"Show me the scrape config for cluster X"
"Get replicaset logs for cluster X"
"Check if control plane metrics are enabled for cluster X"
```

---

## 13. Setup Troubleshooting

When this skill is invoked to help with setup problems, **run the diagnostic commands yourself and fix the issue** — don't just tell the user what to do.

### setup.sh fails: "Node.js not found"

```bash
# Check if Agency bundled Node.js
ls -la ~/.agency/nodejs/*/bin/node
# If found, add to PATH and re-run
export PATH="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin:$PATH"
bash tools/prom-collector-tsg-mcp/setup.sh
```

If no Node.js at all:
- **Windows:** `winget install OpenJS.NodeJS.LTS`
- **WSL2:** `curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash - && sudo apt-get install -y nodejs`

### setup.sh fails: npm install or tsc errors

```bash
cd tools/prom-collector-tsg-mcp
rm -rf node_modules dist
npm install
npx tsc
```

### MCP tools don't appear in Copilot

1. **Check mcp.json path is correct:**
```bash
cat ~/.copilot/mcp.json | python3 -m json.tool
# Verify dist/index.js exists at the path listed
```

2. **Test MCP server directly:**
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | timeout 5 node tools/prom-collector-tsg-mcp/dist/index.js 2>/dev/null
```

3. **Re-run setup.sh** to rebuild and regenerate config.

### ICM MCP: "Failed to get access token" / azureauth

**Windows:** `azureauth` should be pre-installed. If not, ensure you're on a corp machine.

**WSL2:** Verify the shim exists and az is logged in:
```bash
which azureauth          # should be ~/.local/bin/azureauth
az login                 # ensure logged in
az account get-access-token --resource api://icm.microsoft.com --query accessToken -o tsv
```

If shim is missing, re-run `setup.sh`.

### ICM MCP: connection errors / APIM gateway

Must use direct backend URL. Check `agency.toml` has `icm = true` (uses correct URL automatically). If using custom config, verify: `https://icmmcpprod-centralus.azurewebsites.net/` (NOT `icmmcpprod.azurewebsites.net`).

### Built-in MCPs (ado, icm, es-chat) not loading

These come from `agency.toml` at the repo root — Agency auto-discovers it when you run `agency copilot` from inside the repo.
```bash
cat agency.toml          # should show [mcps.builtins]
agency config list       # should find the file
pwd                      # must be inside the repo
```

### Skills not loading

Skills auto-load from `.github/skills/` when cwd is inside the repo:
```bash
ls .github/skills/*/SKILL.md   # verify skills exist
pwd                              # must be inside the repo
```

If running copilot from outside the repo, copy skills globally:
```bash
cp -r .github/skills/* ~/.copilot/skills/
```

### Playwright / browser issues

**Windows:**
- Playwright MCP works for general browsing (dashboards, Grafana, Azure portal)
- **Do NOT use Playwright for ICM portal** — the SPA is too heavy and `browser_snapshot` hangs. Use `tsg_icm_page` instead
- If Playwright fails to find a browser, run `npx playwright install msedge` (Chrome is not installed by default on Windows)
- If using `--browser msedge` and it says "Opening in existing browser session", that means Edge merged into the running session. Playwright needs its own profile; this is handled automatically by the Playwright MCP

**WSL2:** Needs Google Chrome:
```bash
which google-chrome   # check if installed
# If not:
wget -q https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
sudo dpkg -i google-chrome-stable_current_amd64.deb && sudo apt-get install -f
```

For ICM portal browsing on **both platforms**, use `tsg_icm_page` instead of Playwright — it works on both Windows and WSL2 (see Section 1 for Edge CDP setup).

### Edge CDP / tsg_icm_page issues

**Edge CDP not responding (`localhost:9222`):**
1. Verify Edge is running with CDP: `Invoke-WebRequest http://localhost:9222/json/version` (PowerShell) or `curl localhost:9222/json/version` (bash)
2. If no response, launch Edge with `--remote-debugging-port=9222` (see Section 1)
3. **Profile locking:** If Edge says "Opening in existing browser session" → you're using a `--user-data-dir` that's already in use by another Edge process. Use a unique directory (e.g., `C:\Users\<user>\.edge-cdp-debug`)
4. **Edge merges into existing session:** When you launch `msedge.exe` with a data-dir that already has an Edge process, the new instance sends a message to the existing one and exits. The CDP port becomes inaccessible. Solution: ensure no other Edge instance uses that profile directory, or pick a fresh `--user-data-dir`

**ICM shows blank / auth required:**
- Sign in to ICM manually in the CDP Edge window first (navigate to `https://portal.microsofticm.com`)
- Windows Entra ID SSO with FIDO keys works even in the separate CDP Edge profile
- After initial sign-in, the session persists across page reloads

**tsg_icm_page returns empty summary:**
- The tool intercepts `GetIncidentDetails` and `getdescriptionentries` API calls during page reload. If ICM loads from cache without XHR, it may miss the data
- Try with a different ICM ID to rule out incident-specific issues
- Check `CDP_ENDPOINT` env var — on Windows it should be unset (auto-detects `localhost:9222`); on WSL2 it auto-detects `172.29.112.1:9223`

### tsg_icm_page: "Cannot connect to Edge browser via CDP"

This means the Edge CDP connection from WSL2 to the Windows host isn't working. Debug step by step:

```bash
# 1. Is Edge running with the debugging port?
powershell.exe -Command "Get-CimInstance Win32_Process -Filter \"name='msedge.exe'\" | Where-Object { \$_.CommandLine -match 'remote-debugging-port' } | Select-Object ProcessId" 2>/dev/null
# If empty, Edge wasn't launched with --remote-debugging-port=9222

# 2. Does CDP work on the Windows side?
powershell.exe -Command "(Invoke-WebRequest -Uri 'http://localhost:9222/json/version' -UseBasicParsing -TimeoutSec 3).Content" 2>/dev/null
# Should return JSON with "Browser": "Edg/..."

# 3. What's your WSL gateway IP?
ip route show default | awk '{print $3}'
# e.g., 172.24.208.1

# 4. Is the port proxy forwarding correctly?
WSL_GATEWAY=$(ip route show default | awk '{print $3}')
curl -s --connect-timeout 3 "http://${WSL_GATEWAY}:9223/json/version"
# If empty, port proxy isn't set up or firewall is blocking
```

**Common fixes:**
- **Edge not launched with CDP**: Relaunch from WSL:
  ```bash
  powershell.exe -Command "Stop-Process -Name msedge -Force -ErrorAction SilentlyContinue"
  sleep 2
  powershell.exe -Command "Start-Process 'C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe' -ArgumentList '--remote-debugging-port=9222','--user-data-dir=C:\Users\$USER\.playwright-mcp-edge3','--no-first-run'"
  ```
- **Port proxy not set up** (needs admin — will trigger UAC prompt):
  ```bash
  powershell.exe -Command "Start-Process powershell -Verb RunAs -ArgumentList '-Command','netsh interface portproxy add v4tov4 listenport=9223 listenaddress=0.0.0.0 connectport=9222 connectaddress=127.0.0.1; netsh advfirewall firewall add rule name=\"\"\"WSL Edge CDP\"\"\" dir=in action=allow protocol=TCP localport=9223'"
  ```
- **Gateway IP mismatch**: The tool auto-detects the gateway via `ip route`. If it doesn't work, override with:
  ```bash
  export CDP_ENDPOINT="http://$(ip route show default | awk '{print $3}'):9223"
  ```
- **Re-run setup.sh**: This automates all of the above:
  ```bash
  bash tools/prom-collector-tsg-mcp/setup.sh
  ```

### Kusto queries: "403 Forbidden"

```bash
az login                              # ensure logged in
az account show --query tenantId -o tsv  # must be Microsoft tenant
```

If still 403, request JIT access via [MyAccess](https://myaccess.microsoft.com/).

### Geneva MDM MCP not working

Optional — not set up by `setup.sh`. See Section 5. Common issues:
- Not running: `ss -tlnp | grep 5050`
- Needs .NET 8 SDK and private NuGet feed auth
- Hangs on startup: must set `DOTNET_EnableDiagnostics=0`
