package configmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

var (
	configSchemaVersion                                                 string
	kubeletRegex, coreDNSRegex, cAdvisorRegex, kubeProxyRegex           string
	apiserverRegex, kubeStateRegex, nodeExporterRegex, kappieBasicRegex string
	windowsExporterRegex, windowsKubeProxyRegex                         string
	networkobservabilityRetinaRegex, networkobservabilityHubbleRegex    string
	networkobservabilityCiliumRegex, podAnnotationsRegex                string
	acstorCapacityProvisionerRegex, acstorMetricsExporterRegex          string
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
	windowsexporterRegex_minimal_mac           = "windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total"
	windowskubeproxyRegex_minimal_mac          = "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines"
	acstorCapacityProvisionerRegex_minimal_mac = "storage_pool_ready_state|storage_pool_capacity_used_bytes|storage_pool_capacity_provisioned_bytes|storage_pool_snapshot_capacity_reserved_bytes"
	acstorMetricsExporter_minimal_mac          = "disk_pool_ready_state|disk_read_operations_completed_total|disk_write_operations_total|disk_read_operations_time_seconds_total|disk_write_operations_time_seconds_total|disk_errors_total|disk_read_bytes_total|disk_written_bytes_total"
)

// getStringValue checks the type of the value and returns it as a string if possible.
func getStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v) // Convert boolean to string representation
	case nil:
		return ""
	default:
		// Handle other types if needed
		return fmt.Sprintf("%v", v) // Convert any other type to its default string representation
	}
}

func parseConfigMapForKeepListRegex() map[string]interface{} {
	configMap := make(map[string]interface{})
	configMap["minimalingestionprofile"] = "true"
	if _, err := os.Stat(configMapKeepListMountPath); os.IsNotExist(err) {
		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
		return configMap
	}

	content, err := os.ReadFile(configMapKeepListMountPath)
	if err != nil {
		fmt.Printf("Exception while parsing config map for default-targets-metrics-keep-list: %v, using defaults, please check config map for errors\n", err)
		return configMap
	}

	tree, err := toml.Load(string(content))
	if err != nil {
		fmt.Printf("Error parsing TOML: %v\n", err)
		return configMap
	}

	if minimalValue := getStringValue(tree.Get("minimalingestionprofile")); minimalValue != "" {
		configMap["minimalingestionprofile"] = minimalValue
	}

	configMap["kubelet"] = getStringValue(tree.Get("kubelet"))
	configMap["coredns"] = getStringValue(tree.Get("coredns"))
	configMap["cadvisor"] = getStringValue(tree.Get("cadvisor"))
	configMap["kubeproxy"] = getStringValue(tree.Get("kubeproxy"))
	configMap["apiserver"] = getStringValue(tree.Get("apiserver"))
	configMap["kubestate"] = getStringValue(tree.Get("kubestate"))
	configMap["nodeexporter"] = getStringValue(tree.Get("nodeexporter"))
	configMap["kappiebasic"] = getStringValue(tree.Get("kappiebasic"))
	configMap["windowsexporter"] = getStringValue(tree.Get("windowsexporter"))
	configMap["windowskubeproxy"] = getStringValue(tree.Get("windowskubeproxy"))
	configMap["networkobservabilityRetina"] = getStringValue(tree.Get("networkobservabilityRetina"))
	configMap["networkobservabilityHubble"] = getStringValue(tree.Get("networkobservabilityHubble"))
	configMap["networkobservabilityCilium"] = getStringValue(tree.Get("networkobservabilityCilium"))
	configMap["podannotations"] = getStringValue(tree.Get("podannotations"))
	configMap["acstor-capacity-provisioner"] = getStringValue(tree.Get("acstor-capacity-provisioner"))
	configMap["acstor-metrics-exporter"] = getStringValue(tree.Get("acstor-metrics-exporter"))

	fmt.Printf("Parsed config map for default-targets-metrics-keep-list: %v\n", configMap)

	// Print the content of the config map
	// fmt.Println("Content of the config map:")
	// for key, value := range configMap {
	// 	fmt.Printf("%s: %s\n", key, value)
	// }

	return configMap
}

