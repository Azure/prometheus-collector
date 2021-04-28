#!/bin/bash

#Run inotify as a daemon to track changes to the mounted configmap.
inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'

#resourceid override.
if [ -z $AKS_RESOURCE_ID ]; then
      echo "AKS_RESOURCE_ID is empty or not set."
      if [ -z $CLUSTER ]; then
            echo "CLUSTER is empty or not set. Using $NODE_NAME as CLUSTER"
            export customResourceId=$NODE_NAME
            echo "export customResourceId=$NODE_NAME" >> ~/.bashrc
            source ~/.bashrc
            echo "customResourceId:$customResourceId"
      else
            echo "Using CLUSTER as $CLUSTER"
            export customResourceId=$CLUSTER
            echo "export customResourceId=$CLUSTER" >> ~/.bashrc
            source ~/.bashrc
            echo "customResourceId:$customResourceId"
      fi
else
      export customResourceId=$AKS_RESOURCE_ID
      echo "export customResourceId=$AKS_RESOURCE_ID" >> ~/.bashrc
      source ~/.bashrc
      echo "customResourceId:$customResourceId"
fi

#set agent config schema version
if [  -e "/etc/config/settings/schema-version" ] && [  -s "/etc/config/settings/schema-version" ]; then
      #trim
      config_schema_version="$(cat /etc/config/settings/schema-version | xargs)"
      #remove all spaces
      config_schema_version="${config_schema_version//[[:space:]]/}"
      #take first 10 characters
      config_schema_version="$(echo $config_schema_version| cut -c1-10)"

      export AZMON_AGENT_CFG_SCHEMA_VERSION=$config_schema_version
      echo "export AZMON_AGENT_CFG_SCHEMA_VERSION=$config_schema_version" >> ~/.bashrc
      source ~/.bashrc
      echo "AZMON_AGENT_CFG_SCHEMA_VERSION:$AZMON_AGENT_CFG_SCHEMA_VERSION"
fi

#set agent config file version
if [  -e "/etc/config/settings/config-version" ] && [  -s "/etc/config/settings/config-version" ]; then
      #trim
      config_file_version="$(cat /etc/config/settings/config-version | xargs)"
      #remove all spaces
      config_file_version="${config_file_version//[[:space:]]/}"
      #take first 10 characters
      config_file_version="$(echo $config_file_version| cut -c1-10)"

      export AZMON_AGENT_CFG_FILE_VERSION=$config_file_version
      echo "export AZMON_AGENT_CFG_FILE_VERSION=$config_file_version" >> ~/.bashrc
      source ~/.bashrc
      echo "AZMON_AGENT_CFG_FILE_VERSION:$AZMON_AGENT_CFG_FILE_VERSION"
fi

