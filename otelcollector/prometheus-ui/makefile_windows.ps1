$PROMETHEUS_VERSION = (Get-Content ../opentelemetry-collector-builder/PROMETHEUS_VERSION)
Write-Output "========================= cleanup existing prometheusui ========================="
if (Test-Path "prometheusui.exe") {
    Remove-Item prometheusui.exe
}
if (Test-Path "static") {
  Remove-Item -Recurse -Force static
}

Write-Output "========================= Building prometheusui ========================="
Invoke-WebRequest -Uri "https://github.com/prometheus/prometheus/releases/download/v$($PROMETHEUS_VERSION)/prometheus-web-ui-$($PROMETHEUS_VERSION).tar.gz" -OutFile "prometheus-web-ui-$($PROMETHEUS_VERSION).tar.gz"
tar -xvzf "prometheus-web-ui-$($PROMETHEUS_VERSION).tar.gz"
Remove-Item "prometheus-web-ui-$($PROMETHEUS_VERSION).tar.gz"

Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o prometheusui.exe .