#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-scrape-settings"

@configMapMountPath = "/etc/config/settings/default-scrape-settings-enabled"
@configVersion = ""
@configSchemaVersion = ""

@kubeletEnabled = true
@corednsEnabled = true
@cadvisorEnabled = true
@kubeproxyEnabled = true
@apiserverEnabled = true
@kubestateEnabled = true
@nodeexporterEnabled = true
@prometheusCollectorHealthEnabled = true
@prometheusCollectorHealthCcpEnabled = true
@podannotationEnabled = false
@windowsexporterEnabled = false
@windowskubeproxyEnabled = false
@kappiebasicEnabled = true
@kubecontrollermanagerEnabled = false
@kubeschedulerEnabled = false
@kubeapiserverEnabled = false
@clusterautoscalerEnabled = false
@etcdEnabled = false
@noDefaultsEnabled = false
@sendDSUpMetric = false

# Back-end flag to enable/disable ccp metrics collector deployment
@isCcpMetricsDeploymentEnabled = false
if !ENV['CCP_METRICS_ENABLED'].nil? && ENV["CCP_METRICS_ENABLED"].strip.downcase == "true"
  @isCcpMetricsDeploymentEnabled = true
end

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "configmapprometheus-collector-configmap for scrape targets not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default scrape settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    if !parsedConfig[:kubelet].nil?
      @kubeletEnabled = parsedConfig[:kubelet]
      puts "config::Using configmap scrape settings for kubelet: #{@kubeletEnabled}"
    end
    if !parsedConfig[:coredns].nil?
      @corednsEnabled = parsedConfig[:coredns]
      puts "config::Using configmap scrape settings for coredns: #{@corednsEnabled}"
    end
    if !parsedConfig[:cadvisor].nil?
      @cadvisorEnabled = parsedConfig[:cadvisor]
      puts "config::Using configmap scrape settings for cadvisor: #{@cadvisorEnabled}"
    end
    if !parsedConfig[:kubeproxy].nil?
      @kubeproxyEnabled = parsedConfig[:kubeproxy]
      puts "config::Using configmap scrape settings for kubeproxy: #{@kubeproxyEnabled}"
    end
    if !parsedConfig[:apiserver].nil?
      @apiserverEnabled = parsedConfig[:apiserver]
      puts "config::Using configmap scrape settings for apiserver: #{@apiserverEnabled}"
    end
    if !parsedConfig[:kubestate].nil?
      @kubestateEnabled = parsedConfig[:kubestate]
      puts "config::Using configmap scrape settings for kubestate: #{@kubestateEnabled}"
    end
    if !parsedConfig[:nodeexporter].nil?
      @nodeexporterEnabled = parsedConfig[:nodeexporter]
      puts "config::Using configmap scrape settings for nodeexporter: #{@nodeexporterEnabled}"
    end
    if !parsedConfig[:prometheuscollectorhealth].nil?
      @prometheusCollectorHealthEnabled = parsedConfig[:prometheuscollectorhealth]
      puts "config::Using configmap scrape settings for prometheuscollectorhealth: #{@prometheusCollectorHealthEnabled}"
    end
    if !parsedConfig[:windowsexporter].nil?
      @windowsexporterEnabled = parsedConfig[:windowsexporter]
      puts "config::Using configmap scrape settings for windowsexporter: #{@windowsexporterEnabled}"
    end
    if !parsedConfig[:windowskubeproxy].nil?
      @windowskubeproxyEnabled = parsedConfig[:windowskubeproxy]
      puts "config::Using configmap scrape settings for windowskubeproxy: #{@windowskubeproxyEnabled}"
    end
    if !ENV['AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX'].nil? && !ENV['AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX'].empty?
      @podannotationEnabled = "true"
      puts "config::Using configmap scrape settings for podannotations: #{@podannotationEnabled}"
    end
    if !parsedConfig[:kappiebasic].nil?
      @kappiebasicEnabled = parsedConfig[:kappiebasic]
      puts "config::Using configmap scrape settings for kappiebasic: #{@kappiebasicEnabled}"
    end
    if !parsedConfig[:"kube-controller-manager"].nil?
      @kubecontrollermanagerEnabled = parsedConfig[:"kube-controller-manager"]
      puts "config::Using configmap scrape settings for kube-controller-manager: #{@kubecontrollermanagerEnabled}"
    end
    if !parsedConfig[:"kube-scheduler"].nil?
      @kubeschedulerEnabled = parsedConfig[:"kube-scheduler"]
      puts "config::Using configmap scrape settings for kube-scheduler: #{@kubeschedulerEnabled}"
    end
    if !parsedConfig[:"kube-apiserver"].nil?
      @kubeapiserverEnabled = parsedConfig[:"kube-apiserver"]
      puts "config::Using configmap scrape settings for kube-apiserver: #{@kubeapiserverEnabled}"
    end
    if !parsedConfig[:"cluster-autoscaler"].nil?
      @clusterautoscalerEnabled = parsedConfig[:"cluster-autoscaler"]
      puts "config::Using configmap scrape settings for cluster-autoscaler: #{@clusterautoscalerEnabled}"
    end
    if !parsedConfig[:"etcd"].nil?
      @etcdEnabled = parsedConfig[:"etcd"]
      puts "config::Using configmap scrape settings for etcd: #{@etcdEnabled}"
    end
    if !parsedConfig[:"prometheuscollectorhealth-controlplane"].nil?
      @prometheusCollectorHealthCcpEnabled = parsedConfig[:"prometheuscollectorhealth-controlplane"]
      puts "config::Using configmap scrape settings for prometheuscollectorhealth-controlplane: #{@prometheusCollectorHealthCcpEnabled}"
    end

    windowsDaemonset = false
    if ENV["WINMODE"].nil? && ENV["WINMODE"].strip.downcase == "advanced"
      windowsDaemonset = true
    end

    ccpmetricsEnabled = @isCcpMetricsDeploymentEnabled && (@kubecontrollermanagerEnabled || @kubeschedulerEnabled || @kubeapiserverEnabled || @clusterautoscalerEnabled || @etcdEnabled || @prometheusCollectorHealthCcpEnabled)
    if ENV["MODE"].nil? && ENV["MODE"].strip.downcase == "advanced"
      controllerType = ENV["CONTROLLER_TYPE"]
      if controllerType == "DaemonSet" && ENV["OS_TYPE"].downcase == "windows" && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@kubeletEnabled && !@prometheusCollectorHealthEnabled && !@kappiebasicEnabled
        @noDefaultsEnabled = true
      elsif controllerType == "DaemonSet" && ENV["OS_TYPE"].downcase == "linux" && !@kubeletEnabled && !@cadvisorEnabled && !@nodeexporterEnabled && !@prometheusCollectorHealthEnabled && !kappiebasicEnabled
        @noDefaultsEnabled = true
      elsif controllerType == "ReplicaSet" && @sendDsUpMetric && !@kubeletEnabled && !@cadvisorEnabled && !@nodeexporterEnabled && !@corednsEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@prometheusCollectorHealthEnabled && !@podannotationEnabled && !ccpmetricsEnabled
        @noDefaultsEnabled = true
      elsif controllerType == "ReplicaSet" && !@sendDsUpMetric && windowsDaemonset && !@corednsEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@prometheusCollectorHealthEnabled && !@podannotationEnabled && !ccpmetricsEnabled
        @noDefaultsEnabled = true
      # Windows daemonset is not enabled so Windows kube-proxy and node-exporter are scraped from replica
      elsif controllerType == "ReplicaSet" && !@sendDsUpMetric && !windowsDaemonset && !@corednsEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@prometheusCollectorHealthEnabled && !@podannotationEnabled && !ccpmetricsEnabled
        @noDefaultsEnabled = true
      end
    elsif !@kubeletEnabled && !@corednsEnabled && !@cadvisorEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@nodeexporterEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@prometheusCollectorHealthEnabled && !@podannotationEnabled && !ccpmetricsEnabled
      @noDefaultsEnabled = true
    end
    if @noDefaultsEnabled
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "No default scrape configs enabled")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for default scrape settings - #{errorStr}, using defaults, please check config map for errors")
  end
