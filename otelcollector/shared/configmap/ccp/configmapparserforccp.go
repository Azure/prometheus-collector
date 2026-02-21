package ccpconfigmapsettings

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	// "prometheus-collector/shared"
	"github.com/prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Printf("in configmapparserforccp")
	fmt.Printf("waiting for 30 secs...")
	time.Sleep(30 * time.Second) //needed to save a restart at times when config watcher sidecar starts up later than us and hence config map wasn't yet projected into emptydir volume yet during pod startups.

	// Initialize settings config validation as valid; will be set to true if parsing fails
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_METRICS_SETTINGS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("INVALID_SETTINGS_CONFIG_ERROR", "", true)

	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"

	entries, er := os.ReadDir("/etc/config/settings")
	if er != nil {
		fmt.Println("error listing /etc/config/settings", er)
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}

	fmt.Println("done listing /etc/config/settings")

	// Set agent config schema version
	if shared.ExistsAndNotEmpty(configSchemaPath) {
		configVersion, err := shared.ReadAndTrim(configVersionPath)
		if err != nil {
			errMsg := fmt.Sprintf("Unable to read config version from ama-metrics-settings-configmap (%s): %v. Using default configuration", configVersionPath, err)
			fmt.Println(errMsg)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_METRICS_SETTINGS_CONFIG", "true", true)
			shared.SetEnvAndSourceBashrcOrPowershell("INVALID_SETTINGS_CONFIG_ERROR", errMsg, true)
		} else {
			// Remove all spaces and take the first 10 characters
			configVersion = strings.ReplaceAll(configVersion, " ", "")
			if len(configVersion) >= 10 {
				configVersion = configVersion[:10]
			}
			// Set the environment variable
			fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_FILE_VERSION:", configVersion)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", configVersion, true)
		}
	} else {
		fmt.Println("Configmapparserforccp fileversion file doesn't exist. or configmap doesn't exist:", configVersionPath)
	}

	// Set agent config file version
	if shared.ExistsAndNotEmpty(configVersionPath) {
		configSchemaVersion, err := shared.ReadAndTrim(configSchemaPath)
		if err != nil {
			errMsg := fmt.Sprintf("Unable to read schema version from ama-metrics-settings-configmap (%s): %v. Using default configuration", configSchemaPath, err)
			fmt.Println(errMsg)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_METRICS_SETTINGS_CONFIG", "true", true)
			shared.SetEnvAndSourceBashrcOrPowershell("INVALID_SETTINGS_CONFIG_ERROR", errMsg, true)
		} else {
			// Remove all spaces and take the first 10 characters
			configSchemaVersion = strings.ReplaceAll(configSchemaVersion, " ", "")
			if len(configSchemaVersion) >= 10 {
				configSchemaVersion = configSchemaVersion[:10]
			}
			// Set the environment variable
			fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_SCHEMA_VERSION:", configSchemaVersion)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion, true)
		}
	} else {
		fmt.Println("Configmapparserforccp schemaversion file doesn't exist. or configmap doesn't exist:", configSchemaPath)
	}

	var metricsConfigBySection map[string]map[string]string
	var err error
	if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v2" {
		filePaths := []string{"/etc/config/settings/controlplane-metrics", "/etc/config/settings/prometheus-collector-settings"}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to parse v2 settings from ama-metrics-settings-configmap: %v. Falling back to default configuration", cleanSettingsError(err))
			fmt.Println(errMsg)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_METRICS_SETTINGS_CONFIG", "true", true)
			shared.SetEnvAndSourceBashrcOrPowershell("INVALID_SETTINGS_CONFIG_ERROR", errMsg, true)
			fmt.Println("Continuing with default configuration despite settings parse error")
		}
	} else if os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION") == "v1" {
		configDir := "/etc/config/settings"
		metricsConfigBySection, err = shared.ParseV1Config(configDir)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to parse v1 settings from ama-metrics-settings-configmap: %v. Falling back to default configuration", cleanSettingsError(err))
			fmt.Println(errMsg)
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_METRICS_SETTINGS_CONFIG", "true", true)
			shared.SetEnvAndSourceBashrcOrPowershell("INVALID_SETTINGS_CONFIG_ERROR", errMsg, true)
			fmt.Println("Continuing with default configuration despite settings parse error")
		}
	} else {
		fmt.Println("Invalid schema version or no configmap present. Using defaults.")
	}

	// Parse the configmap to set the right environment variables for prometheus collector settings
	parseConfigAndSetEnvInFile(metricsConfigBySection)
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	ConfigureOpentelemetryMetricsSettings(metricsConfigBySection)
	filename = "/opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings(metricsConfigBySection)
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection)

	prometheusCcpConfigMerger()

	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// No need to merge custom prometheus config, only merging in the default configs
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
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

// cleanSettingsError produces a concise, actionable error message for the health metric.
// It strips redundant OS-level detail (e.g. duplicate paths from os.Open wrapping)
// and adds context about the expected configmap source.
func cleanSettingsError(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		// Extract the path from the wrapped error chain
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return fmt.Sprintf("settings file not found: %s (expected from ama-metrics-settings-configmap)", pathErr.Path)
		}
	}
	return err.Error()
}
