package configmapsettings

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/prometheus-collector/shared"
)

const (
	defaultConfigSchemaVersion = "v1"
	defaultConfigFileVersion   = "ver1"
)

// Sets an environment variable from a file or uses default if file is invalid
func setEnvFromFileOrDefault(filePath, envName, defaultValue string) string {
	content, err := readFileContent(filePath)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error reading file %s: %v", filePath, err))
		shared.SetEnvAndSourceBashrcOrPowershell(envName, defaultValue, true)
		return defaultValue
	}

	value := sanitizeContent(content)
	shared.SetEnvAndSourceBashrcOrPowershell(envName, value, true)

	return value
}

// Reads file content as string, handling errors and empty files
func readFileContent(filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil || fileInfo.Size() == 0 {
		return "", fmt.Errorf("file doesn't exist or is empty")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// Sanitizes content by trimming and limiting length
func sanitizeContent(content string) string {
	trimmed := strings.TrimSpace(content)
	noSpaces := strings.ReplaceAll(trimmed, " ", "")
	if len(noSpaces) > 10 {
		return noSpaces[:10]
	}
	return noSpaces
}

func parseSettingsForPodAnnotations(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - parseSettingsForPodAnnotations")
	if err := configurePodAnnotationSettings(metricsConfigBySection); err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	handlePodAnnotationsFile(podAnnotationEnvVarPath)
	shared.EchoSectionDivider("End Processing - parseSettingsForPodAnnotations")
}

func handlePodAnnotationsFile(filename string) {
	// Check if the file exists
	_, e := os.Stat(filename)
	if os.IsNotExist(e) {
		fmt.Printf("File does not exist: %s\n", filename)
		return
	}

	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		index := strings.Index(line, "=")
		if index == -1 {
			fmt.Printf("Skipping invalid line: %s\n", line)
			continue
		}

		key := line[:index]
		value := line[index+1:]

		if key == "AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX" {
			shared.SetEnvAndSourceBashrcOrPowershell(key, value, false)
		} else {
			shared.SetEnvAndSourceBashrcOrPowershell(key, value, false)
		}

	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %s\n", err)
	}
}

func parsePrometheusCollectorConfig(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - parsePrometheusCollectorConfig")
	parseConfigAndSetEnvInFile(metricsConfigBySection)
	handleEnvFileError(collectorSettingsEnvVarPath)
	shared.EchoSectionDivider("End Processing - parsePrometheusCollectorConfig")
}

func parseDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, schemaVersion string) {
	shared.EchoSectionDivider("Start Processing - parseDefaultScrapeSettings")
	tomlparserDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	handleEnvFileError(defaultSettingsEnvVarPath)
	shared.EchoSectionDivider("End Processing - parseDefaultScrapeSettings")
}

func parseDebugModeSettings(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - parseDebugModeSettings")
	if err := ConfigureDebugModeSettings(metricsConfigBySection); err != nil {
		shared.EchoError(err.Error())
	}
	shared.EchoSectionDivider("End Processing - " + name)
}

// func parseSettingsForPodAnnotations() {
// 	executeWithSectionLog("parseSettingsForPodAnnotations", func() error {
// 		if err := configurePodAnnotationSettings(); err != nil {
// 			return err
// 		}
// 		handlePodAnnotationsFile(podAnnotationEnvVarPath)
// 		return nil
// 	})
// }

// func handlePodAnnotationsFile(filename string) {
// 	file, err := os.Open(filename)
// 	if os.IsNotExist(err) {
// 		fmt.Printf("File does not exist: %s\n", filename)
// 		return
// 	} else if err != nil {
// 		fmt.Printf("Error opening file: %s\n", err)
// 		return
// 	}
// 	defer file.Close()

// 	scanner := bufio.NewScanner(file)
// 	for scanner.Scan() {
// 		line := scanner.Text()
// 		if index := strings.Index(line, "="); index != -1 {
// 			key := line[:index]
// 			value := line[index+1:]
// 			shared.SetEnvAndSourceBashrcOrPowershell(key, value, false)
// 		} else {
// 			fmt.Printf("Skipping invalid line: %s\n", line)
// 		}
// 	}

// 	if err := scanner.Err(); err != nil {
// 		fmt.Printf("Error reading file: %s\n", err)
// 	}
// }

func runPrometheusValidator(configPath, outputPath, templatePath string) bool {
	err := shared.StartCommandAndWait(
		"/opt/promconfigvalidator",
		"--config", configPath,
		"--output", outputPath,
		"--otelTemplate", templatePath,
	)
	return err == nil
}

func handleEnvFile(filename string) {
	if err := shared.SetEnvVarsFromFile(filename); err != nil {
		fmt.Printf("Error setting env vars from %s: %v\n", filename, err)
	}
}

