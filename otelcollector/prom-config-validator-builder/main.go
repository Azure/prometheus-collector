package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	gokitLogger "github.com/go-kit/log"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
	yaml "gopkg.in/yaml.v2"
)

type otelConfigStruct struct {
	Receivers struct {
		Prometheus struct {
			Config interface{} `yaml:"config"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Service    interface{} `yaml:"service"`
}

func generateOtelConfig(promFilePath string) error {
	var otelConfig otelConfigStruct
	var otelTemplatePath = "collector-config-template.yml"

	otelConfigFileContents, err := os.ReadFile(otelTemplatePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(otelConfigFileContents), &otelConfig)
	if err != nil {
		return err
	}

	//var promConfig *prometheusConfig.Config
	promConfig, err := prometheusConfig.LoadFile(promFilePath, false, gokitLogger.NewNopLogger())
	if err != nil {
		return err
	}

	// promConfigFileContents, err := os.ReadFile(promFilePath)
	// if err != nil {
	// 	return err
	// }
	// err = yaml.Unmarshal([]byte(promConfigFileContents), &promConfig)
	// if err != nil {
	// 	return err
	// }

	//fmt.Printf("Replacing single $ in regexes to $$ to prevent environment variable replacement\n")

	//scfg := promConfig.sc

	otelConfig.Receivers.Prometheus.Config = promConfig

	mergedConfig, err := yaml.Marshal(otelConfig)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile("merged.yaml", mergedConfig, 0644); err != nil {
		return err
	}
	return nil
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to validate")
	outFile := flag.String("output", "", "Output file path for writing collector config")
	flag.Parse()
	filePath := *configFilePtr
	outputFilePath := *outFile
	fmt.Printf("outfile path - %v", outputFilePath)
	if filePath != "" {
		// configFlag := fmt.Sprintf("--config=%s", filePath)
		// fmt.Printf("prom-config-validator::Config file provided - %s\n", configFlag)
		fmt.Printf("prom-config-validator::Config file provided - %s\n", filePath)

		// dat, err := os.ReadFile(filePath)
		// m := make(map[interface{}]interface{})
		// _ = yaml.Unmarshal([]byte(dat), &m)
		// data, _ := yaml.Marshal(&m)
		// err = ioutil.WriteFile(outputFilePath, data, 0)

		err := generateOtelConfig(filePath)

		flags := new(flag.FlagSet)
		parserProvider.Flags(flags)
		configFlag := fmt.Sprintf("--config=%s", "merged.yaml")

		err = flags.Parse([]string{
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
