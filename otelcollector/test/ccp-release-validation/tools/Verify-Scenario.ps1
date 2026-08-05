<#
.SYNOPSIS
  Verify which CCP control-plane scrape jobs are actually ingesting into an Azure Monitor
  Workspace, attributing each series to *our* patched ama-metrics-ccp agent.

.DESCRIPTION
  When validating a CCP image on an AKS standalone, the ama-metrics-ccp agent is patched so its
  CLUSTER env var points at the cx-1 underlay's resource ID (so addon-token-adapter can obtain a
  real MSI token). A side effect is that cx-1's OWN managed control plane also emits
  controlplane-* series under the SAME `cluster` label. The only distinguishing label is
  `instance` (the scraped pod name).

  This script resolves the live pod list of our CCP namespace and uses it to classify every
  returned series as OURS vs FOREIGN, then probes a job-unique metric for each target.

.EXAMPLE
  .\Verify-Scenario.ps1 -Kubeconfig .\cx-1.kubeconfig -CcpNamespace 6a7393d0e6eaaf00013c8824 `
      -QueryEndpoint https://<amw>.eastus2.prometheus.monitor.azure.com `
      -ClusterLabel standalone-260805w9xxil-cx-1 `
      -Scenario "incomplete v2"
#>
param(
  [Parameter(Mandatory = $true)][string]$Kubeconfig,
  [Parameter(Mandatory = $true)][string]$CcpNamespace,
  [Parameter(Mandatory = $true)][string]$QueryEndpoint,
  [Parameter(Mandatory = $true)][string]$ClusterLabel,
  [string]$Scenario = "unnamed"
)

$ErrorActionPreference = "Stop"

# Job-unique probe metrics from the CCP minimal ingestion profile keep-lists.
# NOTE: rest_client_requests_total is deliberately avoided - it appears in the etcd,
# cluster-autoscaler AND kube-controller-manager keep-lists and would be ambiguous.
$probe = [ordered]@{
  "controlplane-apiserver"               = "apiserver_request_total"
  "controlplane-etcd"                    = "etcd_server_has_leader"
  "controlplane-kube-scheduler"          = "scheduler_schedule_attempts_total"
  "controlplane-kube-controller-manager" = "workqueue_depth"
}

# 1. Resolve the control-plane pods in OUR CCP namespace.
$podsRaw = & kubectl get pods -n $CcpNamespace --kubeconfig $Kubeconfig `
  -o jsonpath="{range .items[*]}{.metadata.name}{'\n'}{end}" 2>$null
$myPods = @($podsRaw -split "`n" | Where-Object { $_ -ne "" })
if ($myPods.Count -eq 0) { throw "No pods found in namespace $CcpNamespace" }

$mine = [ordered]@{
  "controlplane-apiserver"               = @($myPods | Where-Object { $_ -like "kube-apiserver-*" })
  "controlplane-etcd"                    = @($myPods | Where-Object { $_ -like "etcd-$CcpNamespace-*" -and $_ -notlike "*backup-sidecar*" -and $_ -notlike "*etcd-operator*" })
  "controlplane-kube-scheduler"          = @($myPods | Where-Object { $_ -like "kube-scheduler-*" })
  "controlplane-kube-controller-manager" = @($myPods | Where-Object { $_ -like "kube-controller-manager-*" })
}

Write-Host ""
Write-Host "==================== SCENARIO: $Scenario ====================" -ForegroundColor Cyan
Write-Host "Queried at $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') | CCP namespace: $CcpNamespace"
Write-Host ""

# Record the image actually running at query time - guards against a reconciler having
# silently reverted the patched deployment mid-run.
$img = & kubectl get pods -n $CcpNamespace -l rsName=ama-metrics-ccp --kubeconfig $Kubeconfig `
  -o jsonpath="{range .items[*]}{.metadata.name}{' -> '}{.spec.containers[0].image}{'\n'}{end}" 2>$null
Write-Host "ama-metrics-ccp pod/image AT QUERY TIME:"
Write-Host "  $img"

Write-Host "Control-plane pods in OUR CCP namespace (the only valid 'ours' instances):"
foreach ($k in $mine.Keys) {
  $v = if ($mine[$k].Count) { $mine[$k] -join ", " } else { "<none>" }
  Write-Host ("  {0,-38} {1}" -f $k, $v)
}

# 2. Query the AMW for all controlplane-* targets.
$token = (az account get-access-token --resource "https://prometheus.monitor.azure.com" --query accessToken -o tsv)
if (-not $token) { throw "Failed to acquire AMW token" }
$headers = @{ Authorization = "Bearer $token" }

# NOTE: the AMW query API requires exactly one metric name - bare label-matcher queries
# like {job=~"controlplane-.*"} are rejected. Hence `up{...}`.
$query = "up{cluster=`"$ClusterLabel`",job=~`"controlplane-.*`"}"
$uri = "$($QueryEndpoint.TrimEnd('/'))/api/v1/query?query=$([System.Uri]::EscapeDataString($query))"
$r = Invoke-RestMethod -Uri $uri -Headers $headers -Method Get

$results = @{}
foreach ($s in $r.data.result) {
  $job = $s.metric.job
  if (-not $results.ContainsKey($job)) { $results[$job] = @{ mine = @(); other = @() } }
  if ($myPods -contains $s.metric.instance) { $results[$job].mine += $s.metric.instance }
  else { $results[$job].other += $s.metric.instance }
}

Write-Host ""
Write-Host "AMW series for job=~controlplane-.* , cluster=$ClusterLabel :"
Write-Host ("  {0,-38} {1,-12} {2}" -f "JOB", "OURS", "FOREIGN (cx-1's own control plane)")
foreach ($job in ($results.Keys | Sort-Object)) {
  Write-Host ("  {0,-38} {1,-12} {2}" -f $job, $results[$job].mine.Count, $results[$job].other.Count)
}

Write-Host ""
Write-Host "==> VERDICT (our CCP agent only):" -ForegroundColor Yellow
foreach ($j in $probe.Keys) {
  $n = if ($results.ContainsKey($j)) { $results[$j].mine.Count } else { 0 }
  $state = if ($n -gt 0) { "INGESTING ($n series)" } else { "absent" }
  Write-Host ("  {0,-38} {1}" -f $j, $state)
}

# 3. Prove real metric data (not just `up`) for each job.
Write-Host ""
Write-Host "Sample metric values (minimal-ingestion-profile metrics), ours only:"
foreach ($j in $probe.Keys) {
  $metric = $probe[$j]
  $q2 = "count by (instance) ($metric{cluster=`"$ClusterLabel`",job=`"$j`"})"
  $uri2 = "$($QueryEndpoint.TrimEnd('/'))/api/v1/query?query=$([System.Uri]::EscapeDataString($q2))"
  try {
    $r2 = Invoke-RestMethod -Uri $uri2 -Headers $headers -Method Get
    $ours = @($r2.data.result | Where-Object { $myPods -contains $_.metric.instance })
    if ($ours.Count -gt 0) {
      $detail = ($ours | ForEach-Object { "$($_.metric.instance)=$($_.value[1])" }) -join ", "
      Write-Host ("  {0,-34} {1,-38} OURS: {2}" -f $metric, $j, $detail)
    } else {
      Write-Host ("  {0,-34} {1,-38} OURS: <no data>" -f $metric, $j)
    }
  } catch {
    Write-Host ("  {0,-34} {1,-38} query error: {2}" -f $metric, $j, $_.Exception.Message)
  }
}
Write-Host ""
