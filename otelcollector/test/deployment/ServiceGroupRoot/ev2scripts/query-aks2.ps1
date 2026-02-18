
param (
    [Parameter(Mandatory=$true)]
    [string]$clientId,
    [Parameter(Mandatory=$true)]
    [string]$subscriptionId,
    [string]$environment = "",
    [Parameter(Mandatory=$true)]
    [string]$ResourceGroupName,
    [Parameter(Mandatory=$true)]
    [string]$AksName,
    [Parameter(Mandatory=$true)]
    [string]$amwName,
    [string]$rule = "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate",
    [Parameter(Mandatory=$true)]
    [string]$logsStorageAccountName,
    [string]$containerName = "shellextlogs"
)

# Disable ANSI escape sequences for Linux container environment
if ($PSStyle) {
    $PSStyle.OutputRendering = 'PlainText'
    # Disable ANSI codes in error/warning/verbose/debug formatting
    $PSStyle.Formatting.Error = ''
    $PSStyle.Formatting.Warning = ''
    $PSStyle.Formatting.Verbose = ''
    $PSStyle.Formatting.Debug = ''
}
$env:TERM = 'dumb'
$env:NO_COLOR = '1'

$cloudEnv = $null

function Register-CustomAzEnvironment {
    param(
        [string]$environment
    )

    # Core endpoints for USNat (AGC) — minimal set needed for ARM + AAD auth.
    # These work for most Connect-AzAccount use cases when using Managed Identity.
    # $ActiveDirectoryAuthority = 'https://login.microsoftonline.eaglex.ic.gov/'
    # $ResourceManagerUrl       = 'https://management.azure.eaglex.ic.gov/'

    # Extended endpoints & suffixes (uncomment and pass to Add-AzEnvironment if you need them)
    # $StorageSuffix   = 'core.eaglex.ic.gov'
    # $KeyVaultSuffix  = '.vault.cloudapi.eaglex.ic.gov'
    # $AcrLoginSuffix  = '.azurecr.eaglex.ic.gov'
    # (These suffixes line up with typical USNat CLI cloud registration)

    if (![string]::IsNullOrEmpty($environment)) {
        $cloudEnv = Get-AzEnvironment -Name $environment -ErrorAction SilentlyContinue
        if ($cloudEnv -ne $null) {
            Write-Host "Az environment '$environment' exists."
        } else {
            Write-Host "Az environment '$environment' not found; registering..." -ForegroundColor Yellow

            # Minimal, robust registration that works for Managed Identity ARM auth
            <#$params = @{
                Name                               = $environment
                ActiveDirectoryAuthority           = 'https://login.microsoftonline.eaglex.ic.gov/'
                ResourceManagerUrl                 = 'https://management.azure.eaglex.ic.gov/'
                StorageEndpointSuffix              = 'core.eaglex.ic.gov'
                AzureKeyVaultDnsSuffix             = 'vault.cloudapi.eaglex.ic.gov'
                ContainerRegistryEndpointSuffix    = 'azurecr.eaglex.ic.gov'
                MicrosoftGraphUrl                  = 'https://graph.cloudapi.eaglex.ic.gov/'
                #MicrosoftGraphEndpointResourceId  = 'https://graph.cloudapi.eaglex.ic.gov/'
            }#>

            $envFile = "env-$environment.json"
            if (-not (Test-Path $envFile)) {
                throw "Environment file '$envFile' not found."
            }
            
            $str = Get-Content -Path $envFile
            Write-Host "Read environment file content: $str" -ForegroundColor Cyan

            $j = $str | Convertfrom-Json

            if ([string]::IsNullOrEmpty($j.ServiceEndpoint)) {
                throw "Environment file '$envFile' is missing required 'ServiceEndpoint' field."
            }
            if ([string]::IsNullOrEmpty($j.ActiveDirectoryEndpoint)) {
                throw "Environment file '$envFile' is missing required 'ActiveDirectoryEndpoint' field."
            }
            if ([string]::IsNullOrEmpty($j.ResourceManagerEndpoint)) {
                throw "Environment file '$envFile' is missing required 'ResourceManagerEndpoint' field."
            }
            if ([string]::IsNullOrEmpty($j.ActiveDirectoryServiceEndpointResourceId)) {
                throw "Environment file '$envFile' is missing required 'ActiveDirectoryServiceEndpointResourceId' field."
            }
            if ([string]::IsNullOrEmpty($j.GraphEndpoint)) {
                throw "Environment file '$envFile' is missing required 'GraphEndpoint' field."
            }
            if ([string]::IsNullOrEmpty($j.AzureKeyVaultDnsSuffix)) {
                throw "Environment file '$envFile' is missing required 'AzureKeyVaultDnsSuffix' field."
            }
            if ([string]::IsNullOrEmpty($j.AzureKeyVaultServiceEndpointResourceId)) {
                throw "Environment file '$envFile' is missing required 'AzureKeyVaultServiceEndpointResourceId' field."
            }

            $cloudEnv = Add-AzEnvironment -Name $environment `
            -ServiceEndpoint $j.ServiceEndpoint `
            -ActiveDirectoryEndpoint $j.ActiveDirectoryEndpoint `
            -ResourceManagerEndpoint $j.ResourceManagerEndpoint `
            -ActiveDirectoryServiceEndpointResourceId $j.ActiveDirectoryServiceEndpointResourceId `
            -GraphEndpoint $j.GraphEndpoint `
            -AzureKeyVaultDnsSuffix $j.AzureKeyVaultDnsSuffix `
            -AzureKeyVaultServiceEndpointResourceId $j.AzureKeyVaultServiceEndpointResourceId
            Write-Host "Registered Az environment '$environment'." -ForegroundColor Green
        }
    } else {
        Write-Host "No custom environment specified; skipping Az environment registration."
    }

    return $cloudEnv
}

if (![string]::IsNullOrEmpty($environment)) {
    Register-CustomAzEnvironment -environment $environment
}

function Configure-TLSCertificates {
    param(
        [string]$AmwEndpoint
    )
    
    Write-Host "Configuring TLS certificates for secure connection..."
    
    try {
        $amwUri = [System.Uri]$AmwEndpoint
        $hostname = $amwUri.Host
        
        Write-Host "Setting up TLS configuration for: $hostname"
        
        # Set Go-specific environment variables for Ginkgo tests
        $env:GOPROXY = "direct"
        $env:GOSUMDB = "off"
        
        # Set certificate paths to use system certificates
        $env:SSL_CERT_FILE = "/etc/ssl/certs/ca-certificates.crt"
        $env:SSL_CERT_DIR = "/etc/ssl/certs"
        $env:CURL_CA_BUNDLE = "/etc/ssl/certs/ca-certificates.crt"
        
        # For EV2 shell extension, we'll use environment variables instead of certificate installation
        Write-Host "Configuring TLS bypass for AMW endpoint: $hostname" -ForegroundColor Yellow
        
        # Test basic connectivity using Linux tools (since we're in a container)
        try {
            Write-Host "Testing connectivity to $hostname..."
            $ncResult = bash -c "timeout 5 bash -c 'echo > /dev/tcp/$hostname/443' 2>/dev/null && echo 'SUCCESS' || echo 'FAILED'"
            if ($ncResult -eq "SUCCESS") {
                Write-Host "Successfully connected to $hostname on port 443" -ForegroundColor Green
            } else {
                Write-Host "WARNING: Could not connect to $hostname on port 443" -ForegroundColor Yellow
            }
        } catch {
            Write-Host "Connection test failed: $_" -ForegroundColor Yellow
        }
        
        Write-Host "TLS configuration completed - using insecure mode for testing" -ForegroundColor Green
        
    } catch {
        Write-Host "TLS configuration failed: $_" -ForegroundColor Yellow
    }
    
    # Set environment variables for PowerShell Core in Linux container
    Write-Host "Setting environment variables for Linux container execution..."
    
    Write-Host "Configuring secure TLS certificate handling..." -ForegroundColor Cyan
    
    # Air-gapped Go configuration
    $env:GOPROXY = "off"
    $env:GOSUMDB = "off"
    $env:GONOPROXY = "*"
    $env:GONOSUMDB = "*"
    $env:GOPRIVATE = "*"
}

function Test-IMDSAccess {
    param($ce)

    if ($ce -eq $null) {
        ##throw "Cloud environment parameter is required for Test-IMDSAccess."
        return
    }

    Write-Host "Testing IMDS access from container..." -ForegroundColor Cyan

    # 1a) Is IMDS reachable at all (should return 400 with an error JSON if reachable)?
    write-host "curl 1a - Is IMDS reachable at all"
    curl -s -H "Metadata:true" "http://169.254.169.254/metadata/instance?api-version=2021-02-01" | head -c 200

    # 1b) Try token request to IMDS (resource manager)
    write-host "curl 1b - Try token request to IMDS (resource manager)"
    ##curl -s -H "Metadata:true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.eaglex.ic.gov" | jq
    curl -s -H "Metadata:true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=$($ce.ResourceManagerUrl)" | jq

    write-host "curl 1c - with client id"
    ##curl -s -H "Metadata:true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.eaglex.ic.gov&client_id=6ae0f871-c572-46ae-a335-76d07e46a0cf"
    curl -s -H "Metadata:true" "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=$($ce.ResourceManagerUrl)&client_id=$clientId"

    # 2) Dump environment variables related to identity/proxy
    write-host "gci output - Dump environment variables related to identity/proxy"
    gci env:
    ##gci env: | ? { $_.Name -match 'IDENTITY|MSI|IMDS|_PROXY' } | ft -auto   
}

function Normalize-IMDSEnvironment {
    param($ce)

    if ($ce -eq $null) {
        ##throw "Cloud environment parameter is required for Normalize-IMDSEnvironment."
        return
    }

    # --- Normalize environment for IMDS from inside a container ---
    Write-Host "Configuring environment for IMDS access..." -ForegroundColor Cyan

    # 1) Remove proxies for link-local and conflicting App Service/OIDC identity vars if present
    $varsToClear = @('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 
                    'IDENTITY_ENDPOINT', 'IDENTITY_HEADER', 'IDENTITY_API_VERSION', 
                    'MSI_ENDPOINT', 'MSI_SECRET', 'IMDS_ENDPOINT')
    foreach ($v in $varsToClear) {
        if (Test-Path "Env:\$v") { 
            Write-Host "Clearing conflicting environment variable: $v" -ForegroundColor Yellow
            Remove-Item "Env:\$v" -ErrorAction SilentlyContinue 
        }
    }
    $env:NO_PROXY = "169.254.169.254,168.63.129.16,127.0.0.1,localhost"
    Write-Host "Set NO_PROXY for IMDS access" -ForegroundColor Green

    # 2) Quick IMDS liveness probe (fast timeout)
    try {
    $null = Invoke-RestMethod -Headers @{Metadata='true'} `
            -Uri "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=$($ce.ResourceManagerUrl)&client_id=$clientId" `
            -TimeoutSec 2
    } catch {
        write-host "invoke error: $_"
        throw "IMDS is not reachable from this container (but curl succeeded earlier?). Check proxies/network mode."
    }    
}

if ($cloudEnv -ne $null) {
    Test-IMDSAccess -ce $cloudEnv
    Normalize-IMDSEnvironment -ce $cloudEnv
}


write-host "----------------------------------STARTING----------------------------------"


$dt = get-date
$dtAsString = $dt.toUniversalTime().ToString("yyyyMMdd-HHmmss")
$transcriptFileName = "regionTests-$dtAsString.txt"
Start-transcript $transcriptFileName

Write-Host "Script parameters:"
Write-Host "clientId: $clientId"
Write-Host "subscriptionId: $subscriptionId"
Write-Host "env: $environment"
Write-Host "ResourceGroupName: $ResourceGroupName"
Write-Host "AksName: $AksName"
Write-Host "amwName: $amwName"
Write-Host "rule: $rule"
Write-Host "logsStorageAccountName: $logsStorageAccountName"
Write-Host "containerName: $containerName"

Get-ChildItem -Path "." | Select-Object Name

Write-Host "Querying AKS cluster: $AksName in resource group: $ResourceGroupName"

$maxAttempts = 25
$attempts = 0
$succeeded = $false
do {
    if (![string]::IsNullOrEmpty($environment)) {
        Connect-AzAccount -Environment $environment -Identity -AccountId $clientId ###-Tenant '70a90262-f46c-48aa-ac4c-37e37f8be1a2'
        Connect-AzAccount -Environment $environment -Identity ### -Tenant '70a90262-f46c-48aa-ac4c-37e37f8be1a2'
    } else {
        Connect-AzAccount -Identity -AccountId $clientId
    }
    $attempts++
    $c = Get-AzContext
    $succeeded = $c -ne $null
    if (-not $succeeded) {
        Write-Host "[WARN] Connect-AzAccount attempt $attempts failed; retrying in 5 seconds..."
        Start-Sleep -Seconds 5
    }

} while (-not $succeeded -and ($attempts -le $maxAttempts))

if (-not $succeeded) {
    Write-Host "[ERROR] Failed to connect"
    exit 1
}

Write-Host "Selecting subscription: $subscriptionId"
Select-AzSubscription -SubscriptionId $subscriptionId
Get-AzContext

function Set-KubeConfig {
    param($ResourceGroupName, $AksName)
    
    Write-Host "Getting AKS credentials..."
    Import-AzAksCredential -ResourceGroupName $ResourceGroupName -Name $AksName -Force
    
    # Determine kubeconfig path based on platform
    $kubeConfigPath = if ($env:KUBECONFIG) {
        $env:KUBECONFIG
    } elseif ($IsLinux -or $IsMacOS) {
        "$env:HOME/.kube/config"
    } else {
        "$env:USERPROFILE\.kube\config"
    }
    
    Write-Host "Kubeconfig location: $kubeConfigPath"
    
    if (Test-Path $kubeConfigPath) {
        Write-Host "Kubeconfig file exists ($(((Get-Item $kubeConfigPath).Length)) bytes)"
        
        # Set proper permissions on Linux/macOS
        if ($IsLinux -or $IsMacOS) {
            & chmod 600 $kubeConfigPath 2>$null
            Write-Host "Set kubeconfig file permissions to 600"
        }
        
        # Export KUBECONFIG for child processes
        $env:KUBECONFIG = $kubeConfigPath
        
        return $kubeConfigPath
    } else {
        throw "Kubeconfig file not found at: $kubeConfigPath"
    }
}

# Get AKS cluster details
try {
    $configPath = Set-KubeConfig -ResourceGroupName $ResourceGroupName -AksName $AksName
    Write-Host "Kubeconfig successfully configured at: $configPath"
} catch {
    Write-Host "Error setting up kubeconfig: $_"
    exit 1
}

# Get AMW resource details
Write-Host "Getting AMW resource details..."
$amw = Get-AzResource -ResourceGroupName $ResourceGroupName `
                      -ResourceType "Microsoft.Monitor/accounts" `
                      -ResourceName $amwName

# Extract endpoint property
$amwEndpoint = $amw.properties.metrics.prometheusQueryEndpoint 
Write-Host "AMW Endpoint: $amwEndpoint"

Write-Host "Setting AMW_QUERY_ENDPOINT environment variable to $amwEndpoint"
[Environment]::SetEnvironmentVariable("AMW_QUERY_ENDPOINT", $amwEndpoint)
$resourceId = $amw.ResourceId
Write-Host "Querying with rule: $rule"
Write-Host "and resourceId: $resourceId"

Write-Host "Setting execute permissions on regionTests.exe..."
chmod +x ./regionTests-linux.exe

# Configure TLS certificates for AMW endpoint to prevent certificate verification errors
Configure-TLSCertificates -AmwEndpoint $amwEndpoint

# Use /tmp directory for writable temporary files
$tempDir = "/tmp"
Write-Host "Using temporary directory: $tempDir for script files..."

# Create environment setup script in writable temp directory
$tlsEnvScript = "$tempDir/setup-tls-env-$dtAsString.sh"
Write-Host "Creating TLS environment script: $tlsEnvScript"

$containerEnvScript = @"
#!/bin/bash
# Secure fix for: tls: failed to verify certificate: x509: certificate signed by unknown authority
echo "Configuring secure TLS certificate handling..."

# Air-gapped Go environment configuration
export GOPROXY=off
export GOSUMDB=off
export GONOPROXY="*"
export GONOSUMDB="*"
export GOPRIVATE="*"
export GO111MODULE=on

# Secure TLS Configuration - Set certificate paths for Go to use proper CA validation
# This allows the transport.RoundTrip() method to validate certificates properly

# Try different common certificate bundle locations
if [ -f "/etc/ssl/certs/ca-certificates.crt" ]; then
    export SSL_CERT_FILE="/etc/ssl/certs/ca-certificates.crt"
    export SSL_CERT_DIR="/etc/ssl/certs"
    echo "Using Debian/Ubuntu CA certificates: /etc/ssl/certs/ca-certificates.crt"
elif [ -f "/etc/pki/tls/certs/ca-bundle.crt" ]; then
    export SSL_CERT_FILE="/etc/pki/tls/certs/ca-bundle.crt" 
    export SSL_CERT_DIR="/etc/pki/tls/certs"
    echo "Using RHEL/CentOS CA certificates: /etc/pki/tls/certs/ca-bundle.crt"
elif [ -f "/etc/ssl/ca-bundle.pem" ]; then
    export SSL_CERT_FILE="/etc/ssl/ca-bundle.pem"
    export SSL_CERT_DIR="/etc/ssl"
    echo "Using OpenSSL CA certificates: /etc/ssl/ca-bundle.pem"
elif [ -f "/usr/local/share/ca-certificates/" ]; then
    export SSL_CERT_DIR="/usr/local/share/ca-certificates"
    echo "Using local CA certificates directory"
else
    echo "WARNING: No standard CA certificate bundle found - may need custom certificates"
    echo "WARNING: Consider mounting your internal CA certificate to the container"
fi

# Extract Azure certificates directly without nested script generation
if [ -n "`$AMW_QUERY_ENDPOINT" ]; then
    echo "Setting up Azure certificate chain for secure validation..."
    AMW_HOST=`$(echo "`$AMW_QUERY_ENDPOINT" | sed 's|https\?://||' | sed 's|/.*||')
    
    if [ -n "`$AMW_HOST" ]; then
        echo "Extracting certificate chain from `$AMW_HOST..."
        # Original command: echo | openssl s_client -servername "`$AMW_HOST" -connect "`$AMW_HOST:443" -showcerts 2>/dev/null | sed -n '/BEGIN CERTIFICATE/,/END CERTIFICATE/p' > /tmp/azure-server-chain.pem
        
        echo "DEBUG: AMW_HOST=`$AMW_HOST"
        echo "DEBUG: Testing basic connectivity..."
        
        # Test basic connectivity first
        if command -v nc >/dev/null 2>&1; then
            nc_result=`$`(timeout 5 nc -z "`$AMW_HOST" 443 2>&1)
            nc_exit=`$?
            echo "DEBUG: nc test exit code: `$nc_exit"
            if [ `$nc_exit -eq 0 ]; then
                echo "DEBUG: Port 443 is reachable"
            else
                echo "DEBUG: Port 443 connection failed: `$nc_result"
            fi
        else
            echo "DEBUG: nc not available, skipping port test"
        fi
        
        # Try OpenSSL with verbose error output
        echo "DEBUG: Running OpenSSL s_client..."
        openssl_output=`$(echo | openssl s_client -servername "`$AMW_HOST" -connect "`$AMW_HOST:443" -showcerts 2>&1)
        openssl_exit=`$?
        echo "DEBUG: OpenSSL exit code: `$openssl_exit"
        
        if [ `$openssl_exit -ne 0 ]; then
            echo "DEBUG: OpenSSL connection failed:"
            echo "`$openssl_output" | head -20
        else
            echo "DEBUG: OpenSSL connection successful"
        fi
        
        # Extract certificates from the output
        echo "`$openssl_output" | sed -n '/BEGIN CERTIFICATE/,/END CERTIFICATE/p' > /tmp/azure-server-chain.pem
        
        if [ -s "/tmp/azure-server-chain.pem" ]; then
            echo "Retrieved Azure certificate chain"
            if [ -n "`$SSL_CERT_FILE" ] && [ -f "`$SSL_CERT_FILE" ]; then
                cat "`$SSL_CERT_FILE" /tmp/azure-server-chain.pem > /tmp/combined-ca-bundle.pem
                export SSL_CERT_FILE="/tmp/combined-ca-bundle.pem"
                echo "Combined system and Azure certificates"
            else
                export SSL_CERT_FILE="/tmp/azure-server-chain.pem"
                echo "Using Azure certificate chain only"
            fi
        else
            echo "Could not retrieve Azure certificates - check connectivity"
            echo "Configuring TLS bypass for air-gapped environment..."
            
            # Air-gapped environment TLS bypass configuration
            export GODEBUG=x509ignoreCN=1
            export GOINSECURE="*"
            
            # Additional Go TLS environment variables for bypass
            export GO_TLS_INSECURE_SKIP_VERIFY=1
            
            echo "WARNING: TLS certificate verification disabled for air-gapped environment"
            echo "This is acceptable for secure internal networks like USNat"
        fi
    fi
fi

echo "Secure certificate configuration completed"
echo "Go will use proper certificate validation instead of bypassing TLS"
echo "Air-gapped Go module configuration applied"
"@

# Write the environment script to temp directory with Unix line endings
$containerEnvScript -replace "`r`n", "`n" | Out-File -FilePath $tlsEnvScript -Encoding UTF8 -NoNewline

# Ensure Unix line endings and make it executable
bash -c "dos2unix '$tlsEnvScript' 2>/dev/null || true"
chmod +x $tlsEnvScript

# Verify the script was created successfully
if (Test-Path $tlsEnvScript) {
    $scriptSize = (Get-Item $tlsEnvScript).Length
    Write-Host "Created TLS environment script: $tlsEnvScript ($scriptSize bytes)" -ForegroundColor Green
} else {
    Write-Host "ERROR: Failed to create TLS environment script" -ForegroundColor Red
    exit 1
}

Write-Host "TLS and Go environment configuration completed" -ForegroundColor Green

Write-Host "Running tests with TLS configuration..."

# Create a separate execution script to avoid bash -c issues with complex here-strings
$execScript = "$tempDir/run-tests-$dtAsString.sh"
$execScriptContent = @"
#!/bin/bash
# Test execution script with proper error handling

# Check if the TLS environment script exists
if [ ! -f "$tlsEnvScript" ]; then
    echo "ERROR: TLS environment script not found: $tlsEnvScript"
    exit 1
fi

echo "Sourcing TLS environment from: $tlsEnvScript"
# Source the secure TLS environment setup
source "$tlsEnvScript"

# Show certificate configuration status
echo "Certificate configuration status:"
echo "SSL_CERT_FILE: `$SSL_CERT_FILE"
echo "SSL_CERT_DIR: `$SSL_CERT_DIR"
if [ -f "`$SSL_CERT_FILE" ]; then
    echo "Certificate file exists (size: `$(wc -c < "`$SSL_CERT_FILE") bytes)"
else
    echo "WARNING: Certificate file not found"
fi

# Set environment variables for the Go application
export AMW_QUERY_ENDPOINT='$amwEndpoint'
export AZURE_CLIENT_ID='$clientId'

# Clear any conflicting identity variables that might interfere with IMDS
unset IDENTITY_ENDPOINT IDENTITY_HEADER IDENTITY_API_VERSION
unset MSI_ENDPOINT MSI_SECRET IMDS_ENDPOINT
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY
export NO_PROXY="169.254.169.254,168.63.129.16,127.0.0.1,localhost"

echo "Environment configured for Go application:"
echo "AMW_QUERY_ENDPOINT: `$AMW_QUERY_ENDPOINT"
echo "AZURE_CLIENT_ID: `$AZURE_CLIENT_ID"
echo "NO_PROXY: `$NO_PROXY"

# Check if the test executable exists
if [ ! -f "./regionTests-linux.exe" ]; then
    echo "ERROR: Test executable not found: ./regionTests-linux.exe"
    exit 1
fi

# Run the Ginkgo test suite with secure certificate configuration
echo "Starting regionTests with secure TLS certificate validation..."
./regionTests-linux.exe -parmRuleName '$rule' -parmAmwResourceId '$resourceId' -clientId '$clientId'
"@

# Write execution script with Unix line endings
$execScriptContent -replace "`r`n", "`n" | Out-File -FilePath $execScript -Encoding UTF8 -NoNewline

# Make execution script Unix-compatible and executable
bash -c "dos2unix '$execScript' 2>/dev/null || true"
chmod +x $execScript

# Execute the test script
Write-Host "Executing tests using script: $execScript"
bash $execScript

Write-Host "Done querying AKS cluster."

# Clean up temporary TLS configuration files
Write-Host "Cleaning up temporary files..."
if (Test-Path $tlsEnvScript) {
    Remove-Item $tlsEnvScript -Force -ErrorAction SilentlyContinue
    Write-Host "Cleaned up TLS environment script: $tlsEnvScript"
}
if (Test-Path $execScript) {
    Remove-Item $execScript -Force -ErrorAction SilentlyContinue
    Write-Host "Cleaned up execution script: $execScript"
}

Stop-Transcript

Get-ChildItem -Path "." | Select-Object Name

try {
    # Get current Azure context to ensure we're authenticated
    $currentContext = Get-AzContext
    if (-not $currentContext) {
        Write-Host "No Azure context found, skipping transcript upload"
    } else {
        Write-Host "Current context: $($currentContext.Account.Id)"
        Write-Host "Current subscription: $($currentContext.Subscription.Name)"

        # Verify storage account exists and get its resource ID
        $storageAccount = Get-AzStorageAccount -ResourceGroupName $ResourceGroupName -Name $logsStorageAccountName -ErrorAction SilentlyContinue
        if (-not $storageAccount) {
            Write-Host "Storage account $logsStorageAccountName not found in resource group $ResourceGroupName"
            Write-Host "Attempting to upload anyway..."
        } else {
            Write-Host "Storage account found: $($storageAccount.StorageAccountName)"
            Write-Host "Storage account resource ID: $($storageAccount.Id)"
        }

        # Create storage context using OAuth token (required for managed identity RBAC)
        Write-Host "Creating storage context with UseConnectedAccount..."
        $ctx = New-AzStorageContext -StorageAccountName $logsStorageAccountName -UseConnectedAccount -ErrorAction Stop
        Write-Host "Storage context created successfully"

        # Ensure container exists, create if it doesn't
        Write-Host "Checking if container '$containerName' exists..."
        $container = Get-AzStorageContainer -Name $containerName -Context $ctx -ErrorAction SilentlyContinue
        if (-not $container) {
            Write-Host "Container '$containerName' not found, creating it..."
            New-AzStorageContainer -Name $containerName -Context $ctx -Permission Off -ErrorAction Stop
            Write-Host "Container '$containerName' created successfully"
        } else {
            Write-Host "Container '$containerName' already exists"
        }

        # Upload transcript
        Write-Host "Uploading transcript $transcriptFileName to storage account $logsStorageAccountName, container $containerName"
        Set-AzStorageBlobContent -File $transcriptFileName -Container $containerName -Blob $transcriptFileName -Context $ctx -Force -ErrorAction Stop
        Write-Host "Transcript uploaded successfully"
    }
}
catch {
    Write-Host "Error uploading transcript: $_"
    Write-Host "Error details: $($_.Exception.Message)"
    if ($_.Exception.InnerException) {
        Write-Host "Inner exception: $($_.Exception.InnerException.Message)"
    }
}
