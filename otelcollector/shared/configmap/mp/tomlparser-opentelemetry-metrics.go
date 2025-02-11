package configmapsettings

import (
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
)

func ConfigureOpentelemetryMetricsSettings() error {
	configMapSettings, err := parseConfigMapForOpentelemetryMetricsSettings()
	if err != nil || configMapSettings == nil {
		return fmt.Errorf("Error: %v", err)
	}
	enabled := populateSettingValuesFromConfigMap(configMapSettings)

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		if _, err := os.Stat(configMapOpentelemetryMetricsMountPath); os.IsNotExist(err) {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	file, err := os.Create(opentelemetryMetricsEnvVarPath)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %v\n", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("AZMON_FULL_OTLP_ENABLED=%v\n", enabled))

	fmt.Printf("Setting AZMON_FULL_OTLP_ENABLED environment variable: %v\n", enabled)

	return nil
}

func parseConfigMapForOpentelemetryMetricsSettings() (map[string]interface{}, error) {
	// Check if config map file exists
	file, err := os.Open(configMapOpentelemetryMetricsMountPath)
	if err != nil {
		return nil, fmt.Errorf("configmap section not mounted, using defaults")
	}
	defer file.Close()

	if data, err := os.ReadFile(configMapOpentelemetryMetricsMountPath); err == nil {
		parsedConfig := make(map[string]interface{})
		if err := toml.Unmarshal(data, &parsedConfig); err == nil {
			return parsedConfig, nil
		} else {
			return nil, fmt.Errorf("exception while parsing config map: %v, using defaults, please check config map for errors", err)
		}
	} else {
		return nil, fmt.Errorf("error reading config map file: %v", err)
	}
}

func populateOpentelemetryMetricsSettingValuesFromConfigMap(parsedConfig map[string]interface{}) bool {
	enabled := false
	if val, ok := parsedConfig["enabled"]; ok {
		enabled = val.(bool)
		fmt.Printf("Using configmap setting for opentelemetry-metrics: %v\n", enabled)
	} else {
		fmt.Printf("OpentelemetryMetrics configmap does not have enabled value, using default value: %v\n", enabled)
	}
	return enabled
}
