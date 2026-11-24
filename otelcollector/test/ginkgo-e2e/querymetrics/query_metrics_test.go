package querymetrics

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	// "github.com/prometheus/common/model"

	"prometheus-collector/otelcollector/test/utils"
)

func acnsCounterSnapshot(samples model.Vector, source string) map[model.Fingerprint]model.SampleValue {
	snapshot := make(map[model.Fingerprint]model.SampleValue, len(samples))
	for _, sample := range samples {
		if source == "" || string(sample.Metric["source"]) == source {
			snapshot[sample.Metric.Fingerprint()] = sample.Value
		}
	}
	return snapshot
}

func TestACNSCounterSnapshot(t *testing.T) {
	probe := &model.Sample{Metric: model.Metric{"source": "testkube/probe", "qtypes": "a"}, Value: 4}
	other := &model.Sample{Metric: model.Metric{"source": "testkube/other", "qtypes": "a"}, Value: 3}
	samples := model.Vector{probe, other}
	baseline := acnsCounterSnapshot(samples, "testkube/probe")
	other.Value = 10
	after := acnsCounterSnapshot(samples, "testkube/probe")
	if len(baseline) != 1 || len(after) != 1 || after[probe.Metric.Fingerprint()] != baseline[probe.Metric.Fingerprint()] {
		t.Fatal("unrelated pod traffic changed the probe snapshot")
	}
	probe.Value++
	after = acnsCounterSnapshot(samples, "testkube/probe")
	if after[probe.Metric.Fingerprint()] <= baseline[probe.Metric.Fingerprint()] {
		t.Fatal("probe traffic did not increase its counter")
	}
	if len(acnsCounterSnapshot(samples, "")) != 2 {
		t.Fatal("node counters must remain unfiltered")
	}
}