func validateRegexValues(regexValues RegexValues) error {
	// Define a map of field names to their corresponding values
	fields := map[string]string{
		"kubelet":                     regexValues.kubelet,
		"coredns":                     regexValues.coredns,
		"cadvisor":                    regexValues.cadvisor,
		"kubeproxy":                   regexValues.kubeproxy,
		"apiserver":                   regexValues.apiserver,
		"kubestate":                   regexValues.kubestate,
		"nodeexporter":                regexValues.nodeexporter,
		"kappiebasic":                 regexValues.kappiebasic,
		"windowsexporter":             regexValues.windowsexporter,
		"windowskubeproxy":            regexValues.windowskubeproxy,
		"networkobservabilityretina":  regexValues.networkobservabilityretina,
		"networkobservabilityhubble":  regexValues.networkobservabilityhubble,
		"networkobservabilitycilium":  regexValues.networkobservabilitycilium,
		"podannotations":              regexValues.podannotations,
		"minimalingestionprofile":     regexValues.minimalingestionprofile,
		"acstor-capacity-provisioner": regexValues.acstorcapacityprovisioner,
		"acstor-metrics-exporter":     regexValues.acstormetricsexporter,
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
		kubelet:                    getStringValue(parsedConfig["kubelet"]),
		coredns:                    getStringValue(parsedConfig["coredns"]),
		cadvisor:                   getStringValue(parsedConfig["cadvisor"]),
		kubeproxy:                  getStringValue(parsedConfig["kubeproxy"]),
		apiserver:                  getStringValue(parsedConfig["apiserver"]),
		kubestate:                  getStringValue(parsedConfig["kubestate"]),
		nodeexporter:               getStringValue(parsedConfig["nodeexporter"]),
		kappiebasic:                getStringValue(parsedConfig["kappiebasic"]),
		windowsexporter:            getStringValue(parsedConfig["windowsexporter"]),
		windowskubeproxy:           getStringValue(parsedConfig["windowskubeproxy"]),
		networkobservabilityretina: getStringValue(parsedConfig["networkobservabilityRetina"]),
		networkobservabilityhubble: getStringValue(parsedConfig["networkobservabilityHubble"]),
		networkobservabilitycilium: getStringValue(parsedConfig["networkobservabilityCilium"]),
		minimalingestionprofile:    getStringValue(parsedConfig["minimalingestionprofile"]),
		podannotations:             getStringValue(parsedConfig["podannotations"]),
		acstorcapacityprovisioner:  getStringValue(parsedConfig["acstor-capacity-provisioner"]),
		acstormetricsexporter:      getStringValue(parsedConfig["acstor-metrics-exporter"]),
	}

	// Validate regex values
	if err := validateRegexValues(regexValues); err != nil {
		return regexValues, err
	}

	// Logging the values being set
	// fmt.Println("Values being set for regex:")
	// fmt.Printf("kubelet: %s\n", regexValues.kubelet)
	// fmt.Printf("coredns: %s\n", regexValues.coredns)
	// fmt.Printf("cadvisor: %s\n", regexValues.cadvisor)
	// fmt.Printf("kubeproxy: %s\n", regexValues.kubeproxy)
	// fmt.Printf("apiserver: %s\n", regexValues.apiserver)
	// fmt.Printf("kubestate: %s\n", regexValues.kubestate)
	// fmt.Printf("nodeexporter: %s\n", regexValues.nodeexporter)
	// fmt.Printf("kappiebasic: %s\n", regexValues.kappiebasic)
	// fmt.Printf("windowsexporter: %s\n", regexValues.windowsexporter)
	// fmt.Printf("windowskubeproxy: %s\n", regexValues.windowskubeproxy)
	// fmt.Printf("networkobservabilityretina: %s\n", regexValues.networkobservabilityretina)
	// fmt.Printf("networkobservabilityhubble: %s\n", regexValues.networkobservabilityhubble)
	// fmt.Printf("networkobservabilitycilium: %s\n", regexValues.networkobservabilitycilium)
	// fmt.Printf("minimalingestionprofile: %s\n", regexValues.minimalingestionprofile)
	return regexValues, nil // Return regex values and nil error if everything is valid
}

