# Commits <Image> from <Image>-unsigned with the signed binaries staged under <Staging>.
# The unsigned images hold empty placeholders for the host-built binaries, so any failed copy must fail the build.
param([string]$Image, [string]$Staging)
function Invoke-Docker { docker @args; if ($LASTEXITCODE) { throw "docker $args failed" } }
Invoke-Docker create --name pushContainer "$Image-unsigned"
Invoke-Docker cp "$Staging/fpSigning/otelcollector.exe" 'pushContainer:C:\opt\microsoft\otelcollector\otelcollector.exe'
Invoke-Docker cp "$Staging/fpSigning/prometheusui.exe" 'pushContainer:C:\opt\microsoft\otelcollector\prometheusui.exe'
Invoke-Docker cp "$Staging/fpSigning/main.exe" 'pushContainer:C:\opt\microsoft\main.exe'
Invoke-Docker cp "$Staging/ossSigning/fluent-bit/." 'pushContainer:C:\opt\fluent-bit/'
Invoke-Docker cp "$Staging/ossGoSigning/promconfigvalidator.exe" 'pushContainer:C:\opt\promconfigvalidator.exe'
Invoke-Docker cp "$Staging/ossGoSigning/out_appinsights.so" 'pushContainer:C:\opt\fluent-bit\bin\out_appinsights.so'
Invoke-Docker commit pushContainer $Image
Invoke-Docker rm pushContainer
