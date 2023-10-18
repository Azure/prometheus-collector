#!/bin/bash

# Colors for Logging
Color_Off='\033[0m'
Red='\033[0;31m'
Green='\033[0;32m'
Yellow='\033[0;33m'
Cyan='\033[0;36m'

# Echo text in red
echo_error () {
  echo -e "${Red}$1${Color_Off}"
}

# Echo text in yellow
echo_warning () {
  echo -e "${Yellow}$1${Color_Off}"
}

# Echo variable name in Cyan and value in regular color
echo_var () {
  echo -e "${Cyan}$1${Color_Off}=$2"
}

export IS_ARC_CLUSTER="false"
CLUSTER_nocase=$(echo $CLUSTER | tr "[:upper:]" "[:lower:]")
if [[ $CLUSTER_nocase =~ "connectedclusters" ]]; then
  export IS_ARC_CLUSTER="true"
fi
echo "export IS_ARC_CLUSTER=$IS_ARC_CLUSTER" >> ~/.bashrc

# EULA statement is required for Arc extension
if [ "$IS_ARC_CLUSTER" == "true" ]; then
  echo "MICROSOFT SOFTWARE LICENSE TERMS\n\nMICROSOFT Azure Arc-enabled Kubernetes\n\nThis software is licensed to you as part of your or your company's subscription license for Microsoft Azure Services. You may only use the software with Microsoft Azure Services and subject to the terms and conditions of the agreement under which you obtained Microsoft Azure Services. If you do not have an active subscription license for Microsoft Azure Services, you may not use the software. Microsoft Azure Legal Information: https://azure.microsoft.com/en-us/support/legal/"
fi

if [ -z $MODE ]; then
  MODE="simple"
fi
if [ -z $CCP_METRICS_ENABLED ]; then
  CCP_METRICS_ENABLED="false"
fi
echo_var "MODE" "$MODE"
echo_var "CCP_METRICS_ENABLED" "$CCP_METRICS_ENABLED"
echo_var "CONTROLLER_TYPE" "$CONTROLLER_TYPE"
echo_var "CLUSTER" "$CLUSTER"

# wait for configmap sync container to finish initialization
settingsChangedFile="/etc/config/settings/inotifysettingscreated"
if [ "${CCP_METRICS_ENABLED}" == "true" ] && [ ! -f $settingsChangedFile ]; then
  # Disable appinsights telemetry for ccp metrics
  export DISABLE_TELEMETRY=true
  if [ ! -f $settingsChangedFile ]; then
    echo "Waiting for ama-metrics-config-sync container to finish initialization..."
    while true; do
      event=$(inotifywait -q -e create --format '%f' $(dirname "$settingsChangedFile"))
      if [[ "$event" == "$(basename "$settingsChangedFile")" ]]; then
        break
      fi
    done
  fi
fi

#Run inotify as a daemon to track changes to the mounted configmap.
touch /opt/inotifyoutput.txt
inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'


