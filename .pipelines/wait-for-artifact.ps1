# Waits for another job in this run to publish <Prefix>-<attempt> and sets <Variable> to the latest such artifact name.
param([string]$Job, [string]$Prefix, [string]$Variable)
$url = "$env:SYSTEM_COLLECTIONURI$env:SYSTEM_TEAMPROJECTID/_apis/build/builds/$env:BUILD_BUILDID"
$headers = @{ Authorization = "Bearer $env:SYSTEM_ACCESSTOKEN" }
while ($true) {
  # Read the job state first so an artifact published just before completion is still seen.
  $record = (Invoke-RestMethod "$url/timeline?api-version=7.1" -Headers $headers).records | Where-Object { $_.type -eq 'Job' -and $_.identifier -eq "Build.$Job.__default" }
  $names = @((Invoke-RestMethod "$url/artifacts?api-version=7.1" -Headers $headers).value.name) -match "^$Prefix-\d+$"
  $name = $names | Sort-Object { [int]($_ -split '-')[-1] } | Select-Object -Last 1
  if ($name) { break }
  if ($record.state -eq 'completed') { throw "$Job completed without publishing $Prefix" }
  Start-Sleep 5
}
Write-Host "##vso[task.setvariable variable=$Variable]$name"
