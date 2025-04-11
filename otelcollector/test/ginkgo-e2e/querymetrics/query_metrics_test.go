package querymetrics

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	// "github.com/prometheus/common/model"

	"prometheus-collector/otelcollector/test/utils"
)

var _ = Describe("Query Metrics Test Suite", func() {

	DescribeTable("should return the expected results for specified metrics in each job",
		func(job string, expectedMetrics []string) {
			for _, metric := range expectedMetrics {
				query := fmt.Sprintf("%s{job=\"%s\"}", metric, job)

				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				vectorResult, ok := result.(model.Vector)
				Expect(ok).To(BeTrue(), "result should be of type model.Vector for metric %s", metric)
				Expect(vectorResult).NotTo(BeEmpty(), "Metric %s is missing", metric)

				found := false
				noClusterLabel := false
				noJobLabel := false
				noInstanceLabel := false
				for _, sample := range vectorResult {
					if string(sample.Metric["__name__"]) == metric {
						found = true
						break
					}
					if val, ok := sample.Metric["cluster"]; !ok || val == "" {
						noClusterLabel = true
						break
					}
					if val, ok := sample.Metric["job"]; !ok || val == "" {
						noJobLabel = true
						break
					}
					if val, ok := sample.Metric["instance"]; !ok || val == "" {
						noInstanceLabel = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("Expected metric %q not found", metric))
				Expect(noClusterLabel).To(BeFalse(), fmt.Sprintf("Expected metric %q does not have cluster label", metric))
				Expect(noJobLabel).To(BeFalse(), fmt.Sprintf("Expected metric %q does not have job label", metric))
				Expect(noInstanceLabel).To(BeFalse(), fmt.Sprintf("Expected metric %q does not have instance label", metric))
			}
		},
		Entry("default job 'cadvisor'", "cadvisor", []string{
			"container_spec_cpu_period",
			"container_spec_cpu_quota",
			"container_cpu_usage_seconds_total",
			"container_memory_rss",
			"container_network_receive_bytes_total",
			"container_network_transmit_bytes_total",
			"container_network_receive_packets_total",
			"container_network_transmit_packets_total",
			"container_network_receive_packets_dropped_total",
			"container_network_transmit_packets_dropped_total",
			"container_fs_reads_total",
			"container_fs_writes_total",
			"container_fs_reads_bytes_total",
			"container_fs_writes_bytes_total",
			"container_memory_working_set_bytes",
			"container_memory_cache",
			"container_memory_swap",
			"container_cpu_cfs_throttled_periods_total",
			"container_cpu_cfs_periods_total",
			// "container_memory_usage_bytes",
			// "kubernetes_build_info",
		}),
		Entry("default job 'kubelet'", "kubelet", []string{
			"kubelet_volume_stats_used_bytes",
			"kubelet_volume_stats_used_bytes",
			"kubelet_node_name",
			"kubelet_running_pods",
			// "kubelet_running_pod_count",
			"kubelet_running_containers",
			// "kubelet_running_container_count",
			"volume_manager_total_volumes",
			// "kubelet_node_config_error",
			"kubelet_runtime_operations_total",
			"kubelet_runtime_operations_errors_total",
			// "kubelet_runtime_operations_duration_seconds",
			"kubelet_runtime_operations_duration_seconds_bucket",
			"kubelet_runtime_operations_duration_seconds_sum",
			"kubelet_runtime_operations_duration_seconds_count",
			// "kubelet_pod_start_duration_seconds",
			"kubelet_pod_start_duration_seconds_bucket",
			"kubelet_pod_start_duration_seconds_sum",
			"kubelet_pod_start_duration_seconds_count",
			// "kubelet_pod_worker_duration_seconds",
			"kubelet_pod_worker_duration_seconds_bucket",
			"kubelet_pod_worker_duration_seconds_sum",
			"kubelet_pod_worker_duration_seconds_count",
			// "storage_operation_duration_seconds",
			"storage_operation_duration_seconds_bucket",
			"storage_operation_duration_seconds_sum",
			"storage_operation_duration_seconds_count",
			// "storage_operation_errors_total",
			// "kubelet_cgroup_manager_duration_seconds",
			"kubelet_cgroup_manager_duration_seconds_bucket",
			"kubelet_cgroup_manager_duration_seconds_sum",
			"kubelet_cgroup_manager_duration_seconds_count",
			// "kubelet_pleg_relist_duration_seconds",
			"kubelet_pleg_relist_duration_seconds_bucket",
			// "kubelet_pleg_relist_duration_sum",
			"kubelet_pleg_relist_duration_seconds_count",
			// "kubelet_pleg_relist_interval_seconds",
			"kubelet_pleg_relist_interval_seconds_bucket",
			"kubelet_pleg_relist_interval_seconds_sum",
			"kubelet_pleg_relist_interval_seconds_count",
			"rest_client_requests_total",
			// "rest_client_request_duration_seconds",
			"rest_client_request_duration_seconds_bucket",
			"rest_client_request_duration_seconds_sum",
			"rest_client_request_duration_seconds_count",
			"process_resident_memory_bytes",
			"process_cpu_seconds_total",
			"go_goroutines",
			"kubelet_volume_stats_capacity_bytes",
			"kubelet_volume_stats_available_bytes",
			"kubelet_volume_stats_inodes_used",
			"kubelet_volume_stats_inodes",
			"kubernetes_build_info",
		}),
		Entry("job 'nodeexporter' (job=node)", "node", []string{
			"node_cpu_seconds_total",
			"node_memory_MemAvailable_bytes",
			"node_memory_Buffers_bytes",
			"node_memory_Cached_bytes",
			"node_memory_MemFree_bytes",
			"node_memory_Slab_bytes",
			"node_memory_MemTotal_bytes",
			// "node_netstat_Tcp_RetransSegs",
			// "node_netstat_Tcp_OutSegs",
			// "node_netstat_TcpExt_TCPSynRetrans",
			"node_load1",
			"node_load5",
			"node_load15",
			"node_disk_read_bytes_total",
			"node_disk_written_bytes_total",
			"node_disk_io_time_seconds_total",
			"node_filesystem_size_bytes",
			"node_filesystem_avail_bytes",
			"node_filesystem_readonly",
			"node_network_receive_bytes_total",
			"node_network_transmit_bytes_total",
			"node_vmstat_pgmajfault",
			"node_network_receive_drop_total",
			"node_network_transmit_drop_total",
			"node_disk_io_time_weighted_seconds_total",
			"node_exporter_build_info",
			"node_time_seconds",
			"node_uname_info",
		}),
		Entry("job 'kube-state-metrics' (job=kube-state-metrics)", "kube-state-metrics", []string{
			// "kube_job_status_succeeded",
			// "kube_job_spec_completions",
			"kube_daemonset_status_desired_number_scheduled",
			"kube_daemonset_status_number_ready",
			"kube_deployment_status_replicas_ready",
			"kube_pod_container_status_last_terminated_reason",
			// "kube_pod_container_status_waiting_reason",
			"kube_pod_container_status_restarts_total",
			"kube_node_status_allocatable",
			"kube_pod_owner",
			"kube_pod_container_resource_requests",
			"kube_pod_status_phase",
			"kube_pod_container_resource_limits",
			"kube_replicaset_owner",
			// "kube_resourcequota",
			"kube_namespace_status_phase",
			"kube_node_status_capacity",
			"kube_node_info",
			"kube_pod_info",
			"kube_deployment_spec_replicas",
			"kube_deployment_status_replicas_available",
			"kube_deployment_status_replicas_updated",
			"kube_statefulset_status_replicas_ready",
			"kube_statefulset_status_replicas",
			"kube_statefulset_status_replicas_updated",
			// "kube_job_status_start_time",
			// "kube_job_status_active",
			// "kube_job_failed",
			// "kube_horizontalpodautoscaler_status_desired_replicas",
			// "kube_horizontalpodautoscaler_status_current_replicas",
			// "kube_horizontalpodautoscaler_spec_min_replicas",
			// "kube_horizontalpodautoscaler_spec_max_replicas",
			// "kubernetes_build_info",
			"kube_node_status_condition",
			// "kube_node_spec_taint",
			"kube_pod_container_info",
			// "kube_persistentvolumeclaim_access_mode",
			// "kube_persistentvolumeclaim_labels",
			// "kube_persistentvolume_status_phase",
		}),
		Entry("prometheus-reference-app", "prometheus_ref_app", []string{
			"myapp_measurements_total",
			"myapp_temperature",
			"myapp_rainfall",
			"empty_dimension_rainfall",
			"max_dimension_rainfall",
			"upperGaugeFqyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYephoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWymZEGRx_UbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftgzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer",
			"myapp_temperature_summary",
			"myapp_temperature_summary_count",
			"myapp_temperature_summary_sum",
			"myapp_rainfall_summary",
			"myapp_rainfall_summary_count",
			"myapp_rainfall_summary_sum",
			"upperSummaryyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNrEVHc_WymZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwffthzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer",
			"upperSummaryyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNrEVHc_WymZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwffthzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer_count",
			"upperSummaryyOtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNrEVHc_WymZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywsXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwffthzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer_sum",
			"max_dimension_rainfall_summary",
			"max_dimension_rainfall_summary_count",
			"max_dimension_rainfall_summary_sum",
			"empty_dimension_summary",
			"empty_dimension_summary_count",
			"empty_dimension_summary_sum",
			"myapp_temperature_histogram_bucket",
			"myapp_temperature_histogram_count",
			"myapp_temperature_histogram_sum",
			"myapp_rainfall_histogram_bucket",
			"myapp_rainfall_histogram_count",
			"myapp_rainfall_histogram_sum",
			"upperHistogramtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWy_ZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywtrXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftkzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer_bucket",
			"upperHistogramtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWy_ZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywtrXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftkzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer_count",
			"upperHistogramtBTnstaUDVyHTkqkQOTOSbCMUzpBtykcaoOYgphoAVbYzWvBMWHGnCEApFYGwUzayYWTegbAQomgbabGBpgzXZNtEVHczWy_ZEGRxFUbzNVZvvhQutrDYcNDKwRErwUxKuJYxGCEywtrXAvJGCufsEGzDUCmBPfPpcboHdHNjvmdEdtvVZzMTPyfCFwfftkzHSzoBkQSJJZxPUkyzpknfbfwbdUnZftFYqyBzmrbdQfmnMOBcer_sum",
			"max_dimension_rainfall_histogram_bucket",
			"max_dimension_rainfall_histogram_count",
			"max_dimension_rainfall_histogram_sum",
			"empty_dimension_histogram_bucket",
			"empty_dimension_histogram_count",
			"empty_dimension_histogram_sum",
			"untyped_metric",
			"request_processing_seconds_count",
			"request_processing_seconds_sum",
		}),
	)

	DescribeTable("should return the expected labels for specified metrics in each job",
		func(job string, metric string, labels map[string]string) {
			query := fmt.Sprintf("%s{job=\"%s\"}", metric, job)

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())

			vectorResult, ok := result.(model.Vector)
			Expect(ok).To(BeTrue(), "result should be of type model.Vector for metric %s", metric)
			Expect(vectorResult).NotTo(BeEmpty(), "Metric %s is missing", metric)

			for _, sample := range vectorResult {
				for label, expectedValue := range labels {
					val, ok := sample.Metric[model.LabelName(label)]
					Expect(ok).To(BeTrue(), fmt.Sprintf("Expected label %q not found in metric %q for the job %s", label, metric, job))
					Expect(string(val)).To(MatchRegexp(expectedValue), fmt.Sprintf("Label %q in metric %q for job %s has unexpected value: %s", label, metric, job, val))
				}
			}
		},
		Entry("Relabeling with dollar signs", "prometheus_ref_app", "up", map[string]string{
			"double_dollar_sign": "prometheus-reference-app", // Legacy backwards compatibility for $$1 when single $ was not supported
			"single_dollar_sign": "prometheus-reference-app",
		}),
		Entry("Relabeling with $NODE_NAME and $NODE_IP", "node-configmap", "up", map[string]string{
			"node_name_single_dollar_sign": ".+", // Node Name and IP env var substitution is only for daemonset
			"node_ip_single_dollar_sign":   "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}",
			"node_name_double_dollar_sign": ".+",                                        // Legacy backwards compatibility for $$NODE_NAME when single $ was not supported
			"node_ip_double_dollar_sign":   "\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}", // Legacy backwards compatibility for $$NODE_IP when single $ was not supported
		}),
		Entry("Relabeling with dollar signs & external labels for PodMonitor", "default/referenceapp", "up", map[string]string{
			"double_dollar_sign": "\\$1",                     // PodMonitor does not have the legacy backwards compatibility for $$1
			"single_dollar_sign": "prometheus-reference-app", // $1 does work for PodMonitor
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}),
		Entry("Relabeling with dollar signs & external labels for ServiceMonitor", "prometheus-reference-service", "up", map[string]string{
			"double_dollar_sign": "\\$1",                     // ServiceMonitor does not have the legacy backwards compatibility for $$1
			"single_dollar_sign": "prometheus-reference-app", // $1 does work for ServiceMonitor
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}),
		Entry("External labels are applied from ReplicaSet Configmap", "prometheus_ref_app", "up", map[string]string{
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}),
		Entry("External labels are applied from DaemonSet Configmap", "node-configmap", "up", map[string]string{
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}),
		Entry("External labels are applied from Windows DaemonSet Configmap", "windows-node-configmap", "up", map[string]string{
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}),
	)

	Context("When querying metrics", func() {
		It("should return the expected results for up=1 for all default jobs", func() {
			// Define a list of default jobs
			defaultJobs := []string{"kubelet", "cadvisor", "kube-state-metrics", "node", "networkobservability-retina"}

			for _, job := range defaultJobs {
				// Run the query for each job
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, fmt.Sprintf("up{job=\"%s\"} == 1", job))
				Expect(err).NotTo(HaveOccurred())

				// Ensure there are no warnings
				Expect(warnings).To(BeEmpty())

				// Ensure there is at least one result
				vectorResult, ok := result.(model.Vector)
				Expect(ok).To(BeTrue(), "result should be of type model.Vector")
				Expect(vectorResult).NotTo(BeEmpty(), "result should not be empty")

				// Ensure that all results have the 'up' metric with a value of 1
				for _, sample := range vectorResult {
					Expect(string(sample.Metric["__name__"])).To(Equal("up"))
					Expect(sample.Value.String()).To(Equal("1"))
				}
			}
		})
	})
})
