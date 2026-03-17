<#
.SYNOPSIS
  Start Edge with Chrome DevTools Protocol (CDP) remote debugging enabled
  and set up the netsh port proxy for WSL2 access.

.DESCRIPTION
  This script:
  1. Sets up netsh port proxy (9223 → 9222) if not already configured
  2. Adds a firewall rule for WSL access if not already present
  3. Launches Edge with --remote-debugging-port=9222

  After running this script, the prom-collector-tsg MCP server's tsg_icm_page
  tool can connect from WSL2 to scrape ICM incident pages.

.NOTES
  Run as Administrator for port proxy and firewall setup.
  Edge must not already be running, or use a separate --user-data-dir.
#>

param(
    [int]$CDPPort = 9222,
    [int]$ProxyPort = 9223,
    [string]$UserDataDir = "$env:USERPROFILE\.edge-cdp"
)

$ErrorActionPreference = "Continue"

# 1. Check/set up netsh port proxy
Write-Host "Checking port proxy ($ProxyPort -> $CDPPort)..." -ForegroundColor Cyan
$existing = netsh interface portproxy show v4tov4 2>$null | Select-String "$ProxyPort"
if (-not $existing) {
    Write-Host "  Setting up port proxy..." -ForegroundColor Yellow
    $result = Start-Process netsh -ArgumentList "interface portproxy add v4tov4 listenport=$ProxyPort listenaddress=0.0.0.0 connectport=$CDPPort connectaddress=127.0.0.1" -Verb RunAs -PassThru -Wait
    if ($result.ExitCode -eq 0) {
        Write-Host "  Port proxy configured." -ForegroundColor Green
    } else {
        Write-Host "  Failed to set port proxy (need Admin). Run as Administrator." -ForegroundColor Red
    }
} else {
    Write-Host "  Port proxy already configured." -ForegroundColor Green
}

# 2. Check/add firewall rule
Write-Host "Checking firewall rule..." -ForegroundColor Cyan
$fwRule = Get-NetFirewallRule -DisplayName "Edge CDP for WSL" -ErrorAction SilentlyContinue
if (-not $fwRule) {
    Write-Host "  Adding firewall rule..." -ForegroundColor Yellow
    try {
        New-NetFirewallRule -DisplayName "Edge CDP for WSL" -Direction Inbound -LocalPort $ProxyPort -Protocol TCP -Action Allow -ErrorAction Stop | Out-Null
        Write-Host "  Firewall rule added." -ForegroundColor Green
    } catch {
        Write-Host "  Failed to add firewall rule (need Admin). Run as Administrator." -ForegroundColor Red
    }
} else {
    Write-Host "  Firewall rule already exists." -ForegroundColor Green
}

# 3. Find Edge executable
$edgePaths = @(
    "${env:ProgramFiles(x86)}\Microsoft\Edge\Application\msedge.exe",
    "$env:ProgramFiles\Microsoft\Edge\Application\msedge.exe",
    "$env:LOCALAPPDATA\Microsoft\Edge\Application\msedge.exe"
)
$edgeExe = $edgePaths | Where-Object { Test-Path $_ } | Select-Object -First 1

if (-not $edgeExe) {
    Write-Host "ERROR: Microsoft Edge not found." -ForegroundColor Red
    exit 1
}

# 4. Check if Edge is already running with CDP
try {
    $response = Invoke-WebRequest -Uri "http://127.0.0.1:$CDPPort/json/version" -TimeoutSec 2 -ErrorAction Stop
    Write-Host ""
    Write-Host "Edge is already running with CDP on port $CDPPort" -ForegroundColor Green
    $version = $response.Content | ConvertFrom-Json
    Write-Host "  Browser: $($version.Browser)" -ForegroundColor Gray
    Write-Host "  WSL endpoint: http://<wsl-host-ip>:$ProxyPort" -ForegroundColor Gray
    Write-Host ""
    Write-Host "Ready for tsg_icm_page!" -ForegroundColor Green
    exit 0
} catch {
    # Not running, proceed to launch
}

# 5. Launch Edge
Write-Host ""
Write-Host "Launching Edge with CDP on port $CDPPort..." -ForegroundColor Cyan
Write-Host "  User data dir: $UserDataDir" -ForegroundColor Gray

Start-Process $edgeExe -ArgumentList @(
    "--remote-debugging-port=$CDPPort",
    "--user-data-dir=$UserDataDir",
    "--no-first-run"
)

# 6. Wait for CDP to be available
Write-Host "Waiting for CDP endpoint..." -ForegroundColor Cyan
$maxWait = 15
$waited = 0
while ($waited -lt $maxWait) {
    Start-Sleep -Seconds 1
    $waited++
    try {
        $response = Invoke-WebRequest -Uri "http://127.0.0.1:$CDPPort/json/version" -TimeoutSec 2 -ErrorAction Stop
        Write-Host ""
        Write-Host "Edge CDP is ready!" -ForegroundColor Green
        Write-Host "  Local:  http://127.0.0.1:$CDPPort" -ForegroundColor Gray
        Write-Host "  WSL:    http://<wsl-host-ip>:$ProxyPort" -ForegroundColor Gray
        Write-Host ""
        Write-Host "Sign in to ICM if needed, then use tsg_icm_page." -ForegroundColor Yellow
        exit 0
    } catch {
        Write-Host "." -NoNewline
    }
}

Write-Host ""
Write-Host "WARNING: Edge launched but CDP not responding after ${maxWait}s." -ForegroundColor Yellow
Write-Host "Edge may already be running. Close all Edge windows and try again," -ForegroundColor Yellow
Write-Host "or use a different --user-data-dir." -ForegroundColor Yellow