# Check if the instrumentation key needs to be fetched from a storage account (as in airgapped clouds)
if [ ${#APPLICATIONINSIGHTS_AUTH_URL} -ge 1 ]; then  # (check if APPLICATIONINSIGHTS_AUTH_URL has length >=1)
      for BACKOFF in {1..4}; do
            KEY=$(curl -sS $APPLICATIONINSIGHTS_AUTH_URL )
            # there's no easy way to get the HTTP status code from curl, so just check if the result is well formatted
            if [[ $KEY =~ ^[A-Za-z0-9=]+$ ]]; then
                  break
            else
                  sleep $((2**$BACKOFF / 4))  # (exponential backoff)
            fi
      done

      # validate that the retrieved data is an instrumentation key
      if [[ $KEY =~ ^[A-Za-z0-9=]+$ ]]; then
            export APPLICATIONINSIGHTS_AUTH=$(echo $KEY)
            echo "export APPLICATIONINSIGHTS_AUTH=$APPLICATIONINSIGHTS_AUTH" >> ~/.bashrc
            echo "Using cloud-specific instrumentation key"
      else
            # no ikey can be retrieved. Disable telemetry and continue
            export DISABLE_TELEMETRY=true
            echo "export DISABLE_TELEMETRY=true" >> ~/.bashrc
            echo "Could not get cloud-specific instrumentation key (network error?). Disabling telemetry"
      fi
fi

aikey=$(echo $APPLICATIONINSIGHTS_AUTH | base64 --decode)	
export TELEMETRY_APPLICATIONINSIGHTS_KEY=$aikey	
echo "export TELEMETRY_APPLICATIONINSIGHTS_KEY=$aikey" >> ~/.bashrc	

source ~/.bashrc

# Parse the configmap to set the right environment variables for prometheus collector settings
ruby /opt/microsoft/configmapparser/tomlparser-prometheus-collector-settings.rb
cat /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var | while read line; do
      echo $line >> ~/.bashrc
done
source /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var
source ~/.bashrc

# Parse the settings for default scrape configs
ruby /opt/microsoft/configmapparser/tomlparser-default-scrape-settings.rb
if [ -e "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var" ]; then
      cat /opt/microsoft/configmapparser/config_default_scrape_settings_env_var | while read line; do
            echo $line >> ~/.bashrc
      done
      source /opt/microsoft/configmapparser/config_default_scrape_settings_env_var
      source ~/.bashrc
fi

if [ -e "/etc/config/settings/prometheus/prometheus-config" ]; then
      # Currently only logs the success or failure
      /opt/promtool check config /etc/config/settings/prometheus/prometheus-config

      # Use default config if specified config is invalid
      if [ $? -ne 0 ]; then
            echo "export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true" >> ~/.bashrc
            export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true
      # Get prometheus config and replace in otelcollector config
      else 
            ruby /opt/microsoft/configmapparser/tomlparser-prometheus-config.rb

            cat /opt/microsoft/configmapparser/config_prometheus_collector_prometheus_config_env_var | while read line; do
                  echo $line >> ~/.bashrc
            done
            source /opt/microsoft/configmapparser/config_prometheus_collector_prometheus_config_env_var
      fi
else
      echo "export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true" >> ~/.bashrc
      export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true
fi 



source ~/.bashrc

#start cron daemon for logrotate
service cron start

#start otelcollector
if [ -e "/etc/config/settings/prometheus/prometheus-config" ]; then
      echo "prometheus config specified for otel collector:"
      cat /etc/config/settings/prometheus/prometheus-config
fi
echo "Use default prometheus config: ${AZMON_USE_DEFAULT_PROMETHEUS_CONFIG}"

# will need to rotate log file
if [ "$AZMON_USE_DEFAULT_PROMETHEUS_CONFIG" = "true" ]; then
      echo "starting otelcollector with DEFAULT prometheus configuration...."
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config-default.yml --log-level ERROR --log-format json --metrics-level none &> /opt/microsoft/otelcollector/collector-log.txt &
else
      echo "starting otelcollector...."
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config.yml --log-level ERROR --log-format json --metrics-level none &> /opt/microsoft/otelcollector/collector-log.txt &
fi

echo "started otelcollector"

echo "finding files from akv in /etc/config/settings/akv to decode..."
decodeLocation="/opt/akv/decoded"
# secrets can only be alpha numeric chars and dashes
ENCODEDFILES=/etc/config/settings/akv/*
mkdir -p $decodeLocation
for ef in $ENCODEDFILES
do
 name="$(basename -- $ef)"
 echo "decoding $name into $decodeLocation ..."
 base64 -d $ef > $decodeLocation/$name
done


echo "finding decoded files from $decodeLocation ..."
DECODEDFILES=$decodeLocation/*
decodedFiles=""
for df in $DECODEDFILES
do
 echo "found $df"
 if [ ${#decodedFiles} -ge 1 ]; then
  decodedFiles=$decodedFiles:$df
 else
  decodedFiles=$df
 fi
done


export AZMON_METRIC_ACCOUNTS_AKV_FILES=$(echo $decodedFiles)
echo "export AZMON_METRIC_ACCOUNTS_AKV_FILES=$decodedFiles" >> ~/.bashrc
source ~/.bashrc

echo "AKV files for metric account=$AZMON_METRIC_ACCOUNTS_AKV_FILES"

echo "starting metricsextension"
# will need to rotate the entire log location
# will need to remove accountname fetching from env
# Logs at level 'Info' to get metrics processed count. Fluentbit and out_appinsights filter the logs to only send errors and the metrics processed count to the telemetry
/usr/sbin/MetricsExtension -Logger File -LogLevel Info -DataDirectory /opt/MetricsExtensionData -Input otlp_grpc -PfxFile $AZMON_METRIC_ACCOUNTS_AKV_FILES -MonitoringAccount $AZMON_DEFAULT_METRIC_ACCOUNT_NAME -ConfigOverridesFilePath /usr/sbin/me.config $ME_ADDITIONAL_FLAGS &

#get ME version
dpkg -l | grep metricsext | awk '{print $2 " " $3}'
#get ruby version
ruby --version

echo "starting telegraf"
/opt/telegraf/telegraf --config /opt/telegraf/telegraf-prometheus-collector.conf &

echo "starting fluent-bit"
/opt/td-agent-bit/bin/td-agent-bit -c /opt/fluent-bit/fluent-bit.conf -e /opt/fluent-bit/bin/out_appinsights.so &
dpkg -l | grep td-agent-bit | awk '{print $2 " " $3}'

shutdown() {
	echo "shutting down"
	}

trap "shutdown" SIGTERM

sleep inf & wait
