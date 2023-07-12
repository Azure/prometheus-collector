#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "debug-mode-config"
@configMapMountPath = "/etc/config/settings/debug-mode"
@configVersion = ""
@configSchemaVersion = ""
@replicasetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@defaultEnabled = false

def parseConfigMap
  begin
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for debug mode: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    if !parsedConfig.nil? && !parsedConfig[:enabled].nil?
      @defaultEnabled = parsedConfig[:enabled]
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap setting for debug mode: #{@defaultEnabled}")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for debug mode- #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start debug-mode Settings Processing")
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
file = File.open("/opt/microsoft/configmapparser/config_debug_mode_env_var", "w")

if !file.nil?
  if !ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux"
    file.write("export DEBUG_MODE_ENABLED=#{@defaultEnabled}\n")
  else
    file.write("DEBUG_MODE_ENABLED=#{@defaultEnabled}\n")
  end

  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing prometheus-collector config environment variables")
end
# Adding logic to set otlp in service pipeline metrics when debug mode is enabled. This is done in promconfigvalidator for daemonset.
# We need to do this here for the replicaset since we don't run the promconfigvalidator for rs config.
if @defaultEnabled == true
  begin
    controllerType = ENV["CONTROLLER_TYPE"]
    if !controllerType.nil? && !controllerType.empty? && controllerType == "ReplicaSet"
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Setting otlp in the exporter metrics for service pipeline since debug mode is enabled ...")
      config = YAML.load(File.read(@replicasetCollectorConfig))
      if !config.nil?
        config["service"]["pipelines"]["metrics"]["exporters"] = ["otlp", "prometheus"]
        cfgYamlWithDebugModeSettings = YAML::dump(config)
        File.open(@replicasetCollectorConfig, "w") { |file| file.puts cfgYamlWithDebugModeSettings }
      end
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Done setting otlp in the exporter metrics for service pipeline.")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - #{errorStr}")
  end
end

ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End debug-mode Settings Processing")
