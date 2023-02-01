#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-targets-namespace-keep-list-settings"
@configMapMountPath = "/etc/config/settings/default-targets-namespace-keep-list-settings"
@podannotationNamespaceKeepListRegex = ""

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-namespace-keep-list-settings not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-namespace-keep-list-settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    podannotationRegex = parsedConfig[:podannoations]
    if !podannotationRegex.nil? && podannotationRegex.kind_of?(String) && !podannotationRegex.empty?
      if isValidRegex(podAnnotationRegex) == true
        @podannotationNamespaceKeepListRegex = podAnnotationRegex
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap namepace keep list regex for podannotations")
      else
        ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid namespace keep list regex for podannotations")
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "podannotations namespace keep list regex either not specified or not of type string")
    end
  end
end

ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-namespace-keep-list-settings Processing")
if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
  end
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Error loading default-targets-namespace-keep-list-settings - using defaults")
  end
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_def_targets_namespace_keep_list_regex_hash", "w")

namespaceRegexHash = {}
namespaceRegexHash["POD_ANNOTATION_NAMESPACE_KEEP_LIST_REGEX"] = @podannotationNamespaceKeepListRegex

if !file.nil?
  # Close file after writing scrape interval list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(namespaceRegexHash.to_yaml)
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-namespace-keep-list-regex-settings Processing")
