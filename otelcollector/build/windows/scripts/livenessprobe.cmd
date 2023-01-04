@echo off

setlocal enableDelayedExpansion
rem Get the current date and time
for /f "tokens=2 delims==" %%a in ('wmic os get LocalDateTime /VALUE') do set "dt=%%a"
rem Convert the current date and time to epoch time
set /a "epochTimeNow=((%dt:~0,4% - 1970) * 31536000 + (%dt:~4,2% - 1) * 2592000 + (%dt:~6,2% - 1) * 86400 + %dt:~8,2% * 3600 + %dt:~10,2% * 60 + %dt:~12,2%)"

if "%MAC%"=="true" (
    if not exist "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension\TokenConfig.json" (
        if exist "C:\opt\microsoft\liveness\azmon-container-start-time" (
            set /p azmonContainerStartTime=<C:\opt\microsoft\liveness\azmon-container-start-time
            set /a duration=%epochTimeNow%-%azmonContainerStartTime%
            set /a durationInMinutes=%duration% / 60
            if %durationInMinutes%==0 (
                echo %epochTimeNow% "No configuration present for the AKS resource" > C:\dev\write-to-traces
            )
            if %durationInMinutes% GTR 15 (
                echo "(Greater than 15 mins) No configuration present for the AKS resource" > C:\dev\termination-log
                exit /b 1
            )
        )
    ) else (
        tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension > nul
        if errorlevel 1 (
            echo "Metrics Extension is not running (configuration exists)" > C:\dev\termination-log
            exit /b 1
        )
        tasklist /fi "imagename eq MonAgentLauncher.exe" /fo "table"  | findstr MonAgentLauncher > nul
        if errorlevel 1 (
            echo "MonAgentLauncher is not running (configuration exists)" > C:\dev\termination-log
            exit /b 1
        )
    )

    if exist "C:\opt\microsoft\scripts\filesystemwatcher.txt" (
        echo "Config Map Updated or DCR/DCE updated since agent started" > C:\dev\termination-log
        exit /b  1
    )
) else (
    rem Non-MAC mode
    tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension > nul
    if errorlevel 1 (
        echo "Metrics Extension is not running (configuration DOES NOT exist)" > C:\dev\termination-log
        exit /b 1
    )

    tasklist /fi "imagename eq MonAgentLauncher.exe" /fo "table"  | findstr MonAgentLauncher > nul
    if errorlevel 1 (
        echo "MonAgentLauncher is not running (configuration DOES NOT exist)" > C:\dev\termination-log
        exit /b 1
    )
)

@REM "Checking if fluent-bit is running"
tasklist /fi "imagename eq td-agent-bit.exe" /fo "table"  | findstr td-agent-bit
if errorlevel 1 (
    echo "Fluent-Bit is not running"
    exit /b 1
)

@REM "Checking if config map has been updated since agent start"
if exist "C:\opt\microsoft\scripts\filesystemwatcher.txt" (
    echo "Config Map Updated or DCR/DCE updated since agent started"
    exit /b  1
)

@REM REM "Checking if Telegraf is running"
tasklist /fi "imagename eq telegraf.exe" /fo "table"  | findstr telegraf
if errorlevel 1 (
    echo "Telegraf is not running"
    exit /b 1
)

@REM REM "Checking if otelcollector is running"
tasklist /fi "imagename eq otelcollector.exe" /fo "table"  | findstr otelcollector
if errorlevel 1 (
    echo "otelcollector is not running"
    exit /b 1
)

exit /b 0
