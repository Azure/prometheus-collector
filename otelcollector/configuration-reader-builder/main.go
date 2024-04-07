package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"os"

	yaml "gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	CollectorSelector  *metav1.LabelSelector  `yaml:"collector_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
	PrometheusCR       map[string]interface{} `yaml:"prometheus_cr,omitempty"`
}

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

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func updateTAConfigFile(configFilePath string) {
	defaultsMergedConfigFileContents, err := os.ReadFile(configFilePath)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to read file contents from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}
	var promScrapeConfig map[string]interface{}
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to unmarshal merged otel configuration from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}

	promScrapeConfig = otelConfig.Receivers.Prometheus.Config
	// Removing $$ added for regex and replacement in relabel_config and metric_relabel_config added by promconfigvalidator.
	// The $$ are required by the validator's otel get method, but the TA doesnt do env substitution and hence needs to be removed, else TA crashes.
	scrapeConfigs := promScrapeConfig["scrape_configs"]
	if scrapeConfigs != nil {
		var sc = scrapeConfigs.([]interface{})
		for _, scrapeConfig := range sc {
			scrapeConfig := scrapeConfig.(map[interface{}]interface{})
			if scrapeConfig["relabel_configs"] != nil {
				relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})
				for _, relabelConfig := range relabelConfigs {
					relabelConfig := relabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if relabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := relabelConfig["regex"].(string); isString {
							regexString := relabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							relabelConfig["regex"] = modifiedRegexString
						}
					}
					//replace $$ with $ for replacement field
					if relabelConfig["replacement"] != nil {
						replacement := relabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						relabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}

			if scrapeConfig["metric_relabel_configs"] != nil {
				metricRelabelConfigs := scrapeConfig["metric_relabel_configs"].([]interface{})
				for _, metricRelabelConfig := range metricRelabelConfigs {
					metricRelabelConfig := metricRelabelConfig.(map[interface{}]interface{})
					//replace $$ with $ for regex field
					if metricRelabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := metricRelabelConfig["regex"].(string); isString {
							regexString := metricRelabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							metricRelabelConfig["regex"] = modifiedRegexString
						}
					}

					//replace $$ with $ for replacement field
					if metricRelabelConfig["replacement"] != nil {
						replacement := metricRelabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						metricRelabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}
		}
	}

	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		CollectorSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"rsName":                         "ama-metrics",
				"kubernetes.azure.com/managedby": "aks",
			},
		},
		// CollectorSelector: map[string]string{
		// 	"rsName":                         "ama-metrics",
		// 	"kubernetes.azure.com/managedby": "aks",
		// },
		Config: promScrapeConfig,
		PrometheusCR: map[string]interface{}{
			"ServiceMonitorSelector": &metav1.LabelSelector{},
			"PodMonitorSelector":     &metav1.LabelSelector{},
		},
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to read")
	flag.Parse()
	otelConfigFilePath := *configFilePtr
	updateTAConfigFile(otelConfigFilePath)
}
