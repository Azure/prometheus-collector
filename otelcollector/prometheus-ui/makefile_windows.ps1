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
# Extract tar.gz using 7-Zip (make sure 7-Zip is installed)
& 7z x "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" -so | & 7z x -aoa -si -ttar
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o prometheusui.exe .
