#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus-collector-settings"
@configVersion = ""
@configSchemaVersion = ""

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@defaultMetricAccountName = "NONE"

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus collector file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "config::configmap prometheus-collector-configmap for prometheus collector settings mounted, parsing values"
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      puts "config::Successfully parsed mounted config map"
      return parsedConfig
    else
      puts "config::configmapprometheus-collector-configmap for prometheus collector settings not mounted, using defaults"
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for prometheus collector settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  # Get if otel collector prometheus scraping is enabled
  begin
    if !parsedConfig.nil? && !parsedConfig[:prometheus_collector_settings].nil? && !parsedConfig[:prometheus_collector_settings][:default_metric_account].nil? && !parsedConfig[:prometheus_collector_settings][:default_metric_account][:account_name].nil?
      @defaultMetricAccountName = parsedConfig[:prometheus_collector_settings][:default_metric_account][:account_name]
      puts "config::Using config map setting for prometheus collector default metric account name"
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while reading config map settings for prometheus collector settings- #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
puts "****************Start prometheus-collector Settings Processing********************"
if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  configMapSettings = parseConfigMap
  if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
  end
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError("config::unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var", "w")

if !file.nil?
  file.write("export AZMON_DEFAULT_METRIC_ACCOUNT_NAME=#{@defaultMetricAccountName}\n")
  # Close file after writing all metric collection setting environment variables
  file.close
  puts "****************End prometheus-collector Settings Processing********************"
else
  puts "Exception while opening file for writing prometheus-collector config environment variables"
  puts "****************End prometheus-collector Settings Processing********************"
end
