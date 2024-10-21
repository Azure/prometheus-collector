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

// PopulateSharedSettings populates shared config settings, such as cluster_alias
func (cp *ConfigProcessor) PopulateSharedSettings(parsedConfig map[string]string) {
	if value, ok := parsedConfig["cluster_alias"]; ok {
		cp.ClusterAlias = strings.TrimSpace(value)
		fmt.Printf("Got configmap setting for cluster_alias: %s\n", cp.ClusterAlias)
		if cp.ClusterAlias != "" {
			cp.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(cp.ClusterAlias, "_")
			cp.ClusterAlias = strings.Trim(cp.ClusterAlias, "_")
			fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", cp.ClusterAlias)
		}
	}
}

// PopulateSchemaSpecificSettings handles schema-specific settings for both v1 and v2
func (cp *ConfigProcessor) PopulateSchemaSpecificSettings(parsedConfig map[string]string, schemaVersion string) {
	if schemaVersion == "v1" {
		// Handle v1-specific settings
		if value, ok := parsedConfig["default_metric_account_name"]; ok {
			cp.DefaultMetricAccountName = value
			fmt.Printf("Using v1 configmap setting for default metric account name: %s\n", cp.DefaultMetricAccountName)
		}
	} else if schemaVersion == "v2" {
		// Handle v2-specific settings
		fmt.Println("Using v2 schema for additional settings")
		// Example: Populate other v2-specific settings here
		if value, ok := parsedConfig["new_v2_setting"]; ok {
			fmt.Printf("Got new_v2_setting: %s\n", value)
			// Populate the specific v2 setting
		}
	}
}

// WriteConfigToFile writes the processed config to a file
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

// Configure is the main method that parses and applies the config
func (c *Configurator) Configure() {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	fmt.Printf("Configure: Print the value of AZMON_AGENT_CFG_SCHEMA_VERSION: %s\n", configSchemaVersion)

	// Parse the config map
	configMapSettings, err := c.ConfigLoader.ParseConfigMap()
	if err != nil || len(configMapSettings) == 0 {
		fmt.Println("No config map settings found, using defaults")
		return
	}

	// Populate shared settings (e.g., cluster_alias)
	c.ConfigParser.PopulateSharedSettings(configMapSettings)

	// Handle schema-specific settings
	if configSchemaVersion != "" {
		c.ConfigParser.PopulateSchemaSpecificSettings(configMapSettings, strings.TrimSpace(configSchemaVersion))
	} else {
		fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
	}

	// Handle cluster label logic
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

	// Write the settings to the file
	err = c.ConfigWriter.WriteConfigToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}
}

// parseConfigAndSetEnvInFile is the main entry point to configure and write the config
func parseConfigAndSetEnvInFile() {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: collectorSettingsMountPath},
		ConfigParser:   &ConfigProcessor{},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: collectorSettingsEnvVarPath,
	}

	configurator.Configure()
}
