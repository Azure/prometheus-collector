#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
if (!ENV['OS_TYPE'].nil? && ENV['OS_TYPE'].downcase == "linux")
  require "re2"
end
require "yaml"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
@configVersion = ""
@configSchemaVersion = ""

@kubeletRegex = ""
@corednsRegex = ""
@cadvisorRegex = ""
@kubeproxyRegex = ""
@apiserverRegex = ""
@kubestateRegex = ""
@nodeexporterRegex = ""
@windowsexporterRegex = ""
@windowskubeproxyRegex = ""

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus collector file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "config::configmap prometheus-collector-configmap for default-targets-metrics-keep-list mounted, parsing values"
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      puts "config::Successfully parsed mounted config map"
      return parsedConfig
    else
      puts "config::configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults"
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for default-targets-metrics-keep-list: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# RE2 is not supported for windows
def isValidRegex_linux(str)
  begin
    # invalid regex example -> 'sel/\\'
    re2Regex = RE2::Regexp.new(str)
    return re2Regex.ok?
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
    return false
  end
end

def isValidRegex_windows(str)
  begin
    # invalid regex example -> 'sel/\\'
    re2Regex = Regexp.new(str)
    return true
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
    return false
  end
end

def isValidRegex(str)
  if ENV['OS_TYPE'] == "linux"
    return isValidRegex_linux(str)
  else
    return isValidRegex_windows(str)
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    kubeletRegex = parsedConfig[:kubelet]
    if !kubeletRegex.nil? && kubeletRegex.kind_of?(String)
      if !kubeletRegex.empty?
        if isValidRegex(kubeletRegex) == true
          @kubeletRegex = kubeletRegex
          puts "def-target-metrics-keep-list-config::Using configmap metrics keep list regex for kubelet"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::kubeletRegex either not specified or not of type string"
    end

    corednsRegex = parsedConfig[:coredns]
    if !corednsRegex.nil? && corednsRegex.kind_of?(String)
      if !corednsRegex.empty?
        if isValidRegex(corednsRegex) == true
          @corednsRegex = corednsRegex
          puts "def-target-metrics-keep-list-config::Using configmap metrics keep list regex for coredns"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for coredns"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::corednsRegex either not specified or not of type string"
    end

    cadvisorRegex = parsedConfig[:cadvisor]
    if !cadvisorRegex.nil? && cadvisorRegex.kind_of?(String)
      if !cadvisorRegex.empty?
        if isValidRegex(cadvisorRegex) == true
          @cadvisorRegex = cadvisorRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for cadvisor"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for cadvisor"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::cadvisorRegex either not specified or not of type string"
    end

    kubeproxyRegex = parsedConfig[:kubeproxy]
    if !kubeproxyRegex.nil? && kubeproxyRegex.kind_of?(String)
      if !kubeproxyRegex.empty?
        if isValidRegex(kubeproxyRegex) == true
          @kubeproxyRegex = kubeproxyRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for kubeproxy"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for kubeproxy"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::kubeproxyRegex either not specified or not of type string"
    end

    apiserverRegex = parsedConfig[:apiserver]
    if !apiserverRegex.nil? && apiserverRegex.kind_of?(String)
      if !apiserverRegex.empty?
        if isValidRegex(apiserverRegex) == true
          @apiserverRegex = apiserverRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for apiserver"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for apiserver"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::apiserverRegex either not specified or not of type string"
    end

    kubestateRegex = parsedConfig[:kubestate]
    if !kubestateRegex.nil? && kubestateRegex.kind_of?(String)
      if !kubestateRegex.empty?
        if isValidRegex(kubestateRegex) == true
          @kubestateRegex = kubestateRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for kubestate"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for kubestate"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::kubestateRegex either not specified or not of type string"
    end

    nodeexporterRegex = parsedConfig[:nodeexporter]
    if !nodeexporterRegex.nil? && nodeexporterRegex.kind_of?(String)
      if !nodeexporterRegex.empty?
        if isValidRegex(nodeexporterRegex) == true
          @nodeexporterRegex = nodeexporterRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for nodeexporter"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for nodeexporter"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::nodeexporterRegex either not specified or not of type string"
    end

    windowsexporterRegex = parsedConfig[:windowsexporter]
    if !windowsexporterRegex.nil? && windowsexporterRegex.kind_of?(String)
      if !windowsexporterRegex.empty?
        if isValidRegex(windowsexporterRegex) == true
          @windowsexporterRegex = windowsexporterRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for windowsexporter"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for windowsexporter"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::windowsexporterRegex either not specified or not of type string"
    end

    windowskubeproxyRegex = parsedConfig[:windowskubeproxy]
    if !windowskubeproxyRegex.nil? && windowskubeproxyRegex.kind_of?(String)
      if !windowskubeproxyRegex.empty?
        if isValidRegex(windowskubeproxyRegex) == true
          @windowskubeproxyRegex = windowskubeproxyRegex
          puts "def-target-metrics-keep-list-config::Using configmap default scrape settings for windowskubeproxy"
        else
          puts "def-target-metrics-keep-list-config::invalid keep list regex for windowskubeproxy"
        end
      end
    else
      puts "def-target-metrics-keep-list-config::windowskubeproxyRegex either not specified or not of type string"
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while reading config map settings for default targets metrics keep list - #{errorStr}, using defaults, please check config map for errors")
  end
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
puts "****************Start default-targets-metrics-keep-list Processing********************"
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
file = File.open("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", "w")

regexHash = {}
regexHash["KUBELET_METRICS_KEEP_LIST_REGEX"] = @kubeletRegex
regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"] = @corednsRegex
regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"] = @cadvisorRegex
regexHash["KUBEPROXY_METRICS_KEEP_LIST_REGEX"] = @kubeproxyRegex
regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"] = @apiserverRegex
regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"] = @kubestateRegex
regexHash["NODEEXPORTER_METRICS_KEEP_LIST_REGEX"] = @nodeexporterRegex
regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"] = @windowsexporterRegex
regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"] = @windowskubeproxyRegex

if !file.nil?
  # Close file after writing regex keep list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(regexHash.to_yaml)
  file.close
  puts "****************End default-targets-metrics-keep-list Processing********************"
else
  puts "Exception while opening file for writing default-targets-metrics-keep-list regex config hash"
  puts "****************End default-targets-metrics-keep-list Processing********************"
end
