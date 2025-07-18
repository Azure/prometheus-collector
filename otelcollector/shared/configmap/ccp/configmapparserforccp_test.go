package ccpconfigmapsettings

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

type TestConfig struct {
	// Setup parameters
	EnvVars            map[string]string
	UseConfigFiles     bool
	IsDefaultConfig    bool
	ConfigMapFiles     map[string]string
	ConfigMapMountPath string

	// Expected results
	ExpectedEnvVars             map[string]string
	ExpectedKeepListHashMap     map[string]string
	ExpectedScrapeIntervalMap   map[string]string
	ExpectedDefaultContentsPath string
	ExpectedMergedContentsPath  string
}

func setupTest() {
	var originalDefaultScrapeJobs map[string]shared.DefaultScrapeJob
	BeforeEach(func() {
		cleanupEnvVars()
		configSettingsPrefix = "/tmp/settings/"
		os.RemoveAll(configSettingsPrefix)
		// Save a copy of the original DefaultScrapeJobs for restoring later
		originalDefaultScrapeJobs = make(map[string]shared.DefaultScrapeJob)
		for key, job := range shared.ControlPlaneDefaultScrapeJobs {
			originalDefaultScrapeJobs[key] = *job
		}
	})
	AfterEach(func() {
		cleanupEnvVars()
		// Remove any temporary files created during the tests
		os.RemoveAll(configSettingsPrefix)
		// Restore the original DefaultScrapeJobs
		for key, job := range originalDefaultScrapeJobs {
			shared.ControlPlaneDefaultScrapeJobs[key] = &job
		}
		os.RemoveAll("../../../configmapparser/default-prom-configs/test/")
	})
}

