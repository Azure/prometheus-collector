package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/log"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	promconfig "github.com/prometheus/prometheus/config"
)

func main() {
	// receiver := components()
	// receiver := privatepromreceiver.NewFactory()
	configFilePtr := flag.String("config-file", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr
	fmt.Printf("Config file provided : %s", filePath)

	// fmt.Printf("Receiver: %+v\n", receiver)
	// defCfg := receiver.CreateDefaultConfig()
	// fmt.Printf("DefConfig: %+v\n", defCfg)

	configContents, err := promconfig.LoadFile(filePath, false, log.NewNopLogger())
	if err != nil {
		fmt.Printf("Error: %v", err)
	}
	// fmt.Printf("Config contents: %v", configContents)

	receiverConfig := &privatepromreceiver.Config{
		PrometheusConfig: configContents,
	}
	fmt.Printf("ReceiverConfig: %+v\n", receiverConfig)

	cfgErr := receiverConfig.Validate()
	if cfgErr != nil {
		fmt.Printf("Error: %v", cfgErr)
	}
}