end

# Use parser to parse the configmap toml file to a ruby structure
def disableScrapeTargetsByDeployment
  if @isCcpMetricsDeploymentEnabled
    ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "CCP_METRICS mode is enabled. Disable targets from customer nodes after config map processing....")

    @kubestateEnabled = false
    @cadvisorEnabled = false
    @kubeletEnabled = false
    @kappiebasicEnabled = false
    @nodeexporterEnabled = false
    @corednsEnabled = false
    @kubeproxyEnabled = false
    @apiserverEnabled = false
    @prometheusCollectorHealthEnabled = false
  else
    ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "CCP_METRICS mode is disabled. Disable targets from Customer Control Plane after config map processing....")

    @clusterautoscalerEnabled = false
    @kubecontrollermanagerEnabled = false
    @kubeschedulerEnabled = false
    @kubeapiserverEnabled = false
    @etcdEnabled = false
    @prometheusCollectorHealthCcpEnabled = false
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-scrape-settings Processing")
# set default targets for MAC mode
if !@isCcpMetricsDeploymentEnabled && !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true"
  ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "MAC mode is enabled. Only enabling targets kubestate,cadvisor,kubelet,kappiebasic & nodeexporter for linux before config map processing....")

  @corednsEnabled = false
  @kubeproxyEnabled = false
  @apiserverEnabled = false
  @prometheusCollectorHealthEnabled = false
end
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
# disable targets if ccp metrics enabled
disableScrapeTargetsByDeployment


# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_default_scrape_settings_env_var", "w")

$export = "export "
if !ENV['OS_TYPE'].nil? && ENV['OS_TYPE'].downcase == "windows"
  $export = "";
end

if !file.nil?
  file.write($export + "AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED=#{@kubeletEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED=#{@corednsEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED=#{@cadvisorEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED=#{@kubeproxyEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED=#{@apiserverEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED=#{@kubestateEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED=#{@nodeexporterEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=#{@noDefaultsEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED=#{@prometheusCollectorHealthEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED=#{@windowsexporterEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED=#{@windowskubeproxyEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED=#{@kappiebasicEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED=#{@podannotationEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KUBE_CONTROLLER_MANAGER_SCRAPING_ENABLED=#{@kubecontrollermanagerEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KUBE_SCHEDULER_SCRAPING_ENABLED=#{@kubeschedulerEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_KUBE_APISERVER_SCRAPING_ENABLED=#{@kubeapiserverEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_CLUSTER_AUTOSCALER_SCRAPING_ENABLED=#{@clusterautoscalerEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_ETCD_SCRAPING_ENABLED=#{@etcdEnabled}\n")
  file.write($export + "AZMON_PROMETHEUS_COLLECTOR_HEALTH_CCP_SCRAPING_ENABLED=#{@prometheusCollectorHealthCcpEnabled}\n")
  # Close file after writing all metric collection setting environment variables
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-scrape-settings config environment variables")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-scrape-settings Processing")