package configprocessing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"prometheus-collector/otelcollector/test/utils"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // Register service discovery implementations.
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
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

// Partial configmap (no prometheus-collector-settings section) — v1 and v2.
//
// Regression test for the v2 partial-configmap bug AND its v1 parity control. A
// schema-version v2 ama-metrics-settings-configmap that omits the optional
// prometheus-collector-settings section must still have its customizations honored.
// Before the ParseMetricsFiles fix, the missing section file caused the whole v2
// configmap to be discarded and the agent fell back to defaults. v1 uses a separate
// directory-based parser (ParseV1Config) that was never affected, so the v1 case is a
// control that should pass on every agent version.
//
// Both fixtures set a distinctive kube-state-metrics scrape interval (59s) and a custom
// keep-list with PV/PVC metrics that are NOT in the minimal-ingestion-profile baseline.
// If the partial configmap is honored, the served kube-state-metrics job reflects both;
// if it is discarded (the v2 bug), the interval falls back to 30s and the PV/PVC metrics
// are absent from the keep-list regex.
func assertPartialConfigmapHonored(schemaVersion string, namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expectedScrapeInterval string, expectedRegexPatterns []string) {
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
		var kubeStateJobConfig *config.ScrapeConfig
		for _, scrapeJob := range prometheusConfig.ScrapeConfigs {
			if scrapeJob.JobName == "kube-state-metrics" {
				kubeStateJobConfig = scrapeJob
				break
			}
		}
		Expect(kubeStateJobConfig).NotTo(BeNil(), "kube-state-metrics job should be present")

		// The custom scrape interval must survive the partial configmap.
		// For v2 pre-fix the whole configmap was discarded and this would fall back to the 30s default.
		actualScrapeInterval := kubeStateJobConfig.ScrapeInterval.String()
		GinkgoWriter.Printf("[PartialNoPcs-%s] kube-state-metrics scrape interval - expected=%s, actual=%s\n", schemaVersion, expectedScrapeInterval, actualScrapeInterval)
		Expect(actualScrapeInterval).To(Equal(expectedScrapeInterval), "kube-state-metrics should use the custom scrape interval from the partial %s configmap", schemaVersion)

		// The custom keep-list must survive the partial configmap.
		Expect(kubeStateJobConfig.MetricRelabelConfigs).NotTo(BeEmpty(), "kube-state-metrics should have metric relabeling")

		foundCustomRegex := false
		for _, relabelConfig := range kubeStateJobConfig.MetricRelabelConfigs {
			if relabelConfig.Action == "keep" && len(relabelConfig.SourceLabels) > 0 && string(relabelConfig.SourceLabels[0]) == "__name__" {
				foundCustomRegex = true
				regex := relabelConfig.Regex.String()
				GinkgoWriter.Printf("[PartialNoPcs-%s] kube-state-metrics keep regex=%s\n", schemaVersion, regex)

				// The custom PV/PVC metrics (not part of the minimal ingestion profile baseline)
				// must be present. For v2 pre-fix they would be absent because the configmap was discarded.
				for _, pattern := range expectedRegexPatterns {
					Expect(regex).To(ContainSubstring(pattern), "kube-state-metrics regex should contain custom keep-list pattern: %s", pattern)
				}

				// The minimal ingestion profile baseline (e.g. kube_node_info) should still be unioned in.
				Expect(regex).To(ContainSubstring("kube_node_info"), "kube-state-metrics regex should still include the minimal ingestion profile baseline")
				break
			}
		}
		Expect(foundCustomRegex).To(BeTrue(), "kube-state-metrics should have the custom keep-list regex in metric relabeling")

		GinkgoWriter.Printf("[PartialNoPcs-%s] Partial configmap honored: custom interval and keep-list validated successfully\n", schemaVersion)
	}
}

// v1 partial configmap (control: ParseV1Config was never affected by the bug).
var _ = DescribeTable("The Prometheus UI API should honor a partial v1 configmap with no prometheus-collector-settings",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expectedScrapeInterval string, expectedRegexPatterns []string) {
		assertPartialConfigmapHonored("v1", namespace, controllerLabelName, controllerLabelValue, containerName, isLinux, expectedScrapeInterval, expectedRegexPatterns)
	},
	Entry("when a v1 configmap omits prometheus-collector-settings but sets a custom kube-state interval and keep-list",
		"kube-system", "rsName", "ama-metrics", "prometheus-collector", true, "59s",
		[]string{"kube_persistentvolumeclaim_info", "kube_persistentvolume_capacity_bytes"},
		Label(utils.ConfigProcessingPartialNoPcsV1)),
)

