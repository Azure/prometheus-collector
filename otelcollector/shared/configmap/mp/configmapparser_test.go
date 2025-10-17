package configmapsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"

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

// PlatformConfig represents a platform/controller type combination
type PlatformConfig struct {
	ControllerType string
	OSType         string
	Name           string
}

// TestScenario defines a complete test scenario with setup and expectations
type TestScenario struct {
	Name                      string
	Description               string
	UseConfigFiles            bool
	IsDefaultConfig           bool
	ConfigMapMountPath        string
	ExtraEnvVars              map[string]string
	KeepListRegex             string
	ScrapeInterval            string
	MinimalIngestion          *bool // nil means use default
	SchemaVersion             string
	ConfigVersion             string
	ConfigMapContents         map[string]string
	ExpectedConfigPaths       map[string]string // platform name -> expected config file path
	ExpectedMergedConfigPaths map[string]string // platform name -> expected merged config file path
}

// Platform definitions for reuse across tests
var (
	LinuxReplicaSet = PlatformConfig{
		ControllerType: shared.ControllerType.ConfigReaderSidecar,
		OSType:         shared.OSType.Linux,
		Name:           "Linux ReplicaSet",
	}
	LinuxDaemonSet = PlatformConfig{
		ControllerType: shared.ControllerType.DaemonSet,
		OSType:         shared.OSType.Linux,
		Name:           "Linux DaemonSet",
	}
	WindowsDaemonSet = PlatformConfig{
		ControllerType: shared.ControllerType.DaemonSet,
		OSType:         shared.OSType.Windows,
		Name:           "Windows DaemonSet",
	}

	AllPlatforms = []PlatformConfig{LinuxReplicaSet, LinuxDaemonSet, WindowsDaemonSet}
)

func setupTest() {
	var originalDefaultScrapeJobs map[string]shared.DefaultScrapeJob
	BeforeEach(func() {
		cleanupEnvVars()
		configSettingsPrefix = "/tmp/settings/"
		configMapMountPath = ""
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
		os.RemoveAll("../../../configmapparser/default-prom-configs/test/")
	})
}

