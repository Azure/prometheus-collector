package main

import (
    "fmt"
    "os"
    "strings"
)

func confgimapparserforccp() {
	fmt.Printf("in confgimapparserforccp")
	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"
	// Set agent config schema version
    if existsAndNotEmpty("/etc/config/settings/schema-version") {
        configVersion, err := readAndTrim(configVersionPath)
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
		os.Setenv("AZMON_AGENT_CFG_FILE_VERSION", configVersion)
    }

    // Set agent config file version
    if existsAndNotEmpty("/etc/config/settings/config-version") {
        configSchemaVersion, err := readAndTrim(configSchemaPath)
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
		os.Setenv("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion)
    }

	// Parse the configmap to set the right environment variables for prometheus collector settings
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-ccp-prometheus-collector-settings.rb")
	// sets env : AZMON_DEFAULT_METRIC_ACCOUNT_NAME, AZMON_CLUSTER_LABEL, AZMON_CLUSTER_ALIAS, AZMON_OPERATOR_ENABLED_CHART_SETTING in /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err := setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-ccp-default-scrape-settings.rb")
	// sets env: AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED...AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED in /opt/microsoft/configmapparser/config_default_scrape_settings_env_var
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = setEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
    startCommandAndWait("ruby", "/opt/microsoft/configmapparser/tomlparser-ccp-default-targets-metrics-keep-list.rb")
	// sets regexhas file /opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash

	startCommandAndWait("ruby", "/opt/microsoft/configmapparser/prometheus-ccp-config-merger.rb")

	os.Setenv("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false")
	os.Setenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true")

	// No need to merge custom prometheus config, only merging in the default configs
	os.Setenv("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true")
		startCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/collector-config-template.yml")
		if !exists("/opt/collector-config-with-defaults.yml") {
			fmt.Printf("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
		} else {
			sourcePath := "/opt/collector-config-with-defaults.yml"
			destinationPath := "/opt/microsoft/otelcollector/collector-config-default.yml"
			err := copyFile(sourcePath, destinationPath)
					if err != nil {
						fmt.Printf("Error copying file: %v\n", err)
					} else {
						fmt.Println("File copied successfully.")
					}
		}
}
