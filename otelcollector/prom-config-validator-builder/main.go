package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"strings"

	//"github.com/spf13/viper"
	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
	yaml "gopkg.in/yaml.v2"
)

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Receivers  struct {
		Prometheus struct {
			Config PrometheusConfig `yaml:"config"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service interface{} `yaml:"service"`
}

type RelabelConfig struct {
	SourceLabels interface{} `yaml:"source_labels,flow,omitempty"`
	Separator    interface{} `yaml:"separator,omitempty"`
	Regex        string      `yaml:"regex,omitempty"`
	Modulus      interface{} `yaml:"modulus,omitempty"`
	TargetLabel  interface{} `yaml:"target_label,omitempty"`
	Replacement  string      `yaml:"replacement,omitempty"`
	Action       interface{} `yaml:"action,omitempty"`
}

type ScrapeConfig struct {
	JobName                 interface{}      `yaml:"job_name"`
	HonorLabels             interface{}      `yaml:"honor_labels,omitempty"`
	HonorTimestamps         interface{}      `yaml:"honor_timestamps"`
	Params                  interface{}      `yaml:"params,omitempty"`
	ScrapeInterval          interface{}      `yaml:"scrape_interval,omitempty"`
	ScrapeTimeout           interface{}      `yaml:"scrape_timeout,omitempty"`
	MetricsPath             interface{}      `yaml:"metrics_path,omitempty"`
	Scheme                  interface{}      `yaml:"scheme,omitempty"`
	BodySizeLimit           interface{}      `yaml:"body_size_limit,omitempty"`
	SampleLimit             interface{}      `yaml:"sample_limit,omitempty"`
	TargetLimit             interface{}      `yaml:"target_limit,omitempty"`
	LabelLimit              interface{}      `yaml:"label_limit,omitempty"`
	LabelNameLengthLimit    interface{}      `yaml:"label_name_length_limit,omitempty"`
	LabelValueLengthLimit   interface{}      `yaml:"label_value_length_limit,omitempty"`
	ServiceDiscoveryConfigs interface{}      `yaml:"-"`
	HTTPClientConfig        struct{}         `yaml:",inline"`
	RelabelConfigs          []*RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs    []*RelabelConfig `yaml:"metric_relabel_configs,omitempty"`
}

type AlertingConfig struct {
	AlertRelabelConfigs []interface{} `yaml:"alert_relabel_configs,omitempty"`
	AlertmanagerConfigs []interface{} `yaml:"alertmanagers,omitempty"`
}

type PrometheusConfig struct {
	GlobalConfig   interface{}     `yaml:"global"`
	AlertingConfig AlertingConfig  `yaml:"alerting,omitempty"`
	RuleFiles      []interface{}   `yaml:"rule_files,omitempty"`
	ScrapeConfigs  []*ScrapeConfig `yaml:"scrape_configs,omitempty"`
	StorageConfig  interface{}     `yaml:"storage,omitempty"`

	RemoteWriteConfigs []interface{} `yaml:"remote_write,omitempty"`
	RemoteReadConfigs  []interface{} `yaml:"remote_read,omitempty"`
}

func generateOtelConfig(promFilePath string, outputFilePath string, otelConfigTemplatePath string) error {
	var otelConfig OtelConfig

	//test code here

	// viper.SetConfigType("yaml") // or viper.SetConfigType("YAML")

	// any approach to require this configuration into your program.
	// 	var yamlExample = []byte(`
	// Hacker: true
	// name: steve
	// hobbies:
	// - skateboarding
	// - snowboarding
	// - go
	// clothing:
	//   jacket: leather
	//   trousers: denim
	// age: 35
	// eyes : brown
	// beard: true
	// `)

	// viper.ReadConfig(bytes.NewBuffer(yamlExample))

	// fmt.Printf("name: %v\n", viper.Get("name"))

	// test code here

	otelConfigFileContents, err := ioutil.ReadFile(otelConfigTemplatePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(otelConfigFileContents), &otelConfig)
	if err != nil {
		return err
	}

	var promConfig PrometheusConfig

	promConfigFileContents, err := ioutil.ReadFile(promFilePath)
	if err != nil {
		return err
	}
	// viper.SetConfigType("yaml") // or viper.SetConfigType("YAML")

	// viper.ReadConfig(bytes.NewBuffer(promConfigFileContents))

	// fmt.Printf("scrape configs: %v\n", viper.Get("scrape_configs"))

	err = yaml.Unmarshal([]byte(promConfigFileContents), &promConfig)
	if err != nil {
		return err
	}

	var test map[string]interface{}
	err = yaml.Unmarshal([]byte(promConfigFileContents), &test)
	if err != nil {
		return err
	}

	//fmt.Printf("%v\n", test["scrape_configs"])
	var sc = test["scrape_configs"].([]interface{})
	for _, rash := range sc {
		//fmt.Printf("rashmi -%v\n", rash)
		//fmt.Printf("%v\n", rash)
		rashmi := rash.(map[interface{}]interface{})
		fmt.Printf("rashmi -%v\n", rashmi["relabel_configs"])
	}

	// Need this here even though it is present in the receiver's config validate method since we only do the $ manipulation for regex and replacement fields
	// in scrape configs sections and the load method which is called before the validate method fails to unmarshal due to single $.
	// Either approach will fail but the receiver's config load wont return the right error message
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
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	if err != nil {
		return err
	}
	for _, scfg := range promConfig.ScrapeConfigs {
		for _, relabelConfig := range scfg.RelabelConfigs {
			regexString := relabelConfig.Regex
			// Replacing $$ with $ for backward compatibility, since golang doesnt support lookarounds cannot use this regex /(?<!\$)\$(?!\$)/ for checking single $
			modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
			modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
			relabelConfig.Regex = modifiedRegexString

			replacement := relabelConfig.Replacement
			modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
			modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
			if err != nil {
				return err
			}
			relabelConfig.Replacement = modifiedReplacementString
		}
		for _, metricRelabelConfig := range scfg.MetricRelabelConfigs {
			regexString := metricRelabelConfig.Regex
			// Replacing $$ with $ for backward compatibility, since golang doesnt support lookarounds cannot use this regex /(?<!\$)\$(?!\$)/ for checking single $
			modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
			modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")

			metricRelabelConfig.Regex = modifiedRegexString

			replacement := metricRelabelConfig.Replacement
			modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
			modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
			if err != nil {
				return err
			}
			metricRelabelConfig.Replacement = modifiedReplacementString
		}
	}

	otelConfig.Receivers.Prometheus.Config = promConfig

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
		parserProvider.Flags(flags)
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
