package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/converter/expandconverter"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	yaml "gopkg.in/yaml.v2"
)

type OtelConfig struct {
	Exporters  interface{} `yaml:"exporters"`
	Processors interface{} `yaml:"processors"`
	Extensions interface{} `yaml:"extensions"`
	Receivers  struct {
		Prometheus struct {
			Config          interface{} `yaml:"config"`
			TargetAllocator interface{} `yaml:"target_allocator"`
		} `yaml:"prometheus"`
	} `yaml:"receivers"`
	Service struct {
		Extensions interface{} `yaml:"extensions"`
		Pipelines  struct {
			Metrics struct {
				Exporters  interface{} `yaml:"exporters"`
				Processors interface{} `yaml:"processors"`
				Receivers  interface{} `yaml:"receivers"`
			} `yaml:"metrics"`
		} `yaml:"pipelines"`
		Telemetry struct {
			Logs struct {
				Level    interface{} `yaml:"level"`
				Encoding interface{} `yaml:"encoding"`
			} `yaml:"logs"`
		} `yaml:"telemetry"`
	} `yaml:"service"`
}

var RESET = "\033[0m"
var RED = "\033[31m"

const daemonSetControllerType = "DaemonSet"

func logFatalError(message string) {
	// Do not set env var if customer is running outside of agent to just validate config
	if os.Getenv("CONFIG_VALIDATOR_RUNNING_IN_AGENT") == "true" {
		setFatalErrorMessageAsEnvVar(message)
	}

	// Always log the full message
	log.Fatalf("%s%s%s", RED, message, RESET)
}

