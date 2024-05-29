#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
if (!ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux")
  require "re2"
end
require "yaml"
require_relative "ConfigParseErrorLogger"
require_relative "tomlparser-utils"

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
@podannotationRegex = ""
@kappiebasicRegex = ""
@networkobservabilityRetinaRegex = ""
@networkobservabilityHubbleRegex = ""
@networkobservabilityCiliumRegex = ""

#This will always be string "true" as we set the string value in the chart for both MAC and non MAC modes
@minimalIngestionProfile = ENV["MINIMAL_INGESTION_PROFILE"]

@isMacMode = false
if !ENV["MAC"].nil? && !ENV["MAC"].empty? && ENV["MAC"].strip.downcase == "true"
  @isMacMode = true
end

# minimal profile -- list of metrics to white-list for each target for 1p mode (non MAC). This list includes metrics used by default dashboards + alerts.
@kubeletRegex_minimal = "kubelet_volume_stats_used_bytes|kubelet_node_name|kubelet_running_pods|kubelet_running_pod_count|kubelet_running_containers|kubelet_running_container_count|volume_manager_total_volumes|kubelet_node_config_error|kubelet_runtime_operations_total|kubelet_runtime_operations_errors_total|kubelet_runtime_operations_duration_seconds|kubelet_runtime_operations_duration_seconds_bucket|kubelet_runtime_operations_duration_seconds_sum|kubelet_runtime_operations_duration_seconds_count|kubelet_pod_start_duration_seconds|kubelet_pod_start_duration_seconds_bucket|kubelet_pod_start_duration_seconds_sum|kubelet_pod_start_duration_seconds_count|kubelet_pod_worker_duration_seconds|kubelet_pod_worker_duration_seconds_bucket|kubelet_pod_worker_duration_seconds_sum|kubelet_pod_worker_duration_seconds_count|storage_operation_duration_seconds|storage_operation_duration_seconds_bucket|storage_operation_duration_seconds_sum|storage_operation_duration_seconds_count|storage_operation_errors_total|kubelet_cgroup_manager_duration_seconds|kubelet_cgroup_manager_duration_seconds_bucket|kubelet_cgroup_manager_duration_seconds_sum|kubelet_cgroup_manager_duration_seconds_count|kubelet_pleg_relist_duration_seconds|kubelet_pleg_relist_duration_seconds_bucket|kubelet_pleg_relist_duration_sum|kubelet_pleg_relist_duration_seconds_count|kubelet_pleg_relist_interval_seconds|kubelet_pleg_relist_interval_seconds_bucket|kubelet_pleg_relist_interval_seconds_sum|kubelet_pleg_relist_interval_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_available_bytes|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes|kubernetes_build_info|kubelet_certificate_manager_client_ttl_seconds|kubelet_certificate_manager_client_expiration_renew_errors|kubelet_server_expiration_renew_errors|kubelet_certificate_manager_server_ttl_seconds|kubelet_volume_stats_inodes_free"
@corednsRegex_minimal = "coredns_build_info|coredns_panics_total|coredns_dns_responses_total|coredns_forward_responses_total|coredns_dns_request_duration_seconds|coredns_dns_request_duration_seconds_bucket|coredns_dns_request_duration_seconds_sum|coredns_dns_request_duration_seconds_count|coredns_forward_request_duration_seconds|coredns_forward_request_duration_seconds_bucket|coredns_forward_request_duration_seconds_sum|coredns_forward_request_duration_seconds_count|coredns_dns_requests_total|coredns_forward_requests_total|coredns_cache_hits_total|coredns_cache_misses_total|coredns_cache_entries|coredns_plugin_enabled|coredns_dns_request_size_bytes|coredns_dns_request_size_bytes_bucket|coredns_dns_request_size_bytes_sum|coredns_dns_request_size_bytes_count|coredns_dns_response_size_bytes|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
@cadvisorRegex_minimal = "container_spec_cpu_period|container_spec_cpu_quota|container_cpu_usage_seconds_total|container_memory_rss|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_network_receive_packets_total|container_network_transmit_packets_total|container_network_receive_packets_dropped_total|container_network_transmit_packets_dropped_total|container_fs_reads_total|container_fs_writes_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_memory_working_set_bytes|container_memory_cache|container_memory_swap|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_periods_total|container_memory_usage_bytes|kubernetes_build_info"
@kubeproxyRegex_minimal = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_network_programming_duration_seconds|kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_network_programming_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
@apiserverRegex_minimal = "apiserver_request_duration_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_sum|apiserver_request_duration_seconds_count|apiserver_request_total|workqueue_adds_total|workqueue_depth|workqueue_queue_duration_seconds|workqueue_queue_duration_seconds_bucket|workqueue_queue_duration_seconds_sum|workqueue_queue_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count"
@kubestateRegex_minimal = "kube_horizontalpodautoscaler_spec_min_replicas|kube_horizontalpodautoscaler_status_desired_replicas|kube_job_status_active|kube_node_status_capacity|kube_job_status_succeeded|kube_job_spec_completions|kube_daemonset_status_number_misscheduled|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_number_ready|kube_deployment_spec_replicas|kube_deployment_status_replicas_ready|kube_pod_container_status_last_terminated_reason|kube_node_status_condition|kube_pod_container_status_restarts_total|kube_pod_container_resource_requests|kube_pod_status_phase|kube_pod_container_resource_limits|kube_node_status_allocatable|kube_pod_info|kube_pod_owner|kube_resourcequota|kube_statefulset_replicas|kube_statefulset_status_replicas|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas_current|kube_statefulset_status_replicas_updated|kube_namespace_status_phase|kube_node_info|kube_statefulset_metadata_generation|kube_pod_labels|kube_pod_annotations|kube_horizontalpodautoscaler_status_current_replicas|kube_horizontalpodautoscaler_spec_max_replicas|kube_node_spec_taint|kube_pod_container_status_waiting_reason|kube_job_failed|kube_job_status_start_time|kube_deployment_status_replicas_available|kube_deployment_status_replicas_updated|kube_replicaset_owner|kubernetes_build_info|kube_pod_container_info|kube_persistentvolumeclaim_access_mode|kube_persistentvolumeclaim_labels|kube_persistentvolume_status_phase"
@nodeexporterRegex_minimal = "node_filesystem_readonly|node_cpu_seconds_total|node_memory_MemAvailable_bytes|node_memory_Buffers_bytes|node_memory_Cached_bytes|node_memory_MemFree_bytes|node_memory_Slab_bytes|node_memory_MemTotal_bytes|node_netstat_Tcp_RetransSegs|node_netstat_Tcp_OutSegs|node_netstat_TcpExt_TCPSynRetrans|node_load1|node_load5|node_load15|node_disk_read_bytes_total|node_disk_written_bytes_total|node_disk_io_time_seconds_total|node_filesystem_size_bytes|node_filesystem_avail_bytes|node_network_receive_bytes_total|node_network_transmit_bytes_total|node_vmstat_pgmajfault|node_network_receive_drop_total|node_network_transmit_drop_total|node_disk_io_time_weighted_seconds_total|node_exporter_build_info|node_time_seconds|node_uname_info|kubernetes_build_info"
@kappiebasicRegex_minimal = "kappie.*"
@networkobservabilityRetinaRegex_minimal = "networkobservability.*"
@networkobservabilityHubbleRegex_minimal = "hubble_dns_queries_total|hubble_dns_responses_total|hubble_drop_total|hubble_tcp_flags_total"
@networkobservabilityCiliumRegex_minimal = "cilium_drop.*|cilium_forward.*"
@windowsexporterRegex_minimal = "windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total"
@windowskubeproxyRegex_minimal = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"

# minimal profile when MAC mode is enabled. This list includes metrics used by default dashboards + rec rules + alerts, when MAC mode is enabled.
@kubeletRegex_minimal_mac = "kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes|kubelet_node_name|kubelet_running_pods|kubelet_running_pod_count|kubelet_running_sum_containers|kubelet_running_containers|kubelet_running_container_count|volume_manager_total_volumes|kubelet_node_config_error|kubelet_runtime_operations_total|kubelet_runtime_operations_errors_total|kubelet_runtime_operations_duration_seconds_bucket|kubelet_runtime_operations_duration_seconds_sum|kubelet_runtime_operations_duration_seconds_count|kubelet_pod_start_duration_seconds_bucket|kubelet_pod_start_duration_seconds_sum|kubelet_pod_start_duration_seconds_count|kubelet_pod_worker_duration_seconds_bucket|kubelet_pod_worker_duration_seconds_sum|kubelet_pod_worker_duration_seconds_count|storage_operation_duration_seconds_bucket|storage_operation_duration_seconds_sum|storage_operation_duration_seconds_count|storage_operation_errors_total|kubelet_cgroup_manager_duration_seconds_bucket|kubelet_cgroup_manager_duration_seconds_sum|kubelet_cgroup_manager_duration_seconds_count|kubelet_pleg_relist_interval_seconds_bucket|kubelet_pleg_relist_interval_seconds_count|kubelet_pleg_relist_interval_seconds_sum|kubelet_pleg_relist_duration_seconds_bucket|kubelet_pleg_relist_duration_seconds_count|kubelet_pleg_relist_duration_seconds_sum|rest_client_requests_total|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|kubelet_certificate_manager_client_ttl_seconds|kubelet_certificate_manager_client_expiration_renew_errors|kubelet_server_expiration_renew_errors|kubelet_certificate_manager_server_ttl_seconds|kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_inodes_free|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes"
@corednsRegex_minimal_mac = "coredns_build_info|coredns_panics_total|coredns_dns_responses_total|coredns_forward_responses_total|coredns_dns_request_duration_seconds|coredns_dns_request_duration_seconds_bucket|coredns_dns_request_duration_seconds_sum|coredns_dns_request_duration_seconds_count|coredns_forward_request_duration_seconds|coredns_forward_request_duration_seconds_bucket|coredns_forward_request_duration_seconds_sum|coredns_forward_request_duration_seconds_count|coredns_dns_requests_total|coredns_forward_requests_total|coredns_cache_hits_total|coredns_cache_misses_total|coredns_cache_entries|coredns_plugin_enabled|coredns_dns_request_size_bytes|coredns_dns_request_size_bytes_bucket|coredns_dns_request_size_bytes_sum|coredns_dns_request_size_bytes_count|coredns_dns_response_size_bytes|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
@cadvisorRegex_minimal_mac = "container_spec_cpu_quota|container_spec_cpu_period|container_memory_rss|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_network_receive_packets_total|container_network_transmit_packets_total|container_network_receive_packets_dropped_total|container_network_transmit_packets_dropped_total|container_fs_reads_total|container_fs_writes_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_cpu_usage_seconds_total|container_memory_working_set_bytes|container_memory_cache|container_memory_swap|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_periods_total|container_memory_rss|kubernetes_build_info|container_start_time_seconds"
@kubeproxyRegex_minimal_mac = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_network_programming_duration_seconds|kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_network_programming_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
@apiserverRegex_minimal_mac = "apiserver_request_duration_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_sum|apiserver_request_duration_seconds_count|apiserver_request_total|workqueue_adds_total|workqueue_depth|workqueue_queue_duration_seconds|workqueue_queue_duration_seconds_bucket|workqueue_queue_duration_seconds_sum|workqueue_queue_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count"
@kubestateRegex_minimal_mac = "kube_job_status_succeeded|kube_job_spec_completions|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_number_misscheduled|kube_daemonset_status_number_ready|kube_deployment_status_replicas_ready|kube_pod_container_status_last_terminated_reason|kube_pod_container_status_waiting_reason|kube_pod_container_status_restarts_total|kube_node_status_allocatable|kube_pod_owner|kube_pod_container_resource_requests|kube_pod_status_phase|kube_pod_container_resource_limits|kube_replicaset_owner|kube_resourcequota|kube_namespace_status_phase|kube_node_status_capacity|kube_node_info|kube_pod_info|kube_deployment_spec_replicas|kube_deployment_status_replicas_available|kube_deployment_status_replicas_updated|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas|kube_statefulset_status_replicas_updated|kube_job_status_start_time|kube_job_status_active|kube_job_failed|kube_horizontalpodautoscaler_status_desired_replicas|kube_horizontalpodautoscaler_status_current_replicas|kube_horizontalpodautoscaler_spec_min_replicas|kube_horizontalpodautoscaler_spec_max_replicas|kubernetes_build_info|kube_node_status_condition|kube_node_spec_taint|kube_pod_container_info|kube_.*_labels|kube_.*_annotations|kube_service_info|kube_pod_container_status_running|kube_pod_container_status_waiting|kube_pod_container_status_terminated|kube_pod_container_state_started|kube_pod_created|kube_pod_start_time|kube_pod_init_container_info|kube_pod_init_container_status_terminated|kube_pod_init_container_status_terminated_reason|kube_pod_init_container_status_ready|kube_pod_init_container_resource_limits|kube_pod_init_container_status_running|kube_pod_init_container_status_waiting|kube_pod_init_container_status_restarts_total|kube_pod_container_status_ready|kube_pod_init_container_*|kube_pod_deletion_timestamp|kube_pod_status_reason|kube_pod_init_container_resource_requests|kube_persistentvolumeclaim_access_mode|kube_persistentvolumeclaim_labels|kube_persistentvolume_status_phase"
@nodeexporterRegex_minimal_mac = "node_filesystem_readonly|node_memory_MemTotal_bytes|node_cpu_seconds_total|node_memory_MemAvailable_bytes|node_memory_Buffers_bytes|node_memory_Cached_bytes|node_memory_MemFree_bytes|node_memory_Slab_bytes|node_filesystem_avail_bytes|node_filesystem_size_bytes|node_time_seconds|node_exporter_build_info|node_load1|node_vmstat_pgmajfault|node_network_receive_bytes_total|node_network_transmit_bytes_total|node_network_receive_drop_total|node_network_transmit_drop_total|node_disk_io_time_seconds_total|node_disk_io_time_weighted_seconds_total|node_load5|node_load15|node_disk_read_bytes_total|node_disk_written_bytes_total|node_uname_info|kubernetes_build_info|node_boot_time_seconds"
@kappiebasicRegex_minimal_mac = "kappie.*"
@networkobservabilityRetinaRegex_minimal_mac = "networkobservability.*"
@networkobservabilityHubbleRegex_minimal_mac = "hubble_dns_queries_total|hubble_dns_responses_total|hubble_drop_total|hubble_tcp_flags_total"
@networkobservabilityCiliumRegex_minimal_mac = "cilium_drop.*|cilium_forward.*"
@windowsexporterRegex_minimal_mac = "windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total"
@windowskubeproxyRegex_minimal_mac = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"

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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeletRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "corednsRegex either not specified or not of type string")
    end

    cadvisorRegex = parsedConfig[:cadvisor]
    if !cadvisorRegex.nil? && cadvisorRegex.kind_of?(String)
      if !cadvisorRegex.empty?
        if isValidRegex(cadvisorRegex) == true
          @cadvisorRegex = cadvisorRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for cadvisor")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for cadvisor")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "cadvisorRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubeproxyRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "apiserverRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kubestateRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "nodeexporterRegex either not specified or not of type string")
    end

    kappiebasicRegex = parsedConfig[:kappiebasic]
    if !kappiebasicRegex.nil? && kappiebasicRegex.kind_of?(String)
      if !kappiebasicRegex.empty?
        if isValidRegex(kappiebasicRegex) == true
          @kappiebasicRegex = kappiebasicRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for kappiebasic")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for kappiebasic")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "kappiebasicRegex either not specified or not of type string")
    end

    networkobservabilityRetinaRegex = parsedConfig[:networkobservabilityRetina]
    if !networkobservabilityRetinaRegex.nil? && networkobservabilityRetinaRegex.kind_of?(String)
      if !networkobservabilityRetinaRegex.empty?
        if isValidRegex(networkobservabilityRetinaRegex) == true
          @networkobservabilityRetinaRegex = networkobservabilityRetinaRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for networkobservabilityRetina")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for networkobservabilityRetina")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "networkobservabilityRetinaRegex either not specified or not of type string")
    end

    networkobservabilityHubbleRegex = parsedConfig[:networkobservabilityHubble]
    if !networkobservabilityHubbleRegex.nil? && networkobservabilityHubbleRegex.kind_of?(String)
      if !networkobservabilityHubbleRegex.empty?
        if isValidRegex(networkobservabilityHubbleRegex) == true
          @networkobservabilityHubbleRegex = networkobservabilityHubbleRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for networkobservabilityHubble")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for networkobservabilityHubble")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "networkobservabilityHubbleRegex either not specified or not of type string")
    end

    networkobservabilityCiliumRegex = parsedConfig[:networkobservabilityCilium]
    if !networkobservabilityCiliumRegex.nil? && networkobservabilityCiliumRegex.kind_of?(String)
      if !networkobservabilityCiliumRegex.empty?
        if isValidRegex(networkobservabilityCiliumRegex) == true
          @networkobservabilityCiliumRegex = networkobservabilityCiliumRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for networkobservabilityCilium")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for networkobservabilityCilium")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "networkobservabilityCiliumRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "windowsexporterRegex either not specified or not of type string")
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
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "windowskubeproxyRegex either not specified or not of type string")
    end

    podannotationRegex = parsedConfig[:podannotations]
    if !podannotationRegex.nil? && podannotationRegex.kind_of?(String)
      if !podannotationRegex.empty?
        if isValidRegex(podannotationRegex) == true
          @podannotationRegex = podannotationRegex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for podannotations")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for podannotations")
        end
      end
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "podannotationRegex either not specified or not of type string")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for default targets metrics keep list - #{errorStr}, using defaults, please check config map for errors")
  end

  # Provide for overwriting the chart setting for minimal ingestion profile using configmap for MAC mode
  if @isMacMode == true
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "MAC mode set to true - Reading configmap setting for minimalingestionprofile")
    minimalIngestionProfileSetting = parsedConfig[:minimalingestionprofile]
    if !minimalIngestionProfileSetting.nil?
      @minimalIngestionProfile = minimalIngestionProfileSetting.to_s.downcase #Doing this to keep it consistent in the check below for helm chart and configmap
    end
  end
