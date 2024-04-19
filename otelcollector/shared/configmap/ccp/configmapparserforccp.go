package ccpconfigmapsettings

import (
	"fmt"
	"strings"

	"prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Printf("in configmapparserforccp")
	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"
	// Set agent config schema version
	if shared.ExistsAndNotEmpty("/etc/config/settings/schema-version") {
		configVersion, err := shared.ReadAndTrim(configVersionPath)
		if err != nil {
			fmt.Println("Error reading config version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configVersion = strings.ReplaceAll(configVersion, " ", "")
		if len(configVersion) >= 10 {
			configVersion = configVersion[:10]
		}
		// Set the environment variable
		shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_FILE_VERSION", configVersion)
	}

	// Set agent config file version
	if shared.ExistsAndNotEmpty("/etc/config/settings/config-version") {
		configSchemaVersion, err := shared.ReadAndTrim(configSchemaPath)
		if err != nil {
			fmt.Println("Error reading config schema version file:", err)
			return
		}
		// Remove all spaces and take the first 10 characters
		configSchemaVersion = strings.ReplaceAll(configSchemaVersion, " ", "")
		if len(configSchemaVersion) >= 10 {
			configSchemaVersion = configSchemaVersion[:10]
		}
		// Set the environment variable
		shared.SetEnvAndSourceBashrc("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
	}

	// Parse the configmap to set the right environment variables for prometheus collector settings
	parseConfigAndSetEnvInFile()
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err := shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings()
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList()

	prometheusCcpConfigMerger()

	shared.SetEnvAndSourceBashrc("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	shared.SetEnvAndSourceBashrc("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	// No need to merge custom prometheus config, only merging in the default configs
	shared.SetEnvAndSourceBashrc("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
	shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")
	if !shared.Exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Printf("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
	} else {
		sourcePath := "/opt/ccp-collector-config-with-defaults.yml"
		destinationPath := "/opt/microsoft/otelcollector/ccp-collector-config-default.yml"
		err := shared.CopyFile(sourcePath, destinationPath)
		if err != nil {
			fmt.Printf("Error copying file: %v\n", err)
		} else {
			fmt.Println("File copied successfully.")
		}
	}
}
