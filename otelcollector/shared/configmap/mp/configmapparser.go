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

// Executes a function and logs section dividers
func executeWithSectionLog(name string, fn func() error) {
	shared.EchoSectionDivider("Start Processing - " + name)
	if err := fn(); err != nil {
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
	for sectionName, sectionSettings := range ConfigMapSettings {
		executeWithSectionLog(sectionName, func() error {
			return sectionSettings.Configure()
		})
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

	// Merge configurations
	useOperatorMode := os.Getenv("AZMON_OPERATOR_ENABLED") == "true" ||
		os.Getenv("CONTAINER_TYPE") == "ConfigReaderSidecar"
	prometheusConfigMerger(useOperatorMode)

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
		fmt.Println("prom-config-validator::No custom config via configmap or default scrape configs enabled.")
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	}

	// Process additional environment variables if available
	processValidatorEnvVars()

	fmt.Printf("prom-config-validator::Use default prometheus config: %s\n", os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"))
}

func processValidatorEnvVars() {
	validatorEnvFile := "/opt/microsoft/prom_config_validator_env_var"
	if !shared.FileExists(validatorEnvFile) {
		return
	}

	file, err := os.Open(validatorEnvFile)
	if err != nil {
		shared.EchoError("Error opening file:" + err.Error())
		return
	}
	defer file.Close()

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
			key, value := parts[0], parts[1]
			shared.SetEnvAndSourceBashrcOrPowershell(key, value, true)
			fmt.Fprintf(envFile, "%s=%s\n", key, value)
		}
	}

	if err := scanner.Err(); err != nil {
		shared.EchoError("Error reading file:" + err.Error())
		return
	}

	handleEnvFile(validatorEnvFile)
	handleEnvFile("/opt/envvars.env")
}
