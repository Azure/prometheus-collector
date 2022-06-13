package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"context"

	"strings"

	//"go.opentelemetry.io/collector/service/internal/configunmarshaler"
	"go.opentelemetry.io/collector/service"
	//"go.opentelemetry.io/collector/config/mapprovider/filemapprovider"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	yaml "gopkg.in/yaml.v2"
)

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Receivers  struct {
		Prometheus struct {
			Config interface{} `yaml:"config"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service interface{} `yaml:"service"`
}

func generateOtelConfig(promFilePath string, outputFilePath string, otelConfigTemplatePath string) error {
	var otelConfig OtelConfig

	otelConfigFileContents, err := ioutil.ReadFile(otelConfigTemplatePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(otelConfigFileContents), &otelConfig)
	if err != nil {
		return err
	}

	promConfigFileContents, err := ioutil.ReadFile(promFilePath)
	if err != nil {
		return err
	}

	var prometheusConfig map[string]interface{}
	err = yaml.Unmarshal([]byte(promConfigFileContents), &prometheusConfig)
	if err != nil {
		return err
	}

	scrapeConfigs := prometheusConfig["scrape_configs"]
	if scrapeConfigs != nil {
		var sc = scrapeConfigs.([]interface{})
		for _, scrapeConfig := range sc {
			scrapeConfig := scrapeConfig.(map[interface{}]interface{})
			if scrapeConfig["relabel_configs"] != nil {
				relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})
				for _, relabelConfig := range relabelConfigs {
					relabelConfig := relabelConfig.(map[interface{}]interface{})
					//replace $ with $$ for regex field
					if relabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := relabelConfig["regex"].(string); isString {
							regexString := relabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
							relabelConfig["regex"] = modifiedRegexString
						}
					}
					//replace $ with $$ for replacement field
					if relabelConfig["replacement"] != nil {
						replacement := relabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
						relabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}

			if scrapeConfig["metric_relabel_configs"] != nil {
				metricRelabelConfigs := scrapeConfig["metric_relabel_configs"].([]interface{})
				for _, metricRelabelConfig := range metricRelabelConfigs {
					metricRelabelConfig := metricRelabelConfig.(map[interface{}]interface{})
					//replace $ with $$ for regex field
					if metricRelabelConfig["regex"] != nil {
						// Adding this check here since regex can be boolean and the conversion will fail
						if _, isString := metricRelabelConfig["regex"].(string); isString {
							regexString := metricRelabelConfig["regex"].(string)
							modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
							metricRelabelConfig["regex"] = modifiedRegexString
						}
					}

					//replace $ with $$ for replacement field
					if metricRelabelConfig["replacement"] != nil {
						replacement := metricRelabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
						metricRelabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}
		}
	}

	// Need this here even though it is present in the receiver's config validate method since we only do the $ manipulation for regex and replacement fields
	// in scrape configs sections and the load method which is called before the validate method fails to unmarshal due to single $.
	// Either approach will fail but the receiver's config load wont return the right error message
	unsupportedFeatures := make([]string, 0, 4)

	if prometheusConfig["remote_write"] != nil {
		unsupportedFeatures = append(unsupportedFeatures, "remote_write")
	}
	if prometheusConfig["remote_read"] != nil {
		unsupportedFeatures = append(unsupportedFeatures, "remote_read")
	}
	if prometheusConfig["rule_files"] != nil {
		unsupportedFeatures = append(unsupportedFeatures, "rule_files")
	}
	if prometheusConfig["alerting"] != nil {
		unsupportedFeatures = append(unsupportedFeatures, "alerting")
	}
	if len(unsupportedFeatures) != 0 {
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	otelConfig.Receivers.Prometheus.Config = prometheusConfig

	mergedConfig, err := yaml.Marshal(otelConfig)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(outputFilePath, mergedConfig, 0644); err != nil {
		return err
	}
	fmt.Printf("prom-config-validator::Successfully generated otel config\n")
	return nil
}

type stringArrayValue struct {
	values []string
}

func (s *stringArrayValue) Set(val string) error {
	s.values = append(s.values, val)
	return nil
}

func (s *stringArrayValue) String() string {
	return "[" + strings.Join(s.values, ", ") + "]"
}

func main() {
	configFilePtr := flag.String("config", "", "Config file to validate")
	outFilePtr := flag.String("output", "", "Output file path for writing collector config")
	otelTemplatePathPtr := flag.String("otelTemplate", "", "OTel Collector config template file path")
	flag.Parse()
	promFilePath := *configFilePtr
	otelConfigTemplatePath := *otelTemplatePathPtr
	if otelConfigTemplatePath == "" {
		log.Fatalf("prom-config-validator::Please provide otel config template path\n")
		os.Exit(1)
	}
	if promFilePath != "" {
		fmt.Printf("prom-config-validator::Config file provided - %s\n", promFilePath)

		outputFilePath := *outFilePtr
		if outputFilePath == "" {
			outputFilePath = "merged-otel-config.yaml"
		}

		err := generateOtelConfig(promFilePath, outputFilePath, otelConfigTemplatePath)
		if err != nil {
			log.Fatalf("Generating otel config failed: %v\n", err)
			os.Exit(1)
		}

		flags := new(flag.FlagSet)
		//parserProvider.Flags(flags)
		configFlagEx := new(stringArrayValue)
		flags.Var(configFlagEx, "config", "Locations to the config file(s), note that only a"+
		" single location can be set per flag entry e.g. `-config=file:/path/to/first --config=file:path/to/second`.")
		configFlag := fmt.Sprintf("--config=%s", outputFilePath)

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

		cp, err := service.NewConfigProvider(
			service.ConfigProviderSettings{
				Locations:     []string{fmt.Sprintf("file:%s", outputFilePath)},
				MapProviders:  map[string]confmap.Provider{"file": fileprovider.New()},
				MapConverters: []confmap.Converter{expandconverter.New()},
			},
		)
		if err != nil {
			log.Fatalf("prom-config-validator::Cannot load configuration's parser: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("prom-config-validator::Loading configuration...\n")
		cfg, err := cp.Get(context.Background(), factories)
		if err != nil {
			log.Fatalf("prom-config-validator::Cannot load configuration: %v", err)
			os.Exit(1)
		}

		err = cfg.Validate()
		if err != nil {
			log.Fatalf("prom-config-validator::Invalid configuration: %v\n", err)
			os.Exit(1)
		}
	} else {
		log.Fatalf("prom-config-validator::Please provide a config file using the --config flag to validate\n")
		os.Exit(1)
	}
	fmt.Printf("prom-config-validator::Successfully loaded and validated custom prometheus config\n")
	os.Exit(0)
}
