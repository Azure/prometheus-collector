#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
if (!ENV['OS_TYPE'].nil? && ENV['OS_TYPE'].downcase == "linux")
  require "re2"
end
require "yaml"
require_relative "ConfigParseErrorLogger"

LOGGING_PREFIX = "default-scrape-keep-lists"

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

# minimal profile -- list of metrics to white-list for each target
@kubeletRegex_minimal = "kubelet_volume_stats_used_bytes|kubelet_node_name|kubelet_running_pods|kubelet_running_pod_count|kubelet_running_containers|kubelet_running_container_count|volume_manager_total_volumes|kubelet_node_config_error|kubelet_runtime_operations_total|kubelet_runtime_operations_errors_total|kubelet_runtime_operations_duration_seconds|kubelet_runtime_operations_duration_seconds_bucket|kubelet_runtime_operations_duration_seconds_sum|kubelet_runtime_operations_duration_seconds_count|kubelet_pod_start_duration_seconds|kubelet_pod_start_duration_seconds_bucket|kubelet_pod_start_duration_seconds_sum|kubelet_pod_start_duration_seconds_count|kubelet_pod_worker_duration_seconds|kubelet_pod_worker_duration_seconds_bucket|kubelet_pod_worker_duration_seconds_sum|kubelet_pod_worker_duration_seconds_count|storage_operation_duration_seconds|storage_operation_duration_seconds_bucket|storage_operation_duration_seconds_sum|storage_operation_duration_seconds_count|storage_operation_errors_total|kubelet_cgroup_manager_duration_seconds|kubelet_cgroup_manager_duration_seconds_bucket|kubelet_cgroup_manager_duration_seconds_sum|kubelet_cgroup_manager_duration_seconds_count|kubelet_pleg_relist_duration_seconds|kubelet_pleg_relist_duration_seconds_bucket|kubelet_pleg_relist_duration_sum|kubelet_pleg_relist_duration_seconds_count|kubelet_pleg_relist_interval_seconds|kubelet_pleg_relist_interval_seconds_bucket|kubelet_pleg_relist_interval_seconds_sum|kubelet_pleg_relist_interval_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_available_bytes|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes"
@corednsRegex_minimal = "coredns_build_info|coredns_panics_total|coredns_dns_responses_total|coredns_forward_responses_total|coredns_dns_request_duration_seconds|coredns_dns_request_duration_seconds_bucket|coredns_dns_request_duration_seconds_sum|coredns_dns_request_duration_seconds_count|coredns_forward_request_duration_seconds|coredns_forward_request_duration_seconds_bucket|coredns_forward_request_duration_seconds_sum|coredns_forward_request_duration_seconds_count|coredns_dns_requests_total|coredns_forward_requests_total|coredns_cache_hits_total|coredns_cache_misses_total|coredns_cache_entries|coredns_plugin_enabled|coredns_dns_request_size_bytes|coredns_dns_request_size_bytes_bucket|coredns_dns_request_size_bytes_sum|coredns_dns_request_size_bytes_count|coredns_dns_response_size_bytes|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"
@cadvisorRegex_minimal = "container_spec_cpu_period|container_spec_cpu_quota|container_cpu_usage_seconds_total|container_memory_rss|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_network_receive_packets_total|container_network_transmit_packets_total|container_network_receive_packets_dropped_total|container_network_transmit_packets_dropped_total|container_fs_reads_total|container_fs_writes_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_memory_working_set_bytes|container_memory_cache|container_memory_swap|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_periods_total|container_memory_usage_bytes"
@kubeproxyRegex_minimal = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_network_programming_duration_seconds|kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_network_programming_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"
@apiserverRegex_minimal = "apiserver_request_duration_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_sum|apiserver_request_duration_seconds_count|apiserver_request_total|workqueue_adds_total|workqueue_depth|workqueue_queue_duration_seconds|workqueue_queue_duration_seconds_bucket|workqueue_queue_duration_seconds_sum|workqueue_queue_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"
@kubestateRegex_minimal = "kube_node_status_capacity|kube_job_status_succeeded|kube_job_spec_completions|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_number_ready|kube_deployment_spec_replicas|kube_deployment_status_replicas_ready|kube_pod_container_status_last_terminated_reason|kube_node_status_condition|kube_pod_container_status_restarts_total|kube_pod_container_resource_requests|kube_pod_status_phase|kube_pod_container_resource_limits|kube_node_status_allocatable|kube_pod_info|kube_pod_owner|kube_resourcequota|kube_statefulset_replicas|kube_statefulset_status_replicas|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas_current|kube_statefulset_status_replicas_updated|kube_namespace_status_phase|kube_node_info|kube_statefulset_metadata_generation|kube_pod_labels|kube_pod_annotations"
@nodeexporterRegex_minimal = "node_cpu_seconds_total|node_memory_MemAvailable_bytes|node_memory_Buffers_bytes|node_memory_Cached_bytes|node_memory_MemFree_bytes|node_memory_Slab_bytes|node_memory_MemTotal_bytes|node_netstat_Tcp_RetransSegs|node_netstat_Tcp_OutSegs|node_netstat_TcpExt_TCPSynRetrans|node_load1|node_load5|node_load15|node_disk_read_bytes_total|node_disk_written_bytes_total|node_disk_io_time_seconds_total|node_filesystem_size_bytes|node_filesystem_avail_bytes|node_network_receive_bytes_total|node_network_transmit_bytes_total|node_vmstat_pgmajfault|node_network_receive_drop_total|node_network_transmit_drop_total|node_disk_io_time_weighted_seconds_total|node_exporter_build_info|node_time_seconds"
@windowsexporterRegex_minimal = "" #<todo>
@windowskubeproxyRegex_minimal = "" #<todo>

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-metrics-keep-list: #{errorStr}, using defaults, please check config map for errors")
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
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
    return false
  end
