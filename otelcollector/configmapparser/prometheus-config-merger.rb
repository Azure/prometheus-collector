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
@configSchemaVersion = ""
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"
@supportedSchemaVersion = true
@defaultPromConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
@regexHashFile = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
@regexHash = {}
@sendDSUpMetric = false

@kubeletDefaultFileRsSimple = @defaultPromConfigPathPrefix + "kubeletDefaultRsSimple.yml"
@kubeletDefaultFileRsAdvanced = @defaultPromConfigPathPrefix + "kubeletDefaultRsAdvanced.yml"
@kubeletDefaultFileDs = @defaultPromConfigPathPrefix + "kubeletDefaultDs.yml"
@kubeletDefaultFileRsAdvancedWindowsDaemonset = @defaultPromConfigPathPrefix + "kubeletDefaultRsAdvancedWindowsDaemonset.yml"
@corednsDefaultFile = @defaultPromConfigPathPrefix + "corednsDefault.yml"
@cadvisorDefaultFileRsSimple = @defaultPromConfigPathPrefix + "cadvisorDefaultRsSimple.yml"
@cadvisorDefaultFileRsAdvanced = @defaultPromConfigPathPrefix + "cadvisorDefaultRsAdvanced.yml"
@cadvisorDefaultFileDs = @defaultPromConfigPathPrefix + "cadvisorDefaultDs.yml"
@kubeproxyDefaultFile = @defaultPromConfigPathPrefix + "kubeproxyDefault.yml"
@apiserverDefaultFile = @defaultPromConfigPathPrefix + "apiserverDefault.yml"
@kubestateDefaultFile = @defaultPromConfigPathPrefix + "kubestateDefault.yml"
@nodeexporterDefaultFileRsSimple = @defaultPromConfigPathPrefix + "nodeexporterDefaultRsSimple.yml"
@nodeexporterDefaultFileRsAdvanced = @defaultPromConfigPathPrefix + "nodeexporterDefaultRsAdvanced.yml"
@nodeexporterDefaultFileDs = @defaultPromConfigPathPrefix + "nodeexporterDefaultDs.yml"
@prometheusCollectorHealthDefaultFile = @defaultPromConfigPathPrefix + "prometheusCollectorHealth.yml"
@windowsexporterDefaultRsSimpleFile = @defaultPromConfigPathPrefix + "windowsexporterDefaultRsSimple.yml"
@windowsexporterDefaultDsFile = @defaultPromConfigPathPrefix + "windowsexporterDefaultDs.yml"
@windowsexporterDefaultRsAdvancedFile = @defaultPromConfigPathPrefix + "windowsexporterDefaultRsAdvanced.yml"
@windowskubeproxyDefaultFileRsSimpleFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsSimple.yml"
@windowskubeproxyDefaultDsFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultDs.yml"
@windowskubeproxyDefaultRsAdvancedFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsAdvanced.yml"

def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Custom prometheus config exists")
      config = File.read(@configMapMountPath)
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Successfully parsed configmap for prometheus config")
      return config
    else
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "Custom prometheus config does not exist, using only default scrape targets if they are enabled")
      return ""
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing configmap for prometheus config: #{errorStr}. Custom prometheus config will not be used. Please check configmap for errors")
    return ""
  end
