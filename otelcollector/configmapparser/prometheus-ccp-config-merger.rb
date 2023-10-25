#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "prometheus-config-merger"
@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@promMergedConfigPath = "/opt/promMergedConfig.yml"
@mergedDefaultConfigPath = "/opt/defaultsMergedConfig.yml"
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"
@supportedSchemaVersion = true
@defaultPromConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
@regexHashFile = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
@regexHash = {}
@sendDSUpMetric = false
@intervalHashFile = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
@intervalHash = {}

@controlplane_apiserver_default_file = @defaultPromConfigPathPrefix + "controlplane_apiserver.yml"
@controlplane_kube_scheduler_default_file = @defaultPromConfigPathPrefix + "controlplane_kube_scheduler.yml"
@controlplane_kube_controller_manager_default_file = @defaultPromConfigPathPrefix + "controlplane_kube_controller_manager.yml"
@controlplane_cluster_autoscaler_default_file = @defaultPromConfigPathPrefix + "controlplane_cluster_autoscaler.yml"
@controlplane_etcd_default_file = @defaultPromConfigPathPrefix + "controlplane_etcd.yml"
@controlplane_prometheuscollectorhealth_default_file = @defaultPromConfigPathPrefix + "controlplane_prometheuscollectorhealth.yml"

def loadRegexHash
  begin
    @regexHash = YAML.load_file(@regexHashFile)
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception in loadRegexHash for prometheus config: #{errorStr}. Keep list regexes will not be used")
  end
end

def AppendMetricRelabelConfig(yamlConfigFile, keepListRegex)
  begin
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "Adding keep list regex or minimal ingestion regex for #{yamlConfigFile}")
    config = YAML.load(File.read(yamlConfigFile))
    keepListMetricRelabelConfig = [{ "source_labels" => ["__name__"], "action" => "keep", "regex" => keepListRegex }]

    # Iterate through each scrape config and append metric relabel config for keep list
    if !config.nil?
      scrapeConfigs = config["scrape_configs"]
      if !scrapeConfigs.nil? && !scrapeConfigs.empty?
        scrapeConfigs.each { |scfg|
          metricRelabelCfgs = scfg["metric_relabel_configs"]
          if metricRelabelCfgs.nil?
            scfg["metric_relabel_configs"] = keepListMetricRelabelConfig
          else
            scfg["metric_relabel_configs"] = metricRelabelCfgs.concat(keepListMetricRelabelConfig)
          end
        }
        cfgYamlWithMetricRelabelConfig = YAML::dump(config)
        File.open(yamlConfigFile, "w") { |file| file.puts cfgYamlWithMetricRelabelConfig }
      end
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while appending metric relabel config in default target file - #{yamlConfigFile} : #{errorStr}. The keep list regex will not be used")
  end
end

