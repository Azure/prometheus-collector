package configmapsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var testPackageDir string

/*
 * For each type of ama-metrics pod, (Linux ReplicaSet, Linux DaemonSet, Windows DaemonSet):
 * 1) Test that the settings from the configmaps are correctly parsed and set to environment variables.
 * 2) Test that the Prometheus config created is as expected for the settings given.
 */
var _ = Describe("Configmapparser", Ordered, func() {
	AfterEach(func() {
		cleanupEnvVars()
	})

	It("isolates writable default templates", func() {
		root, err := filepath.Abs(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		pkg := filepath.Join(root, "shared", "configmap", "mp")
		defaults := filepath.Join(root, "configmapparser", "default-prom-configs")
		Expect(os.MkdirAll(filepath.Join(pkg, "testdata"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(defaults, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(pkg, "testdata", "collector-config-replicaset.yml"), []byte("{}"), 0o644)).To(Succeed())
		source := filepath.Join(defaults, "ztunnelDefault.yml")
		original := []byte("scrape_configs:\n  - job_name: ztunnel\n    scrape_interval: 30s\n")
		Expect(os.WriteFile(source, original, 0o644)).To(Succeed())

		workingDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(func() { Expect(os.Chdir(workingDir)).To(Succeed()) })
		Expect(os.Chdir(pkg)).To(Succeed())
		Expect(os.Mkdir("relative-temp", 0o755)).To(Succeed())
		GinkgoT().Setenv("TMPDIR", "relative-temp")
		oldPrefix, oldCollector := defaultPromConfigPathPrefix, replicaSetCollectorConfig
		oldRegex, oldInterval, oldMerged := regexHash, intervalHash, mergedDefaultConfigs
		DeferCleanup(func() {
			regexHash, intervalHash, mergedDefaultConfigs = oldRegex, oldInterval, oldMerged
		})
		regexHash = map[string]string{"original": "regex"}
		intervalHash = map[string]string{"original": "interval"}
		mergedDefaultConfigs = map[interface{}]interface{}{"original": "config"}
		DeferCleanup(func() {
			Expect(defaultPromConfigPathPrefix).To(Equal(oldPrefix))
			Expect(replicaSetCollectorConfig).To(Equal(oldCollector))
			Expect(regexHash).To(Equal(map[string]string{"original": "regex"}))
			Expect(intervalHash).To(Equal(map[string]string{"original": "interval"}))
			Expect(mergedDefaultConfigs).To(Equal(map[interface{}]interface{}{"original": "config"}))
		})
		setupProcessedFiles()
		Expect(regexHash).To(BeEmpty())
		Expect(intervalHash).To(BeEmpty())
		Expect(mergedDefaultConfigs).To(BeNil())
		regexHash["changed"] = "regex"
		intervalHash["changed"] = "interval"
		mergedDefaultConfigs = map[interface{}]interface{}{"changed": "config"}
		copied := filepath.Join(defaultPromConfigPathPrefix, "ztunnelDefault.yml")
		UpdateScrapeIntervalConfig(copied, "1m")
		updated, err := os.ReadFile(copied)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(updated)).To(ContainSubstring("scrape_interval: 1m"))
		unchanged, err := os.ReadFile(source)
		Expect(err).NotTo(HaveOccurred())
		Expect(unchanged).To(Equal(original))
	})

	Context("when the settings configmap does not exist", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "ConfigReaderSidecar",
				"CONTROLLER_TYPE": "ReplicaSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})
			setupConfigFiles(true)
			setupProcessedFiles()

			expectedContentsFilePath := filepath.Join(testPackageDir, "testdata", "default-linux-rs.yaml")

			Configmapparser()

			envVars := map[string]string {
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":   "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "",
				"AZMON_CLUSTER_ALIAS":                              "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":              "false",
				"AZMON_OPERATOR_ENABLED":                            "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":            "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":         "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				"DEBUG_MODE_ENABLED": "",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT": "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG": "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityCiliumRegex_minimal_mac),
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorCapacityProvisionerRegex_minimal_mac),
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorMetricsExporter_minimal_mac),
				"LOCALCSIDRIVER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",localCsiDriver_minimal_mac),
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",ztunnel_minimal_mac),
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",istioCni_minimal_mac),
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",dcgmexporter_minimal_mac),
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX": fmt.Sprintf("|%s",controlplaneIstio_minimal_mac),
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string {
				"KUBELET_SCRAPE_INTERVAL": "30s",
				"COREDNS_SCRAPE_INTERVAL": "30s",
				"CADVISOR_SCRAPE_INTERVAL": "30s",
				"KUBEPROXY_SCRAPE_INTERVAL": "30s",
				"APISERVER_SCRAPE_INTERVAL": "30s",
				"KUBESTATE_SCRAPE_INTERVAL": "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL": "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL": "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL": "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL": "30s",
				"ZTUNNEL_SCRAPE_INTERVAL": "30s",
				"ISTIOCNI_SCRAPE_INTERVAL": "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL": "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL": "30s",
			})

			mergedFileContents, err := ioutil.ReadFile(mergedDefaultConfigPath)
			Expect(err).NotTo(HaveOccurred())
			expectedFileContents, err := ioutil.ReadFile(expectedContentsFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mergedFileContents)).To(Equal(string(expectedFileContents)))
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "",
				"CONTROLLER_TYPE": "DaemonSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})
			setupConfigFiles(true)
			setupProcessedFiles()
			expectedContentsFilePath := filepath.Join(testPackageDir, "testdata", "default-linux-ds.yaml")

			Configmapparser()

			envVars := map[string]string {
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":   "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "",
				"AZMON_CLUSTER_ALIAS":                              "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":              "false",
				"AZMON_OPERATOR_ENABLED":                            "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":            "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":         "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				"DEBUG_MODE_ENABLED": "",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT": "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG": "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityCiliumRegex_minimal_mac),
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorCapacityProvisionerRegex_minimal_mac),
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorMetricsExporter_minimal_mac),
				"LOCALCSIDRIVER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",localCsiDriver_minimal_mac),
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",ztunnel_minimal_mac),
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",istioCni_minimal_mac),
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",dcgmexporter_minimal_mac),
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX": fmt.Sprintf("|%s",controlplaneIstio_minimal_mac),
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string {
				"KUBELET_SCRAPE_INTERVAL": "30s",
				"COREDNS_SCRAPE_INTERVAL": "30s",
				"CADVISOR_SCRAPE_INTERVAL": "30s",
				"KUBEPROXY_SCRAPE_INTERVAL": "30s",
				"APISERVER_SCRAPE_INTERVAL": "30s",
				"KUBESTATE_SCRAPE_INTERVAL": "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL": "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL": "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL": "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL": "30s",
				"ZTUNNEL_SCRAPE_INTERVAL": "30s",
				"ISTIOCNI_SCRAPE_INTERVAL": "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL": "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL": "30s",
			})

			mergedFileContents, err := ioutil.ReadFile(mergedDefaultConfigPath)
			Expect(err).NotTo(HaveOccurred())
			expectedFileContents, err := ioutil.ReadFile(expectedContentsFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mergedFileContents)).To(Equal(string(expectedFileContents)))
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
		})
	})


	Context("when the settings configmap sections exist but are empty", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "ConfigReaderSidecar",
				"CONTROLLER_TYPE": "ReplicaSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})
			setupConfigFiles(false)
			setupProcessedFiles()

			fmt.Println("testing replicaset defaults empty")

			Configmapparser()

			envVars := map[string]string {
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":   "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "",
				"AZMON_CLUSTER_ALIAS":                              "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":              "true",
				"AZMON_OPERATOR_ENABLED":                            "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":            "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":         "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":       "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				"DEBUG_MODE_ENABLED": "false",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT": "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG": "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())
			
			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",networkobservabilityCiliumRegex_minimal_mac),
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorCapacityProvisionerRegex_minimal_mac),
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorMetricsExporter_minimal_mac),
				"LOCALCSIDRIVER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",localCsiDriver_minimal_mac),
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",ztunnel_minimal_mac),
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",istioCni_minimal_mac),
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",dcgmexporter_minimal_mac),
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX": fmt.Sprintf("|%s",controlplaneIstio_minimal_mac),
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string {
				"KUBELET_SCRAPE_INTERVAL": "30s",
				"COREDNS_SCRAPE_INTERVAL": "30s",
				"CADVISOR_SCRAPE_INTERVAL": "30s",
				"KUBEPROXY_SCRAPE_INTERVAL": "30s",
				"APISERVER_SCRAPE_INTERVAL": "30s",
				"KUBESTATE_SCRAPE_INTERVAL": "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL": "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL": "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL": "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL": "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL": "30s",
				"ZTUNNEL_SCRAPE_INTERVAL": "30s",
				"ISTIOCNI_SCRAPE_INTERVAL": "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL": "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL": "30s",
			})

			mergedFileContents, err := ioutil.ReadFile(mergedDefaultConfigPath)
			Expect(err).NotTo(HaveOccurred())
			expectedFileContents, err := ioutil.ReadFile(filepath.Join(testPackageDir, "testdata", "default-linux-rs.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mergedFileContents)).To(Equal(string(expectedFileContents)))
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {

		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "ConfigReaderSidecar",
				"CONTROLLER_TYPE": "ReplicaSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})

			schemaVersionFile = createTempFile("schema-version", "v1")
			configVersionFile = createTempFile("config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile("podannotation", `podannotationnamespaceregex = ".*|value"`)
			collectorSettingsMountPath = createTempFile("collector-settings", `cluster_alias = "alias"`)
			defaultSettingsMountPath = createTempFile("default-settings", `
				kubelet = true
				coredns = true
				cadvisor = true
				kubeproxy = true
				apiserver = true
				kubestate = true
				nodeexporter = true
				windowsexporter = true
				windowskubeproxy = true
				kappiebasic = true
				networkobservabilityRetina = true
				networkobservabilityHubble = true
				networkobservabilityCilium = true
				prometheuscollectorhealth = true
			`)
			configMapDebugMountPath = createTempFile("debug-mode", `enabled = true`)
			configMapKeepListMountPath = createTempFile("keep-list", `
				kubelet = "test.*|test2"
				coredns = "test.*|test2"
				cadvisor = "test.*|test2"
				kubeproxy = "test.*|test2"
				apiserver = "test.*|test2"
				kubestate = "test.*|test2"
				nodeexporter = "test.*|test2"
				windowsexporter = "test.*|test2"
				windowskubeproxy = "test.*|test2"
				podannotations = "test.*|test2"
				kappiebasic = "test.*|test2"
				networkobservabilityRetina = "test.*|test2"
				networkobservabilityHubble = "test.*|test2"
				networkobservabilityCilium = "test.*|test2"
				minimalingestionprofile = true
			`)
			configMapScrapeIntervalMountPath = createTempFile("scrape-interval", `
				kubelet = "15s"
				coredns = "15s"
				cadvisor = "15s"
				kubeproxy = "15s"
				apiserver = "15s"
				kubestate = "15s"
				nodeexporter = "15s"
				windowsexporter = "15s"
				windowskubeproxy = "15s"
				kappiebasic = "15s"
				networkobservabilityRetina = "15s"
				networkobservabilityHubble = "15s"
				networkobservabilityCilium = "15s"
				prometheuscollectorhealth = "15s"
				podannotations = "15s"
			`)

			setupProcessedFiles()

			Configmapparser()

			envVars := map[string]string {
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":   "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": ".*|value",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":              "true",
				"AZMON_OPERATOR_ENABLED":                            "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":            "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "true",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "true",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				//"DEBUG_MODE_ENABLED": "true",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT": "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG": "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s",networkobservabilityCiliumRegex_minimal_mac),
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorCapacityProvisionerRegex_minimal_mac),
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",acstorMetricsExporter_minimal_mac),
				"LOCALCSIDRIVER_KEEP_LIST_REGEX": fmt.Sprintf("|%s",localCsiDriver_minimal_mac),
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",ztunnel_minimal_mac),
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",istioCni_minimal_mac),
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s",dcgmexporter_minimal_mac),
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX": fmt.Sprintf("|%s",controlplaneIstio_minimal_mac),
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string {
				"KUBELET_SCRAPE_INTERVAL": "15s",
				"COREDNS_SCRAPE_INTERVAL": "15s",
				"CADVISOR_SCRAPE_INTERVAL": "15s",
				"KUBEPROXY_SCRAPE_INTERVAL": "15s",
				"APISERVER_SCRAPE_INTERVAL": "15s",
				"KUBESTATE_SCRAPE_INTERVAL": "15s",
				"NODEEXPORTER_SCRAPE_INTERVAL": "15s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL": "15s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL": "15s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "15s",
				"POD_ANNOTATION_SCRAPE_INTERVAL": "15s",
				"KAPPIEBASIC_SCRAPE_INTERVAL": "15s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "15s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "15s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "15s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL": "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL": "30s",
				"ZTUNNEL_SCRAPE_INTERVAL": "30s",
				"ISTIOCNI_SCRAPE_INTERVAL": "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL": "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL": "30s",
			})
		})

		It("should process the config for the Linux Daemonset", func() {

		})

		It("should process the config for the Windows Daemonset", func() {

		})
	})

	Context("when minimal ingestion is not true", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should handle it being set to false with the keeplist regex values", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "ConfigReaderSidecar",
				"CONTROLLER_TYPE": "ReplicaSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})

			schemaVersionFile = createTempFile("schema-version", "v1")
			configVersionFile = createTempFile("config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile("podannotation", "")
			collectorSettingsMountPath = createTempFile("collector-settings", "")
			defaultSettingsMountPath = createTempFile("default-settings", "")
			configMapDebugMountPath = createTempFile("debug-mode", "")
			configMapKeepListMountPath = createTempFile("keep-list", `
				kubelet = "test.*|test2"
				coredns = "test.*|test2"
				cadvisor = "test.*|test2"
				kubeproxy = "test.*|test2"
				apiserver = "test.*|test2"
				kubestate = "test.*|test2"
				nodeexporter = "test.*|test2"
				windowsexporter = "test.*|test2"
				windowskubeproxy = "test.*|test2"
				podannotations = "test.*|test2"
				kappiebasic = "test.*|test2"
				networkobservabilityRetina = "test.*|test2"
				networkobservabilityHubble = "test.*|test2"
				networkobservabilityCilium = "test.*|test2"
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = createTempFile("scrape-interval", ``)

			setupProcessedFiles()

			Configmapparser()

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"COREDNS_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"CADVISOR_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"APISERVER_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "",
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "",
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "",
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "",
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "",
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "",
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "",
			})
		})

		It("should handle it being set to false with no keeplist regex values", func() {
			setEnvVars(map[string]string {
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE": "ConfigReaderSidecar",
				"CONTROLLER_TYPE": "ReplicaSet",
				"OS_TYPE": "linux",
				"MODE": "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE": "kube-system",
				"MAC": "true",
			})

			schemaVersionFile = createTempFile("schema-version", "v1")
			configVersionFile = createTempFile("config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile("podannotation", "")
			collectorSettingsMountPath = createTempFile("collector-settings", "")
			defaultSettingsMountPath = createTempFile("default-settings", "")
			configMapDebugMountPath = createTempFile("debug-mode", "")
			configMapKeepListMountPath = createTempFile("keep-list", `
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = createTempFile("scrape-interval", ``)

			setupProcessedFiles()

			Configmapparser()

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string {
				"KUBELET_METRICS_KEEP_LIST_REGEX": "",
				"COREDNS_METRICS_KEEP_LIST_REGEX": "",
				"CADVISOR_METRICS_KEEP_LIST_REGEX": "",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX": "",
				"APISERVER_METRICS_KEEP_LIST_REGEX": "",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX": "",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX": "",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX": "",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX": "",
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX": "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX": "",
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": "",
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": "",
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "",
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "",
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "",
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "",
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "",
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "",
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "",
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "",
			})
		})
	})	
})

