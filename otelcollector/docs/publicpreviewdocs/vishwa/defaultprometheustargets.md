## Default targets, dashboards & recording rules

By default, Azure monitor Managed Prometheus agent sets up & scrapes few targets and few metrics in them, out-of-the box without requiring any customer action. 

# Defaults

* scrape frequency for all default targets/scrapes = 30s

# Default targets scraped 
1. cadvisor (job=cadvisor)
2. nodeexporter (job=node)
3. kubelet (job=kubelet)
4. kube-state-metrics (job=kube-state-metrics)
   
# Default metrics collected from default targets

Below metrics are collected by default from each default target (All other metrics are dropped through relabeling rules). Please see []() on how to add more metrics to `keeplist` per target.

   cadvisor (job=cadvisor)
   * container_memory_rss
   * container_network_receive_bytes_total
   * container_network_transmit_bytes_total
   * container_network_receive_packets_total
   * container_network_transmit_packets_total
   * container_network_receive_packets_dropped_total
   * container_network_transmit_packets_dropped_total
   * container_fs_reads_total
   * container_fs_writes_total
   * container_fs_reads_bytes_total
   * container_fs_writes_bytes_total|container_cpu_usage_seconds_total
  
   kubelet (job=kubelet)
   * kubelet_node_name
   * kubelet_running_pods
   * kubelet_running_pod_count
   * kubelet_running_sum_containers
   * kubelet_running_container_count
   * volume_manager_total_volumes
   * kubelet_node_config_error
   * kubelet_runtime_operations_total
   * kubelet_runtime_operations_errors_total
   * kubelet_runtime_operations_duration_seconds_bucket
   * kubelet_runtime_operations_duration_seconds_sum
   * kubelet_runtime_operations_duration_seconds_count
   * kubelet_pod_start_duration_seconds_bucket
   * kubelet_pod_start_duration_seconds_sum
   * kubelet_pod_start_duration_seconds_count
   * kubelet_pod_worker_duration_seconds_bucket
   * kubelet_pod_worker_duration_seconds_sum
   * kubelet_pod_worker_duration_seconds_count
   * storage_operation_duration_seconds_bucket
   * storage_operation_duration_seconds_sum
   * storage_operation_duration_seconds_count
   * storage_operation_errors_total
   * kubelet_cgroup_manager_duration_seconds_bucket
   * kubelet_cgroup_manager_duration_seconds_sum
   * kubelet_cgroup_manager_duration_seconds_count
   * kubelet_pleg_relist_interval_seconds_bucket
   * kubelet_pleg_relist_interval_seconds_count
   * kubelet_pleg_relist_interval_seconds_sum
   * kubelet_pleg_relist_duration_seconds_bucket
   * kubelet_pleg_relist_duration_seconds_count
   * kubelet_pleg_relist_duration_seconds_sum
   * rest_client_requests_total
   * rest_client_request_duration_seconds_bucket
   * rest_client_request_duration_seconds_sum
   * rest_client_request_duration_seconds_count
   * process_resident_memory_bytes
   * process_cpu_seconds_total
   * go_goroutines
   * kubernetes_build_info
  
   nodexporter (job=node)
   * node_memory_MemTotal_bytes
   * node_cpu_seconds_total
   * node_memory_MemAvailable_bytes
   * node_memory_Buffers_bytes
   * node_memory_Cached_bytes
   * node_memory_MemFree_bytes
   * node_memory_Slab_bytes
   * node_filesystem_avail_bytes
   * node_filesystem_size_bytes
   * node_time_seconds
   * node_exporter_build_info
   * node_load1
   * node_vmstat_pgmajfault
   * node_network_receive_bytes_total
   * node_network_transmit_bytes_total
   * node_network_receive_drop_total
   * node_network_transmit_drop_total
   * node_disk_io_time_seconds_total
   * node_disk_io_time_weighted_seconds_total
   * node_load5
   * node_load15
   * node_disk_read_bytes_total
   * node_disk_written_bytes_total
   * node_uname_info
  
   kube-state-metrics (job=kube-state-metrics)
   * kube_node_status_allocatable
   * kube_pod_owner
   * kube_pod_container_resource_requests
   * kube_pod_status_phase
   * kube_pod_container_resource_limits
   * kube_pod_info|kube_replicaset_owner
   * kube_resourcequota
   * kube_namespace_status_phase
   * kube_node_status_capacity
   * kube_node_info
   * kube_pod_info
   * kube_deployment_spec_replicas
   * kube_deployment_status_replicas_available
   * kube_deployment_status_replicas_updated
   * kube_statefulset_status_replicas_ready
   * kube_statefulset_status_replicas
   * kube_statefulset_status_replicas_updated
   * kube_job_status_start_time
   * kube_job_status_active
   * kube_job_failed
   * kube_horizontalpodautoscaler_status_desired_replicas
   * kube_horizontalpodautoscaler_status_current_replicas
   * kube_horizontalpodautoscaler_spec_min_replicas
   * kube_horizontalpodautoscaler_spec_max_replicas
   * kubernetes_build_info
   * kube_node_status_condition
   * kube_node_spec_taint

