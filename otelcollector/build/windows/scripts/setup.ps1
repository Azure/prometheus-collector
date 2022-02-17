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
###########################################################################################
Write-Host ('Installing Metrics Extension');
try {
    Invoke-WebRequest -Uri https://github.com/microsoft/Docker-Provider/releases/download/ME-OTEL-WINDOWS-TEST/mdmmetricsextension.2.2021.714.2112.nupkg -OutFile /installation/ME/mdmmetricsextension.2.2021.714.2112.zip
    Expand-Archive -Path /installation/ME/mdmmetricsextension.2.2021.714.2112.zip -Destination /installation/ME/
    Move-Item /installation/ME/MetricsExtension /opt/metricextension/
}
catch {
    $e = $_.Exception
    Write-Host "exception when installing Metrics Extension"
    Write-Host $e
    exit 1
}
Write-Host ('Finished installing Metrics Extension')
###########################################################################################
Write-Host ('Installing Fluent Bit');
try {
    # Keep version in sync with linux in setup.sh file
    # $fluentBitUri = 'https://github.com/microsoft/OMS-docker/releases/download/winakslogagent/td-agent-bit-1.4.0-win64.zip'
    $fluentBitUri = 'https://github.com/fluent/fluent-bit/releases/download/v1.8.12/td-agent-bit-1.8.12-win64.zip'
    Invoke-WebRequest -Uri $fluentBitUri -OutFile /installation/td-agent-bit.zip
    Expand-Archive -Path /installation/td-agent-bit.zip -Destination /installation/fluent-bit
    Move-Item -Path /installation/fluent-bit/*/bin/* -Destination /opt/fluent-bit/bin/ -ErrorAction SilentlyContinue
}
catch {
    $e = $_.Exception
    Write-Host "exception when installing fluentbit"
    Write-Host $e
    exit 1
}
Write-Host ('Finished installing fluentbit')
###########################################################################################
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
###########################################################################################
Write-Host ('Installing Telegraf');
try {
    # Keep version in sync with linux in setup.sh file
    $telegrafUri = 'https://dl.influxdata.com/telegraf/releases/telegraf-1.18.0_windows_amd64.zip'
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
###########################################################################################
#Remove gemfile.lock for http_parser gem 0.6.0
#see  - https://github.com/fluent/fluentd/issues/3374 https://github.com/tmm1/http_parser.rb/issues/70
$gemfile = "\ruby26\lib\ruby\gems\2.6.0\gems\http_parser.rb-0.6.0\Gemfile.lock"
$gemfileFullPath = $Env:SYSTEMDRIVE + "\" + $gemfile
If (Test-Path -Path $gemfile ) {
    Write-Host ("Renaming unused gemfile.lock for http_parser 0.6.0")
    Rename-Item -Path $gemfileFullPath -NewName  "renamed_Gemfile_lock.renamed"
}
###########################################################################################
Write-Host ("Removing Install folder")
Remove-Item /installation -Recurse