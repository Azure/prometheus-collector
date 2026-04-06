# Simple Helm upgrade for ama-metrics
# Prerequisites: Run local_testing_aks.ps1 first to generate Chart.yaml and values.yaml

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ChartDir = Join-Path $ScriptDir "..\otelcollector\deploy\addon-chart\azure-monitor-metrics-addon"
$ValuesFile = Join-Path $ChartDir "values.yaml"
$ExpectedCluster = "zane-metrics-custom-ns"
$ExpectedNamespace = "ama-metrics-zane-test"

# Extract namespace, image, and cluster from values.yaml
$ValuesContent = Get-Content -Path $ValuesFile -Raw
$Namespace = if ($ValuesContent -match 'namespace:\s*"([^"]+)"') { $Matches[1] } else { "unknown" }
$ImageTag = if ($ValuesContent -match 'ImageRepository:.*\r?\n\s*ImageTag:\s*(.+)') { $Matches[1].Trim() } else { "unknown" }
$ValuesCluster = if ($ValuesContent -match 'managedClusters/(\S+)') { $Matches[1].Trim('"') } else { "unknown" }

# Verify namespace in values.yaml matches expected
if ($Namespace -ne $ExpectedNamespace) {
    Write-Host "ERROR: Namespace in values.yaml '$Namespace' does not match expected '$ExpectedNamespace'"
    Write-Host "Update local_testing_aks.ps1 and regenerate values.yaml"
    exit 1
}

# Verify cluster in values.yaml matches expected
if ($ValuesCluster -ne $ExpectedCluster) {
    Write-Host "ERROR: Cluster in values.yaml '$ValuesCluster' does not match expected '$ExpectedCluster'"
    Write-Host "Update local_testing_aks.ps1 and regenerate values.yaml"
    exit 1
}

# Verify kubectl context matches expected
$CurrentContext = kubectl config current-context
if ($CurrentContext -notmatch $ExpectedCluster) {
    Write-Host "ERROR: kubectl context '$CurrentContext' does not match expected cluster '$ExpectedCluster'"
    Write-Host "Run: az aks get-credentials --resource-group zane-custom-ns --name $ExpectedCluster --overwrite-existing"
    exit 1
}

Write-Host "Cluster:   $CurrentContext"
Write-Host "Namespace: $Namespace"
Write-Host "Image:     $ImageTag"
Write-Host ""
$Reply = Read-Host "Proceed with helm upgrade? (yes/no)"
if ($Reply -ne "yes") {
    Write-Host "Cancelled."
    exit 0
}

helm upgrade --install ama-metrics $ChartDir --namespace $Namespace --values $ValuesFile
