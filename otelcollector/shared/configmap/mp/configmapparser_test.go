package configmapsettings

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

type collectorTestCase struct {
	controllerType string
	osType         string
	containerType  string
	golden         string
	defaultJobs    []string
	customJobs     []string
}

var (
	linuxReplica = collectorTestCase{
		controllerType: "ReplicaSet",
		osType:         "linux",
		containerType:  "ConfigReaderSidecar",
		golden:         "default-linux-rs.yaml",
		defaultJobs:    []string{"kube-state-metrics", "acstor-capacity-provisioner", "acstor-metrics-exporter", "local-csi-driver", "dcgm-exporter"},
		customJobs:     []string{"kube-dns", "kube-proxy", "kube-apiserver", "kube-state-metrics", "prometheus_collector_health", "kubernetes-pods", "acstor-capacity-provisioner", "acstor-metrics-exporter", "local-csi-driver", "dcgm-exporter"},
	}
	linuxDaemonset = collectorTestCase{
		controllerType: "DaemonSet",
		osType:         "linux",
		golden:         "default-linux-ds.yaml",
		defaultJobs:    []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium"},
		customJobs:     []string{"kubelet", "cadvisor", "node", "kappie-basic", "networkobservability-retina", "networkobservability-hubble", "networkobservability-cilium", "prometheus_collector_health"},
	}
	windowsDaemonset = collectorTestCase{
		controllerType: "DaemonSet",
		osType:         "windows",
		defaultJobs:    []string{"kubelet", "kappie-basic", "networkobservability-retina"},
		customJobs:     []string{"kubelet", "kappie-basic", "networkobservability-retina", "prometheus_collector_health", "windows-exporter", "kube-proxy-windows"},
	}
)

