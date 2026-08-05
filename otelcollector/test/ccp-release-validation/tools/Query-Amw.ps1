param(
  [Parameter(Mandatory=$true)][string]$QueryEndpoint,
  [Parameter(Mandatory=$true)][string]$Query,
  [string]$Time = ""
)

# Query an Azure Monitor Workspace Prometheus endpoint with PromQL.
$token = (az account get-access-token --resource "https://prometheus.monitor.azure.com" --query accessToken -o tsv)
if (-not $token) { Write-Error "Failed to acquire token"; exit 1 }

$uri = "$($QueryEndpoint.TrimEnd('/'))/api/v1/query?query=$([System.Uri]::EscapeDataString($Query))"
if ($Time) { $uri += "&time=$([System.Uri]::EscapeDataString($Time))" }

try {
  $resp = Invoke-RestMethod -Uri $uri -Headers @{ Authorization = "Bearer $token" } -Method Get -ErrorAction Stop
} catch {
  Write-Host "HTTP ERROR: $($_.Exception.Message)"
  if ($_.ErrorDetails.Message) { Write-Host $_.ErrorDetails.Message }
  exit 1
}

if ($resp.status -ne "success") {
  Write-Host "QUERY FAILED: $($resp | ConvertTo-Json -Depth 6)"
  exit 1
}

$results = $resp.data.result
if (-not $results -or $results.Count -eq 0) {
  Write-Host "NO DATA  <- query: $Query"
  exit 0
}

foreach ($r in $results) {
  $labels = ($r.metric.PSObject.Properties | ForEach-Object { "$($_.Name)=$($_.Value)" }) -join ", "
  $val = if ($r.value) { $r.value[1] } else { "" }
  Write-Host "  {$labels} => $val"
}
