package configmapsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var _ = Describe("Configmapparser", func() {
	var testdataDirectory string

	BeforeEach(func() {
		paths := []*string{&schemaVersionFile, &configVersionFile, &configMapDebugMountPath,
			&replicaSetCollectorConfig, &defaultSettingsMountPath, &configMapKeepListMountPath,
			&configMapMountPathForPodAnnotation, &collectorSettingsMountPath, &configMapScrapeIntervalMountPath,
			&podAnnotationEnvVarPath, &collectorSettingsEnvVarPath, &defaultSettingsEnvVarPath,
			&debugModeEnvVarPath, &configMapKeepListEnvVarPath, &scrapeIntervalEnvVarPath,
			&opentelemetryMetricsEnvVarPath, &ksmConfigEnvVarPath,
			&defaultPromConfigPathPrefix, &mergedDefaultConfigPath, &regexHashFile, &intervalHashFile}
		for _, path := range paths {
			original := *path
			DeferCleanup(func() { *path = original })
		}
		directory, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		testdataDirectory = filepath.Join(directory, "testdata")
		defaultPromConfigPathPrefix = filepath.Clean(filepath.Join(directory, "../../../configmapparser/default-prom-configs")) + string(os.PathSeparator)
		content, err := os.ReadFile(filepath.Join(testdataDirectory, "collector-config-replicaset.yml"))
		Expect(err).NotTo(HaveOccurred())
		replicaSetCollectorConfig = createTempFile("collector.yml", string(content))
		opentelemetryMetricsEnvVarPath = createTempFile("otel-env", "")
		ksmConfigEnvVarPath = createTempFile("ksm-env", "")
		workDirectory := GinkgoT().TempDir()
		DeferCleanup(func() { Expect(os.Chdir(directory)).To(Succeed()) })
		Expect(os.Chdir(workDirectory)).To(Succeed())
	})
	AfterEach(func() {
		cleanupEnvVars()
	})

	Context("when the settings configmap does not exist", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
			})
			setupConfigFiles(true)
			setupProcessedFiles()

			expectedContentsFilePath := filepath.Join(testdataDirectory, "default-linux-rs.yaml")

			mountSettingsFiles()
			Configmapparser()

			envVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                                 "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":             "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                            "",
				"AZMON_CLUSTER_LABEL":                                          "",
				"AZMON_CLUSTER_ALIAS":                                          "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":                         "false",
				"AZMON_OPERATOR_ENABLED":                                       "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":                       "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":                    "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":                    "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":                   "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":               "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":            "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED":                                           "",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   fmt.Sprintf("|%s", cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               fmt.Sprintf("|%s", nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            fmt.Sprintf("|%s", windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           fmt.Sprintf("|%s", windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                fmt.Sprintf("|%s", kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityCiliumRegex_minimal_mac,
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "|" + acstorCapacityProvisionerRegex_minimal_mac,
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "|" + acstorMetricsExporter_minimal_mac,
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "|" + localCsiDriver_minimal_mac,
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "|" + ztunnel_minimal_mac,
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "|" + istioCni_minimal_mac,
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + dcgmexporter_minimal_mac,
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "|" + controlplaneIstio_minimal_mac,
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string{
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                 "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL":  "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL":   "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL":       "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL":              "30s",
				"ZTUNNEL_SCRAPE_INTERVAL":                     "30s",
				"ISTIOCNI_SCRAPE_INTERVAL":                    "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL":                "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL":          "30s",
			})

			checkMergedConfig(expectedContentsFilePath)
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "",
				"CONTROLLER_TYPE":        "DaemonSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
			})
			setupConfigFiles(true)
			setupProcessedFiles()
			expectedContentsFilePath := filepath.Join(testdataDirectory, "default-linux-ds.yaml")

			mountSettingsFiles()
			Configmapparser()

			envVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                                 "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":             "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                            "",
				"AZMON_CLUSTER_LABEL":                                          "",
				"AZMON_CLUSTER_ALIAS":                                          "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":                         "false",
				"AZMON_OPERATOR_ENABLED":                                       "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":                       "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":                    "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":                    "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":                   "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":               "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":            "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED":                                           "",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   fmt.Sprintf("|%s", cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               fmt.Sprintf("|%s", nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            fmt.Sprintf("|%s", windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           fmt.Sprintf("|%s", windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                fmt.Sprintf("|%s", kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityCiliumRegex_minimal_mac,
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "|" + acstorCapacityProvisionerRegex_minimal_mac,
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "|" + acstorMetricsExporter_minimal_mac,
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "|" + localCsiDriver_minimal_mac,
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "|" + ztunnel_minimal_mac,
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "|" + istioCni_minimal_mac,
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + dcgmexporter_minimal_mac,
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "|" + controlplaneIstio_minimal_mac,
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string{
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                 "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL":  "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL":   "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL":       "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL":              "30s",
				"ZTUNNEL_SCRAPE_INTERVAL":                     "30s",
				"ISTIOCNI_SCRAPE_INTERVAL":                    "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL":                "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL":          "30s",
			})

			checkMergedConfig(expectedContentsFilePath)
		})

	})

	Context("when the settings configmap sections exist but are empty", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
			})
			setupConfigFiles(false)
			setupProcessedFiles()

			fmt.Println("testing replicaset defaults empty")

			mountSettingsFiles()
			Configmapparser()

			envVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                                 "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":             "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                            "",
				"AZMON_CLUSTER_LABEL":                                          "",
				"AZMON_CLUSTER_ALIAS":                                          "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":                         "true",
				"AZMON_OPERATOR_ENABLED":                                       "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":                       "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":                    "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":                    "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":                   "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":                  "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":               "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":            "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED":           "false",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED":                                           "false",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("|%s", coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   fmt.Sprintf("|%s", cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("|%s", kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               fmt.Sprintf("|%s", nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            fmt.Sprintf("|%s", windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           fmt.Sprintf("|%s", windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                fmt.Sprintf("|%s", kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("|%s", networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityCiliumRegex_minimal_mac,
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "|" + acstorCapacityProvisionerRegex_minimal_mac,
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "|" + acstorMetricsExporter_minimal_mac,
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "|" + localCsiDriver_minimal_mac,
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "|" + ztunnel_minimal_mac,
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "|" + istioCni_minimal_mac,
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + dcgmexporter_minimal_mac,
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "|" + controlplaneIstio_minimal_mac,
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string{
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                 "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL":  "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL":  "30s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL":   "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL":       "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL":              "30s",
				"ZTUNNEL_SCRAPE_INTERVAL":                     "30s",
				"ISTIOCNI_SCRAPE_INTERVAL":                    "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL":                "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL":          "30s",
			})

			checkMergedConfig(filepath.Join(testdataDirectory, "default-linux-rs.yaml"))
		})

	})

	Context("when the settings configmap sections exist and are not default", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
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

			mountSettingsFiles()
			Configmapparser()

			envVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                                 "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":             ".*|value",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                            "",
				"AZMON_CLUSTER_LABEL":                                          "alias",
				"AZMON_CLUSTER_ALIAS":                                          "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":                         "true",
				"AZMON_OPERATOR_ENABLED":                                       "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":                       "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":                    "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":                    "true",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":                   "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":                  "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":               "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "true",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":            "true",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED":           "true",
				"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED":                                           "true",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
			}
			err := checkEnvVars(envVars)
			Expect(err).NotTo(HaveOccurred())

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("test.*|test2|%s", kubeletRegex_minimal_mac),
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    fmt.Sprintf("test.*|test2|%s", coreDNSRegex_minimal_mac),
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   fmt.Sprintf("test.*|test2|%s", cadvisorRegex_minimal_mac),
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("test.*|test2|%s", kubeproxyRegex_minimal_mac),
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("test.*|test2|%s", apiserverRegex_minimal_mac),
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  fmt.Sprintf("test.*|test2|%s", kubestateRegex_minimal_mac),
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               fmt.Sprintf("test.*|test2|%s", nodeexporterRegex_minimal_mac),
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            fmt.Sprintf("test.*|test2|%s", windowsexporterRegex_minimal_mac),
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           fmt.Sprintf("test.*|test2|%s", windowskubeproxyRegex_minimal_mac),
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "test.*|test2",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                fmt.Sprintf("test.*|test2|%s", kappiebasicRegex_minimal_mac),
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s", networkobservabilityRetinaRegex_minimal_mac),
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": fmt.Sprintf("test.*|test2|%s", networkobservabilityHubbleRegex_minimal_mac),
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "test.*|test2|" + networkobservabilityCiliumRegex_minimal_mac,
				"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "|" + acstorCapacityProvisionerRegex_minimal_mac,
				"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "|" + acstorMetricsExporter_minimal_mac,
				"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "|" + localCsiDriver_minimal_mac,
				"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "|" + ztunnel_minimal_mac,
				"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "|" + istioCni_minimal_mac,
				"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + dcgmexporter_minimal_mac,
				"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "|" + controlplaneIstio_minimal_mac,
			})

			checkHashMaps(scrapeIntervalEnvVarPath, map[string]string{
				"KUBELET_SCRAPE_INTERVAL":                     "15s",
				"COREDNS_SCRAPE_INTERVAL":                     "15s",
				"CADVISOR_SCRAPE_INTERVAL":                    "15s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "15s",
				"APISERVER_SCRAPE_INTERVAL":                   "15s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "15s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "15s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "15s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "15s",
				"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "15s",
				"POD_ANNOTATION_SCRAPE_INTERVAL":              "15s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                 "15s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL":  "15s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL":  "15s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL":  "15s",
				"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL":   "30s",
				"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL":       "30s",
				"LOCALCSIDRIVER_SCRAPE_INTERVAL":              "30s",
				"ZTUNNEL_SCRAPE_INTERVAL":                     "30s",
				"ISTIOCNI_SCRAPE_INTERVAL":                    "30s",
				"DCGMEXPORTER_SCRAPE_INTERVAL":                "30s",
				"CONTROLPLANE_ISTIO_SCRAPE_INTERVAL":          "30s",
			})
		})

	})

	Context("when minimal ingestion is not true", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should handle it being set to false with the keeplist regex values", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
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

			mountSettingsFiles()
			Configmapparser()

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    "test.*|test2",
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    "test.*|test2",
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   "test.*|test2",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  "test.*|test2",
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  "test.*|test2",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  "test.*|test2",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               "test.*|test2",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            "test.*|test2",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           "test.*|test2",
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "test.*|test2",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                "test.*|test2",
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
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":                "linux",
				"MODE":                   "advanced",
				"KUBE_STATE_NAME":        "ama-metrics-ksm",
				"POD_NAMESPACE":          "kube-system",
				"MAC":                    "true",
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

			podAnnotationEnvVarPath = createTempFile("podannotation-envvar", "")
			collectorSettingsEnvVarPath = createTempFile("collector-settings-envvar", "")
			defaultSettingsEnvVarPath = createTempFile("default-settings-envvar", "")
			debugModeEnvVarPath = createTempFile("debug-mode-envvar", "")
			configMapKeepListEnvVarPath = createTempFile("keep-list-envvar", "")
			scrapeIntervalEnvVarPath = createTempFile("scrape-interval-envvar", "")

			mountSettingsFiles()
			Configmapparser()

			checkHashMaps(configMapKeepListEnvVarPath, map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                    "",
				"COREDNS_METRICS_KEEP_LIST_REGEX":                    "",
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                   "",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  "",
				"APISERVER_METRICS_KEEP_LIST_REGEX":                  "",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  "",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               "",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            "",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           "",
				"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                "",
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

func checkMergedConfig(expectedPath string) {
	actualBytes, err := os.ReadFile(mergedDefaultConfigPath)
	Expect(err).NotTo(HaveOccurred())
	expectedBytes, err := os.ReadFile(expectedPath)
	Expect(err).NotTo(HaveOccurred())
	var actual, expected map[string]interface{}
	Expect(yaml.Unmarshal(actualBytes, &actual)).To(Succeed())
	Expect(yaml.Unmarshal(expectedBytes, &expected)).To(Succeed())
	fmt.Fprintf(GinkgoWriter, "Actual merged default configuration:\n%s\n", actualBytes)
	Expect(actual).To(Equal(expected))
}

func createTempFile(name string, content string) string {
	path := filepath.Join(GinkgoT().TempDir(), name)
	Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	return path
}

func mountSettingsFiles() {
	directory := filepath.Join(GinkgoT().TempDir(), "settings")
	for name, source := range map[string]string{
		"schema-version": schemaVersionFile, "config-version": configVersionFile,
		"debug-mode": configMapDebugMountPath, "prometheus-collector-settings": collectorSettingsMountPath,
		"default-scrape-settings-enabled":          defaultSettingsMountPath,
		"default-targets-metrics-keep-list":        configMapKeepListMountPath,
		"pod-annotation-based-scraping":            configMapMountPathForPodAnnotation,
		"default-targets-scrape-interval-settings": configMapScrapeIntervalMountPath,
	} {
		content, err := os.ReadFile(source)
		if os.IsNotExist(err) {
			continue
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(directory, 0700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(directory, name), content, 0600)).To(Succeed())
	}
	schemaVersionFile = filepath.Join(directory, "schema-version")
	configVersionFile = filepath.Join(directory, "config-version")
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
		directory := GinkgoT().TempDir()
		configMapDebugMountPath = filepath.Join(directory, "debug-mode")
		defaultSettingsMountPath = filepath.Join(directory, "default-scrape-settings-enabled")
		configMapKeepListMountPath = filepath.Join(directory, "default-targets-metrics-keep-list")
		configMapMountPathForPodAnnotation = filepath.Join(directory, "pod-annotation-based-scraping")
		collectorSettingsMountPath = filepath.Join(directory, "prometheus-collector-settings")
		schemaVersionFile = filepath.Join(directory, "schema-version")
		configVersionFile = filepath.Join(directory, "config-version")
		configMapScrapeIntervalMountPath = filepath.Join(directory, "default-targets-scrape-interval-settings")
	} else {
		schemaVersionFile = createTempFile("schema-version", "v1")
		configVersionFile = createTempFile("config-version", "ver1")
		configMapMountPathForPodAnnotation = createTempFile("podannotation", "")
		collectorSettingsMountPath = createTempFile("collector-settings", "")
		defaultSettingsMountPath = createTempFile("default-settings", "")
		configMapDebugMountPath = createTempFile("debug-mode", "")
		configMapKeepListMountPath = createTempFile("keep-list", "")
		configMapScrapeIntervalMountPath = createTempFile("scrape-interval", "")
	}
}

func setupProcessedFiles() {
	podAnnotationEnvVarPath = createTempFile("podannotation-envvar", "")
	collectorSettingsEnvVarPath = createTempFile("collector-settings-envvar", "")
	defaultSettingsEnvVarPath = createTempFile("default-settings-envvar", "")
	debugModeEnvVarPath = createTempFile("debug-mode-envvar", "")
	configMapKeepListEnvVarPath = createTempFile("keep-list-envvar", "")
	scrapeIntervalEnvVarPath = createTempFile("scrape-interval-envvar", "")

	mergedDefaultConfigPath = createTempFile("merged-default-config", "")
	regexHashFile = configMapKeepListEnvVarPath
	intervalHashFile = scrapeIntervalEnvVarPath
}

func cleanupEnvVars() {
	allEnvVars := []string{
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
