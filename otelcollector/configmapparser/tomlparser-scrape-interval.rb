#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
if (!ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux")
  require "re2"
end
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-scrape-interval-list"

@configMapMountPath = "/etc/config/settings/default-targets-scrape-interval-list"
@configVersion = ""
@configSchemaVersion = ""

@kubeletScrapeInterval = 30
@corednsScrapeInterval = 30
@cadvisorScrapeInterval = 30
@kubeproxyScrapeInterval = 30
@apiserverScrapeInterval = 30
@kubestateScrapeInterval = 30
@nodeexporterScrapeInterval = 30
@windowsexporterScrapeInterval = 30
@windowskubeproxyScrapeInterval = 30
@prometheusCollectorHealthInterval = 30

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-scrape-interval-list not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-scrape-interval-list: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    kubeletScrapeInterval = parsedConfig[:kubelet]
    if !kubeletScrapeInterval.nil?
        @kubeletScrapeInterval = kubeletScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeletScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeletScrapeInterval either not specified or not of type integer")
    end

    corednsScrapeInterval = parsedConfig[:coredns]
    if !corednsScrapeInterval.nil?
        @corednsScrapeInterval = corednsScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for corednsScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "corednsScrapeInterval either not specified or not of type integer")
    end

    cadvisorScrapeInterval = parsedConfig[:cadvisor]
    if !cadvisorScrapeInterval.nil?
        @cadvisorScrapeInterval = cadvisorScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for cadvisorScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "cadvisorScrapeInterval either not specified or not of type integer")
    end

    kubeproxyScrapeInterval = parsedConfig[:kubeproxy]
    if !kubeproxyScrapeInterval.nil?
        @kubeproxyScrapeInterval = kubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeproxyScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeproxyScrapeInterval either not specified or not of type integer")
    end

    apiserverScrapeInterval = parsedConfig[:apiserver]
    if !apiserverScrapeInterval.nil?
        @apiserverScrapeInterval = apiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for apiserverScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "apiserverScrapeInterval either not specified or not of type integer")
    end

    kubestateScrapeInterval = parsedConfig[:kubestate]
    if !kubestateScrapeInterval.nil?
        @kubestateScrapeInterval = kubestateScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubestateScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubestateScrapeInterval either not specified or not of type integer")
    end

    nodeexporterScrapeInterval = parsedConfig[:nodeexporter]
    if !nodeexporterScrapeInterval.nil?
        @nodeexporterScrapeInterval = nodeexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for nodeexporterScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "nodeexporterScrapeInterval either not specified or not of type integer")
    end

    windowsexporterScrapeInterval = parsedConfig[:windowsexporter]
    if !windowsexporterScrapeInterval.nil?
        @windowsexporterScrapeInterval = windowsexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowsexporterScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowsexporterScrapeInterval either not specified or not of type integer")
    end

    windowskubeproxyScrapeInterval = parsedConfig[:windowskubeproxy]
    if !windowskubeproxyScrapeInterval.nil?
        @windowskubeproxyScrapeInterval = windowskubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowskubeproxyScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowskubeproxyScrapeInterval either not specified or not of type integer")
    end

    prometheusCollectorHealthInterval = parsedConfig[:prometheuscollectorhealth]
    if !prometheusCollectorHealthInterval.nil?
        @prometheusCollectorHealthInterval = prometheusCollectorHealthInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for prometheusCollectorHealthInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "prometheusCollectorHealthInterval either not specified or not of type integer")
    end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-scrape-interval-list Processing")
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
file = File.open("/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash", "w")

intervalHash = {}
intervalHash["KUBELET_SCRAPE_INTERVAL"] = @kubeletScrapeInterval
intervalHash["COREDNS_SCRAPE_INTERVAL"] = @corednsScrapeInterval
intervalHash["CADVISOR_SCRAPE_INTERVAL"] = @cadvisorScrapeInterval
intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = @kubeproxyScrapeInterval
intervalHash["APISERVER_SCRAPE_INTERVAL"] = @apiserverScrapeInterval
intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = @kubestateScrapeInterval
intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = @nodeexporterScrapeInterval
intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = @windowsexporterScrapeInterval
intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = @windowskubeproxyScrapeInterval
intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = @prometheusCollectorHealthInterval

if !file.nil?
  # Close file after writing scrape interval list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(intervalHash.to_yaml)
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-scrape-interval-list regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-scrape-interval-list Processing")
