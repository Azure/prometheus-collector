param(
  [Parameter(Mandatory = $true)]
  [string]$Image,
  [Parameter(Mandatory = $true)]
  [string]$ResultsDirectory
)

$ErrorActionPreference = 'Stop'
$scanner = 'mcr.microsoft.com/tvm-containers/shavsa/comet:v1-windows-amd64'
New-Item -ItemType Directory -Force -Path $ResultsDirectory | Out-Null

$platform = docker image inspect --format '{{.Os}}/{{.Architecture}}' $Image
if ($LASTEXITCODE -ne 0 -or $platform.Trim() -ne 'windows/amd64') {
  throw "Expected a local Windows AMD64 target image; got '$platform'."
}
$repoDigests = docker image inspect --format '{{json .RepoDigests}}' $Image | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or @($repoDigests).Count -eq 0) {
  throw "Could not read the pushed target image digest for $Image."
}
Write-Host "CoMET target: $Image"
Write-Host "CoMET target digest: $(@($repoDigests)[0])"

docker pull $scanner
if ($LASTEXITCODE -ne 0) { throw "Failed to pull CoMET scanner image $scanner." }
$scannerDigests = docker image inspect --format '{{json .RepoDigests}}' $scanner | ConvertFrom-Json
if ($LASTEXITCODE -ne 0 -or @($scannerDigests).Count -eq 0) {
  throw "Could not read the pulled CoMET scanner digest for $scanner."
}
Write-Host "CoMET scanner: $scanner"
Write-Host "CoMET scanner digest: $(@($scannerDigests)[0])"
$headers = @{
  Authorization = "Bearer $env:SYSTEM_ACCESSTOKEN"
  'Content-Type' = 'application/json'
}
$oidcUrl = "$($env:SYSTEM_COLLECTIONURI)$($env:SYSTEM_TEAMPROJECTID)/_apis/distributedtask/hubs/build/plans/$($env:SYSTEM_PLANID)/jobs/$($env:SYSTEM_JOBID)/oidctoken?api-version=7.1-preview.1"
$response = Invoke-RestMethod -Uri $oidcUrl -Method Post -Body '{}' -Headers $headers -ContentType 'application/json'
$jwt = $response.oidcToken
if ([string]::IsNullOrWhiteSpace($jwt)) { throw 'ADO returned an empty CoMET OIDC token.' }
Write-Host "##vso[task.setsecret]$jwt"

$dockerArgs = @(
  'run', '--rm', '--isolation=hyperv',
  '-v', '//./pipe/docker_engine://./pipe/docker_engine',
  '-v', 'scanutilitywin:C:\internal',
  '-v', 'qualys_binaries_volume:C:\qualys_binaries',
  '-v', "$($ResultsDirectory):C:\results",
  $scanner, 'scan', '--scan-result-path', 'C:\results',
  '--oidc-token', $jwt, $Image
)
& docker @dockerArgs
$scanExitCode = $LASTEXITCODE

$xmlPath = Join-Path $ResultsDirectory 'result.xml'
$jsonPath = Join-Path $ResultsDirectory 'result.json'
if (!(Test-Path $xmlPath) -or !(Test-Path $jsonPath)) {
  throw "CoMET did not produce both result.xml and result.json in $ResultsDirectory."
}
[xml]$report = Get-Content -Raw $xmlPath
$partialNode = $report.SelectSingleNode('//*[@PARTIAL_REPORT_FLAG]')
if ($partialNode) {
  $partial = $partialNode.GetAttribute('PARTIAL_REPORT_FLAG')
} else {
  $partialNode = $report.SelectSingleNode('//*[local-name()="PARTIAL_REPORT_FLAG"]')
  if ($partialNode) { $partial = $partialNode.InnerText }
}
if ($partial -ne 'false') { throw "CoMET XML is missing or marked partial: $partial." }
Write-Host 'CoMET produced a non-partial XML report; review both reports in the published artifact.'
if ($scanExitCode -ne 0) {
  Write-Host "##vso[task.logissue type=warning]CoMET returned exit code $scanExitCode with reports present; inspect result.xml, result.json, and the scan log."
}
