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

#Run inotify as a daemon to track changes to the mounted configmap.
touch /opt/inotifyoutput.txt
inotifywait /etc/config/settings --daemon --recursive --outfile "/opt/inotifyoutput.txt" --event create,delete --format '%e : %T' --timefmt '+%s'

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

echo_var "MODE" "$MODE"
echo_var "CONTAINER_TYPE" "$CONTAINER_TYPE"
echo_var "CLUSTER" "$CLUSTER"


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
      echo_error "prom-config-validator::No custom config via configmap or default scrape configs enabled."
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


# Run configreader to update the configmap for TargetAllocator
# Add this error handling - if [ $? -ne 0 ] after running the configurationreader exe
if [ "$AZMON_USE_DEFAULT_PROMETHEUS_CONFIG" = "true" ] && [ -e "/opt/microsoft/otelcollector/collector-config-default.yml" ] ; then
      echo_warning "Running config reader with only default scrape configs enabled"
      /opt/configurationreader --config /opt/microsoft/otelcollector/collector-config-default.yml
elif [ -e "/opt/microsoft/otelcollector/collector-config.yml" ]; then
      echo_warning "Running config reader with merged default and custom scrape config via configmap"
      /opt/configurationreader --config /opt/microsoft/otelcollector/collector-config.yml
else
      echo_warning "No configs found via configmap, not running config reader"
fi

# Get ruby version
RUBY_VERSION=`ruby --version`
echo_var "RUBY_VERSION" "$RUBY_VERSION"

# Get golang version
GOLANG_VERSION=`cat /opt/goversion.txt`
echo_var "GOLANG_VERSION" "$GOLANG_VERSION"

shutdown() {
	echo "shutting down"
}

trap "shutdown" SIGTERM

sleep inf & wait