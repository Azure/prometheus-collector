@REM REM "Checking if fluent-bit is running"

tasklist /fi "imagename eq fluent-bit.exe" /fo "table"  | findstr fluent-bit

IF ERRORLEVEL 1 (
    echo "Fluent-Bit is not running"
    exit /b 1
)

@REM "Checking if config map has been updated since agent start"

IF EXIST C:\opt\microsoft\scripts\filesystemwatcher.txt (
    echo "Config Map Updated since agent started"
    exit /b  1
)

@REM REM "Checking if fluentd service is running"
@REM sc query fluentdwinaks | findstr /i STATE | findstr RUNNING

@REM IF ERRORLEVEL 1 (
@REM     echo "Fluentd Service is NOT Running"
@REM     exit /b  1
@REM )


exit /b 0
