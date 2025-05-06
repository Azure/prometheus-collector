package configmapsettings

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

// ConfigureDebugModeSettings configures debug mode based on config map settings
func ConfigureDebugModeSettings() error {
	configMapSettings, err := parseConfigMapForDebugSettings()
	if err != nil || configMapSettings == nil {
		return fmt.Errorf("Error parsing debug settings: %v", err)
	}

	enabled := populateSettingValuesFromConfigMap(configMapSettings)

	// Check config schema version
	if configSchema := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"); configSchema == "v1" {
		if _, err := os.Stat(configMapDebugMountPath); os.IsNotExist(err) {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults\n", configSchema)
		}
	}

	// Write debug mode environment variable
	file, err := os.Create(debugModeEnvVarPath)
	if err != nil {
		return fmt.Errorf("Exception while writing environment variables: %v", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("DEBUG_MODE_ENABLED=%v\n", enabled))
	fmt.Printf("Setting debug mode environment variable: %v\n", enabled)

	// Configure ReplicaSet collector if debug mode is enabled
	if enabled && os.Getenv("CONTROLLER_TYPE") == "ReplicaSet" {
		fmt.Println("Setting prometheus in the exporter metrics for service pipeline...")
		if err := updateReplicaSetConfig(); err != nil {
			return err
		}
		fmt.Println("Done setting prometheus in the exporter metrics for service pipeline.")
	}

	return nil
}

func updateReplicaSetConfig() error {
	var config OtelConfig
	content, err := os.ReadFile(replicaSetCollectorConfig)
	if err != nil {
		return fmt.Errorf("Error reading collector config: %v", err)
	}

	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("Error parsing collector config: %v", err)
	}

	config.Service.Pipelines.Metrics.Exporters = []interface{}{"otlp", "prometheus"}

	if os.Getenv("CCP_METRICS_ENABLED") != "true" {
		config.Service.Pipelines.MetricsTelemetry.Receivers = []interface{}{"prometheus"}
		config.Service.Pipelines.MetricsTelemetry.Exporters = []interface{}{"prometheus/telemetry"}
		config.Service.Pipelines.MetricsTelemetry.Processors = []interface{}{"filter/telemetry"}
	}

	cfgYaml, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("Error marshaling updated config: %v", err)
	}

	return os.WriteFile(replicaSetCollectorConfig, cfgYaml, fs.FileMode(0644))
}

func parseConfigMapForDebugSettings() (map[string]interface{}, error) {
	if _, err := os.Stat(configMapDebugMountPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configmap section not mounted, using defaults")
	}

	data, err := os.ReadFile(configMapDebugMountPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config map file: %v", err)
	}

	parsedConfig := make(map[string]interface{})
	if err := toml.Unmarshal(data, &parsedConfig); err != nil {
		return nil, fmt.Errorf("exception parsing config map: %v", err)
	}

	return parsedConfig, nil
}

func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}) bool {
	if val, ok := parsedConfig["enabled"]; ok {
		enabled := val.(bool)
		fmt.Printf("Using configmap setting for debug mode: %v\n", enabled)
		return enabled
	}

	fmt.Println("Debug mode configmap missing enabled value, using default: false")
	return false
}
