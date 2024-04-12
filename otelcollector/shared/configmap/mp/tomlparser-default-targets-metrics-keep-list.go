package configmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

var (
	configMapKeepListMountPath                                          = "/etc/config/settings/default-targets-metrics-keep-list"
	configSchemaVersion                                                 string
	kubeletRegex, coreDNSRegex, cAdvisorRegex, kubeProxyRegex           string
	apiserverRegex, kubeStateRegex, nodeExporterRegex, kappieBasicRegex string
	netObservabilityRegex, windowsExporterRegex, windowsKubeProxyRegex  string
	networkobservabilityRetinaRegex, networkobservabilityHubbleRegex    string
	networkobservabilityCiliumRegex                                     string
	kubeletRegex_minimal_mac                                            = "kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes|kubelet_node_name|kubelet_running_pods|kubelet_running_pod_count|kubelet_running_sum_containers|kubelet_running_containers|kubelet_running_container_count|volume_manager_total_volumes|kubelet_node_config_error|kubelet_runtime_operations_total|kubelet_runtime_operations_errors_total|kubelet_runtime_operations_duration_seconds_bucket|kubelet_runtime_operations_duration_seconds_sum|kubelet_runtime_operations_duration_seconds_count|kubelet_pod_start_duration_seconds_bucket|kubelet_pod_start_duration_seconds_sum|kubelet_pod_start_duration_seconds_count|kubelet_pod_worker_duration_seconds_bucket|kubelet_pod_worker_duration_seconds_sum|kubelet_pod_worker_duration_seconds_count|storage_operation_duration_seconds_bucket|storage_operation_duration_seconds_sum|storage_operation_duration_seconds_count|storage_operation_errors_total|kubelet_cgroup_manager_duration_seconds_bucket|kubelet_cgroup_manager_duration_seconds_sum|kubelet_cgroup_manager_duration_seconds_count|kubelet_pleg_relist_interval_seconds_bucket|kubelet_pleg_relist_interval_seconds_count|kubelet_pleg_relist_interval_seconds_sum|kubelet_pleg_relist_duration_seconds_bucket|kubelet_pleg_relist_duration_seconds_count|kubelet_pleg_relist_duration_seconds_sum|rest_client_requests_total|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|kubelet_certificate_manager_client_ttl_seconds|kubelet_certificate_manager_client_expiration_renew_errors|kubelet_server_expiration_renew_errors|kubelet_certificate_manager_server_ttl_seconds|kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_inodes_free|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes|kube_persistentvolumeclaim_access_mode|kube_persistentvolumeclaim_labels|kube_persistentvolume_status_phase"
	coreDNSRegex_minimal_mac                                            = "coredns_build_info|coredns_panics_total|coredns_dns_responses_total|coredns_forward_responses_total|coredns_dns_request_duration_seconds|coredns_dns_request_duration_seconds_bucket|coredns_dns_request_duration_seconds_sum|coredns_dns_request_duration_seconds_count|coredns_forward_request_duration_seconds|coredns_forward_request_duration_seconds_bucket|coredns_forward_request_duration_seconds_sum|coredns_forward_request_duration_seconds_count|coredns_dns_requests_total|coredns_forward_requests_total|coredns_cache_hits_total|coredns_cache_misses_total|coredns_cache_entries|coredns_plugin_enabled|coredns_dns_request_size_bytes|coredns_dns_request_size_bytes_bucket|coredns_dns_request_size_bytes_sum|coredns_dns_request_size_bytes_count|coredns_dns_response_size_bytes|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
	cadvisorRegex_minimal_mac                                           = "container_spec_cpu_quota|container_spec_cpu_period|container_memory_rss|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_network_receive_packets_total|container_network_transmit_packets_total|container_network_receive_packets_dropped_total|container_network_transmit_packets_dropped_total|container_fs_reads_total|container_fs_writes_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_cpu_usage_seconds_total|container_memory_working_set_bytes|container_memory_cache|container_memory_swap|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_periods_total|container_memory_rss|kubernetes_build_info|container_start_time_seconds"
	kubeproxyRegex_minimal_mac                                          = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_network_programming_duration_seconds|kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_network_programming_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info"
	apiserverRegex_minimal_mac                                          = "apiserver_request_duration_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_sum|apiserver_request_duration_seconds_count|apiserver_request_total|workqueue_adds_total|workqueue_depth|workqueue_queue_duration_seconds|workqueue_queue_duration_seconds_bucket|workqueue_queue_duration_seconds_sum|workqueue_queue_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count"
	kubestateRegex_minimal_mac                                          = "kube_job_status_succeeded|kube_job_spec_completions|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_number_misscheduled|kube_daemonset_status_number_ready|kube_deployment_status_replicas_ready|kube_pod_container_status_last_terminated_reason|kube_pod_container_status_waiting_reason|kube_pod_container_status_restarts_total|kube_node_status_allocatable|kube_pod_owner|kube_pod_container_resource_requests|kube_pod_status_phase|kube_pod_container_resource_limits|kube_replicaset_owner|kube_resourcequota|kube_namespace_status_phase|kube_node_status_capacity|kube_node_info|kube_pod_info|kube_deployment_spec_replicas|kube_deployment_status_replicas_available|kube_deployment_status_replicas_updated|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas|kube_statefulset_status_replicas_updated|kube_job_status_start_time|kube_job_status_active|kube_job_failed|kube_horizontalpodautoscaler_status_desired_replicas|kube_horizontalpodautoscaler_status_current_replicas|kube_horizontalpodautoscaler_spec_min_replicas|kube_horizontalpodautoscaler_spec_max_replicas|kubernetes_build_info|kube_node_status_condition|kube_node_spec_taint|kube_pod_container_info|kube_.*_labels|kube_.*_annotations|kube_service_info|kube_pod_container_status_running|kube_pod_container_status_waiting|kube_pod_container_status_terminated|kube_pod_container_state_started|kube_pod_created|kube_pod_start_time|kube_pod_init_container_info|kube_pod_init_container_status_terminated|kube_pod_init_container_status_terminated_reason|kube_pod_init_container_status_ready|kube_pod_init_container_resource_limits|kube_pod_init_container_status_running|kube_pod_init_container_status_waiting|kube_pod_init_container_status_restarts_total|kube_pod_container_status_ready|kube_pod_init_container_*|kube_pod_deletion_timestamp|kube_pod_status_reason|kube_pod_init_container_resource_requests"
	nodeexporterRegex_minimal_mac                                       = "node_filesystem_readonly|node_memory_MemTotal_bytes|node_cpu_seconds_total|node_memory_MemAvailable_bytes|node_memory_Buffers_bytes|node_memory_Cached_bytes|node_memory_MemFree_bytes|node_memory_Slab_bytes|node_filesystem_avail_bytes|node_filesystem_size_bytes|node_time_seconds|node_exporter_build_info|node_load1|node_vmstat_pgmajfault|node_network_receive_bytes_total|node_network_transmit_bytes_total|node_network_receive_drop_total|node_network_transmit_drop_total|node_disk_io_time_seconds_total|node_disk_io_time_weighted_seconds_total|node_load5|node_load15|node_disk_read_bytes_total|node_disk_written_bytes_total|node_uname_info|kubernetes_build_info|node_boot_time_seconds"
	kappiebasicRegex_minimal_mac                                        = "kappie.*"
	networkobservabilityRetinaRegex_minimal_mac                         = "networkobservability.*"
	networkobservabilityHubbleRegex_minimal_mac                         = "hubble.*"
	// networkobservabilityCiliumRegex_minimal_mac                         = "cilium_drop.*|cilium_forward.*"
	windowsexporterRegex_minimal_mac  = "windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total"
	windowskubeproxyRegex_minimal_mac = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"
)

