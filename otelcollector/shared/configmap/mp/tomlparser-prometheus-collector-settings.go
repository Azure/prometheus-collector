package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func parseSettingsConfigMap() (map[string]string, error) {
	config := make(map[string]string)

	if _, err := os.Stat(ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmapprometheus-collector-configmap not mounted, using defaults")
		return config, nil
	}

	content, err := os.ReadFile(ConfigMapMountPath)
	if err != nil {
		fmt.Printf("Error reading config map file: %s, using defaults\n", err)
		return nil, err
	}

	for _, line := range strings.Split(string(content), "\n") {
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

		if cp.ClusterAlias != "" {
			// Clean cluster alias to only contain alphanumeric characters and underscores
			cp.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(cp.ClusterAlias, "_")
			cp.ClusterAlias = strings.Trim(cp.ClusterAlias, "_")
			fmt.Printf("After replacement: %s\n", cp.ClusterAlias)
		}
	}

	operatorEnabled := strings.ToLower(os.Getenv("AZMON_OPERATOR_ENABLED"))
	cp.IsOperatorEnabledChartSetting = (operatorEnabled == "true")

	if cp.IsOperatorEnabledChartSetting {
		if value, ok := parsedConfig["operator_enabled"]; ok {
			cp.IsOperatorEnabled = (value == "true")
			fmt.Printf("Configmap setting enabling operator: %t\n", cp.IsOperatorEnabled)
		}
	}
}

func (fcw *FileConfigWriter) WriteConfigToFile(filename string, configParser *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("exception while opening file: %s", err)
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

func configure() {

	// Set cluster label
	if mac := os.Getenv("MAC"); strings.TrimSpace(mac) == "true" {
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		c.ConfigParser.ClusterLabel = clusterArray[len(clusterArray)-1]
	} else {
		c.ConfigParser.ClusterLabel = os.Getenv("CLUSTER")
	}

	// Override cluster label with alias if available
	if c.ConfigParser.ClusterAlias != "" {
		c.ConfigParser.ClusterLabel = c.ConfigParser.ClusterAlias
		fmt.Printf("Using clusterLabel from cluster_alias: %s\n", c.ConfigParser.ClusterAlias)
	}

	fmt.Printf("AZMON_CLUSTER_ALIAS: '%s'\n", c.ConfigParser.ClusterAlias)
	fmt.Printf("AZMON_CLUSTER_LABEL: %s\n", c.ConfigParser.ClusterLabel)

	// Write config to file
	if err := c.ConfigWriter.WriteConfigToFile(c.ConfigFilePath, c.ConfigParser); err != nil {
		fmt.Printf("%v\n", err)
	}
}

func parseConfigAndSetEnvInFile(schemaVersion string) {
	fmt.Printf("Configure: AZMON_AGENT_CFG_SCHEMA_VERSION=%s\n", schemaVersion)

	// Process config map if schema version is v1
	if schemaVersion == "v1" {
		if configMapSettings, err := parseSettingsConfigMap(); err == nil && len(configMapSettings) > 0 {
			c.ConfigParser.PopulateSettingValuesFromConfigMap(configMapSettings)
		}
	} else if _, err := os.Stat(c.ConfigLoader.ConfigMapMountPath); err == nil {
		fmt.Printf("Unsupported/missing schema version - '%s', using defaults\n", schemaVersion)
	}

	configure()
}
