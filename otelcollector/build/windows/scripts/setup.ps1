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
New-Item -Type Directory -Path /static/react-app -Force -ErrorAction SilentlyContinue
############################################################################################
$ProgressPreference = 'SilentlyContinue'
Write-Host ('Installing Metrics Extension');
try {
    Invoke-WebRequest -Uri "https://github.com/Azure/prometheus-collector/releases/download/v6.25.0-main-02-26-2026-12118731/MdmMetricsExtension.2.2026.227.1509.nupkg" -OutFile /installation/ME/mdmmetricsextension.2.2026.227.1509.zip
    Expand-Archive -Path /installation/ME/mdmmetricsextension.2.2026.227.1509.zip -Destination /installation/ME/
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
    $fluentBitUri = 'https://packages.fluentbit.io/windows/fluent-bit-3.0.7-win64.zip'
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
$vcRedistLocation = 'https://aka.ms/vs/17/release/vc_redist.x64.exe'
$vcInstallerLocation = "\installation\vc_redist.x64.exe"
$vcArgs = "/install /quiet /norestart /log \installation\vcredist.log"
Invoke-WebRequest -Uri $vcRedistLocation -OutFile $vcInstallerLocation -ErrorAction Stop
Write-Host 'Finished downloading Visual C++ Redistributable Package'
$vcInstaller = Start-Process $vcInstallerLocation -ArgumentList $vcArgs -NoNewWindow -PassThru -ErrorAction Stop
# Retain the process handle so Windows PowerShell can read ExitCode after exit.
$vcInstaller.Handle | Out-Null
$vcInstaller.WaitForExit()
Write-Host "Visual C++ installer exit code: $($vcInstaller.ExitCode)"
Get-Content -Path '\installation\vcredist*.log' -ErrorAction Stop
if ($vcInstaller.ExitCode -notin @(0, 3010)) {
    throw "Visual C++ Redistributable installation failed with exit code $($vcInstaller.ExitCode)"
}
Copy-Item -Path /Windows/System32/msvcp140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
Copy-Item -Path /Windows/System32/vccorlib140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
Copy-Item -Path /Windows/System32/vcruntime140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
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
    $genevamonitoringagentUri = 'https://github.com/Azure/prometheus-collector/releases/download/Prometheus-MA-Windows-47-7-1/genevamonitoringagent.47.7.1.zip'
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