// getStringValue checks the type of the value and returns it as a string if possible.
func getStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v) // Convert boolean to string representation
	default:
		// Handle other types if needed
		return fmt.Sprintf("%v", v) // Convert any other type to its default string representation
	}
}

func parseConfigMapForKeepListRegex() map[string]interface{} {
	if _, err := os.Stat(configMapKeepListMountPath); os.IsNotExist(err) {
		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
		return nil
	}

	content, err := os.ReadFile(configMapKeepListMountPath)
	if err != nil {
		fmt.Printf("Exception while parsing config map for default-targets-metrics-keep-list: %v, using defaults, please check config map for errors\n", err)
		return nil
	}

	// Print the content read from the file
	fmt.Println("Content of the file:")
	fmt.Println(string(content))

	tree, err := toml.Load(string(content))
	if err != nil {
		fmt.Printf("Error parsing TOML: %v\n", err)
		return nil
	}

	configMap := make(map[string]interface{})
	configMap["kubelet"] = getStringValue(tree.Get("kubelet"))
	configMap["coredns"] = getStringValue(tree.Get("coredns"))
	configMap["cadvisor"] = getStringValue(tree.Get("cadvisor"))
	configMap["kubeproxy"] = getStringValue(tree.Get("kubeproxy"))
	configMap["apiserver"] = getStringValue(tree.Get("apiserver"))
	configMap["kubestate"] = getStringValue(tree.Get("kubestate"))
	configMap["nodeexporter"] = getStringValue(tree.Get("nodeexporter"))
	configMap["kappiebasic"] = getStringValue(tree.Get("kappiebasic"))
	configMap["windowsexporter"] = getStringValue(tree.Get("windowswxporter"))
	configMap["windowskubeproxy"] = getStringValue(tree.Get("windowskubeproxy"))
	configMap["networkobservabilityretinaregex"] = getStringValue(tree.Get("networkobservabilityretinaregex"))
	configMap["networkobservabilityhubbleregex"] = getStringValue(tree.Get("networkobservabilityhubbleregex"))
	configMap["minimalingestionprofile"] = getStringValue(tree.Get("minimalingestionprofile"))

	// Print the content of the config map
	fmt.Println("Content of the config map:")
	for key, value := range configMap {
		fmt.Printf("%s: %s\n", key, value)
	}

	return configMap
}