# If using a trusted CA for HTTP Proxy, copy this over from the node and install
cp /anchors/ubuntu/* /etc/pki/ca-trust/source/anchors 2>/dev/null
cp /anchors/mariner/* /etc/pki/ca-trust/source/anchors 2>/dev/null
cp /anchors/proxy/* /etc/pki/ca-trust/source/anchors 2>/dev/null
update-ca-trust

# These env variables are populated by AKS in every kube-system pod
# Remove ending '/' character since mdsd doesn't recognize this as a valid url
if [ "$http_proxy" != "" ] && [ "${http_proxy: -1}" == "/" ]; then
  export http_proxy=${http_proxy::-1}
fi
if [ "$HTTP_PROXY" != "" ] && [ "${HTTP_PROXY: -1}" == "/" ]; then
 export HTTP_PROXY=${HTTP_PROXY::-1}
fi
if [ "$https_proxy" != "" ] && [ "${https_proxy: -1}" == "/" ]; then
  export https_proxy=${https_proxy::-1}
fi
if [ "$HTTPS_PROXY" != "" ] && [ "${HTTPS_PROXY: -1}" == "/" ]; then
  export HTTPS_PROXY=${HTTPS_PROXY::-1}
fi

# If HTTP Proxy is enabled, HTTP_PROXY will always have a value.
# HTTPS_PROXY will be set to same value as HTTP_PROXY if not specified.
export HTTP_PROXY_ENABLED="false"
if [ "$HTTP_PROXY" != "" ]; then
  export HTTP_PROXY_ENABLED="true"
fi
echo "export HTTP_PROXY_ENABLED=$HTTP_PROXY_ENABLED" >> ~/.bashrc

if [ $IS_ARC_CLUSTER == "true" ] && [ $HTTP_PROXY_ENABLED == "true" ]; then
  proxyprotocol="$(echo $HTTPS_PROXY | grep :// | sed -e's,^\(.*://\).*,\1,g')"
  proxyprotocol=$(echo $proxyprotocol | tr "[:upper:]" "[:lower:]")
  if [ "$proxyprotocol" != "http://" -a "$proxyprotocol" != "https://" ]; then
    echo_error "HTTP Proxy specified does not include http:// or https://"
  fi

  url="$(echo ${HTTPS_PROXY/$proxyprotocol/})"
  creds="$(echo $url | grep @ | cut -d@ -f1)"
  user="$(echo $creds | cut -d':' -f1)"
  password="$(echo $creds | cut -d':' -f2)"
  hostport="$(echo ${url/$creds@/} | cut -d/ -f1)"
  host="$(echo $hostport | sed -e 's,:.*,,g')"
  if [ -z "$host" ]; then
    echo_error "HTTP Proxy specified does not include a host"
  fi
  echo $password | base64 > /opt/microsoft/proxy_password
  export MDSD_PROXY_MODE=application
  echo "export MDSD_PROXY_MODE=$MDSD_PROXY_MODE" >> ~/.bashrc
  export MDSD_PROXY_ADDRESS=$proxyprotocol$hostport
  echo "export MDSD_PROXY_ADDRESS=$MDSD_PROXY_ADDRESS" >> ~/.bashrc
  if [ ! -z "$user" -a ! -z "$password" ]; then
    export MDSD_PROXY_USERNAME=$user
    echo "export MDSD_PROXY_USERNAME=$MDSD_PROXY_USERNAME" >> ~/.bashrc
    export MDSD_PROXY_PASSWORD_FILE=/opt/microsoft/proxy_password
    echo "export MDSD_PROXY_PASSWORD_FILE=$MDSD_PROXY_PASSWORD_FILE" >> ~/.bashrc
  fi
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
fi

aikey=$(echo $APPLICATIONINSIGHTS_AUTH | base64 -d)	
export TELEMETRY_APPLICATIONINSIGHTS_KEY=$aikey	
echo "export TELEMETRY_APPLICATIONINSIGHTS_KEY=$aikey" >> ~/.bashrc	
source ~/.bashrc

# Parse the settings for pod annotations
ruby /opt/microsoft/configmapparser/tomlparser-pod-annotation-based-scraping.rb
if [ -e "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping" ]; then
      cat /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping | while read line; do
            echo $line >> ~/.bashrc
      done
      source /opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping
      source ~/.bashrc
fi

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

# Parse the settings for debug mode
ruby /opt/microsoft/configmapparser/tomlparser-debug-mode.rb
if [ -e "/opt/microsoft/configmapparser/config_debug_mode_env_var" ]; then
      cat /opt/microsoft/configmapparser/config_debug_mode_env_var | while read line; do
            echo $line >> ~/.bashrc
      done
      source /opt/microsoft/configmapparser/config_debug_mode_env_var
      source ~/.bashrc
fi

# Parse the settings for default targets metrics keep list config
ruby /opt/microsoft/configmapparser/tomlparser-default-targets-metrics-keep-list.rb

# Parse the settings for default-targets-scrape-interval-settings config
ruby /opt/microsoft/configmapparser/tomlparser-scrape-interval.rb

# Merge default and custom prometheus config
ruby /opt/microsoft/configmapparser/prometheus-config-merger.rb

echo "export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=false" >> ~/.bashrc
export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=false
echo "export CONFIG_VALIDATOR_RUNNING_IN_AGENT=true" >> ~/.bashrc
export CONFIG_VALIDATOR_RUNNING_IN_AGENT=true
if [ -e "/opt/promMergedConfig.yml" ]; then
      # promconfigvalidator validates by generating an otel config and running through receiver's config load and validate method
      /opt/promconfigvalidator --config "/opt/promMergedConfig.yml" --output "/opt/microsoft/otelcollector/collector-config.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
      if [ $? -ne 0 ] || [ ! -e "/opt/microsoft/otelcollector/collector-config.yml" ]; then
            # Use default config if specified config is invalid
            echo_error "prom-config-validator::Prometheus custom config validation failed. The custom config will not be used"
            echo "export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=true" >> ~/.bashrc
            export AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=true
            if [ -e "/opt/defaultsMergedConfig.yml" ]; then
                  echo_error "prom-config-validator::Running validator on just default scrape configs"
                  /opt/promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
                  if [ $? -ne 0 ] || [ ! -e "/opt/collector-config-with-defaults.yml" ]; then
                        echo_error "prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used"
                  else
                        cp "/opt/collector-config-with-defaults.yml" "/opt/microsoft/otelcollector/collector-config-default.yml"
                  fi
            fi 
            echo "export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true" >> ~/.bashrc
            export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true
      fi
elif [ -e "/opt/defaultsMergedConfig.yml" ]; then
      echo_warning "prom-config-validator::No custom prometheus config found. Only using default scrape configs"
      /opt/promconfigvalidator --config "/opt/defaultsMergedConfig.yml" --output "/opt/collector-config-with-defaults.yml" --otelTemplate "/opt/microsoft/otelcollector/collector-config-template.yml"
      if [ $? -ne 0 ] || [ ! -e "/opt/collector-config-with-defaults.yml" ]; then
            echo_error "prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used"
      else
            echo "prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config"
            cp "/opt/collector-config-with-defaults.yml" "/opt/microsoft/otelcollector/collector-config-default.yml"
      fi
      echo "export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true" >> ~/.bashrc
      export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true

else
      # This else block is needed, when there is no custom config mounted as config map or default configs enabled
      echo_error "prom-config-validator::No custom config or default scrape configs enabled. No scrape configs will be used"
      echo "export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true" >> ~/.bashrc
      export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=true
fi

# Set the environment variables from the prom-config-validator
if [ -e "/opt/microsoft/prom_config_validator_env_var" ]; then
      cat /opt/microsoft/prom_config_validator_env_var | while read line; do
            echo $line >> ~/.bashrc
      done
      source /opt/microsoft/prom_config_validator_env_var
      source ~/.bashrc
fi

source ~/.bashrc
echo "prom-config-validator::Use default prometheus config: ${AZMON_USE_DEFAULT_PROMETHEUS_CONFIG}"

#start cron daemon for logrotate
/usr/sbin/crond -n -s &

#get controller kind in lowercase, trimmed
controllerType=$(echo $CONTROLLER_TYPE | tr "[:upper:]" "[:lower:]" | xargs)
if [ $controllerType = "replicaset" ]; then
   fluentBitConfigFile="/opt/fluent-bit/fluent-bit.conf"
   if [ "$CLUSTER_OVERRIDE" = "true" ]; then
      meConfigFile="/usr/sbin/me_internal.config"
   else
      meConfigFile="/usr/sbin/me.config"
   fi
else
   fluentBitConfigFile="/opt/fluent-bit/fluent-bit-daemonset.conf"
   if [ "$CLUSTER_OVERRIDE" = "true" ]; then
      meConfigFile="/usr/sbin/me_ds_internal.config"
   else
      meConfigFile="/usr/sbin/me_ds.config"
   fi
fi

if [ "${MAC}" == "true" ] && [ "${CCP_METRICS_ENABLED}" == "false" ]; then
      #wait for addon-token-adapter to be healthy
      tokenAdapterWaitsecs=60
      waitedSecsSoFar=1
      while true; do
            if [ $waitedSecsSoFar -gt $tokenAdapterWaitsecs ]; then
                  wget -T 2 -S http://localhost:9999/healthz -Y off 2>&1
                  echo "giving up waiting for token adapter to become healthy after $waitedSecsSoFar secs"
                  # log telemetry about failure after waiting for waitedSecsSoFar and break
                  echo "export tokenadapterUnhealthyAfterSecs=$waitedSecsSoFar" >>~/.bashrc
                  break
            else
                  echo "checking health of token adapter after $waitedSecsSoFar secs"
                  tokenAdapterResult=$(wget -T 2 -S http://localhost:9999/healthz -Y off 2>&1| grep HTTP/|awk '{print $2}'| grep 200)
            fi
            if [ ! -z $tokenAdapterResult ]; then
                        echo "found token adapter to be healthy after $waitedSecsSoFar secs" 
                        # log telemetry about success after waiting for waitedSecsSoFar and break
                        echo "export tokenadapterHealthyAfterSecs=$waitedSecsSoFar" >>~/.bashrc
                        break
            fi
            sleep 1
            waitedSecsSoFar=$(($waitedSecsSoFar + 1))
      done
      source ~/.bashrc
      #end wait for addon-token-adapter to be healthy
fi

export ME_CONFIG_FILE=$meConfigFile	
export FLUENT_BIT_CONFIG_FILE=$fluentBitConfigFile
echo "export ME_CONFIG_FILE=$meConfigFile" >> ~/.bashrc
echo "export FLUENT_BIT_CONFIG_FILE=$fluentBitConfigFile" >> ~/.bashrc
source ~/.bashrc
echo_var "ME_CONFIG_FILE" "$ME_CONFIG_FILE"

if [ "${MAC}" != "true" ]; then
      if [ -z $CLUSTER ]; then
            echo "CLUSTER is empty or not set. Using $NODE_NAME as CLUSTER"
            export customResourceId=$NODE_NAME
            echo "export customResourceId=$NODE_NAME" >> ~/.bashrc
            source ~/.bashrc
      else
            export customResourceId=$CLUSTER
            echo "export customResourceId=$CLUSTER" >> ~/.bashrc
            source ~/.bashrc
      fi

      # Make a copy of the mounted akv directory to see if it changes
      mkdir -p /opt/akv-copy
      cp -r /etc/config/settings/akv /opt/akv-copy

      decodeLocation="/opt/akv/decoded"
      # secrets can only be alpha numeric chars and dashes
      ENCODEDFILES=/etc/config/settings/akv/*
      mkdir -p $decodeLocation
      for ef in $ENCODEDFILES
      do
            name="$(basename -- $ef)"
            base64 -d $ef > $decodeLocation/$name
      done

      DECODEDFILES=$decodeLocation/*
      decodedFiles=""
      for df in $DECODEDFILES
      do
            if [ ${#decodedFiles} -ge 1 ]; then
                  decodedFiles=$decodedFiles:$df
            else
                  decodedFiles=$df
            fi
      done

      export AZMON_METRIC_ACCOUNTS_AKV_FILES=$(echo $decodedFiles)
      echo "export AZMON_METRIC_ACCOUNTS_AKV_FILES=$decodedFiles" >> ~/.bashrc
      source ~/.bashrc

      echo_var "AKV_FILES" "$AZMON_METRIC_ACCOUNTS_AKV_FILES"
      
      echo "Starting metricsextension"
      # will need to rotate the entire log location
      # will need to remove accountname fetching from env
      # Logs at level 'Info' to get metrics processed count. Fluentbit and out_appinsights filter the logs to only send errors and the metrics processed count to the telemetry
      /usr/sbin/MetricsExtension -Logger File -LogLevel Info -DataDirectory /opt/MetricsExtensionData -Input otlp_grpc_prom -PfxFile $AZMON_METRIC_ACCOUNTS_AKV_FILES -MonitoringAccount $AZMON_DEFAULT_METRIC_ACCOUNT_NAME -ConfigOverridesFilePath $ME_CONFIG_FILE $ME_ADDITIONAL_FLAGS > /dev/null &
else
      echo_var "customResourceId" "$CLUSTER"
      export customResourceId=$CLUSTER
      echo "export customResourceId=$CLUSTER" >> ~/.bashrc
      source ~/.bashrc

      trimmedRegion=$(echo $AKSREGION | sed 's/ //g' | awk '{print tolower($0)}')
      export customRegion=$trimmedRegion
      echo "export customRegion=$trimmedRegion" >> ~/.bashrc
      source ~/.bashrc
      echo_var "customRegion" "$trimmedRegion"

      echo "Waiting for 10s for token adapter sidecar to be up and running so that it can start serving IMDS requests"
      # sleep for 10 seconds
      sleep 10

      echo "Setting env variables from envmdsd file for MDSD"
      cat /etc/mdsd.d/envmdsd | while read line; do
            echo $line >> ~/.bashrc
      done
      source /etc/mdsd.d/envmdsd
      echo "Starting MDSD"
      # Use options -T 0x1 or -T 0xFFFF for debug logging
      mdsd -a -A -e ${MDSD_LOG}/mdsd.err -w ${MDSD_LOG}/mdsd.warn -o ${MDSD_LOG}/mdsd.info -q ${MDSD_LOG}/mdsd.qos 2>> /dev/null &

      # Running mdsd --version can't be captured into a variable unlike telegraf and otelcollector, have to run after printing the string
      echo -n -e "${Cyan}MDSD_VERSION${Color_Off}="; mdsd --version

      echo "Waiting for 30s for MDSD to get the config and put them in place for ME"
      # sleep for 30 seconds
      sleep 30

      echo "Reading me config file as a string for configOverrides paramater"
      export meConfigString=`cat $ME_CONFIG_FILE | tr '\r' ' ' |  tr '\n' ' ' | sed 's/\"/\\"/g' | sed 's/ //g'`
      echo "Starting metricsextension"
      /usr/sbin/MetricsExtension -Logger File -LogLevel Info -LocalControlChannel -TokenSource AMCS -DataDirectory /etc/mdsd.d/config-cache/metricsextension -Input otlp_grpc_prom -ConfigOverrides $meConfigString > /dev/null &
fi

# Get ME version
ME_VERSION=`cat /opt/metricsextversion.txt`
echo_var "ME_VERSION" "$ME_VERSION"

# Get ruby version
RUBY_VERSION=`ruby --version`
echo_var "RUBY_VERSION" "$RUBY_VERSION"

# Get golang version
GOLANG_VERSION=`cat /opt/goversion.txt`
echo_var "GOLANG_VERSION" "$GOLANG_VERSION"

# Start otelcollector
if [ "$AZMON_USE_DEFAULT_PROMETHEUS_CONFIG" = "true" ]; then
      echo_warning "Starting otelcollector with only default scrape configs enabled"
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config-default.yml &> /opt/microsoft/otelcollector/collector-log.txt &
else
      echo "Starting otelcollector"
      /opt/microsoft/otelcollector/otelcollector --config /opt/microsoft/otelcollector/collector-config.yml &> /opt/microsoft/otelcollector/collector-log.txt &
fi
OTELCOLLECTOR_VERSION=`/opt/microsoft/otelcollector/otelcollector --version`
echo_var "OTELCOLLECTOR_VERSION" "$OTELCOLLECTOR_VERSION"
PROMETHEUS_VERSION=`cat /opt/microsoft/otelcollector/PROMETHEUS_VERSION`
echo_var "PROMETHEUS_VERSION" "$PROMETHEUS_VERSION"

echo "starting telegraf"
if [ "$TELEMETRY_DISABLED" != "true" ]; then
  /usr/bin/telegraf --config /opt/telegraf/telegraf-prometheus-collector.conf &
  TELEGRAF_VERSION=`cat /opt/telegrafversion.txt`
  echo_var "TELEGRAF_VERSION" "$TELEGRAF_VERSION"
fi

echo "starting fluent-bit"
mkdir /opt/microsoft/fluent-bit
touch /opt/microsoft/fluent-bit/fluent-bit-out-appinsights-runtime.log
fluent-bit -c $FLUENT_BIT_CONFIG_FILE -e /opt/fluent-bit/bin/out_appinsights.so &
FLUENT_BIT_VERSION=`fluent-bit --version`
echo_var "FLUENT_BIT_VERSION" "$FLUENT_BIT_VERSION"
echo_var "FLUENT_BIT_CONFIG_FILE" "$FLUENT_BIT_CONFIG_FILE"

if [ "${MAC}" == "true" ]; then
  # Run inotify as a daemon to track changes to the dcr/dce config folder and restart container on changes, so that ME can pick them up.
  echo "starting inotify for watching mdsd config update"
  touch /opt/inotifyoutput-mdsd-config.txt
  inotifywait /etc/mdsd.d/config-cache/metricsextension/TokenConfig.json --daemon --outfile "/opt/inotifyoutput-mdsd-config.txt" --event ATTRIB --format '%e : %T' --timefmt '+%s'
fi

# Setting time at which the container started running, so that it can be used for empty configuration checks in livenessprobe
epochTimeNow=`date +%s`
echo $epochTimeNow > /opt/microsoft/liveness/azmon-container-start-time
echo_var "AZMON_CONTAINER_START_TIME" "$epochTimeNow"
epochTimeNowReadable=`date --date @$epochTimeNow`
echo_var "AZMON_CONTAINER_START_TIME_READABLE" "$epochTimeNowReadable"

shutdown() {
	echo "shutting down"
}

trap "shutdown" SIGTERM

sleep inf & wait