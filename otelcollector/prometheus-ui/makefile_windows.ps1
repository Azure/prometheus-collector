
Write-Output "========================= cleanup existing prometheusui ========================="
if (Test-Path "prometheusui.exe") {
    Remove-Item prometheusui.exe
}
Write-Output "========================= Building prom config validator ========================="
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o prometheusui.exe .
