package ccpconfigmapsettings

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

var (
	opentelemetryMetricsEnvVarPath = "/opt/microsoft/configmapparser/config_opentelemetry_metrics_env_var"
)

func ConfigureOpentelemetryMetricsSettings(metricsConfigBySection map[string]map[string]string) error {
	if metricsConfigBySection == nil {
		return fmt.Errorf("configmap section not mounted, using defaults")
	}

	enabled := populateOpentelemetryMetricsSettingValuesFromConfigMap(metricsConfigBySection)

	file, err := os.Create(opentelemetryMetricsEnvVarPath)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %v\n", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("AZMON_FULL_OTLP_ENABLED=%v\n", enabled))

	log.Printf("Setting AZMON_FULL_OTLP_ENABLED environment variable: %v\n", enabled)

	return nil
}

func populateOpentelemetryMetricsSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) bool {
	enabled := false

	// Access the nested map and value
	innerMap, ok := metricsConfigBySection["opentelemetry-metrics"]
	if !ok {
		return enabled
	}

	if val, ok := innerMap["enabled"]; ok {
		enabledBool, err := strconv.ParseBool(val)
		if err != nil {
			log.Printf("Invalid value for opentelemetry-metrics enabled: %s, defaulting to %b\n", enabled)
			return enabled
		}
		enabled = enabledBool
		log.Printf("Using configmap setting for opentelemetry-metrics: %v\n", enabled)
	}
	return enabled
}
