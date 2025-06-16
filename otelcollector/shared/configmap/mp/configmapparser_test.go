package configmapsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

/*
 * For each type of ama-metrics pod, (Linux ReplicaSet, Linux DaemonSet, Windows DaemonSet):
 * 1) Test that the settings from the configmaps are correctly parsed and set to environment variables.
 * 2) Test that the Prometheus config created is as expected for the settings given.
 */
var _ = Describe("Configmapparser", Ordered, Label("original-test"), func() {
	var originalDefaultScrapeJobs map[string]shared.DefaultScrapeJob
	BeforeEach(func() {
		cleanupEnvVars()
		configSettingsPrefix = "/tmp/settings/"
		os.RemoveAll(configSettingsPrefix)
		// Save a copy of the original DefaultScrapeJobs for restoring later
		originalDefaultScrapeJobs = make(map[string]shared.DefaultScrapeJob)
		for key, job := range shared.DefaultScrapeJobs {
			originalDefaultScrapeJobs[key] = *job
		}
	})
	AfterEach(func() {
		cleanupEnvVars()
		// Remove any temporary files created during the tests
		os.RemoveAll(configSettingsPrefix)
		// Restore the original DefaultScrapeJobs
		for key, job := range originalDefaultScrapeJobs {
			shared.DefaultScrapeJobs[key] = &job
		}
	})

	Context("when the settings configmap does not exist", func() {
		AfterEach(func() {
			cleanupEnvVars()
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				//"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":         "linux",
				"MODE":            "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE":   "kube-system",
				"MAC":             "true",
			})
			expectedEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                     "",
				"AZMON_AGENT_CFG_FILE_VERSION":                       "",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":   "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                  "",
				"AZMON_CLUSTER_LABEL":                                "",
				"AZMON_CLUSTER_ALIAS":                                "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":               "false",
				"AZMON_OPERATOR_ENABLED":                             "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":             "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":          "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":          "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":         "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":        "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":        "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":     "true",
				"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED": "",
				//"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				//"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
				//"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
			}
			expectedKeepListHashMap := make(map[string]string)
			for jobName, job := range shared.DefaultScrapeJobs {
				expectedKeepListHashMap[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = fmt.Sprintf("|%s", job.MinimalKeepListRegex)
			}
			expectedScrapeIntervalHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "30s",
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				//"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				//"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":             "30s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":  "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
			}
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(true, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath)
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED":   "true",
				"CONTAINER_TYPE":           "",
				"CONTROLLER_TYPE":          "DaemonSet",
				"OS_TYPE":                  "linux",
				"MODE":                     "advanced",
				"KUBE_STATE_NAME":          "ama-metrics-ksm",
				"POD_NAMESPACE":            "kube-system",
				"MAC":                      "true",
				"NODE_NAME":                "test-node",
				"NODE_IP":                  "192.168.1.1",
				"NODE_EXPORTER_TARGETPORT": "9100",
			})
			expectedEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "",
				"AZMON_CLUSTER_ALIAS":                              "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "false",
				"AZMON_OPERATOR_ENABLED":                           "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":        "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":   "true",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "false",
				//"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED":                     "",
				"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				"CONFIG_VALIDATOR_RUNNING_IN_AGENT":      "true",
				"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":    "",
			}
			expectedKeepListHashMap := make(map[string]string)
			for jobName, job := range shared.DefaultScrapeJobs {
				expectedKeepListHashMap[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = fmt.Sprintf("|%s", job.MinimalKeepListRegex)
			}
			expectedScrapeIntervalHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "30s",
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				//"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				//"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":             "30s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":  "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
			}
			isDefaultConfig := true
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"

			checkResults(true, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath)
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
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
				//"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":         "linux",
				"MODE":            "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE":   "kube-system",
				"MAC":             "true",
			})
			expectedEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "",
				"AZMON_CLUSTER_ALIAS":                              "",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "false",
				"AZMON_OPERATOR_ENABLED":                           "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":        "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":   "true",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "false",
				//"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				//"DEBUG_MODE_ENABLED": "false",
				//"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
				//"CONFIG_VALIDATOR_RUNNING_IN_AGENT":   "true",
				//"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG": "true",
			}
			expectedKeepListHashMap := make(map[string]string)
			for jobName, job := range shared.DefaultScrapeJobs {
				expectedKeepListHashMap[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = fmt.Sprintf("|%s", job.MinimalKeepListRegex)
			}
			expectedScrapeIntervalHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "30s",
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				//"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				//"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":             "30s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":  "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
			}
			isDefaultConfig := false
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"

			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath)
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {

		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		BeforeEach(func() {
			cleanupEnvVars()
			configSettingsPrefix = "/tmp/settings/"
			os.RemoveAll(configSettingsPrefix)
		})
		AfterEach(func() {
			cleanupEnvVars()
			configSettingsPrefix = "/tmp/settings/"
			os.RemoveAll(configSettingsPrefix)
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
			expectedEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				"AZMON_OPERATOR_ENABLED":                           "true",
				"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":        "true",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":       "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":   "true",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "true",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "true",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "true",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				//"DEBUG_MODE_ENABLED": "true",
				//"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG": "false",
				//"CONFIG_VALIDATOR_RUNNING_IN_AGENT":      "true",
				//"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":    "true",
			}

			fmt.Println("Creating temporary files for config settings")
			fmt.Println("Config settings prefix:", configSettingsPrefix)
			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", `podannotationnamespaceregex = ".*|value"`)
			collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", `cluster_alias = "alias"`)
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
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
			configMapDebugMountPath = createTempFile(configSettingsPrefix, "debug-mode", `enabled = true`)
			configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
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
				acstor-capacity-provisioner = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				minimalingestionprofile = true
			`)
			configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", `
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
				acstor-capacity-provisioner = "15s"
    			acstor-metrics-exporter = "15s"
			`)
			fmt.Println("Contents of configMapKeepListMountPath:", configMapKeepListMountPath)
			data, err := ioutil.ReadFile(configMapKeepListMountPath)
			if err != nil {
				fmt.Printf("Error reading keep list file: %v\n", err)
			} else {
				fmt.Println(string(data))
			}
			expectedKeepListHashMap := make(map[string]string)
			for jobName, job := range shared.DefaultScrapeJobs {
				expectedKeepListHashMap[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = fmt.Sprintf("test.*|test2|%s", job.MinimalKeepListRegex)
			}

			expectedScrapeIntervalHashMap := map[string]string{
				"KUBELET_SCRAPE_INTERVAL":                     "15s",
				"COREDNS_SCRAPE_INTERVAL":                     "15s",
				"CADVISOR_SCRAPE_INTERVAL":                    "15s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "15s",
				"APISERVER_SCRAPE_INTERVAL":                   "15s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "15s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "15s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "15s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "15s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":   "15s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":              "15s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                 "15s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL":  "15s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL":  "15s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL":  "15s",
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "15s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "15s",
			}

			checkResults(false, false, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-linux-rs.yaml")
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
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				//"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":         "linux",
				"MODE":            "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE":   "kube-system",
				"MAC":             "true",
			})
			expectedKeepListHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_METRICS_KEEP_LIST_REGEX": "test.*|test2",
				"ACSTOR-METRICS-EXPORTER_METRICS_KEEP_LIST_REGEX":     "test.*|test2",
				"KUBELET_METRICS_KEEP_LIST_REGEX":                     "test.*|test2",
				"COREDNS_METRICS_KEEP_LIST_REGEX":                     "test.*|test2",
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                    "test.*|test2",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                   "test.*|test2",
				"APISERVER_METRICS_KEEP_LIST_REGEX":                   "test.*|test2",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                   "test.*|test2",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":                "test.*|test2",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":             "test.*|test2",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":            "test.*|test2",
				"PODANNOTATIONS_METRICS_KEEP_LIST_REGEX":              "test.*|test2",
				"PROMETHEUSCOLLECTORHEALTH_METRICS_KEEP_LIST_REGEX":   "test.*|test2",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                 "test.*|test2",
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX":  "test.*|test2",
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX":  "test.*|test2",
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX":  "test.*|test2",
			}
			expectedScrapeIntervalHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "30s",
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				//"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				//"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":             "30s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":  "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
			}

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
			collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
			configMapDebugMountPath = createTempFile(configSettingsPrefix, "debug-mode", "")
			configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
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
				acstor-capacity-provisioner = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", ``)

			checkResults(false, false, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-rs.yaml")
		})

		It("should handle it being set to false with no keeplist regex values", func() {
			setEnvVars(map[string]string{
				"AZMON_OPERATOR_ENABLED": "true",
				"CONTAINER_TYPE":         "ConfigReaderSidecar",
				//"CONTROLLER_TYPE":        "ReplicaSet",
				"OS_TYPE":         "linux",
				"MODE":            "advanced",
				"KUBE_STATE_NAME": "ama-metrics-ksm",
				"POD_NAMESPACE":   "kube-system",
				"MAC":             "true",
			})
			expectedKeepListHashMap := map[string]string{
				"KUBELET_METRICS_KEEP_LIST_REGEX":                     "",
				"COREDNS_METRICS_KEEP_LIST_REGEX":                     "",
				"CADVISOR_METRICS_KEEP_LIST_REGEX":                    "",
				"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                   "",
				"APISERVER_METRICS_KEEP_LIST_REGEX":                   "",
				"KUBESTATE_METRICS_KEEP_LIST_REGEX":                   "",
				"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":                "",
				"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":             "",
				"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":            "",
				"PODANNOTATIONS_METRICS_KEEP_LIST_REGEX":              "",
				"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                 "",
				"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX":  "",
				"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX":  "",
				"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX":  "",
				"ACSTOR-CAPACITY-PROVISIONER_METRICS_KEEP_LIST_REGEX": "",
				"ACSTOR-METRICS-EXPORTER_METRICS_KEEP_LIST_REGEX":     "",
				"PROMETHEUSCOLLECTORHEALTH_METRICS_KEEP_LIST_REGEX":   "",
			}
			expectedScrapeIntervalHashMap := map[string]string{
				"ACSTOR-CAPACITY-PROVISIONER_SCRAPE_INTERVAL": "30s",
				"ACSTOR-METRICS-EXPORTER_SCRAPE_INTERVAL":     "30s",
				"KUBELET_SCRAPE_INTERVAL":                     "30s",
				"COREDNS_SCRAPE_INTERVAL":                     "30s",
				"CADVISOR_SCRAPE_INTERVAL":                    "30s",
				"KUBEPROXY_SCRAPE_INTERVAL":                   "30s",
				"APISERVER_SCRAPE_INTERVAL":                   "30s",
				"KUBESTATE_SCRAPE_INTERVAL":                   "30s",
				"NODEEXPORTER_SCRAPE_INTERVAL":                "30s",
				"WINDOWSEXPORTER_SCRAPE_INTERVAL":             "30s",
				"WINDOWSKUBEPROXY_SCRAPE_INTERVAL":            "30s",
				//"PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL": "30s",
				//"POD_ANNOTATION_SCRAPE_INTERVAL":              "30s",
				"PODANNOTATIONS_SCRAPE_INTERVAL":             "30s",
				"PROMETHEUSCOLLECTORHEALTH_SCRAPE_INTERVAL":  "30s",
				"KAPPIEBASIC_SCRAPE_INTERVAL":                "30s",
				"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL": "30s",
				"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL": "30s",
			}

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
			collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
			configMapDebugMountPath = createTempFile(configSettingsPrefix, "debug-mode", "")
			configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", ``)

			checkResults(false, false, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-no-keeplist-rs.yaml")
		})
	})
})

func checkResults(useConfigFiles bool, isDefaultConfig bool, expectedEnvVars map[string]string, expectedKeepListHashMap map[string]string, expectedScrapeIntervalHashMap map[string]string, expectedContentsFilePath string) {
	if useConfigFiles {
		setupConfigFiles(isDefaultConfig)
	}
	setupProcessedFiles()
	processConfigFiles()

	err := checkEnvVars(expectedEnvVars)
	Expect(err).NotTo(HaveOccurred())

	checkHashMaps(configMapKeepListEnvVarPath, expectedKeepListHashMap)

	checkHashMaps(scrapeIntervalEnvVarPath, expectedScrapeIntervalHashMap)

	mergedFileContents, err := ioutil.ReadFile(mergedDefaultConfigPath)
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(string(mergedFileContents))
	expectedFileContents, err := ioutil.ReadFile(expectedContentsFilePath)
	Expect(err).NotTo(HaveOccurred())

	var mergedConfig, expectedConfig map[string]interface{}

	err = yaml.Unmarshal(mergedFileContents, &mergedConfig)
	Expect(err).NotTo(HaveOccurred())

	err = yaml.Unmarshal(expectedFileContents, &expectedConfig)
	Expect(err).NotTo(HaveOccurred())

	// Order the scrape_configs by job_name for consistent comparison
	if scrapeConfigs, ok := mergedConfig["scrape_configs"].([]interface{}); ok {
		sort.Slice(scrapeConfigs, func(i, j int) bool {
			iConfig := scrapeConfigs[i].(map[interface{}]interface{})
			jConfig := scrapeConfigs[j].(map[interface{}]interface{})
			return iConfig["job_name"].(string) < jConfig["job_name"].(string)
		})
		mergedConfig["scrape_configs"] = scrapeConfigs
	}

	if scrapeConfigs, ok := expectedConfig["scrape_configs"].([]interface{}); ok {
		sort.Slice(scrapeConfigs, func(i, j int) bool {
			iConfig := scrapeConfigs[i].(map[interface{}]interface{})
			jConfig := scrapeConfigs[j].(map[interface{}]interface{})
			return iConfig["job_name"].(string) < jConfig["job_name"].(string)
		})
		expectedConfig["scrape_configs"] = scrapeConfigs
	}

	// Use BeEquivalentTo which compares content without requiring same order
	Expect(mergedConfig).To(BeEquivalentTo(expectedConfig), "Prometheus config content doesn't match")
}

func createTempFile(dir string, name string, content string) string {
	// Make sure the directory exists
	fmt.Println("creating file with content", content)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		GinkgoT().Fatalf("Failed to create directory %s: %v", dir, err)
	}
	filepath := dir + name
	tempFile, err := os.Create(filepath)
	Expect(err).NotTo(HaveOccurred())
	_, err = tempFile.WriteString(content)
	Expect(err).NotTo(HaveOccurred())

	err = tempFile.Close()
	Expect(err).NotTo(HaveOccurred())

	// Check if the file was created successfully
	info, err := os.Stat(filepath)
	if err != nil {
		GinkgoT().Fatalf("Failed to verify file %s was created: %v", filepath, err)
	}
	if info.Size() != int64(len(content)) {
		GinkgoT().Fatalf("File %s has incorrect size: expected %d, got %d", filepath, len(content), info.Size())
	}
	fmt.Printf("Created temp file: %s with content length: %d\n", filepath, info.Size())
	return filepath // Return the full path instead of just the name
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
		configMapDebugMountPath = "/etc/config/settings/debug-mode"
		replicaSetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
		defaultSettingsMountPath = "/etc/config/settings/default-scrape-settings"
		configMapKeepListMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
		configMapMountPathForPodAnnotation = "/etc/config/settings/pod-annotation-based-scraping"
		collectorSettingsMountPath = "/etc/config/settings/prometheus-collector-settings"
		schemaVersionFile = "/etc/config/settings/schema-version"
		configVersionFile = "/etc/config/settings/config-version"
		configMapScrapeIntervalMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
	} else {
		schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
		configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
		configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
		collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
		defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
		configMapDebugMountPath = createTempFile(configSettingsPrefix, "debug-mode", "")
		configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", "")
		configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", "")
		replicaSetCollectorConfig = "./testdata/collector-config-replicaset.yml"
	}
}

func setupProcessedFiles() {
	podAnnotationEnvVarPath = createTempFile(configSettingsPrefix, "podannotation-envvar", "")
	collectorSettingsEnvVarPath = createTempFile(configSettingsPrefix, "collector-settings-envvar", "")
	defaultSettingsEnvVarPath = createTempFile(configSettingsPrefix, "default-settings-envvar", "")
	debugModeEnvVarPath = createTempFile(configSettingsPrefix, "debug-mode-envvar", "")
	configMapKeepListEnvVarPath = createTempFile(configSettingsPrefix, "keep-list-envvar", "")
	scrapeIntervalEnvVarPath = createTempFile(configSettingsPrefix, "scrape-interval-envvar", "")

	// Create test directory if it doesn't exist
	testDir := "../../../configmapparser/default-prom-configs/test/"
	err := os.MkdirAll(testDir, 0755)
	if err != nil {
		fmt.Printf("Error creating test directory: %v\n", err)
	}

	// Get list of files from source directory
	srcDir := "../../../configmapparser/default-prom-configs/"
	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		fmt.Printf("Error reading source directory: %v\n", err)
	}

	// Copy each file to the test directory
	for _, file := range files {
		if !file.IsDir() {
			srcPath := srcDir + file.Name()
			dstPath := testDir + file.Name()

			// Read source file
			data, err := ioutil.ReadFile(srcPath)
			if err != nil {
				fmt.Printf("Error reading file %s: %v\n", srcPath, err)
				continue
			}

			// Write to destination file
			err = ioutil.WriteFile(dstPath, data, 0644)
			if err != nil {
				fmt.Printf("Error writing file %s: %v\n", dstPath, err)
			}
		}
	}

	scrapeConfigDefinitionPathPrefix = "../../../configmapparser/default-prom-configs/test/"
	mergedDefaultConfigPath = createTempFile(configSettingsPrefix, "merged-default-config", "")
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
