package shared

import "strings"

type DefaultScrapeJob struct {
	JobName                    string
	Enabled                    bool
	OSType                     string
	KubernetesPlane            string
	ControllerType             string
	ScrapeConfigDefinitionFile string
	PlaceholderNames           []string
	MinimalKeepListRegex       string
	CustomerKeepListRegex      string
	KeepListRegex              string
	ScrapeInterval             string
}

type ControllerTypeStrings struct {
	ReplicaSet          string
	DaemonSet           string
	ConfigReaderSidecar string
}

var ControllerType = ControllerTypeStrings{
	ReplicaSet:          "ReplicaSet",
	DaemonSet:           "DaemonSet",
	ConfigReaderSidecar: "ConfigReaderSidecar",
}

type OSTypeStrings struct {
	Linux   string
	Windows string
}

var OSType = OSTypeStrings{
	Linux:   "linux",
	Windows: "windows",
}

type KubernetesPlaneStrings struct {
	ControlPlane string
	DataPlane    string
}

var KubernetesPlane = KubernetesPlaneStrings{
	ControlPlane: "controlplane",
	DataPlane:    "dataplane",
}

type SchemaVersionStrings struct {
	V1  string
	V2  string
	Nil string
}

var SchemaVersion = SchemaVersionStrings{
	V1:  "v1",
	V2:  "v2",
	Nil: "",
}

func ParseSchemaVersion(schema string) string {
	sanitizedSchema := strings.ToLower(strings.TrimSpace(schema))
	switch sanitizedSchema {
	case "v1":
		return SchemaVersion.V1
	case "v2":
		return SchemaVersion.V2
	default:
		return SchemaVersion.Nil
	}
}

