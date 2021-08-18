package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/log"
	promconfig "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/config"
)

func createCustomConfig(cfg promconfig.Config) config.Receiver {
	// func createDefaultConfig(params component.ReceiverCreateParams) config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		PrometheusConfig: cfg,
		// logger : params.Logger,
	}
}

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
	defCfg := receiver.CreateDefaultConfig()
	fmt.Printf("DefConfig: %+v\n", defCfg)

	customConfig := createCustomConfig(configContents)
	fmt.Printf("CustomConfig: %+v\n", customConfig)
	configContents, err := promconfig.LoadFile(filePath, false, log.NewNopLogger())
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	fmt.Printf("Config contents: %v", configContents)
	//cfg.PrometheusConfig = configContents

	//cfgErr := cfg.Validate()
	//if cfgErr != nil {
	//	fmt.Printf("Error: %v", cfgErr)
	//}
}
