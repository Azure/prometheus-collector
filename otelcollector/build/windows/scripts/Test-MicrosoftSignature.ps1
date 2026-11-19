function Test-MicrosoftCodeSignature {
    param([Parameter(Mandatory = $true)][string]$LiteralPath)

    $signature = Get-AuthenticodeSignature -LiteralPath $LiteralPath -ErrorAction Stop
    if ($signature.Status -ne 'Valid' -or
        $signature.SignerCertificate.Subject -notmatch '(^|,\s*)O=Microsoft Corporation(,|$)') {
        throw "A valid Microsoft code signature is required: $LiteralPath"
    }
}

function Get-NuGetSigningCertificate {
    param([Parameter(Mandatory = $true)][string]$LiteralPath)

    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    try {
        Add-Type -AssemblyName System.Security.Cryptography.Pkcs -ErrorAction Stop
    }
    catch {
        Add-Type -AssemblyName System.Security -ErrorAction Stop
    }

    $archive = [System.IO.Compression.ZipFile]::OpenRead($LiteralPath)
    try {
        $entry = $archive.GetEntry('.signature.p7s')
        if ($null -eq $entry) {
            throw "NuGet package has no signature: $LiteralPath"
        }
        $stream = $entry.Open()
        $buffer = New-Object System.IO.MemoryStream
        try {
            $stream.CopyTo($buffer)
            $signature = New-Object System.Security.Cryptography.Pkcs.SignedCms
            $signature.Decode($buffer.ToArray())
            if ($signature.SignerInfos.Count -ne 1 -or $null -eq $signature.SignerInfos[0].Certificate) {
                throw "NuGet package must have one primary signing certificate: $LiteralPath"
            }
            return $signature.SignerInfos[0].Certificate
        }
        finally {
            $buffer.Dispose()
            $stream.Dispose()
        }
    }
    finally {
        $archive.Dispose()
    }
}

function Test-MicrosoftNuGetSignature {
    param(
        [Parameter(Mandatory = $true)][string]$LiteralPath,
        [Parameter(Mandatory = $true)][string]$NuGetPath,
        [Parameter(Mandatory = $true)][string]$ConfigPath
    )

    $certificate = Get-NuGetSigningCertificate -LiteralPath $LiteralPath
    if ($certificate.Subject -notmatch '(^|,\s*)O=Microsoft Corporation(,|$)') {
        throw "NuGet package must be signed by Microsoft: $LiteralPath"
    }
    Test-MicrosoftCodeSignature -LiteralPath $NuGetPath
    $sha256 = [System.Security.Cryptography.SHA256]::Create()
    try {
        $fingerprint = [BitConverter]::ToString($sha256.ComputeHash($certificate.RawData)).Replace('-', '')
    }
    finally {
        $sha256.Dispose()
    }

    & $NuGetPath verify $LiteralPath -All -CertificateFingerprint $fingerprint -ConfigFile $ConfigPath -NonInteractive
    if ($LASTEXITCODE -ne 0) {
        throw "NuGet signature verification failed: $LiteralPath"
    }
}
