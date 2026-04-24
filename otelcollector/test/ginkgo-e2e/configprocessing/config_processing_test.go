package configprocessing

import (
	"encoding/json"
	"fmt"
	"prometheus-collector/otelcollector/test/utils"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
)

/*
 * For each of the pods that we deploy in our chart, ensure each container within that pod has status 'Running'.
 * The replicaset, daemonset, and operator-targets are always deployed.
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
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommon)),
)

/*
 * For each of the pods that have the prometheus-collector container, check otelcollector running.
 */
var _ = DescribeTable("otelcollector is running",
	func(namespace, labelName, labelValue, containerName string, processes []string) {
		err := utils.CheckAllProcessesRunning(K8sClient, Cfg, labelName, labelValue, namespace, containerName, processes)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pod(s)", "kube-system", "rsName", "ama-metrics", "prometheus-collector",
		[]string{
			"otelcollector",
		}, Label(utils.ConfigProcessingCommon),
		FlakeAttempts(3),
	),
	Entry("when checking the ama-metrics-node daemonset pods", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector",
		[]string{
			"otelcollector",
		},
		Label(utils.ConfigProcessingCommon),
		FlakeAttempts(3),
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
			"otelcollector",
		},
		Label(utils.ConfigProcessingCommon),
		FlakeAttempts(3),
	),
)

/*
- For each of the pods that we deploy in our chart, ensure each container within that pod doesn't have errors in the logs except for configmap section not mounted.
- The replicaset, daemonset, and operator-targets are always deployed.
- The label and values are provided to get a list of pods only with that label.
*/
var _ = DescribeTable("The container logs should not contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonNoConfigMaps)),
)

/*
- For each of the pods that we deploy in our chart, ensure each container within that pod has errors in the logs.
- The replicaset, daemonset, and operator-targets are always deployed.
- The label and values are provided to get a list of pods only with that label.
*/
var _ = DescribeTable("The container logs should contain errors",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckContainerLogsForErrors(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).To(HaveOccurred())
	},
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics replica pods", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-node", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonWithErrorConfigMap)),
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
	Entry("when checking the ama-metrics-operator-targets pod", "kube-system", "rsName", "ama-metrics-operator-targets", Label(utils.ConfigProcessingCommonWithConfigMap)),
)

/*
 * Ensure MINIMAL_INGESTION_PROFILE is always logged as true (both without and with configmaps).
 * This validates the defaulting logic and explicit config handling.
 */
var _ = DescribeTable("MINIMAL_INGESTION_PROFILE should be true in logs",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckMinimalIngestionProfileTrue(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	// No configmaps scenario
	Entry("rs pod (no configmaps)", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("linux ds pod (no configmaps)", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	Entry("windows ds pod (no configmaps)", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonNoConfigMaps)),
	// With configmaps scenario
	Entry("rs pod (with configmaps)", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("linux ds pod (with configmaps)", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithConfigMap)),
	Entry("windows ds pod (with configmaps)", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonWithConfigMap)),
	// With v2 configmaps scenario (schema v2)
	Entry("rs pod (with configmaps v2)", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingCommonWithConfigMapV2)),
	Entry("linux ds pod (with configmaps v2)", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingCommonWithConfigMapV2)),
	Entry("windows ds pod (with configmaps v2)", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingCommonWithConfigMapV2)),
)

/*
 * Ensure MINIMAL_INGESTION_PROFILE is logged as false when the settings configmap
 * explicitly sets minimalingestionprofile = false.
 */
var _ = DescribeTable("MINIMAL_INGESTION_PROFILE should be false in logs",
	func(namespace string, controllerLabelName string, controllerLabelValue string) {
		err := utils.CheckMinimalIngestionProfileFalse(K8sClient, namespace, controllerLabelName, controllerLabelValue)
		Expect(err).NotTo(HaveOccurred())
	},
	Entry("rs pod (mip false)", "kube-system", "rsName", "ama-metrics", Label(utils.ConfigProcessingMipFalse)),
	Entry("linux ds pod (mip false)", "kube-system", "dsName", "ama-metrics-node", Label(utils.ConfigProcessingMipFalse)),
	Entry("windows ds pod (mip false)", "kube-system", "dsName", "ama-metrics-win-node", Label(utils.ConfigProcessingMipFalse)),
)

/*
 * Following tests make sure the Prometheus config as seen by otelcollector can be unmarshaled and only contain jobs we expect
 */

// Test case for No configmaps
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		// Debug: always log discovered job names to help diagnose mismatches
		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
		}
		GinkgoWriter.Printf("[NoConfigMaps] jobs=%v\n", jobNames)

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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

