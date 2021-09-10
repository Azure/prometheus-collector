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
	outFile := flag.String("output", "", "Output file path for writing collector config")
	flag.Parse()
	filePath := *configFilePtr
	outputFilePath := *outFile
	fmt.Printf("outfile path - %v", outputFilePath)
	if filePath != "" {
		configFlag := fmt.Sprintf("--config=%s", filePath)
		fmt.Printf("prom-config-validator::Config file provided - %s\n", configFlag)

		dat, err := os.ReadFile(filePath)
		fmt.Printf("%v", dat)

		flags := new(flag.FlagSet)
		parserProvider.Flags(flags)
		err := flags.Parse([]string{
			configFlag,
		})
		if err != nil {
			fmt.Printf("prom-config-validator::Error parsing flags - %v\n", err)
			os.Exit(1)
		}

		factories, err := components()
		if err != nil {
			log.Fatalf("prom-config-validator::Failed to build components: %v\n", err)
			os.Exit(1)
		}

		colParserProvider := parserProvider.Default()

		cp, err := colParserProvider.Get()
		if err != nil {
			fmt.Errorf("prom-config-validator::Cannot load configuration's parser: %w\n", err)
			os.Exit(1)
		}
		fmt.Printf("prom-config-validator::Loading configuration...\n")

		cfg, err := configloader.Load(cp, factories)
		if err != nil {
			log.Fatalf("prom-config-validator::Cannot load configuration: %v", err)
			os.Exit(1)
		}

		err = cfg.Validate()
		if err != nil {
			fmt.Printf("prom-config-validator::Invalid configuration: %w\n", err)
			os.Exit(1)
		}
	} else {
		log.Fatalf("prom-config-validator::Please provide a config file using the --config flag to validate\n")
		os.Exit(1)
	}
	fmt.Printf("prom-config-validator::Successfully loaded and validated custom prometheus config\n")
	os.Exit(0)
}
