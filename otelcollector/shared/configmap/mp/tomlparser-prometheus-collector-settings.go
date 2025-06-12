package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (cp *ConfigProcessor) PopulateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) {
	// Populate default metric account name
	if settings, ok := metricsConfigBySection["prometheus-collector-settings"]; ok {
		if value, ok := settings["default_metric_account_name"]; ok {
			cp.DefaultMetricAccountName = value
			fmt.Printf("Using configmap setting for default metric account name: %s\n", cp.DefaultMetricAccountName)
		}
	}

	// Populate cluster alias
	if settings, ok := metricsConfigBySection["prometheus-collector-settings"]; ok {
		if value, ok := settings["cluster_alias"]; ok {
			cp.ClusterAlias = strings.TrimSpace(value)
			fmt.Printf("Got configmap setting for cluster_alias: %s\n", cp.ClusterAlias)
			if cp.ClusterAlias != "" {
				cp.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(cp.ClusterAlias, "_")
				cp.ClusterAlias = strings.Trim(cp.ClusterAlias, "_")
				fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", cp.ClusterAlias)
			}
		}
	}

	// Populate operator settings
	if operatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED"); operatorEnabled != "" && strings.ToLower(operatorEnabled) == "true" {
		cp.IsOperatorEnabledChartSetting = true
		if settings, ok := metricsConfigBySection["prometheus-collector-settings"]; ok {
			if value, ok := settings["operator_enabled"]; ok {
				cp.IsOperatorEnabled = value == "true"
				fmt.Printf("Configmap setting enabling operator: %t\n", cp.IsOperatorEnabled)
			}
		}
	} else {
		cp.IsOperatorEnabledChartSetting = false
	}

	if operatorHttpsEnabled := os.Getenv("OPERATOR_TARGETS_HTTPS_ENABLED"); operatorHttpsEnabled != "" && strings.ToLower(operatorHttpsEnabled) == "true" {
		cp.TargetallocatorHttpsEnabledChartSetting = true
		cp.TargetallocatorHttpsEnabled = true
		if settings, ok := metricsConfigBySection["prometheus-collector-settings"]; ok {
			if value, ok := settings["https_config"]; ok {
				if strings.ToLower(value) == "false" {
					cp.TargetallocatorHttpsEnabled = false
				}
				fmt.Printf("Configmap setting enabling https between TargetAllocator and Replicaset: %s\n", value)
			}
			fmt.Printf("Effective value for enabling https between TargetAllocator and Replicaset: %t\n", cp.TargetallocatorHttpsEnabled)
		}
	} else {
		cp.TargetallocatorHttpsEnabledChartSetting = false
		cp.TargetallocatorHttpsEnabled = false
	}
}

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
	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_HTTPS_ENABLED_CHART_SETTING=%t\n", configParser.TargetallocatorHttpsEnabledChartSetting))
	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_HTTPS_ENABLED=%t\n", configParser.TargetallocatorHttpsEnabled))
	return nil
}

func (c *Configurator) Configure(metricsConfigBySection map[string]map[string]string) {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Printf("Configure:Print the value of AZMON_AGENT_CFG_SCHEMA_VERSION: %s\n", configSchemaVersion)

	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
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
}

func parseConfigAndSetEnvInFile(metricsConfigBySection map[string]map[string]string) {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: collectorSettingsMountPath},
		ConfigParser:   &ConfigProcessor{},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: collectorSettingsEnvVarPath,
	}

	configurator.Configure(metricsConfigBySection)
}