var _ = Describe("Configmapparser settings and scrape configuration", func() {
	var configDir, fixtureDir string

	BeforeEach(func() {
		isolateConfigmapTestEnvironment()
		testDir := GinkgoT().TempDir()
		sourceDir, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		fixtureDir = filepath.Join(sourceDir, "testdata")
		configDir = filepath.Join(testDir, "settings")
		outputDir := filepath.Join(testDir, "generated")
		templateDir := filepath.Join(testDir, "templates")
		homeDir := filepath.Join(testDir, "home")
		for _, dir := range []string{outputDir, templateDir, homeDir} {
			Expect(os.MkdirAll(dir, 0700)).To(Succeed())
		}
		GinkgoT().Setenv("HOME", homeDir)
		GinkgoT().Setenv("USERPROFILE", homeDir)

		for path, name := range map[*string]string{
			&schemaVersionFile:                      "schema-version",
			&configVersionFile:                      "config-version",
			&configMapDebugMountPath:                "debug-mode",
			&configMapOpentelemetryMetricsMountPath: "opentelemetry-metrics",
			&defaultSettingsMountPath:               "default-scrape-settings-enabled",
			&defaultSettingsMountPathv2:             "default-targets-scrape-enabled",
			&configMapMountPathForPodAnnotation:     "pod-annotation-based-scraping",
			&collectorSettingsMountPath:             "prometheus-collector-settings",
			&configMapKeepListMountPath:             "default-targets-metrics-keep-list",
			&configMapScrapeIntervalMountPath:       "default-targets-scrape-interval-settings",
		} {
			setTestPath(path, filepath.Join(configDir, name))
		}
		setTestPath(&configMapMountPath, filepath.Join(configDir, "prometheus", "prometheus-config"))
		for path, name := range map[*string]string{
			&replicaSetCollectorConfig:      "collector-config-replicaset.yml",
			&podAnnotationEnvVarPath:        "pod-annotation-env",
			&collectorSettingsEnvVarPath:    "collector-settings-env",
			&defaultSettingsEnvVarPath:      "default-settings-env",
			&debugModeEnvVarPath:            "debug-mode-env",
			&opentelemetryMetricsEnvVarPath: "opentelemetry-metrics-env",
			&ksmConfigEnvVarPath:            "ksm-config-env",
			&configMapKeepListEnvVarPath:    "keep-list-env",
			&scrapeIntervalEnvVarPath:       "scrape-interval-env",
			&promMergedConfigPath:           "promMergedConfig.yml",
			&mergedDefaultConfigPath:        "defaultsMergedConfig.yml",
		} {
			setTestPath(path, filepath.Join(outputDir, name))
		}
		setTestPath(&regexHashFile, configMapKeepListEnvVarPath)
		setTestPath(&intervalHashFile, scrapeIntervalEnvVarPath)
		setTestPath(&defaultPromConfigPathPrefix, templateDir+string(os.PathSeparator))

		copyTestFile(filepath.Join(fixtureDir, "collector-config-replicaset.yml"), replicaSetCollectorConfig)
		sourceTemplates := filepath.Join(sourceDir, "..", "..", "..", "configmapparser", "default-prom-configs")
		templates, err := os.ReadDir(sourceTemplates)
		Expect(err).NotTo(HaveOccurred())
		Expect(templates).NotTo(BeEmpty())
		for _, template := range templates {
			if !template.IsDir() {
				copyTestFile(filepath.Join(sourceTemplates, template.Name()), filepath.Join(templateDir, template.Name()))
			}
		}

		oldRegexHash, oldIntervalHash, oldMergedConfigs := regexHash, intervalHash, mergedDefaultConfigs
		DeferCleanup(func() {
			regexHash, intervalHash, mergedDefaultConfigs = oldRegexHash, oldIntervalHash, oldMergedConfigs
		})
		regexHash, intervalHash = make(map[string]string), make(map[string]string)
		mergedDefaultConfigs = nil
		for _, value := range []*string{
			&configSchemaVersion, &kubeletRegex, &coreDNSRegex, &cAdvisorRegex, &kubeProxyRegex,
			&apiserverRegex, &kubeStateRegex, &nodeExporterRegex, &kappieBasicRegex,
			&windowsExporterRegex, &windowsKubeProxyRegex, &networkobservabilityRetinaRegex,
			&networkobservabilityHubbleRegex, &networkobservabilityCiliumRegex, &podAnnotationsRegex,
			&acstorCapacityProvisionerRegex, &acstorMetricsExporterRegex, &localCSIDriverRegex,
			&ztunnelRegex, &istioCniRegex, &dcgmExporterRegex, &controlplaneIstioRegex,
		} {
			original := *value
			DeferCleanup(func() { *value = original })
			*value = ""
		}

		// The merger writes working copies of the default templates in its CWD.
		Expect(os.Chdir(outputDir)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chdir(sourceDir)).To(Succeed()) })
		setEnvVars(map[string]string{
			"AZMON_OPERATOR_ENABLED":   "true",
			"MODE":                     "advanced",
			"WINMODE":                  "advanced",
			"KUBE_STATE_NAME":          "ama-metrics-ksm",
			"POD_NAMESPACE":            "kube-system",
			"MAC":                      "true",
			"NODE_IP":                  "10.0.0.1",
			"NODE_NAME":                "test-node",
			"NODE_EXPORTER_TARGETPORT": "9100",
		})
	})

	setCollectorType := func(collector collectorTestCase) {
		setEnvVars(map[string]string{
			"CONTROLLER_TYPE": collector.controllerType,
			"OS_TYPE":         collector.osType,
			"CONTAINER_TYPE":  collector.containerType,
		})
	}

	writeSettings := func(sections map[string]string) {
		Expect(os.MkdirAll(configDir, 0700)).To(Succeed())
		for name, content := range sections {
			Expect(os.WriteFile(filepath.Join(configDir, name), []byte(content), 0600)).To(Succeed())
		}
	}

	DescribeTable("should process missing or empty settings with defaults",
		func(collector collectorTestCase, settingsPresent bool) {
			setCollectorType(collector)
			if settingsPresent {
				writeSettings(map[string]string{
					"schema-version":                           "v1",
					"config-version":                           "ver1",
					"pod-annotation-based-scraping":            "",
					"prometheus-collector-settings":            "",
					"default-scrape-settings-enabled":          "",
					"debug-mode":                               "",
					"opentelemetry-metrics":                    "",
					"default-targets-metrics-keep-list":        "",
					"default-targets-scrape-interval-settings": "",
				})
			}

			processConfigmapSettings(configDir)

			checkEnvVars(defaultTestEnv(settingsPresent))
			checkHashMaps(configMapKeepListEnvVarPath, defaultTestKeepLists())
			checkHashMaps(scrapeIntervalEnvVarPath, defaultTestIntervals())
			jobs := checkScrapeJobs(collector.defaultJobs)
			for name, job := range jobs {
				Expect(job.ScrapeInterval).To(Equal("30s"), name)
			}
			if collector.golden != "" {
				actual, err := os.ReadFile(mergedDefaultConfigPath)
				Expect(err).NotTo(HaveOccurred())
				expected, err := os.ReadFile(filepath.Join(fixtureDir, collector.golden))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(actual)).To(MatchYAML(string(expected)))
			}
			Expect(os.Getenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT")).To(BeEmpty())
		},
		Entry("missing settings, Linux ReplicaSet", linuxReplica, false),
		Entry("missing settings, Linux DaemonSet", linuxDaemonset, false),
		Entry("missing settings, Windows DaemonSet", windowsDaemonset, false),
		Entry("empty settings, Linux ReplicaSet", linuxReplica, true),
		Entry("empty settings, Linux DaemonSet", linuxDaemonset, true),
		Entry("empty settings, Windows DaemonSet", windowsDaemonset, true),
	)

	DescribeTable("should process non-default settings",
		func(collector collectorTestCase) {
			setCollectorType(collector)
			writeSettings(map[string]string{
				"schema-version":                "v1",
				"config-version":                "ver1",
				"pod-annotation-based-scraping": `podannotationnamespaceregex = ".*|value"`,
				"prometheus-collector-settings": `cluster_alias = "alias"`,
				"debug-mode":                    "enabled = true",
				"default-scrape-settings-enabled": `
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
					prometheuscollectorhealth = true`,
				"default-targets-metrics-keep-list":        testKeepListSettings("test.*|test2", true),
				"default-targets-scrape-interval-settings": testIntervalSettings("15s"),
			})

			processConfigmapSettings(configDir)

			env := defaultTestEnv(true)
			for key := range env {
				if strings.HasPrefix(key, "AZMON_PROMETHEUS_") && strings.HasSuffix(key, "_SCRAPING_ENABLED") && key != "AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED" && key != "AZMON_PROMETHEUS_ZTUNNEL_SCRAPING_ENABLED" && key != "AZMON_PROMETHEUS_ISTIOCNI_SCRAPING_ENABLED" {
					env[key] = "true"
				}
			}
			env["AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"] = "'.*|value'"
			env["AZMON_CLUSTER_LABEL"], env["AZMON_CLUSTER_ALIAS"] = "alias", "alias"
			env["DEBUG_MODE_ENABLED"] = "true"
			checkEnvVars(env)

			keepLists := defaultTestKeepLists()
			for _, key := range customKeepListKeys {
				keepLists[key] = "test.*|test2" + keepLists[key]
			}
			checkHashMaps(configMapKeepListEnvVarPath, keepLists)
			intervals := defaultTestIntervals()
			for _, key := range customIntervalKeys {
				intervals[key] = "15s"
			}
			checkHashMaps(scrapeIntervalEnvVarPath, intervals)
			jobs := checkScrapeJobs(collector.customJobs)
			for name, job := range jobs {
				expectedInterval := "15s"
				switch name {
				case "acstor-capacity-provisioner", "acstor-metrics-exporter", "local-csi-driver", "dcgm-exporter":
					expectedInterval = "30s"
				}
				Expect(job.ScrapeInterval).To(Equal(expectedInterval), name)
			}
			replicaConfig, err := os.ReadFile(replicaSetCollectorConfig)
			Expect(err).NotTo(HaveOccurred())
			if collector.controllerType == "ReplicaSet" {
				var config map[string]interface{}
				Expect(yaml.Unmarshal(replicaConfig, &config)).To(Succeed())
				pipelines := config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})
				Expect(pipelines["metrics"].(map[interface{}]interface{})["exporters"]).To(Equal([]interface{}{"otlp_grpc", "prometheus"}))
			} else {
				originalConfig, err := os.ReadFile(filepath.Join(fixtureDir, "collector-config-replicaset.yml"))
				Expect(err).NotTo(HaveOccurred())
				Expect(replicaConfig).To(Equal(originalConfig))
			}
		},
		Entry("Linux ReplicaSet", linuxReplica),
		Entry("Linux DaemonSet", linuxDaemonset),
		Entry("Windows DaemonSet", windowsDaemonset),
	)

	DescribeTable("should disable minimal ingestion without losing custom keep lists",
		func(keepList string) {
			setCollectorType(linuxReplica)
			writeSettings(map[string]string{
				"schema-version":                    "v1",
				"default-targets-metrics-keep-list": testKeepListSettings(keepList, false),
			})

			processConfigmapSettings(configDir)

			expected := defaultTestKeepLists()
			for key := range expected {
				expected[key] = ""
			}
			for _, key := range customKeepListKeys {
				expected[key] = keepList
			}
			checkHashMaps(configMapKeepListEnvVarPath, expected)
			Expect(os.Getenv("MINIMAL_INGESTION_PROFILE")).To(Equal("false"))
			checkScrapeJobs(linuxReplica.defaultJobs)
		},
		Entry("with custom regexes", "test.*|test2"),
		Entry("without custom regexes", ""),
	)

	It("should read partial v2 settings from the supplied directory", func() {
		setCollectorType(linuxReplica)
		writeSettings(map[string]string{
			"schema-version": "v2",
			"config-version": "ver2",
			"prometheus-collector-settings": `
				cluster_alias = "v2-alias"
				debug-mode = true`,
			"cluster-metrics": `
				default-targets-scrape-enabled: |-
					coredns = true
				minimal-ingestion-profile: |-
					enabled = false
				default-targets-metrics-keep-list: |-
					kubestate = "kube_.*"
				default-targets-scrape-interval-settings: |-
					kubestate = "17s"`,
		})

		processConfigmapSettings(configDir)

		checkEnvVars(map[string]string{
			"CONFIGMAP_VERSION":                         "v2",
			"AZMON_AGENT_CFG_SCHEMA_VERSION":            "v2",
			"AZMON_AGENT_CFG_FILE_VERSION":              "ver2",
			"AZMON_CLUSTER_ALIAS":                       "v2_alias",
			"AZMON_CLUSTER_LABEL":                       "v2_alias",
			"DEBUG_MODE_ENABLED":                        "true",
			"AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED": "true",
			"MINIMAL_INGESTION_PROFILE":                 "false",
		})
		keepLists := defaultTestKeepLists()
		for key := range keepLists {
			keepLists[key] = ""
		}
		keepLists["KUBESTATE_METRICS_KEEP_LIST_REGEX"] = "kube_.*"
		checkHashMaps(configMapKeepListEnvVarPath, keepLists)
		intervals := defaultTestIntervals()
		intervals["KUBESTATE_SCRAPE_INTERVAL"] = "17s"
		checkHashMaps(scrapeIntervalEnvVarPath, intervals)
		jobs := checkScrapeJobs(append([]string{"kube-dns"}, linuxReplica.defaultJobs...))
		Expect(jobs["kube-state-metrics"].ScrapeInterval).To(Equal("17s"))
		Expect(jobs["kube-dns"].ScrapeInterval).To(Equal("30s"))
	})

	It("should report an invalid OpenTelemetry metrics value and write the disabled default", func() {
		var output bytes.Buffer
		originalOutput := log.Writer()
		log.SetOutput(&output)
		DeferCleanup(log.SetOutput, originalOutput)

		Expect(ConfigureOpentelemetryMetricsSettings(map[string]map[string]string{
			"opentelemetry-metrics": {"enabled": "invalid"},
		})).To(Succeed())

		content, err := os.ReadFile(opentelemetryMetricsEnvVarPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("AZMON_FULL_OTLP_ENABLED=false\n"))
		Expect(output.String()).To(ContainSubstring("Invalid value for opentelemetry-metrics enabled: invalid, defaulting to false"))
	})
})

