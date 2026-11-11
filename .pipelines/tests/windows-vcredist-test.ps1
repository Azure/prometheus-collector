$ErrorActionPreference = 'Stop'
$setupPath = Join-Path $PSScriptRoot '..\..\otelcollector\build\windows\scripts\setup.ps1'
$setup = Get-Content $setupPath -Raw
$start = [regex]::Match($setup, "(?s)Write-Host \('Installing Visual C\+\+ Redistributable Package'\).*?\`$vcInstaller.Handle \| Out-Null")
$finish = [regex]::Match($setup, "(?s)\`$vcInstaller.WaitForExit\(\).*?Write-Host \('Finished Installing Visual C\+\+ Redistributable Package'\)")
if (-not $start.Success -or -not $finish.Success) { throw 'VC++ setup blocks not found' }
foreach ($marker in @('Finished installing Metrics Extension', 'Finished installing fluentbit', 'Finished downloading Telegraf', 'Finished downloading GenevaMonitoringAgent')) {
    $position = $setup.IndexOf($marker)
    if ($position -lt $start.Index + $start.Length -or $position -gt $finish.Index) {
        throw "Component setup does not overlap installer: $marker"
    }
}
$install = [scriptblock]::Create($start.Value + "`n" + $finish.Value)

$processStart = [regex]::Match($start.Value, '(?s)\$vcInstaller = .*')
$processFinish = [regex]::Match($finish.Value, '(?s).*?(?=Copy-Item)')
if (-not $processStart.Success -or -not $processFinish.Success) { throw 'Installer process wait not found' }
$waitForInstaller = [scriptblock]::Create($processStart.Value + "`n" + $processFinish.Value)
function Get-Content {
    param($Path, $ErrorAction)
    if ($Path -ne '\installation\vcredist*.log') { throw 'Unexpected installer log path' }
}
foreach ($code in @(0, 3010, 1603)) {
    $vcInstallerLocation = $env:ComSpec
    $vcArgs = "/c exit $code"
    $failed = $false
    try { & $waitForInstaller *> $null } catch { $failed = $true }
    if ($failed -ne ($code -eq 1603)) { throw "Native process exit code mishandled: $code" }
    Write-Host "PASS: native exit code $code"
}

function Invoke-WebRequest {
    param($Uri, $OutFile, $ErrorAction)
    if ($script:scenario -eq 'download-failure') { throw 'Download failed' }
}

function Start-Process {
    param($FilePath, $ArgumentList, [switch]$NoNewWindow, [switch]$PassThru, [switch]$Wait, $ErrorAction)
    if ($Wait -or -not $PassThru) { throw 'Wait for the installer, not its descendant process tree' }
    if ($ArgumentList -ne '/install /quiet /norestart /log \installation\vcredist.log') { throw 'Installer logging or installation flags changed' }
    if ($script:scenario -eq 'start-failure') { throw 'Installer could not start' }
    $process = [pscustomobject]@{ ExitCode = $null }
    $process | Add-Member ScriptMethod WaitForExit {
        $script:waited = $true
        $this.ExitCode = $script:installerExitCode
    }
    return $process
}

function Copy-Item {
    param($Path, $Destination, $ErrorAction)
    if (-not $script:waited) { throw 'Runtime copied before installer completed' }
    if ($script:scenario -eq 'missing-runtime') { throw 'Runtime DLL missing' }
    $script:copies += $Path
}

foreach ($case in @(
    @{ Name = 'success'; Code = 0; Fails = $false },
    @{ Name = 'reboot-required'; Code = 3010; Fails = $false },
    @{ Name = 'installer-failure'; Code = 1603; Fails = $true },
    @{ Name = 'download-failure'; Code = 0; Fails = $true },
    @{ Name = 'start-failure'; Code = 0; Fails = $true },
    @{ Name = 'missing-runtime'; Code = 0; Fails = $true }
)) {
    $script:scenario = $case.Name
    $script:installerExitCode = $case.Code
    $script:waited = $false
    $script:copies = @()
    $failed = $false
    try { & $install *> $null } catch { $failed = $true }
    if ($failed -ne $case.Fails) { throw "Unexpected outcome: $($case.Name)" }
    if (-not $case.Fails) {
        if (-not $script:waited) { throw "Installer was not awaited: $($case.Name)" }
        $expected = '/Windows/System32/msvcp140.dll', '/Windows/System32/vccorlib140.dll', '/Windows/System32/vcruntime140.dll'
        if (($script:copies -join ',') -ne ($expected -join ',')) { throw "Runtime copies changed: $($case.Name)" }
    } elseif ($script:copies.Count -ne 0) {
        throw "Runtime copied after failure: $($case.Name)"
    }
    Write-Host "PASS: $($case.Name)"
}
