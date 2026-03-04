package configmapsettings

import (
	"fmt"
	"log"
	"os"
	"strings"

	"io/fs"

	"github.com/prometheus-collector/shared"
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
		return fmt.Errorf("configmap section not mounted, using defaults")
	}

	enabled := populateSettingValuesFromConfigMap(metricsConfigBySection)

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		if _, err := os.Stat(configMapDebugMountPath); os.IsNotExist(err) {
			log.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	file, err := os.Create(debugModeEnvVarPath)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %v\n", err)
	}
	defer file.Close()

	//if os.Getenv("OS_TYPE") != "" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
	//file.WriteString(fmt.Sprintf("export DEBUG_MODE_ENABLED=%v\n", defaultEnabled))
	//} else {
	file.WriteString(fmt.Sprintf("DEBUG_MODE_ENABLED=%v\n", enabled))

	log.Printf("Setting debug mode environment variable: %v\n", enabled)
	//}

	if enabled {
		controllerType := os.Getenv("CONTROLLER_TYPE")
		if controllerType != "" && controllerType == "ReplicaSet" {
			log.Println("Setting prometheus in the exporter metrics for service pipeline since debug mode is enabled ...")
			var config shared.OtelConfig
			content, err := os.ReadFile(replicaSetCollectorConfig)
			if err != nil {
				return fmt.Errorf("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			err = yaml.Unmarshal(content, &config)
			if err != nil {
				return fmt.Errorf("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			config.Service.Pipelines.Metrics.Exporters = []interface{}{"otlp_grpc", "prometheus"}
			if os.Getenv("CCP_METRICS_ENABLED") != "true" {
				config.Service.Pipelines.MetricsTelemetry.Receivers = []interface{}{"prometheus"}
				config.Service.Pipelines.MetricsTelemetry.Exporters = []interface{}{"prometheus/telemetry"}
				config.Service.Pipelines.MetricsTelemetry.Processors = []interface{}{"filter/telemetry"}
			}
			cfgYamlWithDebugModeSettings, err := yaml.Marshal(config)
			if err != nil {
				return fmt.Errorf("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			err = os.WriteFile(replicaSetCollectorConfig, []byte(cfgYamlWithDebugModeSettings), fs.FileMode(0644))
			if err != nil {
				return fmt.Errorf("Exception while setting prometheus in the exporter metrics for service pipeline when debug mode is enabled - %v\n", err)
			}

			log.Println("Done setting prometheus in the exporter metrics for service pipeline.")
		}
	}

	return nil
}

func populateSettingValuesFromConfigMap(metricsConfigBySection map[string]map[string]string) bool {
	debugSettings := "false"
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	if configSchemaVersion == "" {
		log.Println("AZMON_AGENT_CFG_SCHEMA_VERSION environment variable is not set. Using default value: false")
		return false
	}

	configSchemaVersion = strings.TrimSpace(configSchemaVersion)

	if configSchemaVersion == "v1" {
		debugModeSection, ok := metricsConfigBySection["debug-mode"]
		if !ok {
			log.Println("The 'debug-mode' section is not present in the parsed data. Using default value: false")
			return false
		}

		val, ok := debugModeSection["enabled"]
		if !ok {
			log.Println("The 'enabled' field in 'debug-mode' section is not present in the parsed data. Using default value: false")
			return false
		}
		debugSettings = val

	} else if configSchemaVersion == "v2" {
		prometheusSettings, ok := metricsConfigBySection["prometheus-collector-settings"]
		if !ok {
			log.Println("The 'prometheus-collector-settings' section is not present in the parsed data. Using default value: false")
			return false
		}

		val, ok := prometheusSettings["debug-mode"]
		if !ok {
			log.Println("The 'debug-mode' section is not present in the parsed data. Using default value: false")
			return false
		}
		debugSettings = val

	} else {
		log.Printf("Unsupported config schema version: %s. Using default value: false\n", configSchemaVersion)
		return false
	}

	enabled := strings.ToLower(debugSettings) == "true"
	log.Printf("Using configmap setting for debug mode: %v\n", enabled)
	return enabled
}
