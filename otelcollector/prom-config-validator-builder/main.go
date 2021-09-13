package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	//"regexp"
	"strings"

	//relabel "github.com/prometheus/prometheus/pkg/relabel"
	"go.opentelemetry.io/collector/config/configloader"
	parserProvider "go.opentelemetry.io/collector/service/parserprovider"
	yaml "gopkg.in/yaml.v2"
)

type OtelConfig struct {
	Receivers struct {
		Prometheus struct {
			Config interface{} `yaml:"config"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Service    interface{} `yaml:"service"`
}

// type OtelConfig struct {
// 	Receivers struct {
// 		Prometheus struct {
// 			Config PrometheusConfig `yaml:"config"`
// 		} `yaml:"prometheus"`
// 	} `yaml:"receivers"`
// 	Exporters  interface{} `yaml:"exporters"`
// 	Processors interface{} `yaml:"processors"`
// 	Service    interface{} `yaml:"service"`
// }

// type Regexp struct {
// 	*regexp.Regexp
// 	original string
// }

// func NewRegexp(s string) (Regexp, error) {
// 	regex, err := regexp.Compile("^(?:" + s + ")$")
// 	return Regexp{
// 		Regexp:   regex,
// 		original: s,
// 	}, err
// }

type RelabelConfig struct {
	SourceLabels interface{} `yaml:"source_labels,flow,omitempty"`
	Separator    interface{} `yaml:"separator,omitempty"`
	// Regex        Regexp      `yaml:"regex,omitempty"`
	Regex       string      `yaml:"regex,omitempty"`
	Modulus     interface{} `yaml:"modulus,omitempty"`
	TargetLabel interface{} `yaml:"target_label,omitempty"`
	Replacement string      `yaml:"replacement,omitempty"`
	Action      interface{} `yaml:"action,omitempty"`
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
	GlobalConfig   interface{}    `yaml:"global"`
	AlertingConfig AlertingConfig `yaml:"alerting,omitempty"`
	RuleFiles      []interface{}  `yaml:"rule_files,omitempty"`
	//ScrapeConfigs  []*pconfig.ScrapeConfig `yaml:"scrape_configs,omitempty"`
	// ScrapeConfigs []*ScrapeConfig `yaml:"scrape_configs,omitempty"`
	ScrapeConfigs []*ScrapeConfig `yaml:"scrape_configs,omitempty"`
	StorageConfig interface{}     `yaml:"storage,omitempty"`

	RemoteWriteConfigs []interface{} `yaml:"remote_write,omitempty"`
	RemoteReadConfigs  []interface{} `yaml:"remote_read,omitempty"`
}

func generateOtelConfig(promFilePath string) error {
	fmt.Printf("in generate\n")

	var otelConfig OtelConfig
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
	//promConfig := make(map[interface{}]interface{})
	// promConfig, err := prometheusConfig.LoadFile(promFilePath, false, gokitLogger.NewNopLogger())
	// if err != nil {
	// 	return err
	// }
	var promConfig PrometheusConfig
	promConfigFileContents, err := os.ReadFile(promFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal([]byte(promConfigFileContents), &promConfig)
	if err != nil {
		return err
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
		// Sort the values for deterministic error messages.
		//sort.Strings(unsupportedFeatures)
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	//singleDollarRegex, _ := regexp.Compile(`\$`)
	//singleDollarRegex, err := regexp.Compile(`(?<!\$)\$(?!\$)`)
	if err != nil {
		return err
	}
	//fmt.Printf("here\n")
	for _, scfg := range promConfig.ScrapeConfigs {
		for _, relabelConfig := range scfg.RelabelConfigs {
			// regexString := relabelConfig.Regex.String()
			regexString := relabelConfig.Regex
			// Replacing $$ with $ for backward compatibility, since golang doesnt support lookarounds cannot use this regex /(?<!\$)\$(?!\$)/ for checking single $
			modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
			modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
			//modifiedRegex, err := relabel.NewRegexp(modifiedRegexString)
			//modifiedRegex, err := NewRegexp(modifiedRegexString)
			// if err != nil {
			// 	return err
			// }
			//relabelConfig.Regex = modifiedRegex
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
			// regexString := metricRelabelConfig.Regex.String()
			regexString := metricRelabelConfig.Regex
			// Replacing $$ with $ for backward compatibility
			modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
			modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
			// modifiedRegex, err := relabel.NewRegexp(modifiedRegexString)
			//modifiedRegex, err := NewRegexp(modifiedRegexString)
			// if err != nil {
			// 	return err
			// }
			// metricRelabelConfig.Regex = modifiedRegex
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

	// }
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
