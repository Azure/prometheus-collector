$ErrorActionPreference = 'Stop'
$root = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $root | Out-Null
$originalDiagnostics = $env:COMET_WINDOWS_DIAGNOSTICS
$originalExitDiagnostics = $env:COMET_WINDOWS_EXIT_DIAGNOSTICS
$scanImages = [System.Collections.Generic.List[string]]::new()
$probeImages = [System.Collections.Generic.List[string]]::new()
$global:eventCalls = 0

function Invoke-RestMethod { return @{ oidcToken = 'test-oidc-secret' } }
function Get-CimInstance { return @{ Caption = 'Test Windows' } }
function Get-WindowsFeature { return @{ Name = 'Hyper-V'; InstallState = 'Installed' } }
function Get-Service { return @{ Name = 'Winmgmt'; Status = 'Running' } }
function Get-WinEvent {
  [CmdletBinding()]
  param([string]$ListLog, [hashtable]$FilterHashtable, [int]$MaxEvents)
  if ($ListLog) { return @() }
  return [pscustomobject]@{
    TimeCreated = [DateTime]::UtcNow
    ProviderName = 'Docker Engine'
    Id = 1
    LevelDisplayName = 'Information'
    Message = 'Create process {"CommandLine":"scan","Environment":{"APPLICATIONINSIGHTS_AUTH_PUBLIC":"test-appinsights-secret"}}; The pipe is being closed.'
  }
}
function docker {
  $global:LASTEXITCODE = 0
  if ($args[0] -eq 'events') {
    if ($args -notcontains 'type=container' -or $args -notcontains 'event=die') {
      throw 'Docker events query does not capture container exit events.'
    }
    $global:eventCalls++
    Write-Output '{"Type":"container","Action":"die","Actor":{"Attributes":{"image":"example:windows-ltsc2022","name":"qscan","exitCode":"1"}}}'
  } elseif ($args[0] -eq 'image') {
    if ($args[3] -eq '{{.Os}}/{{.Architecture}}') { return 'windows/amd64' }
    return '["example@sha256:0123456789"]'
  }
  if ($args -contains 'scan') {
    $scanImages.Add($args[-1])
    Write-Output 'scanner output test-oidc-secret'
    $global:LASTEXITCODE = $scanCode
  } elseif ($args -contains '--entrypoint') {
    $encoded = $args[([array]::IndexOf($args, '-EncodedCommand') + 1)]
    $command = [Text.Encoding]::Unicode.GetString([Convert]::FromBase64String($encoded))
    if ($command.Contains('Win32_QuickFixEngineering')) {
      if ($args -notcontains '--isolation=hyperv' -or $args -notcontains 'NT AUTHORITY\SYSTEM') {
        throw 'WMI probe does not match scanner isolation/user.'
      }
      $probeImages.Add($args[([array]::IndexOf($args, '-NoProfile') - 1)])
      Write-Output '<probe>hotfix XML</probe>'
      return
    }
    $mount = $args | Where-Object { $_ -like '*:C:\results' }
    $path = Join-Path $mount.Substring(0, $mount.Length - ':C:\results'.Length) 'diagnostics'
    New-Item -ItemType Directory -Path $path | Out-Null
    Set-Content -Path (Join-Path $path 'qualys.log') -Value 'failure test-oidc-secret'
    if ($env:COMET_WINDOWS_DIAGNOSTICS -eq 'true') {
      if (!$command.StartsWith('$fullDiagnostics = $true') -or !$command.Contains('$fullDiagnostics -and $source')) {
        throw 'Full internal evidence capture is not enabled.'
      }
      [IO.File]::WriteAllBytes((Join-Path $path 'changelist.db'), [byte[]](0, 255, 128, 1))
      Set-Content -Path (Join-Path $path 'console_log-test') -Value 'internal test-oidc-secret'
    }
  }
}

