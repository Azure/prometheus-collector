$ErrorActionPreference = 'Stop'
$root = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $root | Out-Null
$originalDiagnostics = $env:COMET_WINDOWS_DIAGNOSTICS
$scanImages = [System.Collections.Generic.List[string]]::new()
$probeImages = [System.Collections.Generic.List[string]]::new()

function Invoke-RestMethod { return @{ oidcToken = 'test-oidc-secret' } }
function Get-CimInstance { return @{ Caption = 'Test Windows' } }
function Get-WindowsFeature { return @{ Name = 'Hyper-V'; InstallState = 'Installed' } }
function Get-Service { return @{ Name = 'Winmgmt'; Status = 'Running' } }
function docker {
  $global:LASTEXITCODE = 0
  if ($args[0] -eq 'image') {
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
  Write-Host 'Windows CoMET failure diagnostics checks passed.'
} finally {
  $env:COMET_WINDOWS_DIAGNOSTICS = $originalDiagnostics
  Remove-Item -LiteralPath $root -Recurse -Force
}