// All targets disabled
var _ = DescribeTable("The Prometheus UI API should return 1 job in config",
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
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

// Default settings turned on in settings configmap
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
		}
		GinkgoWriter.Printf("[DefaultTargetsEnabled] jobs=%v\n", jobNames)

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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

// All Rs targets turned on in settings configmap
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 10))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "kube-dns", "kube-proxy", "kube-apiserver", "local-csi-driver", "istio-cni", "ztunnel", "dcgm-exporter"}
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

// All ds targets turned on in settings configmap
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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

// All Rs and ds targets turned on in settings configmap
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 10))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "kube-dns", "kube-proxy", "kube-apiserver", "local-csi-driver", "istio-cni", "ztunnel", "dcgm-exporter"}
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 16))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"job-replace", "job-lowercase", "job-uppercase", "job-keep", "job-drop", "job-keepequal", "job-dropequal",
				"job-hashmod", "job-labelmap", "job-labeldrop", "job-labelkeep", "local-csi-driver", "dcgm-exporter"}
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 8))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"prometheus_ref_app", "win_prometheus_ref_app", "application_pods", "local-csi-driver", "dcgm-exporter"}
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
			Expect(ext.String()).To(Equal("{external_label_1=\"external_label_value\", external_label_123=\"external_label_value\"}"))
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 16))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics",
				"job-replace", "job-lowercase", "job-uppercase", "job-keep", "job-drop", "job-keepequal", "job-dropequal",
				"job-hashmod", "job-labelmap", "job-labeldrop", "job-labelkeep", "local-csi-driver", "dcgm-exporter"}
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 5))
			rsJobs := []string{"acstor-capacity-provisioner", "acstor-metrics-exporter", "kube-state-metrics", "local-csi-driver", "dcgm-exporter"}
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
			Expect(ext.String()).To(Equal("{}"))
		}
	},
	Entry("when called inside ama-metrics replica pod", "kube-system", "rsName", "ama-metrics", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettingsError)),
	Entry("when called inside the ama-metrics-node pod", "kube-system", "dsName", "ama-metrics-node", "prometheus-collector", true, Label(utils.ConfigProcessingGlobalSettingsError)),
	Entry("when checking the ama-metrics-win-node", "kube-system", "dsName", "ama-metrics-win-node", "prometheus-collector", false, Label(utils.ConfigProcessingGlobalSettingsError)),
)

// DCGM exporter with custom scrape interval and regex override
var _ = DescribeTable("The Prometheus UI API should respect dcgm-exporter custom scrape interval and regex overrides",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expectedScrapeInterval string, expectedRegexPatterns []string) {
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
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			var dcgmJobConfig *config.ScrapeConfig
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				if scrapeJob.JobName == "dcgm-exporter" {
					dcgmJobConfig = scrapeJob
					break
				}
			}
			Expect(dcgmJobConfig).NotTo(BeNil(), "dcgm-exporter job should be present")

			// Verify custom scrape interval
			actualScrapeInterval := dcgmJobConfig.ScrapeInterval.String()
			GinkgoWriter.Printf("[DCGMCustomConfig] Custom scrape interval - expected=%s, actual=%s\n", expectedScrapeInterval, actualScrapeInterval)
			Expect(actualScrapeInterval).To(Equal(expectedScrapeInterval), "dcgm-exporter should use custom scrape interval from configmap")

			// Verify custom metric regex override
			Expect(dcgmJobConfig.MetricRelabelConfigs).NotTo(BeEmpty(), "dcgm-exporter should have metric relabeling")

			// Find the keep action with custom regex
			foundCustomRegex := false
			for _, relabelConfig := range dcgmJobConfig.MetricRelabelConfigs {
				if relabelConfig.Action == "keep" && len(relabelConfig.SourceLabels) > 0 && string(relabelConfig.SourceLabels[0]) == "__name__" {
					foundCustomRegex = true
					regex := relabelConfig.Regex.String()
					GinkgoWriter.Printf("[DCGMCustomConfig] Custom metric regex=%s\n", regex)

					// Verify all expected patterns are in the regex
					for _, pattern := range expectedRegexPatterns {
						Expect(regex).To(ContainSubstring(pattern), "dcgm-exporter regex should contain custom pattern: %s", pattern)
					}

					// Verify it still includes the minimal ingestion profile pattern (DCGM_.*)
					Expect(regex).To(ContainSubstring("DCGM_"), "dcgm-exporter regex should still include minimal ingestion profile DCGM_ pattern")
					break
				}
			}
			Expect(foundCustomRegex).To(BeTrue(), "dcgm-exporter should have custom regex pattern in metric relabeling")

			GinkgoWriter.Printf("[DCGMCustomConfig] Custom interval and regex override validated successfully\n")
		}
	},
	Entry("when dcgm-exporter has custom 60s interval and specific metric regex",
		"kube-system", "rsName", "ama-metrics", "prometheus-collector", true, "1m0s",
		[]string{"DCGM_FI_DEV_GPU_UTIL", "DCGM_FI_DEV_MEM_COPY_UTIL", "DCGM_FI_DEV_POWER_USAGE"},
		Label(utils.ConfigProcessingDcgmExporterEnabled)),
)