try {
  foreach ($scanCode in 255, 0, 1) {
    $env:COMET_WINDOWS_DIAGNOSTICS = if ($scanCode -eq 1) { 'true' } else { 'false' }
    $env:COMET_WINDOWS_EXIT_DIAGNOSTICS = if ($scanCode -eq 1) { 'true' } else { 'false' }
    $directory = Join-Path $root "$scanCode"
    try {
      & (Join-Path $PSScriptRoot '..\scripts\scan-windows-with-comet.ps1') -Image 'example:windows-ltsc2022' -ResultsDirectory $directory
      throw 'Expected missing-report failure.'
    } catch {
      if ($_.Exception.Message -notlike 'CoMET did not produce both*') { throw }
    }
    $exit = Get-Content -Raw (Join-Path $directory 'scan-exit.json') | ConvertFrom-Json
    if ($exit.dockerExitCode -ne $scanCode) { throw 'Lost the original Docker exit code.' }
    $diagnostic = Get-Content -Raw (Join-Path $directory 'diagnostics\qualys.log')
    if ($diagnostic.Contains('test-oidc-secret') -or !$diagnostic.Contains('[REDACTED]')) {
      throw 'Diagnostic token redaction failed.'
    }
    $console = Get-Content -Raw (Join-Path $directory 'comet-console.txt')
    if ($console.Contains('test-oidc-secret') -or !$console.Contains('[REDACTED]')) {
      throw 'Console token redaction failed.'
    }
    if ($scanCode -eq 1) {
      $events = Get-Content -Raw (Join-Path $directory 'docker-container-die.jsonl')
      if (!$events.Contains('"exitCode":"1"') -or $events.Contains('test-oidc-secret')) {
        throw 'Docker container exit event was not captured or redacted.'
      }
      $eventStatus = Get-Content -Raw (Join-Path $directory 'docker-events-status.json') | ConvertFrom-Json
      if ($eventStatus.exitCode -ne 0) { throw 'Docker event query failed unexpectedly.' }
      $engineEvents = Get-Content -Raw (Join-Path $directory 'docker-engine-events.json')
      if (!$engineEvents.Contains('The pipe is being closed.') -or
        $engineEvents.Contains('test-oidc-secret') -or
        $engineEvents.Contains('test-appinsights-secret') -or
        $engineEvents.Contains('APPLICATIONINSIGHTS_AUTH_PUBLIC') -or
        !$engineEvents.Contains('[REDACTED]')) {
        throw 'Docker-related Windows event evidence was not captured or secrets were not redacted.'
      }
      if ($global:eventCalls -ne 1) { throw "Exit diagnostics ran $global:eventCalls times instead of once for the target scan." }
    } elseif (Test-Path (Join-Path $directory 'docker-container-die.jsonl')) {
      throw 'Exit diagnostics ran without opt-in.'
    }
    if ($scanCode -eq 1) {
      $db = [IO.File]::ReadAllBytes((Join-Path $directory 'diagnostics\changelist.db'))
      if (($db -join ',') -ne '0,255,128,1') { throw 'Binary scan database was corrupted.' }
      $internal = Get-Content -Raw (Join-Path $directory 'diagnostics\console_log-test')
      if ($internal.Contains('test-oidc-secret')) { throw 'Internal console token was not redacted.' }
      if ($probeImages.Count -ne 2 -or $probeImages[1] -ne 'mcr.microsoft.com/windows/servercore:ltsc2022') {
        throw 'Target and matching untouched Server Core were not probed.'
      }
      if ($scanImages.Count -ne 4 -or $scanImages[3] -ne 'mcr.microsoft.com/tvm-containers/shavsa/comet:v1-windows-amd64') {
        throw 'Self-scan was skipped or recursively repeated.'
      }
      $status = Get-Content -Raw (Join-Path $directory 'self-scan-status.json') | ConvertFrom-Json
      if ($status.reportValidation -ne 'failed') { throw 'Missing control reports incorrectly passed validation.' }
    } elseif ($probeImages.Count -ne 0) { throw 'Diagnostics ran without opt-in.' }
  }
  $yaml = Get-Content -Raw (Join-Path $PSScriptRoot '..\azure-pipeline-build.yml')
  foreach ($year in 2019, 2022) {
    if (!$yaml.Contains("artifactName: 'comet-windows-$year-" + '$(System.JobAttempt)' + "'")) {
      throw "Windows $year artifact is not retry-safe."
    }
  }
  if ($yaml -notmatch '(?ms)- name: runWindowsCometExitDiagnostics\s+displayName:.*?\s+type: boolean\s+default: false') {
    throw 'The Windows CoMET exit diagnostics parameter is not default-off.'
  }
  $skipWindows2019 = 'ne(''${{ parameters.runWindowsCometExitDiagnostics }}'', ''true'')'
  if ([regex]::Matches($yaml, [regex]::Escape($skipWindows2019)).Count -ne 2) {
    throw 'The Windows 2019 scan and report are not skipped for a WS2022-only exit diagnostic.'
  }
  if (!$yaml.Contains('COMET_WINDOWS_EXIT_DIAGNOSTICS: ${{ parameters.runWindowsCometExitDiagnostics }}')) {
    throw 'The Windows 2022 job does not receive the exit diagnostics opt-in.'
  }
  Write-Host 'Windows CoMET failure diagnostics checks passed.'
} finally {
  $env:COMET_WINDOWS_DIAGNOSTICS = $originalDiagnostics
  $env:COMET_WINDOWS_EXIT_DIAGNOSTICS = $originalExitDiagnostics
  Remove-Item -LiteralPath $root -Recurse -Force
}
