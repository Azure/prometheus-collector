Start-Transcript -Path fileSystemWatcherTranscript.txt
Write-Host "Removing Existing Event Subscribers"
Get-EventSubscriber -Force | ForEach-Object { $_.SubscriptionId } | ForEach-Object { Unregister-Event -SubscriptionId $_ } > $null
Write-Host "Starting File System Watcher for config map updates and DCR/DCE updates"

$Paths = @("C:\etc\config\settings", "C:\etc\config\settings\prometheus")

if ($env:MAC -eq $true) {
    $Paths = @("C:\etc\config\settings", "C:\etc\config\settings\prometheus") #, "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension")
}

foreach ($path in $Paths)
{
    $FileSystemWatcher = New-Object System.IO.FileSystemWatcher
    $FileSystemWatcher.Path = $path
    # if ($path -eq "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension") {
    #     $FileSystemWatcher.Filter = "TokenConfig.json"
    # }
    $FileSystemWatcher.IncludeSubdirectories = $true
    $EventName = 'Changed', 'Created', 'Deleted', 'Renamed'

    $Action = {
        $fileSystemWatcherStatusPath = "C:\opt\microsoft\scripts\filesystemwatcher.txt"
        $fileSystemWatcherLog = "{0} was  {1} at {2}" -f $Event.SourceEventArgs.FullPath,
        $Event.SourceEventArgs.ChangeType,
        $Event.TimeGenerated
        Write-Host $fileSystemWatcherLog
        Add-Content -Path $fileSystemWatcherStatusPath -Value $fileSystemWatcherLog
    }

    $ObjectEventParams = @{
        InputObject = $FileSystemWatcher
        Action      = $Action
    }

    ForEach ($Item in $EventName) {
        $ObjectEventParams.EventName = $Item
        $name = Split-Path -Path $path -Leaf
        $ObjectEventParams.SourceIdentifier = "$($name).$($Item)"
        $Null = Register-ObjectEvent  @ObjectEventParams
    }
}

# keep this running for the container's lifetime, so that it can listen for changes to the config map mount path
try
{
    do
    {
        Wait-Event -Timeout 60
    } while ($true)
}
finally
{
    Get-EventSubscriber -Force | ForEach-Object { $_.SubscriptionId } | ForEach-Object { Unregister-Event -SubscriptionId $_ }
    Write-Host "Event Handler disabled."
}