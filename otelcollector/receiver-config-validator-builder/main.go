package main

import (
	"fmt"
		"go.opentelemetry.io/collector/service/parserprovider"
)

func main() {

	def := parserProvider.Default()

	fmt.Printf("def parser provider: %v", def)

	// factories, err := components()
	// if err != nil {
	// 	log.Fatalf("failed to build components: %v", err)
	// }
	// info := component.BuildInfo{
	// 	Command:     "custom-collector-distro",
	// 	Description: "Custom OpenTelemetry Collector distributionr",
	// 	Version:     "1.0.0",
	// }

	// col, err := service.New(service.Parameters{BuildInfo: info, Factories: factories})
	// if err != nil {
	// 	log.Fatal("failed to construct the collector server: %w", err)
	// }

	// fmt.Printf("Loading configuration...")

	// cp, err := col.parserProvider.Get()
	// if err != nil {
	// 	return fmt.Errorf("cannot load configuration's parser: %w", err)
	// }

	// fmt.Printf("parser provider - %v", cp)

	// configFilePtr := flag.String("config-file", "", "Config file to validate")
	// flag.Parse()
	// filePath := *configFilePtr
	// fmt.Printf("Config file provided : %s \n", filePath)

	// configContents, err := promconfig.LoadFile(filePath, false, log.NewNopLogger())
	// if err != nil {
	// 	fmt.Printf("Error: %v", err)
	// }

	// receiverConfig := &privatepromreceiver.Config{
	// 	PrometheusConfig: configContents,
	// }

	// cfgErr := receiverConfig.Validate()
	// if cfgErr != nil {
	// 	fmt.Printf("Error: %v", cfgErr)
	// }
}