// Controlplane Istio with custom scrape interval and regex override
// This test requires MESH_MEMBER_METRICS_FQDN to be set in the deployment environment
var _ = DescribeTable("The Prometheus UI API should respect controlplane-istio custom scrape interval and regex overrides",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expectedScrapeInterval string, expectedRegexPatterns []string, notExpectedRegexPatterns []string) {
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
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		if controllerLabelValue == "ama-metrics" {
			// Verify total scrape config count includes controlplane-istio (10 default RS + controlplane-istio)
			Expect(len(prometheusConfig.ScrapeConfigs)).To(BeNumerically("==", 11),
				"should have 11 scrape configs (10 default RS + controlplane-istio)")

			var controlplaneIstioJobConfig *config.ScrapeConfig
			for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
				if scrapeJob.JobName == "controlplane-istio" {
					controlplaneIstioJobConfig = scrapeJob
					break
				}
			}
			Expect(controlplaneIstioJobConfig).NotTo(BeNil(), "controlplane-istio job should be present")

			// Verify custom scrape interval
			actualScrapeInterval := controlplaneIstioJobConfig.ScrapeInterval.String()
			GinkgoWriter.Printf("[ControlplaneIstioCustomConfig] Custom scrape interval - expected=%s, actual=%s\n", expectedScrapeInterval, actualScrapeInterval)
			Expect(actualScrapeInterval).To(Equal(expectedScrapeInterval), "controlplane-istio should use custom scrape interval from configmap")

			// Verify custom metric regex override
			Expect(controlplaneIstioJobConfig.MetricRelabelConfigs).NotTo(BeEmpty(), "controlplane-istio should have metric relabeling")

			// Find the keep action with custom regex
			foundCustomRegex := false
			for _, relabelConfig := range controlplaneIstioJobConfig.MetricRelabelConfigs {
				if relabelConfig.Action == "keep" && len(relabelConfig.SourceLabels) > 0 && string(relabelConfig.SourceLabels[0]) == "__name__" {
					foundCustomRegex = true
					regex := relabelConfig.Regex.String()
					GinkgoWriter.Printf("[ControlplaneIstioCustomConfig] Custom metric regex=%s\n", regex)

					// Verify all expected patterns are in the regex
					for _, pattern := range expectedRegexPatterns {
						Expect(regex).To(ContainSubstring(pattern), "controlplane-istio regex should contain custom pattern: %s", pattern)
					}

					// Verify it still includes the minimal ingestion profile patterns
					Expect(regex).To(ContainSubstring("pilot_xds_pushes"), "controlplane-istio regex should still include minimal ingestion profile pilot_xds_pushes pattern")

					// Verify unconfigured patterns are NOT in the regex (negative test)
					for _, pattern := range notExpectedRegexPatterns {
						Expect(regex).NotTo(ContainSubstring(pattern), "controlplane-istio regex should NOT contain unconfigured pattern: %s", pattern)
					}
					break
				}
			}
			Expect(foundCustomRegex).To(BeTrue(), "controlplane-istio should have custom regex pattern in metric relabeling")

			GinkgoWriter.Printf("[ControlplaneIstioCustomConfig] Custom interval and regex override validated successfully\n")
		}
	},
	Entry("when controlplane-istio has custom 60s interval and specific metric regex",
		"kube-system", "rsName", "ama-metrics", "prometheus-collector", true, "1m",
		[]string{"pilot_xds_pushes", "pilot_xds_push_context_errors", "pilot_conflict_inbound_listener"},
		[]string{"dummy_metric_not_configured"},
		Label(utils.ConfigProcessingControlplaneIstioEnabled)),
)

// parseMinorVersion extracts the minor version number from a Kubernetes version string like "1.35" or "v1.36.0-preview".
func parseMinorVersion(gitVersion string) (int, error) {
	v := strings.TrimPrefix(gitVersion, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected version format: %s", gitVersion)
	}
	return strconv.Atoi(parts[1])
}

