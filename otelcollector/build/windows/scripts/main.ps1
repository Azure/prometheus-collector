#setting it to replicaset by default
$me_config_file = '/opt/metricextension/me_ds.config'

function Set-EnvironmentVariablesAndConfigParser {
    #resourceid override.
    if ([string]::IsNullOrEmpty($env:MAC)) {
        if ([string]::IsNullOrEmpty($env:CLUSTER)) {
            Write-Output "CLUSTER is empty or not set. Using $env:NODE_NAME as CLUSTER"
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:NODE_NAME, "Process")
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:NODE_NAME, "Machine")
            Write-Output "customResourceId=$env:customResourceId"
        }
        else {
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Process")
            [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Machine")
            Write-Output "customResourceId=$env:customResourceId"
        }
    }
    else {
        [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Process")
        [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Machine")

        [System.Environment]::SetEnvironmentVariable("customRegion", $env:AKSREGION, "Process")
        [System.Environment]::SetEnvironmentVariable("customRegion", $env:AKSREGION, "Machine")

        Write-Output "customResourceId=$customResourceId"
        Write-Output "customRegion=$customRegion"
    }

    ############### Environment variables for MA {Start} ###############

    ############################################################################# test
    [System.Environment]::SetEnvironmentVariable("MONITORING_ROLE_INSTANCE", "cloudAgentRoleInstanceIdentity", "Process")
    [System.Environment]::SetEnvironmentVariable("MCS_AZURE_RESOURCE_ENDPOINT", "https://monitor.azure.com/", "Process")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_OsType", "Windows", "Process")
    [System.Environment]::SetEnvironmentVariable("MONITORING_VERSION", "2.0", "Process")
    [System.Environment]::SetEnvironmentVariable("MONITORING_ROLE", "cloudAgentRoleIdentity", "Process")
    [System.Environment]::SetEnvironmentVariable("MONITORING_IDENTITY", "use_ip_address", "Process")
    [System.Environment]::SetEnvironmentVariable("MONITORING_ROLE_INSTANCE", "cloudAgentRoleInstanceIdentity", "Machine")
    [System.Environment]::SetEnvironmentVariable("MCS_AZURE_RESOURCE_ENDPOINT", "https://monitor.azure.com/", "Machine")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_OsType", "Windows", "Machine")
    [System.Environment]::SetEnvironmentVariable("MONITORING_VERSION", "2.0", "Machine")
    [System.Environment]::SetEnvironmentVariable("MONITORING_ROLE", "cloudAgentRoleIdentity", "Machine")
    [System.Environment]::SetEnvironmentVariable("MONITORING_IDENTITY", "use_ip_address", "Machine")
    ############################################################################# test
    [System.Environment]::SetEnvironmentVariable("MONITORING_USE_GENEVA_CONFIG_SERVICE", "false", "Process")
    [System.Environment]::SetEnvironmentVariable("MONITORING_USE_GENEVA_CONFIG_SERVICE", "false", "Machine")
    [System.Environment]::SetEnvironmentVariable("SKIP_IMDS_LOOKUP_FOR_LEGACY_AUTH", "true", "Process")
    [System.Environment]::SetEnvironmentVariable("SKIP_IMDS_LOOKUP_FOR_LEGACY_AUTH", "true", "Machine")
    [System.Environment]::SetEnvironmentVariable("ENABLE_MCS", "true", "Process")
    [System.Environment]::SetEnvironmentVariable("ENABLE_MCS", "true", "Machine")
    [System.Environment]::SetEnvironmentVariable("MDSD_USE_LOCAL_PERSISTENCY", "false", "Process")
    [System.Environment]::SetEnvironmentVariable("MDSD_USE_LOCAL_PERSISTENCY", "false", "Machine")
    ############################################################################# test


    [System.Environment]::SetEnvironmentVariable("MCS_GLOBAL_ENDPOINT", "https://global.handler.control.monitor.azure.com", "Process")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_Location", $env:AKSREGION, "Process")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_ResourceId", $env:CLUSTER, "Process")
    # [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Process")
    [System.Environment]::SetEnvironmentVariable("MCS_CUSTOM_RESOURCE_ID", $env:CLUSTER, "Process")
    [System.Environment]::SetEnvironmentVariable("customRegion", $env:AKSREGION, "Process")
    [System.Environment]::SetEnvironmentVariable("MCS_GLOBAL_ENDPOINT", "https://global.handler.control.monitor.azure.com", "Machine")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_Location", $env:AKSREGION, "Machine")
    [System.Environment]::SetEnvironmentVariable("MA_RoleEnvironment_ResourceId", $env:CLUSTER, "Machine")
    # [System.Environment]::SetEnvironmentVariable("customResourceId", $env:CLUSTER, "Machine")
    [System.Environment]::SetEnvironmentVariable("MCS_CUSTOM_RESOURCE_ID", $env:CLUSTER, "Machine")
    [System.Environment]::SetEnvironmentVariable("customRegion", $env:AKSREGION, "Machine")
    ############### Environment variables for MA {End} ###############

    if ([string]::IsNullOrEmpty($env:MODE)) {
        [System.Environment]::SetEnvironmentVariable("MODE", 'simple', "Process")
        [System.Environment]::SetEnvironmentVariable("MODE", 'simple', "Machine")
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

    [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", "Process")
    [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", "Machine")

    [System.Environment]::SetEnvironmentVariable("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", "Process")
    [System.Environment]::SetEnvironmentVariable("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", "Machine")

    if (Test-Path -Path '/opt/promMergedConfig.yml') {
        C:\opt\promconfigvalidator --config "/opt/promMergedConfig.yml" --output "/opt/microsoft/otelcollector/collector-config.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
        if ( (!($?)) -or (!(Test-Path -Path "/opt/microsoft/otelcollector/collector-config.yml" ))) {
            Write-Output "prom-config-validator::Prometheus custom config validation failed. The custom config will not be used"
            # This env variable is used to indicate that the prometheus custom config was invalid and we fall back to defaults, used for telemetry
            [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", "Process")
            [System.Environment]::SetEnvironmentVariable("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", "Machine")
            if (Test-Path -Path '/opt/defaultsMergedConfig.yml') {
                Write-Output "prom-config-validator::Running validator on just default scrape configs"
                C:\opt\promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
                if ( (!($?)) -or (!(Test-Path -Path "/opt/collector-config-with-defaults.yml" ))) {
                    Write-Output "prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used"
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
        Write-Output "prom-config-validator::No custom prometheus config found. Only using default scrape configs"
        C:\opt\promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
        if ( (!($?)) -or (!(Test-Path -Path "/opt/collector-config-with-defaults.yml" ))) {
            Write-Output "prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used"
        }
        else {
            Write-Output "prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config"
            Copy-Item "/opt/collector-config-with-defaults.yml" "/opt/microsoft/otelcollector/collector-config-default.yml"
        }
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Machine")
    }
    else {
        # This else block is needed, when there is no custom config mounted as config map or default configs enabled
        Write-Output "prom-config-validator::No custom config or default scrape configs enabled. No scrape configs will be used"
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Process")
        [System.Environment]::SetEnvironmentVariable("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", "Machine")
    }

    if (Test-Path -Path '/opt/microsoft/prom_config_validator_env_var') {
        foreach ($line in Get-Content /opt/microsoft/prom_config_validator_env_var) {
            if ($line.Contains('=')) {
                $key = ($line -split '=')[0];
                $value = ($line -split '=')[1];
                [System.Environment]::SetEnvironmentVariable($key, $value, "Process")
                [System.Environment]::SetEnvironmentVariable($key, $value, "Machine")
            }
        }
    }

    # #start cron daemon for logrotate
    # service cron restart

    #start otelcollector
    Write-Output "Use default prometheus config: $env:AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"

    #get controller kind in lowercase, trimmed
    $controllerType = $env:CONTROLLER_TYPE
    $controllerType = $controllerType.Trim()
    $cluster_override = $env:CLUSTER_OVERRIDE
    if ($controllerType -eq "replicaset") {
        if ($cluster_override -eq "true") {
            $meConfigFile = "/opt/metricextension/me_internal.config"
        }
        else {
            $meConfigFile = "/opt/metricextension/me.config"
        }
    }
    else {
        if ($cluster_override -eq "true") {
            $meConfigFile = "/opt/metricextension/me_ds_internal.config"
        }
        else {
            $meConfigFile = "/opt/metricextension/me_ds.config"
        }
    }
    [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $meConfigFile, "Process")
    [System.Environment]::SetEnvironmentVariable("ME_CONFIG_FILE", $meConfigFile, "Machine")


    # Set ME Config file
    if (![string]::IsNullOrEmpty($env:CONTROLLER_TYPE)) {
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
    [System.Environment]::SetEnvironmentVariable("CONTAINER_CPU_LIMIT", $env:CONTAINER_CPU_LIMIT, "Machine")
    [System.Environment]::SetEnvironmentVariable("CONTAINER_MEMORY_LIMIT", $env:CONTAINER_MEMORY_LIMIT, "Machine")

}

function Start-Fluentbit {
    # Run fluent-bit service first so that we do not miss any logs being forwarded by the fluentd service and telegraf service.
    # Run fluent-bit as a background job. Switch this to a windows service once fluent-bit supports natively running as a windows service
    Write-Host "Starting fluent-bit"
    Start-Job -ScriptBlock { Start-Process -NoNewWindow -FilePath "C:\opt\fluent-bit\bin\td-agent-bit.exe" -ArgumentList @("-c", "C:\opt\fluent-bit\fluent-bit-windows.conf", "-e", "C:\opt\fluent-bit\bin\out_appinsights.so") }

}

function Start-Telegraf {
    Write-Host "Installing telegraf service"
    /opt/telegraf/telegraf.exe --service install --config "/opt/telegraf/telegraf-prometheus-collector-windows.conf" > $null

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
    }
}
function Start-OTEL-Collector {
    if ($env:AZMON_USE_DEFAULT_PROMETHEUS_CONFIG -eq "true") {
        Write-Output "Starting otelcollector with only default scrape configs enabled"
        Start-Job -ScriptBlock { Start-Process -RedirectStandardError /opt/microsoft/otelcollector/collector-log.txt -NoNewWindow -FilePath "/opt/microsoft/otelcollector/otelcollector.exe" -ArgumentList @("--config", "/opt/microsoft/otelcollector/collector-config-default.yml") } > $null
    }
    else {
        Write-Output "Starting otelcollector"
        Start-Job -ScriptBlock { Start-Process -RedirectStandardError /opt/microsoft/otelcollector/collector-log.txt -NoNewWindow -FilePath "/opt/microsoft/otelcollector/otelcollector.exe" -ArgumentList @("--config", "/opt/microsoft/otelcollector/collector-config.yml") } > $null
    }
    tasklist /fi "imagename eq otelcollector.exe" /fo "table"  | findstr otelcollector
}

function Set-CertificateForME {
    # Make a copy of the mounted akv directory to see if it changes
    mkdir -p /opt/akv-copy > $null
    Copy-Item -r /etc/config/settings/akv /opt/akv-copy

    Get-ChildItem "C:\etc\config\settings\akv\" |  Foreach-Object {
        if (!($_.Name.startswith('..'))) {
            Import-PfxCertificate -FilePath $_.FullName -CertStoreLocation Cert:\CurrentUser\My > $null
        }
    }
}

function Start-FileSystemWatcher {
    Start-Process powershell -NoNewWindow /opt/scripts/filesystemwatcher.ps1 > $null
}

#start Windows AMA
function Start-MA {
    Write-Output "Starting MA"
    Start-Job -ScriptBlock { Start-Process -NoNewWindow -FilePath "C:\opt\genevamonitoringagent\genevamonitoringagent\Monitoring\Agent\MonAgentLauncher.exe" -ArgumentList @("-useenv") }
}

function Start-ME {
    Write-Output "Starting Metrics Extension"
    Write-Output "ME_CONFIG_FILE = $env:ME_CONFIG_FILE"
    Write-Output "AZMON_DEFAULT_METRIC_ACCOUNT_NAME = $env:AZMON_DEFAULT_METRIC_ACCOUNT_NAME"
    Start-Job -ScriptBlock {
        $me_config_file = $env:ME_CONFIG_FILE
        $AZMON_DEFAULT_METRIC_ACCOUNT_NAME = $env:AZMON_DEFAULT_METRIC_ACCOUNT_NAME
        $ME_ADDITIONAL_FLAGS = $env:ME_ADDITIONAL_FLAGS
        # if (![string]::IsNullOrEmpty($ME_ADDITIONAL_FLAGS)) {
        #     Start-Process -NoNewWindow -FilePath "/opt/metricextension/MetricsExtension/MetricsExtension.Native.exe" -ArgumentList @("-Logger", "File", "-LogLevel", "Info", "-DataDirectory", ".\", "-Input", "otlp_grpc_prom", "-MonitoringAccount", $AZMON_DEFAULT_METRIC_ACCOUNT_NAME, "-ConfigOverridesFilePath", $me_config_file, $ME_ADDITIONAL_FLAGS) > $null
        # }
        # else {
        #     Start-Process -NoNewWindow -FilePath "/opt/metricextension/MetricsExtension/MetricsExtension.Native.exe" -ArgumentList @("-Logger", "File", "-LogLevel", "Info", "-DataDirectory", ".\", "-Input", "otlp_grpc_prom", "-MonitoringAccount", $AZMON_DEFAULT_METRIC_ACCOUNT_NAME, "-ConfigOverridesFilePath", $me_config_file) > $null
        # }

        #     echo "Reading me config file as a string for configOverrides paramater"
        #   export meConfigString=`cat $ME_CONFIG_FILE | tr '\r' ' ' |  tr '\n' ' ' | sed 's/\"/\\"/g' | sed 's/ //g'`
        Start-Process -NoNewWindow -FilePath "/opt/metricextension/MetricsExtension/MetricsExtension.Native.exe" -ArgumentList @("-Logger", "File", "-LogLevel", "Debug", "-LocalControlChannel", "-TokenSource", "AMCS", "-DataDirectory", "C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension\", "-Input", "otlp_grpc_prom", "-ConfigOverridesFilePath", $me_config_file) > $null
        # /opt/metricextension/MetricsExtension/MetricsExtension.Native.exe -Logger Console -LogLevel Info -LocalControlChannel -TokenSource AMCS -DataDirectory C:\opt\genevamonitoringagent\datadirectory\mcs\metricsextension\ -Input otlp_grpc_prom -ConfigOverridesFilePath '/opt/metricextension/me_ds.config'
    }
    tasklist /fi "imagename eq MetricsExtension.Native.exe" /fo "table"  | findstr MetricsExtension
}

Start-Transcript -Path main.txt
# Set-CertificateForME
Set-EnvironmentVariablesAndConfigParser
Start-FileSystemWatcher
Start-Fluentbit
Start-Telegraf
Start-OTEL-Collector
################################################################################################################
# Should I wait for token sidecar adapter to serve IMDS requests as the main prometheus-collector pod takes a while to start??
################################################################################################################
Start-MA
# "Waiting for 121s for MA to get the config and put them in place for ME"
Start-Sleep 122
Start-ME

# Notepad.exe | Out-Null
Write-Output "Starting ping to keep the container running"
ping -t 127.0.0.1 | Out-Null