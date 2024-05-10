#!/bin/bash

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
if [ "${AZMON_OPERATOR_ENABLED}" == "true" ] || [ "${CONTAINER_TYPE}" == "ConfigReaderSidecar" ]; then
      ruby /opt/microsoft/configmapparser/prometheus-config-merger-with-operator.rb
else
      ruby /opt/microsoft/configmapparser/prometheus-config-merger.rb
fi

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