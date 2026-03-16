#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Prometheus-Collector TSG Environment Setup
# ============================================================================
# Run this from the repo after cloning. It handles things that can't be
# checked in as-is (paths, builds, auth workarounds).
#
# What's already in the repo (no setup needed):
#   agency.toml          → built-in MCPs (ado, icm, es-chat) — auto-discovered
#   .github/skills/      → skills (prom-collector-tsg, troubleshooting-setup)
#   .vscode/mcp.json     → VS Code MCP config
#   .github/copilot-instructions.md → repo context for Copilot
#
# What this script does:
#   1. Detects platform (Windows vs WSL2)
#   2. Finds Node.js (system or Agency-bundled)
#   3. Builds the TSG MCP server
#   4. Writes ~/.copilot/mcp.json with correct absolute paths
#   5. Creates azureauth shim (WSL2 only)
#   6. Verifies connectivity
#
# Usage:
#   bash tools/prom-collector-tsg-mcp/setup.sh           # interactive
#   bash tools/prom-collector-tsg-mcp/setup.sh --yes     # non-interactive
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MCP_CONFIG="$HOME/.copilot/mcp.json"
AUTO_YES="${1:-}"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail()  { echo -e "${RED}[FAIL]${NC} $*"; }

confirm() {
    if [[ "$AUTO_YES" == "--yes" ]]; then return 0; fi
    read -rp "$1 [Y/n] " ans
    [[ -z "$ans" || "$ans" =~ ^[Yy] ]]
}

echo ""
echo "=========================================="
echo " Prometheus-Collector TSG Setup"
echo "=========================================="
echo ""

# --- Detect platform ---
PLATFORM="wsl2"
if [[ "$(uname -s)" == "Linux" ]]; then
    grep -qi microsoft /proc/version 2>/dev/null && PLATFORM="wsl2" || PLATFORM="linux"
elif [[ "$(uname -s)" == MINGW* || "$(uname -s)" == MSYS* || "$(uname -s)" == CYGWIN* ]] || command -v powershell.exe &>/dev/null; then
    PLATFORM="windows"
fi
[[ "$PLATFORM" == "linux" ]] && PLATFORM="wsl2"  # treat bare Linux same as WSL2
info "Platform: $PLATFORM"

# --- Find Node.js ---
NODE_BIN=""
if command -v node &>/dev/null; then
    NODE_BIN="$(command -v node)"
elif [[ -x "$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin/node" ]]; then
    NODE_BIN="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin/node"
    export PATH="$HOME/.agency/nodejs/node-v22.21.0-linux-x64/bin:$PATH"
else
    fail "Node.js not found. Install it or install Agency CLI (which bundles Node.js)."
    exit 1
fi
ok "Node.js: $NODE_BIN ($(node --version))"

# --- Check Azure CLI ---
if command -v az &>/dev/null; then
    ok "Azure CLI: $(az --version 2>/dev/null | head -1)"
else
    warn "Azure CLI not found — run: curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash"
fi

# --- Build TSG MCP server ---
info "Building TSG MCP server..."
cd "$SCRIPT_DIR"
[[ -d "node_modules" ]] || npm install --quiet 2>&1 | tail -3
npx tsc 2>&1
if [[ -f "dist/index.js" ]]; then
    ok "Built: $SCRIPT_DIR/dist/index.js"
else
    fail "Build failed"; exit 1
fi

# --- Write ~/.copilot/mcp.json ---
# This is the only file that MUST live outside the repo because it needs
# absolute paths to the built MCP server binary.
# Built-in MCPs (ado, icm, es-chat) are in agency.toml and auto-discovered.
info "Writing MCP config..."
mkdir -p "$(dirname "$MCP_CONFIG")"

if [[ -f "$MCP_CONFIG" ]] && ! confirm "Overwrite existing $MCP_CONFIG?"; then
    ok "Keeping existing MCP config"
