package configmapsettings

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
		testhelpers.UnsetEnvVars(testhelpers.DefaultCollectorEnvVarKeys())
		configSettingsPrefix = "/tmp/settings/"
		configMapParserPrefix = "/tmp/configmapparser/"
		configMapMountPath = ""
		os.RemoveAll(configSettingsPrefix)
		os.RemoveAll(configMapParserPrefix)
		_ = os.MkdirAll(configSettingsPrefix, 0755)
		// Save a copy of the original DefaultScrapeJobs for restoring later
		originalDefaultScrapeJobs = make(map[string]shared.DefaultScrapeJob)
		for key, job := range shared.DefaultScrapeJobs {
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
			shared.DefaultScrapeJobs[key] = &job
		}
		os.RemoveAll("../../../configmapparser/default-prom-configs/test/")
	})
}

func setupConfigFiles(defaultPath bool) {
	prefixes := testhelpers.CollectorPrefixesFor(configSettingsPrefix, configMapParserPrefix, true)
	paths := testhelpers.CollectorConfigPaths{
		ConfigMapDebugMountPath:            &configMapDebugMountPath,
		ReplicaSetCollectorConfig:          &replicaSetCollectorConfig,
		DefaultSettingsMountPath:           &defaultSettingsMountPath,
		ConfigMapMountPathForPodAnnotation: &configMapMountPathForPodAnnotation,
		CollectorSettingsMountPath:         &collectorSettingsMountPath,
		ConfigMapKeepListMountPath:         &configMapKeepListMountPath,
		ConfigMapScrapeIntervalMountPath:   &configMapScrapeIntervalMountPath,
		ConfigMapOpentelemetryMetricsPath:  &configMapOpentelemetryMetricsMountPath,
	}

	if err := testhelpers.SetupCollectorConfigFiles(defaultPath, prefixes, paths); err != nil {
		panic(err)
	}
}

func setupProcessedFiles() {
	prefixes := testhelpers.CollectorPrefixesFor(configSettingsPrefix, configMapParserPrefix, true)
	paths := testhelpers.CollectorProcessedPaths{
		PodAnnotationEnvVarPath:          &podAnnotationEnvVarPath,
		CollectorSettingsEnvVarPath:      &collectorSettingsEnvVarPath,
		DefaultSettingsEnvVarPath:        &defaultSettingsEnvVarPath,
		DebugModeEnvVarPath:              &debugModeEnvVarPath,
		ConfigMapKeepListEnvVarPath:      &configMapKeepListEnvVarPath,
		ScrapeIntervalEnvVarPath:         &scrapeIntervalEnvVarPath,
		OpentelemetryMetricsEnvVarPath:   &opentelemetryMetricsEnvVarPath,
		KsmConfigEnvVarPath:              &ksmConfigEnvVarPath,
		ScrapeConfigDefinitionPathPrefix: &scrapeConfigDefinitionPathPrefix,
		MergedDefaultConfigPath:          &mergedDefaultConfigPath,
		PromMergedConfigPath:             &promMergedConfigPath,
		RegexHashFile:                    &regexHashFile,
		IntervalHashFile:                 &intervalHashFile,
	}

	if err := testhelpers.SetupCollectorProcessedFiles(prefixes, paths); err != nil {
		panic(err)
	}
}

