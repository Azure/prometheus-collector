package configmapsettings

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

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
	}
	for _, envVar := range allEnvVars {
		os.Unsetenv(envVar)
	}
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
		configMapDebugMountPath = "/etc/config/settings/debug-mode"
		replicaSetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
		defaultSettingsMountPath = "/etc/config/settings/default-scrape-settings"
		configMapKeepListMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
		configMapMountPathForPodAnnotation = "/etc/config/settings/pod-annotation-based-scraping"
		collectorSettingsMountPath = "/etc/config/settings/prometheus-collector-settings"
		//schemaVersionFile = "/etc/config/settings/schema-version"
		//configVersionFile = "/etc/config/settings/config-version"
		configMapScrapeIntervalMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
		createTempFile(configSettingsPrefix, "metrics", "")
		createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
	} else {
		//schemaVersionFile = createTempFile(configSettingsPrefix, "schema-version", "v1")
		//configVersionFile = createTempFile(configSettingsPrefix, "config-version", "ver1")
		configMapMountPathForPodAnnotation = createTempFile(configSettingsPrefix, "pod-annotation-based-scraping", "")
		collectorSettingsMountPath = createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
		defaultSettingsMountPath = createTempFile(configSettingsPrefix, "default-scrape-settings-enabled", "")
		configMapDebugMountPath = createTempFile(configSettingsPrefix, "debug-mode", "")
		configMapKeepListMountPath = createTempFile(configSettingsPrefix, "default-targets-metrics-keep-list", "")
		configMapScrapeIntervalMountPath = createTempFile(configSettingsPrefix, "default-targets-scrape-interval-settings", "")
		createTempFile(configSettingsPrefix, "metrics", "")
		createTempFile(configSettingsPrefix, "prometheus-collector-settings", "")
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
	promMergedConfigPath = createTempFile(configSettingsPrefix, "merged-default-and-custom-scrape-configs", "")
	regexHashFile = configMapKeepListEnvVarPath
	intervalHashFile = scrapeIntervalEnvVarPath
}

func createTempFile(dir string, name string, content string) string {
	// Make sure the directory exists
	fmt.Println("creating file with content", content)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return fmt.Sprintf("Failed to create directory %s: %v", dir, err)
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

// Helper function to generate standard linux replica set env vars
func setSetupEnvVars(controllertype string, os string) {
	envVars := map[string]string{}
	if os == shared.OSType.Windows {
		envVars = map[string]string{
			"AZMON_OPERATOR_ENABLED":    "true",
			"CONTAINER_TYPE":            "",
			"CONTROLLER_TYPE":           "DaemonSet",
			"OS_TYPE":                   "windows",
			"MODE":                      "advanced",
			"KUBE_STATE_NAME":           "ama-metrics-ksm",
			"POD_NAMESPACE":             "kube-system",
			"MAC":                       "true",
			"NODE_NAME":                 "test-node",
			"NODE_IP":                   "192.168.1.1",
			"NODE_EXPORTER_TARGETPORT":  "9100",
			"AZMON_SET_GLOBAL_SETTINGS": "true",
		}
	} else if os == shared.OSType.Linux {
		switch controllertype {
		case shared.ControllerType.ReplicaSet:
			envVars = map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            "",
				"CONTROLLER_TYPE":           "ReplicaSet",
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		case shared.ControllerType.ConfigReaderSidecar:
			envVars = map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            "ConfigReaderSidecar",
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		case shared.ControllerType.DaemonSet:
			envVars = map[string]string{
				"AZMON_OPERATOR_ENABLED":    "true",
				"CONTAINER_TYPE":            "",
				"CONTROLLER_TYPE":           "DaemonSet",
				"OS_TYPE":                   "linux",
				"MODE":                      "advanced",
				"KUBE_STATE_NAME":           "ama-metrics-ksm",
				"POD_NAMESPACE":             "kube-system",
				"MAC":                       "true",
				"NODE_NAME":                 "test-node",
				"NODE_IP":                   "192.168.1.1",
				"NODE_EXPORTER_TARGETPORT":  "9100",
				"AZMON_SET_GLOBAL_SETTINGS": "true",
			}
		default:
			envVars = map[string]string{}
		}
	}

	setEnvVars(envVars)
}

func getDefaultExpectedEnvVars() map[string]string {
	return map[string]string{
		"AZMON_AGENT_CFG_SCHEMA_VERSION":                               "",
		"AZMON_AGENT_CFG_FILE_VERSION":                                 "",
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
		"AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED":           "",
		"AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED":             "",
		"AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED":            "false",
		"AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED":           "false",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED": "true",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED": "true",
		"AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED": "true",
		"AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED":                 "false",
		"DEBUG_MODE_ENABLED":                                           "",
	}
}

// Helper function to generate standard expected keep list map
func getExpectedKeepListMap(includeMinimalKeepList bool, value string) map[string]string {
	expectedKeepListHashMap := make(map[string]string)
	for jobName, job := range shared.DefaultScrapeJobs {
		keyName := fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))
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
	for jobName, _ := range shared.DefaultScrapeJobs {
		keyName := fmt.Sprintf("%s_SCRAPE_INTERVAL", strings.ToUpper(jobName))
		if value == "" {
			expectedScrapeIntervalHashMap[keyName] = "30s"
		} else {
			expectedScrapeIntervalHashMap[keyName] = value
		}
	}
	return expectedScrapeIntervalHashMap
}

func CreateTempFilesFromConfigMapTestCase(testCaseFileName string, schemaVersion string) error {
	var testCaseDir string
	switch schemaVersion {
	case shared.SchemaVersion.V1:
		testCaseDir = "./configmap-test-cases/v1/"
	case shared.SchemaVersion.V2:
		testCaseDir = "./configmap-test-cases/v2/"
	default:
		return fmt.Errorf("unsupported schema version: %s", schemaVersion)
	}
	filePath := testCaseDir + testCaseFileName

	// Read the YAML file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read test case file %s: %v", filePath, err)
	}

	// Parse the YAML into a map
	var yamlData map[string]interface{}
	err = yaml.Unmarshal(data, &yamlData)
	if err != nil {
		return fmt.Errorf("failed to parse YAML from %s: %v", filePath, err)
	}

	for fileName, data := range yamlData {
		// Convert the value to YAML string
		var content string
		if data == nil {
			content = ""
		} else {
			contentBytes, err := yaml.Marshal(data)
			if err != nil {
				return fmt.Errorf("failed to marshal content for key %s: %v", fileName, err)
			}
			content = string(contentBytes)
		}

		createTempFile(configSettingsPrefix, fileName, content)
	}

	return nil
}
