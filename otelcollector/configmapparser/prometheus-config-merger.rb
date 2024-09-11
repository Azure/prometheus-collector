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

@kubeletDefaultFileRsSimple = @defaultPromConfigPathPrefix + "kubeletDefaultRsSimple.yml"
@kubeletDefaultFileRsAdvanced = @defaultPromConfigPathPrefix + "kubeletDefaultRsAdvanced.yml"
@kubeletDefaultFileDs = @defaultPromConfigPathPrefix + "kubeletDefaultDs.yml"
@criDefaultFileDs = @defaultPromConfigPathPrefix + "criDefaultDs.yml"
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
@windowskubeproxyDefaultFileRsSimpleFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsSimple.yml"
@windowskubeproxyDefaultDsFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultDs.yml"
@podannotationsDefaultFile = @defaultPromConfigPathPrefix + "podannotationsDefault.yml"
@windowskubeproxyDefaultRsAdvancedFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsAdvanced.yml"
@kappiebasicDefaultFileDs = @defaultPromConfigPathPrefix + "kappieBasicDefaultDs.yml"
@networkobservabilityRetinaDefaultFileDs = @defaultPromConfigPathPrefix + "networkobservabilityRetinaDefaultDs.yml"
@networkobservabilityHubbleDefaultFileDs = @defaultPromConfigPathPrefix + "networkobservabilityHubbleDefaultDs.yml"
@networkobservabilityCiliumDefaultFileDs = @defaultPromConfigPathPrefix + "networkobservabilityCiliumDefaultDs.yml"

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

def loadIntervalHash
  begin
    @intervalHash = YAML.load_file(@intervalHashFile)
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception in loadIntervalHash for prometheus config: #{errorStr}. Scrape interval will not be used")
  end
end

def UpdateScrapeIntervalConfig(yamlConfigFile, scrapeIntervalSetting)
  begin
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "Updating scrape interval config for #{yamlConfigFile}")
    config = YAML.load(File.read(yamlConfigFile))
    scrapeIntervalConfig = scrapeIntervalSetting

    # Iterate through each scrape config and update scrape interval config
    if !config.nil?
      scrapeConfigs = config["scrape_configs"]
      if !scrapeConfigs.nil? && !scrapeConfigs.empty?
        scrapeConfigs.each { |scfg|
          scrapeCfgs = scfg["scrape_interval"]
          if !scrapeCfgs.nil?
            scfg["scrape_interval"] = scrapeIntervalConfig
          end
        }
        cfgYamlWithScrapeConfig = YAML::dump(config)
        File.open(yamlConfigFile, "w") { |file| file.puts cfgYamlWithScrapeConfig }
      end
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while updating scrape interval config in default target file - #{yamlConfigFile} : #{errorStr}. The Scrape interval will not be used")
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

