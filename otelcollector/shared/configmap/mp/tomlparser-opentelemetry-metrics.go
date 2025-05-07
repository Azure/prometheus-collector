package configmapsettings

import (
	"fmt"
	"os"
	"strings"
)

func ConfigureOpentelemetryMetricsSettings(metricsConfigBySection map[string]map[string]string) error {
	if metricsConfigBySection == nil {
		return fmt.Errorf("configmap section not mounted, using defaults")
	}
	enabled := populateSettingValuesFromConfigMap(metricsConfigBySection)

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
