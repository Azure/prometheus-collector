param([string]$Pipeline = "$PSScriptRoot/../azure-pipeline-build.yml")
$ErrorActionPreference = 'Stop'
$pipeline = Get-Content $Pipeline -Raw
$windows2019 = ($pipeline -split '      - job: Windows2019_Prometheus_Collector', 2)[1] -split '      - job: Windows2022_Prometheus_Collector', 2
$windows2022 = ($pipeline -split '      - job: Windows2022_Prometheus_Collector', 2)[1] -split '      - job: WindowsMultiArch_Prometheus_Collector', 2
$match = [regex]::Match($windows2019[0], '(?m)^        - powershell: \|\r?\n((?: {12}[^\r\n]*\r?\n)+)          displayName: "Build: wait for the tag job"')
if (-not $match.Success -or $match.Index -ge $windows2019[0].IndexOf("displayName: 'Publish signed binaries for the WS2022 image'") -or
    $windows2022[0].IndexOf('wait-for-artifact.ps1 Windows2019_Prometheus_Collector windows-signed') -lt 0 -or
    $windows2022[0].IndexOf('wait-for-artifact.ps1 Windows2019_Prometheus_Collector windows-signed') -ge $windows2022[0].IndexOf('docker push $(WINDOWS_FULL_IMAGE_NAME)')) {
    throw 'Windows tag gate or signed artifact ordering is missing'
}

function Invoke-RestMethod {
    param($Uri, $Headers)
    $script:calls++
    $state = if ($script:calls -eq 1) { 'inProgress' } else { 'completed' }
    return @{ records = @(@{ type = 'Job'; identifier = 'Build.Image_Tags_and_Ev2_Artifacts.__default'; state = $state; result = $script:result }) }
}
function Start-Sleep { param($Seconds) }
$gate = [scriptblock]::Create($match.Groups[1].Value)
foreach ($result in 'succeeded', 'succeededWithIssues', 'failed', 'canceled', 'skipped') {
    $script:calls = 0
    $script:result = $result
    $failed = $false
    try { & $gate } catch { $failed = $true }
    if ($script:calls -ne 2 -or $failed -ne ($result -notin 'succeeded', 'succeededWithIssues')) {
        throw "Unexpected Windows tag gate behavior for $result"
    }
}
Write-Host 'Windows tag gate passed'
