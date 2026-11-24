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
New-Item -Type Directory -Path /opt/microsoft/configmapparser -ErrorAction SilentlyContinue
New-Item -Type Directory -Path /opt/microsoft/scripts -ErrorAction SilentlyContinue
############################################################################################
$ProgressPreference = 'SilentlyContinue'
# tar.exe extracts these zips byte-identically to Expand-Archive, about 8x faster.
function Expand-Zip($Path, $Destination) {
    New-Item -Type Directory -Path $Destination -Force | Out-Null
    tar.exe -xf $Path -C $Destination
    if ($LASTEXITCODE -ne 0) { throw "Failed to extract $Path" }
}
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
    Expand-Zip /installation/ME/mdmmetricsextension.2.2026.227.1509.zip /installation/ME/
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
    Expand-Zip /installation/fluent-bit.zip /installation/fluent-bit
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
# The bundle's Burn engine relaunches itself via ShellExecute "runas", which stalls 30m in WS2019 Hyper-V builds.
# Install its x64 MSIs directly instead, after the same payload hash checks and with the same MSI properties Burn uses.
function Get-VcRedistMsiPackages($Bundle, $Destination) {
    New-Item -Type Directory -Path $Destination -Force | Out-Null
    $bytes = [IO.File]::ReadAllBytes($Bundle)
    $pe = [BitConverter]::ToInt32($bytes, 0x3C)
    $sections = $pe + 24 + [BitConverter]::ToUInt16($bytes, $pe + 20)
    $burn = 0..([BitConverter]::ToUInt16($bytes, $pe + 6) - 1) | ForEach-Object { $sections + 40 * $_ } |
        Where-Object { [Text.Encoding]::ASCII.GetString($bytes, $_, 8).TrimEnd([char]0) -eq '.wixburn' } |
        ForEach-Object { [BitConverter]::ToInt32($bytes, $_ + 20) }
    if ($null -eq $burn -or [BitConverter]::ToUInt32($bytes, $burn) -ne 0x00f14300) { throw 'Visual C++ installer is not a WiX Burn bundle' }
    # Burn v3 layout: UX container after the engine stub, attached container after the engine's original signature.
    $containers = @{
        ux       = [BitConverter]::ToUInt32($bytes, $burn + 24), [BitConverter]::ToUInt32($bytes, $burn + 48)
        attached = ([BitConverter]::ToUInt32($bytes, $burn + 32) + [BitConverter]::ToUInt32($bytes, $burn + 36)), [BitConverter]::ToUInt32($bytes, $burn + 52)
    }
    foreach ($name in $containers.Keys) {
        $offset, $size = $containers[$name]
        if ([Text.Encoding]::ASCII.GetString($bytes, $offset, 4) -ne 'MSCF' -or [BitConverter]::ToUInt32($bytes, $offset + 8) -ne $size) {
            throw "Unexpected Visual C++ $name container"
        }
        $cab = New-Object byte[] $size
        [Array]::Copy($bytes, $offset, $cab, 0, $size)
        [IO.File]::WriteAllBytes("$Destination\$name.cab", $cab)
        New-Item -Type Directory -Path "$Destination\$name" -Force | Out-Null
        expand.exe -F:* "$Destination\$name.cab" "$Destination\$name" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "Failed to expand Visual C++ $name container" }
    }
    $manifest = [xml](Get-Content "$Destination\ux\0" -Raw)
    foreach ($id in 'vcRuntimeMinimum_x64', 'vcRuntimeAdditional_x64') {
        $package = $manifest.BurnManifest.Chain.MsiPackage | Where-Object Id -eq $id
        if (-not $package) { throw "Visual C++ package $id not found" }
        foreach ($payload in $manifest.BurnManifest.Payload | Where-Object Id -in $package.PayloadRef.Id) {
            $file = "$Destination\attached\$($payload.SourcePath)"
            if ((Get-FileHash $file -Algorithm SHA1).Hash -ne $payload.Hash) { throw "Hash mismatch for $($payload.FilePath)" }
            New-Item -Type Directory -Path (Split-Path "$Destination\$($payload.FilePath)") -Force | Out-Null
            Move-Item $file "$Destination\$($payload.FilePath)"
        }
        [pscustomobject]@{
            Id         = $id
            Path       = "$Destination\$(($manifest.BurnManifest.Payload | Where-Object Id -eq $id).FilePath)"
            Properties = ($package.MsiProperty | ForEach-Object { "$($_.Id)=$($_.Value)" }) -join ' '
        }
    }
}
$vcRedistLocation = 'https://aka.ms/vc14/vc_redist.x64.exe'
$vcInstallerLocation = "\installation\vc_redist.x64.exe"
Invoke-WebRequest -Uri $vcRedistLocation -OutFile $vcInstallerLocation -ErrorAction Stop
Test-MicrosoftCodeSignature -LiteralPath $vcInstallerLocation
$installerVersion = (Get-Item -LiteralPath $vcInstallerLocation -ErrorAction Stop).VersionInfo
if ([version]::new($installerVersion.FileMajorPart, $installerVersion.FileMinorPart, $installerVersion.FileBuildPart, $installerVersion.FilePrivatePart) -lt [version]'14.40.33816.0') {
    throw "Visual C++ installer version $($installerVersion.FileVersion) is below 14.40.33816.0"
}
Write-Host 'Finished downloading Visual C++ Redistributable Package'
foreach ($vcPackage in (Get-VcRedistMsiPackages $vcInstallerLocation '\installation\vcredist')) {
    $vcArgs = "/i `"$($vcPackage.Path)`" /qn /norestart $($vcPackage.Properties) /l*v \installation\vcredist_$($vcPackage.Id).log"
    $vcInstaller = Start-Process msiexec.exe -ArgumentList $vcArgs -NoNewWindow -PassThru -ErrorAction Stop
    # Retain the process handle so Windows PowerShell can read ExitCode after exit.
    $vcInstaller.Handle | Out-Null
    $vcInstaller.WaitForExit()
    Write-Host "Visual C++ $($vcPackage.Id) installer exit code: $($vcInstaller.ExitCode)"
    Get-Content -Path "\installation\vcredist_$($vcPackage.Id).log" -ErrorAction Stop
    if ($vcInstaller.ExitCode -notin @(0, 3010)) {
        throw "Visual C++ Redistributable installation failed with exit code $($vcInstaller.ExitCode)"
    }
}
$runtimeVersion = (Get-ItemProperty -LiteralPath 'HKLM:\SOFTWARE\Microsoft\VisualStudio\14.0\VC\Runtimes\X64' -ErrorAction Stop).Version
if ([version]$runtimeVersion.TrimStart('v') -lt [version]'14.40.33816.0') {
    throw "Installed Visual C++ runtime version $runtimeVersion is below 14.40.33816.0"
}
Copy-Item -Path /Windows/System32/msvcp140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
Copy-Item -Path /Windows/System32/vccorlib140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
Copy-Item -Path /Windows/System32/vcruntime140.dll -Destination /opt/fluent-bit/bin -ErrorAction Stop
Write-Host ('Finished Installing Visual C++ Redistributable Package')
############################################################################################
Write-Host ('Installing Telegraf');
try {
    $telegrafUri = 'https://dl.influxdata.com/telegraf/releases/telegraf-1.40.0_windows_amd64.zip'
    Invoke-WebRequest -Uri $telegrafUri -OutFile /installation/telegraf.zip
    Expand-Zip /installation/telegraf.zip /installation/telegraf
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
    Invoke-WebRequest -Uri $genevamonitoringagentUri -OutFile /installation/genevamonitoringagent.zip -ErrorAction Stop
    Expand-Zip /installation/genevamonitoringagent.zip /installation/genevamonitoringagent
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
