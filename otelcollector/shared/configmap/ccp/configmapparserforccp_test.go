package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
	"github.com/prometheus-collector/shared/configmap/common/testhelpers"
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
		testhelpers.UnsetEnvVars(testhelpers.DefaultCollectorEnvVarKeys())
		configSettingsPrefix = "/tmp/settings/"
		configMapParserPrefix = "/tmp/configmapparser/"
		os.RemoveAll(configSettingsPrefix)
		os.RemoveAll(configMapParserPrefix)
		_ = os.MkdirAll(configSettingsPrefix, 0755)
		_ = os.MkdirAll(configMapParserPrefix, 0755)
		// Save a copy of the original DefaultScrapeJobs for restoring later
		originalDefaultScrapeJobs = make(map[string]shared.DefaultScrapeJob)
		for key, job := range shared.ControlPlaneDefaultScrapeJobs {
			originalDefaultScrapeJobs[key] = *job
		}
	})
	AfterEach(func() {
		testhelpers.UnsetEnvVars(testhelpers.DefaultCollectorEnvVarKeys())
		// Remove any temporary files created during the tests
		os.RemoveAll(configSettingsPrefix)
		os.RemoveAll(configMapParserPrefix)
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())

			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, true, "", false)
			expectedScrapeIntervalHashMap := testhelpers.ExpectedScrapeIntervalMap(shared.ControlPlaneDefaultScrapeJobs, "")
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(true, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist but are empty", func() {
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())

			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, true, "", false)
			expectedScrapeIntervalHashMap := testhelpers.ExpectedScrapeIntervalMap(shared.ControlPlaneDefaultScrapeJobs, "")
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		It("should process the config for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())
			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
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
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, true, "test.*|test2", false)

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			collectorSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", `cluster_alias = "alias"`)
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
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
			configMapKeepListMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())
			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
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
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, true, "", false)
			expectedScrapeIntervalHashMap := testhelpers.ExpectedScrapeIntervalMap(shared.ControlPlaneDefaultScrapeJobs, "")
			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, false, "test.*|test2", false)
			expectedScrapeIntervalHashMap := testhelpers.ExpectedScrapeIntervalMap(shared.ControlPlaneDefaultScrapeJobs, "")

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			collectorSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
			configMapKeepListMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())

			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
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
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, true, "test.*|test2", false)
			expectedContentsFilePath := "./testdata/advanced-linux-rs.yaml"

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
			`)
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "controlplane-metrics", `
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
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "cluster-metrics", `
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())

			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
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
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, false, "test.*|test2", false)
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-rs.yaml"

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
			`)
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "controlplane-metrics", `
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
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "cluster-metrics", `
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
			Expect(testhelpers.SetControlPlaneEnvVars(shared.ControllerType.ReplicaSet, shared.OSType.Linux)).To(Succeed())
			expectedEnvVars := testhelpers.DefaultControlPlaneEnvVars()
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
			expectedKeepListHashMap := testhelpers.ExpectedKeepListMap(shared.ControlPlaneDefaultScrapeJobs, false, "", false)
			expectedScrapeIntervalHashMap := testhelpers.ExpectedScrapeIntervalMap(shared.ControlPlaneDefaultScrapeJobs, "")
			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "controlplane-metrics", `
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
		Expect(testhelpers.CheckEnvVars(expectedEnvVars)).To(Succeed())
	}

	keepListHash, err := testhelpers.ReadYAMLStringMap(configMapKeepListEnvVarPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(keepListHash).To(BeComparableTo(expectedKeepListHashMap))

	mergedFileContents, err := os.ReadFile(mergedDefaultConfigPath)
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(string(mergedFileContents))
	expectedDefaultFileContents, err := os.ReadFile(expectedDefaultContentsFilePath)
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

func setupConfigFiles(defaultPath bool) {
	prefixes := testhelpers.CollectorPrefixesFor(configSettingsPrefix, configMapParserPrefix, false)
	paths := testhelpers.CollectorConfigPaths{
		CollectorSettingsMountPath: &collectorSettingsMountPath,
		DefaultSettingsMountPath:   &defaultSettingsMountPath,
		ConfigMapKeepListMountPath: &configMapKeepListMountPath,
	}

	if err := testhelpers.SetupCollectorConfigFiles(defaultPath, prefixes, paths); err != nil {
		panic(err)
	}

	testhelpers.MustCreateTempFile(configSettingsPrefix, "controlplane-metrics", "")
}

func setupProcessedFiles() {
	prefixes := testhelpers.CollectorPrefixesFor(configSettingsPrefix, configMapParserPrefix, false)
	paths := testhelpers.CollectorProcessedPaths{
		CollectorSettingsEnvVarPath:      &collectorSettingsEnvVarPath,
		DefaultSettingsEnvVarPath:        &defaultSettingsEnvVarPath,
		ConfigMapKeepListEnvVarPath:      &configMapKeepListEnvVarPath,
		ScrapeConfigDefinitionPathPrefix: &scrapeConfigDefinitionPathPrefix,
		MergedDefaultConfigPath:          &mergedDefaultConfigPath,
		OpentelemetryMetricsEnvVarPath:   &opentelemetryMetricsEnvVarPath,
	}

	if err := testhelpers.SetupCollectorProcessedFiles(prefixes, paths); err != nil {
		panic(err)
	}
}