// Basic auth ServiceMonitor — verify RBAC, config, and targets
var _ = Describe("Basic auth ServiceMonitor scraping", Label(utils.ConfigProcessingBasicAuthSmon), func() {
	It("should have the correct ClusterRole permissions, config, and healthy targets for the basic-auth ServiceMonitor", func() {
		GinkgoWriter.Printf("[BasicAuth] Starting test — waiting 120s for cluster to stabilize...\n")
		time.Sleep(120 * time.Second)

		// --- Detect Kubernetes version ---
		GinkgoWriter.Printf("[BasicAuth] Step 1: Detecting Kubernetes version...\n")
		versionInfo, err := utils.GetKubernetesVersion(K8sClient)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuth] Kubernetes version: %s\n", versionInfo.GitVersion)

		minor, err := parseMinorVersion(versionInfo.GitVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuth] Parsed minor version: %d\n", minor)

		// --- ClusterRole assertions ---
		GinkgoWriter.Printf("[BasicAuth] Step 2: Checking ClusterRole ama-metrics-reader for secrets rule...\n")
		clusterRole, err := utils.GetClusterRole(K8sClient, "ama-metrics-reader")
		Expect(err).NotTo(HaveOccurred())
		Expect(clusterRole).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuth] ClusterRole ama-metrics-reader found with %d rules\n", len(clusterRole.Rules))

		// Check for a generic secrets rule (no resourceNames constraint)
		hasGenericSecretsRule := false
		for _, rule := range clusterRole.Rules {
			isSecretsResource := false
			for _, res := range rule.Resources {
				if res == "secrets" {
					isSecretsResource = true
					break
				}
			}
			if !isSecretsResource {
				continue
			}
			// Skip rules that are scoped to specific resourceNames (e.g., aad-msi-auth-token)
			if len(rule.ResourceNames) > 0 {
				GinkgoWriter.Printf("[BasicAuth] Skipping scoped secrets rule (resourceNames=%v)\n", rule.ResourceNames)
				continue
			}
			hasGetListWatch := false
			for _, verb := range rule.Verbs {
				if verb == "get" || verb == "list" || verb == "watch" {
					hasGetListWatch = true
					break
				}
			}
			if hasGetListWatch {
				hasGenericSecretsRule = true
				GinkgoWriter.Printf("[BasicAuth] Found generic secrets rule with verbs: %v\n", rule.Verbs)
				break
			}
		}

		if minor < 36 {
			GinkgoWriter.Printf("[BasicAuth] K8s < 1.36: expecting generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeTrue(), "On K8s < 1.36, ClusterRole ama-metrics-reader should have a generic secrets get/list/watch rule")
			GinkgoWriter.Printf("[BasicAuth] ClusterRole secrets rule check passed\n")
		} else {
			GinkgoWriter.Printf("[BasicAuth] K8s >= 1.36: expecting NO generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeFalse(), "On K8s >= 1.36, ClusterRole ama-metrics-reader should NOT have a generic secrets get/list/watch rule")

			// Verify namespaced Role and RoleBinding exist
			GinkgoWriter.Printf("[BasicAuth] Checking namespaced Role and RoleBinding in basic-auth-test...\n")
			role, err := utils.GetRole(K8sClient, "basic-auth-test", "ama-metrics-secrets-reader")
			Expect(err).NotTo(HaveOccurred(), "Role ama-metrics-secrets-reader should exist in basic-auth-test namespace")
			Expect(role).NotTo(BeNil())

			roleBinding, err := utils.GetRoleBinding(K8sClient, "basic-auth-test", "ama-metrics-secrets-rolebinding")
			Expect(err).NotTo(HaveOccurred(), "RoleBinding ama-metrics-secrets-rolebinding should exist in basic-auth-test namespace")
			Expect(roleBinding).NotTo(BeNil())
			GinkgoWriter.Printf("[BasicAuth] Verified Role and RoleBinding exist in basic-auth-test namespace\n")
		}

		// --- Config check: verify basic-auth job exists with correct basic_auth config ---
		GinkgoWriter.Printf("[BasicAuth] Step 3: Querying Prometheus config from ama-metrics RS pod...\n")
		var apiResponse utils.APIResponse
		err = utils.QueryPromUIFromPod(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/status/config", true, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuth] Successfully retrieved Prometheus config\n")

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuth] Parsed Prometheus config with %d scrape configs\n", len(prometheusConfig.ScrapeConfigs))

		// Find the basic-auth scrape job
		var basicAuthJob *config.ScrapeConfig
		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
			if strings.Contains(sc.JobName, "basic-auth") {
				basicAuthJob = sc
			}
		}
		GinkgoWriter.Printf("[BasicAuth] All jobs: %v\n", jobNames)
		Expect(basicAuthJob).NotTo(BeNil(), "Expected a scrape job containing 'basic-auth' in its name, found jobs: %v", jobNames)
		GinkgoWriter.Printf("[BasicAuth] Found basic-auth job: %s\n", basicAuthJob.JobName)

		// Verify the job has basic_auth configured with username=admin
		GinkgoWriter.Printf("[BasicAuth] Step 4: Verifying basic_auth credentials in scrape config...\n")
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth).NotTo(BeNil(), "basic-auth job should have basic_auth configured")
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth.Username).To(Equal("admin"), "basic-auth job should have username=admin")
		GinkgoWriter.Printf("[BasicAuth] Verified basic_auth username=admin\n")

		// --- Targets check: verify target is up ---
		// Wait for at least one scrape interval to elapse so targets appear
		GinkgoWriter.Printf("[BasicAuth] Step 5: Waiting 60s for first scrape interval to complete...\n")
		time.Sleep(60 * time.Second)

		// Query targets from both RS pods since the basic-auth target may be assigned to either one
		GinkgoWriter.Printf("[BasicAuth] Querying targets from both RS pods...\n")
		var targetsResponses = make([]*utils.APIResponse, 2)
		err = utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/targets", true, true, targetsResponses)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuth] Successfully queried targets from RS pods\n")

		foundBasicAuthTarget := false
		for podIdx, targetsResponse := range targetsResponses {
			if targetsResponse == nil {
				GinkgoWriter.Printf("[BasicAuth] RS pod %d: no response (nil), skipping\n", podIdx)
				continue
			}
			Expect(targetsResponse.Data).NotTo(BeNil())

			var targetsResult v1.TargetsResult
			json.Unmarshal([]byte(targetsResponse.Data), &targetsResult)
			Expect(targetsResult).NotTo(BeNil())
			Expect(targetsResult.Active).NotTo(BeNil())
			GinkgoWriter.Printf("[BasicAuth] RS pod %d: found %d active targets\n", podIdx, len(targetsResult.Active))

			for _, target := range targetsResult.Active {
				if strings.Contains(string(target.ScrapePool), "basic-auth") {
					foundBasicAuthTarget = true
					GinkgoWriter.Printf("[BasicAuth] Found target in RS pod %d scrape pool: %s, health: %s, lastError: %s\n", podIdx, target.ScrapePool, target.Health, target.LastError)
					Expect(target.Health).To(Equal(v1.HealthGood), "basic-auth target should be healthy (up)")
					break
				}
			}
			if foundBasicAuthTarget {
				break
			}
			GinkgoWriter.Printf("[BasicAuth] RS pod %d: no basic-auth target found in active targets\n", podIdx)
		}
		Expect(foundBasicAuthTarget).To(BeTrue(), "Expected to find an active target in a basic-auth scrape pool in at least one RS pod")

		GinkgoWriter.Printf("[BasicAuth] All checks passed\n")
	})
})