func createTempFile(name string, content string) string {
	return createTempFileIn("", name, content)
}

func createTempFileIn(dir, name, content string) string {
	tempFile, err := ioutil.TempFile(dir, name)
	Expect(err).NotTo(HaveOccurred())
	_, err = tempFile.WriteString(content)
	Expect(err).NotTo(HaveOccurred())
	Expect(tempFile.Close()).To(Succeed())
	path, err := filepath.Abs(tempFile.Name())
	Expect(err).NotTo(HaveOccurred())
	return path
}

func checkEnvVars(envVars map[string]string) error {
	for key, value := range envVars {
		if os.Getenv(key) != value {
			return fmt.Errorf("Expected %s to be %s, but got %s", key, value, os.Getenv(key))
		}
	}
	return nil
}

func checkHashMaps(filepath string, expectedHash map[string]string) {
	regexFileContents, err := ioutil.ReadFile(filepath)
	Expect(err).NotTo(HaveOccurred())
	var hash map[string]string
	err = yaml.Unmarshal([]byte(regexFileContents), &hash)
	Expect(err).NotTo(HaveOccurred())
	Expect(hash).To(BeComparableTo(expectedHash))
}

func setEnvVars(envVars map[string]string) {
	for key, value := range envVars {
		os.Setenv(key, value)
	}
}