var _ = Describe("Configmapparserforccp", Ordered, func() {
	setupTest()

	Context("when the settings configmap does not exist", func() {
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)

			expectedEnvVars := getDefaultExpectedEnvVars()
			expectedKeepListHashMap := getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(true, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist but are empty", func() {
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)

			expectedEnvVars := getDefaultExpectedEnvVars()
			expectedKeepListHashMap := getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		It("should process the config for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)
			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "true",
				"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "true",
				"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "true",
				"AZMON_PROMETHEUS_ETCD_ENABLED":                    "true",
				"AZMON_PROMETHEUS_NODE-AUTO-PROVISIONING_ENABLED":  "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(true, "test.*|test2")

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", `cluster_alias = "alias"`)
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
				kubelet = true
				coredns = false
				cadvisor = true
				kubeproxy = false
				apiserver = false
				kubestate = true
				nodeexporter = true
				windowsexporter = false
				windowskubeproxy = false
				kappiebasic = true
				networkobservabilityRetina = true
				networkobservabilityHubble = true
				networkobservabilityCilium = true
				prometheuscollectorhealth = false
				controlplane-apiserver = true
				controlplane-cluster-autoscaler = true
				controlplane-kube-scheduler = true
				controlplane-kube-controller-manager = true
				controlplane-node-auto-provisioning = true
				controlplane-etcd = true
				acstor-capacity-provisioner = true
				acstor-metrics-exporter = true
			`)
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
				controlplane-apiserver = "test.*|test2"
				controlplane-cluster-autoscaler = "test.*|test2"
				controlplane-kube-scheduler = "test.*|test2"
				controlplane-kube-controller-manager = "test.*|test2"
				controlplane-etcd = "test.*|test2"
				controlplane-node-auto-provisioning = "test.*|test2"
				acstor-capacity-provisioner = "test.*|test2"
				acstor-metrics-exporter = "test.*|test2"
				minimalingestionprofile = true
			`)

			checkResults(false, false, expectedEnvVars, expectedKeepListHashMap, nil, "./testdata/advanced-linux-rs.yaml", "")
		})
	})

	Context("when the configmap sections exist but all scrape jobs are false", func() {
		It("should process the config with no scrape jobs enabled for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)
			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_APISERVER_ENABLED":               "false",
				"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "false",
				"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "false",
				"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "false",
				"AZMON_PROMETHEUS_ETCD_ENABLED":                    "false",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")
			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
				kubelet = false
				coredns = false
				cadvisor = false
				kubeproxy = false
				apiserver = false
				kubestate = false
				nodeexporter = false
				windowsexporter = false
				windowskubeproxy = false
				kappiebasic = false
				networkobservabilityRetina = false
				networkobservabilityHubble = false
				networkobservabilityCilium = false
				acstor-capacity-provisioner = false
				acstor-metrics-exporter = false
				controlplane-apiserver = false
				controlplane-cluster-autoscaler = false
				controlplane-kube-scheduler = false
				controlplane-kube-controller-manager = false
				controlplane-node-auto-provisioning = false
				controlplane-etcd = false
				prometheuscollectorhealth = false
			`)
			isDefaultConfig := false
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when minimal ingestion is not true", func() {
		It("should handle it being set to false with the keeplist regex values", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)
			expectedKeepListHashMap := getExpectedKeepListMap(false, "test.*|test2")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
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
				controlplane-apiserver = "test.*|test2"
				controlplane-cluster-autoscaler = "test.*|test2"
				controlplane-kube-scheduler = "test.*|test2"
				controlplane-kube-controller-manager = "test.*|test2"
				controlplane-node-auto-provisioning = "test.*|test2"
				controlplane-etcd = "test.*|test2"
				minimalingestionprofile = false
			`)
			checkResults(false, false, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-rs.yaml", "")
		})
	})

	Context("when the settings configmap uses v2 and the sections are not default", Label("v2"), func() {
		It("should process the config with custom settings for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)

			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "true",
				"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "true",
				"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "true",
				"AZMON_PROMETHEUS_ETCD_ENABLED":                    "true",
				"AZMON_PROMETHEUS_NODE-AUTO-PROVISIONING_ENABLED":  "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(true, "test.*|test2")
			expectedContentsFilePath := "./testdata/advanced-linux-rs.yaml"

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = createTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
			`)
			_ = createTempFile(configSettingsPrefix, "controlplane-metrics", `
				default-targets-scrape-enabled: |-
					apiserver = true
					cluster-autoscaler = true
					kube-scheduler = true
					kube-controller-manager = true
					etcd = true
					node-auto-provisioning = true
				default-targets-metrics-keep-list: |-
					apiserver = "test.*|test2"
					cluster-autoscaler = "test.*|test2"
					kube-scheduler = "test.*|test2"
					kube-controller-manager = "test.*|test2"
					etcd = "test.*|test2"
					node-auto-provisioning = "test.*|test2"
				minimal-ingestion-profile: |-
					enabled = true
	  		`)
			_ = createTempFile(configSettingsPrefix, "cluster-metrics", `
				default-targets-scrape-enabled: |-
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
					acstor-capacity-provisioner = true
					acstor-metrics-exporter = true
					prometheuscollectorhealth = false
				pod-annotation-based-scraping: |-
					podannotationnamespaceregex = ".*|value"
				default-targets-metrics-keep-list: |-
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
				minimal-ingestion-profile: |-
					enabled = true
				default-targets-scrape-interval-settings: |-
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

			isDefaultConfig := false
			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, nil, expectedContentsFilePath, "")
		})
		It("should process when the minimal ingestion profile is false for CCP", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)

			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "true",
				"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "true",
				"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "true",
				"AZMON_PROMETHEUS_ETCD_ENABLED":                    "true",
				"AZMON_PROMETHEUS_NODE-AUTO-PROVISIONING_ENABLED":  "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(false, "test.*|test2")
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-rs.yaml"

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = createTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
			`)
			_ = createTempFile(configSettingsPrefix, "controlplane-metrics", `
				default-targets-scrape-enabled: |-
					apiserver = true
					cluster-autoscaler = true
					kube-scheduler = true
					kube-controller-manager = true
					etcd = true
					node-auto-provisioning = true
				default-targets-metrics-keep-list: |-
					apiserver = "test.*|test2"
					cluster-autoscaler = "test.*|test2"
					kube-scheduler = "test.*|test2"
					kube-controller-manager = "test.*|test2"
					etcd = "test.*|test2"
					node-auto-provisioning = "test.*|test2"
				minimal-ingestion-profile: |-
					enabled = false
	  		`)
			_ = createTempFile(configSettingsPrefix, "cluster-metrics", `
				default-targets-scrape-enabled: |-
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
					acstor-capacity-provisioner = true
					acstor-metrics-exporter = true
					prometheuscollectorhealth = false
				pod-annotation-based-scraping: |-
					podannotationnamespaceregex = ".*|value"
				default-targets-metrics-keep-list: |-
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
				minimal-ingestion-profile: |-
					enabled = true
				default-targets-scrape-interval-settings: |-
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

			isDefaultConfig := false
			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, nil, expectedContentsFilePath, "")
		})
		It("should process the config with no scrape jobs enabled and without all sections for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)
			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_APISERVER_ENABLED":               "false",
				"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "false",
				"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "false",
				"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "false",
				"AZMON_PROMETHEUS_ETCD_ENABLED":                    "false",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(false, "")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")
			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			_ = createTempFile(configSettingsPrefix, "controlplane-metrics", `
				default-targets-scrape-enabled: |-
					apiserver = false
					cluster-autoscaler = false
					kube-scheduler = false
					kube-controller-manager = false
					etcd = false
					node-auto-provisioning = false
				minimal-ingestion-profile: |-
					enabled = false
	  		`)
			isDefaultConfig := false
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})
})

