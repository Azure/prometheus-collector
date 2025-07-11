package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"time"

	"github.com/prometheus-collector/shared"
)

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

	// Process config schema and version
	shared.ProcessConfigFile(configVersionPath, "AZMON_AGENT_CFG_FILE_VERSION")
	shared.ProcessConfigFile(configSchemaPath, "AZMON_AGENT_CFG_SCHEMA_VERSION")

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
