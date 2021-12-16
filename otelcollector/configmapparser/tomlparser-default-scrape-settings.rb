#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

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
@windowsexporterEnabled = false
@windowskubeproxyEnabled = false
@noDefaultsEnabled = false

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus collector file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "config::configmap prometheus-collector-configmap for default scrape settings mounted, parsing values"
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      puts "config::Successfully parsed mounted config map"
      return parsedConfig
    else
      puts "config::configmapprometheus-collector-configmap for default scrape settings not mounted, using defaults"
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for default scrape settings: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    if !parsedConfig[:kubelet].nil?
      @kubeletEnabled = parsedConfig[:kubelet]
      puts "config::Using configmap default scrape settings for kubelet"
    end
    if !parsedConfig[:coredns].nil?
      @corednsEnabled = parsedConfig[:coredns]
      puts "config::Using configmap default scrape settings for coredns"
    end
    if !parsedConfig[:cadvisor].nil?
      @cadvisorEnabled = parsedConfig[:cadvisor]
      puts "config::Using configmap default scrape settings for cadvisor"
    end
    if !parsedConfig[:kubeproxy].nil?
      @kubeproxyEnabled = parsedConfig[:kubeproxy]
      puts "config::Using configmap default scrape settings for kubeproxy"
    end
    if !parsedConfig[:apiserver].nil?
      @apiserverEnabled = parsedConfig[:apiserver]
      puts "config::Using configmap default scrape settings for apiserver"
    end
    if !parsedConfig[:kubestate].nil?
      @kubestateEnabled = parsedConfig[:kubestate]
      puts "config::Using configmap default scrape settings for kubestate"
    end
    if !parsedConfig[:nodeexporter].nil?
      @nodeexporterEnabled = parsedConfig[:nodeexporter]
      puts "config::Using configmap default scrape settings for nodeexporter"
    end
    if !parsedConfig[:prometheuscollectorhealth].nil?
      @prometheusCollectorHealthEnabled = parsedConfig[:prometheuscollectorhealth]
      puts "config::Using configmap default scrape settings for prometheuscollectorhealth"
    end
    if !parsedConfig[:windowsexporter].nil?
      @windowsexporterEnabled = parsedConfig[:windowsexporter]
      puts "config::Using configmap default scrape settings for windowsexporter"
    end
    if !parsedConfig[:windowskubeproxy].nil?
      @windowskubeproxyEnabled = parsedConfig[:windowskubeproxy]
      puts "config::Using configmap default scrape settings for windowskubeproxy"
    end

    if ENV["MODE"].nil? && ENV["MODE"].strip.downcase == "advanced"
      controllerType = ENV["CONTROLLER_TYPE"]
      if controllerType == "DaemonSet" && !@kubeletEnabled && !@cadvisorEnabled && !@nodeexporterEnabled && !@prometheusCollectorHealthEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled
        @noDefaultsEnabled = true
        puts "config::No default scrape configs enabled"
      elsif controllerType == "ReplicaSet" && !@corednsEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@prometheusCollectorHealthEnabled
        @noDefaultsEnabled = true
        puts "config::No default scrape configs enabled"
      end
    elsif !@kubeletEnabled && !@corednsEnabled && !@cadvisorEnabled && !@kubeproxyEnabled && !@apiserverEnabled && !@kubestateEnabled && !@nodeexporterEnabled && !@windowsexporterEnabled && !@windowskubeproxyEnabled && !@prometheusCollectorHealthEnabled
      @noDefaultsEnabled = true
      puts "config::No default scrape configs enabled"
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while reading config map settings for default scrape settings - #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
puts "****************Start default-scrape-settings Processing********************"
if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  configMapSettings = parseConfigMap
  if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
  end
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError("config::unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end

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
  # Close file after writing all metric collection setting environment variables
  file.close
  puts "****************End default-scrape-settings Processing********************"
else
  puts "Exception while opening file for writing default-scrape-settings config environment variables"
  puts "****************End default-scrape-settings Processing********************"
end