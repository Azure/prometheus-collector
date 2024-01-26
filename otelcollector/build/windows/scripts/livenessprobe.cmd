@echo off
rem Get the current date and time
setlocal enableextensions
setlocal enabledelayedexpansion
for /f %%x in ('wmic path win32_utctime get /format:list ^| findstr "="') do ( set %%x )
set /a z=(14-100%Month%%%100)/12, y=10000%Year%%%10000-z
set /a ut=y*365+y/4-y/100+y/400+(153*(100%Month%%%100+12*z-3)+2)/5+Day-719469
set /a epochTimeNow=%ut%*86400 + 100%Hour%%%100*3600 + 100%Minute%%%100*60 + 100%Second%%%100

set /a durationInMinutes = -1

REM Run tasklist once and capture the output
set "MetricsExtension=false"
set "MonAgentLauncher=false"
set "otelcollector=false"

for /f "tokens=*" %%a in ('tasklist /fo "table"') do (
    set "output=%%a"

    REM Check for MetricsExtension.Native.exe
    echo !output! | findstr /i "MetricsExtension" > nul
    if !errorlevel! equ 0 set MetricsExtension=true

    REM Check for MonAgentLauncher.exe
    echo !output! | findstr /i "MonAgentLauncher" > nul
    if !errorlevel! equ 0 set MonAgentLauncher=true

    REM Check for otelcollector.exe
    echo !output! | findstr /i "otelcollector" > nul
    if !errorlevel! equ 0 set otelcollector=true
)

if "%MAC%" == "" (
    rem Non-MAC mode
    if %MetricsExtension%==false (
        echo "Metrics Extension is not running (Non-MAC mode)"
        goto eof
    )
) else (
    if "%MAC%" == "true" (
        @rem Checking if TokenConfig file exists, if it doesn't, it means that there is no DCR/DCE config for this resource and ME/MDSD will fail to start
        @rem avoid the pods from going into crashloopbackoff, we are restarting the pod with this message every 15 minutes.
        if not exist "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension\TokenConfig.json" (
            if exist "C:\opt\microsoft\liveness\azmon-container-start-time" (
                for /f "delims=" %%a in (C:\opt\microsoft\liveness\azmon-container-start-time) do (
                        set firstline=%%a
                        set /a azmonContainerStartTime=!firstline!
                )
                set /a duration=%epochTimeNow%-!azmonContainerStartTime!
                set /a durationInMinutes=!duration! / 60
                if !durationInMinutes! == 0 (
                    echo %epochTimeNow% "No configuration present for the AKS resource"
                )
                if !durationInMinutes! GTR 15 (
                    echo "Greater than 15 mins, No configuration present for the AKS resource"
                    goto eof
                )
            )
        ) else (
            if %MetricsExtension%==false (
                echo "Metrics Extension is not running (configuration exists)"
                goto eof
            )
            if %MonAgentLauncher%==false (
                echo "MonAgentLauncher is not running (configuration exists)"
                goto eof
            )
        )
    )
)

@REM "Checking if config map has been updated since agent start"
if exist "C:\opt\microsoft\scripts\filesystemwatcher.txt" (
    echo "Config Map Updated or DCR/DCE updated since agent started"
    goto eof
)

@REM "Checking if otelcollector is running"
if %otelcollector%==false (
    echo "otelcollector is not running"
    goto eof
)

endlocal

exit /B 0

:eof
exit /B 1
