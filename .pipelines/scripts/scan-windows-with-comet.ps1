param(
  [Parameter(Mandatory = $true)]
  [string]$Image,
  [Parameter(Mandatory = $true)]
  [string]$ResultsDirectory,
  [switch]$ControlScan
)

$ErrorActionPreference = 'Stop'
$scanner = 'mcr.microsoft.com/tvm-containers/shavsa/comet:v1-windows-amd64'
New-Item -ItemType Directory -Force -Path $ResultsDirectory | Out-Null
$fullDiagnostics = $env:COMET_WINDOWS_DIAGNOSTICS -eq 'true'
$captureExitDiagnostics = $env:COMET_WINDOWS_EXIT_DIAGNOSTICS -eq 'true' -and !$ControlScan

if ($fullDiagnostics -and !$ControlScan) {
  $hostProbes = @(
    { Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber },
    { Get-WindowsFeature Hyper-V, Containers | Select-Object Name, InstallState },
    { Get-Service Winmgmt, vmcompute, vmms, docker | Select-Object Name, Status, StartType }
  )
  foreach ($probe in $hostProbes) {
    try { & $probe | Format-List | Out-String | Add-Content (Join-Path $ResultsDirectory 'host-probes.txt') }
    catch { "Probe $probe failed: $_" | Add-Content (Join-Path $ResultsDirectory 'host-probes.txt') }
  }
  foreach ($command in 'version', 'info') {
    $ErrorActionPreference = 'Continue'
    docker $command 2>&1 | Out-File (Join-Path $ResultsDirectory "docker-$command.txt")
    $code = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    "Docker $command exit: $code" | Add-Content (Join-Path $ResultsDirectory "docker-$command.txt")
  }
  if ($Image -notmatch '-(ltsc2019|ltsc2022)$') { throw 'Cannot determine matching Server Core tag for diagnostic probe.' }
  $baseImage = "mcr.microsoft.com/windows/servercore:$($Matches[1])"
  docker pull $baseImage
  if ($LASTEXITCODE -ne 0) { throw "Cannot pull diagnostic base image $baseImage." }
  $queryProbe = @'
$ErrorActionPreference = 'Stop'
whoami
foreach ($probe in @(
  { Get-Service Winmgmt | Format-List Name, Status, StartType },
  { Get-CimInstance Win32_OperatingSystem | Format-List Caption, Version, BuildNumber },
  { Get-CimInstance Win32_QuickFixEngineering | Format-List HotFixID, InstalledOn },
  { Get-Item C:\Windows\System32\wbem\WMIC.exe | Select-Object FullName, VersionInfo }
)) {
  try { & $probe } catch { Write-Output "Probe $probe failed: $_" }
}
$ErrorActionPreference = 'Continue'
& C:\Windows\System32\wbem\WMIC.exe qfe list /format:rawxml
Write-Output "WMIC exit: $LASTEXITCODE"
'@
  $encodedProbe = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($queryProbe))
  foreach ($probeImage in $Image, $baseImage) {
    $name = if ($probeImage -eq $Image) { 'target' } else { 'servercore' }
    docker image inspect $probeImage | Out-File (Join-Path $ResultsDirectory "$name-image.json")
    if ($LASTEXITCODE -ne 0) { throw "Cannot inspect diagnostic image $probeImage." }
    $ErrorActionPreference = 'Continue'
    docker run --rm --isolation=hyperv --user 'NT AUTHORITY\SYSTEM' --entrypoint powershell `
      $probeImage -NoProfile -EncodedCommand $encodedProbe 2>&1 |
      Out-File (Join-Path $ResultsDirectory "$name-wmi-probe.txt")
    $code = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    "Probe container exit: $code" | Add-Content (Join-Path $ResultsDirectory "$name-wmi-probe.txt")
  }
}

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
$consolePath = Join-Path $ResultsDirectory 'comet-console.txt'
$scanStartedAt = [DateTime]::UtcNow
$ErrorActionPreference = 'Continue'
& docker @dockerArgs 2>&1 | ForEach-Object {
  $line = $_.ToString().Replace($jwt, '[REDACTED]')
  Write-Host $line
  Add-Content -LiteralPath $consolePath -Value $line
}
$scanExitCode = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
Write-Host "CoMET Docker exit code: $scanExitCode"
@{
  image = $Image
  targetDigests = @($repoDigests)
  scannerDigests = @($scannerDigests)
  dockerExitCode = $scanExitCode
} | ConvertTo-Json | Set-Content -Path (Join-Path $ResultsDirectory 'scan-exit.json') -Encoding UTF8

if ($captureExitDiagnostics) {
  $eventsPath = Join-Path $ResultsDirectory 'docker-container-die.jsonl'
  $eventsSince = $scanStartedAt.ToString('o')
  $eventsUntil = [DateTime]::UtcNow.ToString('o')
  $ErrorActionPreference = 'Continue'
  $eventOutput = @(& docker events --since $eventsSince --until $eventsUntil --filter type=container --filter event=die --format '{{json .}}' 2>&1)
  $eventsExitCode = $LASTEXITCODE
  $ErrorActionPreference = 'Stop'
  $redactedEventOutput = @($eventOutput | ForEach-Object { $_.ToString().Replace($jwt, '[REDACTED]') })
  [System.IO.File]::WriteAllLines($eventsPath, [string[]]$redactedEventOutput)
  @{
    exitCode = $eventsExitCode
    since = $eventsSince
    until = $eventsUntil
  } | ConvertTo-Json | Set-Content -Path (Join-Path $ResultsDirectory 'docker-events-status.json') -Encoding UTF8
  if ($eventsExitCode -ne 0) {
    Write-Host "##vso[task.logissue type=warning]Could not capture Docker container exit events: docker events exit $eventsExitCode."
  }

  $engineEvents = [System.Collections.Generic.List[object]]::new()
  $engineErrors = [System.Collections.Generic.List[string]]::new()
  $logNames = [System.Collections.Generic.List[string]]::new()
  $logNames.Add('Application')
  $logNames.Add('System')
  try {
    Get-WinEvent -ListLog '*' -ErrorAction Stop |
      Where-Object { $_.LogName -match 'Docker|Container|HCS|Hyper-V' } |
      ForEach-Object { $logNames.Add($_.LogName) }
  } catch {
    $engineErrors.Add("Could not enumerate Docker-related Windows event logs: $_")
  }
  foreach ($logName in $logNames | Sort-Object -Unique) {
    try {
      Get-WinEvent -FilterHashtable @{ LogName = $logName; StartTime = $scanStartedAt.ToLocalTime() } -MaxEvents 500 -ErrorAction Stop |
        Where-Object {
          $_.ProviderName -match 'Docker|Container|HCS|Hyper-V|vmcompute' -or
            $_.Message -match 'docker_engine|pipe is being closed|container|HCS|vmcompute'
        } |
        ForEach-Object {
          $message = [regex]::Replace($_.Message, '(?s)"Environment"\s*:\s*\{[^{}]*\}', '"Environment":{"[REDACTED]":true}')
          $engineEvents.Add(@{
            logName = $logName
            timeCreated = $_.TimeCreated
            provider = $_.ProviderName
            id = $_.Id
            level = $_.LevelDisplayName
            message = $message
          })
        }
    } catch {
      $engineErrors.Add("Could not read Windows event log '$logName': $_")
    }
  }
  $engineLogJson = @{
    since = $scanStartedAt.ToString('o')
    events = @($engineEvents)
    errors = @($engineErrors)
  } | ConvertTo-Json -Depth 5
  [System.IO.File]::WriteAllText(
    (Join-Path $ResultsDirectory 'docker-engine-events.json'),
    $engineLogJson.Replace($jwt, '[REDACTED]'))
  if ($engineErrors.Count -gt 0) {
    Write-Host "##vso[task.logissue type=warning]Some Docker-related Windows event logs could not be collected; see docker-engine-events.json."
  }
}

$xmlPath = Join-Path $ResultsDirectory 'result.xml'
$jsonPath = Join-Path $ResultsDirectory 'result.json'
if ($fullDiagnostics -or $scanExitCode -ne 0 -or !(Test-Path $xmlPath) -or !(Test-Path $jsonPath)) {
  $copyDiagnostics = @'
$ErrorActionPreference = 'Stop'
foreach ($source in 'C:\internal', 'C:\qualys_binaries') {
  $files = @(Get-ChildItem -LiteralPath $source -Recurse -File)
  $files | Select-Object FullName, Length | ConvertTo-Json -Depth 3 | Set-Content ("C:\results\" + (Split-Path $source -Leaf) + "-files.json")
  foreach ($file in $files) {
    if (!($fullDiagnostics -and $source -eq 'C:\internal') -and $file.Extension -notin '.log', '.txt', '.json', '.xml', '.out', '.err') { continue }
    $destination = Join-Path 'C:\results\diagnostics' $file.FullName.Substring(3)
    New-Item -ItemType Directory -Force -Path (Split-Path $destination) | Out-Null
    Copy-Item -LiteralPath $file.FullName -Destination $destination
  }
}
'@
  $copyDiagnostics = '$fullDiagnostics = $' + $fullDiagnostics.ToString().ToLowerInvariant() + "`n" + $copyDiagnostics
  $encodedDiagnostics = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($copyDiagnostics))
  docker run --rm --isolation=hyperv --entrypoint powershell `
    -v 'scanutilitywin:C:\internal:ro' `
    -v 'qualys_binaries_volume:C:\qualys_binaries:ro' `
    -v "$($ResultsDirectory):C:\results" `
    $scanner -NoProfile -EncodedCommand $encodedDiagnostics
  $diagnosticsExitCode = $LASTEXITCODE
  if ($diagnosticsExitCode -ne 0) {
    Write-Host "##vso[task.logissue type=warning]Could not preserve Qualys diagnostics: Docker exit $diagnosticsExitCode."
  }
  $diagnosticsDirectory = Join-Path $ResultsDirectory 'diagnostics'
  if (Test-Path $diagnosticsDirectory) {
    Get-ChildItem -LiteralPath $diagnosticsDirectory -Recurse -File | ForEach-Object {
      if ($_.Extension -notin '.log', '.txt', '.json', '.xml', '.out', '.err' -and $_.Name -notlike 'console_log-*') { return }
      $content = [System.IO.File]::ReadAllText($_.FullName)
      [System.IO.File]::WriteAllText($_.FullName, $content.Replace($jwt, '[REDACTED]'))
    }
  }
}

if ($fullDiagnostics -and !$ControlScan) {
  $controlDirectory = Join-Path $ResultsDirectory 'self-scan'
  try {
    & $PSCommandPath -Image $scanner -ResultsDirectory $controlDirectory -ControlScan
    @{ reportValidation = 'passed' } | ConvertTo-Json | Set-Content (Join-Path $ResultsDirectory 'self-scan-status.json')
  } catch {
    Write-Host "##vso[task.logissue type=warning]CoMET self-scan failed report validation: $_"
    @{ reportValidation = 'failed'; error = "$_" } | ConvertTo-Json | Set-Content (Join-Path $ResultsDirectory 'self-scan-status.json')
  }
}

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
