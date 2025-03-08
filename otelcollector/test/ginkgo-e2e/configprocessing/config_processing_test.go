package configprocessing

import (
	"encoding/json"
	"prometheus-collector/otelcollector/test/utils"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
)

/*
 * For each of the pods that we deploy in our chart, ensure each container within that pod has status 'Running'.
 * The replicaset, daemonset, and kube-state-metrics are always deployed.
 * The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
 * The label and values are provided to get a list of pods only with that label.
 */
var _ = DescribeTable("The containers should be running",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckIfAllContainersAreRunning(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommon)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommon)),
	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommon)),
	// Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommon)),
	// Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)

// /*
//  * For each of the DS pods that we deploy in our chart, ensure that all nodes have been used to schedule these pods.
//  * The label and values are provided to get a list of pods only with that label.
//  * The osLabel is provided to check on all DS pods based on the OS.
//  */
// var _ = DescribeTable("The pods should be scheduled in all nodes",
// 	func(namespace string, controllerLabelName string, controllerLabelValue string, osLabel string) {
// 		err := utils.CheckIfAllPodsScheduleOnNodes(K8sClient, namespace, controllerLabelName, controllerLabelValue, osLabel)
// 		Expect(err).NotTo(HaveOccurred())
// 	},
// 	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "linux"),
// 	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", "windows", Label(utils.WindowsLabel)),
// )

// /*
//  * For each of the DS pods that we deploy in our chart, ensure that all specific nodes like ARM64,FIPS have been used to schedule these pods.
//  * The label and values are provided to get a list of pods only with that label.
//  */
// var _ = DescribeTable("The pods should be scheduled in all Fips and ARM64 nodes",
// 	func(namespace string, controllerLabelName string, controllerLabelValue string, nodeLabelKey string, nodeLabelValue string) {
// 		err := utils.CheckIfAllPodsScheduleOnSpecificNodesLabels(K8sClient, namespace, controllerLabelName, controllerLabelValue, nodeLabelKey, nodeLabelValue)
// 		Expect(err).NotTo(HaveOccurred())
// 	},
// 	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "kubernetes.azure.com/fips_enabled", "true", Label(utils.FIPSLabel)),
// 	Entry("when checking the ama-metrics-win-node pod", "kube-system", "dsName", "ama-metrics-win-node", "kubernetes.azure.com/fips_enabled", "true", Label(utils.WindowsLabel), Label(utils.FIPSLabel)),
// 	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", "kubernetes.io/arch", "arm64", Label(utils.ARM64Label)),
// )

/*
 * For each of the pods that have the prometheus-collector container, check otelcollector running.
 * The linux replicaset and daemonset will should have the same processes running.
 */
var _ = DescribeTable("otelcollector is running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
		[]string{
			// "fluent-bit",
			"otelcollector",
			// "mdsd -a -A -e",
			// "MetricsExtension",
			// "inotifywait /etc/config/settings",
			// "inotifywait /etc/mdsd.d",
			// "crond",
		}, Label(utils.ConfigProcessingCommon),
	),
	Entry("when checking the ama-metrics-node daemonset pods", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
		[]string{
			// "fluent-bit",
			"otelcollector",
			// "mdsd -a -A -e",
			// "MetricsExtension",
			// "inotifywait /etc/config/settings",
			// "inotifywait /etc/mdsd.d",
			// "crond",
		},
		Label(utils.ConfigProcessingCommon),
	),
)

/*
 * For windows daemonset pods that have the prometheus-collector container, check otelcollector running.
 */
var _ = DescribeTable("otelcollector is running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllWindowsProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics-win-node daemonset pods", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector",
		[]string{
			// "fluent-bit",
			"otelcollector",
			// "MetricsExtension",
			// "MonAgentLauncher",
			// "MonAgentHost",
			// "MonAgentManager",
			// "MonAgentCore",
		},
		Label(utils.ConfigProcessingCommon),
		FlakeAttempts(3),
	),
)

/*
- For each of the pods that we deploy in our chart, ensure each container within that pod doesn't have errors in the logs.
- The replicaset, daemonset, and kube-state-metrics are always deployed.
- The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
- The label and values are provided to get a list of pods only with that label.
*/
var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err.Error()).To(ContainSubstring("configmap section not mounted, using defaults"))
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	// Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	// Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)

