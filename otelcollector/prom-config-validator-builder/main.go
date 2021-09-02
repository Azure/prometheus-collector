package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
)

func main() {
	configFilePtr := flag.String("config", "", "Config file to validate")
	flag.Parse()
	filePath := *configFilePtr

	if filePath != "" {
		configFlag := fmt.Sprintf("--config=%s", filePath)
		fmt.Printf("config file provided - %s\n", configFlag)

		flags := new(flag.FlagSet)
		parserProvider.Flags(flags)
		err := flags.Parse([]string{
			configFlag,
		})
		if err != nil {
			fmt.Printf("Error parsing flags - %v\n", err)
			os.Exit(1)
		}

		factories, err := components()
		if err != nil {
			log.Fatalf("failed to build components: %v\n", err)
			os.Exit(1)
		}

		colParserProvider := parserProvider.Default()

		cp, err := colParserProvider.Get()
		if err != nil {
			fmt.Errorf("cannot load configuration's parser: %w\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loading configuration...\n")

		cfg, err := configloader.Load(cp, factories)
		if err != nil {
			log.Fatalf("Cannot load configuration: %v", err)
			os.Exit(1)
		}

		err = cfg.Validate()
		if err != nil {
			fmt.Printf("Invalid configuration: %w\n", err)
			os.Exit(1)
		}
	} else {
		log.Fatalf("Please provide a config file using the --config flag to validate\n")
		os.Exit(1)
	}
	os.Exit(0)
}
