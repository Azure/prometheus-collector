package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"os"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	LabelSelector      map[string]string      `yaml:"label_selector,omitempty"`
	Config             map[string]interface{} `yaml:"config"`
	AllocationStrategy string                 `yaml:"allocation_strategy,omitempty"`
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
	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"rsName":                         "ama-metrics",
			"kubernetes.azure.com/managedby": "aks",
		},
		Config: promScrapeConfig,
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml for the TargetAllocator to pick up new config changes")
}

func hasConfigChanged(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			fmt.Println("Error getting file info:", err)
			os.Exit(1)
		}

		return fileInfo.Size() > 0
	}
	return false
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK
	message := "\ntargetallocator is running."

	// if hasConfigChanged("/opt/inotifyoutput-ta-config.txt") {
	// 	status = http.StatusServiceUnavailable
	// 	message += "\ninotifyoutput.txt has been updated - targetallocator-config changed"
	// }

	w.WriteHeader(status)
	fmt.Fprintln(w, message)
	// if status != http.StatusOK {
	// 	fmt.Printf(message)
	// }
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to read")
	flag.Parse()
	otelConfigFilePath := *configFilePtr
	updateTAConfigFile(otelConfigFilePath)

	http.HandleFunc("/health", healthHandler)
	http.ListenAndServe(":8081", nil)
}
