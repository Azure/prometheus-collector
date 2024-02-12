package prometheusui

import (
	"encoding/json"
	"prometheus-collector/otelcollector/test/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
)

var _ = DescribeTable("The Prometheus UI API should return the scrape pools",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, expected []string) {
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/scrape_pools", &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var scrapePoolData utils.ScrapePoolData
		json.Unmarshal([]byte(apiResponse.Data), &scrapePoolData)
		Expect(scrapePoolData).NotTo(BeNil())
		Expect(scrapePoolData.ScrapePools).To(ConsistOf(expected))
	},
	Entry("when called inside the ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
		[]string {
			"application_pods",
      "kube-apiserver",
      "kube-dns",
      "kube-proxy",
      "kube-state-metrics",
      "kubernetes-pods",
      "prometheus_ref_app",
      "win_prometheus_ref_app",
		},
	),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
		[]string {
			"cadvisor",
			"kappie-basic",
			"kubelet",
			"networkobservability-cilium",
			"networkobservability-hubble",
			"networkobservability-retina",
			"node",
		},
	),
)

var _ = DescribeTable("The Prometheus UI API should return a valid config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string) {
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector"),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector"),
)

var _ = DescribeTable("The Prometheus UI API should return the targets",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string) {
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/targets", &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var targetsResult v1.TargetsResult
		json.Unmarshal([]byte(apiResponse.Data), &targetsResult)
		Expect(targetsResult).NotTo(BeNil())

		// fmt.Printf("Active Targets: %d\n", len(targetsResult.Active))
		// for _, target := range targetsResult.Active {
		// 	fmt.Printf("Discovered Labels: %v\n", target.DiscoveredLabels)
		// 	fmt.Printf("Labels: %v\n", target.Labels)
		// 	fmt.Printf("Last Error: %s\n", target.LastError)
		// 	fmt.Printf("Health Status: %s\n", target.Health)
		// }

		// fmt.Printf("Dropped Targets: %d\n", len(targetsResult.Dropped))
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector"),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector"),
)

var _ = DescribeTable("The Prometheus UI API should return the targets metadata",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string) {
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName,
			"/api/v1/targets/metadata?match_target=\\{job=\\\"prometheus_ref_app\\\"\\}",
			&apiResponse,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var metricMetadataResult []v1.MetricMetadata
		json.Unmarshal([]byte(apiResponse.Data), &metricMetadataResult)
		Expect(metricMetadataResult).NotTo(BeNil())
		Expect(len(metricMetadataResult)).To(BeNumerically("==", 64))
		for _, metricMetadata := range metricMetadataResult {
			Expect(metricMetadata.Target).NotTo(BeNil())
			Expect(metricMetadata.Metric).NotTo(BeEmpty())
			Expect(metricMetadata.Type).NotTo(BeEmpty())
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector"),
	//Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector"),
)