func setupConfigFiles(defaultPath bool) {
	if defaultPath {
		configMapDebugMountPath   = "/etc/config/settings/debug-mode"
		replicaSetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
		defaultSettingsMountPath = "/etc/config/settings/default-scrape-settings"
		configMapKeepListMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
		configMapMountPathForPodAnnotation = "/etc/config/settings/pod-annotation-based-scraping"
		collectorSettingsMountPath = "/etc/config/settings/prometheus-collector-settings"
		schemaVersionFile = "/etc/config/settings/schema-version"
		configVersionFile = "/etc/config/settings/config-version"
		configMapScrapeIntervalMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
	} else {
		schemaVersionFile = createTempFile("schema-version", "v1")
		configVersionFile = createTempFile("config-version", "ver1")
		configMapMountPathForPodAnnotation = createTempFile("podannotation", "")
		collectorSettingsMountPath = createTempFile("collector-settings", "")
		defaultSettingsMountPath = createTempFile("default-settings", "")
		configMapDebugMountPath = createTempFile("debug-mode", "")
		configMapKeepListMountPath = createTempFile("keep-list", "")
		configMapScrapeIntervalMountPath = createTempFile("scrape-interval", "")
		replicaSetCollectorConfig = "./testdata/collector-config-replicaset.yml"
	}
}