end

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

    advancedMode = false #default is false
    windowsDaemonset = false #default is false

    # get current mode (advanced or not...)
    currentMode = ENV["MODE"].strip.downcase
    if currentMode == "advanced"
      advancedMode = true
    end

    # get if windowsdaemonset is enabled or not (ie. WINMODE env = advanced or not...)
    winMode = ENV["WINMODE"].strip.downcase
    if winMode == "advanced" 
      windowsDaemonset = true
    end

    defaultConfigs = []
    if !ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].downcase == "true"
      kubeletMetricsKeepListRegex = @regexHash["KUBELET_METRICS_KEEP_LIST_REGEX"]
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          if !kubeletMetricsKeepListRegex.nil? && !kubeletMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@kubeletDefaultFileRsSimple, kubeletMetricsKeepListRegex)
          end
          defaultConfigs.push(@kubeletDefaultFileRsSimple)
        elsif windowsDaemonset == true && @sendDSUpMetric == true
          defaultConfigs.push(@kubeletDefaultFileRsAdvancedWindowsDaemonset)
        elsif @sendDSUpMetric == true
          defaultConfigs.push(@kubeletDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true && (windowsDaemonset == true || ENV["OS_TYPE"].downcase == "linux")
          if !kubeletMetricsKeepListRegex.nil? && !kubeletMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@kubeletDefaultFileDs, kubeletMetricsKeepListRegex)
          end
          contents = File.read(@kubeletDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          contents = contents.gsub("$$OS_TYPE$$", ENV["OS_TYPE"])
          File.open(@kubeletDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@kubeletDefaultFileDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      corednsMetricsKeepListRegex = @regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"]
      if !corednsMetricsKeepListRegex.nil? && !corednsMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@corednsDefaultFile, corednsMetricsKeepListRegex)
      end
      defaultConfigs.push(@corednsDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      cadvisorMetricsKeepListRegex = @regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"]
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          if !cadvisorMetricsKeepListRegex.nil? && !cadvisorMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@cadvisorDefaultFileRsSimple, cadvisorMetricsKeepListRegex)
          end
          defaultConfigs.push(@cadvisorDefaultFileRsSimple)
        elsif @sendDSUpMetric == true
          defaultConfigs.push(@cadvisorDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true && ENV["OS_TYPE"].downcase == "linux"
          if !cadvisorMetricsKeepListRegex.nil? && !cadvisorMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@cadvisorDefaultFileDs, cadvisorMetricsKeepListRegex)
          end
          contents = File.read(@cadvisorDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@cadvisorDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@cadvisorDefaultFileDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      kubeproxyMetricsKeepListRegex = @regexHash["KUBEPROXY_METRICS_KEEP_LIST_REGEX"]
      if !kubeproxyMetricsKeepListRegex.nil? && !kubeproxyMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@kubeproxyDefaultFile, kubeproxyMetricsKeepListRegex)
      end
      defaultConfigs.push(@kubeproxyDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      apiserverMetricsKeepListRegex = @regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"]
      if !apiserverMetricsKeepListRegex.nil? && !apiserverMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@apiserverDefaultFile, apiserverMetricsKeepListRegex)
      end
      defaultConfigs.push(@apiserverDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      kubestateMetricsKeepListRegex = @regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"]
      if !kubestateMetricsKeepListRegex.nil? && !kubestateMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@kubestateDefaultFile, kubestateMetricsKeepListRegex)
      end
      contents = File.read(@kubestateDefaultFile)
      contents = contents.gsub("$$KUBE_STATE_NAME$$", ENV["KUBE_STATE_NAME"])
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@kubestateDefaultFile, "w") { |file| file.puts contents }
      defaultConfigs.push(@kubestateDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      nodeexporterMetricsKeepListRegex = @regexHash["NODEEXPORTER_METRICS_KEEP_LIST_REGEX"]

      if currentControllerType == @replicasetControllerType
        if advancedMode == true && @sendDSUpMetric == true
          contents = File.read(@nodeexporterDefaultFileRsAdvanced)
          contents = contents.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
          contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
          File.open(@nodeexporterDefaultFileRsAdvanced, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileRsAdvanced)
        elsif advancedMode == false
          if !nodeexporterMetricsKeepListRegex.nil? && !nodeexporterMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@nodeexporterDefaultFileRsSimple, nodeexporterMetricsKeepListRegex)
          end
          contents = File.read(@nodeexporterDefaultFileRsSimple)
          contents = contents.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
          contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
          File.open(@nodeexporterDefaultFileRsSimple, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileRsSimple)
        end
      else
        if advancedMode == true && ENV["OS_TYPE"].downcase == "linux"
          if !nodeexporterMetricsKeepListRegex.nil? && !nodeexporterMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@nodeexporterDefaultFileDs, nodeexporterMetricsKeepListRegex)
          end
          contents = File.read(@nodeexporterDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_EXPORTER_TARGETPORT$$", ENV["NODE_EXPORTER_TARGETPORT"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@nodeexporterDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileDs)
        end
      end
    end

    # Collector health config should be enabled or disabled for both replicaset and daemonset
    if !ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].downcase == "true"
      defaultConfigs.push(@prometheusCollectorHealthDefaultFile)
    end

    if !ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      winexporterMetricsKeepListRegex = @regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"]
      if currentControllerType == @replicasetControllerType && advancedMode == false && ENV["OS_TYPE"].downcase == "linux"
        if !winexporterMetricsKeepListRegex.nil? && !winexporterMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowsexporterDefaultRsSimpleFile, winexporterMetricsKeepListRegex)
        end
        contents = File.read(@windowsexporterDefaultRsSimpleFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowsexporterDefaultRsSimpleFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowsexporterDefaultRsSimpleFile)
      elsif currentControllerType == @daemonsetControllerType && advancedMode == true && windowsDaemonset == true && ENV["OS_TYPE"].downcase == "windows"
        if !winexporterMetricsKeepListRegex.nil? && !winexporterMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowsexporterDefaultDsFile, winexporterMetricsKeepListRegex)
        end
        contents = File.read(@windowsexporterDefaultDsFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowsexporterDefaultDsFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowsexporterDefaultDsFile)
      
      # If advanced mode and windows daemonset are enabled, only the up metric is needed from the replicaset
      elsif currentControllerType == @replicasetControllerType && advancedMode == true && windowsDaemonset == true && @sendDSUpMetric == true && ENV["OS_TYPE"].downcase == "linux"
        defaultConfigs.push(@windowsexporterDefaultRsAdvancedFile)
      
      # If advanced mode is enabled, but not the windows daemonset, scrape windows kubelet from the replicaset as if it's simple mode
      elsif currentControllerType == @replicasetControllerType && advancedMode == true && windowsDaemonset == false && ENV["OS_TYPE"].downcase == "linux"
        if !winexporterMetricsKeepListRegex.nil? && !winexporterMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowsexporterDefaultRsSimpleFile, winexporterMetricsKeepListRegex)
        end
        defaultConfigs.push(@windowsexporterDefaultRsSimpleFile)
      end
    end

    if !ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].downcase == "true"
      winkubeproxyMetricsKeepListRegex = @regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"]
      if currentControllerType == @replicasetControllerType && advancedMode == false && ENV["OS_TYPE"].downcase == "linux"
        if !winkubeproxyMetricsKeepListRegex.nil? && !winkubeproxyMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowskubeproxyDefaultFileRsSimpleFile, winkubeproxyMetricsKeepListRegex)
        end
        contents = File.read(@windowskubeproxyDefaultFileRsSimpleFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowskubeproxyDefaultFileRsSimpleFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowskubeproxyDefaultFileRsSimpleFile)
      elsif currentControllerType == @daemonsetControllerType && advancedMode == true && windowsDaemonset == true && ENV["OS_TYPE"].downcase == "windows"
        if !winkubeproxyMetricsKeepListRegex.nil? && !winkubeproxyMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowskubeproxyDefaultDsFile, winkubeproxyMetricsKeepListRegex)
        end
        contents = File.read(@windowskubeproxyDefaultDsFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowskubeproxyDefaultDsFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowskubeproxyDefaultDsFile)
      
      # If advanced mode and windows daemonset are enabled, only the up metric is needed from the replicaset
      elsif currentControllerType == @replicasetControllerType && advancedMode == true && windowsDaemonset == true && @sendDSUpMetric == true && ENV["OS_TYPE"].downcase == "linux"
        defaultConfigs.push(@windowskubeproxyDefaultRsAdvancedFile)

      # If advanced mode is enabled, but not the windows daemonset, scrape windows kubelet from the replicaset as if it's simple mode
      elsif currentControllerType == @replicasetControllerType && advancedMode == true && windowsDaemonset == false && ENV["OS_TYPE"].downcase == "linux"
        if !winkubeproxyMetricsKeepListRegex.nil? && !winkubeproxyMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowskubeproxyDefaultRsSimpleFile, winkubeproxyMetricsKeepListRegex)
        end
        defaultConfigs.push(@windowskubeproxyDefaultFileRsSimpleFile)
      end
    end

    @mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while merging default scrape targets - #{errorStr}. No default scrape tragets will be included")
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