end

# -------Apply profile for ingestion--------
# Logical OR-ing profile regex with customer provided regex
# so the theory here is --
# if customer provided regex is valid, our regex validation for that will pass, and when minimal ingestion profile is true, a OR of customer provided regex with our minimal profile regex would be a valid regex as well, so we dont check again for the wholistic validation of merged regex
# if customer provided regex is invalid, our regex validation for customer provided regex will fail, and if minimal ingestion profile is enabled, we will use that and ignore customer provided one
def populateRegexValuesWithMinimalIngestionProfile
  begin
    if @minimalIngestionProfile == "true"
      if @isMacMode == true
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "minimalIngestionProfile=true, MAC is enabled. Applying appropriate MAC Regexes")
        @kubeletRegex = @kubeletRegex + "|" + @kubeletRegex_minimal_mac
        @corednsRegex = @corednsRegex + "|" + @corednsRegex_minimal_mac
        @cadvisorRegex = @cadvisorRegex + "|" + @cadvisorRegex_minimal_mac
        @kubeproxyRegex = @kubeproxyRegex + "|" + @kubeproxyRegex_minimal_mac
        @apiserverRegex = @apiserverRegex + "|" + @apiserverRegex_minimal_mac
        @kubestateRegex = @kubestateRegex + "|" + @kubestateRegex_minimal_mac
        @nodeexporterRegex = @nodeexporterRegex + "|" + @nodeexporterRegex_minimal_mac
        @kappiebasicRegex = @kappiebasicRegex + "|" + @kappiebasicRegex_minimal_mac
        @networkobservabilityRetinaRegex = @networkobservabilityRetinaRegex + "|" + @networkobservabilityRetinaRegex_minimal_mac
        @networkobservabilityHubbleRegex = @networkobservabilityHubbleRegex + "|" + @networkobservabilityHubbleRegex_minimal_mac
        @networkobservabilityCiliumRegex = @networkobservabilityCiliumRegex + "|" + @networkobservabilityCiliumRegex_minimal_mac
        @windowsexporterRegex = @windowsexporterRegex + "|" + @windowsexporterRegex_minimal_mac
        @windowskubeproxyRegex = @windowskubeproxyRegex + "|" + @windowskubeproxyRegex_minimal_mac
      else
        ConfigParseErrorLogger.log(LOGGING_PREFIX, "minimalIngestionProfile=true, MAC is not enabled. Applying appropriate non-MAC Regexes")
        @kubeletRegex = @kubeletRegex + "|" + @kubeletRegex_minimal
        @corednsRegex = @corednsRegex + "|" + @corednsRegex_minimal
        @cadvisorRegex = @cadvisorRegex + "|" + @cadvisorRegex_minimal
        @kubeproxyRegex = @kubeproxyRegex + "|" + @kubeproxyRegex_minimal
        @apiserverRegex = @apiserverRegex + "|" + @apiserverRegex_minimal
        @kubestateRegex = @kubestateRegex + "|" + @kubestateRegex_minimal
        @nodeexporterRegex = @nodeexporterRegex + "|" + @nodeexporterRegex_minimal
        @kappiebasicRegex = @kappiebasicRegex + "|" + @kappiebasicRegex_minimal
        @networkobservabilityRetinaRegex = @networkobservabilityRetinaRegex + "|" + @networkobservabilityRetinaRegex_minimal
        @networkobservabilityHubbleRegex = @networkobservabilityHubbleRegex + "|" + @networkobservabilityHubbleRegex_minimal
        @networkobservabilityCiliumRegex = @networkobservabilityCiliumRegex + "|" + @networkobservabilityCiliumRegex_minimal
        @windowsexporterRegex = @windowsexporterRegex + "|" + @windowsexporterRegex_minimal
        @windowskubeproxyRegex = @windowskubeproxyRegex + "|" + @windowskubeproxyRegex_minimal
      end
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while populating regex values with minimal ingestion profile - #{errorStr}, skipping applying minimal ingestion profile regexes")
  end
end

# ----End applying profile for ingestion--------

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

# Populate the regex values after reading the configmap settings based on the minimal ingestion profile value
populateRegexValuesWithMinimalIngestionProfile

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
regexHash["POD_ANNOTATION_METRICS_KEEP_LIST_REGEX"] = @podannotationRegex
regexHash["KAPPIEBASIC_METRICS_KEEP_LIST_REGEX"] = @kappiebasicRegex
regexHash["NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX"] = @networkobservabilityRetinaRegex
regexHash["NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX"] = @networkobservabilityHubbleRegex
regexHash["NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX"] = @networkobservabilityCiliumRegex

if !file.nil?
  # Close file after writing regex keep list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(regexHash.to_yaml)
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-metrics-keep-list regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-metrics-keep-list Processing")
