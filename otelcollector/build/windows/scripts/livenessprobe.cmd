@REM REM "Checking if fluent-bit is running"

@REM tasklist /fi "imagename eq fluent-bit.exe" /fo "table"  | findstr fluent-bit

@REM IF ERRORLEVEL 1 (
@REM     echo "Fluent-Bit is not running"
@REM     exit /b 1
@REM )

@REM REM "Checking if config map has been updated since agent start"

@REM IF EXIST C:\opt\microsoft\scripts\filesystemwatcher.txt (
@REM     echo "Config Map Updated since agent started"
@REM     exit /b  1
@REM )

@REM REM "Checking if fluentd service is running"
@REM sc query fluentdwinaks | findstr /i STATE | findstr RUNNING

@REM IF ERRORLEVEL 1 (
@REM     echo "Fluentd Service is NOT Running"
@REM     exit /b  1
@REM )


exit /b 0
