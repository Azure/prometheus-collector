package configmapsettings

import (
	"fmt"
	"log"
	"os"
	"strings"

	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

func (cp *ConfigProcessor) PopulateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) {
	sharedSettings := cmcommon.CollectorSettings{}
	cmcommon.PopulateSharedCollectorSettings(&sharedSettings, metricsConfigBySection, func(format string, args ...interface{}) {
		log.Printf(format, args...)
	})

	cp.DefaultMetricAccountName = sharedSettings.MetricAccountName
	cp.ClusterAlias = sharedSettings.ClusterAlias
	cp.ClusterLabel = sharedSettings.ClusterLabel
	cp.IsOperatorEnabled = sharedSettings.OperatorEnabled
	cp.IsOperatorEnabledChartSetting = sharedSettings.OperatorEnabledChart

	if operatorHttpsEnabled := os.Getenv("OPERATOR_TARGETS_HTTPS_ENABLED"); operatorHttpsEnabled != "" && strings.ToLower(operatorHttpsEnabled) == "true" {
		cp.TargetallocatorHttpsEnabledChartSetting = true
		cp.TargetallocatorHttpsEnabled = true
		if settings, ok := metricsConfigBySection["prometheus-collector-settings"]; ok {
			if value, ok := settings["https_config"]; ok {
				if strings.ToLower(value) == "false" {
					cp.TargetallocatorHttpsEnabled = false
				}
				log.Printf("Configmap setting enabling https between TargetAllocator and Replicaset: %s\n", value)
			}
			log.Printf("Effective value for enabling https between TargetAllocator and Replicaset: %t\n", cp.TargetallocatorHttpsEnabled)
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

	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_HTTPS_ENABLED_CHART_SETTING=%t\n", configParser.TargetallocatorHttpsEnabledChartSetting))
	file.WriteString(fmt.Sprintf("AZMON_OPERATOR_HTTPS_ENABLED=%t\n", configParser.TargetallocatorHttpsEnabled))
	return nil
}

func (c *Configurator) Configure(metricsConfigBySection map[string]map[string]string) {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	log.Printf("Configure:Print the value of AZMON_AGENT_CFG_SCHEMA_VERSION: %s\n", configSchemaVersion)

	trimmedSchemaVersion := strings.TrimSpace(configSchemaVersion)
	if trimmedSchemaVersion != "" {
		normalizedSchema := strings.ToLower(trimmedSchemaVersion)
		if normalizedSchema == "v1" || normalizedSchema == "v2" {
			c.ConfigParser.PopulateSettingValuesFromConfigMap(metricsConfigBySection)
		}
	} else {
		if _, err := os.Stat(c.ConfigLoader.ConfigMapMountPath); err == nil {
			log.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
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
		log.Printf("Using clusterLabel from cluster_alias: %s\n", c.ConfigParser.ClusterAlias)
	}

	log.Printf("AZMON_CLUSTER_ALIAS: '%s'\n", c.ConfigParser.ClusterAlias)
	log.Printf("AZMON_CLUSTER_LABEL: %s\n", c.ConfigParser.ClusterLabel)

	err := c.ConfigWriter.WriteConfigToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
}

func parseConfigAndSetEnvInFile(metricsConfigBySection map[string]map[string]string) {

	operatorHttpsEnabled := false
	if strings.ToLower(os.Getenv("OPERATOR_TARGETS_HTTPS_ENABLED")) == "true" {
		operatorHttpsEnabled = true
	}

	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: collectorSettingsMountPath},
		ConfigParser:   &ConfigProcessor{TargetallocatorHttpsEnabled: operatorHttpsEnabled},
		ConfigWriter:   &FileConfigWriter{ConfigProcessor: &ConfigProcessor{}},
		ConfigFilePath: collectorSettingsEnvVarPath,
	}

	configurator.Configure(metricsConfigBySection)
}
