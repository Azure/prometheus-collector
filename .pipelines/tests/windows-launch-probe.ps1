param(
    [ValidateSet('both', 'direct', 'runas')]
    [string]$Mode = 'both'
)

$ErrorActionPreference = 'Stop'
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class LaunchProbe {
    [DllImport("kernel32.dll", SetLastError = true)]
    public static extern bool GetExitCodeProcess(IntPtr process, out uint exitCode);
}
'@

foreach ($launch in @('direct', 'runas')) {
    if ($Mode -ne 'both' -and $Mode -ne $launch) { continue }
    $start = [System.Diagnostics.ProcessStartInfo]::new()
    $start.FileName = "$env:SystemRoot\System32\WindowsPowerShell\v1.0\powershell.exe"
    $start.Arguments = '-NoProfile -NonInteractive -Command "Start-Sleep -Seconds 2; exit 0"'
    $start.UseShellExecute = ($launch -eq 'runas')
    if ($launch -eq 'runas') { $start.Verb = 'runas' } else { $start.CreateNoWindow = $true }
    Write-Output ("{0} {1} launch-start" -f [DateTime]::UtcNow.ToString('o'), $launch)
    $process = [System.Diagnostics.Process]::Start($start)
    if ($null -eq $process) { throw "$launch process did not start" }
    $handle = $process.Handle
    $process.WaitForExit()
    $exitCode = [uint32]0
    if (-not [LaunchProbe]::GetExitCodeProcess($handle, [ref]$exitCode)) {
        throw [System.ComponentModel.Win32Exception]::new([Runtime.InteropServices.Marshal]::GetLastWin32Error())
    }
    Write-Output ("{0} {1} launch-end exit={2}" -f [DateTime]::UtcNow.ToString('o'), $launch, $exitCode)
    if ($exitCode -ne 0) { throw "$launch child failed: $exitCode" }
}
