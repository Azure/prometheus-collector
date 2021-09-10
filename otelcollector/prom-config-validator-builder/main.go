package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
	yaml "gopkg.in/yaml.v2"
)

// type otelConfigStruct struct {
// 	Exporters struct {
// 		File struct {
// 			Path string `yaml:"path"`
// 		} `yaml:"file"`
// 		Otlp struct {
// 			Endpoint       string `yaml:"endpoint"`
// 			Insecure       bool   `yaml:"insecure"`
// 			Compression    string `yaml:"compression"`
// 			RetryOnFailure struct {
// 				Enabled bool `yaml:"enabled"`
// 			} `yaml:"retry_on_failure"`
// 			Timeout string `yaml:"timeout"`
// 		} `yaml:"otlp"`
// 	} `yaml:"exporters"`
// 	Processors struct {
// 		Batch struct {
// 			SendBatchSize    int    `yaml:"send_batch_size"`
// 			Timeout          string `yaml:"timeout"`
// 			SendBatchMaxSize int    `yaml:"send_batch_max_size"`
// 		} `yaml:"batch"`
// 		Resource struct {
// 			Attributes []struct {
// 				Key    string `yaml:"key"`
// 				Value  string `yaml:"value"`
// 				Action string `yaml:"action"`
// 			} `yaml:"attributes"`
// 		} `yaml:"resource"`
// 	} `yaml:"processors"`
// 	Receivers struct {
// 		Prometheus struct {
// 			Config interface{} `yaml:"config"`
// 		} `yaml:"prometheus"`
// 	} `yaml:"receivers"`
// 	Service struct {
// 		Pipelines struct {
// 			Metrics struct {
// 				Receivers  []string `yaml:"receivers"`
// 				Exporters  []string `yaml:"exporters"`
// 				Processors []string `yaml:"processors"`
// 			} `yaml:"metrics"`
// 		} `yaml:"pipelines"`
// 	} `yaml:"service"`
// }
//}

// type otelConfigStruct struct {
// 	Receivers struct {
// 		Prometheus struct {
// 			Config interface{} `yaml:"config"`
// 		} `yaml:"prometheus"`
// 	} `yaml:"receivers"`
// 	Exporters  interface{} `yaml:"exporters"`
// 	Processors interface{} `yaml:"processors"`
// 	Service    interface{} `yaml:"service"`
// }

func generateOtelConfig(promFilePath string) error {
	//var otelConfig otelConfigStruct
	var otelConfig collectorConfig
	var otelTemplatePath = "collector-config-template.yml"

	otelConfigFileContents, err := os.ReadFile(otelTemplatePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(otelConfigFileContents), &otelConfig)
	if err != nil {
		return err
	}

	promConfig := make(map[interface{}]interface{})

	promConfigFileContents, err := os.ReadFile(promFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(promConfigFileContents), &promConfig)
	if err != nil {
		return err
	}

	otelConfig.Receivers.Prometheus.Config = promConfig

	// data, _ := yaml.Marshal(&m)
	// err = ioutil.WriteFile(outputFilePath, data, 0)

	// if err != nil {
	// 	return err
	// }
	// if err := yaml.Unmarshal(bs, &master); err != nil {
	// 	return err
	// }

	// var override map[string]interface{}
	// bs, err = ioutil.ReadFile(promFilePath)
	// if err != nil {
	// 	return err
	// }
	// if err := yaml.Unmarshal(bs, &override); err != nil {
	// 	return err
	// }

	// // for k, v := range override {
	// // 	master[k] = v
	// // }

	// master["receivers"]["prometheus"]["config"] = override

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
