package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"strings"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

// PopulateSettingValuesFromConfigMap populates settings from the parsed configuration.
func (cp *ConfigProcessor) PopulateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) {
	fmt.Println("metricsConfigBySection:", metricsConfigBySection)

	sharedSettings := cmcommon.CollectorSettings{}
	cmcommon.PopulateSharedCollectorSettings(&sharedSettings, metricsConfigBySection, func(format string, args ...interface{}) {
		fmt.Printf(format, args...)
	})

	cp.DefaultMetricAccountName = sharedSettings.MetricAccountName
	cp.ClusterAlias = sharedSettings.ClusterAlias
	cp.ClusterLabel = sharedSettings.ClusterLabel
	cp.IsOperatorEnabled = sharedSettings.OperatorEnabled
	cp.IsOperatorEnabledChartSetting = sharedSettings.OperatorEnabledChart
}

// WriteConfigToFile writes the configuration settings to a file.
func (fcw *FileConfigWriter) WriteConfigToFile(filename string, configParser *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	sharedSettings := cmcommon.CollectorSettings{
		MetricAccountName:    configParser.DefaultMetricAccountName,
		ClusterAlias:         configParser.ClusterAlias,
		ClusterLabel:         configParser.ClusterLabel,
		OperatorEnabled:      configParser.IsOperatorEnabled,
		OperatorEnabledChart: configParser.IsOperatorEnabledChartSetting,
	}

	if err := cmcommon.WriteSharedCollectorSettings(file, sharedSettings); err != nil {
		return err
	}

	return nil
}

// Configure processes the configuration and writes it to a file.
func (c *Configurator) Configure(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	fmt.Printf("Start prometheus-collector-settings Processing\n")

	if configSchemaVersion == shared.SchemaVersion.V1 || configSchemaVersion == shared.SchemaVersion.V2 {
		if len(metricsConfigBySection) > 0 {
			c.ConfigParser.PopulateSettingValuesFromConfigMap(metricsConfigBySection)
		}
	} else {
		if _, err := os.Stat(c.ConfigLoader.ConfigMapMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	if mac := os.Getenv("MAC"); mac != "" && strings.TrimSpace(mac) == "true" {
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		c.ConfigParser.ClusterLabel = clusterArray[len(clusterArray)-1]
	} else {
		c.ConfigParser.ClusterLabel = os.Getenv("CLUSTER")
	}

	if c.ConfigParser.ClusterAlias != "" && len(c.ConfigParser.ClusterAlias) > 0 {
		c.ConfigParser.ClusterLabel = c.ConfigParser.ClusterAlias
		fmt.Printf("Using clusterLabel from cluster_alias: %s\n", c.ConfigParser.ClusterAlias)
	}

	fmt.Printf("AZMON_CLUSTER_ALIAS: '%s'\n", c.ConfigParser.ClusterAlias)
	fmt.Printf("AZMON_CLUSTER_LABEL: %s\n", c.ConfigParser.ClusterLabel)

	err := c.ConfigWriter.WriteConfigToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	fmt.Printf("End prometheus-collector-settings Processing\n")
}

// parseConfigAndSetEnvInFile initializes the configurator and processes the configuration.
func parseConfigAndSetEnvInFile(metricsConfigBySection map[string]map[string]string, schemaVersion string) {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: collectorSettingsMountPath},
		ConfigParser:   &ConfigProcessor{},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: collectorSettingsEnvVarPath,
	}

	configurator.Configure(metricsConfigBySection, schemaVersion)
}
