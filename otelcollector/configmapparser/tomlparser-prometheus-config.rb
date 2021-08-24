#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
require "deep_merge"
require "yaml"
require_relative "ConfigParseErrorLogger"

@configMapMountPath = "/etc/config/settings/prometheus/prometheus-config"
@collectorConfigTemplatePath = "/opt/microsoft/otelcollector/collector-config-template.yml"
@collectorConfigPath = "/opt/microsoft/otelcollector/collector-config.yml"
@configVersion = ""
@configSchemaVersion = ""
@replicasetControllerType = "replicaset"
@daemonsetControllerType = "daemonset"
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

    # Indent for the otelcollector config and remove comments
    # @indentedConfig = configString

    defaultConfigs = []
    # @indentedConfig = configString.gsub(/\R+/, "\n        ")
    # @indentedConfig = @indentedConfig.gsub(/#.*/, "")
    if !ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          defaultConfigs.push(@kubeletDefaultStringRsSimple)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultStringRsSimple)
        else
          defaultConfigs.push(@kubeletDefaultStringRsAdvanced)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultStringRsAdvanced)
        end
      else
        if advancedMode == true
          @kubeletDefaultStringDs = @kubeletDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @kubeletDefaultStringDs = @kubeletDefaultStringDs.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          defaultConfigs.push(@kubeletDefaultStringDs)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeletDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@corednsDefaultString)
      # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @corednsDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"].downcase == "true"
      if currentControllerType == @replicasetControllerType
        if advancedMode == false
          defaultConfigs.push(@cadvisorDefaultStringRsSimple)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @cadvisorDefaultStringRsSimple)
        else
          defaultConfigs.push(@cadvisorDefaultStringRsAdvanced)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @cadvisorDefaultStringRsAdvanced)
        end
      else
        if advancedMode == true
          @cadvisorDefaultStringDs = @cadvisorDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @cadvisorDefaultStringDs = @cadvisorDefaultStringDs.gsub("$$NODE_NAME$$", ENV["NODE_NAME"])
          defaultConfigs.push(@cadvisorDefaultStringDs)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @cadvisorDefaultStringDs)
        end
      end
    end
    if !ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@kubeproxyDefaultString)
      # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubeproxyDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      defaultConfigs.push(@apiserverDefaultString)
      # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @apiserverDefaultString)
    end
    if !ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].nil? && ENV["AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"].downcase == "true" && currentControllerType == @replicasetControllerType
      @kubestateDefaultString = @kubestateDefaultString.gsub("$$KUBE_STATE_NAME$$", ENV["KUBE_STATE_NAME"])
      @kubestateDefaultString = @kubestateDefaultString.gsub("$$POD_NAMESPACE$$", ENV["POD_NAMESPACE"])
      defaultConfigs.push(@kubestateDefaultString)
      # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @kubestateDefaultString)
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
        # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, nodeexporterDefaultStringRs)
      else
        if advancedMode == true
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub("$$NODE_IP$$", ENV["NODE_IP"])
          @nodeexporterDefaultStringDs = @nodeexporterDefaultStringDs.gsub("$$NODE_EXPORTER_TARGETPORT$$", ENV["NODE_EXPORTER_TARGETPORT"])
          defaultConfigs.push(@nodeexporterDefaultStringDs)
          # @indentedConfig = addDefaultScrapeConfig(@indentedConfig, @nodeexporterDefaultStringDs)
        end
      end
    end

    @mergedPromConfig = addDefaultScrapeConfig(configString, defaultConfigs)

    puts "config::Using config map setting for prometheus config"
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while substituting the prometheus config in the otelcollector config - #{errorStr}, using defaults, please check config map for errors")
    @mergedPromConfig = ""
  end
end

def replaceRelabelConfigRegex(relabelConfigs)
  begin
    relabelConfigs.each { |relabelConfig|
      action = relabelConfig["action"]
      if !action.nil? && !action.empty? # && (action.strip.casecmp("labeldrop") == 0 || action.strip.casecmp("labelkeep") == 0)
        replacement = relabelConfig["replacement"]
        if !replacement.nil? && !replacement.empty?
          relabelConfig["replacement"] = replacement.gsub("$", "$$")
        end
        regex = relabelConfig["regex"]
        if !regex.nil? && !regex.empty?
          relabelConfig["regex"] = regex.gsub("$", "$$")
        end
      end
    }
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while replacing relabel config regexes - #{errorStr}")
  end
