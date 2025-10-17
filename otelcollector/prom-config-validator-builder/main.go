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

	"github.com/prometheus-collector/shared"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
	yaml "gopkg.in/yaml.v2"
)

var RESET = "\033[0m"
var RED = "\033[31m"

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
	file, err := os.OpenFile("/opt/microsoft/prom_config_validator_env_var", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("prom-config-validator::Unable to open file - prom_config_validator_env_var")
	}
	setEnvVarString := fmt.Sprintf("export INVALID_CONFIG_FATAL_ERROR=\"%s\"\n", truncatedMessage)
	if os.Getenv("OS_TYPE") != "linux" {
		setEnvVarString = fmt.Sprintf("INVALID_CONFIG_FATAL_ERROR=%s\n", truncatedMessage)
	}
	_, err = file.Write([]byte(setEnvVarString))
	if err != nil {
		log.Println("prom-config-validator::Unable to write to the file prom_config_validator_env_var")
	}
	file.Close()
}

func generateOtelConfig(promFilePath string, outputFilePath string, otelConfigTemplatePath string) error {
	var otelConfig shared.OtelConfig

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
							// Doing the below since we dont want to substitute $ with $$ for env variables NODE_NAME and NODE_IP.
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_NAME", "${env:NODE_NAME}")
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_IP", "${env:NODE_IP}")
							relabelConfig["regex"] = modifiedRegexString
						}
					}
					//replace $ with $$ for replacement field
					if relabelConfig["replacement"] != nil {
						replacement := relabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_NAME", "${env:NODE_NAME}")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_IP", "${env:NODE_IP}")
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
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_NAME", "${env:NODE_NAME}")
							modifiedRegexString = strings.ReplaceAll(modifiedRegexString, "$$NODE_IP", "${env:NODE_IP}")
							metricRelabelConfig["regex"] = modifiedRegexString
						}
					}

					//replace $ with $$ for replacement field
					if metricRelabelConfig["replacement"] != nil {
						replacement := metricRelabelConfig["replacement"].(string)
						modifiedReplacementString := strings.ReplaceAll(replacement, "$$", "$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$", "$$")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_NAME", "${env:NODE_NAME}")
						modifiedReplacementString = strings.ReplaceAll(modifiedReplacementString, "$$NODE_IP", "${env:NODE_IP}")
						metricRelabelConfig["replacement"] = modifiedReplacementString
					}
				}
			}

			if scrapeConfig["static_configs"] != nil {
				staticConfigs := scrapeConfig["static_configs"].([]interface{})
				for _, staticConfig := range staticConfigs {
					staticConfig := staticConfig.(map[interface{}]interface{})
					if staticConfig["targets"] != nil {
						targets := staticConfig["targets"].([]interface{})
						for i, target := range targets {
							if _, isString := target.(string); isString {
								targetValue := target.(string)
								modifiedtargetValue := strings.ReplaceAll(targetValue, "$$NODE_NAME", "$NODE_NAME")
								modifiedtargetValue = strings.ReplaceAll(modifiedtargetValue, "$$NODE_IP", "$NODE_IP")
								modifiedtargetValue = strings.ReplaceAll(modifiedtargetValue, "$NODE_NAME", "${env:NODE_NAME}")
								modifiedtargetValue = strings.ReplaceAll(modifiedtargetValue, "$NODE_IP", "${env:NODE_IP}")
								staticConfig["targets"].([]interface{})[i] = modifiedtargetValue
							}
						}
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

	globalSettingsFromMergedOtelConfig := prometheusConfig["global"]

	if globalSettingsFromMergedOtelConfig != nil {
		globalSettings := globalSettingsFromMergedOtelConfig.(map[interface{}]interface{})
		scrapeInterval := globalSettings["scrape_interval"]
		if (len(globalSettings) > 1) || (len(globalSettings) == 1 && scrapeInterval != "15s") {
			setEnvVarString := fmt.Sprintf("AZMON_GLOBAL_SETTINGS_CONFIGURED=true\n")
			file, err := os.OpenFile("/opt/microsoft/prom_config_validator_env_var", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				log.Println("prom-config-validator::Unable to open file - prom_config_validator_env_var")
			}
			_, err = file.Write([]byte(setEnvVarString))
			if err != nil {
				log.Println("prom-config-validator::Unable to write to the file prom_config_validator_env_var")
			}
			file.Close()
			if err != nil {
				log.Println("prom-config-validator::Unable to close file prom_config_validator_env_var", err)
			} else {
				log.Printf("prom-config-validator::Successfully set env variables for global config in file prom_config_validator_env_var\n")
			}
		}
	}

	otelConfig.Receivers.Prometheus.Config = prometheusConfig

	if os.Getenv("DEBUG_MODE_ENABLED") == "true" {
		otelConfig.Service.Pipelines.Metrics.Exporters = []interface{}{"otlp", "prometheus"}
		if os.Getenv("CCP_METRICS_ENABLED") != "true" {
			otelConfig.Service.Pipelines.MetricsTelemetry.Receivers = []interface{}{"prometheus"}
			otelConfig.Service.Pipelines.MetricsTelemetry.Exporters = []interface{}{"prometheus/telemetry"}
			otelConfig.Service.Pipelines.MetricsTelemetry.Processors = []interface{}{"filter/telemetry"}
		}
	}

	mergedConfig, err := yaml.Marshal(otelConfig)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(outputFilePath, mergedConfig, 0644); err != nil {
		return err
	}
	log.Printf("prom-config-validator::Successfully generated otel config\n")
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
		log.Printf("prom-config-validator::Config file provided - %s\n", promFilePath)

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

		fmp := fileprovider.NewFactory()
		envp := envprovider.NewFactory()
		providers := []confmap.ProviderFactory{fmp, envp}
		cp, err := otelcol.NewConfigProvider(
			otelcol.ConfigProviderSettings{
				ResolverSettings: confmap.ResolverSettings{
					URIs:              []string{fmt.Sprintf("file:%s", outputFilePath)},
					ProviderFactories: providers,
				},
			},
		)
		if err != nil {
			logFatalError(fmt.Errorf("prom-config-validator::Cannot load configuration's parser: %w\n", err).Error())
			os.Exit(1)
		}

		log.Printf("prom-config-validator::Loading configuration...\n")
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
	log.Printf("prom-config-validator::Successfully loaded and validated prometheus config\n")
	os.Exit(0)
}
