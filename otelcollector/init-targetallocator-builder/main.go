package main

import (
	"fmt"
	"log"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type Config struct {
	LabelSelector      map[string]string `yaml:"label_selector,omitempty"`
	AllocationStrategy string            `yaml:"allocation_strategy,omitempty"`
}

var RESET = "\033[0m"
var RED = "\033[31m"

var taConfigFilePath = "/ta-configuration/targetallocator.yaml"

func logFatalError(message string) {
	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func main() {
	// promScrapeConfig := otelConfig.Receivers.Prometheus.Config
	targetAllocatorConfig := Config{
		AllocationStrategy: "consistent-hashing",
		LabelSelector: map[string]string{
			"rsName":                         "ama-metrics",
			"kubernetes.azure.com/managedby": "aks",
		},
	}

	targetAllocatorConfigYaml, _ := yaml.Marshal(targetAllocatorConfig)
	if err := os.WriteFile(taConfigFilePath, targetAllocatorConfigYaml, 0644); err != nil {
		logFatalError(fmt.Sprintf("config-reader::Unable to write to: %s - %v\n", taConfigFilePath, err))
		os.Exit(1)
	}

	log.Println("Updated file - targetallocator.yaml to initialize TargetAllocator with empty config")
	os.Exit(0)

	// // Waiting until config file is created by the sidecar
	// if _, err := os.Stat("/conf/targetallocator.yaml"); err == nil {
	// 	fmt.Println("Config file created at /conf/targetallocator.yaml")
	// 	os.Exit(0)

	// } else {
	// 	time.Sleep(1 * time.Second)
	// }
}
