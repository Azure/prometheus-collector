@REM "Checking if fluent-bit is running"

tasklist /fi "imagename eq td-agent-bit.exe" /fo "table"  | findstr td-agent-bit

IF ERRORLEVEL 1 (
    echo "Fluent-Bit is not running"
    exit /b 1
)

@REM "Checking if config map has been updated since agent start"

IF EXIST C:\opt\microsoft\scripts\filesystemwatcher.txt (
    echo "Config Map Updated since agent started"
    exit /b  1
)

@REM REM "Checking if Telegraf is running"

tasklist /fi "imagename eq telegraf.exe" /fo "table"  | findstr telegraf

IF ERRORLEVEL 1 (
    echo "Telegraf is not running"
    exit /b 1
)

@REM REM "Checking if MA is running"

tasklist /fi "imagename eq MonAgentLauncher.exe" /fo "table"  | findstr MonAgentLauncher

IF ERRORLEVEL 1 (
    echo "MonAgentLauncher.exe is not running"
    exit /b 1
)


@REM REM "Checking if MetricsExtension is running"

tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension

IF ERRORLEVEL 1 (
    echo "MetricsExtension is not running"
    exit /b 1
)

@REM REM "Checking if otelcollector is running"

tasklist /fi "imagename eq otelcollector.exe" /fo "table"  | findstr otelcollector

IF ERRORLEVEL 1 (
    echo "otelcollector is not running"
    exit /b 1
)

exit /b 0