// v2 partial configmap (regression test for the ParseMetricsFiles fix).
var _ = DescribeTable("The Prometheus UI API should honor a partial v2 configmap with no prometheus-collector-settings",
	func(namespace string, controllerLabelName string, controllerLabelValue string, containerName string, isLinux bool, expectedScrapeInterval string, expectedRegexPatterns []string) {
		assertPartialConfigmapHonored("v2", namespace, controllerLabelName, controllerLabelValue, containerName, isLinux, expectedScrapeInterval, expectedRegexPatterns)
	},
	Entry("when a v2 configmap omits prometheus-collector-settings but sets a custom kube-state interval and keep-list",
		"kube-system", "rsName", "ama-metrics", "prometheus-collector", true, "59s",
		[]string{"kube_persistentvolumeclaim_info", "kube_persistentvolume_capacity_bytes"},
		Label(utils.ConfigProcessingPartialNoPcsV2)),
)

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

		parsedVersion, err := utilversion.ParseSemantic(versionInfo.GitVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuth] Parsed version: %s\n", parsedVersion.String())

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

		if parsedVersion.LessThan(utilversion.MustParseSemantic("v1.37.0")) {
			GinkgoWriter.Printf("[BasicAuth] K8s < 1.37: expecting generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeTrue(), "On K8s < 1.37, ClusterRole ama-metrics-reader should have a generic secrets get/list/watch rule")
			GinkgoWriter.Printf("[BasicAuth] ClusterRole secrets rule check passed\n")
		} else {
			GinkgoWriter.Printf("[BasicAuth] K8s >= 1.37: expecting NO generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeFalse(), "On K8s >= 1.37, ClusterRole ama-metrics-reader should NOT have a generic secrets get/list/watch rule")

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
		Expect(json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)).To(Succeed())
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
			Expect(json.Unmarshal([]byte(targetsResponse.Data), &targetsResult)).To(Succeed())
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
		Expect(json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)).To(Succeed())
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

// Basic auth ServiceMonitor with v2 configmap — verify RBAC, config, and targets
var _ = Describe("Basic auth ServiceMonitor scraping (v2)", Label(utils.ConfigProcessingBasicAuthSmonV2), func() {
	It("should have the correct ClusterRole permissions, config, and healthy targets for the basic-auth ServiceMonitor with v2 configmap", func() {
		GinkgoWriter.Printf("[BasicAuthV2] Starting test — waiting 120s for cluster to stabilize...\n")
		time.Sleep(120 * time.Second)

		// --- Detect Kubernetes version ---
		GinkgoWriter.Printf("[BasicAuthV2] Step 1: Detecting Kubernetes version...\n")
		versionInfo, err := utils.GetKubernetesVersion(K8sClient)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuthV2] Kubernetes version: %s\n", versionInfo.GitVersion)

		parsedVersion, err := utilversion.ParseSemantic(versionInfo.GitVersion)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuthV2] Parsed version: %s\n", parsedVersion.String())

		// --- ClusterRole assertions ---
		GinkgoWriter.Printf("[BasicAuthV2] Step 2: Checking ClusterRole ama-metrics-reader for secrets rule...\n")
		clusterRole, err := utils.GetClusterRole(K8sClient, "ama-metrics-reader")
		Expect(err).NotTo(HaveOccurred())
		Expect(clusterRole).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuthV2] ClusterRole ama-metrics-reader found with %d rules\n", len(clusterRole.Rules))

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
			if len(rule.ResourceNames) > 0 {
				GinkgoWriter.Printf("[BasicAuthV2] Skipping scoped secrets rule (resourceNames=%v)\n", rule.ResourceNames)
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
				GinkgoWriter.Printf("[BasicAuthV2] Found generic secrets rule with verbs: %v\n", rule.Verbs)
				break
			}
		}

		if parsedVersion.LessThan(utilversion.MustParseSemantic("v1.37.0")) {
			GinkgoWriter.Printf("[BasicAuthV2] K8s < 1.37: expecting generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeTrue(), "On K8s < 1.37, ClusterRole ama-metrics-reader should have a generic secrets get/list/watch rule")
			GinkgoWriter.Printf("[BasicAuthV2] ClusterRole secrets rule check passed\n")
		} else {
			GinkgoWriter.Printf("[BasicAuthV2] K8s >= 1.37: expecting NO generic secrets rule in ClusterRole\n")
			Expect(hasGenericSecretsRule).To(BeFalse(), "On K8s >= 1.37, ClusterRole ama-metrics-reader should NOT have a generic secrets get/list/watch rule")

			GinkgoWriter.Printf("[BasicAuthV2] Checking namespaced Role and RoleBinding in basic-auth-test...\n")
			role, err := utils.GetRole(K8sClient, "basic-auth-test", "ama-metrics-secrets-reader")
			Expect(err).NotTo(HaveOccurred(), "Role ama-metrics-secrets-reader should exist in basic-auth-test namespace")
			Expect(role).NotTo(BeNil())

			roleBinding, err := utils.GetRoleBinding(K8sClient, "basic-auth-test", "ama-metrics-secrets-rolebinding")
			Expect(err).NotTo(HaveOccurred(), "RoleBinding ama-metrics-secrets-rolebinding should exist in basic-auth-test namespace")
			Expect(roleBinding).NotTo(BeNil())
			GinkgoWriter.Printf("[BasicAuthV2] Verified Role and RoleBinding exist in basic-auth-test namespace\n")
		}

		// --- Config check: verify basic-auth job exists with correct basic_auth config ---
		GinkgoWriter.Printf("[BasicAuthV2] Step 3: Querying Prometheus config from ama-metrics RS pod...\n")
		var apiResponse utils.APIResponse
		err = utils.QueryPromUIFromPod(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/status/config", true, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuthV2] Successfully retrieved Prometheus config\n")

		var prometheusConfigResult v1.ConfigResult
		Expect(json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)).To(Succeed())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())
		GinkgoWriter.Printf("[BasicAuthV2] Parsed Prometheus config with %d scrape configs\n", len(prometheusConfig.ScrapeConfigs))

		var basicAuthJob *config.ScrapeConfig
		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
			if strings.Contains(sc.JobName, "basic-auth") {
				basicAuthJob = sc
			}
		}
		GinkgoWriter.Printf("[BasicAuthV2] All jobs: %v\n", jobNames)
		Expect(basicAuthJob).NotTo(BeNil(), "Expected a scrape job containing 'basic-auth' in its name, found jobs: %v", jobNames)
		GinkgoWriter.Printf("[BasicAuthV2] Found basic-auth job: %s\n", basicAuthJob.JobName)

		GinkgoWriter.Printf("[BasicAuthV2] Step 4: Verifying basic_auth credentials in scrape config...\n")
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth).NotTo(BeNil(), "basic-auth job should have basic_auth configured")
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth.Username).To(Equal("admin"), "basic-auth job should have username=admin")
		GinkgoWriter.Printf("[BasicAuthV2] Verified basic_auth username=admin\n")

		// --- Targets check: verify target is up ---
		GinkgoWriter.Printf("[BasicAuthV2] Step 5: Waiting 60s for first scrape interval to complete...\n")
		time.Sleep(60 * time.Second)

		GinkgoWriter.Printf("[BasicAuthV2] Querying targets from both RS pods...\n")
		var targetsResponses = make([]*utils.APIResponse, 2)
		err = utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/targets", true, true, targetsResponses)
		Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("[BasicAuthV2] Successfully queried targets from RS pods\n")

		foundBasicAuthTarget := false
		for podIdx, targetsResponse := range targetsResponses {
			if targetsResponse == nil {
				GinkgoWriter.Printf("[BasicAuthV2] RS pod %d: no response (nil), skipping\n", podIdx)
				continue
			}
			Expect(targetsResponse.Data).NotTo(BeNil())

			var targetsResult v1.TargetsResult
			Expect(json.Unmarshal([]byte(targetsResponse.Data), &targetsResult)).To(Succeed())
			Expect(targetsResult.Active).NotTo(BeNil())
			GinkgoWriter.Printf("[BasicAuthV2] RS pod %d: found %d active targets\n", podIdx, len(targetsResult.Active))

			for _, target := range targetsResult.Active {
				if strings.Contains(string(target.ScrapePool), "basic-auth") {
					foundBasicAuthTarget = true
					GinkgoWriter.Printf("[BasicAuthV2] Found target in RS pod %d scrape pool: %s, health: %s, lastError: %s\n", podIdx, target.ScrapePool, target.Health, target.LastError)
					Expect(target.Health).To(Equal(v1.HealthGood), "basic-auth target should be healthy (up)")
					break
				}
			}
			if foundBasicAuthTarget {
				break
			}
			GinkgoWriter.Printf("[BasicAuthV2] RS pod %d: no basic-auth target found in active targets\n", podIdx)
		}
		Expect(foundBasicAuthTarget).To(BeTrue(), "Expected to find an active target in a basic-auth scrape pool in at least one RS pod")

		GinkgoWriter.Printf("[BasicAuthV2] All checks passed\n")
	})
})