func copyTestFile(source, destination string) {
	content, err := os.ReadFile(source)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	ExpectWithOffset(1, os.WriteFile(destination, content, 0600)).To(Succeed())
	DeferCleanup(func() {
		unchanged, err := os.ReadFile(source)
		Expect(err).NotTo(HaveOccurred())
		Expect(unchanged).To(Equal(content), "source fixture must not be modified: "+source)
	})
}

func isolateConfigmapTestEnvironment() {
	original := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		original[key] = value
	}
	DeferCleanup(func() {
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if _, existed := original[key]; !existed {
				Expect(os.Unsetenv(key)).To(Succeed())
			}
		}
		for key, value := range original {
			Expect(os.Setenv(key, value)).To(Succeed())
		}
	})
	for key := range original {
		if strings.HasPrefix(key, "AZMON_") {
			Expect(os.Unsetenv(key)).To(Succeed())
		}
	}
	for _, key := range []string{
		"CONTAINER_TYPE", "CONTROLLER_TYPE", "OS_TYPE", "MODE", "WINMODE", "MAC", "CLUSTER",
		"KUBE_STATE_NAME", "POD_NAMESPACE", "NODE_NAME", "NODE_IP", "NODE_EXPORTER_NAME",
		"NODE_EXPORTER_TARGETPORT", "CCP_METRICS_ENABLED", "OPERATOR_TARGETS_HTTPS_ENABLED",
		"MESH_MEMBER_METRICS_FQDN", "MINIMAL_INGESTION_PROFILE", "CONFIGMAP_VERSION",
		"CONFIG_VALIDATOR_RUNNING_IN_AGENT", "DEBUG_MODE_ENABLED",
	} {
		Expect(os.Unsetenv(key)).To(Succeed())
	}
}

