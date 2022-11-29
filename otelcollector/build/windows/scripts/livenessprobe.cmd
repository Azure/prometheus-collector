@REM REM "Checking if fluent-bit is running"

@REM tasklist /fi "imagename eq td-agent-bit.exe" /fo "table"  | findstr td-agent-bit

@REM IF ERRORLEVEL 1 (
@REM     echo "Fluent-Bit is not running"
@REM     exit /b 1
@REM )

@REM @REM "Checking if config map has been updated since agent start"

@REM IF EXIST C:\opt\microsoft\scripts\filesystemwatcher.txt (
@REM     echo "Config Map Updated since agent started"
@REM     exit /b  1
@REM )

@REM @REM REM "Checking if Telegraf is running"

@REM tasklist /fi "imagename eq telegraf.exe" /fo "table"  | findstr telegraf

@REM IF ERRORLEVEL 1 (
@REM     echo "Telegraf is not running"
@REM     exit /b 1
@REM )


@REM @REM REM "Checking if MetricsExtension is running"

@REM tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension

@REM IF ERRORLEVEL 1 (
@REM     echo "MetricsExtension is not running"
@REM     exit /b 1
@REM )

@REM @REM REM "Checking if otelcollector is running"

@REM tasklist /fi "imagename eq otelcollector.exe" /fo "table"  | findstr otelcollector

@REM IF ERRORLEVEL 1 (
@REM     echo "otelcollector is not running"
@REM     exit /b 1
@REM )

exit /b 0
