Write-Output "========================= cleanup existing promconfigvalidator ========================="
Remove-Item promconfigvalidator
Write-Output "========================= Building prom config validator ========================="
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o promconfigvalidator .
Move-Item promconfigvalidator promconfigvalidator.exe