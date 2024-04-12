package configmapsettings

import (
	"fmt"
	"os"
	"strings"

	"io/fs"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

const (
	loggingPrefix             = "debug-mode-config"
	configMapDebugMountPath   = "/etc/config/settings/debug-mode"
	replicaSetCollectorConfig = "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
)

var (
	defaultEnabled = false
)

// ConfigureDebugModeSettings reads debug mode settings from a config map,
// sets default values if necessary, writes environment variables to a file,
// and modifies a YAML configuration file based on debug mode settings.
func ConfigureDebugModeSettings() {
	fmt.Println("Start debug-mode Settings Processing")

	configMapSettings, err := parseConfigMapForDebugSettings()
	if err != nil {
		fmt.Fprint(os.Stdout, []any{"Error: %v", err}...)
	}
	if configMapSettings != nil {
		populateSettingValuesFromConfigMap(configMapSettings)
	}

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		if _, err := os.Stat(configMapDebugMountPath); os.IsNotExist(err) {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	file, err := os.Create("/opt/microsoft/configmapparser/config_debug_mode_env_var")
	if err != nil {
		fmt.Printf("Exception while opening file for writing prometheus-collector config environment variables: %s\n", err)
		return
	}
	defer file.Close()

	if os.Getenv("OS_TYPE") != "" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
		file.WriteString(fmt.Sprintf("export DEBUG_MODE_ENABLED=%v\n", defaultEnabled))
	} else {
		file.WriteString(fmt.Sprintf("DEBUG_MODE_ENABLED=%v\n", defaultEnabled))
	}

	if defaultEnabled {
		controllerType := os.Getenv("CONTROLLER_TYPE")
		if controllerType != "" && controllerType == "ReplicaSet" {
			fmt.Println("Setting otlp in the exporter metrics for service pipeline since debug mode is enabled ...")
			var config map[string]interface{}
			content, err := os.ReadFile(replicaSetCollectorConfig)
			if err != nil {
				fmt.Printf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %s\n", err)
				return
			}

			err = yaml.Unmarshal(content, &config)
			if err != nil {
				fmt.Printf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %s\n", err)
				return
			}

			if config != nil {
				exporters := []string{"otlp", "prometheus"}
				config["service"].(map[interface{}]interface{})["pipelines"].(map[interface{}]interface{})["metrics"].(map[interface{}]interface{})["exporters"] = exporters

				cfgYamlWithDebugModeSettings, err := yaml.Marshal(config)
				if err != nil {
					fmt.Printf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %s\n", err)
					return
				}

				err = os.WriteFile(replicaSetCollectorConfig, []byte(cfgYamlWithDebugModeSettings), fs.FileMode(0644))
				if err != nil {
					fmt.Printf("Exception while setting otlp in the exporter metrics for service pipeline when debug mode is enabled - %s\n", err)
					return
				}
			}
			fmt.Println("Done setting otlp in the exporter metrics for service pipeline.")
		}
	}

	fmt.Println("End debug-mode Settings Processing")
}

func parseConfigMapForDebugSettings() (map[string]interface{}, error) {
	if data, err := os.ReadFile(configMapMountPath); err == nil {
		parsedConfig := make(map[string]interface{})
		if err := toml.Unmarshal(data, &parsedConfig); err == nil {
			return parsedConfig, nil
		} else {
			return nil, fmt.Errorf("exception while parsing config map for debug mode: %v, using defaults, please check config map for errors\n", err)
		}
	} else {
		return nil, fmt.Errorf("error reading config map file: %v", err)
	}
}

func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}) {
	if val, ok := parsedConfig["enabled"]; ok {
		defaultEnabled = val.(bool)
		fmt.Printf("Using configmap setting for debug mode: %v\n", defaultEnabled)
	}
}
