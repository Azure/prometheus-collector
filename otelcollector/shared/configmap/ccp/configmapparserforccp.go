package ccp

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus-collector/otelcollector/shared/configmap/core"
	"github.com/prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Println("Starting Control Plane Configuration...")

	// Wait for config watcher sidecar to project config maps
	fmt.Println("Waiting for 30 seconds to ensure config maps are projected...")
	time.Sleep(30 * time.Second)

	// Initialize the config manager
	configManager := core.NewConfigManager("controlplane")

	// Parse config map
	var metricsConfigBySection map[string]map[string]string
	var err error

	// Process config version and schema version
	processVersions()

	// Get schema version
	schemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	// Parse config map based on schema version
	if schemaVersion == "v2" {
		filePaths := []string{
			"/etc/config/settings/controlplane-metrics",
			"/etc/config/settings/prometheus-collector-settings",
		}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Error parsing metrics files: %v\n", err)
		}
	} else if schemaVersion == "v1" {
		configDir := "/etc/config/settings"
		metricsConfigBySection, err = shared.ParseV1Config(configDir)
		if err != nil {
			fmt.Printf("Error parsing V1 config: %v\n", err)
		}
	} else {
		fmt.Println("Invalid schema version or no schema version specified. Using defaults.")
	}

	// Process configurations
	shared.EchoSectionDivider("Start Processing - CCP Configuration")

	// Process prometheus collector config
	processPrometheusCollectorConfig(metricsConfigBySection)

	// Process default scrape settings
	if err := configManager.ProcessDefaultScrapeSettings(metricsConfigBySection); err != nil {
		shared.EchoError(fmt.Sprintf("Error processing default scrape settings: %v", err))
	}

	// Process metrics keep list
	if err := configManager.ProcessMetricsKeepList(metricsConfigBySection); err != nil {
		shared.EchoError(fmt.Sprintf("Error processing metrics keep list: %v", err))
	}

	// Process scrape intervals
	if err := configManager.ProcessScrapeIntervals(metricsConfigBySection); err != nil {
		shared.EchoError(fmt.Sprintf("Error processing scrape intervals: %v", err))
	}

	// Merge prometheus configurations
	if err := configManager.MergePrometheusConfigs(); err != nil {
		shared.EchoError(fmt.Sprintf("Error merging prometheus configs: %v", err))
	}

	// Validate and apply configuration
	if err := configManager.ValidateAndApplyConfig(); err != nil {
		shared.EchoError(fmt.Sprintf("Error validating configuration: %v", err))
	}

	shared.EchoSectionDivider("End Processing - CCP Configuration")
}

// processVersions processes the config version and schema version
func processVersions() {
	configVersionPath := "/etc/config/settings/config-version"
	configSchemaPath := "/etc/config/settings/schema-version"

	// Process config version
	if shared.FileExists(configVersionPath) {
		configVersion, err := shared.ReadFile(configVersionPath)
		if err == nil {
			trimmedVersion := strings.TrimSpace(string(configVersion))
			trimmedVersion = strings.ReplaceAll(trimmedVersion, " ", "")

			// Limit to 10 characters
			if len(trimmedVersion) > 10 {
				trimmedVersion = trimmedVersion[:10]
			}

			// Set environment variable
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", trimmedVersion, true)
		}
	}

	// Process schema version
	if shared.FileExists(configSchemaPath) {
		schemaVersion, err := shared.ReadFile(configSchemaPath)
		if err == nil {
			trimmedVersion := strings.TrimSpace(string(schemaVersion))
			trimmedVersion = strings.ReplaceAll(trimmedVersion, " ", "")

			// Limit to 10 characters
			if len(trimmedVersion) > 10 {
				trimmedVersion = trimmedVersion[:10]
			}

			// Set environment variable
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", trimmedVersion, true)
		}
	}
}

// processPrometheusCollectorConfig processes prometheus collector configuration
func processPrometheusCollectorConfig(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - Prometheus Collector Config")

	// Path for collector settings env vars
	collectorSettingsEnvVarPath := "/opt/microsoft/configmapparser/collector_settings_env_var"

	// Extract collector settings from config map
	settings := map[string]string{
		"AZMON_DEFAULT_METRIC_ACCOUNT_NAME":    "",
		"AZMON_CLUSTER_LABEL":                  os.Getenv("CLUSTER"),
		"AZMON_CLUSTER_ALIAS":                  "",
		"AZMON_OPERATOR_ENABLED_CHART_SETTING": "false",
		"AZMON_OPERATOR_ENABLED":               "false",
	}

	// Handle MAC cluster name
	if mac := os.Getenv("MAC"); mac == "true" {
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		settings["AZMON_CLUSTER_LABEL"] = clusterArray[len(clusterArray)-1]
	}

	// Get schema version
	schemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	if schemaVersion == "v1" || schemaVersion == "v2" {
		sectionName := "cluster_alias"

		if section, ok := metricsConfigBySection[sectionName]; ok {
			// Get default metric account name
			if value, ok := section["default_metric_account_name"]; ok {
				settings["AZMON_DEFAULT_METRIC_ACCOUNT_NAME"] = value
			}

			// Get cluster alias
			if value, ok := section["cluster_alias"]; ok {
				clusterAlias := strings.TrimSpace(value)
				if clusterAlias != "" {
					// Sanitize cluster alias
					sanitized := regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(clusterAlias, "_")
					sanitized = strings.Trim(sanitized, "_")
					settings["AZMON_CLUSTER_ALIAS"] = sanitized
					settings["AZMON_CLUSTER_LABEL"] = sanitized
				}
			}

			// Check operator enabled setting
			operatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")
			if operatorEnabled == "true" {
				settings["AZMON_OPERATOR_ENABLED_CHART_SETTING"] = "true"

				if value, ok := section["operator_enabled"]; ok {
					settings["AZMON_OPERATOR_ENABLED"] = value
				}
			}
		}
	}

	// Create collector settings env file
	file, err := os.Create(collectorSettingsEnvVarPath)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error creating collector settings env file: %v", err))
		return
	}
	defer file.Close()

	// Write settings to file and set environment variables
	for key, value := range settings {
		fmt.Fprintf(file, "%s=%s\n", key, value)
		shared.SetEnvAndSourceBashrcOrPowershell(key, value, true)
	}

	shared.EchoSectionDivider("End Processing - Prometheus Collector Config")
}
