package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (fcl *FilesystemConfigLoader) ParseConfigMap() (map[string]string, error) {
	config := make(map[string]string)

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmapprometheus-collector-configmap for prometheus collector settings not mounted, using defaults")
		return config, nil
	}

	content, err := os.ReadFile(fcl.ConfigMapMountPath)
	if err != nil {
		fmt.Printf("Error reading config map file: %s, using defaults, please check config map for errors\n", err)
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return config, nil
}

func (cp *ConfigProcessor) PopulateSettingValuesFromConfigMap(parsedConfig map[string]string) {
	if value, ok := parsedConfig["default_metric_account_name"]; ok {
		cp.DefaultMetricAccountName = value
		fmt.Printf("Using configmap setting for default metric account name: %s\n", cp.DefaultMetricAccountName)
	}

	if value, ok := parsedConfig["cluster_alias"]; ok {
		cp.ClusterAlias = strings.TrimSpace(value)
		fmt.Printf("Got configmap setting for cluster_alias: %s\n", cp.ClusterAlias)
		// Only perform the replacement if cp.ClusterAlias is not an empty string
		if cp.ClusterAlias != "" {
			cp.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(cp.ClusterAlias, "_")
			cp.ClusterAlias = strings.Trim(cp.ClusterAlias, "_") // Trim underscores from the beginning and end (since cluster_alias is being passed in as "" which are being replaced with _)
			fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", cp.ClusterAlias)
		}
	}

	if operatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED"); operatorEnabled != "" && strings.ToLower(operatorEnabled) == "true" {
		cp.IsOperatorEnabledChartSetting = true
		if value, ok := parsedConfig["operator_enabled"]; ok {
			if value == "true" {
				cp.IsOperatorEnabled = true
			} else {
				cp.IsOperatorEnabled = false
			}
			fmt.Printf("Configmap setting enabling operator: %s\n", cp.IsOperatorEnabled)
		}
	} else {
		cp.IsOperatorEnabledChartSetting = false
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
	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED_CHART_SETTING=%s\n", configParser.IsOperatorEnabledChartSetting))
	if configParser.IsOperatorEnabled {
		file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED=%s\n", configParser.IsOperatorEnabled))
		file.WriteString(fmt.Sprintf("AZMON_OPERATOR_ENABLED_CFG_MAP_SETTING=%s\n", configParser.IsOperatorEnabled))
	}
	return nil
}
func (c *Configurator) Configure() {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	fmt.Println("Start prometheus-collector-settings Processing")
	fmt.Printf("Configure:Print the value of AZMON_AGENT_CFG_SCHEMA_VERSION: %s\n", os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"))

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings, err := c.ConfigLoader.ParseConfigMap()
		if err == nil && len(configMapSettings) > 0 {
			c.ConfigParser.PopulateSettingValuesFromConfigMap(configMapSettings)
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

	fmt.Println("End prometheus-collector-settings Processing")
}

func parseConfigAndSetEnvInFile() {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/prometheus-collector-settings"},
		ConfigParser:   &ConfigProcessor{},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_prometheus_collector_settings_env_var",
	}

	configurator.Configure()
}
