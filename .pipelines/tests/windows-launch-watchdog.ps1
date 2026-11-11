param(
    [string]$Image,
    [string]$EncodedScript,
    [int]$TimeoutSeconds = 1980,
    [int]$PollSeconds = 5
)

$ErrorActionPreference = 'Stop'
$container = @(docker run --detach --isolation=hyperv $Image powershell.exe -NoProfile -EncodedCommand $EncodedScript)
if ($LASTEXITCODE -ne 0 -or $container.Count -ne 1 -or $container[0] -notmatch '^[0-9a-f]{64}$') {
    throw 'Diagnostic container did not return an exact container ID'
}
$id = $container[0]
try {
    $clock = [Diagnostics.Stopwatch]::StartNew()
    do {
        $json = docker inspect --format '{{json .State}}' $id
        if ($LASTEXITCODE -ne 0) { throw "Cannot inspect diagnostic container $id" }
        $state = $json | ConvertFrom-Json
        if ($state.Running -isnot [bool] -or $state.Status -isnot [string] -or
            ($state.ExitCode -isnot [int] -and $state.ExitCode -isnot [long])) {
            throw "Malformed diagnostic container state for $id"
        }
        if (-not $state.Running) { break }
        Start-Sleep -Seconds $PollSeconds
    } while ($clock.Elapsed.TotalSeconds -lt $TimeoutSeconds)

    docker logs $id
    if ($LASTEXITCODE -ne 0) { throw "Cannot read diagnostic container $id logs" }
    if ($state.Running) { throw "Diagnostic container timed out after $TimeoutSeconds seconds; exit unavailable" }
    if ($state.Status -ne 'exited') { throw "Diagnostic container stopped in state $($state.Status)" }
    if ($state.ExitCode -ne 0) { throw "Diagnostic container failed with exit code $($state.ExitCode)" }
} finally {
    docker rm --force $id
    if ($LASTEXITCODE -ne 0) { throw "Cannot remove diagnostic container $id" }
}
