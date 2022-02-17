#setting it to replicaset by default
$me_config_file = '/opt/metricextension/me_ds.config'

function Set-EnvironmentVariablesAndConfigParser {

    if ([string]::IsNullOrEmpty($env:MODE)) {
        [System.Environment]::SetEnvironmentVariable("MODE", 'simple', "Process")
        [System.Environment]::SetEnvironmentVariable("MODE", 'simple', "Machine")
    }

    #resourceid override.
    if ([string]::IsNullOrEmpty($env:AKS_RESOURCE_ID)) {
        Write-Output "AKS_RESOURCE_ID is empty or not set."
        if ([string]::IsNullOrEmpty($env:CLUSTER)) {
            Write-Output "CLUSTER is empty or not set. Using $env:NODE_NAME as CLUSTER"
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:NODE_NAME, "Process")
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:NODE_NAME, "Machine")
            Write-Output "customResourceId:$env:customResourceId"
        }
        else {
            Write-Output "Using CLUSTER as $env:CLUSTER"
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Process")
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Machine")
            Write-Output "customResourceId:$env:customResourceId"
        }
    }
    else {
        Write-Output "AKS_RESOURCE_ID is set already so setting customResourceId=$env:AKS_RESOURCE_ID"
        [System.Environment]::SetEnvironmentVariable("customResourceId", $env:AKS_RESOURCE_ID, "Process")
        [System.Environment]::SetEnvironmentVariable("customResourceId", $env:AKS_RESOURCE_ID, "Machine")
        Write-Output "customResourceId:$customResourceId"
    }

    #set agent config schema version
    if (Test-Path -Path '/etc/config/settings/schema-version') {
        #trim
        $config_schema_version = Get-Content -Path /etc/config/settings/schema-version
        #remove all spaces
        $config_schema_version = $config_schema_version.trim()
        #take first 10 characters
        if ($config_schema_version.Length -gt 10) {
            $config_schema_version = $config_schema_version.SubString(0, 10)
        }
        [System.Environment]::SetEnvironmentVariable("AZMON_AGENT_CFG_SCHEMA_VERSION", $config_schema_version, "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_AGENT_CFG_SCHEMA_VERSION", $config_schema_version, "Machine")
    }

    #set agent config file version
    if (Test-Path -Path '/etc/config/settings/config-version') {
        #trim
        $config_file_version = Get-Content -Path /etc/config/settings/config-version
        #remove all spaces
        $config_file_version = $config_file_version.Trim()
        #take first 10 characters
        if ($config_file_version.Length -gt 10) {
            $config_file_version = $config_file_version.Substring(0, 10)
        }
        [System.Environment]::SetEnvironmentVariable("AZMON_AGENT_CFG_FILE_VERSION", $config_file_version, "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_AGENT_CFG_FILE_VERSION", $config_file_version, "Machine")
    }

    # Need to do this before the SA fetch for AI key for airgapped clouds so that it is not overwritten with defaults.
    $appInsightsAuth = [System.Environment]::GetEnvironmentVariable("APPLICATIONINSIGHTS_AUTH", "process")
    if (![string]::IsNullOrEmpty($appInsightsAuth)) {
        [System.Environment]::SetEnvironmentVariable("APPLICATIONINSIGHTS_AUTH", $appInsightsAuth, "machine")
        Write-Host "Successfully set environment variable APPLICATIONINSIGHTS_AUTH - $($appInsightsAuth) for target 'machine'..."
    }
    else {
        Write-Host "Failed to set environment variable APPLICATIONINSIGHTS_AUTH for target 'machine' since it is either null or empty"
    }

    $aiKeyDecoded = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String($env:APPLICATIONINSIGHTS_AUTH))
    [System.Environment]::SetEnvironmentVariable("TELEMETRY_APPLICATIONINSIGHTS_KEY", $aiKeyDecoded, "Process")
    [System.Environment]::SetEnvironmentVariable("TELEMETRY_APPLICATIONINSIGHTS_KEY", $aiKeyDecoded, "Machine")

    # Kaveesh TODO : airgapped cloud app insights key
    # # Check if the instrumentation key needs to be fetched from a storage account (as in airgapped clouds)
    # if [ ${#APPLICATIONINSIGHTS_AUTH_URL} -ge 1 ]; then  # (check if APPLICATIONINSIGHTS_AUTH_URL has length >=1)
    #       for BACKOFF in {1..4}; do
    #             KEY=$(curl -sS $APPLICATIONINSIGHTS_AUTH_URL )
    #             # there's no easy way to get the HTTP status code from curl, so just check if the result is well formatted
    #             if [[ $KEY =~ ^[A-Za-z0-9=]+$ ]]; then
    #                   break
    #             else
    #                   sleep $((2**$BACKOFF / 4))  # (exponential backoff)
    #             fi
    #       done

    #       # validate that the retrieved data is an instrumentation key
    #       if [[ $KEY =~ ^[A-Za-z0-9=]+$ ]]; then
    #             export APPLICATIONINSIGHTS_AUTH=$(echo $KEY)
    #             echo "export APPLICATIONINSIGHTS_AUTH=$APPLICATIONINSIGHTS_AUTH" >> ~/.bashrc
    #             echo "Using cloud-specific instrumentation key"
    #       else
    #             # no ikey can be retrieved. Disable telemetry and continue
    #             export DISABLE_TELEMETRY=true
    #             echo "export DISABLE_TELEMETRY=true" >> ~/.bashrc
    #             echo "Could not get cloud-specific instrumentation key (network error?). Disabling telemetry"
    #       fi
    # fi

    
    # run config parser
    ruby /opt/microsoft/configmapparser/tomlparser-prometheus-collector-settings.rb

    if (Test-Path -Path '/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var') {
        foreach ($line in Get-Content /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var) {
            if ($line.Contains('=')) {
                $key = ($line -split '=')[0];
                $value = ($line -split '=')[1];
                [System.Environment]::SetEnvironmentVariable($key, $value, "Process")
                [System.Environment]::SetEnvironmentVariable($key, $value, "Machine")
            }
        }
    }

    # Parse the settings for default scrape configs
    ruby /opt/microsoft/configmapparser/tomlparser-default-scrape-settings.rb
    if (Test-Path -Path '/opt/microsoft/configmapparser/config_default_scrape_settings_env_var') {
        foreach ($line in Get-Content /opt/microsoft/configmapparser/config_default_scrape_settings_env_var) {
            if ($line.Contains('=')) {
                $key = ($line -split '=')[0];
                $value = ($line -split '=')[1];
                [System.Environment]::SetEnvironmentVariable($key, $value, "Process")
                [System.Environment]::SetEnvironmentVariable($key, $value, "Machine")
            }
        }
    }

    # Parse the settings for default targets metrics keep list config
    ruby /opt/microsoft/configmapparser/tomlparser-default-targets-metrics-keep-list.rb

    # Merge default anf custom prometheus config
    ruby /opt/microsoft/configmapparser/prometheus-config-merger.rb

    if (Test-Path -Path '/opt/promMergedConfig.yml') {
        C:\opt\promconfigvalidator --config "/opt/promMergedConfig.yml" --output "/opt/microsoft/otelcollector/collector-config.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
        if ( (!($?)) -or (!(Test-Path -Path "/opt/microsoft/otelcollector/collector-config.yml" ))) {
            Write-Output "Prometheus custom config validation failed, using defaults"
            # This env variable is used to indicate that the prometheus custom config was invalid and we fall back to defaults, used for telemetry
            [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", "Process")
            [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", "Machine")
            if (Test-Path -Path '/opt/defaultsMergedConfig.yml') {
                C:\opt\promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
                if ( (!($?)) -or (!(Test-Path -Path "/opt/collector-config-with-defaults.yml" ))) {
                    Write-Output "Prometheus default config validation failed, using empty job as collector config"
                }
                else {
                    Copy-Item "/opt/collector-config-with-defaults.yml" "/opt/microsoft/otelcollector/collector-config-default.yml"
                }
            }
            [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Process")
            [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Machine")
        }
    }
    elseif (Test-Path -Path '/opt/defaultsMergedConfig.yml') {
        Write-Output "No custom config found, using defaults"
        C:\opt\promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
        if ( (!($?)) -or (!(Test-Path -Path "/opt/collector-config-with-defaults.yml" ))) {
            Write-Output "Prometheus default config validation failed, using empty job as collector config"
        }
        else {
            Write-Output "Prometheus default config validation succeeded, using this as collector config"
            Copy-Item "/opt/collector-config-with-defaults.yml" "/opt/microsoft/otelcollector/collector-config-default.yml"
        }
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Machine")
    }
    else {
        # This else block is needed, when there is no custom config mounted as config map or default configs enabled
        Write-Output "No custom config found, using defaults"
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Machine")
    }

    # #start cron daemon for logrotate
    # service cron restart

    #start otelcollector
    Write-Output "Use default prometheus config: $env:AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"

    #get controller kind in lowercase, trimmed
    $controllerType = $env:CONTROLLER_TYPE
    $controllerType = $controllerType.Trim()
    if ($controllerType -eq "replicaset") {
        $meConfigFile = "/opt/metricextension/me.config"
    }
    else {
        $meConfigFile = "/opt/metricextension/me_ds.config"
    }
    [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $meConfigFile, "Process")
    [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $meConfigFile, "Machine")


    # Set ME Config file
    if (![string]::IsNullOrEmpty($env:CONTROLLER_TYPE)) {
        Write-Output "Setting the environment variable ME_CONFIG_FILE"
        [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $me_config_file, "Process")
        [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $me_config_file, "Machine")
    }

    # Set variables for telegraf (runs in machine environment)
    [System.Environment]::SetEnvironmentVariable("AGENT_VERSION", $env:AGENT_VERSION, "Machine")
    [System.Environment]::SetEnvironmentVariable("customResourceId", $env:customResourceId, "Machine")
    [System.Environment]::SetEnvironmentVariable("NODE_NAME", $env:NODE_NAME, "Machine")
    [System.Environment]::SetEnvironmentVariable("NODE_IP", $env:NODE_IP, "Machine")
    [System.Environment]::SetEnvironmentVariable("MODE", $env:MODE, "Machine")
    [System.Environment]::SetEnvironmentVariable("CONTROLLER_TYPE", $env:CONTROLLER_TYPE, "Machine")
    [System.Environment]::SetEnvironmentVariable("POD_NAMESPACE", $env:POD_NAMESPACE, "Machine")
    [System.Environment]::SetEnvironmentVariable("POD_NAME", $env:POD_NAME, "Machine")
    [System.Environment]::SetEnvironmentVariable("OS_TYPE", $env:OS_TYPE, "Machine")

}

function Start-Fluentbit {
    # Run fluent-bit service first so that we do not miss any logs being forwarded by the fluentd service and telegraf service.
    # Run fluent-bit as a background job. Switch this to a windows service once fluent-bit supports natively running as a windows service
    Start-Job -ScriptBlock { Start-Process -NoNewWindow -FilePath "C:\opt\fluent-bit\bin\fluent-bit.exe" -ArgumentList @("-c", "C:\opt\fluent-bit\fluent-bit-windows.conf", "-e", "C:\opt\fluent-bit\bin\out_appinsights.so") }

}

function Start-Telegraf {
    Write-Host "Installing telegraf service"
    /opt/telegraf/telegraf.exe --service install --config "/opt/telegraf/telegraf-prometheus-collector-windows.conf"

    # Setting delay auto start for telegraf since there have been known issues with windows server and telegraf -
    # https://github.com/influxdata/telegraf/issues/4081
    # https://github.com/influxdata/telegraf/issues/3601
    try {
        $serverName = [System.Environment]::GetEnvironmentVariable("POD_NAME", "process")
        if (![string]::IsNullOrEmpty($serverName)) {
            sc.exe \\$serverName config telegraf start= delayed-auto
            Write-Host "Successfully set delayed start for telegraf"

        }
        else {
            Write-Host "Failed to get environment variable POD_NAME to set delayed telegraf start"
        }
    }
    catch {
        $e = $_.Exception
        Write-Host $e
        Write-Host "exception occured in delayed telegraf start.. continuing without exiting"
    }
    Write-Host "Running telegraf service in test mode"
    /opt/telegraf/telegraf.exe --config "/opt/telegraf/telegraf-prometheus-collector-windows.conf" --test
    Write-Host "Starting telegraf service"
    # C:\opt\telegraf\telegraf.exe --service start
    /opt/telegraf/telegraf.exe --config "/opt/telegraf/telegraf-prometheus-collector-windows.conf" --service start

    # Trying to start telegraf again if it did not start due to fluent bit not being ready at startup
    Get-Service telegraf | findstr Running
    if ($? -eq $false) {
        Write-Host "trying to start telegraf in again in 30 seconds, since fluentbit might not have been ready..."
        Start-Sleep -s 30
        /opt/telegraf/telegraf.exe --service start
        Get-Service telegraf
    }
}

function Start-ME {
    Write-Output "Starting Metrics Extension..."
    Write-Output "ME_CONFIG_FILE = $env:ME_CONFIG_FILE"
    Write-Output "AZMON_DEFAULT_METRIC_ACCOUNT_NAME = $env:AZMON_DEFAULT_METRIC_ACCOUNT_NAME"
    Start-Job -ScriptBlock { 
        $me_config_file = $env:ME_CONFIG_FILE
        $AZMON_DEFAULT_METRIC_ACCOUNT_NAME = $env:AZMON_DEFAULT_METRIC_ACCOUNT_NAME
        $ME_ADDITIONAL_FLAGS = $env:ME_ADDITIONAL_FLAGS
        if (![string]::IsNullOrEmpty($ME_ADDITIONAL_FLAGS)) {
            Start-Process -NoNewWindow -FilePath "/opt/metricextension/MetricsExtension/MetricsExtension.Native.exe" -ArgumentList @("-Logger", "File", "-LogLevel", "Info", "-DataDirectory", ".\", "-Input", "otlp_grpc", "-MonitoringAccount", $AZMON_DEFAULT_METRIC_ACCOUNT_NAME, "-ConfigOverridesFilePath", $me_config_file, $ME_ADDITIONAL_FLAGS)
        }
        else {
            Start-Process -NoNewWindow -FilePath "/opt/metricextension/MetricsExtension/MetricsExtension.Native.exe" -ArgumentList @("-Logger", "File", "-LogLevel", "Info", "-DataDirectory", ".\", "-Input", "otlp_grpc", "-MonitoringAccount", $AZMON_DEFAULT_METRIC_ACCOUNT_NAME, "-ConfigOverridesFilePath", $me_config_file) 
        }
    }
    tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension
}

function Start-OTEL-Collector {
    if ($env:AZMON_USE_DEFAULT_PROMETHEUS_CONFIG -eq "true") {
        Write-Output "starting otelcollector with DEFAULT prometheus configuration...."
        Start-Job -ScriptBlock { Start-Process -RedirectStandardError /opt/microsoft/otelcollector/collector-log.txt -NoNewWindow -FilePath "/opt/microsoft/otelcollector/otelcollector.exe" -ArgumentList @("--config", "/opt/microsoft/otelcollector/collector-config-default.yml", "--log-level", "WARN", "--log-format", "json", "--metrics-level", "detailed") }
    }
    else {
        Write-Output "starting otelcollector...."
        Start-Job -ScriptBlock { Start-Process -RedirectStandardError /opt/microsoft/otelcollector/collector-log.txt -NoNewWindow -FilePath "/opt/microsoft/otelcollector/otelcollector.exe" -ArgumentList @("--config", "/opt/microsoft/otelcollector/collector-config.yml", "--log-level", "WARN", "--log-format", "json", "--metrics-level", "detailed") }
    }
    tasklist /fi "imagename eq otelcollector.exe" /fo "table"  | findstr otelcollector
}

function Set-CertificateForME {
    # Make a copy of the mounted akv directory to see if it changes
    mkdir -p /opt/akv-copy
    Copy-Item -r /etc/config/settings/akv /opt/akv-copy

    Get-ChildItem "C:\etc\config\settings\akv\" |  Foreach-Object { 
        if (!($_.Name.startswith('..'))) {
        Import-PfxCertificate -FilePath $_.FullName -CertStoreLocation Cert:\CurrentUser\My
        }
    }
}

function Start-FileSystemWatcher {
    Start-Process powershell -NoNewWindow /opt/scripts/filesystemwatcher.ps1
}

Start-Transcript -Path main.txt

Set-CertificateForME
Set-EnvironmentVariablesAndConfigParser

Start-FileSystemWatcher

Start-Fluentbit
Start-Telegraf
Start-OTEL-Collector
Start-ME

# Notepad.exe | Out-Null
Write-Output "Starting ping to keep the container running"
ping -t 127.0.0.1 | Out-Null