func setEnvVars(envVars map[string]string) {
	for key, value := range envVars {
		GinkgoT().Setenv(key, value)
	}
}

func checkEnvVars(expected map[string]string) {
	for key, value := range expected {
		ExpectWithOffset(1, os.Getenv(key)).To(Equal(value), key)
	}
}

func checkHashMaps(path string, expected map[string]string) {
	content, err := os.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var actual map[string]string
	ExpectWithOffset(1, yaml.Unmarshal(content, &actual)).To(Succeed())
	ExpectWithOffset(1, actual).To(Equal(expected))
}

type scrapeJob struct {
	Name           string `yaml:"job_name"`
	ScrapeInterval string `yaml:"scrape_interval"`
}

func checkScrapeJobs(expected []string) map[string]scrapeJob {
	content, err := os.ReadFile(mergedDefaultConfigPath)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	var config struct {
		Jobs []scrapeJob `yaml:"scrape_configs"`
	}
	ExpectWithOffset(1, yaml.Unmarshal(content, &config)).To(Succeed())
	jobs := make(map[string]scrapeJob)
	names := make([]string, 0, len(config.Jobs))
	for _, job := range config.Jobs {
		names = append(names, job.Name)
		jobs[job.Name] = job
	}
	ExpectWithOffset(1, names).To(ConsistOf(expected))
	ExpectWithOffset(1, string(content)).NotTo(MatchRegexp(`\$\$[A-Z_]+\$\$`))
	return jobs
}

