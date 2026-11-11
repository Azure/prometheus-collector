package querymetrics

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"prometheus-collector/otelcollector/test/utils"
)

var _ = Describe("Query Metrics Test Suite", func() {
	DescribeTable("should apply the minimal ingestion profile to node exporter metrics",
		func(expectNonMinimalMetric bool) {
			upQuery := utils.FreshMetricQuery(
				fmt.Sprintf("up{job=%q,cluster=%q}", "node", ExpectedCluster), suiteStarted)
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, upQuery)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, "up", "node", ExpectedCluster, time.Now(), true)).To(Succeed())
			}, 8*time.Minute, 15*time.Second).Should(Succeed())

			metric := "node_filefd_allocated"
			query := utils.FreshMetricQuery(
				fmt.Sprintf("%s{job=%q,cluster=%q}", metric, "node", ExpectedCluster), suiteStarted)
			check := func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				if expectNonMinimalMetric {
					g.Expect(samples).NotTo(BeEmpty(), "non-minimal metric %s was not ingested", metric)
					g.Expect(utils.ValidateMetricSamples(samples, metric, "node", ExpectedCluster, time.Now(), false)).To(Succeed())
				} else {
					g.Expect(samples).To(BeEmpty(), "non-minimal metric %s was ingested with MIP enabled", metric)
				}
			}
			if expectNonMinimalMetric {
				Eventually(check, 8*time.Minute, 15*time.Second).Should(Succeed())
			} else {
				Consistently(check, 90*time.Second, 15*time.Second).Should(Succeed())
			}
		},
		Entry("filters a non-minimal metric when enabled", false, Label("mip-enabled")),
		Entry("ingests the same metric when disabled", true, Label("mip-disabled")),
	)

	It("ingests the configured alias and only the admitted annotation namespace after this suite starts", Label("cluster-identity"), func() {
		Expect(os.Getenv("EXPECTED_CLUSTER")).To(Equal("Customer_Config_POC"), "use the customer-settings alias fixture and an independent expected identity")
		Eventually(func(g Gomega) {
			for _, target := range []struct{ metric, job string }{
				{"up", "kubelet"}, {"weather_temperature", "kubernetes-pods"},
			} {
				selector := fmt.Sprintf("%s{job=%q,cluster=%q}", target.metric, target.job, ExpectedCluster)
				if target.job == "kubernetes-pods" {
					selector = fmt.Sprintf("%s{job=%q,cluster=%q,kubernetes_namespace=%q,kubernetes_pod_name=%q}",
						target.metric, target.job, ExpectedCluster, "ama-settings-included", "annotation-target")
				}
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, utils.FreshMetricQuery(selector, suiteStarted))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, target.metric, target.job, ExpectedCluster, time.Now(), target.metric == "up")).To(Succeed())
			}
			excluded := fmt.Sprintf("weather_temperature{job=%q,cluster=%q,kubernetes_namespace=%q,kubernetes_pod_name=%q}",
				"kubernetes-pods", ExpectedCluster, "ama-settings-excluded", "annotation-target")
			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, utils.FreshMetricQuery(excluded, suiteStarted))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(warnings).To(BeEmpty())
			samples, ok := result.(model.Vector)
			g.Expect(ok).To(BeTrue())
			g.Expect(samples).To(BeEmpty(), "excluded namespace must not ingest after this slot's query suite starts")
		}, 8*time.Minute, 15*time.Second).Should(Succeed())
	})

	DescribeTable("should return the expected results for specified Prometheus metrics in each job",
		func(job string, expectedMetrics []string) {
			for _, metric := range expectedMetrics {
				query := metricQuery(metric, job)

				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				vectorResult, ok := result.(model.Vector)
				Expect(ok).To(BeTrue(), "result should be of type model.Vector for metric %s", metric)
				Expect(vectorResult).NotTo(BeEmpty(), "Metric %s is missing", metric)

				Expect(utils.ValidateMetricSamples(vectorResult, metric, job, ExpectedCluster, time.Now(), false)).To(Succeed())
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
			// "kubelet_volume_stats_used_bytes",
			// "kubelet_volume_stats_used_bytes",
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
			// "kubelet_volume_stats_capacity_bytes",
			// "kubelet_volume_stats_available_bytes",
			// "kubelet_volume_stats_inodes_used",
			// "kubelet_volume_stats_inodes",
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
			// "kube_statefulset_status_replicas_ready",
			// "kube_statefulset_status_replicas",
			// "kube_statefulset_status_replicas_updated",
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
			"kube_pod_status_ready",
			"kube_pod_status_scheduled",
			"kube_node_spec_unschedulable",
			// Only emitted when the cluster has statefulsets/PVs; uncomment if the e2e cluster gets them.
			// "kube_statefulset_metadata_generation",
			// "kube_statefulset_status_observed_generation",
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

	It("ingests fresh Windows exporter CPU, memory and container sentinels from every Windows collector node", Label(utils.WindowsLabel, "cluster-network"), func() {
		if os.Getenv("EXPECT_WINDOWS_EXPORTER") != "true" {
			Skip("Windows exporter is not enabled for this lane")
		}
		pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-win-node")
		Expect(err).NotTo(HaveOccurred())
		for _, metric := range []string{"windows_cpu_time_total", "windows_memory_available_bytes", "windows_container_cpu_usage_seconds_total"} {
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, metricQuery(metric, "windows-exporter"))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, metric, "windows-exporter", ExpectedCluster, time.Now(), false)).To(Succeed())
				for _, pod := range pods {
					found := false
					for _, sample := range samples {
						found = found || string(sample.Metric["instance"]) == pod.Spec.NodeName
					}
					g.Expect(found).To(BeTrue(), "node %s has no fresh %s sample", pod.Spec.NodeName, metric)
				}
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		}
	})

	It("ingests managed Retina node and Hubble pod metrics on non-Cilium ACNS", Label("managed-acns"), func() {
		nodePods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-node")
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePods).NotTo(BeEmpty(), "ACNS basic metrics require Linux collector nodes")

		started := time.Now()
		for range 3 {
			_, err := net.LookupHost("prometheus-reference-service.default.svc.cluster.local")
			Expect(err).NotTo(HaveOccurred())
		}
		for _, target := range []struct {
			job, metric string
		}{
			{"networkobservability-retina", "networkobservability_forward_bytes"},
			{"networkobservability-hubble", "hubble_dns_queries_total"},
		} {
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(metricQuery("up", target.job), started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, "up", target.job, ExpectedCluster, time.Now(), true)).To(Succeed())
				g.Expect(samples).NotTo(BeEmpty(), "managed %s scrape target is required", target.job)
				warnings, result, err = utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(metricQuery(target.metric, target.job), started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok = result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, target.metric, target.job, ExpectedCluster, time.Now(), false)).To(Succeed())
				g.Expect(samples).NotTo(BeEmpty(), "fresh managed %s metric is required", target.metric)
				if target.job == "networkobservability-retina" {
					for _, pod := range nodePods {
						g.Expect(pod.Spec.NodeName).NotTo(BeEmpty())
						found := false
						for _, sample := range samples {
							found = found || string(sample.Metric["instance"]) == pod.Spec.NodeName
						}
						g.Expect(found).To(BeTrue(), "node %s has no fresh managed Retina sample", pod.Spec.NodeName)
					}
				}
				trafficQuery := fmt.Sprintf("increase(%s{job=%q,cluster=%q}[5m])",
					target.metric, target.job, ExpectedCluster)
				warnings, result, err = utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(trafficQuery, started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				traffic, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(traffic).NotTo(BeEmpty(), "managed %s traffic increase is required", target.metric)
				observedTraffic := false
				podTraffic := false
				for _, sample := range traffic {
					g.Expect(string(sample.Metric["job"])).To(Equal(target.job))
					g.Expect(string(sample.Metric["cluster"])).To(Equal(ExpectedCluster))
					observedTraffic = observedTraffic || sample.Value > 0
					podTraffic = podTraffic || (sample.Value > 0 &&
						(sample.Metric["source"] != "" || sample.Metric["destination"] != ""))
				}
				g.Expect(observedTraffic).To(BeTrue(), "managed %s traffic must increase after test traffic", target.metric)
				if target.job == "networkobservability-hubble" && len(samples) > 0 {
					g.Expect(podTraffic).To(BeTrue(), "pod-level Hubble metrics must identify a source or destination pod")
				}
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		}
	})

	It("ingests node network counters after traffic through the selected AKS network profile", Label("network-profile"), func() {
		started := time.Now()
		baselines := map[string]map[string]model.SampleValue{}
		for _, metric := range []string{"node_network_receive_bytes_total", "node_network_transmit_bytes_total"} {
			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, metricQuery(metric, "node"))
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
			samples, ok := result.(model.Vector)
			Expect(ok).To(BeTrue())
			Expect(samples).NotTo(BeEmpty())
			baselines[metric] = map[string]model.SampleValue{}
			for _, sample := range samples {
				key := string(sample.Metric["instance"]) + "/" + string(sample.Metric["device"])
				baselines[metric][key] = sample.Value
			}
		}

		client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}
		for range 3 {
			_, err := net.LookupHost("prometheus-reference-service.default.svc.cluster.local")
			Expect(err).NotTo(HaveOccurred())
			response, err := client.Get("http://prometheus-reference-service.default.svc.cluster.local:2112/metrics")
			Expect(err).NotTo(HaveOccurred())
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			_, copyErr := io.Copy(io.Discard, response.Body)
			closeErr := response.Body.Close()
			Expect(copyErr).NotTo(HaveOccurred())
			Expect(closeErr).NotTo(HaveOccurred())
		}

		for _, metric := range []string{"node_network_receive_bytes_total", "node_network_transmit_bytes_total"} {
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(metricQuery(metric, "node"), started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, metric, "node", ExpectedCluster, time.Now(), false)).To(Succeed())
				increased := false
				for _, sample := range samples {
					key := string(sample.Metric["instance"]) + "/" + string(sample.Metric["device"])
					baseline, existed := baselines[metric][key]
					increased = increased || (existed && sample.Value > baseline)
				}
				g.Expect(increased).To(BeTrue(), "%s did not increase after DNS and service traffic", metric)
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		}
	})

	It("scales the Linux system pool for pending workload and returns to its minimum", Label("autoscaler"), func() {
		ctx := context.Background()
		const namespaceName = "candidate-autoscaler-probe"
		namespaceClient := K8sClient.CoreV1().Namespaces()
		namespace, err := namespaceClient.Get(ctx, namespaceName, metav1.GetOptions{})
		if err == nil {
			Expect(namespace.Labels["app"]).To(Equal(namespaceName))
			Expect(namespace.Annotations["candidate.cluster"]).To(Equal(ExpectedCluster))
			Expect(namespaceClient.Delete(ctx, namespaceName, metav1.DeleteOptions{})).To(Succeed())
			Eventually(func() bool {
				_, err := namespaceClient.Get(ctx, namespaceName, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, 5*time.Minute, 10*time.Second).Should(BeTrue())
		} else {
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "unexpected error checking owned autoscaler namespace: %v", err)
		}

		poolNodeCount := func() (int, error) {
			pods, err := K8sClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
				LabelSelector: "dsName=ama-metrics-node",
			})
			if err != nil {
				return 0, err
			}
			count := 0
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning && pod.Spec.NodeName != "" {
					count++
				}
			}
			return count, nil
		}
		eventuallyPoolCount := func(g Gomega) int {
			count, err := poolNodeCount()
			g.Expect(err).NotTo(HaveOccurred())
			return count
		}
		Eventually(eventuallyPoolCount, 20*time.Minute, 15*time.Second).Should(Equal(1))

		namespace, err = namespaceClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:        namespaceName,
			Labels:      map[string]string{"app": namespaceName},
			Annotations: map[string]string{"candidate.cluster": ExpectedCluster},
		}}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			err := namespaceClient.Delete(ctx, namespace.Name, metav1.DeleteOptions{})
			Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue(), "failed to remove owned autoscaler namespace: %v", err)
			Eventually(func(g Gomega) bool {
				_, err := namespaceClient.Get(ctx, namespace.Name, metav1.GetOptions{})
				g.Expect(err == nil || apierrors.IsNotFound(err)).To(BeTrue())
				return apierrors.IsNotFound(err)
			}, 5*time.Minute, 10*time.Second).Should(BeTrue())
			Eventually(eventuallyPoolCount, 20*time.Minute, 15*time.Second).Should(Equal(1))
		})

		for index := range 3 {
			_, err = K8sClient.CoreV1().Pods(namespaceName).Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("scale-probe-%d", index)},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux", "kubernetes.azure.com/agentpool": "system",
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name: "pause", Image: "mcr.microsoft.com/oss/kubernetes/pause:3.9",
						Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2"),
						}},
					}},
				},
			}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
		Eventually(func(g Gomega) {
			for index := range 3 {
				pod, err := K8sClient.CoreV1().Pods(namespaceName).Get(
					ctx, fmt.Sprintf("scale-probe-%d", index), metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
			}
		}, 20*time.Minute, 15*time.Second).Should(Succeed())
		count, err := poolNodeCount()
		Expect(err).NotTo(HaveOccurred())
		Expect(count).To(BeNumerically(">", 1))
	})

	It("ingests only the configured KSM pod-label and namespace-annotation allowlists", Label("ksm-allowlist"), func() {
		ctx := context.Background()
		namespaceName := fmt.Sprintf("candidate-ksm-%d", time.Now().UnixNano())
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
			Annotations: map[string]string{
				"settings-test": "included",
				"not-allowed":   "excluded",
			},
		}}
		_, err := K8sClient.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(K8sClient.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})).To(Succeed())
		})
		podName := "allowlist-probe"
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: podName, Namespace: namespaceName,
			Labels: map[string]string{"app": "included", "not-allowed": "excluded"},
		}, Spec: corev1.PodSpec{
			NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
			Containers: []corev1.Container{{
				Name: "pause", Image: "mcr.microsoft.com/oss/kubernetes/pause:3.9",
			}},
		}}
		_, err = K8sClient.CoreV1().Pods(namespaceName).Create(ctx, pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() {
			Expect(K8sClient.CoreV1().Pods(namespaceName).Delete(ctx, podName, metav1.DeleteOptions{})).To(Succeed())
		})

		started := time.Now()
		Eventually(func(g Gomega) {
			for _, check := range []struct {
				metric, query string
				found         bool
			}{
				{"kube_pod_labels", fmt.Sprintf(`kube_pod_labels{job="kube-state-metrics",namespace=%q,pod=%q,label_app="included"}`, namespaceName, podName), true},
				{"kube_pod_labels", fmt.Sprintf(`kube_pod_labels{job="kube-state-metrics",namespace=%q,pod=%q,label_not_allowed="excluded"}`, namespaceName, podName), false},
				{"kube_namespace_annotations", fmt.Sprintf(`kube_namespace_annotations{job="kube-state-metrics",namespace=%q,annotation_settings_test="included"}`, namespaceName), true},
				{"kube_namespace_annotations", fmt.Sprintf(`kube_namespace_annotations{job="kube-state-metrics",namespace=%q,annotation_not_allowed="excluded"}`, namespaceName), false},
			} {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(check.query, started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				if check.found {
					g.Expect(samples).NotTo(BeEmpty(), "allowlisted KSM metric is missing: %s", check.query)
					g.Expect(utils.ValidateMetricSamples(
						samples, check.metric, "kube-state-metrics", ExpectedCluster, time.Now(), false,
					)).To(Succeed())
				} else {
					g.Expect(samples).To(BeEmpty(), "non-allowlisted KSM metric was ingested: %s", check.query)
				}
			}
		}, 8*time.Minute, 15*time.Second).Should(Succeed())
	})

	DescribeTable("should return the expected labels for specified metrics in each job",
		func(job string, metric string, labels map[string]string) {
			query := metricQuery(metric, job)

			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())

			vectorResult, ok := result.(model.Vector)
			Expect(ok).To(BeTrue(), "result should be of type model.Vector for metric %s", metric)
			Expect(vectorResult).NotTo(BeEmpty(), "Metric %s is missing", metric)
			Expect(utils.ValidateMetricSamples(vectorResult, metric, job, ExpectedCluster, time.Now(), false)).To(Succeed())

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
		}, Label(utils.LinuxDaemonsetCustomConfig), Label(utils.ConfigProcessingSettingsNodeConfigMap)),
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
		}, Label(utils.LinuxDaemonsetCustomConfig), Label(utils.ConfigProcessingSettingsNodeConfigMap)),
		Entry("External labels are applied from Windows DaemonSet Configmap", "windows-node-configmap", "up", map[string]string{
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}, Label(utils.WindowsLabel), Label(utils.ConfigProcessingSettingsNodeConfigMap)),
	)

	Context("When querying metrics", func() {
		DescribeTable("has fresh healthy kubelet data from every collector node", Label("cluster-network"),
			func(controller string) {
				pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", controller)
				Expect(err).NotTo(HaveOccurred())
				Eventually(func(g Gomega) {
					warnings, result, err := utils.InstantQuery(PrometheusQueryClient, metricQuery("up", "kubelet"))
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(warnings).To(BeEmpty())
					samples, ok := result.(model.Vector)
					g.Expect(ok).To(BeTrue())
					g.Expect(utils.ValidateMetricSamples(samples, "up", "kubelet", ExpectedCluster, time.Now(), true)).To(Succeed())
					for _, pod := range pods {
						g.Expect(pod.Spec.NodeName).NotTo(BeEmpty())
						found := false
						for _, sample := range samples {
							found = found || string(sample.Metric["instance"]) == pod.Spec.NodeName
						}
						g.Expect(found).To(BeTrue(), "node %s has no fresh kubelet sample", pod.Spec.NodeName)
					}
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
			},
			Entry("Linux", "ama-metrics-node"),
			Entry("Windows", "ama-metrics-win-node", Label(utils.WindowsLabel)),
		)

		DescribeTable("should return the expected results for up=1 for all jobs",
			func(jobs []string) {
				for _, job := range jobs {
					// Run the query for the job
					warnings, result, err := utils.InstantQuery(PrometheusQueryClient, metricQuery("up", job))
					Expect(err).NotTo(HaveOccurred(), "failed to execute query for job %s", job)

					// Ensure there are no warnings
					Expect(warnings).To(BeEmpty(), "warnings should be empty for job %s", job)

					// Ensure there is at least one result
					vectorResult, ok := result.(model.Vector)
					Expect(ok).To(BeTrue(), "result should be of type model.Vector for job %s", job)
					Expect(vectorResult).NotTo(BeEmpty(), "result should not be empty for job %s", job)

					// Ensure that all results have the 'up' metric with a value of 1
					Expect(utils.ValidateMetricSamples(vectorResult, "up", job, ExpectedCluster, time.Now(), true)).To(Succeed())
				}
			},
			Entry("AKS jobs", []string{"kubelet", "cadvisor", "kube-state-metrics", "node", "networkobservability-retina"}, Label(utils.RetinaLabel)),
			Entry("Arc jobs", []string{"kubelet", "cadvisor", "kube-state-metrics", "node"}, Label(utils.ArcExtensionLabel)),
		)

		It("should return the expected results for OTLP data-quality validation metrics", Label(utils.OTLPLabel), func() {
			metrics := []string{
				"otlpapp.intcounter.total",
				"otlpapp.floatcounter.total",
				"otlpapp.intgauge",
				"otlpapp.floatgauge",
				"otlpapp.intupdowncounter",
				"otlpapp.floatupdowncounter",
				"otlpapp.intexponentialhistogram",
				"otlpapp.floatexponentialhistogram",
				"otlpapp.intexplicithistogram",
				"otlpapp.floatexplicithistogram",
			}

			for _, metric := range metrics {
				query := utils.FreshMetricQuery(fmt.Sprintf("{%q,cluster=%q}", metric, ExpectedCluster), time.Time{})

				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, query)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				vectorResult, ok := result.(model.Vector)
				Expect(ok).To(BeTrue(), "result should be of type model.Vector for metric %s", metric)
				Expect(vectorResult).NotTo(BeEmpty(), "Metric %s is missing", metric)

				labelName := "temporality"
				deltaLabelFound := false
				cumulativeLabelFound := false
				for _, sample := range vectorResult {
					val, ok := sample.Metric[model.LabelName(labelName)]
					if !ok {
						Expect(ok).To(BeTrue(), fmt.Sprintf("Expected label %q not found in metric %q", labelName, metric))
					}
					if string(val) == "delta" {
						deltaLabelFound = true
					} else if string(val) == "cumulative" {
						cumulativeLabelFound = true
					}

					Expect(string(sample.Metric["cluster"])).To(Equal(ExpectedCluster))
					Expect(time.Since(sample.Timestamp.Time())).To(BeNumerically("<", 5*time.Minute))
				}

				Expect(deltaLabelFound).To(BeTrue(), fmt.Sprintf("Expected metric %q to have a sample with delta temporality label", metric))
				Expect(cumulativeLabelFound).To(BeTrue(), fmt.Sprintf("Expected metric %q to have a sample with cumulative temporality label", metric))
			}
		})

	})
})