func validateRegexValues(regexValues RegexValues) error {
	// Define a map of field names to their corresponding values
	fields := map[string]string{
		"kubelet":                         regexValues.kubelet,
		"coredns":                         regexValues.coredns,
		"cadvisor":                        regexValues.cadvisor,
		"kubeproxy":                       regexValues.kubeproxy,
		"apiserver":                       regexValues.apiserver,
		"kubestate":                       regexValues.kubestate,
		"nodeexporter":                    regexValues.nodeexporter,
		"kappiebasic":                     regexValues.kappiebasic,
		"netobservability":                regexValues.netobservability,
		"windowsexporter":                 regexValues.windowsexporter,
		"windowskubeproxy":                regexValues.windowskubeproxy,
		"networkobservabilityretinaregex": regexValues.networkobservabilityretinaregex,
		"networkobservabilityhubbleregex": regexValues.networkobservabilityhubbleregex,
		"minimalingestionprofile":         regexValues.minimalingestionprofile,
	}

	// Iterate over the fields and validate each regex
	for key, value := range fields {
		if value != "" && !isValidRegex(value) {
			return fmt.Errorf("invalid regex for %s: %s", key, value)
		}
	}

	return nil
}

func populateKeepListFromConfigMap(parsedConfig map[string]interface{}) (RegexValues, error) {
	regexValues := RegexValues{
		kubelet:                         getStringValue(parsedConfig["kubelet"]),
		coredns:                         getStringValue(parsedConfig["coredns"]),
		cadvisor:                        getStringValue(parsedConfig["cadvisor"]),
		kubeproxy:                       getStringValue(parsedConfig["kubeproxy"]),
		apiserver:                       getStringValue(parsedConfig["apiserver"]),
		kubestate:                       getStringValue(parsedConfig["kubestate"]),
		nodeexporter:                    getStringValue(parsedConfig["nodeexporter"]),
		kappiebasic:                     getStringValue(parsedConfig["kappiebasic"]),
		netobservability:                getStringValue(parsedConfig["netobservability"]),
		windowsexporter:                 getStringValue(parsedConfig["windowsexporter"]),
		windowskubeproxy:                getStringValue(parsedConfig["windowskubeproxy"]),
		networkobservabilityretinaregex: getStringValue(parsedConfig["networkobservabilityretinaregex"]),
		networkobservabilityhubbleregex: getStringValue(parsedConfig["networkobservabilityhubbleregex"]),
		minimalingestionprofile:         getStringValue(parsedConfig["minimalingestionprofile"]),
	}

	// Validate regex values
	if err := validateRegexValues(regexValues); err != nil {
		return regexValues, err
	}

	// Logging the values being set
	fmt.Println("Values being set:")
	fmt.Printf("kubelet: %s\n", regexValues.kubelet)
	fmt.Printf("coredns: %s\n", regexValues.coredns)
	fmt.Printf("cadvisor: %s\n", regexValues.cadvisor)
	fmt.Printf("kubeproxy: %s\n", regexValues.kubeproxy)
	fmt.Printf("apiserver: %s\n", regexValues.apiserver)
	fmt.Printf("kubestate: %s\n", regexValues.kubestate)
	fmt.Printf("nodeexporter: %s\n", regexValues.nodeexporter)
	fmt.Printf("kappiebasic: %s\n", regexValues.kappiebasic)
	fmt.Printf("netobservability: %s\n", regexValues.netobservability)
	fmt.Printf("windowsexporter: %s\n", regexValues.windowsexporter)
	fmt.Printf("windowskubeproxy: %s\n", regexValues.windowskubeproxy)
	fmt.Printf("networkobservabilityretinaregex: %s\n", regexValues.networkobservabilityretinaregex)
	fmt.Printf("networkobservabilityhubbleregex: %s\n", regexValues.networkobservabilityhubbleregex)
	fmt.Printf("minimalingestionprofile: %s\n", regexValues.minimalingestionprofile)
	return regexValues, nil // Return regex values and nil error if everything is valid
}

