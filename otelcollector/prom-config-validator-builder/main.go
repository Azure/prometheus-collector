package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	gokitLogger "github.com/go-kit/log"
	prometheusConfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/relabel"
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
	fmt.Printf("in generate\n")

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
	// Need this here even though it is present in the validate method because in the absence of this, only the $ for regex and replacement fields
	// in scrape config are replaced and the load method of the collector fails due to single $. This is because validate does this check after the load
	// is done
	unsupportedFeatures := make([]string, 0, 4)
	if len(promConfig.RemoteWriteConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_write")
	}
	if len(promConfig.RemoteReadConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "remote_read")
	}
	if len(promConfig.RuleFiles) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "rule_files")
	}
	if len(promConfig.AlertingConfig.AlertRelabelConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.relabel_configs")
	}
	if len(promConfig.AlertingConfig.AlertmanagerConfigs) != 0 {
		unsupportedFeatures = append(unsupportedFeatures, "alert_config.alertmanagers")
	}
	if len(unsupportedFeatures) != 0 {
		// Sort the values for deterministic error messages.
		//sort.Strings(unsupportedFeatures)
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	//singleDollarRegex, _ := regexp.Compile(`\$`)
	singleDollarRegex, err := regexp.Compile(`/(?<!\$)\$(?!\$)/`)
	if err != nil {
		return err
	}
	//fmt.Printf("here\n")
	for _, scfg := range promConfig.ScrapeConfigs {
		for _, relabelConfig := range scfg.RelabelConfigs {
			//fmt.Printf("here\n")
			regexString := relabelConfig.Regex.String()
			fmt.Printf("regex- %v\n", regexString)
			modifiedRegexString := singleDollarRegex.ReplaceAllLiteralString(regexString, "$$")
			modifiedRegex, err := relabel.NewRegexp(modifiedRegexString)
			if err != nil {
				return err
			}
			relabelConfig.Regex = modifiedRegex

			replacement := relabelConfig.Replacement
			fmt.Printf("replacement: %s\n", replacement)
			modifiedReplacementString := singleDollarRegex.ReplaceAllLiteralString(replacement, "$$")
			// modifiedReplacement, err := relabel.NewRegexp(modifiedReplacementString)
			// if err != nil {
			// 	return err
			// }
			relabelConfig.Replacement = modifiedReplacementString
			//relabelConfig.Action = "rashmi"
			//fmt.Printf("%v\n", relabelConfig)
		}
		for _, metricRelabelConfig := range scfg.MetricRelabelConfigs {
			regexString := metricRelabelConfig.Regex.String()
			fmt.Println(singleDollarRegex.ReplaceAllLiteralString(regexString, "$$"))

			replacement := metricRelabelConfig.Replacement
			fmt.Println(singleDollarRegex.ReplaceAllLiteralString(replacement, "$$"))
		}

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
		if err != nil {
			log.Fatalf("Generating otel config failed: %v\n", err)
			os.Exit(1)
		}
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