func Configmapparser() {
	shared.ProcessConfigFile(configVersionFile, "AZMON_AGENT_CFG_FILE_VERSION")
	shared.ProcessConfigFile(schemaVersionFile, "AZMON_AGENT_CFG_SCHEMA_VERSION")

	var metricsConfigBySection map[string]map[string]string
	var err error
	var schemaVersion = shared.ParseSchemaVersion(os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"))
	switch schemaVersion {
	case shared.SchemaVersion.V2:
		filePaths := []string{"/etc/config/settings/metrics", "/etc/config/settings/prometheus-collector-settings"}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Error parsing files: %v\n", err)
			return
		}
	case shared.SchemaVersion.V1:
		configDir := "/etc/config/settings"
		metricsConfigBySection, err = shared.ParseV1Config(configDir)
		if err != nil {
			fmt.Printf("Error parsing config: %v\n", err)
			return
		}
	default:
		fmt.Println("Invalid schema version or no configmap present. Using defaults.")
	}

	// Check if /etc/config/settings/config-version exists
	if _, err := os.Stat("/etc/config/settings/config-version"); os.IsNotExist(err) {
		metricsConfigBySection = nil
		fmt.Println("Config version file not found. Setting metricsConfigBySection to nil i.e. no configmap is mounted")
	}

	parseSettingsForPodAnnotations(metricsConfigBySection)
	parsePrometheusCollectorConfig(metricsConfigBySection)
	parseDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	parseDebugModeSettings(metricsConfigBySection)

	tomlparserTargetsMetricsKeepList(metricsConfigBySection, schemaVersion)
	tomlparserScrapeInterval(metricsConfigBySection, schemaVersion)

	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	containerType := os.Getenv("CONTAINER_TYPE")

	if azmonOperatorEnabled == "true" || containerType == "ConfigReaderSidecar" {
		prometheusConfigMerger(true)
	} else {
		prometheusConfigMerger(false)
	}

	// Set version environment variables
	// schemaVersion := setEnvFromFileOrDefault(schemaVersionFile, "AZMON_AGENT_CFG_SCHEMA_VERSION", defaultConfigSchemaVersion)
	// configVersion := setEnvFromFileOrDefault(configVersionFile, "AZMON_AGENT_CFG_FILE_VERSION", defaultConfigFileVersion)

	// Parse settings
	// parseSettingsForPodAnnotations()

	// Parse configs
	// executeWithSectionLog("parsePrometheusCollectorConfig", func() error {
	// 	parseConfigAndSetEnvInFile(schemaVersion)
	// 	handleEnvFile(collectorSettingsEnvVarPath)
	// 	return nil
	// })

	// executeWithSectionLog("parseDefaultScrapeSettings", func() error {
	// 	tomlparserDefaultScrapeSettings()
	// 	handleEnvFile(defaultSettingsEnvVarPath)
	// 	return nil
	// })

	// executeWithSectionLog("parseDebugModeSettings", func() error {
	// 	err := ConfigureDebugModeSettings()
	// 	if err == nil {
	// 		handleEnvFile(debugModeEnvVarPath)
	// 	}
	// 	return err
	// })

	// // Process TOML configs
	// tomlparserTargetsMetricsKeepList()
	// tomlparserScrapeInterval()

	// Set default flags
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// Handle prometheus configuration
	templatePath := "/opt/microsoft/otelcollector/collector-config-template.yml"
	outputPath := "/opt/microsoft/otelcollector/collector-config.yml"

	if shared.FileExists("/opt/promMergedConfig.yml") && !shared.FileExists(outputPath) {
		if runPrometheusValidator("/opt/promMergedConfig.yml", outputPath, templatePath) {
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_SET_GLOBAL_SETTINGS", "true", true)
		} else {
			fmt.Println("prom-config-validator::Prometheus custom config validation failed. The custom config will not be used")
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", true)

			if shared.FileExists(mergedDefaultConfigPath) {
				fmt.Println("prom-config-validator::Running validator on just default scrape configs")
				defaultOutputPath := "/opt/collector-config-with-defaults.yml"
				if runPrometheusValidator(mergedDefaultConfigPath, defaultOutputPath, templatePath) {
					shared.CopyFile(defaultOutputPath, "/opt/microsoft/otelcollector/collector-config-default.yml")
				} else {
					fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
				}
			}
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
		}
	} else if shared.FileExists(mergedDefaultConfigPath) {
		fmt.Println("prom-config-validator::No custom prometheus config found. Only using default scrape configs")
		defaultOutputPath := "/opt/collector-config-with-defaults.yml"
		if runPrometheusValidator(mergedDefaultConfigPath, defaultOutputPath, templatePath) {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config")
			shared.CopyFile(defaultOutputPath, "/opt/microsoft/otelcollector/collector-config-default.yml")
		} else {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
		}
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	} else {
		// This else block is needed, when there is no custom config mounted as config map or default configs enabled
		fmt.Println("prom-config-validator::No custom config via configmap or default scrape configs enabled.")
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	}

	if _, err := os.Stat("/opt/microsoft/prom_config_validator_env_var"); err == nil {
		file, err := os.Open("/opt/microsoft/prom_config_validator_env_var")
		if err != nil {
			shared.EchoError("Error opening file:" + err.Error())
			return
		}
		defer file.Close()

		// Create or truncate envvars.env file
		envFile, err := os.Create("/opt/envvars.env")
		if err != nil {
			shared.EchoError("Error creating env file:" + err.Error())
			return
		}
		defer envFile.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				shared.SetEnvAndSourceBashrcOrPowershell(key, value, true)

				// Write to envvars.env
				fmt.Fprintf(envFile, "%s=%s\n", key, value)
			}
		}
		if err := scanner.Err(); err != nil {
			shared.EchoError("Error reading file:" + err.Error())
			return
		}

		// Source prom_config_validator_env_var
		filename := "/opt/microsoft/prom_config_validator_env_var"
		err = shared.SetEnvVarsFromFile(filename)
		if err != nil {
			fmt.Printf("Error when settinng env for /opt/microsoft/prom_config_validator_env_var: %v\n", err)
		}

		filename = "/opt/envvars.env"
		err = shared.SetEnvVarsFromFile(filename)
		if err != nil {
			fmt.Printf("Error when settinng env for /opt/envvars.env: %v\n", err)
		}
	}

	fmt.Printf("prom-config-validator::Use default prometheus config: %s\n", os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"))
}
