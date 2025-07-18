package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/prometheus-collector/shared"
)

func Configmapparserforccp() {
	fmt.Printf("in configmapparserforccp")
	fmt.Printf("waiting for 30 secs...")
	time.Sleep(30 * time.Second) //needed to save a restart at times when config watcher sidecar starts up later than us and hence config map wasn't yet projected into emptydir volume yet during pod startups.

	processAndMergeConfigFiles()

	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// No need to merge custom prometheus config, only merging in the default configs
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml", "--output", "/opt/ccp-collector-config-with-defaults.yml", "--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")
	if !shared.Exists("/opt/ccp-collector-config-with-defaults.yml") {
		fmt.Println("prom-config-validator::Prometheus default scrape config validation failed. No scrape configs will be used")
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

func processAndMergeConfigFiles() {
	configVersionPath := configVersionFile
	configSchemaPath := schemaVersionFile

	entries, er := os.ReadDir(configSettingsPrefix)
	if er != nil {
		fmt.Println("error listing /etc/config/settings", er)
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}

	fmt.Println("done listing /etc/config/settings")

	if shared.ExistsAndNotEmpty(configSchemaPath) {
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
		fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_FILE_VERSION:", configVersion)
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_FILE_VERSION", configVersion, true)
	} else {
		fmt.Println("Configmapparserforccp fileversion file doesn't exist. or configmap doesn't exist:", configVersionPath)
	}

	// Set agent config file version
	if shared.ExistsAndNotEmpty(configVersionPath) {
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
		fmt.Println("Configmapparserforccp setting env var AZMON_AGENT_CFG_SCHEMA_VERSION:", configSchemaVersion)
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_AGENT_CFG_SCHEMA_VERSION", configSchemaVersion, true)
	} else {
		fmt.Println("Configmapparserforccp schemaversion file doesn't exist. or configmap doesn't exist:", configSchemaPath)
	}

	var metricsConfigBySection map[string]map[string]string
	var err error
	var schemaVersion = shared.ParseSchemaVersion(os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"))
	switch schemaVersion {
	case shared.SchemaVersion.V2:
		filePaths := []string{configSettingsPrefix + "controlplane-metrics", configSettingsPrefix + "prometheus-collector-settings"}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Error parsing files: %v\n", err)
			return
		}
	case shared.SchemaVersion.V1:
		configDir := configSettingsPrefix
		metricsConfigBySection, err = shared.ParseV1Config(configDir)
		if err != nil {
			fmt.Printf("Error parsing config: %v\n", err)
			return
		}
	default:
		fmt.Println("Invalid schema version or no configmap present. Using defaults.")
	}

	// Parse the configmap to set the right environment variables for prometheus collector settings
	parseConfigAndSetEnvInFile(metricsConfigBySection, schemaVersion)
	filename := collectorSettingsEnvVarPath
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for %s: %v\n", collectorSettingsEnvVarPath, err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	filename = defaultSettingsEnvVarPath
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for %s: %v\n", defaultSettingsEnvVarPath, err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection, schemaVersion)

	prometheusCcpConfigMerger()
}
