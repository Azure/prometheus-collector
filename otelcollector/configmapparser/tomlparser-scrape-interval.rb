#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
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
@podannotationScrapeInterval = "30s"
@kappiebasicScrapeInterval = "30s"
@kubecontrollermanagerScrapeInterval = "30s"
@kubeschedulerScrapeInterval = "30s"
@kubeapiserverScrapeInterval = "30s"
@clusterautoscalerScrapeInterval = "30s"
@etcdScrapeInterval = "30s"
@prometheusCollectorHealthCcpInterval = "30s"

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-scrape-interval-settings not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-scrape-interval-settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    kubeletScrapeInterval = parsedConfig[:kubelet]
    if !kubeletScrapeInterval.nil?
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
    end

    corednsScrapeInterval = parsedConfig[:coredns]
    if !corednsScrapeInterval.nil?
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
    end

    cadvisorScrapeInterval = parsedConfig[:cadvisor]
    if !cadvisorScrapeInterval.nil?
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
    end

    kubeproxyScrapeInterval = parsedConfig[:kubeproxy]
    if !kubeproxyScrapeInterval.nil?
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
    end

    apiserverScrapeInterval = parsedConfig[:apiserver]
    if !apiserverScrapeInterval.nil?
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
    end

    kubestateScrapeInterval = parsedConfig[:kubestate]
    if !kubestateScrapeInterval.nil?
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
    end

    nodeexporterScrapeInterval = parsedConfig[:nodeexporter]
    if !nodeexporterScrapeInterval.nil?
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
    end

    windowsexporterScrapeInterval = parsedConfig[:windowsexporter]
    if !windowsexporterScrapeInterval.nil?
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
    end

    windowskubeproxyScrapeInterval = parsedConfig[:windowskubeproxy]
    if !windowskubeproxyScrapeInterval.nil?
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
    end

    kappiebasicScrapeInterval = parsedConfig[:kappiebasic]
    if !kappiebasicScrapeInterval.nil?
      matched = MATCHER.match(kappiebasicScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kappiebasicScrapeInterval = "30s"
        @kappiebasicScrapeInterval = kappiebasicScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for kappie")
      else
        @kappiebasicScrapeInterval = kappiebasicScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kappiebasicScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kappiebasicScrapeInterval override not specified in configmap")
    end

    kubecontrollermanagerScrapeInterval = parsedConfig[:"kube-controller-manager"]
    if !kubecontrollermanagerScrapeInterval.nil?
      matched = MATCHER.match(kubecontrollermanagerScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubecontrollermanagerScrapeInterval = "30s"
        @kubecontrollermanagerScrapeInterval = kubecontrollermanagerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for kube-controller-manager")
      else
        @kubecontrollermanagerScrapeInterval = kubecontrollermanagerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubecontrollermanagerScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubecontrollermanagerScrapeInterval override not specified in configmap")
    end

    kubeschedulerScrapeInterval = parsedConfig[:"kube-scheduler"]
    if !kubeschedulerScrapeInterval.nil?
      matched = MATCHER.match(kubeschedulerScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubeschedulerScrapeInterval = "30s"
        @kubeschedulerScrapeInterval = kubeschedulerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for kube-scheduler")
      else
        @kubeschedulerScrapeInterval = kubeschedulerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeschedulerScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeschedulerScrapeInterval override not specified in configmap")
    end

    kubeapiserverScrapeInterval = parsedConfig[:"kube-apiserver"]
    if !kubeapiserverScrapeInterval.nil?
      matched = MATCHER.match(kubeapiserverScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        kubeapiserverScrapeInterval = "30s"
        @kubeapiserverScrapeInterval = kubeapiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for kube-apiserver")
      else
        @kubeapiserverScrapeInterval = kubeapiserverScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for kubeapiserverScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeapiserverScrapeInterval override not specified in configmap")
    end

    clusterautoscalerScrapeInterval = parsedConfig[:"cluster-autoscaler"]
    if !clusterautoscalerScrapeInterval.nil?
      matched = MATCHER.match(clusterautoscalerScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        clusterautoscalerScrapeInterval = "30s"
        @clusterautoscalerScrapeInterval = clusterautoscalerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for cluster-autoscaler")
      else
        @clusterautoscalerScrapeInterval = clusterautoscalerScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for clusterautoscalerScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "clusterautoscalerScrapeInterval override not specified in configmap")
    end

    etcdScrapeInterval = parsedConfig[:"etcd"]
    if !etcdScrapeInterval.nil?
      matched = MATCHER.match(kubeapiserverScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        etcdScrapeInterval = "30s"
        @etcdScrapeInterval = etcdScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s for etcd")
      else
        @etcdScrapeInterval = etcdScrapeInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for etcdScrapeInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "etcdScrapeInterval override not specified in configmap")
    end

    prometheusCollectorHealthInterval = parsedConfig[:prometheuscollectorhealth]
    if !prometheusCollectorHealthInterval.nil?
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

    prometheusCollectorHealthCcpInterval = parsedConfig[:"prometheuscollectorhealth-controlplane"]
    if !prometheusCollectorHealthCcpInterval.nil?
      matched = MATCHER.match(prometheusCollectorHealthCcpInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        prometheusCollectorHealthCcpInterval = "30s"
        @prometheusCollectorHealthCcpInterval = prometheusCollectorHealthCcpInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        @prometheusCollectorHealthCcpInterval = prometheusCollectorHealthCcpInterval
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for prometheusCollectorHealthCcpInterval")
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "prometheusCollectorHealthCcpInterval override not specified in configmap")
    end

    podannotationScrapeInterval = parsedConfig[:podannotations]
    if !podannotationScrapeInterval.nil?
      matched = MATCHER.match(podannotationScrapeInterval)
      if !matched
        # set default scrape interval to 30s if its not in the proper format
        podannotationScrapeInterval = "30s"
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Incorrect regex pattern for duration, set default scrape interval to 30s")
      else
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap scrape settings for podannotationScrapeInterval")
      end
      @podannotationScrapeInterval = podannotationScrapeInterval
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "podannotationScrapeInterval override not specified in configmap")
    end
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-scrape-interval-settings Processing")
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
intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = @podannotationScrapeInterval
intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = @kappiebasicScrapeInterval
intervalHash["KUBE_CONTROLLER_MANAGER_SCRAPE_INTERVAL"] = @kubecontrollermanagerScrapeInterval
intervalHash["KUBE_SCHEDULER_SCRAPE_INTERVAL"] = @kubeschedulerScrapeInterval
intervalHash["KUBE_APISERVER_SCRAPE_INTERVAL"] = @kubeapiserverScrapeInterval
intervalHash["CLUSTER_AUTOSCALER_SCRAPE_INTERVAL"] = @clusterautoscalerScrapeInterval
intervalHash["ETCD_SCRAPE_INTERVAL"] = @etcdScrapeInterval
intervalHash["PROMETHEUS_COLLECTOR_HEALTH_CCP_SCRAPE_INTERVAL"] = @prometheusCollectorHealthCcpInterval

if !file.nil?
  # Close file after writing scrape interval list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(intervalHash.to_yaml)
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-scrape-interval-settings regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-scrape-interval-settings Processing")
