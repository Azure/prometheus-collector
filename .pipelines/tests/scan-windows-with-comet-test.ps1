$ErrorActionPreference = 'Stop'
$directory = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $directory | Out-Null

function Invoke-RestMethod { return @{ oidcToken = 'test-oidc-secret' } }
function docker {
  $global:LASTEXITCODE = 0
  if ($args[0] -eq 'image') {
    if ($args[3] -eq '{{.Os}}/{{.Architecture}}') { return 'windows/amd64' }
    return '["example@sha256:0123456789"]'
  }
  if ($args -contains 'scan') {
    $global:LASTEXITCODE = 255
  } elseif ($args -contains '--entrypoint') {
    $path = Join-Path $directory 'diagnostics'
    New-Item -ItemType Directory -Path $path | Out-Null
    Set-Content -Path (Join-Path $path 'qualys.log') -Value 'failure test-oidc-secret'
  }
}

try {
  try {
    & (Join-Path $PSScriptRoot '..\scripts\scan-windows-with-comet.ps1') -Image 'example:windows' -ResultsDirectory $directory
    throw 'Expected missing-report failure.'
  } catch {
    if ($_.Exception.Message -notlike 'CoMET did not produce both*') { throw }
  }
  $exit = Get-Content -Raw (Join-Path $directory 'scan-exit.json') | ConvertFrom-Json
  if ($exit.dockerExitCode -ne 255) { throw 'Lost the original Docker exit code.' }
  $diagnostic = Get-Content -Raw (Join-Path $directory 'diagnostics\qualys.log')
  if ($diagnostic.Contains('test-oidc-secret') -or !$diagnostic.Contains('[REDACTED]')) {
    throw 'Diagnostic token redaction failed.'
  }
  $yaml = Get-Content -Raw (Join-Path $PSScriptRoot '..\azure-pipeline-build.yml')
  foreach ($year in 2019, 2022) {
    if (!$yaml.Contains("artifactName: 'comet-windows-$year-" + '$(System.JobAttempt)' + "'")) {
      throw "Windows $year artifact is not retry-safe."
    }
  }
  Write-Host 'Windows CoMET failure diagnostics checks passed.'
} finally {
  Remove-Item -LiteralPath $directory -Recurse -Force
}
