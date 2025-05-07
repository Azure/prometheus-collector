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

func setConfigSchemaVersionEnv() {
	fileInfo, err := os.Stat(schemaVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", defaultConfigSchemaVersion, true)
		return
	}
	content, err := os.ReadFile(schemaVersionFile)
	if err != nil {
		shared.EchoError("Error reading schema version file:" + err.Error())
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", defaultConfigSchemaVersion, true)
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configSchemaVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configSchemaVersion) > 10 {
		configSchemaVersion = configSchemaVersion[:10]
	}
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion, true)
}

func setConfigFileVersionEnv() {
	fileInfo, err := os.Stat(configVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", defaultConfigFileVersion, true)
		return
	}
	content, err := os.ReadFile(configVersionFile)
	if err != nil {
		shared.EchoError("Error reading config version file:" + err.Error())
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", defaultConfigFileVersion, true)
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configFileVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configFileVersion) > 10 {
		configFileVersion = configFileVersion[:10]
	}
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", configFileVersion, true)
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

func parseDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - parseDefaultScrapeSettings")
	tomlparserDefaultScrapeSettings(metricsConfigBySection)
	handleEnvFileError(defaultSettingsEnvVarPath)
	shared.EchoSectionDivider("End Processing - parseDefaultScrapeSettings")
}

func parseDebugModeSettings(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - parseDebugModeSettings")
	if err := ConfigureDebugModeSettings(metricsConfigBySection); err != nil {
		shared.EchoError(err.Error())
		return
	}
	handleEnvFileError(debugModeEnvVarPath)
	shared.EchoSectionDivider("End Processing - parseDebugModeSettings")
}

func parseOpentelemetryMetricsSettings() {
	shared.EchoSectionDivider("Start Processing - parseOpentelemetryMetricsSettings")
	if err := ConfigureOpentelemetryMetricsSettings(); err != nil {
		shared.EchoError(err.Error())
		return
	}
	handleEnvFileError(opentelemetryMetricsEnvVarPath)
	shared.EchoSectionDivider("End Processing - parseOpentelemetryMetricsSettings")
}

func handleEnvFileError(filename string) {
	err := shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when setting env for %s: %v\n", filename, err)
	}
}

func Configmapparser() {
	setConfigFileVersionEnv()
	setConfigSchemaVersionEnv()

	var metricsConfigBySection map[string]map[string]string
	var err error
	if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v2" {
		filePaths := []string{"/etc/config/settings/cluster-metrics", "/etc/config/settings/prometheus-collector-settings"}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Using defaults as error parsing files: %v\n", err)
		}
	} else if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v1" {
		configDir := "/etc/config/settings"
		metricsConfigBySection, err = shared.ParseV1Config(configDir)
		if err != nil {
			fmt.Printf("Using defaults as error parsing config: %v\n", err)
		}
	} else {
		fmt.Println("Invalid schema version. Using defaults.")
	}

	// Check if /etc/config/settings/config-version exists
	if _, err := os.Stat("/etc/config/settings/config-version"); os.IsNotExist(err) {
		metricsConfigBySection = nil
		fmt.Println("Config version file not found. Setting metricsConfigBySection to nil i.e. no configmap is mounted")
	}

	parseSettingsForPodAnnotations(metricsConfigBySection)
	parsePrometheusCollectorConfig(metricsConfigBySection)
	parseDefaultScrapeSettings(metricsConfigBySection)
	parseDebugModeSettings(metricsConfigBySection)
	parseOpentelemetryMetricsSettings()

	tomlparserTargetsMetricsKeepList(metricsConfigBySection)
	tomlparserScrapeInterval(metricsConfigBySection)

	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	containerType := os.Getenv("CONTAINER_TYPE")

	if azmonOperatorEnabled == "true" || containerType == "ConfigReaderSidecar" {
		prometheusConfigMerger(true)
	} else {
		prometheusConfigMerger(false)
	}

	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// Running promconfigvalidator if promMergedConfig.yml exists
	if shared.FileExists("/opt/promMergedConfig.yml") {
		if !shared.FileExists("/opt/microsoft/otelcollector/collector-config.yml") {
			err := shared.StartCommandAndWait("/opt/promconfigvalidator",
				"--config", "/opt/promMergedConfig.yml",
				"--output", "/opt/microsoft/otelcollector/collector-config.yml",
				"--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml",
			)
			if err != nil {
				fmt.Println("prom-config-validator::Prometheus custom config validation failed. The custom config will not be used")
				fmt.Printf("Command execution failed: %v\n", err)
				shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", true)
				if shared.FileExists(mergedDefaultConfigPath) {
					fmt.Println("prom-config-validator::Running validator on just default scrape configs")
					shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", mergedDefaultConfigPath, "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
					if !shared.FileExists("/opt/collector-config-with-defaults.yml") {
						fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
					} else {
						shared.CopyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
					}
				}
				shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
			} else {
				shared.SetEnvAndSourceBashrcOrPowershell("AZMON_SET_GLOBAL_SETTINGS", "true", true)
			}
		}
	} else if _, err := os.Stat(mergedDefaultConfigPath); err == nil {
		fmt.Println("prom-config-validator::No custom prometheus config found. Only using default scrape configs")
		err := shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", mergedDefaultConfigPath, "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
		if err != nil {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
			fmt.Printf("Command execution failed: %v\n", err)
		} else {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config")
			shared.CopyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
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
