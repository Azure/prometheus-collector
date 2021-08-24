package main

import (
	"flag"
	"fmt"

	"github.com/go-kit/log"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	promconfig "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/service/parserprovider"
)

func main() {
	configFilePtr := flag.String("config-file", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr
	fmt.Printf("Config file provided : %s \n", filePath)

	cp, err := parserprovider.Get()
	if err != nil {
		return fmt.Errorf("cannot load configuration's parser: %w", err)
	}
	configContents, err := promconfig.LoadFile(filePath, false, log.NewNopLogger())
	if err != nil {
		fmt.Printf("Error: %v", err)
	}

	receiverConfig := &privatepromreceiver.Config{
		PrometheusConfig: configContents,
	}
	// fmt.Printf("ReceiverConfig: %+v\n", receiverConfig)

	_ = receiverConfig.Unmarshal(cp)
	cfgErr := receiverConfig.Validate()
	if cfgErr != nil {
		fmt.Printf("Error: %v", cfgErr)
	}
}
