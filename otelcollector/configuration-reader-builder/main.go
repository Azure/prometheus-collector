package main

import (
	"flag"
	"fmt"
	"log"

	"os"

	yaml "gopkg.in/yaml.v2"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator/config"
)

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          promconfig.Config      `yaml:"config"`
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
	var promScrapeConfig promconfig.Config
	var otelConfig OtelConfig
	err = yaml.Unmarshal([]byte(defaultsMergedConfigFileContents), &otelConfig)
	if err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to unmarshal merged otel configuration from: %s - %v\n", configFilePath, err))
		os.Exit(1)
	}

	promScrapeConfig = promconfig.Config(otelConfig.Receivers.Prometheus.Config)
	targetAllocatorConfig := config.Config{
		LabelSelector: map[string]string{
			"rsName":                         "ama-metrics",
			"kubernetes.azure.com/managedby": "aks",
		},
		Config: &promScrapeConfig,
		//PrometheusCR: config.PrometheusCRConfig {
		//	ScrapeInterval: model.Duration(time.Second * 60),
		//},
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
