# Temporary method to build the OTL collector and fluent-bit for windows
param(
    [Parameter()]
    [string]$PROMETHEUS_VERSION = ""  # Default value if not provided
)

$ErrorActionPreference = "Stop"
$otel = Split-Path -Parent $PSScriptRoot

# The Go modules are independent, so build them concurrently. Each runs in its own Windows PowerShell because jobs
# drop native stderr (the go errors), and via -File because -EncodedCommand writes errors to stderr as CLIXML.
function Start-Build([string]$Name, [string]$Dir, [string]$Command) {
    Set-Content "$env:TEMP\$Name.ps1" "`$ErrorActionPreference = 'Stop'`n$Command`nexit `$LASTEXITCODE"
    $process = Start-Process powershell.exe -WorkingDirectory "$otel\$Dir" -NoNewWindow -PassThru `
        -ArgumentList "-NoProfile -NonInteractive -ExecutionPolicy Bypass -File `"$env:TEMP\$Name.ps1`"" `
        -RedirectStandardOutput "$env:TEMP\$Name.out" -RedirectStandardError "$env:TEMP\$Name.err"
    $null = $process.Handle # ExitCode stays empty unless the handle is opened before the process exits
    @{ Name = $Name; Process = $process }
}

$mainBuild = @'
New-Item -Path ./shared/configmap/mp/, ./shared/configmap/ccp/ -ItemType Directory -Force | Out-Null
foreach ($d in '', 'configmap/mp/', 'configmap/ccp/') {
    Copy-Item -Path "../shared/$d*.go", "../shared/${d}go.mod", "../shared/${d}go.sum" -Destination "./shared/$d"
}
go version
go mod tidy
go build -buildmode=pie -o main.exe ./main.go
'@

$builds = @(
    Start-Build otelcollector opentelemetry-collector-builder 'Remove-Item otelcollector.exe -ErrorAction Ignore; go get; go build -o otelcollector.exe .'
    Start-Build fluent-bit-plugin fluent-bit\src '.\makefile_windows.ps1'
    Start-Build promconfigvalidator prom-config-validator-builder '.\makefile_windows.ps1'
    Start-Build main main $mainBuild
    Start-Build prometheusui prometheus-ui ".\makefile_windows.ps1 -PROMETHEUS_VERSION '$PROMETHEUS_VERSION'"
)

$failed = @()
foreach ($build in $builds) {
    $process = $build.Process
    $process.WaitForExit()
    Write-Output "========================= $($build.Name) exited $($process.ExitCode) after $([int]($process.ExitTime - $process.StartTime).TotalSeconds)s ========================="
    Get-Content "$env:TEMP\$($build.Name).out", "$env:TEMP\$($build.Name).err"
    if ($process.ExitCode -ne 0) { $failed += $build.Name }
}
if ($failed) { throw "Windows builds failed: $failed" }