func setupProcessedFiles() {
	workingDir, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	tempDir, err := os.MkdirTemp("", "configmapparser-")
	Expect(err).NotTo(HaveOccurred())
	paths := []*string{&testPackageDir, &defaultPromConfigPathPrefix, &replicaSetCollectorConfig,
		&podAnnotationEnvVarPath, &collectorSettingsEnvVarPath, &defaultSettingsEnvVarPath,
		&debugModeEnvVarPath, &configMapKeepListEnvVarPath, &scrapeIntervalEnvVarPath,
		&opentelemetryMetricsEnvVarPath,
		&mergedDefaultConfigPath, &promMergedConfigPath, &configMapMountPath, &configMapSettingsDir,
		&regexHashFile, &intervalHashFile}
	saved := make([]string, len(paths))
	for i, path := range paths {
		saved[i] = *path
	}
	savedRegex, savedInterval, savedMerged := regexHash, intervalHash, mergedDefaultConfigs
	DeferCleanup(func() {
		for i, path := range paths {
			*path = saved[i]
		}
		regexHash, intervalHash, mergedDefaultConfigs = savedRegex, savedInterval, savedMerged
		Expect(os.Chdir(workingDir)).To(Succeed())
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})
	absoluteTempDir, err := filepath.Abs(tempDir)
	Expect(err).NotTo(HaveOccurred())
	tempDir = absoluteTempDir
	testPackageDir = workingDir
	regexHash = map[string]string{}
	intervalHash = map[string]string{}
	mergedDefaultConfigs = nil
	defaultsDir := filepath.Join(workingDir, "../../../configmapparser/default-prom-configs")
	Expect(copyDefaultConfigs(defaultsDir, tempDir)).To(Succeed())
	configMapSettingsDir = filepath.Join(tempDir, "settings")
	for _, setting := range []struct {
		path string
		name string
	}{
		{configMapMountPathForPodAnnotation, "pod-annotation-based-scraping"},
		{collectorSettingsMountPath, "prometheus-collector-settings"},
		{defaultSettingsMountPath, "default-scrape-settings-enabled"},
		{configMapDebugMountPath, "debug-mode"},
		{configMapKeepListMountPath, "default-targets-metrics-keep-list"},
		{configMapScrapeIntervalMountPath, "default-targets-scrape-interval-settings"},
	} {
		relative, err := filepath.Rel(os.TempDir(), setting.path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			continue
		}
		content, err := os.ReadFile(setting.path)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(configMapSettingsDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configMapSettingsDir, setting.name), content, 0o644)).To(Succeed())
	}
	configMapMountPath = filepath.Join(tempDir, "no-custom-config")
	promMergedConfigPath = filepath.Join(tempDir, "promMergedConfig.yml")
	sourceConfig, err := os.ReadFile(filepath.Join(testPackageDir, "testdata", "collector-config-replicaset.yml"))
	Expect(err).NotTo(HaveOccurred())
	replicaSetCollectorConfig = filepath.Join(tempDir, "collector-config-replicaset.yml")
	Expect(os.WriteFile(replicaSetCollectorConfig, sourceConfig, 0o644)).To(Succeed())
	Expect(os.Chdir(tempDir)).To(Succeed())
	podAnnotationEnvVarPath = createTempFileIn(tempDir, "podannotation-envvar", "")
	collectorSettingsEnvVarPath = createTempFileIn(tempDir, "collector-settings-envvar", "")
	defaultSettingsEnvVarPath = createTempFileIn(tempDir, "default-settings-envvar", "")
	debugModeEnvVarPath = createTempFileIn(tempDir, "debug-mode-envvar", "")
	configMapKeepListEnvVarPath = createTempFileIn(tempDir, "keep-list-envvar", "")
	scrapeIntervalEnvVarPath = createTempFileIn(tempDir, "scrape-interval-envvar", "")
	opentelemetryMetricsEnvVarPath = createTempFileIn(tempDir, "opentelemetry-metrics-envvar", "")

	mergedDefaultConfigPath = createTempFileIn(tempDir, "merged-default-config", "")
	regexHashFile = configMapKeepListEnvVarPath
	intervalHashFile = scrapeIntervalEnvVarPath
}

func copyDefaultConfigs(source, tempDir string) error {
	prefix := filepath.Join(tempDir, "default-prom-configs")
	if err := os.CopyFS(prefix, os.DirFS(source)); err != nil {
		return err
	}
	defaultPromConfigPathPrefix = prefix + string(os.PathSeparator)
	return nil
}

func cleanupEnvVars() {
	allEnvVars := []string {
		"CONTAINER_TYPE",
		"CONTROLLER_TYPE",
		"OS_TYPE",
		"MODE",
		"KUBE_STATE_NAME",
		"POD_NAMESPACE",
		"MAC",
		"AZMON_AGENT_CFG_SCHEMA_VERSION",
		"AZMON_AGENT_CFG_FILE_VERSION",
		"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX",
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME",
		"AZMON_CLUSTER_LABEL",
		"AZMON_CLUSTER_ALIAS",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING",
		"AZMON_OPERATOR_ENABLED",
		"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING",
		"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED",
		"DEBUG_MODE_ENABLED",
		"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG",
		"CONFIG_VALIDATOR_RUNNING_IN_AGENT",
		"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG",
	}
	for _, envVar := range allEnvVars {
		os.Unsetenv(envVar)
	}
}