// Basic auth secret update with v2 configmap — verify updated username is reflected in config
var _ = Describe("Basic auth secret update (v2)", Label(utils.ConfigProcessingBasicAuthSecretUpdateV2), func() {
	It("should reflect the updated username in the Prometheus scrape config with v2 configmap", func() {
		GinkgoWriter.Printf("[BasicAuthSecretUpdateV2] Starting test — waiting 120s for config reload...\n")
		time.Sleep(120 * time.Second)

		// Query config from ama-metrics RS
		GinkgoWriter.Printf("[BasicAuthSecretUpdateV2] Querying Prometheus config...\n")
		var apiResponse utils.APIResponse
		err := utils.QueryPromUIFromPod(K8sClient, Cfg, "kube-system", "rsName", "ama-metrics", "prometheus-collector", "/api/v1/status/config", true, &apiResponse)
		Expect(err).NotTo(HaveOccurred())
		Expect(apiResponse.Data).NotTo(BeNil())

		var prometheusConfigResult v1.ConfigResult
		Expect(json.Unmarshal([]byte(apiResponse.Data), &prometheusConfigResult)).To(Succeed())
		Expect(prometheusConfigResult.YAML).NotTo(BeEmpty())

		prometheusConfig, err := config.Load(prometheusConfigResult.YAML, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(prometheusConfig).NotTo(BeNil())
		Expect(prometheusConfig.ScrapeConfigs).NotTo(BeNil())

		var basicAuthJob *config.ScrapeConfig
		var jobNames []string
		for _, sc := range prometheusConfig.ScrapeConfigs {
			jobNames = append(jobNames, sc.JobName)
			if strings.Contains(sc.JobName, "basic-auth") {
				basicAuthJob = sc
			}
		}
		GinkgoWriter.Printf("[BasicAuthSecretUpdateV2] All jobs: %v\n", jobNames)
		Expect(basicAuthJob).NotTo(BeNil(), "Expected a scrape job containing 'basic-auth' in its name, found jobs: %v", jobNames)

		Expect(basicAuthJob.HTTPClientConfig.BasicAuth).NotTo(BeNil(), "basic-auth job should have basic_auth configured")
		GinkgoWriter.Printf("[BasicAuthSecretUpdateV2] basic_auth username=%s\n", basicAuthJob.HTTPClientConfig.BasicAuth.Username)
		Expect(basicAuthJob.HTTPClientConfig.BasicAuth.Username).To(Equal("newadmin"), "basic-auth job should have updated username=newadmin after secret update")

		GinkgoWriter.Printf("[BasicAuthSecretUpdateV2] Secret update verification passed\n")
	})
})

/*
 * When the settings configmap sets deny_fs_access_through_sms = true, the config-reader must write
 * deny_fs_access_through_sms: true into the TargetAllocator config so ServiceMonitor/PodMonitor
 * scrape configs that read local files (bearerTokenFile, ca/cert/key files) are dropped.
 */
var _ = Describe("deny_fs_access_through_sms setting", Label(utils.ConfigProcessingDenyFSAccess), func() {
	It("is written to the TargetAllocator config by the config-reader", func() {
		Eventually(func() (string, error) {
			pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
			if err != nil {
				return "", err
			}
			stdout, _, err := utils.ExecCmd(K8sClient, Cfg, pods[0].Name, "config-reader", "kube-system",
				[]string{"cat", "/ta-configuration/targetallocator.yaml"})
			return stdout, err
		}, 5*time.Minute, 15*time.Second).Should(ContainSubstring("deny_fs_access_through_sms: true"))
	})
})

var _ = Describe("Customer ConfigMap annotation selection",
	Label("config-processing-customer-settings-v1", "config-processing-customer-settings-v2"), func() {
		It("discovers the included annotated pod but never scrapes the excluded pod", func() {
			Eventually(func(g Gomega) {
				for _, ns := range []string{"ama-settings-included", "ama-settings-excluded"} {
					pod, err := K8sClient.CoreV1().Pods(ns).Get(context.Background(), "annotation-target", metav1.GetOptions{})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(string(pod.Status.Phase)).To(Equal("Running"), ns)
					g.Expect(pod.Status.PodIP).NotTo(BeEmpty(), ns)
				}
				responses, err := utils.QueryCollectorPromUI("/api/v1/targets")
				g.Expect(err).NotTo(HaveOccurred())
				included, excluded := false, false
				for _, response := range responses {
					var result struct {
						ActiveTargets []struct {
							ScrapePool string            `json:"scrapePool"`
							Labels     map[string]string `json:"labels"`
							Health     string            `json:"health"`
							LastScrape time.Time         `json:"lastScrape"`
						} `json:"activeTargets"`
					}
					g.Expect(json.Unmarshal(response.Data, &result)).To(Succeed())
					for _, target := range result.ActiveTargets {
						if target.ScrapePool != "kubernetes-pods" || target.Labels["kubernetes_pod_name"] != "annotation-target" {
							continue
						}
						switch target.Labels["kubernetes_namespace"] {
						case "ama-settings-included":
							included = included || target.Health == "up"
							g.Expect(target.LastScrape).To(BeTemporally(">", time.Now().Add(-5*time.Minute)))
						case "ama-settings-excluded":
							excluded = true
						}
					}
				}
				g.Expect(included).To(BeTrue(), "included annotation target must be discovered and healthy")
				g.Expect(excluded).To(BeFalse(), "excluded annotation target must not be scraped")
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		})
	})

var _ = Describe("Invalid custom scrape configuration with default targets disabled",
	Label(utils.ConfigProcessingNoDefaultTargetsInvalidCustomConfig), func() {
		It("falls back to empty_job and reports the validation failure", func() {
			type source struct {
				key, value string
				linux      bool
			}
			sources := []source{
				{"rsName", "ama-metrics", true},
				{"dsName", "ama-metrics-node", true},
			}
			if os.Getenv("EXPECT_WINDOWS_EXPORTER") == "true" {
				sources = append(sources, source{"dsName", "ama-metrics-win-node", false})
			}
			Eventually(func(g Gomega) {
				for _, source := range sources {
					pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", source.key, source.value)
					g.Expect(err).NotTo(HaveOccurred())
					responses := make([]*utils.APIResponse, len(pods))
					for i := range responses {
						responses[i] = &utils.APIResponse{}
					}
					g.Expect(utils.QueryPromUIFromPods(K8sClient, Cfg, "kube-system", source.key, source.value,
						"prometheus-collector", "/api/v1/status/config", source.linux, true, responses)).To(Succeed())
					for _, response := range responses {
						var result v1.ConfigResult
						g.Expect(json.Unmarshal(response.Data, &result)).To(Succeed())
						cfg, err := config.Load(result.YAML, nil)
						g.Expect(err).NotTo(HaveOccurred())
						var jobs []string
						for _, job := range cfg.ScrapeConfigs {
							jobs = append(jobs, job.JobName)
						}
						g.Expect(jobs).To(ConsistOf("empty_job"), source.value)
					}
				}
				pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", "ama-metrics-operator-targets")
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					logs, err := K8sClient.CoreV1().Pods("kube-system").GetLogs(pod.Name, &corev1.PodLogOptions{
						Container: "config-reader",
					}).DoRaw(context.Background())
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(string(logs)).To(ContainSubstring("Prometheus custom config validation failed"), pod.Name)
				}
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
		})
	})

var _ = Describe("Monitor filesystem access enforcement", Label("config-processing-scenario-deny-fs-access"), func() {
	BeforeEach(func() {
		Expect(os.Getenv("SCENARIO_RUN_ID")).To(MatchRegexp(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`))
	})
	It("drops ServiceMonitor file references while retaining a file-free control", func() {
		denied := []string{}
		for _, endpoint := range []map[string]interface{}{
			{"port": "ba-metrics", "bearerTokenFile": "/ama-e2e-nonexistent/token"},
			{"port": "ba-metrics", "tlsConfig": map[string]interface{}{"caFile": "/ama-e2e-nonexistent/ca"}},
			{"port": "ba-metrics", "tlsConfig": map[string]interface{}{
				"certFile": "/ama-e2e-nonexistent/cert", "keyFile": "/ama-e2e-nonexistent/key"}},
		} {
			denied = append(denied, createMonitor("servicemonitors", "default", endpoint))
		}
		allowed := createMonitor("servicemonitors", "default", map[string]interface{}{"port": "ba-metrics"})
		expectMonitorJobs([]string{allowed}, denied)
	})

	It("rejects unsupported PodMonitor file references at strict CRD admission", func() {
		client := utils.MonitorClient("podmonitors", "default")
		monitor, err := client.Create(context.Background(),
			utils.MonitorObject("podmonitors", "default", map[string]interface{}{
				"port": "ba-metrics", "bearerTokenFile": "/ama-e2e-nonexistent/token",
			}, scenarioRunLabels()), metav1.CreateOptions{FieldValidation: "Strict"})
		if err == nil {
			DeferCleanup(func() {
				Expect(client.Delete(context.Background(), monitor.GetName(), metav1.DeleteOptions{})).To(Succeed())
			})
		}
		Expect(apierrors.IsBadRequest(err) || apierrors.IsInvalid(err)).To(BeTrue(), "PodMonitor schema must reject, not silently prune, unsupported local-file fields: %v", err)
		Expect(err.Error()).To(ContainSubstring("bearerTokenFile"))
	})
})

var _ = Describe("Monitor secret namespace enforcement", func() {
	It("does not watch secret updates with an empty namespace allowlist, but watches an allowlisted namespace",
		Label("config-processing-empty-secret-namespaces"), func() {
			version, err := K8sClient.Discovery().ServerVersion()
			Expect(err).NotTo(HaveOccurred())
			parsed, err := utilversion.ParseSemantic(version.GitVersion)
			Expect(err).NotTo(HaveOccurred())
			if !parsed.AtLeast(utilversion.MustParseSemantic("v1.37.0")) {
				Skip("scoped secret watching is supported on Kubernetes 1.37+; use the current auto-upgrading lane")
			}
			namespace := "ama-secret-watch-test"
			secretName := "settings-auth-creds"
			for _, verb := range []string{"get", "list", "watch"} {
				review, err := K8sClient.AuthorizationV1().SubjectAccessReviews().Create(context.Background(),
					&authv1.SubjectAccessReview{Spec: authv1.SubjectAccessReviewSpec{
						User:   "system:serviceaccount:kube-system:ama-metrics-serviceaccount",
						Groups: []string{"system:serviceaccounts", "system:serviceaccounts:kube-system", "system:authenticated"},
						ResourceAttributes: &authv1.ResourceAttributes{
							Namespace: namespace, Verb: verb, Resource: "secrets", Name: secretName,
						},
					}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(review.Status.Allowed).To(BeTrue(), "unexpected %s secret RBAC in %s", verb, namespace)
			}
			cm, err := K8sClient.CoreV1().ConfigMaps("kube-system").Get(context.Background(),
				"ama-metrics-settings-configmap", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			match := regexp.MustCompile(`(?m)^\s*secrets_access_namespaces\s*=\s*"([^"]*)"`).
				FindStringSubmatch(cm.Data["prometheus-collector-settings"])
			namespaces := ""
			if len(match) > 1 {
				namespaces = strings.TrimSpace(match[1])
			}
			Expect(namespaces).To(BeEmpty(), "empty-allowlist fixture required")
			secret, err := K8sClient.CoreV1().Secrets(namespace).Create(context.Background(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName},
				StringData: map[string]string{"username": "synthetic-user", "password": "synthetic-not-a-credential"},
			}, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			allowed := []string{}
			secretJobs := []string{}
			for _, resource := range []string{"servicemonitors", "podmonitors"} {
				secretJob := createMonitor(resource, namespace, map[string]interface{}{
					"port": "ba-metrics", "path": "/httpsmetrics", "basicAuth": map[string]interface{}{
						"username": map[string]interface{}{"name": secret.Name, "key": "username"},
						"password": map[string]interface{}{"name": secret.Name, "key": "password"},
					},
				})
				allowed = append(allowed, secretJob)
				secretJobs = append(secretJobs, secretJob+"/0")
				allowed = append(allowed, createMonitor(resource, namespace, map[string]interface{}{"port": "ba-metrics"}))
			}
			expectMonitorJobs(allowed, nil)
			Eventually(func() error { return monitorTransitionState(secretJobs, "synthetic-user", false) },
				5*time.Minute, 15*time.Second).Should(Succeed())
			instance, err := utils.AllocatorInstance()
			Expect(err).NotTo(HaveOccurred())
			secret.StringData = nil
			secret.Data = map[string][]byte{"username": []byte("synthetic-rotated"), "password": []byte("synthetic-not-a-credential")}
			_, err = K8sClient.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Consistently(func() error {
				current, err := utils.AllocatorInstance()
				if err != nil {
					return err
				}
				if current != instance {
					return fmt.Errorf("target allocator restarted during watch observation")
				}
				return monitorTransitionState(secretJobs, "synthetic-user", false)
			}, 5*time.Minute, 15*time.Second).Should(Succeed())

			utils.RestoreSettingsAfterTransition()
			cm.Data["prometheus-collector-settings"] = regexp.MustCompile(`(?m)^\s*secrets_access_namespaces\s*=.*\n?`).
				ReplaceAllString(cm.Data["prometheus-collector-settings"], "")
			cm.Data["prometheus-collector-settings"] += "\nsecrets_access_namespaces = \"" + namespace + "\"\n"
			utils.ReplaceSettings(cm.Data)
			Eventually(func() error {
				current, err := utils.AllocatorInstance()
				if err != nil {
					return err
				}
				if current == instance {
					return fmt.Errorf("target allocator has not reloaded the allowlist")
				}
				return monitorTransitionState(secretJobs, "synthetic-rotated", false)
			}, 8*time.Minute, 15*time.Second).Should(Succeed())
			instance, err = utils.AllocatorInstance()
			Expect(err).NotTo(HaveOccurred())
			Eventually(utils.AllocatorSecretWatchReady, 5*time.Minute, 15*time.Second).Should(Succeed())
			secret, err = K8sClient.CoreV1().Secrets(namespace).Get(context.Background(), secret.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			secret.Data["username"] = []byte("synthetic-allowed")
			_, err = K8sClient.CoreV1().Secrets(namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() error {
				current, err := utils.AllocatorInstance()
				if err != nil {
					return err
				}
				if current != instance {
					return fmt.Errorf("target allocator restarted during positive watch control")
				}
				return monitorTransitionState(secretJobs, "synthetic-allowed", false)
			}, 5*time.Minute, 15*time.Second).Should(Succeed())
		})
})

func scenarioRunLabels() map[string]interface{} {
	runID := os.Getenv("SCENARIO_RUN_ID")
	if runID == "" {
		return nil
	}
	return map[string]interface{}{"ama-metrics.azure.com/scenario-run": runID}
}

func createMonitor(resource, namespace string, endpoint map[string]interface{}) string {
	return utils.CreateMonitor(resource, namespace, endpoint, scenarioRunLabels())
}

var _ = Describe("Customer alias and annotation transitions", Serial, func() {
	for _, variant := range []struct{ schema, label string }{
		{"v1", utils.ConfigProcessingCustomerTransitionsV1},
		{"v2", utils.ConfigProcessingCustomerTransitionsV2},
	} {
		schema := variant.schema
		It("changes and clears alias, then updates and removes annotation namespace selection in "+schema,
			Label(variant.label), func() {
				utils.RestoreSettingsAfterTransition()
				api, fallback := utils.TransitionAPI(), utils.TransitionCluster()
				for _, phase := range []struct{ file, cluster, namespace string }{
					{"initial.yaml", "Customer_Config_POC", "ama-settings-included"},
					{"alias-updated.yaml", "Customer_Config_Updated", "ama-settings-included"},
					{"alias-cleared.yaml", fallback, "ama-settings-included"},
					{"namespace-updated.yaml", fallback, "ama-settings-excluded"},
					{"namespace-invalid.yaml", fallback, ""},
					{"namespace-removed.yaml", fallback, ""},
				} {
					changed := utils.SettingsChanged("customer-transitions-"+schema, phase.file)
					started := utils.SettingsPhase("customer-transitions-"+schema, phase.file)
					since := time.Time{}
					if changed {
						since = started
					}
					if phase.file == "namespace-invalid.yaml" {
						expectInvalidAnnotationRegexReported(started)
					}
					utils.WaitForCollectorsSettled(since)
					Eventually(func() error {
						targets, err := utils.TransitionTargets()
						if err != nil {
							return err
						}
						return utils.ValidateTransitionTargets(targets, []string{
							"podMonitor/ama-settings-included/settings-alias/0",
							"serviceMonitor/ama-settings-included/settings-alias/0",
						}, "up", started)
					}, 8*time.Minute, 15*time.Second).Should(Succeed())
					for _, job := range []string{"ama-settings-included/settings-alias", "settings-monitor-target"} {
						utils.ExpectTransitionMetric(api, "weather_temperature", job, phase.cluster, "", started)
						utils.ExpectTransitionMetric(api, "up", job, phase.cluster, "", started)
					}
					utils.ExpectWindowsKubeProxy(api, phase.cluster, started)
					expected := annotationExpectation{}
					if phase.namespace != "" {
						expected = annotationExpectation{Regex: "^" + phase.namespace + "$", Namespaces: []string{phase.namespace}}
					}
					expectAnnotationNamespaces(api, phase.cluster, expected, started)
				}
			})
	}
})

// expectInvalidAnnotationRegexReported checks that an invalid annotation regex is reported after the
// normal fresh-process reload of both the collector and the config-reader.
func expectInvalidAnnotationRegexReported(started time.Time) {
	Eventually(func(g Gomega) {
		for _, source := range []struct{ label, container string }{
			{"ama-metrics", "prometheus-collector"}, {"ama-metrics-operator-targets", "config-reader"},
		} {
			pods, err := utils.GetPodsWithLabel(K8sClient, "kube-system", "rsName", source.label)
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				startedAt := time.Time{}
				for _, status := range pod.Status.ContainerStatuses {
					if status.Name == source.container && status.Ready && status.State.Running != nil {
						startedAt = status.State.Running.StartedAt.Time
					}
				}
				g.Expect(startedAt.After(started)).To(BeTrue(),
					"%s/%s must reload through its normal fresh-process lifecycle", pod.Name, source.container)
				logs, err := K8sClient.CoreV1().Pods("kube-system").GetLogs(pod.Name, &corev1.PodLogOptions{
					Container: source.container, SinceTime: &metav1.Time{Time: started},
				}).DoRaw(context.Background())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(strings.Contains(string(logs), "Invalid namespace regex for pod annotations: [")).To(BeTrue(),
					"%s/%s must report the invalid annotation regex", pod.Name, source.container)
			}
		}
	}, 8*time.Minute, 15*time.Second).Should(Succeed())
}

var _ = Describe("Credential reload and allowlist transitions, not valid-password rotation", Serial, func() {
	for _, variant := range []struct{ schema, label string }{
		{"v1", utils.ConfigProcessingSecretTransitionsV1},
		{"v2", utils.ConfigProcessingSecretTransitionsV2},
	} {
		schema := variant.schema
		Describe(schema, Ordered, Label(variant.label), func() {
			var api v1.API
			var cluster string
			jobs := []string{"podMonitor/ama-settings-auth/settings-auth/0", "serviceMonitor/ama-settings-auth/settings-auth/0"}
			metrics := []string{"ama-settings-auth/settings-auth", "settings-auth-target"}
			BeforeAll(func() {
				utils.RestoreSettingsAfterTransition()
				secret, err := K8sClient.CoreV1().Secrets("ama-settings-auth").Get(context.Background(),
					"settings-auth-creds", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				original := secret.DeepCopy().Data
				DeferCleanup(func() { utils.ReplaceAuthSecret(original) })
				api, cluster = utils.TransitionAPI(), utils.TransitionCluster()
				AddReportEntry("true-credential-rotation", "deferred by user; only invalid-to-valid auth recovery is exercised")
			})
			It("reloads the dummy Secret from rejected credentials to authenticated fresh ingestion without a restart", func() {
				utils.WaitForCollectorsSettled(time.Time{})
				started, restarts := time.Now(), utils.CollectorRestarts()
				Eventually(func() error {
					return monitorTransitionState(jobs, "deliberately-invalid-test-user", false)
				},
					8*time.Minute, 15*time.Second).Should(Succeed())
				Eventually(func() error {
					targets, err := utils.TransitionTargets()
					if err != nil {
						return err
					}
					return utils.ValidateTransitionTargets(targets, jobs, "down", started)
				}, 8*time.Minute, 15*time.Second).Should(Succeed())
				expectAuthRecovered(api, cluster, jobs, metrics, utils.RecoverAuthSecret())
				Expect(utils.CollectorRestarts()).To(Equal(restarts), "Secret reload must not restart collectors, allocator or endpoint")
			})
			It("stops watching Secret updates for empty and omitted allowlists, then resumes watching", func() {
				version, err := K8sClient.Discovery().ServerVersion()
				Expect(err).NotTo(HaveOccurred())
				parsed, err := utilversion.ParseSemantic(version.GitVersion)
				Expect(err).NotTo(HaveOccurred())
				if !parsed.AtLeast(utilversion.MustParseSemantic("v1.37.0")) {
					Skip("unsupported below Kubernetes1.37; auth recovery passed separately, scoped enforcement is not covered")
				}
				for _, phase := range []string{"empty.yaml", "omitted.yaml"} {
					instance, err := utils.AllocatorInstance()
					Expect(err).NotTo(HaveOccurred())
					settingsStarted := utils.SettingsPhase("secret-transitions-"+schema, phase)
					Eventually(func() error {
						current, err := utils.AllocatorInstance()
						if err != nil {
							return err
						}
						if current == instance {
							return fmt.Errorf("target allocator has not reloaded the settings")
						}
						return monitorTransitionState(jobs, "admin", false)
					}, 8*time.Minute, 15*time.Second).Should(Succeed())
					utils.WaitForCollectorsSettled(settingsStarted)
					instance, err = utils.AllocatorInstance()
					Expect(err).NotTo(HaveOccurred())
					utils.ReplaceAuthSecret(map[string][]byte{"username": []byte("deliberately-invalid-test-user"), "password": []byte("pwd")})
					started := time.Now()
					Consistently(func() error {
						current, err := utils.AllocatorInstance()
						if err != nil {
							return err
						}
						if current != instance {
							return fmt.Errorf("target allocator restarted during watch observation")
						}
						return monitorTransitionState(jobs, "admin", false)
					}, 5*time.Minute, 15*time.Second).Should(Succeed())
					for _, job := range metrics {
						utils.ExpectTransitionMetric(api, "my_metric", job, cluster, "", started)
					}
					utils.SettingsPhase("secret-transitions-"+schema, "allowed.yaml")
					Eventually(func() error {
						current, err := utils.AllocatorInstance()
						if err != nil {
							return err
						}
						if current == instance {
							return fmt.Errorf("target allocator has not restored the allowlist")
						}
						return monitorTransitionState(jobs, "deliberately-invalid-test-user", false)
					}, 8*time.Minute, 15*time.Second).Should(Succeed())
					Eventually(utils.AllocatorSecretWatchReady, 5*time.Minute, 15*time.Second).Should(Succeed())
					expectAuthRecovered(api, cluster, jobs, metrics, utils.RecoverAuthSecret())
				}
			})
		})
	}
})

var _ = Describe("Dedicated Windows kube-proxy ingestion", Label(utils.ConfigProcessingWindowsKubeProxy), func() {
	It("observes the actual node endpoint and fresh healthy per-node AMW samples", func() {
		utils.ExpectWindowsKubeProxy(utils.TransitionAPI(), utils.TransitionCluster(), time.Now())
	})
})
