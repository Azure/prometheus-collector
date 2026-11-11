param(
    [ValidateSet('both', 'direct', 'runas')]
    [string]$Mode = 'both',
    [int]$RunAsTimeoutSeconds = 1860
)

$ErrorActionPreference = 'Stop'
foreach ($launch in @('direct', 'runas')) {
    if ($Mode -ne 'both' -and $Mode -ne $launch) { continue }
    $job = Start-Job -ArgumentList $launch -ScriptBlock {
        param($launch)
        $ErrorActionPreference = 'Stop'
        Write-Output ("{0} {1} launch-start" -f [DateTime]::UtcNow.ToString('o'), $launch)
        $arguments = @{ FilePath = $env:ComSpec; ArgumentList = '/d /c exit 0'; PassThru = $true }
        if ($launch -eq 'runas') { $arguments.Verb = 'RunAs' } else { $arguments.NoNewWindow = $true }
        $process = Start-Process @arguments
        $process.Handle | Out-Null
        if (-not $process.WaitForExit(5000)) { throw "$launch child did not exit" }
        Write-Output ("{0} {1} launch-end exit={2}" -f [DateTime]::UtcNow.ToString('o'), $launch, $process.ExitCode)
        if ($process.ExitCode -ne 0) { throw "$launch child failed: $($process.ExitCode)" }
    }
    $timeout = if ($launch -eq 'direct') { 60 } else { $RunAsTimeoutSeconds }
    if ($null -eq (Wait-Job -Job $job -Timeout $timeout)) {
        Receive-Job -Job $job -ErrorAction Continue
        Write-Output ("{0} {1} timeout after {2}s; exit unavailable" -f [DateTime]::UtcNow.ToString('o'), $launch, $timeout)
        Stop-Job -Job $job
        Remove-Job -Job $job -Force
        continue
    }
    try {
        Receive-Job -Job $job -ErrorAction Stop
        if ($job.State -ne 'Completed') { throw "$launch probe failed: $($job.State)" }
    } finally {
        Remove-Job -Job $job -Force
    }
}
