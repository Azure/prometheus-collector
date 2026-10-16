# ICM Investigation Quick Start

Investigate Azure Managed Prometheus ICMs with Copilot CLI.

## Setup

### 1. Install Agency CLI

**Windows (PowerShell):**
```powershell
winget install GitHub.AgencyCLI
```

**Linux / WSL2:**
```bash
curl -fsSL https://aka.ms/install-agency | bash
```

### 2. Clone the repo and start Copilot

```bash
git clone https://github.com/Azure/prometheus-collector.git
cd prometheus-collector
git checkout grwehner/tsg-tooling-and-devbox
agency copilot
```

### 3. Ask Copilot to finish setup

```
> Set up my troubleshooting environment
```

Copilot will install prerequisites, build the MCP server, configure auth, and verify connectivity. Restart Copilot after setup to load the new tools:

```
> exit
agency copilot
> Investigate ICM 12345678
```
