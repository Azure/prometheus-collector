Start-Transcript -Path fileSystemWatcherTranscript.txt
Write-Host "Removing Existing Event Subscribers"
Get-EventSubscriber -Force | ForEach-Object { $_.SubscriptionId } | ForEach-Object { Unregister-Event -SubscriptionId $_.SubscriptionId } > $null
Write-Host "Starting File System Watcher for config map updates and DCR/DCE updates"

$Paths = @("C:\etc\config\settings", "C:\etc\config\settings\prometheus")

if ($env:MAC -ieq "true") {
    Write-Host "MAC environment variable is true, adding metrics extension path"
    $Paths += "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension"
}

Write-Host "Watching the following paths:"
$Paths | ForEach-Object { Write-Host "`t$_" }

foreach ($path in $Paths)
{
    # Check if path exists, skip it if it doesn't
    if (-not (Test-Path $path)) {
        Write-Host "Path does not exist, skipping: $path"
        continue
    }

    Write-Host "Setting up watcher for path: $path"

    $FileSystemWatcher = New-Object System.IO.FileSystemWatcher
    $FileSystemWatcher.Path = $path

    # Special handling for metricsextension path
    if ($path -eq "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension") {
        $FileSystemWatcher.Filter = "TokenConfig.json"
        Write-Host "Applied filter: TokenConfig.json"
    }

    $FileSystemWatcher.IncludeSubdirectories = $true
    $EventName = 'Changed', 'Created', 'Deleted', 'Renamed'

    $Action = {
        $fileSystemWatcherStatusPath = "C:\opt\microsoft\scripts\filesystemwatcher.txt"
        $fileSystemWatcherLog = "{0} was {1} at {2}" -f $Event.SourceEventArgs.FullPath,
                                                       $Event.SourceEventArgs.ChangeType,
                                                       $Event.TimeGenerated

        Write-Host "Change detected: $fileSystemWatcherLog"

        $dir = Split-Path $fileSystemWatcherStatusPath
        if (-not (Test-Path $dir)) {
            Write-Host "Creating directory: $dir"
            New-Item -Path $dir -ItemType Directory -Force
        }

        Add-Content -Path $fileSystemWatcherStatusPath -Value $fileSystemWatcherLog
        Write-Host "Log written to $fileSystemWatcherStatusPath"
    }

    $ObjectEventParams = @{
        InputObject = $FileSystemWatcher
        Action      = $Action
    }

    foreach ($Item in $EventName) {
        $ObjectEventParams.EventName = $Item
        $name = Split-Path -Path $path -Leaf
        $ObjectEventParams.SourceIdentifier = "$name.$Item"
        Write-Host "Registering event: $($ObjectEventParams.SourceIdentifier)"
        $null = Register-ObjectEvent @ObjectEventParams
    }
}

# Dynamic watcher for metricsextension if it gets created later
$metricsextensionPath = "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension"
if (-not (Test-Path $metricsextensionPath)) {
    Write-Host "Watching for the creation of the metricsextension directory..."
    
    $parentPath = "C:\opt\genevamonitoringagent\datadirectory\mcs"
    $FileSystemWatcherParent = New-Object System.IO.FileSystemWatcher
    $FileSystemWatcherParent.Path = $parentPath
    $FileSystemWatcherParent.Filter = "metricsextension"
    $FileSystemWatcherParent.IncludeSubdirectories = $false
    $FileSystemWatcherParent.EnableRaisingEvents = $true

    $ActionParent = {
        if (Test-Path $metricsextensionPath) {
            Write-Host "Detected the creation of metricsextension, adding watcher."
            
            # Now add the watcher for TokenConfig.json
            $FileSystemWatcher = New-Object System.IO.FileSystemWatcher
            $FileSystemWatcher.Path = $metricsextensionPath
            $FileSystemWatcher.Filter = "TokenConfig.json"
            $FileSystemWatcher.IncludeSubdirectories = $true
            $FileSystemWatcher.EnableRaisingEvents = $true

            # Register event handlers
            foreach ($Item in $EventName) {
                $ObjectEventParams.EventName = $Item
                $ObjectEventParams.SourceIdentifier = "$metricsextensionPath.$Item"
                Write-Host "Registering event: $($ObjectEventParams.SourceIdentifier)"
                $null = Register-ObjectEvent @ObjectEventParams
            }
        }
    }

    $ObjectEventParamsParent = @{
        InputObject = $FileSystemWatcherParent
        Action      = $ActionParent
    }

    # Register event to monitor when 'metricsextension' folder is created
    Register-ObjectEvent @ObjectEventParamsParent
}

try {
    Write-Host "Entering event loop. Waiting for file system changes..."
    do {
        Wait-Event -Timeout 60
    } while ($true)
}
finally {
    Write-Host "Cleaning up event subscribers..."
    Get-EventSubscriber -Force | ForEach-Object { $_.SubscriptionId } | ForEach-Object { Unregister-Event -SubscriptionId $_.SubscriptionId }
    Write-Host "Event Handler disabled."
}