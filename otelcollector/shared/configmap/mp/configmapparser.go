package configmapsettings

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/prometheus-collector/shared"
)

func setConfigSchemaVersionEnv() {
	schemaVersionFile := "/etc/config/settings/schema-version"
	fileInfo, err := os.Stat(schemaVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		return
	}
	content, err := os.ReadFile(schemaVersionFile)
	if err != nil {
		shared.EchoError("Error reading schema version file:" + err.Error())
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configSchemaVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configSchemaVersion) > 10 {
		configSchemaVersion = configSchemaVersion[:10]
	}
	shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
}

func setConfigFileVersionEnv() {
	configVersionFile := "/etc/config/settings/config-version"
	fileInfo, err := os.Stat(configVersionFile)
	if err != nil || fileInfo.Size() == 0 {
		return
	}
	content, err := os.ReadFile(configVersionFile)
	if err != nil {
		shared.EchoError("Error reading config version file:" + err.Error())
		return
	}
	trimmedContent := strings.TrimSpace(string(content))
	configFileVersion := strings.ReplaceAll(trimmedContent, " ", "")
	if len(configFileVersion) > 10 {
		configFileVersion = configFileVersion[:10]
	}
	shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_FILE_VERSION", configFileVersion)
}

func parseSettingsForPodAnnotations() {
	// fmt.Printf("Start Processing - %s\n", LOGGING_PREFIX)
	fmt.Printf("Start Processing - pod annotations\n")
	if err := configurePodAnnotationSettings(); err != nil {
		fmt.Printf("%v\n", err)
		return
	}
	filename := "/opt/microsoft/configmapparser/config_def_pod_annotation_based_scraping"
	handleEnvFileError(filename)
	fmt.Println("End Processing - pod annotations")
}

func parsePrometheusCollectorConfig() {
	parseConfigAndSetEnvInFile()
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	handleEnvFileError(filename)
}

func parseDefaultScrapeSettings() {
	tomlparserDefaultScrapeSettings()
	filename := "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	handleEnvFileError(filename)
}

func parseDebugModeSettings() {
	if err := ConfigureDebugModeSettings(); err != nil {
		shared.EchoError(err.Error())
		return
	}
	filename := "/opt/microsoft/configmapparser/config_debug_mode_env_var"
	handleEnvFileError(filename)
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
	parseSettingsForPodAnnotations()
	parsePrometheusCollectorConfig()
	parseDefaultScrapeSettings()
	parseDebugModeSettings()

	tomlparserTargetsMetricsKeepList()
	tomlparserScrapeInterval()

	azmonOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
	containerType := os.Getenv("CONTAINER_TYPE")

	if azmonOperatorEnabled == "true" || containerType == "ConfigReaderSidecar" {
		prometheusConfigMerger(true)
	} else {
		prometheusConfigMerger(false)
	}

	shared.SetEnvAndSourceBashrc("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	shared.SetEnvAndSourceBashrc("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	// Running promconfigvalidator if promMergedConfig.yml exists
	if shared.FileExists("/opt/promMergedConfig.yml") {
		if !shared.FileExists("/opt/microsoft/otelcollector/collector-config.yml") {
			cmd := exec.Command("/opt/promconfigvalidator",
				"--config", "/opt/promMergedConfig.yml",
				"--output", "/opt/microsoft/otelcollector/collector-config.yml",
				"--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml",
			)
			err := cmd.Run()
			if err != nil {
				// Log error everywhere
				fmt.Println("prom-config-validator::Prometheus custom config validation failed. The custom config will not be used")
				shared.EchoError(err.Error())
				shared.SetEnvAndSourceBashrc("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true")
				if shared.FileExists("/opt/defaultsMergedConfig.yml") {
					fmt.Println("prom-config-validator::Running validator on just default scrape configs")
					shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
					if !shared.FileExists("/opt/collector-config-with-defaults.yml") {
						fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
					} else {
						shared.CopyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
					}
				}
				shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
			}
		}
	} else if _, err := os.Stat("/opt/defaultsMergedConfig.yml"); err == nil {
		fmt.Println("prom-config-validator::No custom prometheus config found. Only using default scrape configs")
		cmd := exec.Command("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
		if err := cmd.Run(); err != nil {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
			shared.EchoError(err.Error())
		} else {
			fmt.Println("prom-config-validator::Prometheus default scrape config validation succeeded, using this as collector config")
			shared.CopyFile("/opt/collector-config-with-defaults.yml", "/opt/microsoft/otelcollector/collector-config-default.yml")
		}
		shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
	} else {
		// This else block is needed, when there is no custom config mounted as config map or default configs enabled
		fmt.Println("prom-config-validator::No custom config via configmap or default scrape configs enabled.")
		shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
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
				os.Setenv(key, value)

				// Write to envvars.env
				fmt.Fprintf(envFile, "%s=%s\n", key, value)
			}
		}
		if err := scanner.Err(); err != nil {
			shared.EchoError("Error reading file:" + err.Error())
			return
		}

		// Source prom_config_validator_env_var
		cmd := exec.Command("bash", "-c", "source /opt/microsoft/prom_config_validator_env_var && env")
		if err := cmd.Run(); err != nil {
			shared.EchoError("Error sourcing env file:" + err.Error())
			return
		}

		// Source envvars.env
		cmd = exec.Command("bash", "-c", "source /opt/envvars.env && env")
		if err := cmd.Run(); err != nil {
			shared.EchoError("Error sourcing envvars.env:" + err.Error())
			return
		}
	}

	fmt.Printf("prom-config-validator::Use default prometheus config: %s\n", os.Getenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG"))
}
