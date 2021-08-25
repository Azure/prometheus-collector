package main

import (
	"flag"
	"fmt"
	"log"

	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
)

func main() {
	// factories, err := components()
	// if err != nil {
	// 	log.Fatalf("failed to build components: %v\n", err)
	// }
	// flags := new(flag.FlagSet)
	// parserProvider.Flags(flags)

	configFilePtr := flag.String("config", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr

	if filePath != "" {
		configFlag := fmt.Sprintf("--config=%s", filePath)
		fmt.Printf("config file provided - %s\n", configFlag)

		flags := new(flag.FlagSet)
		parserProvider.Flags(flags)
		err := flags.Parse([]string{
			// "--config=testdata/otelcol-config.yaml",
			configFlag,
		})
		if err != nil {
			fmt.Printf("Error parsing flags - %v\n", err)
		}

		factories, err := components()
		if err != nil {
			log.Fatalf("failed to build components: %v\n", err)
		}

		colParserProvider := parserProvider.Default()

		cp, err := colParserProvider.Get()
		if err != nil {
			fmt.Errorf("cannot load configuration's parser: %w\n", err)
		}
		fmt.Printf("Loading configuration...\n")

		cfg, err := configloader.Load(cp, factories)
		if err != nil {
			log.Fatalf("Cannot load configuration: %v", err)
		}

		err = cfg.Validate()
		if err != nil {
			fmt.Printf("Invalid configuration: %w\n", err)
		}
	} else {
		log.Fatalf("Please provide a config file using the --config flag to validate\n")
	}

	// var cp *configparser.Parser

	// var cfg *config.Config
	// cfg, err = configloader.Load(cp, factories)

	// fmt.Printf("config - %v", cfg)

	// colParserProvider := parserProvider.Default("")

	// cp, err := colParserProvider.Get()
	// if err != nil {
	// 	fmt.Errorf("cannot load configuration's parser: %w", err)
	// }

	// fmt.Printf("def parser provider: %v", cp)

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
