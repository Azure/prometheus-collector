#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
@collectorConfigPath = "/opt/microsoft/otelcollector/collector-config.yml"
@otelCustomPromConfigPath = "/opt/promCollectorConfig.yml"
@configVersion = ""
@configSchemaVersion = ""
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"
@supportedSchemaVersion = true
@defaultPromConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@mergedPromConfig = ""
@useDefaultConfig = true

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

puts "****************Start Merging Default and Custom Prometheus Config If Valid********************"
populateDefaultPrometheusConfig
@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]

if (ENV["AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"] != "true")
  if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
    puts "prometheus-config-merger::Supported config schema version found - will be merging custom prometheus config"
  else
    if (File.file?(@configMapMountPath))
      @supportedSchemaVersion = false
      ConfigParseErrorLogger.logError("prometheus-config-merger::unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
    end
  end
end

begin
  # If prometheus custom config is valid, then merge default configs with this and use it as config for otelcollector
  if (ENV["AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"] != "true" && !!@supportedSchemaVersion)
    otelCustomConfig = File.read(@otelCustomPromConfigPath)
    if !otelCustomConfig.empty? && !otelCustomConfig.nil?
      if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
        puts "prometheus-config-merger::Starting to merge default prometheus config values in collector.yml with custom prometheus config"
        collectorConfig = YAML.load(otelCustomConfig)
        promConfig = collectorConfig["receivers"]["prometheus"]["config"] #["scrape_configs"]
        mergedPromConfig = @mergedDefaultConfigs.deep_merge!(promConfig)
        # Doing this instead of gsub because gsub causes ruby's string interpreter to strip escape characters from regex
        collectorConfig["receivers"]["prometheus"]["config"] = mergedPromConfig
        collectorNewConfig = YAML::dump(collectorConfig)
      else
        # If default merged config is empty, then use the prometheus custom config alone
        puts "prometheus-config-merger::merged default configs is empty, so ignoring them"
        collectorNewConfig = otelCustomConfig
      end
      File.open(@collectorConfigPath, "w") { |file| file.puts collectorNewConfig }
      @useDefaultConfig = false
    end
  else
    # If prometheus custom config is invalid or not applied as configmap
    puts "prometheus-config-merger::Starting to merge default prometheus config values in collector template"
    collectorTemplate = YAML.load(File.read(@collectorConfigTemplatePath))
    if !@mergedDefaultConfigs.nil? && !@mergedDefaultConfigs.empty?
      collectorTemplate["receivers"]["prometheus"]["config"] = @mergedDefaultConfigs
      collectorNewConfig = YAML::dump(collectorTemplate)
      File.open(@collectorConfigPath, "w") { |file| file.puts collectorNewConfig }
      @useDefaultConfig = false
    end
  end
rescue => errorStr
  ConfigParseErrorLogger.logError("prometheus-config-merger::Exception while merging otel custom prometheus config and default config - #{errorStr}")
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_prometheus_collector_prometheus_config_env_var", "w")

if !file.nil?
  file.write("export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=#{@useDefaultConfig}\n")
  # Close file after writing all metric collection setting environment variables
  file.close
  puts "****************Done Merging Default and Valid Custom Prometheus Config********************"
else
  puts "Exception while opening file for writing prometheus config environment variables"
  puts "****************Done Merging Default and Valid Custom Prometheus Config********************"
end