def mergeDefaultAndCustomScrapeConfigs(customPromConfig)
  mergedConfigYaml = ""
  begin
    if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Merging default and custom scrape configs")
      customPrometheusConfig = YAML.load(customPromConfig)
      mergedConfigs = @mergedDefaultConfigs.deep_merge!(customPrometheusConfig)
      mergedConfigYaml = YAML::dump(mergedConfigs)
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Done merging default scrape config(s) with custom prometheus config, writing them to file")
    else
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "The merged default scrape config is nil or empty, using only custom scrape config")
      mergedConfigYaml = customPromConfig
    end
    File.open(@promMergedConfigPath, "w") { |file| file.puts mergedConfigYaml }
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while merging default and custom scrape configs- #{errorStr}")
  end
end

ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start Merging Default and Custom Prometheus Config")
# Populate default scrape config(s) if AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED is set to false
# and write them as a collector config file, in case the custom config validation fails,
# and we need to fall back to defaults
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

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]

if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  prometheusConfigString = parseConfigMap
  if !prometheusConfigString.nil? && !prometheusConfigString.empty?
    mergeDefaultAndCustomScrapeConfigs(prometheusConfigString)
  end
else
  if (File.file?(@configMapMountPath))
    @supportedSchemaVersion = false
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Done Merging Default and Custom Prometheus Config")