// Basic auth secret update — verify updated username is reflected in config
var _ = Describe("Basic auth secret update", Label(utils.ConfigProcessingBasicAuthSecretUpdate), func() {
	It("should reflect the updated username in the Prometheus scrape config", func() {
		time.Sleep(120 * time.Second)

		// Query config from ama-metrics RS
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/status/config", true, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)
		Expect(prometheusConfigResult).NotTo(BeNil())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		// Find the basic-auth scrape job
		var basicAuthJob *config.ScrapeConfig
		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
			if strings.Contains(sc.JobName, "basic-auth") {
				basicAuthJob = sc
			}
		}
		GinkgoWriter.Printf("[BasicAuthSecretUpdate] All jobs: %v\n", jobNames)
		Expect(basicAuthJob).NotTo(BeNil(), "Expected a scrape job containing 'basic-auth' in its name, found jobs: %v", jobNames)

		// Verify the job has basic_auth with updated username=newadmin
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth).NotTo(BeNil(), "basic-auth job should have basic_auth configured")
		GinkgoWriter.Printf("[BasicAuthSecretUpdate] basic_auth username=%s\n", basicAuthJob.HTTPClientConfig.BasicAuth.Username)
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth.Username).To(Equal("newadmin"), "basic-auth job should have updated username=newadmin after secret update")

		GinkgoWriter.Printf("[BasicAuthSecretUpdate] Secret update verification passed\n")
	})
})
