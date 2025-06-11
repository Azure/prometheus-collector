package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus-collector/shared"
)

// PopulateSettingValuesFromConfigMap populates settings from the parsed configuration.
func (cp *ConfigProcessor) PopulateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) {
	// Extract the prometheus-collector-settings section
	if settings, ok := metricsConfigBySection["cluster_alias"]; ok {
		if value, ok := settings["default_metric_account_name"]; ok {
			cp.DefaultMetricAccountName = value
			fmt.Printf("Using configmap setting for default metric account name: %s\n", cp.DefaultMetricAccountName)
		}

		if value, ok := settings["cluster_alias"]; ok {
			cp.ClusterAlias = strings.TrimSpace(value)
			fmt.Printf("Got configmap setting for cluster_alias: %s\n", cp.ClusterAlias)
			if cp.ClusterAlias != "" {
				cp.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(cp.ClusterAlias, "_")
				cp.ClusterAlias = strings.Trim(cp.ClusterAlias, "_")
				fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", cp.ClusterAlias)
			}
		}

		if operatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED"); operatorEnabled != "" && strings.ToLower(operatorEnabled) == "true" {
			cp.IsOperatorEnabledChartSetting = true
			if value, ok := settings["operator_enabled"]; ok {
				cp.IsOperatorEnabled = value == "true"
				fmt.Printf("Configmap setting enabling operator: %t\n", cp.IsOperatorEnabled)
			}
		} else {
			cp.IsOperatorEnabledChartSetting = false
		}
	} else {
		cp.IsOperatorEnabledChartSetting = false
		fmt.Println("prometheus-collector-settings section not found in metricsConfigBySection, using defaults")
	}
}

// WriteConfigToFile writes the configuration settings to a file.
func (fcw *FileConfigWriter) WriteConfigToFile(filename string, configParser *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("AZMON_DEFAULT_METRIC_ACCOUNT_NAME=%s\n", configParser.DefaultMetricAccountName))
	file.WriteString(fmt.Sprintf("AZMON_CLUSTER_LABEL=%s\n", configParser.ClusterLabel))
	file.WriteString(fmt.Sprintf("AZMON_CLUSTER_ALIAS=%s\n", configParser.ClusterAlias))
	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED_CHART_SETTING=%t\n", configParser.IsOperatorEnabledChartSetting))
	if configParser.IsOperatorEnabled {
		file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED=%t\n", configParser.IsOperatorEnabled))
		file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING=%t\n", configParser.IsOperatorEnabled))
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
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/prometheus-collector-settings"},
		ConfigParser:   &ConfigProcessor{},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var",
	}

	configurator.Configure(metricsConfigBySection, schemaVersion)
}
