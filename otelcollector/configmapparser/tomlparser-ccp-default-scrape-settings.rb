#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-scrape-settings"

@configMapMountPath = "/etc/config/settings/default-scrape-settings-enabled"
@configVersion = ""
@configSchemaVersion = ""

@controlplane_kube_controller_manager_enabled = false
@controlplane_kube_scheduler_enabled = false
@controlplane_apiserver_enabled = true
@controlplane_cluster_autoscaler_enabled = false
@controlplane_etcd_enabled = true
@controleplane_prometheuscollectorhealth_enabled = false
@noDefaultsEnabled = false

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
    if !parsedConfig[:"controlplane-kube-controller-manager"].nil?
      @controlplane_kube_controller_manager_enabled = parsedConfig[:"controlplane-kube-controller-manager"]
      puts "config::Using configmap scrape settings for controlplane-kube-controller-manager: #{@controlplane_kube_controller_manager_enabled}"
    end
    if !parsedConfig[:"controlplane-kube-scheduler"].nil?
      @controlplane_kube_scheduler_enabled = parsedConfig[:"controlplane-kube-scheduler"]
      puts "config::Using configmap scrape settings for controlplane-kube-scheduler: #{@controlplane_kube_scheduler_enabled}"
    end
    if !parsedConfig[:"controlplane-apiserver"].nil?
      @controlplane_apiserver_enabled = parsedConfig[:"controlplane-apiserver"]
      puts "config::Using configmap scrape settings for controlplane-apiserver: #{@controlplane_apiserver_enabled}"
    end
    if !parsedConfig[:"controlplane-cluster-autoscaler"].nil?
      @controlplane_cluster_autoscaler_enabled = parsedConfig[:"controlplane-cluster-autoscaler"]
      puts "config::Using configmap scrape settings for controlplane-cluster-autoscaler: #{@controlplane_cluster_autoscaler_enabled}"
    end
    if !parsedConfig[:"controlplane-etcd"].nil?
      @controlplane_etcd_enabled = parsedConfig[:"controlplane-etcd"]
      puts "config::Using configmap scrape settings for controlplane-etcd: #{@controlplane_etcd_enabled}"
    end
    if !parsedConfig[:"controlplane-prometheuscollectorhealth"].nil?
      @controleplane_prometheuscollectorhealth_enabled = parsedConfig[:"controlplane-prometheuscollectorhealth"]
      puts "config::Using configmap scrape settings for controlplane_prometheuscollectorhealth: #{@controleplane_prometheuscollectorhealth_enabled}"
    end

    if ENV["MODE"].nil? && ENV["MODE"].strip.downcase == "advanced"
      controllerType = ENV["CONTROLLER_TYPE"]
      if controllerType == "ReplicaSet" && ENV["OS_TYPE"].downcase == "linux" && !@controlplane_kube_controller_manager_enabled && !@controlplane_kube_scheduler_enabled && !@controlplane_apiserver_enabled && !@controlplane_cluster_autoscaler_enabled && !@controlplane_etcd_enabled
        @noDefaultsEnabled = true
      end
    elsif !@controlplane_kube_controller_manager_enabled && !@controlplane_kube_scheduler_enabled && !@controlplane_apiserver_enabled && !@controlplane_cluster_autoscaler_enabled && !@controlplane_etcd_enabled
      @noDefaultsEnabled = true
    end
    if @noDefaultsEnabled
      ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "No default scrape configs enabled")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for default scrape settings - #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-scrape-settings Processing")
# set default targets for MAC mode
if !ENV['MAC'].nil? && !ENV['MAC'].empty? && ENV['MAC'].strip.downcase == "true"
  ConfigParseErrorLogger.logWarning(LOGGING_PREFIX, "MAC mode is enabled. Only enabling targets controlplane_apiserver_enabled, controlplane_etcd_enabled for linux before config map processing....")
  @controlplane_kube_controller_manager_enabled = false
  @controlplane_kube_scheduler_enabled = false
  @controlplane_cluster_autoscaler_enabled = false
  @controleplane_prometheuscollectorhealth_enabled = false
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

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_default_scrape_settings_env_var", "w")

if !file.nil?
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED=#{@controlplane_kube_controller_manager_enabled}\n")
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED=#{@controlplane_kube_scheduler_enabled}\n")
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED=#{@controlplane_apiserver_enabled}\n")
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED=#{@controlplane_cluster_autoscaler_enabled}\n")
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED=#{@controlplane_etcd_enabled}\n")
  file.write("AZMON_PROMETHEUS_CONTROLPLANE_COLLECTOR_HEALTH_SCRAPING_ENABLED=#{@controleplane_prometheuscollectorhealth_enabled}\n")
  file.write("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=#{@noDefaultsEnabled}\n")
  # Close file after writing all metric collection setting environment variables
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-scrape-settings config environment variables")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-scrape-settings Processing")