def AppendRelabelConfig(yamlConfigFile, relabelConfig, keepRegex)
  begin
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "Adding relabel config for #{yamlConfigFile}")
    config = YAML.load(File.read(yamlConfigFile))

    # Iterate through each scrape config and append metric relabel config for keep list
    if !config.nil?
      scrapeConfigs = config["scrape_configs"]
      if !scrapeConfigs.nil? && !scrapeConfigs.empty?
        scrapeConfigs.each { |scfg|
          relabelCfgs = scfg["relabel_configs"]
          if relabelCfgs.nil?
            scfg["relabel_configs"] = relabelConfig
          else
            scfg["relabel_configs"] = relabelCfgs.concat(relabelConfig)
          end
        }
        cfgYamlWithRelabelConfig = YAML::dump(config)
        File.open(yamlConfigFile, "w") { |file| file.puts cfgYamlWithRelabelConfig }
      end
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while appending relabel config in default target file - #{yamlConfigFile} : #{errorStr}. The keep list regex will not be used")
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
      kubeletScrapeInterval = @intervalHash["KUBELET_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          UpdateScrapeIntervalConfig(@kubeletDefaultFileRsSimple, kubeletScrapeInterval)
          if !kubeletMetricsKeepListRegex.nil? && !kubeletMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@kubeletDefaultFileRsSimple, kubeletMetricsKeepListRegex)
          end
          defaultConfigs.push(@kubeletDefaultFileRsSimple)
        elsif windowsDaemonset == true && @sendDSUpMetric == true
          UpdateScrapeIntervalConfig(@kubeletDefaultFileRsAdvancedWindowsDaemonset, kubeletScrapeInterval)
          defaultConfigs.push(@kubeletDefaultFileRsAdvancedWindowsDaemonset)
        elsif @sendDSUpMetric == true
          UpdateScrapeIntervalConfig(@kubeletDefaultFileRsAdvanced, kubeletScrapeInterval)
          defaultConfigs.push(@kubeletDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true && (windowsDaemonset == true || ENV["OS_TYPE"].downcase == "linux")
          UpdateScrapeIntervalConfig(@kubeletDefaultFileDs, kubeletScrapeInterval)
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
        if advancedMode == true && (ENV["OS_TYPE"].downcase == "linux")
          UpdateScrapeIntervalConfig(@criDefaultFileDs, kubeletScrapeInterval)
          contents = File.read(@criDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          contents = contents.gsub("$$OS_TYPE$$", ENV["OS_TYPE"])
          File.open(@criDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@criDefaultFileDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      corednsMetricsKeepListRegex = @regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"]
      corednsScrapeInterval = @intervalHash["COREDNS_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@corednsDefaultFile, corednsScrapeInterval)
      if !corednsMetricsKeepListRegex.nil? && !corednsMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@corednsDefaultFile, corednsMetricsKeepListRegex)
      end
      defaultConfigs.push(@corednsDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      cadvisorMetricsKeepListRegex = @regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"]
      cadvisorScrapeInterval = @intervalHash["CADVISOR_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          UpdateScrapeIntervalConfig(@cadvisorDefaultFileRsSimple, cadvisorScrapeInterval)
          if !cadvisorMetricsKeepListRegex.nil? && !cadvisorMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@cadvisorDefaultFileRsSimple, cadvisorMetricsKeepListRegex)
          end
          defaultConfigs.push(@cadvisorDefaultFileRsSimple)
        elsif @sendDSUpMetric == true
          UpdateScrapeIntervalConfig(@cadvisorDefaultFileRsAdvanced, cadvisorScrapeInterval)
          defaultConfigs.push(@cadvisorDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true && ENV["OS_TYPE"].downcase == "linux"
          UpdateScrapeIntervalConfig(@cadvisorDefaultFileDs, cadvisorScrapeInterval)
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
      kubeproxyScrapeInterval = @intervalHash["KUBEPROXY_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@kubeproxyDefaultFile, kubeproxyScrapeInterval)
      if !kubeproxyMetricsKeepListRegex.nil? && !kubeproxyMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@kubeproxyDefaultFile, kubeproxyMetricsKeepListRegex)
      end
      defaultConfigs.push(@kubeproxyDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      apiserverMetricsKeepListRegex = @regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"]
      apiserverScrapeInterval = @intervalHash["APISERVER_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@apiserverDefaultFile, apiserverScrapeInterval)
      if !apiserverMetricsKeepListRegex.nil? && !apiserverMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@apiserverDefaultFile, apiserverMetricsKeepListRegex)
      end
      defaultConfigs.push(@apiserverDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      kubestateMetricsKeepListRegex = @regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"]
      kubestateScrapeInterval = @intervalHash["KUBESTATE_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@kubestateDefaultFile, kubestateScrapeInterval)
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
      nodeexporterScrapeInterval = @intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        if advancedMode == true && @sendDSUpMetric == true
          UpdateScrapeIntervalConfig(@nodeexporterDefaultFileRsAdvanced, nodeexporterScrapeInterval)
          contents = File.read(@nodeexporterDefaultFileRsAdvanced)
          contents = contents.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
          contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
          File.open(@nodeexporterDefaultFileRsAdvanced, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileRsAdvanced)
        elsif advancedMode == false
          UpdateScrapeIntervalConfig(@nodeexporterDefaultFileRsSimple, nodeexporterScrapeInterval)
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
          UpdateScrapeIntervalConfig(@nodeexporterDefaultFileDs, nodeexporterScrapeInterval)
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

    if !ENV["AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED"].downcase == "true"
      kappiebasicMetricsKeepListRegex = @regexHash["KAPPIEBASIC_METRICS_KEEP_LIST_REGEX"]
      kappiebasicScrapeInterval = @intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        #do nothing -- kappie is not supported to be scrapped automatically outside ds. if needed, customer can disable this ds target, and enable rs scraping thru custom config map
      else #kappie scraping will be turned ON by default only when in MAC/addon mode (for both windows & linux)
        if advancedMode == true  && !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true" #&& ENV["OS_TYPE"].downcase == "linux"
          UpdateScrapeIntervalConfig(@kappiebasicDefaultFileDs, kappiebasicScrapeInterval)
          if !kappiebasicMetricsKeepListRegex.nil? && !kappiebasicMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@kappiebasicDefaultFileDs, kappiebasicMetricsKeepListRegex)
          end
          contents = File.read(@kappiebasicDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@kappiebasicDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@kappiebasicDefaultFileDs)
        end
      end
    end

    if !ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED"].downcase == "true"
      networkobservabilityRetinaMetricsKeepListRegex = @regexHash["NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX"]
      networkobservabilityRetinaScrapeInterval = @intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        #do nothing -- kappie is not supported to be scrapped automatically outside ds. if needed, customer can disable this ds target, and enable rs scraping thru custom config map
      else #networkobservabilityRetina scraping will be turned ON by default only when in MAC/addon mode (for both windows & linux)
        if advancedMode == true  && !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true" #&& ENV["OS_TYPE"].downcase == "linux"
          UpdateScrapeIntervalConfig(@networkobservabilityRetinaDefaultFileDs, networkobservabilityRetinaScrapeInterval)
          if !networkobservabilityRetinaMetricsKeepListRegex.nil? && !networkobservabilityRetinaMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@networkobservabilityRetinaDefaultFileDs, networkobservabilityRetinaMetricsKeepListRegex)
          end
          contents = File.read(@networkobservabilityRetinaDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@networkobservabilityRetinaDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@networkobservabilityRetinaDefaultFileDs)
        end
      end
    end

    if !ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED"].downcase == "true"
      networkobservabilityHubbleMetricsKeepListRegex = @regexHash["NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX"]
      networkobservabilityHubbleScrapeInterval = @intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        #do nothing -- kappie is not supported to be scrapped automatically outside ds. if needed, customer can disable this ds target, and enable rs scraping thru custom config map
      else #networkobservabilityHubble scraping will be turned ON by default only when in MAC/addon mode (for both windows & linux)
        if advancedMode == true  && !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true" && ENV["OS_TYPE"].downcase == "linux"
          UpdateScrapeIntervalConfig(@networkobservabilityHubbleDefaultFileDs, networkobservabilityHubbleScrapeInterval)
          if !networkobservabilityHubbleMetricsKeepListRegex.nil? && !networkobservabilityHubbleMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@networkobservabilityHubbleDefaultFileDs, networkobservabilityHubbleMetricsKeepListRegex)
          end
          contents = File.read(@networkobservabilityHubbleDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@networkobservabilityHubbleDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@networkobservabilityHubbleDefaultFileDs)
        end
      end
    end

    if !ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED"].downcase == "true"
      networkobservabilityCiliumMetricsKeepListRegex = @regexHash["NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX"]
      networkobservabilityCiliumScrapeInterval = @intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType
        #do nothing -- kappie is not supported to be scrapped automatically outside ds. if needed, customer can disable this ds target, and enable rs scraping thru custom config map
      else #networkobservabilityCilium scraping will be turned ON by default only when in MAC/addon mode (for both windows & linux)
        if advancedMode == true  && !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true" && ENV["OS_TYPE"].downcase == "linux"
          UpdateScrapeIntervalConfig(@networkobservabilityCiliumDefaultFileDs, networkobservabilityCiliumScrapeInterval)
          if !networkobservabilityCiliumMetricsKeepListRegex.nil? && !networkobservabilityCiliumMetricsKeepListRegex.empty?
            AppendMetricRelabelConfig(@networkobservabilityCiliumDefaultFileDs, networkobservabilityCiliumMetricsKeepListRegex)
          end
          contents = File.read(@networkobservabilityCiliumDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@networkobservabilityCiliumDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@networkobservabilityCiliumDefaultFileDs)
        end
      end
    end


    # Collector health config should be enabled or disabled for both replicaset and daemonset
    if !ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].downcase == "true"
      prometheusCollectorHealthInterval = @intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@prometheusCollectorHealthDefaultFile, prometheusCollectorHealthInterval)
      defaultConfigs.push(@prometheusCollectorHealthDefaultFile)
    end

    if !ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      winexporterMetricsKeepListRegex = @regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"]
      windowsexporterScrapeInterval = @intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType && advancedMode == false && ENV["OS_TYPE"].downcase == "linux"
        UpdateScrapeIntervalConfig(@windowsexporterDefaultRsSimpleFile, windowsexporterScrapeInterval)
        if !winexporterMetricsKeepListRegex.nil? && !winexporterMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowsexporterDefaultRsSimpleFile, winexporterMetricsKeepListRegex)
        end
        contents = File.read(@windowsexporterDefaultRsSimpleFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowsexporterDefaultRsSimpleFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowsexporterDefaultRsSimpleFile)
      elsif currentControllerType == @daemonsetControllerType && advancedMode == true && windowsDaemonset == true && ENV["OS_TYPE"].downcase == "windows"
        UpdateScrapeIntervalConfig(@windowsexporterDefaultDsFile, windowsexporterScrapeInterval)
        if !winexporterMetricsKeepListRegex.nil? && !winexporterMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowsexporterDefaultDsFile, winexporterMetricsKeepListRegex)
        end
        contents = File.read(@windowsexporterDefaultDsFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowsexporterDefaultDsFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowsexporterDefaultDsFile)
      end
    end

    if !ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].downcase == "true"
      winkubeproxyMetricsKeepListRegex = @regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"]
      windowskubeproxyScrapeInterval = @intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"]
      if currentControllerType == @replicasetControllerType && advancedMode == false && ENV["OS_TYPE"].downcase == "linux"
        UpdateScrapeIntervalConfig(@windowskubeproxyDefaultFileRsSimpleFile, windowskubeproxyScrapeInterval)
        if !winkubeproxyMetricsKeepListRegex.nil? && !winkubeproxyMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowskubeproxyDefaultFileRsSimpleFile, winkubeproxyMetricsKeepListRegex)
        end
        contents = File.read(@windowskubeproxyDefaultFileRsSimpleFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowskubeproxyDefaultFileRsSimpleFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowskubeproxyDefaultFileRsSimpleFile)
      elsif currentControllerType == @daemonsetControllerType && advancedMode == true && windowsDaemonset == true && ENV["OS_TYPE"].downcase == "windows"
        UpdateScrapeIntervalConfig(@windowskubeproxyDefaultDsFile, windowskubeproxyScrapeInterval)
        if !winkubeproxyMetricsKeepListRegex.nil? && !winkubeproxyMetricsKeepListRegex.empty?
          AppendMetricRelabelConfig(@windowskubeproxyDefaultDsFile, winkubeproxyMetricsKeepListRegex)
        end
        contents = File.read(@windowskubeproxyDefaultDsFile)
        contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
        contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
        File.open(@windowskubeproxyDefaultDsFile, "w") { |file| file.puts contents }
        defaultConfigs.push(@windowskubeproxyDefaultDsFile)
      end
    end

    if !ENV["AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"].downcase == "true"  && currentControllerType == @replicasetControllerType
      podannotationNamespacesRegex = ENV["AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"]
      podannotationMetricsKeepListRegex = @regexHash["POD_ANNOTATION_METRICS_KEEP_LIST_REGEX"]
      podannotationScrapeInterval = @intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"]
      UpdateScrapeIntervalConfig(@podannotationsDefaultFile, podannotationScrapeInterval)
      if !podannotationMetricsKeepListRegex.nil? && !podannotationMetricsKeepListRegex.empty?
        AppendMetricRelabelConfig(@podannotationsDefaultFile, podannotationMetricsKeepListRegex)
      end
      if !podannotationNamespacesRegex.nil? && !podannotationNamespacesRegex.empty?
        relabelConfig = [{ "source_labels" => ["__meta_kubernetes_namespace"], "action" => "keep", "regex" => podannotationNamespacesRegex }]
        AppendRelabelConfig(@podannotationsDefaultFile, relabelConfig, podannotationNamespacesRegex)
      end
      defaultConfigs.push(@podannotationsDefaultFile)
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

