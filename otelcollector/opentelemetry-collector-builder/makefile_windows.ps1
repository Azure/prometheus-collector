# Temporary method to build the OTL collector and fluent-bit for windows

# building otelcollector
Write-Output "building otelcollector"
if (Test-Path "otelcollector.exe") {
    Remove-Item .\otelcollector.exe
}
go get
go build -o otelcollector.exe .

Write-Output "FINISHED building otelcollector"

# building fluent-bit plugin

Write-Output "building fluent-bit plugin"

Set-Location ..
Set-Location fluent-bit
Set-Location src

.\makefile_windows.ps1

Set-Location ..
Set-Location ..
Set-Location opentelemetry-collector-builder

Write-Output "FINISHED building fluent-bit plugin"

Write-Output "building promconfigvalidator"

Set-Location ..
Set-Location prom-config-validator-builder

.\makefile_windows.ps1

Set-Location ..
Set-Location opentelemetry-collector-builder

Write-Output "FINISHED building promconfigvalidator"

Set-Location ..
Set-Location main

# Create directories
New-Item -Path "./shared/configmap/mp/" -ItemType Directory -Force
New-Item -Path "./shared/configmap/ccp/" -ItemType Directory -Force
# New-Item -Path "./main/" -ItemType Directory -Force

# Copy shared Go files
Copy-Item -Path "../shared/*.go" -Destination "./shared/"
Copy-Item -Path "../shared/go.mod" -Destination "./shared/"
Copy-Item -Path "../shared/go.sum" -Destination "./shared/"
Copy-Item -Path "../shared/configmap/mp/*.go" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/mp/go.mod" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/mp/go.sum" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/ccp/*.go" -Destination "./shared/configmap/ccp/"
Copy-Item -Path "../shared/configmap/ccp/go.mod" -Destination "./shared/configmap/ccp/"
Copy-Item -Path "../shared/configmap/ccp/go.sum" -Destination "./shared/configmap/ccp/"

# # Copy main Go files
# Copy-Item -Path "./main/*.go" -Destination "./main/"
# Copy-Item -Path "./go.mod" -Destination "./main/"
# Copy-Item -Path "./go.sum" -Destination "./main/"
# Set-Location main

go version
go mod tidy
go build -buildmode=pie -o "main.exe" "./main.go"

Write-Output "Build main executable completed"

Set-Location ..
Set-Location opentelemetry-collector-builder

