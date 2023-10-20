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

@clusterAlias = ""  # user provided alias (thru config map or chart param)
@clusterLabel = ""  # value of the 'cluster' label in every time series scraped
@isOperatorEnabled = ""
@isOperatorEnabledChartSetting = ""

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

  begin
    if !parsedConfig.nil? && !parsedConfig[:cluster_alias].nil?
      @clusterAlias = parsedConfig[:cluster_alias].strip
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Got configmap setting for cluster_alias:#{@clusterAlias}")
      @clusterAlias = @clusterAlias.gsub(/[^0-9a-z]/i, "_") #replace all non alpha-numeric characters with "_"  -- this is to ensure that all down stream places where this is used (like collector, telegraf config etc are keeping up with sanity)
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "After g-subing configmap setting for cluster_alias:#{@clusterAlias}")
    end
  rescue => errorStr
    @clusterAlias = ""
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for cluster_alias in prometheus collector settings- #{errorStr}, using defaults, please check config map for errors")
  end

  # Safeguard to fall back to non operator model, enable to set to true or false only when toggle is enabled
  if !ENV["AZMON_OPERATOR_ENABLED"].nil? && ENV["AZMON_OPERATOR_ENABLED"].downcase == "true"
    begin
      @isOperatorEnabledChartSetting = "true"
      if !parsedConfig.nil? && !parsedConfig[:operator_enabled].nil?
        @isOperatorEnabled = parsedConfig[:operator_enabled]
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Configmap setting enabling operator: #{@isOperatorEnabled}")
      end
    rescue => errorStr
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for prometheus collector settings- #{errorStr}, using defaults, please check config map for errors")
    end
  else
    @isOperatorEnabledChartSetting = "false"
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

# get clustername from cluster's full ARM resourceid (to be used for mac mode as 'cluster' label)
begin
  if !ENV["MAC"].nil? && !ENV["MAC"].empty? && ENV["MAC"].strip.downcase == "true"
    resourceArray = ENV["CLUSTER"].strip.split("/")
    @clusterLabel = resourceArray[resourceArray.length - 1]
  else
    @clusterLabel = ENV["CLUSTER"]
  end
rescue => errorStr
  @clusterLabel = ENV["CLUSTER"]
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing to determine cluster label from full cluster resource id in prometheus collector settings- #{errorStr}, using default as full CLUSTER passed-in '#{@clusterLabel}'")
end

#override cluster label with cluster alias, if alias is specified

if !@clusterAlias.nil? && !@clusterAlias.empty? && @clusterAlias.length > 0
  @clusterLabel = @clusterAlias
  ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using clusterLabel from cluster_alias:#{@clusterAlias}")
end

ConfigParseErrorLogger.log(LOGGING_PREFIX, "AZMON_CLUSTER_ALIAS:'#{@clusterAlias}'")
ConfigParseErrorLogger.log(LOGGING_PREFIX, "AZMON_CLUSTER_LABEL:#{@clusterLabel}")

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var", "w")

if !file.nil?
  if !ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux"
    file.write("export AZMON_DEFAULT_METRIC_ACCOUNT_NAME=#{@defaultMetricAccountName}\n")
    file.write("export AZMON_CLUSTER_LABEL=#{@clusterLabel}\n") #used for cluster label value when scraping
    file.write("export AZMON_CLUSTER_ALIAS=#{@clusterAlias}\n") #used only for telemetry
    file.write("export AZMON_OPERATOR_ENABLED_CHART_SETTING=#{@isOperatorEnabledChartSetting}\n")
    if !@isOperatorEnabled.nil? && !@isOperatorEnabled.empty? && @isOperatorEnabled.length > 0
      file.write("export AZMON_OPERATOR_ENABLED=#{@isOperatorEnabled}\n")
      file.write("export AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING=#{@isOperatorEnabled}\n")
    end
  else
    file.write("AZMON_DEFAULT_METRIC_ACCOUNT_NAME=#{@defaultMetricAccountName}\n")
    file.write("AZMON_CLUSTER_LABEL=#{@clusterLabel}\n") #used for cluster label value when scraping
    file.write("AZMON_CLUSTER_ALIAS=#{@clusterAlias}\n") #used only for telemetry
  end

  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing prometheus-collector config environment variables")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End prometheus-collector-settings Processing")