# Default dashboards

Below are the default dashboards that are auto-configured by Azure monitor Managed Prometheus during the time of monitoring enablement (thru Ux and CLI) on the chosen Azure Managed Grafana instance. Source code for these mixin dashboards can be found [here](https://github.com/Azure/prometheus-collector/tree/main/mixins)

1. Kubernetes / Compute Resources / Cluster
2. Kubernetes / Compute Resources / Namespace (Pods)
3. Kubernetes / Compute Resources / Node (Pods)
4. Kubernetes / Compute Resources / Pod
5. Kubernetes / Compute Resources / Namespace (Workloads)
6. Kubernetes / Compute Resources / Workload
7. Kubernetes / Kubelet
8. Node Exporter / USE Method / Node
9. Node Exporter / Nodes

# Default recording rules

Below are the default recording rules that are auto-configured by Azure monitor Managed Prometheus during the time of monitoring enablement (thru Ux and CLI) on the chosen Azure Monitor Workspace. Source code for these mixin recording rules can be found [here](https://github.com/Azure/prometheus-collector/tree/main/mixins)


1.	cluster:node_cpu:ratio_rate5m
2.	namespace_cpu:kube_pod_container_resource_requests:sum
3.	namespace_cpu:kube_pod_container_resource_limits:sum
4.	:node_memory_MemAvailable_bytes:sum
5.	namespace_memory:kube_pod_container_resource_requests:sum
6.	namespace_memory:kube_pod_container_resource_limits:sum
7.	namespace_workload_pod:kube_pod_owner:relabel
8.	node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate
9.  cluster:namespace:pod_cpu:active:kube_pod_container_resource_requests
10. cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits
11. cluster:namespace:pod_memory:active:kube_pod_container_resource_requests
12. cluster:namespace:pod_memory:active:kube_pod_container_resource_limits
13. node_namespace_pod_container:container_memory_working_set_bytes
14. node_namespace_pod_container:container_memory_rss
15. node_namespace_pod_container:container_memory_cache
16. node_namespace_pod_container:container_memory_swap
17. instance:node_cpu_utilisation:rate5m
18. instance:node_load1_per_cpu:ratio
19. instance:node_memory_utilisation:ratio
20. instance:node_vmstat_pgmajfault:rate5m
21. instance:node_network_receive_bytes_excluding_lo:rate5m
22. instance:node_network_transmit_bytes_excluding_lo:rate5m
23. instance:node_network_receive_drop_excluding_lo:rate5m
24. instance:node_network_transmit_drop_excluding_lo:rate5m
25. instance_device:node_disk_io_time_seconds:rate5m
26. instance_device:node_disk_io_time_weighted_seconds:rate5m
27. instance:node_num_cpu:sum
