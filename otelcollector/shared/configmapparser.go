package main

import (
	"fmt"
	"os"
	"strings"
)

func setAgentConfigVersionEnv() {
	configVersionPath := "/etc/config/settings/config-version"
	if existsAndNotEmpty(configVersionPath) {
		configVersion, err := readAndTrim(configVersionPath)
		if err != nil {
			fmt.Println("Error reading config version file:", err)
			return
		}
		configVersion = cleanAndTruncate(configVersion)
		os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configVersion)
	}
}

func setConfigSchemaVersionEnv() {
	configSchemaPath := "/etc/config/settings/schema-version"
	if existsAndNotEmpty(configSchemaPath) {
		configSchemaVersion, err := readAndTrim(configSchemaPath)
		if err != nil {
			fmt.Println("Error reading config schema version file:", err)
			return
		}
		configSchemaVersion = cleanAndTruncate(configSchemaVersion)
		os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
	}
}

func parseSettingsForPodAnnotations() {
	fmt.Println("In parseSettingsForPodAnnotations")
}

func parsePrometheusCollectorConfig() {
	parseConfigAndSetEnvInFile()
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	handleEnvFileError(filename)
}

func parseDefaultScrapeSettings() {
	tomlparserCCPDefaultScrapeSettings()
	filename := "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	handleEnvFileError(filename)
}

func handleEnvFileError(filename string) {
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when setting env for %s: %v\n", filename, err)
	}
}

func cleanAndTruncate(input string) string {
	input = strings.ReplaceAll(input, " ", "")
	if len(input) >= 10 {
		input = input[:10]
	}
	return input
}

func configmapparser() {
	setAgentConfigVersionEnv()
	setConfigSchemaVersionEnv()
	parsePrometheusCollectorConfig()
	parseDefaultScrapeSettings()

	tomlparserCCPTargetsMetricsKeepList()
	prometheusCcpConfigMerger()

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
	startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")

	if !exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
	} else {
		sourcePath := "/opt/ccp-collector-config-with-defaults.yml"
		destinationPath := "/opt/microsoft/otelcollector/ccp-collector-config-default.yml"
		if err := copyFile(sourcePath, destinationPath); err != nil {
			fmt.Printf("Error copying file: %v\n", err)
		} else {
			fmt.Println("File copied successfully.")
		}
	}
}

func main() {
	configmapparser()
}
