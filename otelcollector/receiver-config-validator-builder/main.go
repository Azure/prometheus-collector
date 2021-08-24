package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/log"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	promconfig "github.com/prometheus/prometheus/config"
)

func main() {
	configFilePtr := flag.String("config-file", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr
	fmt.Printf("Config file provided : %s \n", filePath)

	configContents, err := promconfig.LoadFile(filePath, false, log.NewNopLogger())
	if err != nil {
		fmt.Printf("Error: %v", err)
	}

	receiverConfig := &privatepromreceiver.Config{
		PrometheusConfig: configContents,
	}
	// fmt.Printf("ReceiverConfig: %+v\n", receiverConfig)

	cfgErr := receiverConfig.Validate()
	if cfgErr != nil {
		fmt.Printf("Error: %v", cfgErr)
	}
}