var _ = DescribeTable("The container logs should contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	// Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	// Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)

/*
- For each of the pods that we deploy in our chart, ensure each container within that pod doesn't have errors in the logs.
- The replicaset, daemonset, and kube-state-metrics are always deployed.
- The operator-targets and node-exporter workloads are checked if the 'operator' or 'arc-extension' label is included in the test run.
- The label and values are provided to get a list of pods only with that label.
*/
var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonWithConfigMap)),
	// Entry("when checking the ama-metrics-ksm pod", "kube-system", "app.kubernetes.io/name", "ama-metrics-ksm"),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonWithConfigMap)),
	// Entry("when checking the prometheus-node-exporter pod", "kube-system", "app", "prometheus-node-exporter", Label(utils.ArcExtensionLabel)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// No configmaps
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingNoConfigMaps)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingNoConfigMaps)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingNoConfigMaps)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// All targets disabled
var _ = DescribeTable("The Prometheus UI API should return 1 job in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())
		Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 1))
		Expect(prometheusConfig.ScrapeConfigs[0].JobName).To(Equal("empty_job"))

	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingAllTargetsDisabled)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingAllTargetsDisabled)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingAllTargetsDisabled)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// Default settings turned on in settings configmap
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingNoConfigMaps)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingNoConfigMaps)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingNoConfigMaps)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// All Rs targets turned on in settings configmap
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 6))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "kube-dns", "kube-proxy", "kube-apiserver"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingRsTargetsEnabled)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingRsTargetsEnabled)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingRsTargetsEnabled)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// All ds targets turned on in settings configmap
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina", "windows-exporter", "kube-proxy-windows"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingDsTargetsEnabled)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingDsTargetsEnabled)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingDsTargetsEnabled)),
)

/*
 * Test that the Prometheus UI /config API endpoint returns a Prometheus config that can be unmarshaled.
 */
// All Rs and ds targets turned on in settings configmap
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		// // var prometheusConfigResult v1.ConfigResult
		// var prometheusConfigResult map[string]interface{}
		// json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		// Expect(prometheusConfigResult).NotTo(BeNil())
		// //Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())
		// scrapeConfigs := prometheusConfigResult["scrape_configs"]
		// //prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		// Expect(err).NotTo(HaveOccurred())
		// Expect(scrapeConfigs).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 6))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "kube-dns", "kube-proxy", "kube-apiserver"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina", "windows-exporter", "kube-proxy-windows"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingAllTargetsEnabled)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingAllTargetsEnabled)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingAllTargetsEnabled)),
)

// Only Custom configmap with all actions
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 14))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"job-replace", "job-lowercase", "job-uppercase", "job-keep", "job-drop", "job-keepequal", "job-dropequal",
				"job-hashmod", "job-labelmap", "job-labeldrop", "job-labelkeep"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingOnlyCustomConfigMapWithAllActions)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingOnlyCustomConfigMapWithAllActions)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingOnlyCustomConfigMapWithAllActions)),
)

// Global settings added, settings configmap def targets and configmap all actions
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"prometheus_ref_app", "win_prometheus_ref_app"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
		if controllerLabelValue == "ama-metrics" {
			ext := prometheusConfig.GlobalConfig.ExternalLabels
			Expect(ext.String()).To(Equal("{extlabel1=\"extlabel1\", extlabel2=\"extlabel2\"}"))
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettings)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettings)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingGlobalSettings)),
)

// Global and custom job added in node configmap, settings configmap def targets and configmap all actions
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 14))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"job-replace", "job-lowercase", "job-uppercase", "job-keep", "job-drop", "job-keepequal", "job-dropequal",
				"job-hashmod", "job-labelmap", "job-labeldrop", "job-labelkeep"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 8))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium", "node-configmap"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 4))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina", "windows-node-configmap"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
		if controllerLabelValue == "ama-metrics-node" {
			ext := prometheusConfig.GlobalConfig.ExternalLabels
			Expect(ext.String()).To(Equal("{external_label_1=\"external_label_value\", external_label_123=\"external_label_value\"}"))
		}
		if controllerLabelValue == "ama-metrics-win-node" {
			ext := prometheusConfig.GlobalConfig.ExternalLabels
			Expect(ext.String()).To(Equal("{external_label_1=\"external_label_value\", external_label_123=\"external_label_value\"}"))
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingSettingsNodeConfigMap)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingSettingsNodeConfigMap)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingSettingsNodeConfigMap)),
)

// Errorprone settings configmap, no custom config
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingSettingsError)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingSettingsError)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingSettingsError)),
)

// Errorprone custom configmap, def settings targets turned on
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingCustomConfigMapError)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingCustomConfigMapError)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingCustomConfigMapError)),
)

// Errorprone global settings, def settings targets turned on, custom configmap all actions
var _ = DescribeTable("The Prometheus UI API should return some jobs in config",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool) {
		time.Sleep(120 * time.Second)
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, namespace, controllerLabelName, controllerLabelValue, containerName, "/api/v1/status/config", isLinux, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, true, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(rsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 7))
			linuxDsJobs := []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(linuxDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		} else if controllerLabelValue == "ama-metrics-win-node" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 3))
			windowsDsJobs := []string{"kubelet", "kappie-basic", "networkobservability-retina"}
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				Expect(windowsDsJobs).To(ContainElement(scrapeJob.JobName))
			}
		}
		if controllerLabelValue == "ama-metrics" {
			ext := prometheusConfig.GlobalConfig.ExternalLabels
			Expect(ext).To(BeNil())
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettingsError)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettingsError)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingGlobalSettingsError)),
)
