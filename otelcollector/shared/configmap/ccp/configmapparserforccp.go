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

	// Process config schema and version
	shared.ProcessConfigFile(configVersionPath, "AZMON_AGENT_CFG_FILE_VERSION")
	shared.ProcessConfigFile(configSchemaPath, "AZMON_AGENT_CFG_SCHEMA_VERSION")

	var metricsConfigBySection map[string]map[string]string
	var err error
	var schemaVersion = shared.ParseSchemaVersion(os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"))
	switch schemaVersion {
	case shared.SchemaVersion.V2:
		filePaths := []string{"/etc/config/settings/controlplane-metrics", "/etc/config/settings/prometheus-collector-settings"}
		metricsConfigBySection, err = shared.ParseMetricsFiles(filePaths)
		if err != nil {
			fmt.Printf("Error parsing files: %v\n", err)
			return
		}
	case shared.SchemaVersion.V1:
		configDir := "/etc/config/settings"
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
	filename := "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	filename = "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_default_scrape_settings_env_var: %v\n", err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection, schemaVersion)

	prometheusCcpConfigMerger()

	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)


	// No need to merge custom prometheus config, only merging in the default configs
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	shared.StartCommandAndWait("/opt/promconfigvalidator", "--config", "/opt/defaultsMergedConfig.yml",
		"--output", "/opt/ccp-collector-config-with-defaults.yml",
		"--otelTemplate", "/opt/microsoft/otelcollector/ccp-collector-config-template.yml")
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
}
