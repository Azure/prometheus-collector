. (Join-Path $PSScriptRoot 'Test-MicrosoftSignature.ps1')

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
Write-Host ('Installing Metrics Extension');
try {
    $nugetPath = '\installation\nuget.exe'
    $nugetConfigPath = '\installation\NuGet.Config'
    Invoke-WebRequest -Uri 'https://dist.nuget.org/win-x86-commandline/v6.14.3/nuget.exe' -OutFile $nugetPath -ErrorAction Stop
    Set-Content -LiteralPath $nugetConfigPath -Value '<configuration />' -Encoding UTF8 -ErrorAction Stop
    $metricsExtensionPackage = '\installation\ME\mdmmetricsextension.2.2026.227.1509.nupkg'
    Invoke-WebRequest -Uri "https://github.com/Azure/prometheus-collector/releases/download/v6.25.0-main-02-26-2026-12118731/MdmMetricsExtension.2.2026.227.1509.nupkg" -OutFile $metricsExtensionPackage -ErrorAction Stop
    Test-MicrosoftNuGetSignature -LiteralPath $metricsExtensionPackage -NuGetPath $nugetPath -ConfigPath $nugetConfigPath
    Move-Item -LiteralPath $metricsExtensionPackage -Destination '\installation\ME\mdmmetricsextension.2.2026.227.1509.zip' -ErrorAction Stop
    Expand-Archive -Path /installation/ME/mdmmetricsextension.2.2026.227.1509.zip -Destination /installation/ME/ -ErrorAction Stop
    Move-Item /installation/ME/MetricsExtension /opt/metricextension/ -ErrorAction Stop
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
$vcRedistLocation = 'https://aka.ms/vs/16/release/vc_redist.x64.exe'
$vcInstallerLocation = "\installation\vc_redist.x64.exe"
$vcArgs = "/install /quiet /norestart"
$ProgressPreference = 'SilentlyContinue'
try {
    Invoke-WebRequest -Uri $vcRedistLocation -OutFile $vcInstallerLocation -ErrorAction Stop
    Test-MicrosoftCodeSignature -LiteralPath $vcInstallerLocation
    $vcProcess = Start-Process $vcInstallerLocation -ArgumentList $vcArgs -NoNewWindow -Wait -PassThru -ErrorAction Stop
    if ($vcProcess.ExitCode -notin @(0, 3010)) {
        throw "Visual C++ installation failed with exit code $($vcProcess.ExitCode)"
    }
    Copy-Item -Path /Windows/System32/msvcp140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
    Copy-Item -Path /Windows/System32/vccorlib140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
    Copy-Item -Path /Windows/System32/vcruntime140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
}
catch {
    Write-Host "exception while installing Visual C++ Redistributable"
    Write-Host $_.Exception
    exit 1
}
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
    $genevamonitoringagentUri = 'https://github.com/Azure/prometheus-collector/releases/download/Prometheus-MA-Windows-47-14-2/genevamonitoringagent.47.14.2.zip'
    $genevamonitoringagentSha256 = '8f1b572c5caad58b9beed8c6072b690c0988c05d1daefd52f7afe50702f99e2a'
    Invoke-WebRequest -Uri $genevamonitoringagentUri -OutFile /installation/genevamonitoringagent.zip -ErrorAction Stop
    if ((Get-FileHash -Path /installation/genevamonitoringagent.zip -Algorithm SHA256 -ErrorAction Stop).Hash -ne $genevamonitoringagentSha256) {
        throw 'GenevaMonitoringAgent archive SHA256 mismatch'
    }
    Expand-Archive -Path /installation/genevamonitoringagent.zip -Destination /installation/genevamonitoringagent -ErrorAction Stop
    # This archive is not NuGet-signed; these checks cover the Microsoft executables we launch, not the entire archive.
    foreach ($relativePath in @('MonAgentLauncher.exe', 'MonAgentCore.exe', 'Extensions\AMACoreAgent\AgentLauncher.exe', 'Extensions\AMACoreAgent\AMACoreAgent.exe')) {
        Test-MicrosoftCodeSignature -LiteralPath (Join-Path '\installation\genevamonitoringagent\Monitoring\Agent' $relativePath)
    }
    Move-Item -Path /installation/genevamonitoringagent -Destination /opt/genevamonitoringagent/ -ErrorAction Stop
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