#this will enforce num labels, label name length & label value length for every scrape job to be with-in azure monitor supported limits
# by injecting these into every custom scrape job's config. For default scrape jobs, this is already included in them. We do this here, so the config validation can happen after we inject these into the custom scrape jobs .
def setLabelLimitsPerScrape(prometheusConfigString)
  customConfig = prometheusConfigString
  ConfigParseErrorLogger.log(LOGGING_PREFIX, "setLabelLimitsPerScrape()")
  begin
    if !customConfig.nil? && !customConfig.empty?
      limitedCustomConfig = YAML.load(customConfig)
      limitedCustomscrapes = limitedCustomConfig["scrape_configs"]
      if !limitedCustomscrapes.nil? && !limitedCustomscrapes.empty?
        limitedCustomscrapes.each { |scrape|
          scrape["label_limit"] = 63
          scrape["label_name_length_limit"] = 511
          scrape["label_value_length_limit"] = 1023
          ConfigParseErrorLogger.log(LOGGING_PREFIX, " Successfully set label limits in custom scrape config for job #{scrape["job_name"]}")
        }
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "Done setting label limits for custom scrape config ...")
        return YAML::dump(limitedCustomConfig)
      else
        ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "No Jobs found to set label limits while processing custom scrape config")
        return prometheusConfigString
      end
    else
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "Nothing to set for label limits while processing custom scrape config")
      return prometheusConfigString
    end
  rescue => errStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception when setting label limits while processing custom scrape config - #{errStr}")
    return prometheusConfigString
  end