end

def isValidRegex_windows(str)
  begin
    # invalid regex example -> 'sel/\\'
    re2Regex = Regexp.new(str)
    return true
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while validating regex for target metric keep list - #{errorStr}, regular expression str - #{str}")
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
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for kubelet")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for kubelet")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeletRegex either not specified or not of type string")
    end

    corednsRegex = parsedConfig[:coredns]
    if !corednsRegex.nil? && corednsRegex.kind_of?(String)
      if !corednsRegex.empty?
        if isValidRegex(corednsRegex) == true
          @corednsRegex = corednsRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for coredns")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for coredns")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "corednsRegex either not specified or not of type string")
    end

    cadvisorRegex = parsedConfig[:cadvisor]
    if !cadvisorRegex.nil? && cadvisorRegex.kind_of?(String)
      if !cadvisorRegex.empty?
        if isValidRegex(cadvisorRegex) == true
          @cadvisorRegex = cadvisorRegex
          pConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for cadvisor")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for cadvisor")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "cadvisorRegex either not specified or not of type string")
    end

    kubeproxyRegex = parsedConfig[:kubeproxy]
    if !kubeproxyRegex.nil? && kubeproxyRegex.kind_of?(String)
      if !kubeproxyRegex.empty?
        if isValidRegex(kubeproxyRegex) == true
          @kubeproxyRegex = kubeproxyRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for kubeproxy")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for kubeproxy")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubeproxyRegex either not specified or not of type string")
    end

    apiserverRegex = parsedConfig[:apiserver]
    if !apiserverRegex.nil? && apiserverRegex.kind_of?(String)
      if !apiserverRegex.empty?
        if isValidRegex(apiserverRegex) == true
          @apiserverRegex = apiserverRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for apiserver")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for apiserver")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "apiserverRegex either not specified or not of type string")
    end

    kubestateRegex = parsedConfig[:kubestate]
    if !kubestateRegex.nil? && kubestateRegex.kind_of?(String)
      if !kubestateRegex.empty?
        if isValidRegex(kubestateRegex) == true
          @kubestateRegex = kubestateRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for kubestate")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for kubestate")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "kubestateRegex either not specified or not of type string")
    end

    nodeexporterRegex = parsedConfig[:nodeexporter]
    if !nodeexporterRegex.nil? && nodeexporterRegex.kind_of?(String)
      if !nodeexporterRegex.empty?
        if isValidRegex(nodeexporterRegex) == true
          @nodeexporterRegex = nodeexporterRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for nodeexporter")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for nodeexporter")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "nodeexporterRegex either not specified or not of type string")
    end

    windowsexporterRegex = parsedConfig[:windowsexporter]
    if !windowsexporterRegex.nil? && windowsexporterRegex.kind_of?(String)
      if !windowsexporterRegex.empty?
        if isValidRegex(windowsexporterRegex) == true
          @windowsexporterRegex = windowsexporterRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for windowsexporter")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for windowsexporter")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowsexporterRegex either not specified or not of type string")
    end

    windowskubeproxyRegex = parsedConfig[:windowskubeproxy]
    if !windowskubeproxyRegex.nil? && windowskubeproxyRegex.kind_of?(String)
      if !windowskubeproxyRegex.empty?
        if isValidRegex(windowskubeproxyRegex) == true
          @windowskubeproxyRegex = windowskubeproxyRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for windowskubeproxy")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for windowskubeproxy")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "windowskubeproxyRegex either not specified or not of type string")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for default targets metrics keep list - #{errorStr}, using defaults, please check config map for errors")
  end

# -------Apply profile for ingestion--------
# Logical OR-ing profile regex with customer provided regex
# so the theory here is --
    # if customer provided regex is valid, our regex validation for that will pass, and when minimal ingestion profile is true, a OR of customer provided regex with our minimal profile regex would be a valid regex as well, so we dont check again for the wholistic validation of merged regex
    # if customer provided regex is invalid, our regex validation for customer provided regex will fail, and if minimal ingestion profile is enabled, we will use that and ignore customer provided one

@minimalIngestionProfile = ENV["MINIMAL_INGESTION_PROFILE"] #this when enabled, will always be string "true" as we set the string value in the chart
if @minimalIngestionProfile == "true"
  ConfigParseErrorLogger.log(LOGGING_PREFIX, "minimalIngestionProfile=true. Applying appropriate Regexes")
  @kubeletRegex = @kubeletRegex + "|"  + @kubeletRegex_minimal
  @corednsRegex = @corednsRegex + "|" + @corednsRegex_minimal
  @cadvisorRegex = @cadvisorRegex + "|" + @cadvisorRegex_minimal
  @kubeproxyRegex = @kubeproxyRegex + "|" + @kubeproxyRegex_minimal
  @apiserverRegex = @apiserverRegex + "|" + @apiserverRegex_minimal
  @kubestateRegex = @kubestateRegex + "|" + @kubestateRegex_minimal
  @nodeexporterRegex = @nodeexporterRegex + "|" + @nodeexporterRegex_minimal
end 
# ----End appliing profile for ingestion--------
end

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-metrics-keep-list Processing")
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
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-metrics-keep-list regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-metrics-keep-list Processing")
