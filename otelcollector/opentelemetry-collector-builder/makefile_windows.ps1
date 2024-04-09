# Temporary method to build the OTL collector and fluent-bit for windows

# building otelcollector
Write-Output "building otelcollector"
if (Test-Path "otelcollector.exe") {
    Remove-Item .\otelcollector.exe
}
go get
go build -o otelcollector.exe .

Write-Output "FINISHED building otelcollector"

# building fluent-bit plugin

Write-Output "building fluent-bit plugin"

Set-Location ..
Set-Location fluent-bit
Set-Location src

.\makefile_windows.ps1

Set-Location ..
Set-Location ..
Set-Location opentelemetry-collector-builder

Write-Output "FINISHED building fluent-bit plugin"

Write-Output "building promconfigvalidator"

Set-Location ..
Set-Location prom-config-validator-builder

.\makefile_windows.ps1

Set-Location ..
Set-Location prometheus-ui

.\makefile_windows.ps1

Set-Location ..
Set-Location opentelemetry-collector-builder

Write-Output "FINISHED building promconfigvalidator"