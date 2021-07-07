#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
@collectorConfigPath = "/opt/microsoft/otelcollector/collector-config.yml"
@configVersion = ""
@configSchemaVersion = ""
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@indentedConfig = ""
@useDefaultConfig = true

@kubeletDefaultStringRs = "job_name: kubelet\nscheme: https\nmetrics_path: /metrics\nscrape_interval: 30s\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nbearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nrelabel_configs:\n- source_labels: [__metrics_path__]\n  regex: (.*)\n  target_label: metrics_path\nkubernetes_sd_configs:\n- role: node"
@kubeletDefaultStringDs = "job_name: kubelet\nscheme: https\nmetrics_path: /metrics\nscrape_interval: 30s\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nbearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nrelabel_configs:\n- source_labels: [__metrics_path__]\n  regex: (.*)\n  target_label: metrics_path\nstatic_configs:\n- targets: ['$$NODE_IP$$:10250']"
@corednsDefaultString = "job_name: kube-dns\nscheme: http\nmetrics_path: /metrics\nscrape_interval: 30s\nrelabel_configs:\n- action: keep\n  source_labels: [__meta_kubernetes_namespace,__meta_kubernetes_pod_name]\n  separator: '/'\n  regex: 'kube-system/coredns.+'\n- source_labels: [__meta_kubernetes_pod_container_port_name]\n  action: keep\n  regex: metrics\n- source_labels: [__meta_kubernetes_pod_name]\n  target_label: pod\nkubernetes_sd_configs:\n- role: pod"
@cadvisorDefaultStringRs = "job_name: cadvisor\nscheme: https\nmetrics_path: /metrics/cadvisor\nscrape_interval: 30s\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nbearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nkubernetes_sd_configs:\n- role: node"
@cadvisorDefaultStringDs = "job_name: cadvisor\nscheme: https\nmetrics_path: /metrics/cadvisor\nscrape_interval: 30s\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nbearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nstatic_configs:\n- targets: ['$$NODE_IP$$:10250']"
@kubeproxyDefaultString = "job_name: kube-proxy\nscrape_interval: 30s\nkubernetes_sd_configs:\n- role: pod\nrelabel_configs:\n- action: keep\n  source_labels: [__meta_kubernetes_namespace,__meta_kubernetes_pod_name]\n  separator: '/'\n  regex: 'kube-system/kube-proxy.+'\n- source_labels:\n  - __address__\n  action: replace\n  target_label: __address__\n  regex: (.+?)(\\:\\d+)?\n  replacement: $$1:10249"
@apiserverDefaultString = "job_name: kube-apiserver\nscrape_interval: 30s\nkubernetes_sd_configs:\n- role: endpoints\nscheme: https\ntls_config:\n  ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n  insecure_skip_verify: true\nbearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\nrelabel_configs:\n- source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]\n  action: keep\n  regex: default;kubernetes;https"
@kubestateDefaultString = "job_name: kube-state-metrics\nscrape_interval: 30s\nstatic_configs:\n- targets: ['$$KUBE_STATE_NAME$$.$$POD_NAMESPACE$$.svc.cluster.local:8080']"
@nodeexporterDefaultStringRs = "job_name: node\nscheme: http\nscrape_interval: 30s\nkubernetes_sd_configs:\n- role: endpoints\n  namespaces:\n   names:\n   - $$POD_NAMESPACE$$\nrelabel_configs:\n- action: keep\n  source_labels: [__meta_kubernetes_endpoints_name]\n  regex: $$NODE_EXPORTER_NAME$$"
@nodeexporterDefaultStringDs = "job_name: node\nscheme: http\nscrape_interval: 30s\nstatic_configs:\n- targets: ['$$NODE_IP$$:$$NODE_EXPORTER_TARGETPORT$$']"

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    puts "config::configmap prometheus-collector-configmap for prometheus-config file: #{@configMapMountPath}"
    if (File.file?(@configMapMountPath) && ENV["AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"] != "true")
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
    # check if running in daemonset or replicaset
    currentControllerType = ENV["CONTROLLER_TYPE"].strip.downcase

    advancedMode = false #default is false

    # get current mode (advanced or not...)
    currentMode = ENV["MODE"].strip.downcase
    if currentMode == "advanced"
      advancedMode = true
    end
    
    # Indent for the otelcollector config
    @indentedConfig = configString.gsub(/\R+/, "\n        ")
    if !ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultStringRs)
        end
      else
        if advancedMode == true
          @kubeletDefaultStringDs = @kubeletDefaultStringDs.gsub('$$NODE_IP$$', ENV["NODE_IP"])
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @corednsDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @cadvisorDefaultStringRs)
        end
      else
        if advancedMode == true
          @cadvisorDefaultStringDs = @cadvisorDefaultStringDs.gsub('$$NODE_IP$$', ENV["NODE_IP"])
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @cadvisorDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeproxyDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @apiserverDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @kubestateDefaultString = @kubestateDefaultString.gsub('$$KUBE_STATE_NAME$$', ENV["KUBE_STATE_NAME"])
      @kubestateDefaultString = @kubestateDefaultString.gsub('$$POD_NAMESPACE$$', ENV["POD_NAMESPACE"])
      @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubestateDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          @nodeexporterDefaultStringRs = @nodeexporterDefaultStringRs.gsub('$$NODE_EXPORTER_NAME$$', ENV["NODE_EXPORTER_NAME"])
          @nodeexporterDefaultStringRs = @nodeexporterDefaultStringRs.gsub('$$POD_NAMESPACE$$', ENV["POD_NAMESPACE"])
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @nodeexporterDefaultStringRs)
        end
      else
        if advancedMode == true
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub('$$NODE_IP$$', ENV["NODE_IP"])
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub('$$NODE_EXPORTER_TARGETPORT$$', ENV["NODE_EXPORTER_TARGETPORT"])
          @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @nodeexporterDefaultStringDs)
        end
      end
    end
    puts "config::Using config map setting for prometheus config"
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
