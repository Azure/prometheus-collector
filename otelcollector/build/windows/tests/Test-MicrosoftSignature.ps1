$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '..\scripts\Test-MicrosoftSignature.ps1')

$temporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ('prom-signature-tests-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
$script:passed = 0
$script:signatureStatus = 'Valid'
$script:signerSubject = 'CN=Microsoft Corporation, O=Microsoft Corporation, C=US'

function Get-AuthenticodeSignature {
    [CmdletBinding()]
    param([string]$LiteralPath)
    [pscustomobject]@{
        Status = $script:signatureStatus
        SignerCertificate = if ($null -eq $script:signerSubject) { $null } else { [pscustomobject]@{ Subject = $script:signerSubject } }
    }
}

function Test-Accepted {
    param([string]$Name, [scriptblock]$Action)
    & $Action | Out-Null
    $script:passed++
    Write-Host "PASS: $Name"
}

function Test-Rejected {
    param([string]$Name, [scriptblock]$Action, [string]$ExpectedMessage)
    $message = $null
    try { & $Action | Out-Null } catch { $message = $_.Exception.Message }
    if ($null -eq $message -or $message -notmatch $ExpectedMessage) {
        throw "Expected rejection for '$Name'; received '$message'."
    }
    $script:passed++
    Write-Host "PASS: $Name"
}

try {
    $payload = Join-Path $temporaryDirectory 'payload.exe'
    Set-Content -LiteralPath $payload -Value 'not executed' -Encoding ASCII
    Test-Accepted 'Valid Microsoft executable' { Test-MicrosoftCodeSignature -LiteralPath $payload }
    foreach ($status in @('NotSigned', 'HashMismatch', 'NotTrusted', 'UnknownError')) {
        $script:signatureStatus = $status
        Test-Rejected "Reject executable status $status" { Test-MicrosoftCodeSignature -LiteralPath $payload } 'valid Microsoft code signature'
    }
    $script:signatureStatus = 'Valid'
    foreach ($subject in @('CN=Microsoft Corporation, O=Other Corporation, C=US', 'CN=Other, O=Microsoft Corporation Evil, C=US', $null)) {
        $script:signerSubject = $subject
        Test-Rejected 'Reject wrong or missing publisher' { Test-MicrosoftCodeSignature -LiteralPath $payload } 'valid Microsoft code signature'
    }
    $script:signerSubject = 'CN=Microsoft Corporation, O=Microsoft Corporation, C=US'

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $package = Join-Path $temporaryDirectory 'package.nupkg'
    $archive = [System.IO.Compression.ZipFile]::Open($package, [System.IO.Compression.ZipArchiveMode]::Create)
    $archive.Dispose()
    Test-Rejected 'Reject unsigned NuGet archive' { Get-NuGetSigningCertificate -LiteralPath $package } 'has no signature'

    function Get-NuGetSigningCertificate {
        param([string]$LiteralPath)
        [pscustomobject]@{ Subject = $script:packagePublisher; RawData = [byte[]]@(1, 2, 3) }
    }
    $nuget = Join-Path $temporaryDirectory 'nuget.cmd'
    $commandLog = Join-Path $temporaryDirectory 'command.txt'
    $config = Join-Path $temporaryDirectory 'NuGet.Config'
    Set-Content -LiteralPath $config -Value '<configuration />' -Encoding UTF8
    Set-Content -LiteralPath $nuget -Value @('@echo off', "@echo %* > `"$commandLog`"", 'exit /b 0') -Encoding ASCII

    $script:packagePublisher = 'CN=Other Publisher, O=Other Corporation, C=US'
    Test-Rejected 'Reject non-Microsoft NuGet signer' {
        Test-MicrosoftNuGetSignature -LiteralPath $package -NuGetPath $nuget -ConfigPath $config
    } 'must be signed by Microsoft'
    if (Test-Path -LiteralPath $commandLog) { throw 'Verifier ran for an unapproved package publisher.' }

    $script:packagePublisher = 'CN=Microsoft Corporation, O=Microsoft Corporation, C=US'
    $script:signatureStatus = 'NotTrusted'
    Test-Rejected 'Reject untrusted verification tool before execution' {
        Test-MicrosoftNuGetSignature -LiteralPath $package -NuGetPath $nuget -ConfigPath $config
    } 'valid Microsoft code signature'
    if (Test-Path -LiteralPath $commandLog) { throw 'Untrusted verification tool was executed.' }

    $script:signatureStatus = 'Valid'
    Set-Content -LiteralPath $nuget -Value @('@echo off', 'exit /b 7') -Encoding ASCII
    Test-Rejected 'Propagate package verifier failure' {
        Test-MicrosoftNuGetSignature -LiteralPath $package -NuGetPath $nuget -ConfigPath $config
    } 'NuGet signature verification failed'

    Set-Content -LiteralPath $nuget -Value @('@echo off', "@echo %* > `"$commandLog`"", 'exit /b 0') -Encoding ASCII
    Test-Accepted 'Run full verification with the expected signer and isolated configuration' {
        Test-MicrosoftNuGetSignature -LiteralPath $package -NuGetPath $nuget -ConfigPath $config
        $arguments = Get-Content -LiteralPath $commandLog -Raw
        if ($arguments -notmatch '-All' -or $arguments -notmatch '-CertificateFingerprint [A-F0-9]{64}' -or
            -not $arguments.Contains($config)) {
            throw 'Full signature verification arguments were not passed.'
        }
    }
    Write-Host "Passed $script:passed signature guard tests."
}
finally {
    Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
}