var _ = Describe("Scenario cluster metrics", func() {
	var expectedCluster string
	BeforeEach(func() {
		expectedCluster = os.Getenv("EXPECTED_CLUSTER")
		if expectedCluster == "" {
			Skip("Requires a scenario")
		}
	})
	It("ingests the configured alias and only its admitted annotation namespace", Label("cluster-identity"), func() {
		Expect(expectedCluster).To(Equal("Customer_Config_POC"))
		started := time.Now()
		pods := fmt.Sprintf(`weather_temperature{job="kubernetes-pods",cluster=%q,kubernetes_pod_name="annotation-target"`, expectedCluster)
		included := pods + `,kubernetes_namespace="ama-settings-included"}`
		excluded := pods + `,kubernetes_namespace="ama-settings-excluded"}`
		Eventually(func(g Gomega) {
			g.Expect(utils.ValidateMetricSamples(utils.FreshSamples(g, PrometheusQueryClient, utils.MetricQuery("up", "kubelet", expectedCluster), started),
				"up", "kubelet", expectedCluster, time.Now(), true)).To(Succeed())
			g.Expect(utils.ValidateMetricSamples(utils.FreshSamples(g, PrometheusQueryClient, included, started),
				"weather_temperature", "kubernetes-pods", expectedCluster, time.Now(), false)).To(Succeed())
		}, 8*time.Minute, 15*time.Second).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(utils.FreshSamples(g, PrometheusQueryClient, excluded, started)).To(BeEmpty())
		}, 90*time.Second, 15*time.Second).Should(Succeed())
	})

	It("ingests only the configured KSM pod labels and namespace annotations", Label("ksm-allowlist"), func() {
		const job = "kube-state-metrics"
		started := time.Now()
		checks := []struct{ metric, match, allowed, denied string }{
			{"kube_pod_labels", `,pod="allowlist-probe"`, "label_app", "label_not_allowed"},
			{"kube_namespace_annotations", "", "annotation_settings_test", "annotation_not_allowed"},
		}
		Eventually(func(g Gomega) {
			for _, check := range checks {
				selector := fmt.Sprintf(`%s{job=%q,cluster=%q,namespace="candidate-ksm-allowlist"%s}`,
					check.metric, job, expectedCluster, check.match)
				samples := utils.FreshSamples(g, PrometheusQueryClient, selector, started)
				g.Expect(utils.ValidateMetricSamples(samples, check.metric, job, expectedCluster, time.Now(), false)).To(Succeed())
				for _, sample := range samples {
					g.Expect(sample.Metric[model.LabelName(check.allowed)]).To(Equal(model.LabelValue("included")))
					g.Expect(sample.Metric).NotTo(HaveKey(model.LabelName(check.denied)))
				}
			}
		}, 8*time.Minute, 15*time.Second).Should(Succeed())
	})

	DescribeTable("ingests fresh node ConfigMap external labels", Label(utils.ConfigProcessingSettingsNodeConfigMap),
		func(job string) {
			started := time.Now()
			Eventually(func(g Gomega) {
				samples := utils.FreshSamples(g, PrometheusQueryClient, utils.MetricQuery("up", job, expectedCluster), started)
				g.Expect(utils.ValidateMetricSamples(samples, "up", job, expectedCluster, time.Now(), true)).To(Succeed())
				for _, sample := range samples {
					g.Expect(sample.Metric["external_label_1"]).To(Equal(model.LabelValue("external_label_value")))
					g.Expect(sample.Metric["external_label_123"]).To(Equal(model.LabelValue("external_label_value")))
				}
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		},
		Entry("Linux", "node-configmap"),
		Entry("Windows", "windows-node-configmap", Label(utils.WindowsLabel)),
	)

	It("filters a non-minimal node exporter metric with the minimal ingestion profile", Label("mip-enabled"), func() {
		started := time.Now()
		upQuery := utils.FreshMetricQuery(utils.MetricQuery("up", "node", expectedCluster), started)
		Eventually(func(g Gomega) {
			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, upQuery)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(warnings).To(BeEmpty())
			samples, ok := result.(model.Vector)
			g.Expect(ok).To(BeTrue())
			g.Expect(utils.ValidateMetricSamples(samples, "up", "node", expectedCluster, time.Now(), true)).To(Succeed())
		}, 8*time.Minute, 15*time.Second).Should(Succeed())

		metric := "node_filefd_allocated"
		query := utils.FreshMetricQuery(utils.MetricQuery(metric, "node", expectedCluster), started)
		Consistently(func(g Gomega) {
			utils.CheckMinimalProfile(g, PrometheusQueryClient, query, metric)
		}, 90*time.Second, 15*time.Second).Should(Succeed())
	})
	It("ingests fresh Windows exporter CPU, memory and container sentinels from every Windows collector node", Label(utils.WindowsLabel, "cluster-network"), func() {
		if os.Getenv("EXPECT_WINDOWS_EXPORTER") != "true" {
			Skip("Windows exporter is not enabled for this lane")
		}
		pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-win-node")
		Expect(err).NotTo(HaveOccurred())
		Expect(pods).NotTo(BeEmpty())
		for _, metric := range []string{"windows_cpu_time_total", "windows_memory_available_bytes", "windows_container_cpu_usage_seconds_total"} {
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, utils.MetricQuery(metric, "windows-exporter", expectedCluster))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, metric, "windows-exporter", expectedCluster, time.Now(), false)).To(Succeed())
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

	DescribeTable("ingests managed ACNS node and pod metrics after controlled DNS and forward traffic", func(cilium bool) {
		nodePods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", "ama-metrics-node")
		Expect(err).NotTo(HaveOccurred())
		Expect(nodePods).NotTo(BeEmpty(), "ACNS basic metrics require Linux collector nodes")
		podName, podNamespace := os.Getenv("SCENARIO_POD_NAME"), os.Getenv("SCENARIO_POD_NAMESPACE")
		Expect(podName).NotTo(BeEmpty(), "workflow pod name is required for ACNS attribution")
		Expect(podNamespace).NotTo(BeEmpty(), "workflow pod namespace is required for ACNS attribution")
		probeSource := podNamespace + "/" + podName

		client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}
		nodeJob, nodeMetric := "networkobservability-retina", "networkobservability_forward_bytes"
		if cilium {
			nodeJob, nodeMetric = "networkobservability-cilium", "cilium_forward_bytes_total"
		}
		for _, target := range []struct {
			job, metric string
		}{
			{nodeJob, nodeMetric},
			{"networkobservability-hubble", "hubble_dns_queries_total"},
		} {
			query := utils.MetricQuery(target.metric, target.job, expectedCluster)
			source := ""
			if target.job == "networkobservability-hubble" {
				source = probeSource
			}
			if source != "" {
				warmupStarted := time.Now()
				Eventually(func(g Gomega) {
					_, err := net.LookupHost("prometheus-reference-service.default.svc.cluster.local")
					g.Expect(err).NotTo(HaveOccurred())
					samples := utils.FreshSamples(g, PrometheusQueryClient, query, warmupStarted)
					g.Expect(acnsCounterSnapshot(samples, source)).NotTo(BeEmpty())
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
			}
			var baseline map[model.Fingerprint]model.SampleValue
			baselineStarted := time.Now()
			Eventually(func(g Gomega) {
				notBefore := time.Now().Add(-time.Minute)
				if notBefore.Before(baselineStarted) {
					notBefore = baselineStarted
				}
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(query, notBefore))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(samples).NotTo(BeEmpty())
				snapshot := acnsCounterSnapshot(samples, source)
				g.Expect(snapshot).NotTo(BeEmpty(), "no pre-probe %s counter for %q", target.metric, source)
				baseline = snapshot
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
			started := time.Now()
			Eventually(func(g Gomega) {
				_, err := net.LookupHost("prometheus-reference-service.default.svc.cluster.local")
				g.Expect(err).NotTo(HaveOccurred())
				response, err := client.Get("http://prometheus-reference-service.default.svc.cluster.local:2112/metrics")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(response.StatusCode).To(Equal(http.StatusOK))
				_, copyErr := io.Copy(io.Discard, response.Body)
				closeErr := response.Body.Close()
				g.Expect(copyErr).NotTo(HaveOccurred())
				g.Expect(closeErr).NotTo(HaveOccurred())
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(utils.MetricQuery("up", target.job, expectedCluster), started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, "up", target.job, expectedCluster, time.Now(), true)).To(Succeed())
				g.Expect(samples).NotTo(BeEmpty(), "managed %s scrape target is required", target.job)
				warnings, result, err = utils.InstantQuery(PrometheusQueryClient,
					utils.FreshMetricQuery(query, started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok = result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, target.metric, target.job, expectedCluster, time.Now(), false)).To(Succeed())
				g.Expect(samples).NotTo(BeEmpty(), "fresh managed %s metric is required", target.metric)
				if target.job == nodeJob {
					for _, pod := range nodePods {
						g.Expect(pod.Spec.NodeName).NotTo(BeEmpty())
						found := false
						for _, sample := range samples {
							found = found || string(sample.Metric["instance"]) == pod.Spec.NodeName
						}
						g.Expect(found).To(BeTrue(), "node %s has no fresh managed %s sample", pod.Spec.NodeName, nodeJob)
					}
				}
				observedTraffic := false
				for fingerprint, value := range acnsCounterSnapshot(samples, source) {
					before, existed := baseline[fingerprint]
					observedTraffic = observedTraffic || (existed && value > before)
				}
				g.Expect(observedTraffic).To(BeTrue(), "managed %s counter must exceed the fresh pre-probe snapshot", target.metric)
				fmt.Fprintf(GinkgoWriter, "Managed ACNS evidence: job=%s metric=%s source=%q freshSamples=%d positiveIncrease=%t\n",
					target.job, target.metric, source, len(samples), observedTraffic)
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		}
	},
		Entry("managed non-Cilium Retina", Label("managed-acns"), false),
		Entry("managed Cilium", Label("managed-acns-cilium"), true),
	)

	It("ingests node network counters after traffic through the selected AKS network profile", Label("network-profile"), func() {
		started := time.Now()
		baselines := map[string]map[string]model.SampleValue{}
		for _, metric := range []string{"node_network_receive_bytes_total", "node_network_transmit_bytes_total"} {
			warnings, result, err := utils.InstantQuery(PrometheusQueryClient, utils.MetricQuery(metric, "node", expectedCluster))
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
					utils.FreshMetricQuery(utils.MetricQuery(metric, "node", expectedCluster), started))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, metric, "node", expectedCluster, time.Now(), false)).To(Succeed())
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

	DescribeTable("has fresh healthy kubelet data from every collector node", Label("cluster-network"),
		func(controller string) {
			pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "dsName", controller)
			Expect(err).NotTo(HaveOccurred())
			Expect(pods).NotTo(BeEmpty())
			Eventually(func(g Gomega) {
				warnings, result, err := utils.InstantQuery(PrometheusQueryClient, utils.MetricQuery("up", "kubelet", expectedCluster))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(warnings).To(BeEmpty())
				samples, ok := result.(model.Vector)
				g.Expect(ok).To(BeTrue())
				g.Expect(utils.ValidateMetricSamples(samples, "up", "kubelet", expectedCluster, time.Now(), true)).To(Succeed())
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
})

var _ = Describe("Scenario OTLP ingestion", Label("scenario-otlp", "otlp"), func() {
	It("ingests a fresh run-specific OTLP metric", func() {
		expectedCluster := os.Getenv("EXPECTED_CLUSTER")
		if !utils.HasOTLPContext(expectedCluster, os.Getenv("SCENARIO_RUN_ID")) {
			Skip("Requires a scenario")
		}
		id := make([]byte, 16)
		_, err := rand.Read(id)
		Expect(err).NotTo(HaveOccurred())
		marker := hex.EncodeToString(id)
		started := time.Now()
		client := &http.Client{Timeout: 10 * time.Second}
		Eventually(func(g Gomega) {
			g.Expect(utils.ProbeOTLP(client, "http://candidate-otlp.kube-system.svc:56681/v1/metrics",
				PrometheusQueryClient, expectedCluster, marker, started)).To(Succeed())
		}, 10*time.Minute, 15*time.Second).Should(Succeed())
	})
})

var _ = Describe("Query Metrics Test Suite", func() {

	DescribeTable("should return the expected results for specified Prometheus metrics in each job",
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
		}, Label(utils.LinuxDaemonsetCustomConfig)),
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
		}, Label(utils.LinuxDaemonsetCustomConfig)),
		Entry("External labels are applied from Windows DaemonSet Configmap", "windows-node-configmap", "up", map[string]string{
			"external_label_1":   "external_label_value",
			"external_label_123": "external_label_value",
		}, Label(utils.WindowsLabel)),
	)

	Context("When querying metrics", func() {
		DescribeTable("should return the expected results for up=1 for all jobs",
			func(jobs []string) {
				for _, job := range jobs {
					// Run the query for the job
					warnings, result, err := utils.InstantQuery(PrometheusQueryClient, fmt.Sprintf("up{job=\"%s\"} == 1", job))
					Expect(err).NotTo(HaveOccurred(), "failed to execute query for job %s", job)

					// Ensure there are no warnings
					Expect(warnings).To(BeEmpty(), "warnings should be empty for job %s", job)

					// Ensure there is at least one result
					vectorResult, ok := result.(model.Vector)
					Expect(ok).To(BeTrue(), "result should be of type model.Vector for job %s", job)
					Expect(vectorResult).NotTo(BeEmpty(), "result should not be empty for job %s", job)

					// Ensure that all results have the 'up' metric with a value of 1
					for _, sample := range vectorResult {
						Expect(string(sample.Metric["__name__"])).To(Equal("up"), "metric name should be 'up' for job %s", job)
						Expect(sample.Value.String()).To(Equal("1"), "metric value should be '1' for job %s", job)
					}
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
				query := fmt.Sprintf("{\"%s\"}", metric)

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

					_, ok = sample.Metric[model.LabelName("cluster")]
					Expect(ok).To(BeTrue(), fmt.Sprintf("Expected label %q not found in metric %q", "cluster", metric))
				}

				Expect(deltaLabelFound).To(BeTrue(), fmt.Sprintf("Expected metric %q to have a sample with delta temporality label", metric))
				Expect(cumulativeLabelFound).To(BeTrue(), fmt.Sprintf("Expected metric %q to have a sample with cumulative temporality label", metric))
			}
		})

	})
})
