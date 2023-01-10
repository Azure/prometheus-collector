#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
<<<<<<< HEAD
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
=======
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-scrape-interval-settings"

# Checking to see if the duration matches the pattern specified in the prometheus config
# Link to documenation with regex pattern -> https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
MATCHER = /^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$/

@configMapMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
@configVersion = ""
@configSchemaVersion = ""

@kubeletScrapeInterval = "30s"
@corednsScrapeInterval = "30s"
@cadvisorScrapeInterval = "30s"
@kubeproxyScrapeInterval = "30s"
@apiserverScrapeInterval = "30s"
@kubestateScrapeInterval = "30s"
@nodeexporterScrapeInterval = "30s"
@windowsexporterScrapeInterval = "30s"
@windowskubeproxyScrapeInterval = "30s"
@prometheusCollectorHealthInterval = "30s"
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
<<<<<<< HEAD
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-scrape-interval-list not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-scrape-interval-list: #{errorStr}, using defaults, please check config map for errors")
=======
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-scrape-interval-settings not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-scrape-interval-settings: #{errorStr}, using defaults, please check config map for errors")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    kubeletScrapeInterval = parsedConfig[:kubelet]
    if !kubeletScrapeInterval.nil?
<<<<<<< HEAD
        @kubeletScrapeInterval = kubeletScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeletScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeletScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(kubeletScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubeletScrapeInterval = "30s"
        @kubeletScrapeInterval = kubeletScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @kubeletScrapeInterval = kubeletScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeletScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeletScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    corednsScrapeInterval = parsedConfig[:coredns]
    if !corednsScrapeInterval.nil?
<<<<<<< HEAD
        @corednsScrapeInterval = corednsScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for corednsScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "corednsScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(corednsScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        corednsScrapeInterval = "30s"
        @corednsScrapeInterval = corednsScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @corednsScrapeInterval = corednsScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for corednsScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "corednsScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    cadvisorScrapeInterval = parsedConfig[:cadvisor]
    if !cadvisorScrapeInterval.nil?
<<<<<<< HEAD
        @cadvisorScrapeInterval = cadvisorScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for cadvisorScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "cadvisorScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(cadvisorScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        cadvisorScrapeInterval = "30s"
        @cadvisorScrapeInterval = cadvisorScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @cadvisorScrapeInterval = cadvisorScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for cadvisorScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "cadvisorScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    kubeproxyScrapeInterval = parsedConfig[:kubeproxy]
    if !kubeproxyScrapeInterval.nil?
<<<<<<< HEAD
        @kubeproxyScrapeInterval = kubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeproxyScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeproxyScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(kubeproxyScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubeproxyScrapeInterval = "30s"
        @kubeproxyScrapeInterval = kubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @kubeproxyScrapeInterval = kubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeproxyScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeproxyScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    apiserverScrapeInterval = parsedConfig[:apiserver]
    if !apiserverScrapeInterval.nil?
<<<<<<< HEAD
        @apiserverScrapeInterval = apiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for apiserverScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "apiserverScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(apiserverScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        apiserverScrapeInterval = "30s"
        @apiserverScrapeInterval = apiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @apiserverScrapeInterval = apiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for apiserverScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "apiserverScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    kubestateScrapeInterval = parsedConfig[:kubestate]
    if !kubestateScrapeInterval.nil?
<<<<<<< HEAD
        @kubestateScrapeInterval = kubestateScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubestateScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubestateScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(kubestateScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubestateScrapeInterval = "30s"
        @kubestateScrapeInterval = kubestateScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @kubestateScrapeInterval = kubestateScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubestateScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubestateScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    nodeexporterScrapeInterval = parsedConfig[:nodeexporter]
    if !nodeexporterScrapeInterval.nil?
<<<<<<< HEAD
        @nodeexporterScrapeInterval = nodeexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for nodeexporterScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "nodeexporterScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(nodeexporterScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        nodeexporterScrapeInterval = "30s"
        @nodeexporterScrapeInterval = nodeexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @nodeexporterScrapeInterval = nodeexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for nodeexporterScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "nodeexporterScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    windowsexporterScrapeInterval = parsedConfig[:windowsexporter]
    if !windowsexporterScrapeInterval.nil?
<<<<<<< HEAD
        @windowsexporterScrapeInterval = windowsexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowsexporterScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowsexporterScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(windowsexporterScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        windowsexporterScrapeInterval = "30s"
        @windowsexporterScrapeInterval = windowsexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @windowsexporterScrapeInterval = windowsexporterScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowsexporterScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "windowsexporterScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    windowskubeproxyScrapeInterval = parsedConfig[:windowskubeproxy]
    if !windowskubeproxyScrapeInterval.nil?
<<<<<<< HEAD
        @windowskubeproxyScrapeInterval = windowskubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowskubeproxyScrapeInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowskubeproxyScrapeInterval either not specified or not of type integer")
=======
      matched = MATCHER.match(windowskubeproxyScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        windowskubeproxyScrapeInterval = "30s"
        @windowskubeproxyScrapeInterval = windowskubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @windowskubeproxyScrapeInterval = windowskubeproxyScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for windowskubeproxyScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "windowskubeproxyScrapeInterval override not specified in configmap")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
    end

    prometheusCollectorHealthInterval = parsedConfig[:prometheuscollectorhealth]
    if !prometheusCollectorHealthInterval.nil?
<<<<<<< HEAD
        @prometheusCollectorHealthInterval = prometheusCollectorHealthInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for prometheusCollectorHealthInterval")
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "prometheusCollectorHealthInterval either not specified or not of type integer")
    end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-scrape-interval-list Processing")
=======
      matched = MATCHER.match(prometheusCollectorHealthInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        prometheusCollectorHealthInterval = "30s"
        @prometheusCollectorHealthInterval = prometheusCollectorHealthInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @prometheusCollectorHealthInterval = prometheusCollectorHealthInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for prometheusCollectorHealthInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "prometheusCollectorHealthInterval override not specified in configmap")
    end
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-scrape-interval-settings Processing")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
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
<<<<<<< HEAD
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-scrape-interval-list regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-scrape-interval-list Processing")
=======
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-scrape-interval-settings regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-scrape-interval-settings Processing")
>>>>>>> cfcb0cb4162eb34b2a2bff62963ed8e383df9b7f
