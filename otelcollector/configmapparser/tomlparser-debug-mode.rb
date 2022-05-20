#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/debug-mode"
@configVersion = ""
@configSchemaVersion = ""

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@defaultEnabled = false

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus collector file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "config::configmap prometheus-collector-configmap for debug mode mounted, parsing values"
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      puts "config::Successfully parsed mounted config map"
      return parsedConfig
    else
      puts "config::configmapprometheus-collector-configmap for debug mode not mounted, using defaults"
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for debug mode: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  # Get if otel collector prometheus scraping is enabled
  begin
    if !parsedConfig.nil? && !parsedConfig[:enabled].nil?
      @defaultEnabled = parsedConfig[:enabled]
      puts "config::Using config map setting for debug mode"
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while reading config map settings for debug mode- #{errorStr}, using defaults, please check config map for errors")
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
file = File.open("/opt/microsoft/configmapparser/config_debug_mode_env_var", "w")

if !file.nil?
  if !ENV['OS_TYPE'].nil? && ENV['OS_TYPE'].downcase == "linux"
    file.write("export DEBUG_MODE_ENABLED=#{@defaultEnabled}\n")
  else
    file.write("DEBUG_MODE_ENABLED=#{@defaultEnabled}\n")
  end
  
  file.close
else
  puts "Exception while opening file for writing prometheus-collector config environment variables".red
end
puts "****************End debug-mode Settings Processing********************".green