end

# Populate default scrape config(s) if AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED is set to false
# and write them as a collector config file, in case the custom config validation fails,
# and we need to fall back to defaults
def writeDefaultScrapeTargetsFile()
  ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start Merging Default and Custom Prometheus Config")
  if !ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].downcase == "false"
    begin
      loadRegexHash
      loadIntervalHash
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
    @kubeletDefaultFileRsSimple, @kubeletDefaultFileRsAdvanced, @kubeletDefaultFileDs, @criDefaultFileDs, @kubeletDefaultFileRsAdvancedWindowsDaemonset,
    @corednsDefaultFile, @cadvisorDefaultFileRsSimple, @cadvisorDefaultFileRsAdvanced, @cadvisorDefaultFileDs, @kubeproxyDefaultFile,
    @apiserverDefaultFile, @kubestateDefaultFile, @nodeexporterDefaultFileRsSimple, @nodeexporterDefaultFileRsAdvanced, @nodeexporterDefaultFileDs,
    @prometheusCollectorHealthDefaultFile, @windowsexporterDefaultRsSimpleFile, @windowsexporterDefaultDsFile,
    @windowskubeproxyDefaultFileRsSimpleFile, @windowskubeproxyDefaultDsFile, @podannotationsDefaultFile
  ]

  defaultFilesArray.each { |currentFile|
    contents = File.read(currentFile)
    contents = contents.gsub("$$SCRAPE_INTERVAL$$", scrapeInterval)
    File.open(currentFile, "w") { |file| file.puts contents }
  }