// Helper functions (these should mirror the ones in the MP test file)

func checkResults(useConfigFiles bool, isDefaultConfig bool, expectedEnvVars map[string]string, expectedKeepListHashMap map[string]string, expectedScrapeIntervalHashMap map[string]string, expectedDefaultContentsFilePath string, expectedMergedContentsFilePath string) {
	if useConfigFiles {
		setupConfigFiles(isDefaultConfig)
	}
	setupProcessedFiles()
	processAndMergeConfigFiles()

	if expectedEnvVars != nil {
		err := checkEnvVars(expectedEnvVars)
		Expect(err).NotTo(HaveOccurred())
	}

	checkHashMaps(configMapKeepListEnvVarPath, expectedKeepListHashMap)

	mergedFileContents, err := ioutil.ReadFile(mergedDefaultConfigPath)
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(string(mergedFileContents))
	expectedDefaultFileContents, err := ioutil.ReadFile(expectedDefaultContentsFilePath)
	Expect(err).NotTo(HaveOccurred())

	var mergedConfig, expectedConfig, customMergedConfig, expectedCustomConfig map[string]interface{}

	err = yaml.Unmarshal(mergedFileContents, &mergedConfig)
	Expect(err).NotTo(HaveOccurred())

	fmt.Println("Merged Config:", mergedConfig)

	err = yaml.Unmarshal(expectedDefaultFileContents, &expectedConfig)
	Expect(err).NotTo(HaveOccurred())

	// Order the scrape_configs by job_name for consistent comparison
	for _, config := range []map[string]interface{}{mergedConfig, expectedConfig, customMergedConfig, expectedCustomConfig} {
		if config != nil {
			if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
				sort.Slice(scrapeConfigs, func(i, j int) bool {
					iConfig := scrapeConfigs[i].(map[interface{}]interface{})
					jConfig := scrapeConfigs[j].(map[interface{}]interface{})
					return iConfig["job_name"].(string) < jConfig["job_name"].(string)
				})
				config["scrape_configs"] = scrapeConfigs
			}
		}
	}

	// Use BeEquivalentTo which compares content without requiring same order
	Expect(mergedConfig).To(BeEquivalentTo(expectedConfig), "Prometheus config content doesn't match")
	if expectedMergedContentsFilePath != "" {
		Expect(customMergedConfig).To(BeEquivalentTo(expectedCustomConfig), "Expected Custom Prometheus config content doesn't match")
	}
}

func setEnvVars(envVars map[string]string) {
	for key, value := range envVars {
		os.Setenv(key, value)
	}
}

func setSetupEnvVars(controllertype string, os string) {
	var envVars map[string]string
	switch controllertype {
	case shared.ControllerType.ReplicaSet:
		envVars = map[string]string{
			"AZMON_OPERATOR_ENABLED":    "false",
			"CONTAINER_TYPE":            "",
			"CONTROLLER_TYPE":           "ReplicaSet",
			"OS_TYPE":                   "linux",
			"MODE":                      "advanced",
			"KUBE_STATE_NAME":           "ama-metrics-ksm",
			"POD_NAMESPACE":             "kube-system",
			"MAC":                       "true",
			"AZMON_SET_GLOBAL_SETTINGS": "true",
		}
	default:
		envVars = map[string]string{}
	}

	setEnvVars(envVars)
}