func populateRegexValuesWithMinimalIngestionProfile(regexValues RegexValues) {
	if regexValues.minimalingestionprofile == "true" {
		kubeletRegex += regexValues.kubelet + "|" + kubeletRegex_minimal_mac
		coreDNSRegex += regexValues.coredns + "|" + coreDNSRegex_minimal_mac
		cAdvisorRegex += regexValues.cadvisor + "|" + cadvisorRegex_minimal_mac
		kubeProxyRegex += regexValues.kubeproxy + "|" + kubeproxyRegex_minimal_mac
		apiserverRegex += regexValues.apiserver + "|" + apiserverRegex_minimal_mac
		kubeStateRegex += regexValues.kubestate + "|" + kubestateRegex_minimal_mac
		nodeExporterRegex += regexValues.nodeexporter + "|" + nodeexporterRegex_minimal_mac
		kappieBasicRegex += regexValues.kappiebasic + "|" + kappiebasicRegex_minimal_mac
		windowsExporterRegex += regexValues.windowsexporter + "|" + windowsexporterRegex_minimal_mac
		windowsKubeProxyRegex += regexValues.windowskubeproxy + "|" + windowskubeproxyRegex_minimal_mac
		networkobservabilityRetinaRegex += regexValues.networkobservabilityretinaregex + "|" + networkobservabilityRetinaRegex_minimal_mac
		networkobservabilityHubbleRegex += regexValues.networkobservabilityhubbleregex + "|" + networkobservabilityHubbleRegex_minimal_mac

		// Print the updated regex strings after appending values
		// Only log this in debug mode
		fmt.Println("Updated Regex Strings After Appending:")
		fmt.Println("KubeletRegex:", kubeletRegex)
		fmt.Println("CoreDNSRegex:", coreDNSRegex)
		fmt.Println("CAdvisorRegex:", cAdvisorRegex)
		fmt.Println("KubeProxyRegex:", kubeProxyRegex)
		fmt.Println("APIServerRegex:", apiserverRegex)
		fmt.Println("KubeStateRegex:", kubeStateRegex)
		fmt.Println("NodeExporterRegex:", nodeExporterRegex)
		fmt.Println("KappieBasicRegex:", kappieBasicRegex)
		fmt.Println("NetObservabilityRegex:", netObservabilityRegex)
		fmt.Println("WindowsExporterRegex:", windowsExporterRegex)
		fmt.Println("WindowsKubeProxyRegex:", windowsKubeProxyRegex)
		fmt.Println("NetworkObservabilityRetinaRegex:", networkobservabilityRetinaRegex)
		fmt.Println("NetworkObservabilityHubbleRegex:", networkobservabilityHubbleRegex)
	} else {
		fmt.Println("minimalIngestionProfile:", regexValues.minimalingestionprofile)
	}
}

func tomlparserTargetsMetricsKeepList() {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	var regexValues RegexValues

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForKeepListRegex()
		if configMapSettings != nil {
			var err error
			regexValues, err = populateKeepListFromConfigMap(configMapSettings)
			if err != nil {
				fmt.Printf("Error populating setting values: %v\n", err)
				return
			}
		}
	} else {
		if _, err := os.Stat(configMapKeepListMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	populateRegexValuesWithMinimalIngestionProfile(regexValues)

	// Write settings to a YAML file.
	data := map[string]string{
		"KUBELET_METRICS_KEEP_LIST_REGEX":                    kubeletRegex,
		"COREDNS_METRICS_KEEP_LIST_REGEX":                    coreDNSRegex,
		"CADVISOR_METRICS_KEEP_LIST_REGEX":                   cAdvisorRegex,
		"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  kubeProxyRegex,
		"APISERVER_METRICS_KEEP_LIST_REGEX":                  apiserverRegex,
		"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  kubeStateRegex,
		"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               nodeExporterRegex,
		"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            windowsExporterRegex,
		"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           windowsKubeProxyRegex,
		"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             podannotationNamespaceRegex,
		"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                kappieBasicRegex,
		"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": networkobservabilityRetinaRegex,
		"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": networkobservabilityHubbleRegex,
		"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": networkobservabilityCiliumRegex,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	fmt.Println("End default-targets-metrics-keep-list Processing")
}
