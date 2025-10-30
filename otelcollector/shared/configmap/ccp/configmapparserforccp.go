package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"time"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

const (
	defaultConfigSchemaVersion = "v1"
	defaultConfigFileVersion   = "ver1"
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
	entries, er := os.ReadDir(configSettingsPrefix)
	if er != nil {
		fmt.Println("error listing /etc/config/settings", er)
	}

	for _, e := range entries {
		fmt.Println(e.Name())
	}

	fmt.Println("done listing /etc/config/settings")

	schemaVersion := cmcommon.SetEnvFromFile(schemaVersionFile, "AZMON_AGENT_CFG_SCHEMA_VERSION", defaultConfigSchemaVersion)
	cmcommon.SetEnvFromFile(configVersionFile, "AZMON_AGENT_CFG_FILE_VERSION", defaultConfigFileVersion)

	metricsConfigBySection, err := cmcommon.LoadMetricsConfiguration(
		schemaVersion,
		[]string{configSettingsPrefix + "controlplane-metrics", configSettingsPrefix + "prometheus-collector-settings"},
		configSettingsPrefix,
	)
	if err != nil {
		fmt.Printf("Error parsing config: %v\n", err)
		return
	}

	parsedSchema := shared.ParseSchemaVersion(schemaVersion)

	// Parse the configmap to set the right environment variables for prometheus collector settings
	parseConfigAndSetEnvInFile(metricsConfigBySection, parsedSchema)
	filename := collectorSettingsEnvVarPath
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for %s: %v\n", collectorSettingsEnvVarPath, err)
	}

	ConfigureOpentelemetryMetricsSettings(metricsConfigBySection)
	filename = "/opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var"
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for /opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var: %v\n", err)
	}

	// Parse the settings for default scrape configs
	tomlparserCCPDefaultScrapeSettings(metricsConfigBySection, parsedSchema)
	filename = defaultSettingsEnvVarPath
	err = shared.SetEnvVarsFromFile(filename)
	if err != nil {
		fmt.Printf("Error when settinng env for %s: %v\n", defaultSettingsEnvVarPath, err)
	}

	// Parse the settings for default targets metrics keep list config
	tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection, parsedSchema)

	prometheusCcpConfigMerger()
}
