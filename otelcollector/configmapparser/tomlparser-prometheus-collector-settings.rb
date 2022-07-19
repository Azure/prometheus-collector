#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "config"

@configMapMountPath = "/etc/config/settings/prometheus-collector-settings"
@configVersion = ""
@configSchemaVersion = ""

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@defaultMetricAccountName = "NONE"

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmapprometheus-collector-configmap for prometheus collector settings not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for prometheus collector settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  # Get if otel collector prometheus scraping is enabled
  begin
    if !parsedConfig.nil? && !parsedConfig[:default_metric_account_name].nil?
      @defaultMetricAccountName = parsedConfig[:default_metric_account_name]
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap setting for default metric account name: #{@defaultMetricAccountName}")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for prometheus collector settings- #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start prometheus-collector-settings Processing")
if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  configMapSettings = parseConfigMap
  if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
  end
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var", "w")

if !file.nil?
  if !ENV['OS_TYPE'].nil? && ENV['OS_TYPE'].downcase == "linux"
    file.write("export AZMON_DEFAULT_METRIC_ACCOUNT_NAME=#{@defaultMetricAccountName}\n")
  else
    file.write("AZMON_DEFAULT_METRIC_ACCOUNT_NAME=#{@defaultMetricAccountName}\n")
  end
  
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing prometheus-collector config environment variables")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End prometheus-collector-settings Processing")