func getDefaultExpectedEnvVars() map[string]string {
	return map[string]string{
		"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "",
		"AZMON_AGENT_CFG_FILE_VERSION":                     "",
		"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "",
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                "",
		"AZMON_CLUSTER_LABEL":                              "",
		"AZMON_CLUSTER_ALIAS":                              "",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "false",
		"AZMON_OPERATOR_ENABLED":                           "false",
		"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":           "",
		"AZMON_PROMETHEUS_APISERVER_ENABLED":               "true",
		"AZMON_PROMETHEUS_CLUSTER-AUTOSCALER_ENABLED":      "false",
		"AZMON_PROMETHEUS_KUBE-SCHEDULER_ENABLED":          "false",
		"AZMON_PROMETHEUS_KUBE-CONTROLLER-MANAGER_ENABLED": "false",
		"AZMON_PROMETHEUS_ETCD_ENABLED":                    "true",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
		"DEBUG_MODE_ENABLED":                               "",
		"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":           "",
		"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                "",
		"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":              "",
		//"MAC":                                              "true",
	}
}

// Helper function to generate standard expected keep list map
func getExpectedKeepListMap(includeMinimalKeepList bool, value string) map[string]string {
	expectedKeepListHashMap := make(map[string]string)
	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		// TODO: METRICS_KEEP_LIST OR KEEP_LIST_REGEX
		keyName := fmt.Sprintf("CONTROLPLANE_%s_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		if includeMinimalKeepList {
			expectedKeepListHashMap[keyName] = fmt.Sprintf("%s|%s", value, job.MinimalKeepListRegex)
		} else {
			expectedKeepListHashMap[keyName] = value
		}
	}
	return expectedKeepListHashMap
}

// Helper function to generate standard expected scrape interval map
func getExpectedScrapeIntervalMap(value string) map[string]string {
	expectedScrapeIntervalHashMap := make(map[string]string)
	for jobName := range shared.ControlPlaneDefaultScrapeJobs {
		keyName := fmt.Sprintf("%s_SCRAPE_INTERVAL", strings.ToUpper(jobName))
		if value == "" {
			expectedScrapeIntervalHashMap[keyName] = "30s"
		} else {
			expectedScrapeIntervalHashMap[keyName] = value
		}
	}
	return expectedScrapeIntervalHashMap
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

func setupConfigFiles(defaultPath bool) {
	if defaultPath {
		defaultSettingsMountPath = "/etc/config/settings/default-scrape-settings"
		configMapKeepListMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
		collectorSettingsMountPath = "/etc/config/settings/prometheus-collector-settings"
		//schemaVersionFile = "/etc/config/settings/schema-version"
		//configVersionFile = "/etc/config/settings/config-version"
		createTempFile(configSettingsPrefix, "controlplane-metrics", "")
		createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
	} else {
		//schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
		//configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
		collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
		defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
		configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", "")
		createTempFile(configSettingsPrefix, "cluster-metrics", "")
		createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
	}
}

func setupProcessedFiles() {
	collectorSettingsEnvVarPath = createTempFile(configSettingsPrefix, "collector-settings-envvar", "")
	defaultSettingsEnvVarPath = createTempFile(configSettingsPrefix, "default-settings-envvar", "")
	configMapKeepListEnvVarPath = createTempFile(configSettingsPrefix, "keep-list-envvar", "")

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

func cleanupEnvVars() {
	allEnvVars := []string{
		"CONTAINER_TYPE",
		"CONTROLLER_TYPE",
		"OS_TYPE",
		"MODE",
		"KUBE_STATE_NAME",
		"POD_NAMESPACE",
		"MAC",
		"NODE_NAME",
		"NODE_IP",
		"NODE_EXPORTER_TARGETPORT",
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
		"AZMON_PROMETHEUS_APISERVER_ENABLED",
		"AZMON_PROMETHEUS_CLUSTER_AUTOSCALER_ENABLED",
		"AZMON_PROMETHEUS_KUBESCHEDULER_ENABLED",
		"AZMON_PROMETHEUS_KUBECONTROLLERMANAGER_ENABLED",
		"AZMON_PROMETHEUS_ETCD_ENABLED",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED",
		"DEBUG_MODE_ENABLED",
		"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG",
		"CONFIG_VALIDATOR_RUNNING_IN_AGENT",
		"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG",
		"MAC",
	}
	for _, envVar := range allEnvVars {
		os.Unsetenv(envVar)
	}
}