func populateRegexValuesWithMinimalIngestionProfile(regexValues RegexValues) {
	if regexValues.minimalingestionprofile == "true" {
		kubeletRegex = fmt.Sprintf("%s|%s", regexValues.kubelet, kubeletRegex_minimal_mac)
		coreDNSRegex = fmt.Sprintf("%s|%s", regexValues.coredns, coreDNSRegex_minimal_mac)
		cAdvisorRegex = fmt.Sprintf("%s|%s", regexValues.cadvisor, cadvisorRegex_minimal_mac)
		kubeProxyRegex = fmt.Sprintf("%s|%s", regexValues.kubeproxy, kubeproxyRegex_minimal_mac)
		apiserverRegex = fmt.Sprintf("%s|%s", regexValues.apiserver, apiserverRegex_minimal_mac)
		kubeStateRegex = fmt.Sprintf("%s|%s", regexValues.kubestate, kubestateRegex_minimal_mac)
		nodeExporterRegex = fmt.Sprintf("%s|%s", regexValues.nodeexporter, nodeexporterRegex_minimal_mac)
		kappieBasicRegex = fmt.Sprintf("%s|%s", regexValues.kappiebasic, kappiebasicRegex_minimal_mac)
		windowsExporterRegex = fmt.Sprintf("%s|%s", regexValues.windowsexporter, windowsexporterRegex_minimal_mac)
		windowsKubeProxyRegex = fmt.Sprintf("%s|%s", regexValues.windowskubeproxy, windowskubeproxyRegex_minimal_mac)
		networkobservabilityRetinaRegex = fmt.Sprintf("%s|%s", regexValues.networkobservabilityretina, networkobservabilityRetinaRegex_minimal_mac)
		networkobservabilityHubbleRegex = fmt.Sprintf("%s|%s", regexValues.networkobservabilityhubble, networkobservabilityHubbleRegex_minimal_mac)
		networkobservabilityCiliumRegex = regexValues.networkobservabilitycilium
		podAnnotationsRegex = regexValues.podannotations
		acstorCapacityProvisionerRegex = fmt.Sprintf("%s|%s", regexValues.acstorcapacityprovisioner, acstorCapacityProvisionerRegex_minimal_mac)
		acstorMetricsExporterRegex = fmt.Sprintf("%s|%s", regexValues.acstormetricsexporter, acstorMetricsExporter_minimal_mac)

		// Print the updated regex strings after appending values
		// Only log this in debug mode
		// fmt.Println("Updated Regex Strings After Appending:")
		// fmt.Println("KubeletRegex:", kubeletRegex)
		// fmt.Println("CoreDNSRegex:", coreDNSRegex)
		// fmt.Println("CAdvisorRegex:", cAdvisorRegex)
		// fmt.Println("KubeProxyRegex:", kubeProxyRegex)
		// fmt.Println("APIServerRegex:", apiserverRegex)
		// fmt.Println("KubeStateRegex:", kubeStateRegex)
		// fmt.Println("NodeExporterRegex:", nodeExporterRegex)
		// fmt.Println("KappieBasicRegex:", kappieBasicRegex)
		// fmt.Println("WindowsExporterRegex:", windowsExporterRegex)
		// fmt.Println("WindowsKubeProxyRegex:", windowsKubeProxyRegex)
		// fmt.Println("NetworkObservabilityRetinaRegex:", networkobservabilityRetinaRegex)
		// fmt.Println("NetworkObservabilityRetinaRegex:", networkobservabilityRetinaRegex)
		// fmt.Println("NetworkObservabilityCiliumRegex:", networkobservabilityCiliumRegex)
	} else {
		fmt.Println("minimalIngestionProfile:", regexValues.minimalingestionprofile)

		kubeletRegex = regexValues.kubelet
		coreDNSRegex = regexValues.coredns
		cAdvisorRegex = regexValues.cadvisor
		kubeProxyRegex = regexValues.kubeproxy
		apiserverRegex = regexValues.apiserver
		kubeStateRegex = regexValues.kubestate
		nodeExporterRegex = regexValues.nodeexporter
		kappieBasicRegex = regexValues.kappiebasic
		windowsExporterRegex = regexValues.windowsexporter
		windowsKubeProxyRegex = regexValues.windowskubeproxy
		networkobservabilityRetinaRegex = regexValues.networkobservabilityretina
		networkobservabilityHubbleRegex = regexValues.networkobservabilityhubble
		networkobservabilityCiliumRegex = regexValues.networkobservabilitycilium
		podAnnotationsRegex = regexValues.podannotations
		acstorCapacityProvisionerRegex = regexValues.acstorcapacityprovisioner
		acstorMetricsExporterRegex = regexValues.acstormetricsexporter
	}
}

func tomlparserTargetsMetricsKeepList() {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	shared.EchoSectionDivider("Start Processing - tomlparserTargetsMetricsKeepList")

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
		"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             podAnnotationsRegex,
		"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                kappieBasicRegex,
		"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": networkobservabilityRetinaRegex,
		"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": networkobservabilityHubbleRegex,
		"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": networkobservabilityCiliumRegex,
		"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           acstorCapacityProvisionerRegex,
		"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              acstorMetricsExporterRegex,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile(configMapKeepListEnvVarPath, []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	shared.EchoSectionDivider("End Processing - tomlparserTargetsMetricsKeepList")
}
