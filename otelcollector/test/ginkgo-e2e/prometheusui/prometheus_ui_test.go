package prometheusui

import (
	"encoding/json"
	"fmt"
	"prometheus-collector/otelcollector/test/utils"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
)

var _ = Describe("The Prometheus UI API", func() {
	Context("for all replicaset pods", func() {
		It("should return the scrape pools for all replicaset pods", func() {
			// Test that the Prometheus UI /scrape_pools API endpoint returns a list that contains at least the default targets.
			var apiResponses = make([]*utils.APIResponse, 2)
			err := utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/scrape_pools", true, true, apiResponses)
			Expect(err).NotTo(HaveOccurred())

			allScrapePools := make([]string, 0)
			for _, apiResponse := range apiResponses {
				Expect(apiResponse).NotTo(BeNil())
				Expect(apiResponse.Data).NotTo(BeNil())

				var scrapePoolData utils.ScrapePoolData
				json.Unmarshal([]byte(apiResponse.Data), &scrapePoolData)
				Expect(scrapePoolData).NotTo(BeNil())
				allScrapePools = append(allScrapePools, scrapePoolData.ScrapePools...)
			}

			Expect(allScrapePools).To(ContainElements("kube-state-metrics", "kubernetes-pods", "prometheus_ref_app"))
		})
	})
})

/*
 * Test that the Prometheus UI /scrape_pools API endpoint returns a list that contains at least the default targets.
 */
var _ = DescribeTable("The Prometheus UI API should return the scrape pools",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expected []string) {
		time.Sleep(60 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/scrape_pools", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())

		Expect(apiResponse).NotTo(BeNil())
		Expect(apiResponse.Data).NotTo(BeNil())

		var scrapePoolData utils.ScrapePoolData
		json.Unmarshal([]byte(apiResponse.Data), &scrapePoolData)
		Expect(scrapePoolData).NotTo(BeNil())
		Expect(scrapePoolData.ScrapePools).To(ContainElements(expected))
	},
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
		true,
		[]string{
			"cadvisor",
			"kubelet",
			"networkobservability-cilium",
			"networkobservability-hubble",
			"networkobservability-retina",
			"node",
		},
	),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
		false,
		[]string{
			"kappie-basic",
			"kubelet",
			"networkobservability-retina",
		},
		Label(utils.WindowsLabel),
	),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
var _ = DescribeTable("The Prometheus UI API should return a valid config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, jobList []string) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())

		for _, expectedJob := range jobList {
			found := false
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				if scrapeJob.JobName == expectedJob {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), fmt.Sprintf("Expected job '%s' was not found in scrape configs", expectedJob))
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, []string{"kubelet", "cadvisor", "node", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, []string{"kubelet", "kappie-basic", "networkobservability-retina"}, Label(utils.WindowsLabel)),
)

/*
 * Test that the Prometheus UI /targets API endpoint returns a list of active and dropped targets.
 */
var _ = DescribeTable("The Prometheus UI API should return the targets",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(180 * time.Second)

		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/targets", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var targetsResult v1.TargetsResult
		json.Unmarshal([]byte(apiResponse.Data), &targetsResult)

		Expect(targetsResult).NotTo(BeNil())
		Expect(targetsResult.Active).NotTo(BeNil())
		Expect(targetsResult.Dropped).NotTo(BeNil())
		for _, target := range targetsResult.Active {
			Expect(target.DiscoveredLabels).NotTo(BeNil())
			Expect(target.Labels).NotTo(BeNil())
		}
		Expect(targetsResult.Dropped).NotTo(BeNil())
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.WindowsLabel)),
)

/*
 * Test that the Prometheus UI /targets/metadata API endpoiont returns a list of targets with metadata.
 */
var _ = DescribeTable("The Prometheus UI API should return the targets metadata",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(240 * time.Second)

		var apiResponse utils.APIResponse
		queryPath := "/api/v1/targets/metadata?match_target=\\{job=\\\"prometheus_ref_app\\\"\\}"
		if !isLinux {
			queryPath = "/api/v1/targets/metadata?match_target={job=`\"kubelet`\"}"
		}
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName,
			queryPath, isLinux,
			&apiResponse,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var metricMetadataResult []v1.MetricMetadata
		json.Unmarshal([]byte(apiResponse.Data), &metricMetadataResult)

		Expect(metricMetadataResult).NotTo(BeNil())
		for _, metricMetadata := range metricMetadataResult {
			Expect(metricMetadata.Target).NotTo(BeNil())
			Expect(metricMetadata.Metric).NotTo(BeEmpty())
			Expect(metricMetadata.Type).NotTo(BeEmpty())
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.WindowsLabel)),
)

/*
 * Test that the Prometheus UI /metrics endpoiont returns the Prometheus metrics.
 */
var _ = DescribeTable("The Prometheus UI should return the /metrics data",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {

		time.Sleep(300 * time.Second)

		pods, err := utils.GetPodsWithLabel(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())

		for _, pod := range pods {
			// Execute the command and capture the output
			var command []string
			if isLinux {
				command = []string{"sh", "-c", "curl --noproxy \"*\" \"http://localhost:9090/metrics\""}
			} else {
				command = []string{"powershell", "-c", "(curl \"http://localhost:9090/metrics\" -UseBasicParsing).Content"}
			}
			stdout, _, err := utils.ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, command)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).NotTo(BeEmpty())
			Expect(stdout).NotTo(ContainSubstring("404 page not found"))
			Expect(stdout).To(ContainSubstring("prometheus_target_scrape_pool_targets"))
		}

		return
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.WindowsLabel)),
)

/*
 * Test that the Prometheus UI does not return a 404 for each UI page.
 */
var _ = DescribeTable("The Prometheus UI should return a 200 for its UI pages",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, uiPaths []string) {

		time.Sleep(360 * time.Second)
		pods, err := utils.GetPodsWithLabel(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())

		for _, pod := range pods {
			// Execute the command and capture the output
			for _, uiPath := range uiPaths {
				var command []string
				if isLinux {
					command = []string{"sh", "-c", fmt.Sprintf("curl --noproxy \"*\" \"http://localhost:9090%s\"", uiPath)}
				} else {
					command = []string{"powershell", "-c", fmt.Sprintf("(curl \"http://localhost:9090/%s\" -UseBasicParsing).Content", uiPath)}
				}
				stdout, _, err := utils.ExecCmd(K8sClient, Cfg, pod.Name, containerName, namespace, command)
				Expect(err).NotTo(HaveOccurred())
				Expect(stdout).NotTo(BeEmpty())
				Expect(stdout).NotTo(ContainSubstring("404 page not found"))
			}

			return
		}
	},
	Entry("when called inside the ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true,
		[]string{
			"/agent",
			"/config",
			"/targets",
			"/service-discovery",
		},
	),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true,
		[]string{
			"/agent",
			"/config",
			"/targets",
			"/service-discovery",
		},
	),
	Entry("when called inside the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false,
		[]string{
			"/agent",
			"/config",
			"/targets",
			"/service-discovery",
		},
		Label(utils.WindowsLabel),
	),
)