var _ = Describe("Configmapparser", Ordered, func() {
	setupTest()

	Context("when the settings configmap does not exist", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
			isDefaultConfig = true
			useConfigFiles = false
		})

		It("should process the config with defaults for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
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
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
			isDefaultConfig = true
			useConfigFiles = true
		})
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"CONFIGMAP_VERSION": "v1",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"CONFIGMAP_VERSION": "v1",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())

			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "true",
				"CONFIGMAP_VERSION":                            "v1",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			isDefaultConfig := true

			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when the settings configmap sections exist and are not default", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, extraEnvVars map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars = map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"DEBUG_MODE_ENABLED":                               "true",
				"AZMON_KSM_CONFIG_ENABLED":                         "true",
				"CONFIGMAP_VERSION":                                "v1",
			}

			scrapeOverrides := testhelpers.CloneJobEnabledStates(shared.DefaultScrapeJobs)
			for _, job := range []string{
				"podannotations",
				"kubelet",
				"coredns",
				"cadvisor",
				"kubeproxy",
				"apiserver",
				"kubestate",
				"nodeexporter",
				"windowsexporter",
				"windowskubeproxy",
				"kappiebasic",
				"networkobservabilityRetina",
				"networkobservabilityHubble",
				"networkobservabilityCilium",
				"acstor-capacity-provisioner",
				"local-csi-driver",
				"acstor-metrics-exporter",
				"dcgmexporter",
				"prometheuscollectorhealth",
			} {
				scrapeOverrides[job] = true
			}

			overrides := testhelpers.BuildEnvVarOverrides(scrapeOverrides, func(jobName string) string {
				return testhelpers.BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
			})
			for key, value := range overrides {
				extraEnvVars[key] = value
			}

			for k, v := range extraEnvVars {
				expectedEnvVars[k] = v
			}

			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "test.*|test2", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "15s")

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			configMapMountPathForPodAnnotation = testhelpers.MustCreateTempFile(configSettingsPrefix, "pod-annotation-based-scraping", `podannotationnamespaceregex = ".*|value"`)
			collectorSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", `cluster_alias = "alias"`)
			ksmConfigYaml := "resources:\n  secrets: {}\n  configmaps: {}\nlabels_allow_list:\n  pods:\n    - app8\nannotations_allow_list:\n  namespaces:\n    - kube-system\n    - default\n"
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "ksm-config", ksmConfigYaml)
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", `
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
				dcgmexporter = true
				prometheuscollectorhealth = true
			`)
			configMapDebugMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "debug-mode", `enabled = true`)
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
    			dcgmexporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
				minimalingestionprofile = true
			`)
			configMapScrapeIntervalMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", `
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
    			dcgmexporter = "15s"
				ztunnel = "15s"
				istio-cni = "15s"
				waypoint-proxy = "15s"
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-linux-rs.yaml", "")
		})

		It("should process the config for the Linux Daemonset", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-linux-ds.yaml", "")
		})

		It("should process the config for the Windows Daemonset", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-windows-ds.yaml", "")
		})
	})

	Context("when some of the configmap sections exist but not all", func() {

		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"DEBUG_MODE_ENABLED":                               "true",
				"CONFIGMAP_VERSION":                                "v1",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedEnvVars[testhelpers.BuildEnvVarName("podannotations", "_SCRAPING_ENABLED")] = "true"
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "test.*|test2", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "15s")

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configMapMountPathForPodAnnotation = testhelpers.MustCreateTempFile(configSettingsPrefix, "pod-annotation-based-scraping", `podannotationnamespaceregex = ".*|value"`)
			configMapDebugMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "debug-mode", `enabled = true`)
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
    			dcgmexporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
				minimalingestionprofile = true
			`)
			configMapScrapeIntervalMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", `
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
    			dcgmexporter = "15s"
				ztunnel = "15s"
				istio-cni = "15s"
				waypoint-proxy = "15s"
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})
		It("should process the config with some sections missing for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/not-all-sections-linux-rs.yaml", "")
		})

		It("should process the config with some sections missing for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/not-all-sections-linux-ds.yaml", "")
		})

		It("should process the config with some sections missing for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
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
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED": "",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "true",
				"DEBUG_MODE_ENABLED":                               "false",
				"CONFIGMAP_VERSION":                                "v1",
			}

			scrapeOverrides := testhelpers.CloneJobEnabledStates(shared.DefaultScrapeJobs)
			for jobName := range scrapeOverrides {
				scrapeOverrides[jobName] = false
			}

			overrides := testhelpers.BuildEnvVarOverrides(scrapeOverrides, func(jobName string) string {
				return testhelpers.BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
			})
			for key, value := range overrides {
				extraEnvVars[key] = value
			}

			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
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
				local-csi-driver = false
				acstor-metrics-exporter = false
				dcgmexporter = false
				prometheuscollectorhealth = false
			`)
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config with no scrape jobs enabled for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with no scrape jobs enabled for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with no scrape jobs enabled for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
			expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})
	})

	Context("when minimal ingestion is false and has keeplist regex values", func() {
		var expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var useConfigFiles, isDefaultConfig bool

		BeforeEach(func() {
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, false, "test.*|test2", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "")
			configMapMountPathForPodAnnotation = testhelpers.MustCreateTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
			collectorSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
			configMapDebugMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "debug-mode", "")
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
				local-csi-driver = "test.*|test2"
    			acstor-metrics-exporter = "test.*|test2"
    			dcgmexporter = "test.*|test2"
				prometheuscollectorhealth = "test.*|test2"
				ztunnel = "test.*|test2"
				istio-cni = "test.*|test2"
				waypoint-proxy = "test.*|test2"
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", ``)
			useConfigFiles = false
			isDefaultConfig = false
		})

		It("should process the scrape jobs enabled for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-rs.yaml", "")
		})

		It("should process the scrape jobs enabled for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-linux-ds.yaml", "")
		})

		It("should process the scrape jobs enabled for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when minimal ingestion is false and has no keeplist regex values", func() {
		var expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var useConfigFiles, isDefaultConfig bool
		BeforeEach(func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, false, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v1")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			configMapMountPathForPodAnnotation = testhelpers.MustCreateTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
			collectorSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
			defaultSettingsMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
			configMapDebugMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "debug-mode", "")
			configMapKeepListMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", `
				minimalingestionprofile = false
			`)
			configMapScrapeIntervalMountPath = testhelpers.MustCreateTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", ``)
			useConfigFiles = false
			isDefaultConfig = false
		})

		It("should process the scrape jobs enabled for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-no-keeplist-rs.yaml", "")
		})

		It("should process the scrape jobs enabled for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-minimal-no-keeplist-ds.yaml", "")
		})

		It("should process the scrape jobs enabled for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, nil, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/no-scrape-jobs-linux-rs.yaml", "")
		})
	})

	Context("when the custom configmap exists", Label("custom-config"), func() {
		Context("and the settings configmap sections do not exist", func() {
			var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
			var isDefaultConfig, useConfigFiles bool
			BeforeEach(func() {
				expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
				expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
				expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
				isDefaultConfig = true
				useConfigFiles = false
			})
			It("should process the config with defaults for the Linux ReplicaSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-and-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Linux DaemonSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-and-defaults-ds.yaml")
			})
			It("should process the config with defaults for the Windows DaemonSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
				expectedEnvVars := testhelpers.DefaultManagedPrometheusEnvVars()
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
				expectedEnvVars := testhelpers.DefaultManagedPrometheusEnvVars()
				extraEnvVars := map[string]string{
					"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v1",
					"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
					"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
					"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED": "",
					"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "true",
					"DEBUG_MODE_ENABLED":                               "false",
					"CONFIGMAP_VERSION":                                "v1",
				}

				scrapeOverrides := testhelpers.CloneJobEnabledStates(shared.DefaultScrapeJobs)
				for jobName := range scrapeOverrides {
					scrapeOverrides[jobName] = false
				}

				overrides := testhelpers.BuildEnvVarOverrides(scrapeOverrides, func(jobName string) string {
					return testhelpers.BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
				})
				for key, value := range overrides {
					extraEnvVars[key] = value
				}

				for key, value := range extraEnvVars {
					expectedEnvVars[key] = value
				}
				expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
				expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")

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
					local-csi-driver = false
					acstor-metrics-exporter = false
					dcgmexporter = false
					prometheuscollectorhealth = false
				`)
				isDefaultConfig = false
				useConfigFiles = false
				configMapMountPath = "./testdata/custom-prometheus-config.yaml"
			})
			It("should process the config with defaults for the Linux ReplicaSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Linux DaemonSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
			It("should process the config with defaults for the Windows DaemonSet", func() {
				Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
				expectedContentsFilePath := "./testdata/no-scrape-jobs-linux-rs.yaml"

				checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "./testdata/custom-prometheus-config-no-defaults-rs.yaml")
			})
		})
	})

	Context("when the settings configmap uses v2 and the sections exist but are empty", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":       "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":         "ver1",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING": "true",
				"CONFIGMAP_VERSION":                    "v2",
			}
			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
			isDefaultConfig = true
			useConfigFiles = true

			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
		})
		It("should process the config with defaults for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/default-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with defaults for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
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
			expectedEnvVars = testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":                   "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                     "ver1",
				"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX": "'.*|value'",
				"AZMON_CLUSTER_LABEL":                              "alias",
				"AZMON_CLUSTER_ALIAS":                              "alias",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":             "true",
				"AZMON_PROMETHEUS_PODANNOTATIONS_SCRAPING_ENABLED": "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":     "false",
				"DEBUG_MODE_ENABLED":                               "true",
				"AZMON_KSM_CONFIG_ENABLED":                         "true",
				"CONFIGMAP_VERSION":                                "v2",
			}

			scrapeOverrides := testhelpers.CloneJobEnabledStates(shared.DefaultScrapeJobs)
			for _, job := range []string{
				"kubelet",
				"coredns",
				"cadvisor",
				"kubeproxy",
				"apiserver",
				"kubestate",
				"nodeexporter",
				"windowsexporter",
				"windowskubeproxy",
				"kappiebasic",
				"networkobservabilityRetina",
				"networkobservabilityHubble",
				"networkobservabilityCilium",
				"podannotations",
				"acstor-capacity-provisioner",
				"local-csi-driver",
				"acstor-metrics-exporter",
				"dcgmexporter",
			} {
				scrapeOverrides[job] = true
			}
			scrapeOverrides["prometheuscollectorhealth"] = false

			overrides := testhelpers.BuildEnvVarOverrides(scrapeOverrides, func(jobName string) string {
				return testhelpers.BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
			})
			for key, value := range overrides {
				extraEnvVars[key] = value
			}

			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, true, "test.*|test2", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "15s")
			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
			_ = testhelpers.MustCreateTempFile(configSettingsPrefix, "prometheus-collector-settings", `
    			cluster_alias = "alias"
   				debug-mode = true
    			https_config = true
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
					local-csi-driver = true
					acstor-metrics-exporter = true
					dcgmexporter = true
					prometheuscollectorhealth = false
				pod-annotation-based-scraping: |-
					podannotationnamespaceregex = ".*|value"
				ksm-config: |-
					resources:
					  secrets: {}
					  configmaps: {}
					labels_allow_list:
					  pods:
					    - app8
					annotations_allow_list:
					  namespaces:
					    - kube-system
					    - default
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
					dcgmexporter = "test.*|test2"
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
					dcgmexporter = "15s"
					ztunnel = "15s"
					istio-cni = "15s"
					waypoint-proxy = "15s"
			`)
			configMapMountPath = "./testdata/advanced-linux-rs.yaml"
			isDefaultConfig = false
			useConfigFiles = false
		})

		It("should process the config with custom settings for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/advanced-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with custom settings for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/advanced-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the config with custom settings for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, "./testdata/advanced-windows-ds.yaml", "")
		})
	})

	Context("when the settings configmap uses v2, minimal ingestion is false, and not all sections are present", func() {
		var expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap map[string]string
		var isDefaultConfig, useConfigFiles bool
		BeforeEach(func() {
			expectedEnvVars := testhelpers.DefaultManagedPrometheusEnvVars()
			extraEnvVars := map[string]string{
				"AZMON_AGENT_CFG_SCHEMA_VERSION":               "v2",
				"AZMON_AGENT_CFG_FILE_VERSION":                 "ver1",
				"AZMON_OPERATOR_ENABLED_CHART_SETTING":         "true",
				"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED": "false",
				"DEBUG_MODE_ENABLED":                           "false",
				"CONFIGMAP_VERSION":                            "v2",
			}

			scrapeOverrides := testhelpers.CloneJobEnabledStates(shared.DefaultScrapeJobs)
			for _, job := range []string{
				"kubelet",
				"coredns",
				"cadvisor",
				"kubeproxy",
				"apiserver",
				"kubestate",
				"nodeexporter",
				"windowsexporter",
				"windowskubeproxy",
				"kappiebasic",
				"networkobservabilityRetina",
				"networkobservabilityHubble",
				"networkobservabilityCilium",
				"acstor-capacity-provisioner",
				"local-csi-driver",
				"acstor-metrics-exporter",
				"dcgmexporter",
			} {
				scrapeOverrides[job] = true
			}
			scrapeOverrides["prometheuscollectorhealth"] = false

			overrides := testhelpers.BuildEnvVarOverrides(scrapeOverrides, func(jobName string) string {
				return testhelpers.BuildEnvVarName(jobName, "_SCRAPING_ENABLED")
			})
			for key, value := range overrides {
				extraEnvVars[key] = value
			}

			for key, value := range extraEnvVars {
				expectedEnvVars[key] = value
			}
			expectedKeepListHashMap = testhelpers.ExpectedKeepListMap(shared.DefaultScrapeJobs, false, "", true)
			expectedScrapeIntervalHashMap = testhelpers.ExpectedScrapeIntervalMap(shared.DefaultScrapeJobs, "")
			schemaVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "schema-version", "v2")
			fmt.Println("Schema version file created at:", schemaVersionFile)
			configVersionFile = testhelpers.MustCreateTempFile(configSettingsPrefix, "config-version", "ver1")
			fmt.Println("Config version file created at:", configVersionFile)
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
					local-csi-driver = true
					acstor-metrics-exporter = true
					dcgmexporter = true
					prometheuscollectorhealth = false
				minimal-ingestion-profile: |-
					enabled = false
			`)
			configMapMountPath = "./testdata/advanced-linux-rs.yaml"
			isDefaultConfig = false
			useConfigFiles = false
		})
		It("should process the settings for the Linux ReplicaSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.ConfigReaderSidecar, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-rs.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the settings for the Linux DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Linux)).To(Succeed())
			expectedContentsFilePath := "./testdata/advanced-no-minimal-linux-ds.yaml"
			checkResults(useConfigFiles, isDefaultConfig, expectedEnvVars, expectedKeepListHashMap, expectedScrapeIntervalHashMap, expectedContentsFilePath, "")
		})

		It("should process the settings for the Windows DaemonSet", func() {
			Expect(testhelpers.SetManagedPrometheusEnvVars(shared.ControllerType.DaemonSet, shared.OSType.Windows)).To(Succeed())
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

	err := testhelpers.CheckEnvVars(expectedEnvVars)
	Expect(err).NotTo(HaveOccurred())

	keepListHash, err := testhelpers.ReadYAMLStringMap(configMapKeepListEnvVarPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(keepListHash).To(BeComparableTo(expectedKeepListHashMap))

	scrapeIntervalHash, err := testhelpers.ReadYAMLStringMap(scrapeIntervalEnvVarPath)
	Expect(err).NotTo(HaveOccurred())
	Expect(scrapeIntervalHash).To(BeComparableTo(expectedScrapeIntervalHashMap))

	mergedFileContents, err := os.ReadFile(mergedDefaultConfigPath)
	Expect(err).NotTo(HaveOccurred())
	fmt.Println(string(mergedFileContents))
	expectedDefaultFileContents, err := os.ReadFile(expectedDefaultContentsFilePath)
	Expect(err).NotTo(HaveOccurred())

	var customMergedConfigFileContents, expectedCustomMergedConfigFileContents []byte
	if expectedMergedContentsFilePath != "" {
		customMergedConfigFileContents, err = os.ReadFile(promMergedConfigPath)
		Expect(err).NotTo(HaveOccurred())
		fmt.Println(string(customMergedConfigFileContents))

		expectedCustomMergedConfigFileContents, err = os.ReadFile(expectedMergedContentsFilePath)
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