var _ = Describe("Configmapparser", Ordered, func() {
	setupTest()

	Context("when the settings configmap does not exist", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			expectedKeepListHashMap = getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
			isDefaultConfig = true
			useConfigFiles = true
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			extraEnvVars := map[string]string{
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist but are empty", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			expectedKeepListHashMap = getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
			isDefaultConfig = true
			useConfigFiles = true
		})
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)

			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap := getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap := getExpectedScrapeIntervalMap("")
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(false, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, extraEnvVars map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			extraEnvVars = map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
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
				"DEBUG_MODE_ENABLED": "true",
			}
			for k, v := range extraEnvVars {
				expectedEnvVars[k] = v
			}

			expectedKeepListHashMap = getExpectedKeepListMap(true, "test.*|test2")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("15s")

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
				acstor-capacity-provisioner = true
				local-csi-driver = true
				acstor-metrics-exporter = true
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
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
				local-csi-driver = "15s"
    			acstor-metrics-exporter = "15s"
				ztunnel = "15s"
				istio-cni = "15s"
				waypoint-proxy = "15s"
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-linux-rs.yaml", "")
		})

		It("should process the config for the Linux Daemonset", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-linux-ds.yaml", "")
		})

		It("should process the config for the Windows Daemonset", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-windows-ds.yaml", "")
		})
	})

	Context("when some of the configmap sections exist but not all", func() {

		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v1",
				//"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED": "true",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				"DEBUG_MODE_ENABLED":                           "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = getExpectedKeepListMap(true, "test.*|test2")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("15s")

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", `podannotationnamespaceregex = ".*|value"`)
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
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
				local-csi-driver = "15s"
    			acstor-metrics-exporter = "15s"
				ztunnel = "15s"
				istio-cni = "15s"
				waypoint-proxy = "15s"
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})
		It("should process the config with some sections missing for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/not-all-sections-linux-rs.yaml", "")
		})

		It("should process the config with some sections missing for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/not-all-sections-linux-ds.yaml", "")
		})

		It("should process the config with some sections missing for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			extraEnvVars := map[string]string{
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when the configmap sections exist but all scrape jobs are false", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                 "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                   "ver1",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":           "true",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":      "false",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":     "false",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":    "false",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":    "false",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":    "false",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED": "false",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "false",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "false",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "true",
				"DEBUG_MODE_ENABLED": "false",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
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
				local-csi-driver = false
				acstor-metrics-exporter = false
				prometheuscollectorhealth = false
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config with no scrape jobs enabled for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with no scrape jobs enabled for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with no scrape jobs enabled for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when minimal ingestion is false and has keeplist regex values", func() {
		var expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var useConfigFiles, isDefaultConfig bool

		BeforeEach(func() {
			expectedKeepListHashMap = getExpectedKeepListMap(false, "test.*|test2")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "")
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", ``)
			useConfigFiles = false
			isDefaultConfig = false
		})

		It("should process the scrape jobs enabled for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-rs.yaml", "")
		})

		It("should process the scrape jobs enabled for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-ds.yaml", "")
		})

		It("should process the scrape jobs enabled for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when minimal ingestion is false and has no keeplist regex values", func() {
		var expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var useConfigFiles, isDefaultConfig bool
		BeforeEach(func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedKeepListHashMap = getExpectedKeepListMap(false, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")

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
			useConfigFiles = false
			isDefaultConfig = false
		})

		It("should process the scrape jobs enabled for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-no-keeplist-rs.yaml", "")
		})

		It("should process the scrape jobs enabled for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-no-keeplist-ds.yaml", "")
		})

		It("should process the scrape jobs enabled for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when the custom configmap exists", Label("custom-config"), func() {
		Context("and the settings configmap sections do not exist", func() {
			var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
			var isDefaultConfig, useConfigFiles bool
			BeforeEach(func() {
				expectedEnvVars = getDefaultExpectedEnvVars()
				expectedKeepListHashMap = getExpectedKeepListMap(true, "")
				expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
				isDefaultConfig = true
				useConfigFiles = true
			})
			It("should process the config with defaults for the Linux ReplicaSet", func() {
				setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-and-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Linux DaemonSet", func() {
				setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-and-defaults-ds.yaml")
			})
			It("should process the config with defaults for the Windows DaemonSet", func() {
				setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedEnvVars := getDefaultExpectedEnvVars()
				extraEnvVars := map[string]string{
					"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
				}
				for key, value := range extraEnvVars {
					expectedEnvVars[key] = value
				}
				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "./testdata/custom-prometheus-config-windows-ds.yaml")
			})
		})
		Context("and the settings configmap sections have all default scrape configs set to false", func() {
			var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
			var isDefaultConfig, useConfigFiles bool
			BeforeEach(func() {
				expectedEnvVars := getDefaultExpectedEnvVars()
				extraEnvVars := map[string]string{
					"AZMON_AGENT_CFG_SCHEMA_VERSION":                     "v1",
					"AZMON_AGENT_CFG_FILE_VERSION":                       "ver1",
					"AZMON_OPERATOR_ENABLED_CHART_SETTING":               "true",
					"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":          "false",
					"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":          "false",
					"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":         "false",
					"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":        "false",
					"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":        "false",
					"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":        "false",
					"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED":     "false",
					"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "",
					"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":   "",
					"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "false",
					"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "false",
					//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
					"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "false",
					"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "false",
					"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "false",
					"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "true",
					"DEBUG_MODE_ENABLED": "false",
					//"AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG":                       "false",
					//"CONFIG_VALIDATOR_RUNNING_IN_AGENT":                            "true",
					//"AZMON_USE_DEFAULT_PROMETHEUS_CONFIG":                          "true",
				}
				for key, value := range extraEnvVars {
					expectedEnvVars[key] = value
				}
				expectedKeepListHashMap = getExpectedKeepListMap(true, "")
				expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")

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
					local-csi-driver = false
					acstor-metrics-exporter = false
					prometheuscollectorhealth = false
				`)
				isDefaultConfig = false
				useConfigFiles = false
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
			})
			It("should process the config with defaults for the Linux ReplicaSet", func() {
				setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Linux DaemonSet", func() {
				setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Windows DaemonSet", func() {
				setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
		})
	})

	Context("when the settings configmap uses v2 and the sections exist but are empty", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION": "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":   "ver1",
				// TODO: investigate
				"AZMON_OPERATOR_ENABLED_CHART_SETTING": "false",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = getExpectedKeepListMap(true, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
			isDefaultConfig = true
			useConfigFiles = true

			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
		})
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			extraEnvVars := map[string]string{
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when the settings configmap uses v2 and the sections are not default", Label("v2"), func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
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
				"DEBUG_MODE_ENABLED": "true",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = getExpectedKeepListMap(true, "test.*|test2")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("15s")
			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = createTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
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
				local-csi-driver = true
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
				local-csi-driver = "test.*|test2"
					acstor-metrics-exporter = "test.*|test2"
					prometheuscollectorhealth = "test.*|test2"
					ztunnel = "test.*|test2"
					istio-cni = "test.*|test2"
					waypoint-proxy = "test.*|test2"
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
				local-csi-driver = "15s"
					acstor-metrics-exporter = "15s"
					ztunnel = "15s"
					istio-cni = "15s"
					waypoint-proxy = "15s"
			`)
			configMapMountPath = "./testdata/advanced-linux-rs.yaml"
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config with custom settings for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/advanced-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with custom settings for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/advanced-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with custom settings for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-windows-ds.yaml", "")
		})
	})

	Context("when the settings configmap uses v2, minimal ingestion is false, and not all sections are present", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars := getDefaultExpectedEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                 "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                   "ver1",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":           "true",
				"AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED":      "true",
				"AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED":     "true",
				"AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED":    "true",
				"AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED": "true",
				//"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":  "true",
				"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED": "true",
				//"AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED":                "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
				"DEBUG_MODE_ENABLED": "false",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = getExpectedKeepListMap(false, "")
			expectedScrapeIntervalHashMap = getExpectedScrapeIntervalMap("")
			schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
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
				local-csi-driver = true
					acstor-metrics-exporter = true
					prometheuscollectorhealth = false
				minimal-ingestion-profile: |-
					enabled = false
			`)
			configMapMountPath = "./testdata/advanced-linux-rs.yaml"
			isDefaultConfig = false
			useConfigFiles = false
		})
		It("should process the settings for the Linux ReplicaSet", func() {
			setSetupEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the settings for the Linux DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the settings for the Windows DaemonSet", func() {
			setSetupEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-no-minimal-windows-ds.yaml", "")
		})
	})
})

func checkResults(useConfigFiles bool, isDefaultConfig bool, expectedEnvVars map[string]string, expectedKeepListHashMap map[string]string, expectedScrapeIntervalHashMap map[string]string, expectedDefaultContentsFilePath string, expectedMergedContentsFilePath string) {
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
	expectedDefaultFileContents, err := ioutil.ReadFile(expectedDefaultContentsFilePath)
	Expect(err).NotTo(HaveOccurred())

	var customMergedConfigFileContents, expectedCustomMergedConfigFileContents []byte
	if expectedMergedContentsFilePath != "" {
		customMergedConfigFileContents, err = ioutil.ReadFile(promMergedConfigPath)
		Expect(err).NotTo(HaveOccurred())
		fmt.Println(string(customMergedConfigFileContents))

		expectedCustomMergedConfigFileContents, err = ioutil.ReadFile(expectedMergedContentsFilePath)
		Expect(err).NotTo(HaveOccurred())
	}

	var mergedConfig, expectedConfig, customMergedConfig, expectedCustomConfig map[string]interface{}

	err = yaml.Unmarshal(mergedFileContents, &mergedConfig)
	Expect(err).NotTo(HaveOccurred())

	err = yaml.Unmarshal(expectedDefaultFileContents, &expectedConfig)
	Expect(err).NotTo(HaveOccurred())

	err = yaml.Unmarshal(customMergedConfigFileContents, &customMergedConfig)
	Expect(err).NotTo(HaveOccurred())

	err = yaml.Unmarshal(expectedCustomMergedConfigFileContents, &expectedCustomConfig)
	Expect(err).NotTo(HaveOccurred())

	// Order the scrape_configs by job_name for consistent comparison
	for _, config := range []map[string]interface{}{mergedConfig, expectedConfig, customMergedConfig, expectedCustomConfig} {
		if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
			sort.Slice(scrapeConfigs, func(i, j int) bool {
				iConfig := scrapeConfigs[i].(map[interface{}]interface{})
				jConfig := scrapeConfigs[j].(map[interface{}]interface{})
				return iConfig["job_name"].(string) < jConfig["job_name"].(string)
			})
			config["scrape_configs"] = scrapeConfigs
		}
	}

	// Use BeEquivalentTo which compares content without requiring same order
	// Only compare default configs when there's no custom config expected
	if expectedMergedContentsFilePath == "" {
		Expect(mergedConfig).To(BeEquivalentTo(expectedConfig), "Prometheus config content doesn't match")
	} else {
		// When custom config is expected, compare the custom merged config instead
		Expect(customMergedConfig).To(BeEquivalentTo(expectedCustomConfig), "Expected Custom Prometheus config content doesn't match")
	}
}
