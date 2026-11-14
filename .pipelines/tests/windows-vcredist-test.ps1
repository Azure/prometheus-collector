$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$setupPath = Join-Path $PSScriptRoot '..\..\otelcollector\build\windows\scripts\setup.ps1'
$setup = Get-Content $setupPath -Raw
$block = [regex]::Match($setup, "(?s)Write-Host \('Installing Visual C\+\+ Redistributable Package'\).*?Write-Host \('Finished Installing Visual C\+\+ Redistributable Package'\)")
if (-not $block.Success) { throw 'VC++ setup block not found' }
$progressIndex = $setup.IndexOf("`$ProgressPreference = 'SilentlyContinue'")
$firstDownload = $setup.IndexOf('Invoke-WebRequest')
if ($progressIndex -lt 0 -or $progressIndex -gt $firstDownload) {
    throw 'Progress output must be suppressed before the first download'
}
foreach ($marker in @('Finished installing Metrics Extension', 'Finished installing fluentbit')) {
    if ($block.Index -lt $setup.IndexOf($marker)) { throw "VC++ install must follow $marker" }
}
if ($setup.IndexOf("Write-Host ('Installing Telegraf')") -lt ($block.Index + $block.Length)) {
    throw 'VC++ install must finish before Telegraf setup'
}
$extract = [Management.Automation.Language.Parser]::ParseInput($setup, [ref]$null, [ref]$null).Find({
    param($ast) $ast -is [Management.Automation.Language.FunctionDefinitionAst] -and $ast.Name -eq 'Get-VcRedistMsiPackages'
}, $true)
if (-not $extract -or $block.Value.IndexOf($extract.Extent.Text) -lt 0) { throw 'VC++ MSI extraction not found in VC++ block' }
$install = [scriptblock]::Create($block.Value.Replace($extract.Extent.Text, ''))

# Extract the bundle setup.ps1 downloads, so a Burn layout change fails here instead of in docker build.
. ([scriptblock]::Create($extract.Extent.Text))
$work = Join-Path ([IO.Path]::GetTempPath()) "vcredist-test-$PID"
New-Item -Type Directory -Path $work -Force | Out-Null
$url = [regex]::Match($block.Value, "\`$vcRedistLocation = '([^']+)'").Groups[1].Value
Invoke-WebRequest -Uri $url -OutFile "$work\vc_redist.x64.exe" -UseBasicParsing
$packages = @(Get-VcRedistMsiPackages "$work\vc_redist.x64.exe" "$work\real")
if (($packages.Id -join ',') -ne 'vcRuntimeMinimum_x64,vcRuntimeAdditional_x64') { throw 'Unexpected VC++ MSI packages' }
foreach ($package in $packages) {
    if (-not (Test-Path $package.Path) -or -not (Test-Path (Join-Path (Split-Path $package.Path) 'cab1.cab'))) { throw "MSI or cab missing: $($package.Id)" }
    if ($package.Properties -ne 'MSIFASTINSTALL=7 NOVSUI=1') { throw "Unexpected MSI properties: $($package.Properties)" }
}
Write-Host 'PASS: bundle MSI extraction'
function Get-FileHash { [pscustomobject]@{ Hash = 'tampered' } }
$failed = $false
try { Get-VcRedistMsiPackages "$work\vc_redist.x64.exe" "$work\tampered" *> $null } catch { $failed = $true }
if (-not $failed) { throw 'Payload hash mismatch was not rejected' }
Remove-Item function:Get-FileHash
Remove-Item $work -Recurse -Force
Write-Host 'PASS: payload hash mismatch'

$script:packages = @(
    [pscustomobject]@{ Id = 'min'; Path = '\installation\vcredist\min.msi'; Properties = 'A=1' },
    [pscustomobject]@{ Id = 'add'; Path = '\installation\vcredist\add.msi'; Properties = 'B=2' }
)
function Invoke-WebRequest {
    param($Uri, $OutFile, $ErrorAction)
    if ($script:scenario -eq 'download-failure') { throw 'Download failed' }
}
function Get-VcRedistMsiPackages {
    param($Bundle, $Destination)
    if ($script:scenario -eq 'extract-failure') { throw 'Extraction failed' }
    $script:packages
}
function Start-Process {
    param($FilePath, $ArgumentList, [switch]$NoNewWindow, [switch]$PassThru, [switch]$Wait, $ErrorAction)
    if ($Wait -or -not $PassThru) { throw 'Wait for the installer, not its descendant process tree' }
    $package = $script:packages[$script:started.Count]
    $expected = "/i `"$($package.Path)`" /qn /norestart $($package.Properties) /l*v \installation\vcredist_$($package.Id).log"
    if ($FilePath -ne 'msiexec.exe' -or $ArgumentList -ne $expected) { throw "Installer logging or installation flags changed: $ArgumentList" }
    if ($script:scenario -eq 'start-failure') { throw 'Installer could not start' }
    # A real, briefly running process checks the wait and Windows PowerShell ExitCode handling.
    $process = Microsoft.PowerShell.Management\Start-Process $env:ComSpec -ArgumentList "/c ping -n 2 127.0.0.1 >nul & exit $($script:exitCodes[$script:started.Count])" -NoNewWindow -PassThru
    $script:started += $process
    $process
}
function Get-Content {
    param($Path, $ErrorAction)
    if ($Path -ne "\installation\vcredist_$($script:packages[$script:started.Count - 1].Id).log") { throw "Unexpected installer log path: $Path" }
}
function Copy-Item {
    param($Path, $Destination, $ErrorAction)
    if ($script:started.Count -ne 2 -or ($script:started | Where-Object { -not $_.HasExited })) { throw 'Runtime copied before installers completed' }
    if ($script:scenario -eq 'missing-runtime') { throw 'Runtime DLL missing' }
    $script:copies += $Path
}

foreach ($case in @(
    @{ Name = 'success'; Codes = 0, 0; Fails = $false },
    @{ Name = 'reboot-required'; Codes = 0, 3010; Fails = $false },
    @{ Name = 'installer-failure'; Codes = 1603, 0; Fails = $true },
    @{ Name = 'download-failure'; Codes = 0, 0; Fails = $true },
    @{ Name = 'extract-failure'; Codes = 0, 0; Fails = $true },
    @{ Name = 'start-failure'; Codes = 0, 0; Fails = $true },
    @{ Name = 'missing-runtime'; Codes = 0, 0; Fails = $true }
)) {
    $script:scenario = $case.Name
    $script:exitCodes = $case.Codes
    $script:started = @()
    $script:copies = @()
    $failed = $false
    try { & $install *> $null } catch { $failed = $true }
    if ($failed -ne $case.Fails) { throw "Unexpected outcome: $($case.Name)" }
    if (-not $case.Fails) {
        $expected = '/Windows/System32/msvcp140.dll', '/Windows/System32/vccorlib140.dll', '/Windows/System32/vcruntime140.dll'
        if (($script:copies -join ',') -ne ($expected -join ',')) { throw "Runtime copies changed: $($case.Name)" }
    } elseif ($script:copies.Count -ne 0) {
        throw "Runtime copied after failure: $($case.Name)"
    }
    if ($case.Name -eq 'installer-failure' -and $script:started.Count -ne 1) { throw 'Installed a package after a failed one' }
    Write-Host "PASS: $($case.Name)"
}