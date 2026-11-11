$ErrorActionPreference = 'Stop'
$global:probeContainer = 'a' * 64

function docker {
    $global:LASTEXITCODE = 0
    switch ($args[0]) {
        'run' {
            if ($global:probeCase -eq 'bad-id') { 'invalid' } else { $global:probeContainer }
        }
        'inspect' {
            switch ($global:probeCase) {
                'timeout' { '{"Running":true,"Status":"running","ExitCode":0}' }
                'nonzero' { '{"Running":false,"Status":"exited","ExitCode":7}' }
                'malformed' { '{"Running":false}' }
                'inspect-error' { $global:LASTEXITCODE = 1 }
                default { '{"Running":false,"Status":"exited","ExitCode":0}' }
            }
        }
        'logs' { 'diagnostic output' }
        'rm' {
            if ($args[-1] -ne $global:probeContainer) { throw 'Unexpected container removal' }
            $global:probeRemoved++
        }
        default { throw "Unexpected Docker command: $($args[0])" }
    }
}

$cases = @{
    'success' = ''
    'nonzero' = 'failed with exit code 7'
    'timeout' = 'timed out after 1 seconds'
    'malformed' = 'Malformed diagnostic container state'
    'inspect-error' = 'Cannot inspect diagnostic container'
    'bad-id' = 'did not return an exact container ID'
}
foreach ($global:probeCase in $cases.Keys) {
    $global:probeRemoved = 0
    $message = ''
    try {
        & "$PSScriptRoot\windows-launch-watchdog.ps1" -Image 'fake' -EncodedScript 'fake' -TimeoutSeconds 1 -PollSeconds 1 | Out-Null
    } catch {
        $message = $_.Exception.Message
    }
    if (($cases[$global:probeCase] -eq '' -and $message -ne '') -or
        ($cases[$global:probeCase] -ne '' -and -not $message.Contains($cases[$global:probeCase]))) {
        throw "Unexpected result for ${global:probeCase}: $message"
    }
    $expectedRemoved = if ($global:probeCase -eq 'bad-id') { 0 } else { 1 }
    if ($global:probeRemoved -ne $expectedRemoved) { throw "Unexpected cleanup for $global:probeCase" }
    Write-Output "PASS $global:probeCase"
}
