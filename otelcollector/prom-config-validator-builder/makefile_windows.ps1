Write-Output "========================= cleanup existing promconfigvalidator ========================="
if (Test-Path "promconfigvalidator.exe") {
    Remove-Item promconfigvalidator.exe
}
Write-Output "========================= Building prom config validator ========================="
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o promconfigvalidator.exe .