func defaultTestEnv(settingsPresent bool) map[string]string {
	expected := map[string]string{
		"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "v1",
		"AZMON_AGENT_CFG_FILE_VERSION":                                 "ver1",
		"CONFIGMAP_VERSION":                                            "not_present",
		"AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX":             "",
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":                            "",
		"AZMON_CLUSTER_LABEL":                                          "",
		"AZMON_CLUSTER_ALIAS":                                          "",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING":                         "false",
		"AZMON_OPERATOR_ENABLED":                                       "true",
		"AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING":                       "",
		"AZMON_OPERATOR_HTTPS_ENABLED_CHART_SETTING":                   "false",
		"AZMON_OPERATOR_HTTPS_ENABLED":                                 "false",
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
		"AZMON_PROMETHEUS_ACSTORCAPACITYPROVISIONER_SCRAPING_ENABLED":  "true",
		"AZMON_PROMETHEUS_ACSTORMETRICSEXPORTER_SCRAPING_ENABLED":      "true",
		"AZMON_PROMETHEUS_LOCALCSIDRIVER_SCRAPING_ENABLED":             "true",
		"AZMON_PROMETHEUS_ZTUNNEL_SCRAPING_ENABLED":                    "false",
		"AZMON_PROMETHEUS_ISTIOCNI_SCRAPING_ENABLED":                   "false",
		"AZMON_PROMETHEUS_DCGMEXPORTER_SCRAPING_ENABLED":               "true",
		"AZMON_PROMETHEUS_CONTROLPLANE_ISTIO_ENABLED":                  "false",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
		"DEBUG_MODE_ENABLED":                                           "",
		"AZMON_FULL_OTLP_ENABLED":                                      "",
		"MINIMAL_INGESTION_PROFILE":                                    "true",
	}
	if settingsPresent {
		expected["CONFIGMAP_VERSION"] = "v1"
		expected["AZMON_OPERATOR_ENABLED_CHART_SETTING"] = "true"
		expected["DEBUG_MODE_ENABLED"] = "false"
		expected["AZMON_FULL_OTLP_ENABLED"] = "false"
	}
	return expected
}