end

def setGlobalScrapeConfigInDefaultFilesIfExists(configString)
  customConfig = YAML.load(configString)
  # set scrape interval to 30s for updating the default merged config
  scrapeInterval = "30s"
  if customConfig.has_key?("global") && customConfig["global"].has_key?("scrape_interval")
    scrapeInterval = customConfig["global"]["scrape_interval"]
    # Checking to see if the duration matches the pattern specified in the prometheus config
    # Link to documenation with regex pattern -> https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
    matched = /^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$/.match(scrapeInterval)
    if !matched
      # set default global scrape interval to 1m if its not in the proper format
      customConfig["global"]["scrape_interval"] = "1m"
      scrapeInterval = "30s"
    end
  end
  setDefaultFileScrapeInterval(scrapeInterval)
  return YAML::dump(customConfig)
end

prometheusConfigString = parseConfigMap
if !prometheusConfigString.nil? && !prometheusConfigString.empty?
  modifiedPrometheusConfigString = setGlobalScrapeConfigInDefaultFilesIfExists(prometheusConfigString)
  writeDefaultScrapeTargetsFile()
  #set label limits for every custom scrape job, before merging the default & custom config
  labellimitedconfigString = setLabelLimitsPerScrape(modifiedPrometheusConfigString)
  mergeDefaultAndCustomScrapeConfigs(labellimitedconfigString)
else
  setDefaultFileScrapeInterval("30s")
  writeDefaultScrapeTargetsFile()
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Done Merging Default and Custom Prometheus Config")
