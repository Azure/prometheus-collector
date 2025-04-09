Write-Host ('Creating folder structure')
New-Item -Type Directory -Path /installation/ME/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /installation/fluent-bit/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/metricextension/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/fluent-bit/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/telegraf/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/otelcollector/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/certificate/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/state/ -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/ruby -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/microsoft/liveness -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/genevamonitoringagent -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/genevamonitoringagent/datadirectory -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /etc/genevamonitoringagent
New-Item -Type Directory -Path /static/react -Force -ErrorAction SilentlyContinue
############################################################################################
Write-Host ('Installing Metrics Extension');
try {
    Invoke-WebRequest -Uri "https://github.com/Azure/prometheus-collector/releases/download/v6.14.0-main-01-23-2025-677596d1/MdmMetricsExtension.2.2025.123.2222.nupkg" -OutFile /installation/ME/mdmmetricsextension.2.2025.123.2222.zip
    Expand-Archive -Path /installation/ME/mdmmetricsextension.2.2025.123.2222.zip -Destination /installation/ME/
    Move-Item /installation/ME/MetricsExtension /opt/metricextension/
}
catch {
    $e = $_.Exception
    Write-Host "exception when installing Metrics Extension"
    Write-Host $e
    exit 1
}
Write-Host ('Finished installing Metrics Extension')
############################################################################################
Write-Host ('Installing Fluent Bit');
try {
    # Keep version in sync with linux in setup.sh file
    # $fluentBitUri = 'https://github.com/microsoft/OMS-docker/releases/download/winakslogagent/td-agent-bit-1.4.0-win64.zip'
    $fluentBitUri = 'https://releases.fluentbit.io/3.0/fluent-bit-3.0.7-win64.zip'
    Invoke-WebRequest -Uri $fluentBitUri -OutFile /installation/fluent-bit.zip
    Expand-Archive -Path /installation/fluent-bit.zip -Destination /installation/fluent-bit
    Move-Item -Path /installation/fluent-bit/*/bin/* -Destination /opt/fluent-bit/bin/ -ErrorAction SilentlyContinue
}
catch {
    $e = $_.Exception
    Write-Host "exception when installing fluentbit"
    Write-Host $e
    exit 1
}
Write-Host ('Finished installing fluentbit')
############################################################################################
Write-Host ('Installing Visual C++ Redistributable Package')
$vcRedistLocation = 'https://aka.ms/vs/16/release/vc_redist.x64.exe'
$vcInstallerLocation = "\installation\vc_redist.x64.exe"
$vcArgs = "/install /quiet /norestart"
$ProgressPreference = 'SilentlyContinue'
Invoke-WebRequest -Uri $vcRedistLocation -OutFile $vcInstallerLocation
Start-Process $vcInstallerLocation -ArgumentList $vcArgs -NoNewWindow -Wait
Copy-Item -Path /Windows/System32/msvcp140.dll -Destination /opt/fluent-bit/bin
Copy-Item -Path /Windows/System32/vccorlib140.dll -Destination /opt/fluent-bit/bin
Copy-Item -Path /Windows/System32/vcruntime140.dll -Destination /opt/fluent-bit/bin
Write-Host ('Finished Installing Visual C++ Redistributable Package')
############################################################################################
Write-Host ('Installing Telegraf');
try {
    # Keep version in sync with linux in setup.sh file
    $telegrafUri = 'https://dl.influxdata.com/telegraf/releases/telegraf-1.24.2_windows_amd64.zip'
    Invoke-WebRequest -Uri $telegrafUri -OutFile /installation/telegraf.zip
    Expand-Archive -Path /installation/telegraf.zip -Destination /installation/telegraf
    Move-Item -Path /installation/telegraf/*/* -Destination /opt/telegraf/ -ErrorAction SilentlyContinue
}
catch {
    $ex = $_.Exception
    Write-Host "exception while downloading telegraf for windows"
    Write-Host $ex
    exit 1
}
Write-Host ('Finished downloading Telegraf')
############################################################################################
Write-Host ('Installing GenevaMonitoringAgent');
try {
    $genevamonitoringagentUri = 'https://github.com/Azure/prometheus-collector/releases/download/Promtheus-MA-Windows-4.1.2024/GenevaMonitoringAgent.46.15.4.zip'
    Invoke-WebRequest -Uri $genevamonitoringagentUri -OutFile /installation/genevamonitoringagent.zip
    Expand-Archive -Path /installation/genevamonitoringagent.zip -Destination /installation/genevamonitoringagent
    Move-Item -Path /installation/genevamonitoringagent -Destination /opt/genevamonitoringagent/ -ErrorAction SilentlyContinue
}
catch {
    $ex = $_.Exception
    Write-Host "exception while downloading genevamonitoringagent for windows"
    Write-Host $ex
    exit 1
}
Write-Host ('Finished downloading GenevaMonitoringAgent')
############################################################################################
Write-Host ("Removing Install folder")
Remove-Item /installation -Recurse
