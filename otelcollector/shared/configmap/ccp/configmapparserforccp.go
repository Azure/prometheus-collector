package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Println("in configmapparserforccp")
	fmt.Println("waiting for 30 secs...")
	time.Sleep(30 * time.Second)

	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"

	// Debug: List directory contents
	if entries, err := os.ReadDir("/etc/config/settings"); err != nil {
		fmt.Println("error listing /etc/config/settings:", err)
	} else {
		for _, e := range entries {
			fmt.Println(e.Name())
		}
		fmt.Println("done listing /etc/config/settings")
	}

	// Process config schema and version
	processConfigFile(configVersionPath, "AZMON_AGENT_CFG_FILE_VERSION")
	processConfigFile(configSchemaPath, "AZMON_AGENT_CFG_SCHEMA_VERSION")

	// Parse configurations and set environment variables
	parseConfigAndSetEnvInFile()
	setEnvFromFile("/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var")

	tomlparserCCPDefaultScrapeSettings()
	setEnvFromFile("/opt/microsoft/configmapparser/config_default_scrape_settings_env_var")

	tomlparserCCPTargetsMetricsKeepList()
	prometheusCcpConfigMerger()

	// Set required environment variables
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)

	// Run the command to validate and generate config
	shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml",
		"--output", "/opt/ccp-collector-config-with-defaults.yml",
		"--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")

	// Copy the generated config if it exists
	if !shared.Exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
	} else if err := shared.CopyFile("/opt/ccp-collector-config-with-defaults.yml",
		"/opt/microsoft/otelcollector/ccp-collector-config-default.yml"); err != nil {
		fmt.Printf("Error copying file: %v\n", err)
	} else {
		fmt.Println("File copied successfully.")
	}
}

func processConfigFile(path, envVar string) {
	if shared.ExistsAndNotEmpty(path) {
		value, err := shared.ReadAndTrim(path)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", path, err)
			return
		}

		value = strings.ReplaceAll(value, " ", "")
		if len(value) >= 10 {
			value = value[:10]
		}

		fmt.Printf("Setting env var %s: %s\n", envVar, value)
		shared.SetEnvAndSourceBashrcOrPowershell(envVar, value, true)
	} else {
		fmt.Printf("File doesn't exist or is empty: %s\n", path)
	}
}

func setEnvFromFile(filename string) {
	if err := shared.SetEnvVarsFromFile(filename); err != nil {
		fmt.Printf("Error setting env vars from %s: %v\n", filename, err)
	}
}
