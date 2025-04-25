param (
    [string]$PROMETHEUS_VERSION = ""
)

Write-Output "========================= cleanup existing prometheusui ========================="
if (Test-Path "prometheusui.exe") {
    Remove-Item prometheusui.exe
}
Write-Output "========================= Building prometheusui ========================="
Write-Output "Using Prometheus version: $PROMETHEUS_VERSION"
Invoke-WebRequest -Uri "https://github.com/prometheus/prometheus/releases/download/v$PROMETHEUS_VERSION/prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" -OutFile "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"

# Verify the downloaded file exists
Write-Output "Verifying downloaded file..."
if (Test-Path "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz") {
    Get-Item "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" | Format-List Name, Length, LastWriteTime
} else {
    Write-Error "Failed to download prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"
    exit 1
}

tar -xvzf "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"

# List extracted files/directories to determine extraction path
Write-Output "Listing files in working directory after extraction:"
Get-ChildItem -Path . | Select-Object Name, LastWriteTime, @{Name="Type";Expression={if($_.PSIsContainer){"Directory"}else{"File"}}}

# Clean up the downloaded tar.gz file
Write-Output "Cleaning up downloaded archive..."
Remove-Item "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o prometheusui.exe .