# Get the list of default configs to be included in the otel's prometheus config
def populateDefaultPrometheusConfig
  begin
    # check if running in daemonset or replicaset
    currentControllerType = ENV["CONTROLLER_TYPE"].strip.downcase
    
    defaultConfigs = []
    if !ENV["AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      kubeControllerManagerMetricsKeepListRegex = @regexHash["CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX"]
      if !kubeControllerManagerMetricsKeepListRegex.nil? && !kubeControllerManagerMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@controlplane_kube_controller_manager_default_file, kubeControllerManagerMetricsKeepListRegex)
      end
      contents = File.read(@controlplane_kube_controller_manager_default_file)
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@controlplane_kube_controller_manager_default_file, "w") { |file| file.puts contents }
      defaultConfigs.push(@controlplane_kube_controller_manager_default_file)
    end

    if !ENV["AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      controlplaneKubeSchedulerKeepListRegex = @regexHash["CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX"]

      if !controlplaneKubeSchedulerKeepListRegex.nil? && !controlplaneKubeSchedulerKeepListRegex.empty?
        AppendMetricRelabelConfig(@controlplane_kube_scheduler_default_file, controlplaneKubeSchedulerKeepListRegex)
      end
      contents = File.read(@controlplane_kube_scheduler_default_file)
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@controlplane_kube_scheduler_default_file, "w") { |file| file.puts contents }
      defaultConfigs.push(@controlplane_kube_scheduler_default_file)
    end

    if !ENV["AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      controlplaneApiserverKeepListRegex = @regexHash["CONTROLPLANE_APISERVER_KEEP_LIST_REGEX"]
      
      if !controlplaneApiserverKeepListRegex.nil? && !controlplaneApiserverKeepListRegex.empty?
        AppendMetricRelabelConfig(@controlplane_apiserver_default_file, controlplaneApiserverKeepListRegex)
      end
      contents = File.read(@controlplane_apiserver_default_file)
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@controlplane_apiserver_default_file, "w") { |file| file.puts contents }
      defaultConfigs.push(@controlplane_apiserver_default_file)
    end

    if !ENV["AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      controlplaneClusterAutoscalerKeepListRegex = @regexHash["CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX"]
      
      if !controlplaneClusterAutoscalerKeepListRegex.nil? && !controlplaneClusterAutoscalerKeepListRegex.empty?
        AppendMetricRelabelConfig(@controlplane_cluster_autoscaler_default_file, controlplaneClusterAutoscalerKeepListRegex)
      end
      contents = File.read(@controlplane_cluster_autoscaler_default_file)
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@controlplane_cluster_autoscaler_default_file, "w") { |file| file.puts contents }
      defaultConfigs.push(@controlplane_cluster_autoscaler_default_file)
    end

    if !ENV["AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      controlplaneEtcdKeepListRegex = @regexHash["CONTROLPLANE_ETCD_KEEP_LIST_REGEX"]
      
      if !controlplaneEtcdKeepListRegex.nil? && !controlplaneEtcdKeepListRegex.empty?
        AppendMetricRelabelConfig(@controlplane_etcd_default_file, controlplaneEtcdKeepListRegex)
      end
      contents = File.read(@controlplane_etcd_default_file)
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@controlplane_etcd_default_file, "w") { |file| file.puts contents }
      defaultConfigs.push(@controlplane_etcd_default_file)
    end

    if !ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].downcase == "true"
      defaultConfigs.push(@controlplane_prometheuscollectorhealth_default_file)
    end

    @mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while merging default scrape targets - #{errorStr}. No default scrape targets will be included")
    @mergedDefaultConfigs = ""
  end
end

def mergeDefaultScrapeConfigs(defaultScrapeConfigs)
  mergedDefaultConfigs = ""
  begin
    if defaultScrapeConfigs.length > 0
      mergedDefaultConfigs = YAML.load("scrape_configs:")
      # Load each of the default scrape configs and merge them
      defaultScrapeConfigs.each { |defaultScrapeConfig|
        # Load yaml from default config
        defaultConfigYaml = YAML.load(File.read(defaultScrapeConfig))
        mergedDefaultConfigs = mergedDefaultConfigs.deep_merge!(defaultConfigYaml)
      }
    end
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "Done merging #{defaultScrapeConfigs.length} default prometheus config(s)")
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while adding default scrape config- #{errorStr}. No default scrape targets will be included")
    mergedDefaultConfigs = ""
  end
  return mergedDefaultConfigs
end

def writeDefaultScrapeTargetsFile()
  ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start Updating Default Prometheus Config")
  if !ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].downcase == "false"
    begin
      loadRegexHash
      populateDefaultPrometheusConfig
      if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Starting to merge default prometheus config values in collector template as backup")
        mergedDefaultConfigYaml = YAML::dump(@mergedDefaultConfigs)
        File.open(@mergedDefaultConfigPath, "w") { |file| file.puts mergedDefaultConfigYaml }
      end
    rescue => errorStr
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Error while populating default scrape targets and writing them to the default scrape targets file")
    end
  end
end

def setDefaultFileScrapeInterval(scrapeInterval)
  defaultFilesArray = [
    @controlplane_apiserver_default_file, @controlplane_kube_scheduler_default_file, @controlplane_kube_controller_manager_default_file,
    @controlplane_cluster_autoscaler_default_file, @controlplane_etcd_default_file, @controlplane_prometheuscollectorhealth_default_file
  ]

  defaultFilesArray.each { |currentFile|
    contents = File.read(currentFile)
    contents = contents.gsub("$$SCRAPE_INTERVAL$$", scrapeInterval)
    File.open(currentFile, "w") { |file| file.puts contents }
  }
end

setDefaultFileScrapeInterval("30s")
writeDefaultScrapeTargetsFile()

ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Done creating default targets file")
