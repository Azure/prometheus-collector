#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
@collectorConfigPath = "/opt/microsoft/otelcollector/collector-config.yml"
@configVersion = ""
@configSchemaVersion = ""

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@indentedConfig = ""
@useDefaultConfig = true

@kubeletDefaultString = "job_name: 'kubernetes-nodes'\nscheme: https\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nauthorization:\n  credentials_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nkubernetes_sd_configs:\n- role: node\nrelabel_configs:\n- action: labelmap\n  regex: __meta_kubernetes_node_label_(.+)"
@corednsDefaultString = "job_name: kube-dns\nhonor_labels: true\nkubernetes_sd_configs:\n- role: pod\nrelabel_configs:\n- action: keep\n  source_labels:\n  - __meta_kubernetes_namespace\n  - __meta_kubernetes_pod_name\n  separator: '/'\n  regex: 'kube-system/coredns.+'\n- source_labels:\n  - __meta_kubernetes_pod_container_port_name\n  action: keep\n  regex: metrics\n- source_labels:\n  - __meta_kubernetes_pod_name\n  action: replace\n  target_label: instance\n- action: labelmap\n  regex: __meta_kubernetes_pod_label_(.+)"

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus-config file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath))
      puts "config::configmap prometheus-collector-configmap for prometheus config mounted, parsing values"
      config = File.read(@configMapMountPath)
      puts "config::Successfully parsed mounted config map"
      return config
    else
      puts "config::configmap prometheus-collector-configmap for prometheus config not mounted, using defaults"
      return ""
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while parsing config map for prometheus config : #{errorStr}, using defaults, please check config map for errors")
    return ""
  end
end

# Get the prometheus config and indent correctly for otelcollector config
def populateSettingValuesFromConfigMap(configString)
  begin
    # Indent for the otelcollector config
    @indentedConfig = configString.gsub(/\R+/, "\n        ")
    if !ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].downcase == "true"
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true"
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @corednsDefaultString)
    end
    puts "config::Using config map setting for prometheus config"
    puts @indentedConfig
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while substituting the prometheus config in the otelcollector config - #{errorStr}, using defaults, please check config map for errors")
    @indentedConfig = ""
  end
end

def addDefaultScrapeConfig(indentedConfig, defaultScrapeConfig)

  # Check where to put the extra scrape config
  scrapeConfigString = "scrape_configs:"
  scrapeConfigIndex = indentedConfig.index(scrapeConfigString)
  if !scrapeConfigIndex.nil?
    indexToAddAt = scrapeConfigIndex + scrapeConfigString.length

    # Get how far indented the existing scrape configs are and add the scrape config at the same indentation
    matched = indentedConfig.match("scrape_configs\s*:\s*\n(\s*)-(\s+).*")
    if !matched.nil? && !matched.captures.nil? && matched.captures.length > 1
      whitespaceBeforeDash = matched.captures[0]
      whiteSpaceAfterDash = matched.captures[1]

      # Match indentation for everything underneath "- job_name:" (Include an extra space for -)
      indentedDefaultConfig = defaultScrapeConfig.gsub(/\R+/, sprintf("\n%s %s", whitespaceBeforeDash, whiteSpaceAfterDash))

      # Match indentation and add dash for the starting line "- job_name:"
      indentedDefaultConfig = sprintf("\n%s-%s%s\n", whitespaceBeforeDash, whiteSpaceAfterDash, indentedDefaultConfig)

      # Add the indented scrape config to the existing config
      indentedConfig = indentedConfig.insert(indexToAddAt, indentedDefaultConfig)
    end

  # The section "scrape_configs:" isn't in config, so add it at the beginning and the extra scrape config underneath
  # Don't need to match indentation since there aren't any other scrape configs
  else
    indentedDefaultConfig = defaultScrapeConfig.gsub(/\R+/, "\n          ")
    stringToAdd = sprintf("scrape_configs:\n        - %s\n        ", indentedDefaultConfig)
    indentedConfig = indentedConfig.insert(0, stringToAdd)
  end

  return indentedConfig
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
puts "****************Start Prometheus Config Processing********************"
if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  prometheusConfigString = parseConfigMap
  # Need to populate default configs even if specfied config is empty
  populateSettingValuesFromConfigMap(prometheusConfigString)
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError("config::unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end

begin
  puts "config::Starting to substitute the placeholders in collector.yml"
  #Replace the placeholder value in the otelcollector with values from custom config
  text = File.read(@collectorConfigTemplatePath)
  new_contents = text.gsub("$AZMON_PROMETHEUS_CONFIG", @indentedConfig)
  File.open(@collectorConfigPath, "w") { |file| file.puts new_contents }
  @useDefaultConfig = false
rescue => errorStr
  ConfigParseErrorLogger.logError("Exception while substituing placeholders for prometheus config - #{errorStr}")
end

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_prometheus_collector_prometheus_config_env_var", "w")

if !file.nil?
  file.write("export AZMON_USE_DEFAULT_PROMETHEUS_CONFIG=#{@useDefaultConfig}\n")
  # Close file after writing all metric collection setting environment variables
  file.close
  puts "****************End Prometheus Config Processing********************"
else
  puts "Exception while opening file for writing prometheus config environment variables"
  puts "****************End Prometheus Config Processing********************"
end