var DefaultScrapeJobs = map[string]*DefaultScrapeJob{
	"kubelet": {
		JobName:                    "kubelet",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "kubeletDefaultDs.yml",
		PlaceholderNames:           []string{"NODE_NAME", "NODE_IP", "OS_TYPE"},
		MinimalKeepListRegex:       "kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_used_bytes|kubelet_node_name|kubelet_running_pods|kubelet_running_pod_count|kubelet_running_sum_containers|kubelet_running_containers|kubelet_running_container_count|volume_manager_total_volumes|kubelet_node_config_error|kubelet_runtime_operations_total|kubelet_runtime_operations_errors_total|kubelet_runtime_operations_duration_seconds_bucket|kubelet_runtime_operations_duration_seconds_sum|kubelet_runtime_operations_duration_seconds_count|kubelet_pod_start_duration_seconds_bucket|kubelet_pod_start_duration_seconds_sum|kubelet_pod_start_duration_seconds_count|kubelet_pod_worker_duration_seconds_bucket|kubelet_pod_worker_duration_seconds_sum|kubelet_pod_worker_duration_seconds_count|storage_operation_duration_seconds_bucket|storage_operation_duration_seconds_sum|storage_operation_duration_seconds_count|storage_operation_errors_total|kubelet_cgroup_manager_duration_seconds_bucket|kubelet_cgroup_manager_duration_seconds_sum|kubelet_cgroup_manager_duration_seconds_count|kubelet_pleg_relist_interval_seconds_bucket|kubelet_pleg_relist_interval_seconds_count|kubelet_pleg_relist_interval_seconds_sum|kubelet_pleg_relist_duration_seconds_bucket|kubelet_pleg_relist_duration_seconds_count|kubelet_pleg_relist_duration_seconds_sum|rest_client_requests_total|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|kubelet_certificate_manager_client_ttl_seconds|kubelet_certificate_manager_client_expiration_renew_errors|kubelet_server_expiration_renew_errors|kubelet_certificate_manager_server_ttl_seconds|kubelet_volume_stats_available_bytes|kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_inodes_free|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes|kube_persistentvolumeclaim_access_mode|kube_persistentvolumeclaim_labels|kube_persistentvolume_status_phase",
		ScrapeInterval:             "30s",
	},
	"coredns": {
		JobName:                    "coredns",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "corednsDefault.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "coredns_build_info|coredns_panics_total|coredns_dns_responses_total|coredns_forward_responses_total|coredns_dns_request_duration_seconds|coredns_dns_request_duration_seconds_bucket|coredns_dns_request_duration_seconds_sum|coredns_dns_request_duration_seconds_count|coredns_forward_request_duration_seconds|coredns_forward_request_duration_seconds_bucket|coredns_forward_request_duration_seconds_sum|coredns_forward_request_duration_seconds_count|coredns_dns_requests_total|coredns_forward_requests_total|coredns_cache_hits_total|coredns_cache_misses_total|coredns_cache_entries|coredns_plugin_enabled|coredns_dns_request_size_bytes|coredns_dns_request_size_bytes_bucket|coredns_dns_request_size_bytes_sum|coredns_dns_request_size_bytes_count|coredns_dns_response_size_bytes|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|coredns_dns_response_size_bytes_bucket|coredns_dns_response_size_bytes_sum|coredns_dns_response_size_bytes_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"cadvisor": {
		JobName:                    "cadvisor",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "cadvisorDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "container_spec_cpu_quota|container_spec_cpu_period|container_memory_rss|container_network_receive_bytes_total|container_network_transmit_bytes_total|container_network_receive_packets_total|container_network_transmit_packets_total|container_network_receive_packets_dropped_total|container_network_transmit_packets_dropped_total|container_fs_reads_total|container_fs_writes_total|container_fs_reads_bytes_total|container_fs_writes_bytes_total|container_cpu_usage_seconds_total|container_memory_working_set_bytes|container_memory_cache|container_memory_swap|container_cpu_cfs_throttled_periods_total|container_cpu_cfs_periods_total|container_memory_rss|kubernetes_build_info|container_start_time_seconds",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"kubeproxy": {
		JobName:                    "kubeproxy",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "kubeproxyDefault.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|kubeproxy_network_programming_duration_seconds|kubeproxy_network_programming_duration_seconds_bucket|kubeproxy_network_programming_duration_seconds_sum|kubeproxy_network_programming_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"apiserver": {
		JobName:                    "apiserver",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "apiserverDefault.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "apiserver_request_duration_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_sum|apiserver_request_duration_seconds_count|apiserver_request_total|workqueue_adds_total|workqueue_depth|workqueue_queue_duration_seconds|workqueue_queue_duration_seconds_bucket|workqueue_queue_duration_seconds_sum|workqueue_queue_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines|kubernetes_build_info|apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"kubestate": {
		JobName:                    "kubestate",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "kubestateDefault.yml",
		PlaceholderNames:           []string{"KUBE_STATE_NAME", "POD_NAMESPACE"},
		MinimalKeepListRegex:       "kube_job_status_succeeded|kube_job_spec_completions|kube_daemonset_status_desired_number_scheduled|kube_daemonset_status_current_number_scheduled|kube_daemonset_status_number_misscheduled|kube_daemonset_status_number_ready|kube_deployment_status_replicas_ready|kube_pod_container_status_last_terminated_reason|kube_pod_container_status_waiting_reason|kube_pod_container_status_restarts_total|kube_node_status_allocatable|kube_pod_owner|kube_pod_container_resource_requests|kube_pod_status_phase|kube_pod_container_resource_limits|kube_replicaset_owner|kube_resourcequota|kube_namespace_status_phase|kube_node_status_capacity|kube_node_info|kube_pod_info|kube_deployment_spec_replicas|kube_deployment_status_replicas_available|kube_deployment_status_replicas_updated|kube_statefulset_status_replicas_ready|kube_statefulset_status_replicas|kube_statefulset_status_replicas_updated|kube_job_status_start_time|kube_job_status_active|kube_job_failed|kube_horizontalpodautoscaler_status_desired_replicas|kube_horizontalpodautoscaler_status_current_replicas|kube_horizontalpodautoscaler_spec_min_replicas|kube_horizontalpodautoscaler_spec_max_replicas|kubernetes_build_info|kube_node_status_condition|kube_node_spec_taint|kube_pod_container_info|kube_.*_labels|kube_.*_annotations|kube_service_info|kube_pod_container_status_running|kube_pod_container_status_waiting|kube_pod_container_status_terminated|kube_pod_container_state_started|kube_pod_created|kube_pod_start_time|kube_pod_init_container_info|kube_pod_init_container_status_terminated|kube_pod_init_container_status_terminated_reason|kube_pod_init_container_status_ready|kube_pod_init_container_resource_limits|kube_pod_init_container_status_running|kube_pod_init_container_status_waiting|kube_pod_init_container_status_restarts_total|kube_pod_container_status_ready|kube_pod_init_container_*|kube_pod_deletion_timestamp|kube_pod_status_reason|kube_pod_init_container_resource_requests",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"nodeexporter": {
		JobName:                    "nodeexporter",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "nodeexporterDefaultDs.yml",
		PlaceholderNames:           []string{"NODE_NAME", "NODE_IP", "NODE_EXPORTER_TARGETPORT"},
		MinimalKeepListRegex:       "node_filesystem_readonly|node_memory_MemTotal_bytes|node_cpu_seconds_total|node_memory_MemAvailable_bytes|node_memory_Buffers_bytes|node_memory_Cached_bytes|node_memory_MemFree_bytes|node_memory_Slab_bytes|node_filesystem_avail_bytes|node_filesystem_size_bytes|node_time_seconds|node_exporter_build_info|node_load1|node_vmstat_pgmajfault|node_network_receive_bytes_total|node_network_transmit_bytes_total|node_network_receive_drop_total|node_network_transmit_drop_total|node_disk_io_time_seconds_total|node_disk_io_time_weighted_seconds_total|node_load5|node_load15|node_disk_read_bytes_total|node_disk_written_bytes_total|node_uname_info|kubernetes_build_info|node_boot_time_seconds",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"kappiebasic": {
		JobName:                    "kappiebasic",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "kappieBasicDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "kappie.*",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"windowsexporter": {
		JobName:                    "windowsexporter",
		Enabled:                    false,
		OSType:                     OSType.Windows,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "windowsexporterDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "windows_system_system_up_time|windows_cpu_time_total|windows_memory_available_bytes|windows_os_visible_memory_bytes|windows_memory_cache_bytes|windows_memory_modified_page_list_bytes|windows_memory_standby_cache_core_bytes|windows_memory_standby_cache_normal_priority_bytes|windows_memory_standby_cache_reserve_bytes|windows_memory_swap_page_operations_total|windows_logical_disk_read_seconds_total|windows_logical_disk_write_seconds_total|windows_logical_disk_size_bytes|windows_logical_disk_free_bytes|windows_net_bytes_total|windows_net_packets_received_discarded_total|windows_net_packets_outbound_discarded_total|windows_container_available|windows_container_cpu_usage_seconds_total|windows_container_memory_usage_commit_bytes|windows_container_memory_usage_private_working_set_bytes|windows_container_network_receive_bytes_total|windows_container_network_transmit_bytes_total",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"windowskubeproxy": {
		JobName:                    "windowskubeproxy",
		Enabled:                    false,
		OSType:                     OSType.Windows,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "windowskubeproxyDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "kubeproxy_sync_proxy_rules_duration_seconds|kubeproxy_sync_proxy_rules_duration_seconds_bucket|kubeproxy_sync_proxy_rules_duration_seconds_sum|kubeproxy_sync_proxy_rules_duration_seconds_count|rest_client_requests_total|rest_client_request_duration_seconds|rest_client_request_duration_seconds_bucket|rest_client_request_duration_seconds_sum|rest_client_request_duration_seconds_count|process_resident_memory_bytes|process_cpu_seconds_total|go_goroutines",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"networkobservabilityRetina": {
		JobName:                    "networkobservabilityRetina",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "networkobservabilityRetinaDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "networkobservability.*",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"networkobservabilityHubble": {
		JobName:                    "networkobservabilityHubble",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "networkobservabilityHubbleDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "hubble.*",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"networkobservabilityCilium": {
		JobName:                    "networkobservabilityCilium",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.DaemonSet,
		ScrapeConfigDefinitionFile: "networkobservabilityCiliumDefaultDs.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "cilium_drop.*|cilium_forward.*",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"podannotations": {
		JobName:                    "podannotations",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "podannotationsDefault.yml",
		PlaceholderNames:           []string{"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"},
		MinimalKeepListRegex:       "",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"acstor-capacity-provisioner": {
		JobName:                    "acstor-capacity-provisioner",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "acstorCapacityProvisionerDefaultFile.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "storage_pool_ready_state|storage_pool_capacity_used_bytes|storage_pool_capacity_provisioned_bytes|storage_pool_snapshot_capacity_reserved_bytes",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"acstor-metrics-exporter": {
		JobName:                    "acstor-metrics-exporter",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "acstorMetricsExporterDefaultFile.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "disk_pool_ready_state|disk_read_operations_completed_total|disk_write_operations_completed_total|disk_read_operations_time_seconds_total|disk_write_operations_time_seconds_total|disk_errors_total|disk_read_bytes_total|disk_written_bytes_total|disk_readonly_errors_gauge",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"prometheuscollectorhealth": {
		JobName:                    "prometheuscollectorhealth",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.DataPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "prometheusCollectorHealthDefault.yml",
		PlaceholderNames:           []string{},
		MinimalKeepListRegex:       "",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
}

var ControlPlaneDefaultScrapeJobs = map[string]*DefaultScrapeJob{
	"apiserver": {
		JobName:                    "apiserver",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_apiserver.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_request_sli_duration_seconds_count|apiserver_request_sli_duration_seconds_sum|process_start_time_seconds|apiserver_request_duration_seconds_count|apiserver_request_duration_seconds_sum|apiserver_storage_list_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"cluster-autoscaler": {
		JobName:                    "cluster-autoscaler",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_cluster_autoscaler.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "rest_client_requests_total|cluster_autoscaler_(last_activity|cluster_safe_to_autoscale|scale_down_in_cooldown|scaled_up_nodes_total|unneeded_nodes_count|unschedulable_pods_count|nodes_count)|cloudprovider_azure_api_request_(errors|duration_seconds_(bucket|count))",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"kube-scheduler": {
		JobName:                    "kube-scheduler",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_kube_scheduler.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "scheduler_pending_pods|scheduler_unschedulable_pods|scheduler_pod_scheduling_attempts|scheduler_queue_incoming_pods_total|scheduler_preemption_attempts_total|scheduler_preemption_victims|scheduler_scheduling_attempt_duration_seconds|scheduler_schedule_attempts_total|scheduler_pod_scheduling_duration_seconds",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"kube-controller-manager": {
		JobName:                    "kube-controller-manager",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_kube_controller_manager.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "rest_client_request_duration_seconds|rest_client_requests_total|workqueue_depth",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"etcd": {
		JobName:                    "etcd",
		Enabled:                    true,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_etcd.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "etcd_server_has_leader|rest_client_requests_total|etcd_mvcc_db_total_size_in_bytes|etcd_mvcc_db_total_size_in_use_in_bytes|etcd_server_slow_read_indexes_total|etcd_server_slow_apply_total|etcd_network_client_grpc_sent_bytes_total|etcd_server_heartbeat_send_failures_total",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
	"node-auto-provisioning": {
		JobName:                    "node-auto-provisioning",
		Enabled:                    false,
		OSType:                     OSType.Linux,
		KubernetesPlane:            KubernetesPlane.ControlPlane,
		ControllerType:             ControllerType.ReplicaSet,
		ScrapeConfigDefinitionFile: "controlplane_node_auto_provisioning.yml",
		PlaceholderNames:           []string{"POD_NAMESPACE"},
		MinimalKeepListRegex:       "karpenter_(nodes_created_total|nodes_terminated_total|voluntary_disruption_eligible_nodes|nodeclaims_disrupted_total|voluntary_disruption_decisions_total|pods_state)",
		CustomerKeepListRegex:      "",
		ScrapeInterval:             "30s",
	},
}
