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
# @defaultConfigs = []

# Setting default values which will be used in case they are not set in the configmap or if configmap doesnt exist
@mergedPromConfig = ""
@useDefaultConfig = true

@kubeletDefaultStringRsSimple = "scrape_configs:\n- job_name: kubelet\n  scheme: https\n  metrics_path: /metrics\n  scrape_interval: 30s\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  relabel_configs:\n  - source_labels: [__metrics_path__]\n    regex: (.*)\n    target_label: metrics_path\n  kubernetes_sd_configs:\n  - role: node"
@kubeletDefaultStringRsAdvanced = "scrape_configs:\n- job_name: kubelet\n    scheme: https\n    metrics_path: /metrics\n    scrape_interval: 30s\n    tls_config:\n      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n      insecure_skip_verify: true\n    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n    relabel_configs:\n    - source_labels: [__metrics_path__]\n    regex: (.*)\n    target_label: metrics_path\n  metric_relabel_configs:\n  - source_labels: [__name__]\n    action: keep\n    regex: \"up\"\n  kubernetes_sd_configs:\n  - role: node"
@kubeletDefaultStringDs = "scrape_configs:\n- job_name: kubelet\n  scheme: https\n  metrics_path: /metrics\n  scrape_interval: 30s\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  relabel_configs:\n  - source_labels: [__metrics_path__]\n    regex: (.*)\n    target_label: metrics_path\n  - source_labels: [__address__]\n    replacement: '$$NODE_NAME$$'\n    target_label: instance\n  static_configs:\n  - targets: ['$$NODE_IP$$:10250']"
@corednsDefaultString = "scrape_configs:\n- job_name: kube-dns\n  scheme: http\n  metrics_path: /metrics\n  scrape_interval: 30s\n  relabel_configs:\n  - action: keep\n    source_labels: [__meta_kubernetes_namespace,__meta_kubernetes_pod_name]\n    separator: '/'\n    regex: 'kube-system/coredns.+'\n  - source_labels: [__meta_kubernetes_pod_container_port_name]\n    action: keep\n    regex: metrics\n  - source_labels: [__meta_kubernetes_pod_name]\n    target_label: pod\n  kubernetes_sd_configs:\n  - role: pod"
@cadvisorDefaultStringRsSimple = "scrape_configs:\n- job_name: cadvisor\n  scheme: https\n  metrics_path: /metrics/cadvisor\n  scrape_interval: 30s\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  kubernetes_sd_configs:\n  - role: node"
@cadvisorDefaultStringRsAdvanced = "scrape_configs:\n- job_name: cadvisor\n  scheme: https\n  metrics_path: /metrics/cadvisor\n  scrape_interval: 30s\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  metric_relabel_configs:\n  - source_labels: [__name__]\n    action: keep\n    regex: \"up\"\n  kubernetes_sd_configs:\n  - role: node"
@cadvisorDefaultStringDs = "scrape_configs:\n- job_name: cadvisor\n  scheme: https\n  metrics_path: /metrics/cadvisor\n  scrape_interval: 30s\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  relabel_configs:\n  - source_labels: [__address__]\n    replacement: '$$NODE_NAME$$'\n    target_label: instance\n  static_configs:\n  - targets: ['$$NODE_IP$$:10250']"
@kubeproxyDefaultString = "scrape_configs:\n- job_name: kube-proxy\n  scrape_interval: 30s\n  kubernetes_sd_configs:\n  - role: pod\n  relabel_configs:\n  - action: keep\n    source_labels: [__meta_kubernetes_namespace,__meta_kubernetes_pod_name]\n    separator: '/'\n    regex: 'kube-system/kube-proxy.+'\n  - source_labels:\n    - __address__\n    action: replace\n    target_label: __address__\n    regex: (.+?)(\\:\\d+)?\n    replacement: $$1:10249"
@apiserverDefaultString = "scrape_configs:\n- job_name: kube-apiserver\n  scrape_interval: 30s\n  kubernetes_sd_configs:\n  - role: endpoints\n  scheme: https\n  tls_config:\n    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt\n    insecure_skip_verify: true\n  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token\n  relabel_configs:\n  - source_labels: [__meta_kubernetes_namespace, __meta_kubernetes_service_name, __meta_kubernetes_endpoint_port_name]\n    action: keep\n    regex: default;kubernetes;https"
@kubestateDefaultString = "scrape_configs:\n- job_name: kube-state-metrics\n  scrape_interval: 30s\n  static_configs:\n  - targets: ['$$KUBE_STATE_NAME$$.$$POD_NAMESPACE$$.svc.cluster.local:8080']"
@nodeexporterDefaultStringRsSimple = "scrape_configs:\n- job_name: node\n  scheme: http\n  scrape_interval: 30s\n  kubernetes_sd_configs:\n  - role: endpoints\n    namespaces:\n     names:\n     - $$POD_NAMESPACE$$\n  relabel_configs:\n  - action: keep\n    source_labels: [__meta_kubernetes_endpoints_name]\n    regex: $$NODE_EXPORTER_NAME$$"
@nodeexporterDefaultStringRsAdvanced = "scrape_configs:\n- job_name: node\n  scheme: http\n  scrape_interval: 30s\n  kubernetes_sd_configs:\n  - role: endpoints\n    namespaces:\n     names:\n     - $$POD_NAMESPACE$$\n  relabel_configs:\n  - action: keep\n    source_labels: [__meta_kubernetes_endpoints_name]\n    regex: $$NODE_EXPORTER_NAME$$\n  metric_relabel_configs:\n  - source_labels: [__name__]\n    action: keep\n    regex: \"up\""
@nodeexporterDefaultStringDs = "scrape_configs:\n- job_name: node\n  scheme: http\n  scrape_interval: 30s\n  static_configs:\n  - targets: ['$$NODE_IP$$:$$NODE_EXPORTER_TARGETPORT$$']"