func defaultTestKeepLists() map[string]string {
	return map[string]string{
		"KUBELET_METRICS_KEEP_LIST_REGEX":                    "|" + kubeletRegex_minimal_mac,
		"COREDNS_METRICS_KEEP_LIST_REGEX":                    "|" + coreDNSRegex_minimal_mac,
		"CADVISOR_METRICS_KEEP_LIST_REGEX":                   "|" + cadvisorRegex_minimal_mac,
		"KUBEPROXY_METRICS_KEEP_LIST_REGEX":                  "|" + kubeproxyRegex_minimal_mac,
		"APISERVER_METRICS_KEEP_LIST_REGEX":                  "|" + apiserverRegex_minimal_mac,
		"KUBESTATE_METRICS_KEEP_LIST_REGEX":                  "|" + kubestateRegex_minimal_mac,
		"NODEEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + nodeexporterRegex_minimal_mac,
		"WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX":            "|" + windowsexporterRegex_minimal_mac,
		"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX":           "|" + windowskubeproxyRegex_minimal_mac,
		"POD_ANNOTATION_METRICS_KEEP_LIST_REGEX":             "",
		"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX":                "|" + kappiebasicRegex_minimal_mac,
		"NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityRetinaRegex_minimal_mac,
		"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityHubbleRegex_minimal_mac,
		"NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX": "|" + networkobservabilityCiliumRegex_minimal_mac,
		"ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX":           "|" + acstorCapacityProvisionerRegex_minimal_mac,
		"ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX":              "|" + acstorMetricsExporter_minimal_mac,
		"LOCALCSIDRIVER_KEEP_LIST_REGEX":                     "|" + localCsiDriver_minimal_mac,
		"ZTUNNEL_METRICS_KEEP_LIST_REGEX":                    "|" + ztunnel_minimal_mac,
		"ISTIOCNI_METRICS_KEEP_LIST_REGEX":                   "|" + istioCni_minimal_mac,
		"DCGMEXPORTER_METRICS_KEEP_LIST_REGEX":               "|" + dcgmexporter_minimal_mac,
		"CONTROLPLANE_ISTIO_KEEP_LIST_REGEX":                 "|" + controlplaneIstio_minimal_mac,
	}
}

