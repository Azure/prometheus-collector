param (
    [string]$PROMETHEUS_VERSION = ""
)

Write-Output "========================= cleanup existing prometheusui ========================="
if (Test-Path "prometheusui.exe") {
    Remove-Item prometheusui.exe
}
Write-Output "========================= Building prometheusui ========================="
Write-Output "Using Prometheus version: $PROMETHEUS_VERSION"
Invoke-WebRequest -Uri "https://github.com/prometheus/prometheus/releases/download/v$PROMETHEUS_VERSION/prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" -OutFile "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"
# Verify the downloaded file exists
Write-Output "Verifying downloaded file..."
if (Test-Path "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz") {
    Get-Item "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" | Format-List Name, Length, LastWriteTime
} else {
    Write-Error "Failed to download prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"
    exit 1
}
# Extract tar.gz using 7-Zip (make sure 7-Zip is installed)
& 7z x "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz" -so | & 7z x -si "-ttar" -o ".\prometheus-web-ui-$PROMETHEUS_VERSION"

# List extracted files/directories to determine extraction path
Write-Output "Listing files in working directory after extraction:"
Get-ChildItem -Path . | Select-Object Name, LastWriteTime, @{Name="Type";Expression={if($_.PSIsContainer){"Directory"}else{"File"}}}

# Verify extraction was successful
Write-Output "Verifying extraction..."
$uiFolderName = "prometheus-web-ui-$PROMETHEUS_VERSION"
if (Test-Path $uiFolderName) {
    Write-Output "Extraction successful. UI folder found: $uiFolderName"
} else {
    Write-Error "Extraction failed. UI folder not found: $uiFolderName"
    exit 1
}

# Move the static folder from the extracted package to the current directory
Write-Output "Moving static folder to current directory..."
$staticSourcePath = Join-Path -Path $uiFolderName -ChildPath "static"
if (Test-Path $staticSourcePath) {
    # Remove existing static folder if it exists
    if (Test-Path "static") {
        Write-Output "Removing existing static folder..."
        Remove-Item -Path "static" -Recurse -Force
    }
    # Copy the static folder to current directory
    Write-Output "Copying static folder from $staticSourcePath..."
    Copy-Item -Path $staticSourcePath -Destination "static" -Recurse
    Write-Output "Static folder successfully moved to current directory."
} else {
    Write-Error "Static folder not found in the extracted package: $staticSourcePath"
    exit 1
}

# Clean up the extracted directory
Write-Output "Removing extracted package directory..."
Remove-Item -Path $uiFolderName -Recurse -Force

# Clean up the downloaded tar.gz file
Write-Output "Cleaning up downloaded archive..."
Remove-Item "prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz"
Write-Output "========================= go get  ========================="
go get
Write-Output "========================= go build  ========================="
go build -o prometheusui.exe .