else
    [[ -f "$MCP_CONFIG" ]] && cp "$MCP_CONFIG" "${MCP_CONFIG}.bak" && info "Backed up to ${MCP_CONFIG}.bak"

    if [[ "$PLATFORM" == "windows" ]]; then
        # Windows: Playwright works natively for ICM browsing
        cat > "$MCP_CONFIG" << MCPJSON
{
  "mcpServers": {
    "prom-collector-tsg": {
      "command": "node",
      "args": ["$(echo "$SCRIPT_DIR/dist/index.js" | sed 's/\\/\\\\/g')"]
    },
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"]
    }
  }
}
MCPJSON
    else
        # WSL2/Linux: Playwright needs Chrome + disable-gpu
        cat > "$MCP_CONFIG" << MCPJSON
{
  "mcpServers": {
    "prom-collector-tsg": {
      "command": "$NODE_BIN",
      "args": ["$SCRIPT_DIR/dist/index.js"]
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
MCPJSON
    fi
    ok "Wrote $MCP_CONFIG (prom-collector-tsg + playwright)"
    info "Built-in MCPs (ado, icm, es-chat) are in agency.toml — auto-discovered when you run copilot from the repo"
fi

# --- WSL2: azureauth shim ---
if [[ "$PLATFORM" == "wsl2" ]] && ! command -v azureauth &>/dev/null; then
    info "Creating azureauth shim (WSL2)..."
    mkdir -p "$HOME/.local/bin"
    cat > "$HOME/.local/bin/azureauth" << 'AUTHSCRIPT'
#!/bin/bash
SCOPE=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        aad) shift ;; --scope) SCOPE="$2"; shift 2 ;;
        --client|--tenant|--mode|--output) shift 2 ;; *) shift ;;
    esac
done
[[ -z "$SCOPE" ]] && { echo "Error: --scope required" >&2; exit 1; }
RESOURCE="${SCOPE%/.default}"; RESOURCE="${RESOURCE%/}"
TOKEN=$(az account get-access-token --resource "$RESOURCE" --query accessToken --output tsv 2>/dev/null)
[[ -z "$TOKEN" ]] && TOKEN=$(az account get-access-token --resource "$SCOPE" --query accessToken --output tsv 2>/dev/null)
[[ -z "$TOKEN" ]] && { echo "Error: Failed to get token" >&2; exit 1; }
echo "$TOKEN"
AUTHSCRIPT
    chmod +x "$HOME/.local/bin/azureauth"
    echo "$PATH" | grep -q "$HOME/.local/bin" || { echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$HOME/.bashrc"; export PATH="$HOME/.local/bin:$PATH"; }
    ok "azureauth shim installed"
fi

# --- Verify ---
if az account show &>/dev/null 2>&1; then
    ok "Azure CLI logged in"
else
    warn "Not logged in — run: az login"
fi

# --- Summary ---
echo ""
echo "=========================================="
echo " Setup Complete!"
echo "=========================================="
echo ""
echo "  To start investigating ICMs:"
echo ""
echo "    cd $REPO_ROOT"
echo "    agency copilot"
echo "    > \"Investigate ICM 12345678\""
echo ""
echo "  What's auto-discovered from the repo:"
echo "    agency.toml        → ado, icm, es-chat MCPs"
echo "    .github/skills/    → prom-collector-tsg, troubleshooting-setup"
echo ""
echo "  What's in ~/.copilot/mcp.json:"
echo "    prom-collector-tsg → TSG diagnostic queries"
echo "    playwright         → browser automation"
echo ""
if [[ "$PLATFORM" == "wsl2" ]]; then
    echo "  WSL2: For ICM portal browsing, start Edge on Windows with:"
    echo "    msedge --remote-debugging-port=9222"
    echo "  Then set up port proxy (admin PowerShell):"
    echo "    netsh interface portproxy add v4tov4 listenport=9223 \\"
    echo "      listenaddress=0.0.0.0 connectport=9222 connectaddress=127.0.0.1"
    echo ""
fi

