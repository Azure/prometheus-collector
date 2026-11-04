param([switch]$VerifyOutputOwnership)
$ErrorActionPreference = 'Stop'
$output = [IO.Path]::TrimEndingDirectorySeparator([IO.Path]::GetFullPath($env:SCAN_OUTPUT))
$claimPath = "$output.claim"
if ([string]::IsNullOrWhiteSpace($env:BUILD_BUILDID)) { throw 'BLOCKED: actual scan build ID required' }
if ($VerifyOutputOwnership) {
    $receipt = [IO.File]::ReadAllText($claimPath) | ConvertFrom-Json
    if ($receipt.owner -isnot [string] -or $receipt.output -isnot [string] -or
        $receipt.buildId -isnot [string] -or $env:SCAN_OUTPUT_OWNER -cnotmatch '^[0-9a-f]{32}$' -or
        $receipt.owner -cne $env:SCAN_OUTPUT_OWNER -or $receipt.output -cne $output -or
        $receipt.buildId -cne $env:BUILD_BUILDID -or -not [IO.Directory]::Exists($output)) {
        throw 'BLOCKED: output publication claim does not match this invocation'
    }
    Write-Host "##vso[task.setvariable variable=scanOutputPublishable]true"
    return
}
function Invoke-Docker {
    & docker @args
    if ($LASTEXITCODE -ne 0) { throw "Docker $($args[0]) failed: $LASTEXITCODE" }
}
$targets = Get-Content $env:TARGETS -Raw | ConvertFrom-Json
$target = $targets.images[[int]$env:TARGET_INDEX]
if ($target.ref -notmatch '^[^\s@]+@sha256:[0-9a-f]{64}$' -or
    $env:SCANNER_IMAGE -notmatch '^[^\s@]+@sha256:[0-9a-f]{64}$') {
    throw 'BLOCKED: target and scanner must be digest-pinned'
}
$owner = (New-Guid).ToString('N')
# Keep the claim after exit or failure; contenders must never adopt an earlier invocation's root.
$claim = [IO.File]::Open($claimPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
try {
    $receipt = [Text.Encoding]::UTF8.GetBytes((@{owner=$owner; output=$output; buildId=$env:BUILD_BUILDID} |
        ConvertTo-Json))
    $claim.Write($receipt, 0, $receipt.Length)
    $claim.Flush($true)
}
finally { $claim.Dispose() }
New-Item -ItemType Directory -Path $output | Out-Null
Write-Host "##vso[task.setvariable variable=scanOutputOwner]$owner"
Write-Host "##vso[task.setvariable variable=scanOutputOwned]true"
$volume = "prscan-$owner"
$pullAttempted = $false
$volumeAttempted = $false
$failure = $null
$cleanupErrors = [Collections.Generic.List[string]]::new()
try {
    New-Item -ItemType Directory -Force "$output/comet", "$output/trivy" | Out-Null
    $started = [DateTime]::UtcNow.ToString('o')
    $imagesBefore = @(Invoke-Docker image ls --all --quiet --no-trunc)
    $pullAttempted = $true
    Invoke-Docker pull $target.ref
    $inspect = (Invoke-Docker image inspect $target.ref | ConvertFrom-Json)[0]
    if ($target.ref -notin $inspect.RepoDigests -or $inspect.Os -ne $target.os -or
        $inspect.Architecture -ne $target.arch -or
        ($target.os -eq 'windows' -and $inspect.OsVersion -ne $target.osVersion)) {
        throw 'BLOCKED: pulled image identity/platform mismatch'
    }
    $driver = Invoke-Docker info --format '{{.Driver}}'
    if ($target.os -eq 'linux' -and $driver -ne 'overlay2') { throw 'BLOCKED: overlay2 required' }
    $uri = "$env:SYSTEM_COLLECTIONURI$env:SYSTEM_TEAMPROJECTID/_apis/distributedtask/hubs/build/plans/$env:SYSTEM_PLANID/jobs/$env:SYSTEM_JOBID/oidctoken?api-version=7.1-preview.1"
    $token = (Invoke-RestMethod -Method Post -Uri $uri -Headers @{
        Authorization = "Bearer $env:SYSTEM_ACCESSTOKEN"
    }).oidcToken
    if (-not $token) { throw 'BLOCKED: no job OIDC token' }
    Write-Host "##vso[task.setsecret]$token"
    if ($volume -in @(Invoke-Docker volume ls --quiet)) { throw 'BLOCKED: scan volume already exists' }
    $volumeAttempted = $true
    Invoke-Docker volume create --label "pr-e2e.invocation=$owner" $volume | Out-Null
    $volumeInfo = (Invoke-Docker volume inspect $volume | ConvertFrom-Json)[0]
    if ($volumeInfo.Labels.'pr-e2e.invocation' -ne $owner) { throw 'BLOCKED: scan volume ownership mismatch' }
    $mount = if ($target.os -eq 'windows') { '\\.\pipe\docker_engine:\\.\pipe\docker_engine' } else { '/var/run/docker.sock:/var/run/docker.sock' }
    $shared = if ($target.os -eq 'windows') { "${volume}:C:\scanutilitywin" } else { "${volume}:/scanutility" }
    $result = if ($target.os -eq 'windows') { 'C:\results' } else { '/results' }
    $isolation = if ($target.os -eq 'windows') { @('--isolation=hyperv') } else { @() }
    $languages = @('JAVA', 'Python', 'GO', 'Node.js', '.Net', 'Ruby')
    $scanArguments = @('--force-manifest-download', '--min-severity-results', '0',
        '--scan-languages', ($languages -join ','))
    # ponytail: container CLI shape is documented, but host/path compatibility still needs a proof run.
    $lines = & docker run --rm --pull=never @isolation -v $mount -v $shared -v "${output}/comet:$result" $env:SCANNER_IMAGE `
        scan --oidc-token $token @scanArguments --scan-result-path $result $target.ref 2>&1
    $exitCode = $LASTEXITCODE
    $safe = ($lines -join "`n").Replace($token, '[REDACTED]') -replace 'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+', '[REDACTED]'
    [IO.File]::WriteAllText("$output/comet/scan.log", $safe)
    @{exitCode=$exitCode; finishedUtc=[DateTime]::UtcNow.ToString('o')} |
        ConvertTo-Json | Set-Content "$output/completion.json"
    @{requestedRef=$target.ref; target=$inspect; commit=$targets.source.commit; buildId=$targets.source.buildId;
      scannedInBuildId=$env:BUILD_BUILDID; scannerImage=$env:SCANNER_IMAGE; startedUtc=$started;
      scanConfiguration=@{arguments=$scanArguments}; configurationVerified=$false;
      languagesRequested=$languages; languagesVerified=@();
      storageDriver=$driver} | ConvertTo-Json -Depth 20 | Set-Content "$output/provenance.json"
    if ($target.os -eq 'linux') {
        $actual = & trivy --version --format json
        if ($LASTEXITCODE -ne 0 -or ($actual | ConvertFrom-Json).Version -ne $env:TRIVY_VERSION) {
            throw 'BLOCKED: pinned Trivy version is not provisioned'
        }
        & trivy image --platform "$($target.os)/$($target.arch)" --format json --exit-code 0 `
            --ignorefile /dev/null --output "$output/trivy/result.json" $target.ref
        if ($LASTEXITCODE -ne 0) { throw 'BLOCKED: Trivy failed' }
        $metadata = & trivy --version --format json
        if ($LASTEXITCODE -ne 0) { throw 'BLOCKED: Trivy DB provenance failed' }
        @{exitCode=0; dbUpdatedAt=($metadata | ConvertFrom-Json).VulnerabilityDB.UpdatedAt;
          finishedUtc=[DateTime]::UtcNow.ToString('o'); version=($metadata | ConvertFrom-Json).Version} |
            ConvertTo-Json | Set-Content "$output/trivy/metadata.json"
    }
    if ($exitCode -notin 0,1) { throw "BLOCKED: CoMET exit $exitCode" }
    if (-not (Test-Path -LiteralPath "$output/comet/result.xml" -PathType Leaf)) {
        throw 'BLOCKED: CoMET XML output missing'
    }
}
catch {
    $failure = $_
}
finally {
    if ($volumeAttempted) {
        try {
            if ($volume -in @(Invoke-Docker volume ls --quiet)) {
                $volumeInfo = (Invoke-Docker volume inspect $volume | ConvertFrom-Json)[0]
                if ($volumeInfo.Labels.'pr-e2e.invocation' -ne $owner) {
                    throw 'BLOCKED: refusing cleanup of a volume without invocation ownership'
                }
                Invoke-Docker volume rm $volume | Out-Null
            }
        }
        catch { $cleanupErrors.Add("Volume cleanup: $($_.Exception.Message)") }
    }
    if ($pullAttempted) {
        try {
            $newImages = @(Invoke-Docker image ls --all --quiet --no-trunc |
                Where-Object { $_ -notin $imagesBefore })
            foreach ($image in $newImages) {
                $imageInfo = (Invoke-Docker image inspect $image | ConvertFrom-Json)[0]
                if ($target.ref -in $imageInfo.RepoDigests) {
                    Invoke-Docker image rm $target.ref | Out-Null
                    break
                }
            }
        }
        catch { $cleanupErrors.Add("Image cleanup: $($_.Exception.Message)") }
    }
    try {
        Get-ChildItem $output -Recurse -File | Get-FileHash -Algorithm SHA256 |
            Select-Object Path,Hash | ConvertTo-Json | Set-Content "$output/sha256sums.json"
    }
    catch { $cleanupErrors.Add("Evidence hashing: $($_.Exception.Message)") }
}
if ($failure) {
    foreach ($errorMessage in $cleanupErrors) { Write-Warning $errorMessage -WarningAction Continue }
    throw $failure
}
if ($cleanupErrors.Count) { throw "Scanner cleanup failed: $($cleanupErrors -join '; ')" }
