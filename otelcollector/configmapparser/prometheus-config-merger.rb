#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
@collectorConfigWithDefaultPromConfigs = "/opt/microsoft/otelcollector/collector-config-default.yml"
@promMergedConfigPath = "/opt/promMergedConfig.yml"
@configSchemaVersion = ""
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"
@supportedSchemaVersion = true
@defaultPromConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"

@kubeletDefaultFileRsSimple = @defaultPromConfigPathPrefix + "kubeletDefaultRsSimple.yml"
@kubeletDefaultFileRsAdvanced = @defaultPromConfigPathPrefix + "kubeletDefaultRsAdvanced.yml"
@kubeletDefaultFileDs = @defaultPromConfigPathPrefix + "kubeletDefaultDs.yml"
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
@windowsexporterDefaultFile = @defaultPromConfigPathPrefix + "windowsexporterDefault.yml"
@windowskubeproxyDefaultFile = @defaultPromConfigPathPrefix + "windowskubeproxyDefault.yml"

def parseConfigMap
  begin
    # Check to see if config map is created
    puts "prometheus-config-merger::Checking to see if prometheus-config file exists: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "prometheus-config-merger::configmap prometheus-collector-configmap for prometheus config mounted, parsing values"
      config = File.read(@configMapMountPath)
      puts "prometheus-config-merger::Successfully parsed mounted config map"
      return config
    else
      puts "prometheus-config-merger::configmap prometheus-collector-configmap for prometheus config not mounted, using defaults"
      return ""
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for prometheus config : #{errorStr}, using defaults, please check config map for errors")
    return ""
  end
end

# Get the list of default configs to be included in the otel's prometheus config
def populateDefaultPrometheusConfig
  begin
    # check if running in daemonset or replicaset
    currentControllerType = ENV["CONTROLLER_TYPE"].strip.downcase

    advancedMode = false #default is false

    # get current mode (advanced or not...)
    currentMode = ENV["MODE"].strip.downcase
    if currentMode == "advanced"
      advancedMode = true
    end

    defaultConfigs = []
    if !ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          defaultConfigs.push(@kubeletDefaultFileRsSimple)
        else
          defaultConfigs.push(@kubeletDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true
          contents = File.read(@kubeletDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@kubeletDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@kubeletDefaultFileDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@corednsDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          defaultConfigs.push(@cadvisorDefaultFileRsSimple)
        else
          defaultConfigs.push(@cadvisorDefaultFileRsAdvanced)
        end
      else
        if advancedMode == true
          contents = File.read(@cadvisorDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          File.open(@cadvisorDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@cadvisorDefaultFileDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@kubeproxyDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@apiserverDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      contents = File.read(@kubestateDefaultFile)
      contents = contents.gsub("$$KUBE_STATE_NAME$$", ENV["KUBE_STATE_NAME"])
      contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      File.open(@kubestateDefaultFile, "w") { |file| file.puts contents }
      defaultConfigs.push(@kubestateDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == true
          contents = File.read(@nodeexporterDefaultFileRsAdvanced)
          contents = contents.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
          contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
          File.open(@nodeexporterDefaultFileRsAdvanced, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileRsAdvanced)
        else
          contents = File.read(@nodeexporterDefaultFileRsSimple)
          contents = contents.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
          contents = contents.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
          File.open(@nodeexporterDefaultFileRsSimple, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileRsSimple)
        end
      else
        if advancedMode == true
          contents = File.read(@nodeexporterDefaultFileDs)
          contents = contents.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          contents = contents.gsub("$$NODE_EXPORTER_TARGETPORT$$", ENV["NODE_EXPORTER_TARGETPORT"])
          File.open(@nodeexporterDefaultFileDs, "w") { |file| file.puts contents }
          defaultConfigs.push(@nodeexporterDefaultFileDs)
        end
      end
    end

    # Collector health config should be enabled or disabled for both replicaset and daemonset
    if !ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"].downcase == "true"
      defaultConfigs.push(@prometheusCollectorHealthDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@windowsexporterDefaultFile)
    end
    if !ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@windowskubeproxyDefaultFile)
    end
    @mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
  rescue => errorStr
    ConfigParseErrorLogger.logError("prometheus-config-merger::Exception while merging default prometheus configs - #{errorStr}, using defaults")
    @mergedDefaultConfigs = ""
  end
end

def mergeDefaultScrapeConfigs(defaultScrapeConfigs)
  puts "prometheus-config-merger::Adding default scrape configs..."
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
    puts "prometheus-config-merger::Done merging #{defaultScrapeConfigs.length} default prometheus config(s)"
  rescue => errorStr
    ConfigParseErrorLogger.logError("prometheus-config-merger::Exception while adding default scrape config- #{errorStr}, using defaults")
    mergedDefaultConfigs = ""
  end
  return mergedDefaultConfigs
end

def mergeDefaultAndCustomScrapeConfigs(customPromConfig)
  mergedConfigYaml = ""
  begin
    if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
      puts "prometheus-config-merger::Merging default and custom scrape configs..."
      customPrometheusConfig = YAML.load(customPromConfig)
      mergedConfigs = @mergedDefaultConfigs.deep_merge!(customPrometheusConfig)
      mergedConfigYaml = YAML::dump(mergedConfigs)
      puts "prometheus-config-merger::Done merging default scrape config(s) with custom prometheus config(s), writing them to file"
    else
      puts "prometheus-config-merger::Merged default scrape config nil or empty, using custom scrape configs to write to file..."
      mergedConfigYaml = customPromConfig
    end
    File.open(@promMergedConfigPath, "w") { |file| file.puts mergedConfigYaml }
  rescue => errorStr
    ConfigParseErrorLogger.logError("prometheus-config-merger::Exception while merging default and custom scrape configs- #{errorStr}, using defaults")
  end
end

puts "****************Start Merging Default and Custom Prometheus Config********************"

# Populate default scrape config(s) if AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED is set to false
# and write them as a collector config file, in case the custom config validation fails,
# and we need to fall back to defaults
if !ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"].downcase == "false"
  begin
    populateDefaultPrometheusConfig
    if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
      puts "prometheus-config-merger::Starting to merge default prometheus config values in collector template as backup"
      collectorTemplate = YAML.load(File.read(@collectorConfigTemplatePath))
      collectorTemplate["receivers"]["prometheus"]["config"] = @mergedDefaultConfigs
      collectorNewConfig = YAML::dump(collectorTemplate)
      File.open(@collectorConfigWithDefaultPromConfigs, "w") { |file| file.puts collectorNewConfig }
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("prometheus-config-merger::Error while populating defaults and writing them to the defaults file")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]

if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  puts "prometheus-config-merger::Supported config schema version found - will be parsing custom prometheus config"
  prometheusConfigString = parseConfigMap
  if !prometheusConfigString.nil? && !prometheusConfigString.empty?
    mergeDefaultAndCustomScrapeConfigs(prometheusConfigString)
  end
else
  if (File.file?(@configMapMountPath))
    @supportedSchemaVersion = false
    ConfigParseErrorLogger.logError("prometheus-config-merger::unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end
puts "****************Done Merging Default and Custom Prometheus Config********************"