func setFatalErrorMessageAsEnvVar(message string) {
	// Truncate to use as a dimension in the invalid config metric for prometheus-collector-health job
	truncatedMessage := message
	if len(message) > 1023 {
		truncatedMessage = message[:1023]
	}

	// Replace newlines for env var to be set correctly
	re := regexp.MustCompile("\\n")
	truncatedMessage = re.ReplaceAllString(truncatedMessage, "")

	// Write env var to a file so it can be used by other processes
	file, err := os.Create("/opt/microsoft/prom_config_validator_env_var")
	if err != nil {
		log.Println("prom-config-validator::Unable to create file for prom_config_validator_env_var")
	}
	setEnvVarString := fmt.Sprintf("export INVALID_CONFIG_FATAL_ERROR=\"%s\"\n", truncatedMessage)
	if os.Getenv("OS_TYPE") != "linux" {
		setEnvVarString = fmt.Sprintf("INVALID_CONFIG_FATAL_ERROR=%s\n", truncatedMessage)
	}
	_, err = file.WriteString(setEnvVarString)
	if err != nil {
		log.Println("prom-config-validator::Unable to write to the file prom_config_validator_env_var")
	}
	file.Close()
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

	controllerType := os.Getenv("CONTROLLER_TYPE")
	isOperatorEnabled := os.Getenv("AZMON_OPERATOR_ENABLED")

	prometheusConfig, err := parsePrometheusConfig(promConfigFileContents)
	if err != nil {
		return err
	}

	err = modifyRelabelConfigs(prometheusConfig, controllerType, isOperatorEnabled)
	if err != nil {
		return err
	}

	unsupportedFeatures := getUnsupportedFeatures(prometheusConfig)
	if len(unsupportedFeatures) != 0 {
		return fmt.Errorf("unsupported features:\n\t%s", strings.Join(unsupportedFeatures, "\n\t"))
	}

	otelConfig.Receivers.Prometheus.Config = prometheusConfig

	if os.Getenv("DEBUG_MODE_ENABLED") == "true" {
		otelConfig.Service.Pipelines.Metrics.Exporters = []interface{}{"otlp", "prometheus"}
	}

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

func parsePrometheusConfig(promConfigFileContents []byte) (map[string]interface{}, error) {
	var prometheusConfig map[string]interface{}
	err := yaml.Unmarshal(promConfigFileContents, &prometheusConfig)
	if err != nil {
		return nil, err
	}
	return prometheusConfig, nil
}

func modifyRelabelConfigs(prometheusConfig map[string]interface{}, controllerType string, isOperatorEnabled string) error {
	scrapeConfigs := prometheusConfig["scrape_configs"]
	if scrapeConfigs != nil {
		var sc = scrapeConfigs.([]interface{})
		for _, scrapeConfig := range sc {
			scrapeConfig := scrapeConfig.(map[interface{}]interface{})
			if scrapeConfig["relabel_configs"] != nil {
				relabelConfigs := scrapeConfig["relabel_configs"].([]interface{})
				for _, relabelConfig := range relabelConfigs {
					relabelConfig := relabelConfig.(map[interface{}]interface{})
					modifyRelabelConfigFields(relabelConfig, controllerType, isOperatorEnabled)
				}
			}

			if scrapeConfig["metric_relabel_configs"] != nil {
				metricRelabelConfigs := scrapeConfig["metric_relabel_configs"].([]interface{})
				for _, metricRelabelConfig := range metricRelabelConfigs {
					metricRelabelConfig := metricRelabelConfig.(map[interface{}]interface{})
					modifyRelabelConfigFields(metricRelabelConfig, controllerType, isOperatorEnabled)
				}
			}
		}
	}
	return nil
}

func modifyRelabelConfigFields(relabelConfig map[interface{}]interface{}, controllerType string, isOperatorEnabled string) {
	if relabelConfig["regex"] != nil {
		if regexString, isString := relabelConfig["regex"].(string); isString {
			modifiedRegexString := strings.ReplaceAll(regexString, "$$", "$")
			if strings.EqualFold(controllerType, daemonSetControllerType) || strings.EqualFold(isOperatorEnabled, "false") {
				modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$", "$$")
				modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_NAME", "$NODE_NAME")
				modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_IP", "$NODE_IP")
			}
			relabelConfig["regex"] = modifiedRegexString
		}
	}

	if relabelConfig["replacement"] != nil {
		if replacement, isString := relabelConfig["replacement"].(string); isString {
			modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
			if strings.EqualFold(controllerType, daemonSetControllerType) || strings.EqualFold(isOperatorEnabled, "false") {
				modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
				modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_NAME", "$NODE_NAME")
				modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_IP", "$NODE_IP")
			}
			relabelConfig["replacement"] = modifiedReplacementString
		}
	}
}

func getUnsupportedFeatures(prometheusConfig map[string]interface{}) []string {
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
	return unsupportedFeatures
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
	log.SetFlags(0)
	configFilePtr := flag.String("config", "", "Config file to validate")
	outFilePtr := flag.String("output", "", "Output file path for writing collector config")
	otelTemplatePathPtr := flag.String("otelTemplate", "", "OTel Collector config template file path")
	flag.Parse()
	promFilePath := *configFilePtr
	otelConfigTemplatePath := *otelTemplatePathPtr
	if otelConfigTemplatePath == "" {
		logFatalError("prom-config-validator::Please provide otel config template path\n")
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
			logFatalError(fmt.Sprintf("Generating otel config failed: %v\n", err))
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
			logFatalError(fmt.Sprintf("prom-config-validator::Error parsing flags - %v\n", err))
			os.Exit(1)
		}

		factories, err := components()
		if err != nil {
			logFatalError(fmt.Sprintf("prom-config-validator::Failed to build components: %v\n", err))
			os.Exit(1)
		}

		cp, err := otelcol.NewConfigProvider(
			otelcol.ConfigProviderSettings{
				ResolverSettings: confmap.ResolverSettings{
					URIs:       []string{fmt.Sprintf("file:%s", outputFilePath)},
					Providers:  map[string]confmap.Provider{"file": fileprovider.New()},
					Converters: []confmap.Converter{expandconverter.New()},
				},
			},
		)
		if err != nil {
			logFatalError(fmt.Errorf("prom-config-validator::Cannot load configuration's parser: %w\n", err).Error())
			os.Exit(1)
		}

		fmt.Printf("prom-config-validator::Loading configuration...\n")
		cfg, err := cp.Get(context.Background(), factories)
		if err != nil {
			logFatalError(fmt.Sprintf("prom-config-validator::Cannot load configuration: %v", err))
			os.Exit(1)
		}

		err = cfg.Validate()
		if err != nil {
			logFatalError(fmt.Errorf("prom-config-validator::Invalid configuration: %w\n", err).Error())
			os.Exit(1)
		}
	} else {
		logFatalError("prom-config-validator::Please provide a config file using the --config flag to validate\n")
		os.Exit(1)
	}
	fmt.Printf("prom-config-validator::Successfully loaded and validated prometheus config\n")
	os.Exit(0)
}
