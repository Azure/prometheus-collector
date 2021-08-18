package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/log"
	"github.com/prometheus/prometheus/config"
)

func main() {
	receiver := components()
	configFilePtr := flag.String("config-file", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr
	fmt.Printf("Config file provided : %s", filePath)
	// info := component.BuildInfo{
	// 	Command:     "custom-receiver-validator",
	// 	Description: "Custom Receiver validator",
	// 	Version:     "1.0.0",
	// }
	// fmt.Printf("Receiver: %v", receiver)
	fmt.Printf("Receiver: %+v\n", receiver)
	cfg := receiver.CreateDefaultConfig()
	fmt.Printf("Config: %+v\n", cfg)
	configContents, err := config.LoadFile(filePath, false, log.NewNopLogger())
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	fmt.Printf("Config contents: %v", configContents)
	cfg{PrometheusConfig: configContents}

	cfgErr := cfg.Validate()
	if cfgErr != nil {
		fmt.Printf("Error: %v", cfgErr)
	}
}