# Get the prometheus config and indent correctly for otelcollector config
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
          defaultConfigs.push(@kubeletDefaultStringRsSimple)
        else
          defaultConfigs.push(@kubeletDefaultStringRsAdvanced)
        end
      else
        if advancedMode == true
          @kubeletDefaultStringDs = @kubeletDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @kubeletDefaultStringDs = @kubeletDefaultStringDs.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          defaultConfigs.push(@kubeletDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@corednsDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          defaultConfigs.push(@cadvisorDefaultStringRsSimple)
        else
          defaultConfigs.push(@cadvisorDefaultStringRsAdvanced)
        end
      else
        if advancedMode == true
          @cadvisorDefaultStringDs = @cadvisorDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @cadvisorDefaultStringDs = @cadvisorDefaultStringDs.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          defaultConfigs.push(@cadvisorDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@kubeproxyDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@apiserverDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @kubestateDefaultString = @kubestateDefaultString.gsub("$$KUBE_STATE_NAME$$", ENV["KUBE_STATE_NAME"])
      @kubestateDefaultString = @kubestateDefaultString.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      defaultConfigs.push(@kubestateDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        nodeexporterDefaultStringRs = @nodeexporterDefaultStringRsSimple
        if advancedMode == true
          nodeexporterDefaultStringRs = @nodeexporterDefaultStringRsAdvanced
        end
        nodeexporterDefaultStringRs = nodeexporterDefaultStringRs.gsub("$$NODE_EXPORTER_NAME$$", ENV["NODE_EXPORTER_NAME"])
        nodeexporterDefaultStringRs = nodeexporterDefaultStringRs.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
        defaultConfigs.push(nodeexporterDefaultStringRs)
      else
        if advancedMode == true
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub("$$NODE_EXPORTER_TARGETPORT$$", ENV["NODE_EXPORTER_TARGETPORT"])
          defaultConfigs.push(@nodeexporterDefaultStringDs)
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
        defaultConfigYaml = YAML.load(defaultScrapeConfig)
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
