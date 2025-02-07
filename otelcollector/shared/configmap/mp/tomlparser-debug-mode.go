package configmapsettings

import (
	"fmt"
	"os"
	"strings"

	"io/fs"

	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix = "debug-mode-config"
)

// ConfigureDebugModeSettings reads debug mode settings from the parsed config map,
// sets default values if necessary, writes environment variables to a file,
// and modifies a YAML configuration file based on debug mode settings.
func ConfigureDebugModeSettings(metricsConfigBySection map[string]map[string]string) error {
	if metricsConfigBySection == nil {
		return fmt.Errorf("parsed config map data is nil")
	}

	enabled := populateSettingValuesFromConfigMap(metricsConfigBySection)

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		if _, err := os.Stat(configMapDebugMountPath); os.IsNotExist(err) {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	file, err := os.Create(debugModeEnvVarPath)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %v\n", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("DEBUG_MODE_ENABLED=%v\n", enabled))
	fmt.Printf("Setting debug mode environment variable: %v\n", enabled)

	if enabled {
		controllerType := os.Getenv("CONTROLLER_TYPE")
		if controllerType != "" && controllerType == "ReplicaSet" {
			fmt.Println("Setting otlp in the exporter metrics for service pipeline since debug mode is enabled ...")
			var config map[string]interface{}
			content, err := os.ReadFile(replicaSetCollectorConfig)
			if err != nil {
				return fmt.Errorf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			err = yaml.Unmarshal(content, &config)
			if err != nil {
				return fmt.Errorf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			if config != nil {
				exporters := []string{"otlp", "prometheus"}
				config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})["metrics"].(map[interface{}]interface{})["exporters"] = exporters

				cfgYamlWithDebugModeSettings, err := yaml.Marshal(config)
				if err != nil {
					return fmt.Errorf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
				}

				err = os.WriteFile(replicaSetCollectorConfig, []byte(cfgYamlWithDebugModeSettings), fs.FileMode(0644))
				if err != nil {
					return fmt.Errorf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
				}
			}
			fmt.Println("Done setting otlp in the exporter metrics for service pipeline.")
		}
	}

	return nil
}

func populateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) bool {
	if len(metricsConfigBySection) == 0 {
		fmt.Println("Parsed config map is empty. Using default debug mode value: false")
		return false
	}

	debugSettings, ok := metricsConfigBySection["debug-mode"]
	if !ok {
		fmt.Println("The 'debug-mode' section is not present in the parsed data. Using default value: false")
		return false
	}

	val, ok := debugSettings["enabled"]
	if !ok {
		fmt.Println("The 'enabled' key is missing in the 'debug-mode-config' section. Using default value: false")
		return false
	}

	enabled := strings.ToLower(val) == "true"
	fmt.Printf("Using configmap setting for debug mode: %v\n", enabled)
	return enabled
}
