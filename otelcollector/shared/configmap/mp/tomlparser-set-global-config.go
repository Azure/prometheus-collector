package configmapsettings

import (
	"fmt"
	"log"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          map[string]interface{} `yaml:"config"`
			TargetAllocator interface{}            `yaml:"target_allocator"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}

func SetGlobalSettingsInCollectorConfig() {
	azmonSetGlobalSettings := os.Getenv("AZMON_SET_GLOBAL_SETTINGS")

	if azmonSetGlobalSettings == "true" {
		mergedCollectorConfigPath := "/opt/microsoft/otelcollector/collector-config.yml"
		mergedCollectorConfigFileContents, err := os.ReadFile(mergedCollectorConfigPath)
		if err != nil {
			fmt.Printf("Unable to read file contents from: %s - %v\n", mergedCollectorConfigPath, err)
			return
		}
		var promScrapeConfig map[string]interface{}
		var otelConfig OtelConfig
		err = yaml.Unmarshal([]byte(mergedCollectorConfigFileContents), &otelConfig)
		if err != nil {
			fmt.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", mergedCollectorConfigFileContents, err)
			return
		}

		promScrapeConfig = otelConfig.Receivers.Prometheus.Config
		globalSettingsFromMergedOtelConfig := promScrapeConfig["global"]

		if globalSettingsFromMergedOtelConfig != nil {
			fmt.Println("Found global settings in merged otel config, triyng to replace replicaset collector config")
			collectorConfigReplicasetPath := "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
			replicasetCollectorConfigFileContents, err := os.ReadFile(collectorConfigReplicasetPath)
			if err != nil {
				fmt.Printf("Unable to read file contents from: %s - %v\n", replicasetCollectorConfigFileContents, err)
				return
			}
			var otelConfigReplicaset OtelConfig
			err = yaml.Unmarshal([]byte(replicasetCollectorConfigFileContents), &otelConfigReplicaset)
			if err != nil {
				fmt.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", replicasetCollectorConfigFileContents, err)
				return
			}
			otelConfigReplicaset.Receivers.Prometheus.Config = map[string]interface{}{"global": ""}
			otelConfigReplicaset.Receivers.Prometheus.Config["global"] = globalSettingsFromMergedOtelConfig
			otelReplacedConfigYaml, _ := yaml.Marshal(otelConfigReplicaset)
			if err := os.WriteFile(collectorConfigReplicasetPath, otelReplacedConfigYaml, 0644); err != nil {
				fmt.Printf("Unable to write to: %s - %v\n", collectorConfigReplicasetPath, err)
				return
			}

			log.Println("Updated file with global settings", collectorConfigReplicasetPath)
			return
		}
		fmt.Println("Global settings are empty in custom config map, making no replacement")
	}
}
