package configmapsettings

import (
	"log"
	"os"

	"github.com/prometheus-collector/shared"
	yaml "gopkg.in/yaml.v2"
)

func SetGlobalSettingsInCollectorConfig() {
	azmonSetGlobalSettings := os.Getenv("AZMON_SET_GLOBAL_SETTINGS")

	if azmonSetGlobalSettings == "true" {
		mergedCollectorConfigPath := "/opt/microsoft/otelcollector/collector-config.yml"
		mergedCollectorConfigFileContents, err := os.ReadFile(mergedCollectorConfigPath)
		if err != nil {
			log.Printf("Unable to read file contents from: %s - %v\n", mergedCollectorConfigPath, err)
			return
		}
		var promScrapeConfig map[string]interface{}
		var otelConfig shared.OtelConfig
		err = yaml.Unmarshal([]byte(mergedCollectorConfigFileContents), &otelConfig)
		if err != nil {
			log.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", mergedCollectorConfigFileContents, err)
			return
		}

		promScrapeConfig = otelConfig.Receivers.Prometheus.Config
		globalSettingsFromMergedOtelConfig := promScrapeConfig["global"]

		if globalSettingsFromMergedOtelConfig != nil {
			log.Println("Found global settings in merged otel config, triyng to replace replicaset collector config")
			collectorConfigReplicasetPath := "/opt/microsoft/otelcollector/collector-config-replicaset.yml"
			replicasetCollectorConfigFileContents, err := os.ReadFile(collectorConfigReplicasetPath)
			if err != nil {
				log.Printf("Unable to read file contents from: %s - %v\n", replicasetCollectorConfigFileContents, err)
				return
			}
			var otelConfigReplicaset shared.OtelConfig
			err = yaml.Unmarshal([]byte(replicasetCollectorConfigFileContents), &otelConfigReplicaset)
			if err != nil {
				log.Printf("Unable to unmarshal merged otel configuration from: %s - %v\n", replicasetCollectorConfigFileContents, err)
				return
			}
			otelConfigReplicaset.Receivers.Prometheus.Config = map[string]interface{}{"global": ""}
			otelConfigReplicaset.Receivers.Prometheus.Config["global"] = globalSettingsFromMergedOtelConfig
			otelReplacedConfigYaml, _ := yaml.Marshal(otelConfigReplicaset)
			if err := os.WriteFile(collectorConfigReplicasetPath, otelReplacedConfigYaml, 0644); err != nil {
				log.Printf("Unable to write to: %s - %v\n", collectorConfigReplicasetPath, err)
				return
			}

			log.Println("Updated file with global settings", collectorConfigReplicasetPath)
			return
		}
		log.Println("Global settings are empty in custom config map, making no replacement")
	}
}