end

def addDefaultScrapeConfig(configString, defaultScrapeConfigs)
  puts "config::Adding default scrape configs..."
  mergedCustomAndDefaultConfig = ""
  begin
    # Load custom prometheus config and check if it is valid, doing this since empty configmap returns false when we load config yaml
    # using the yaml library
    isPromCustomConfigValid = !!YAML.load(configString)
    if isPromCustomConfigValid == true
      promCustomConfig = YAML.load(configString)
      scfgs = promCustomConfig["scrape_configs"]
      puts "config::Starting to replace $ with $$ for regexes in relabel_configs and metric_relabel_configs if any "
      if !scfgs.nil? && !scfgs.empty? && scfgs.length > 0
        scfgs.each { |scfg|
          relabelConfigs = scfg["relabel_configs"]
          if !relabelConfigs.nil? && !relabelConfigs.empty? && relabelConfigs.length > 0
            replaceRelabelConfigRegex(relabelConfigs)
          end
          metricRelabelConfigs = scfg["metric_relabel_configs"]
          if !metricRelabelConfigs.nil? && !metricRelabelConfigs.empty? && metricRelabelConfigs.length > 0
            replaceRelabelConfigRegex(metricRelabelConfigs)
          end
        }
      end
      puts "config::Done replacing $ with $$ for regexes in relabel_configs and metric_relabel_configs"
      puts "config::Prometheus custom config is a valid yaml, merging custom scrape config with defaults"
    else
      puts "config::Prometheus custom config is either an invalid yaml or empty, merging empty scrape config with defaults"
      promCustomConfig = YAML.load("scrape_configs:")
    end
    #     input = YAML.load(File.read("prom-config-with-relabel.yaml"))

    # scfgs = input["scrape_configs"]

    # if !scfgs.nil? && !scfgs.empty? && scfgs.length > 0
    #   scfgs.each { |scfg|
    #     relabelConfigs = scfg["relabel_configs"]
    #     if !relabelConfigs.nil? && !relabelConfigs.empty? && relabelConfigs.length > 0
    #       relabelConfigs.each { |relabelConfig|
    #         action = relabelConfig["action"]
    #         if !action.nil? && !action.empty? && (action.strip.casecmp("labeldrop") == 0 || action.strip.casecmp("labelkeep") == 0)
    #           replacement = relabelConfig["replacement"]
    #           if !replacement.nil? && !replacement.empty?
    #             relabelConfig["replacement"] = replacement.gsub("$", "$$")
    #           end
    #         end
    #       }
    #     end
    #   }
    # end

    # Load each of the default scrape configs and merge it with prometheus custom config
    defaultScrapeConfigs.each { |defaultScrapeConfig|
      # Load yaml from default config
      defaultConfigYaml = YAML.load(defaultScrapeConfig)
      promCustomConfig = promCustomConfig.deep_merge!(defaultConfigYaml)
    }
    mergedCustomAndDefaultConfig = promCustomConfig
    puts "config::Done merging custom prometheus scrape config with defaults"
  rescue => errorStr
    ConfigParseErrorLogger.logError("Exception while adding default scrape config- #{errorStr}, using defaults")
  end
  return mergedCustomAndDefaultConfig
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
  if !@mergedPromConfig.nil? && !@mergedPromConfig.empty?
    puts "config::Starting to set prometheus config values in collector.yml"
    collectorTemplate = YAML.load(File.read(@collectorConfigTemplatePath))
    # Doing this instead of gsub because gsub causes ruby's string interpreter to strip escape characters from regex
    collectorTemplate["receivers"]["prometheus"]["config"] = @mergedPromConfig
    collectorNewConfig = YAML::dump(collectorTemplate)
    File.open(@collectorConfigPath, "w") { |file| file.puts collectorNewConfig }
    @useDefaultConfig = false
  else
    puts "config::config is empty so using the default config with an empty job"
  end
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
