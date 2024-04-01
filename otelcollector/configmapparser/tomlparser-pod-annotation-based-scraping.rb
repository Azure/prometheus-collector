#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "yaml"
require_relative "ConfigParseErrorLogger"
require_relative "tomlparser-utils"

LOGGING_PREFIX = "pod-annotation-based-scraping"
@configMapMountPath = "/etc/config/settings/pod-annotation-based-scraping"
@podannotationNamespaceRegex = ""

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap section not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    podannotationRegex = parsedConfig[:podannotationnamespaceregex]
    # Make backwards compatible
    if podannotationRegex.nil? || podannotationRegex.empty?
      podannotationRegex = parsedConfig[:podannotationnamepsaceregex]
    end
    if !podannotationRegex.nil? && podannotationRegex.kind_of?(String) && !podannotationRegex.empty?
      if isValidRegex(podannotationRegex) == true
        @podannotationNamespaceRegex = podannotationRegex
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap namepace regex for podannotations")
      else
        ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid namespace regex for podannotations")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "podannotations namespace regex either not specified or not of type string")
    end
  end
end

ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start Processing")
configMapSettings = parseConfigMap
if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
elsif (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Error loading configmap section - using defaults")
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping", "w")

namespaceRegexHash = {}
namespaceRegexHash["POD_ANNOTATION_NAMESPACES_REGEX"] = @podannotationNamespaceRegex

if !file.nil?
  # Close file after writing scrape interval list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write("export AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX=#{@podannotationNamespaceRegex}\n")
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-namespace-keep-list-regex-settings Processing")