var customKeepListKeys = []string{
	"KUBELET_METRICS_KEEP_LIST_REGEX", "COREDNS_METRICS_KEEP_LIST_REGEX",
	"CADVISOR_METRICS_KEEP_LIST_REGEX", "KUBEPROXY_METRICS_KEEP_LIST_REGEX",
	"APISERVER_METRICS_KEEP_LIST_REGEX", "KUBESTATE_METRICS_KEEP_LIST_REGEX",
	"NODEEXPORTER_METRICS_KEEP_LIST_REGEX", "WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX",
	"WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX", "POD_ANNOTATION_METRICS_KEEP_LIST_REGEX",
	"KAPPIEBASIC_METRICS_KEEP_LIST_REGEX", "NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX",
	"NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX", "NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX",
}

var customIntervalKeys = []string{
	"KUBELET_SCRAPE_INTERVAL", "COREDNS_SCRAPE_INTERVAL", "CADVISOR_SCRAPE_INTERVAL",
	"KUBEPROXY_SCRAPE_INTERVAL", "APISERVER_SCRAPE_INTERVAL", "KUBESTATE_SCRAPE_INTERVAL",
	"NODEEXPORTER_SCRAPE_INTERVAL", "WINDOWSEXPORTER_SCRAPE_INTERVAL",
	"WINDOWSKUBEPROXY_SCRAPE_INTERVAL", "PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL",
	"POD_ANNOTATION_SCRAPE_INTERVAL", "KAPPIEBASIC_SCRAPE_INTERVAL",
	"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL", "NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL",
	"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL",
}

func defaultTestIntervals() map[string]string {
	expected := make(map[string]string)
	for _, key := range customIntervalKeys {
		expected[key] = "30s"
	}
	for _, key := range []string{
		"ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL", "ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL",
		"LOCALCSIDRIVER_SCRAPE_INTERVAL", "ZTUNNEL_SCRAPE_INTERVAL", "ISTIOCNI_SCRAPE_INTERVAL",
		"DCGMEXPORTER_SCRAPE_INTERVAL", "CONTROLPLANE_ISTIO_SCRAPE_INTERVAL",
	} {
		expected[key] = "30s"
	}
	return expected
}

func testKeepListSettings(regex string, minimalIngestion bool) string {
	var settings strings.Builder
	for _, key := range []string{
		"kubelet", "coredns", "cadvisor", "kubeproxy", "apiserver", "kubestate", "nodeexporter",
		"windowsexporter", "windowskubeproxy", "podannotations", "kappiebasic",
		"networkobservabilityRetina", "networkobservabilityHubble", "networkobservabilityCilium",
	} {
		settings.WriteString(key + " = \"" + regex + "\"\n")
	}
	if minimalIngestion {
		settings.WriteString("minimalingestionprofile = true\n")
	} else {
		settings.WriteString("minimalingestionprofile = false\n")
	}
	return settings.String()
}

func testIntervalSettings(interval string) string {
	var settings strings.Builder
	for _, key := range []string{
		"kubelet", "coredns", "cadvisor", "kubeproxy", "apiserver", "kubestate", "nodeexporter",
		"windowsexporter", "windowskubeproxy", "prometheuscollectorhealth", "podannotations",
		"kappiebasic", "networkobservabilityRetina", "networkobservabilityHubble", "networkobservabilityCilium",
	} {
		settings.WriteString(key + " = \"" + interval + "\"\n")
	}
	return settings.String()
}
