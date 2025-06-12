package configmapsettings

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/prometheus-collector/shared"

	"gopkg.in/yaml.v2"
)

const (
	configMapMountPath               = "/etc/config/settings/prometheus/prometheus-config"
	replicasetControllerType         = "replicaset"
	daemonsetControllerType          = "daemonset"
	configReaderSidecarContainerType = "configreadersidecar"
)

var (
	regexHash            = make(map[string]string)
	intervalHash         = make(map[string]string)
	mergedDefaultConfigs map[interface{}]interface{}
)

// parseConfigMap reads the prometheus config from the configmap
func parseConfigMap() string {
	defer func() {
		if r := recover(); r != nil {
			shared.EchoError(fmt.Sprintf("Recovered from panic: %v\n", r))
		}
	}()

	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		shared.EchoWarning("Custom prometheus config does not exist, using only default scrape targets if they are enabled")
		return ""
	}

	config, err := os.ReadFile(configMapMountPath)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Exception while parsing configmap: %s", err))
		return ""
	}

	return string(config)
}

func UpdateScrapeIntervalConfig(yamlConfigFile, scrapeIntervalSetting string) {
	fmt.Printf("Updating scrape interval config for %s\n", yamlConfigFile)

	var config map[string]interface{}
	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v\n", yamlConfigFile, err)
		return
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		fmt.Printf("Error unmarshalling YAML: %v\n", err)
		return
	}

	// Update scrape interval for each scrape config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				scfgMap["scrape_interval"] = scrapeIntervalSetting
			}
		}

		// Write updated config back to file
		cfgYaml, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML: %v\n", err)
			return
		}

		if err := os.WriteFile(yamlConfigFile, cfgYaml, fs.FileMode(0644)); err != nil {
			fmt.Printf("Error writing file: %v\n", err)
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML\n")
	}
}

// AppendMetricRelabelConfig adds a metric relabel config to keep specific metrics
func AppendMetricRelabelConfig(yamlConfigFile, keepListRegex string) error {
	fmt.Printf("Appending keep list regex to %s\n", yamlConfigFile)

	var config map[string]interface{}
	content, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("error unmarshalling YAML: %v", err)
	}

	keepListMetricRelabelConfig := map[string]interface{}{
		"source_labels": []interface{}{"__name__"},
		"action":        "keep",
		"regex":         keepListRegex,
	}

	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for i, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				stringScfgMap := make(map[string]interface{})
				for k, v := range scfgMap {
					if key, ok := k.(string); ok {
						stringScfgMap[key] = v
					} else {
						return fmt.Errorf("non-string key in scrape config: %v", k)
					}
				}

				// Add or update metric_relabel_configs
				if metricRelabelCfgs, ok := stringScfgMap["metric_relabel_configs"].([]interface{}); ok {
					stringScfgMap["metric_relabel_configs"] = append(metricRelabelCfgs, keepListMetricRelabelConfig)
				} else {
					stringScfgMap["metric_relabel_configs"] = []interface{}{keepListMetricRelabelConfig}
				}

				// Convert back for YAML marshalling
				interfaceScfgMap := make(map[interface{}]interface{})
				for k, v := range stringScfgMap {
					interfaceScfgMap[k] = v
				}
				scrapeConfigs[i] = interfaceScfgMap
			}
		}
		config["scrape_configs"] = scrapeConfigs
	} else {
		fmt.Println("No 'scrape_configs' found in the YAML")
		return nil
	}

	cfgYaml, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshalling YAML: %v", err)
	}

	return os.WriteFile(yamlConfigFile, cfgYaml, os.ModePerm)
}

// UpdatePlaceholders replaces placeholders in a file with environment variables
func UpdatePlaceholders(yamlConfigFile string, placeholders []string) error {
	contents, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return err
	}

	for _, placeholder := range placeholders {
		contents = []byte(strings.ReplaceAll(string(contents), fmt.Sprintf("$$%s$$", placeholder), os.Getenv(placeholder)))
	}

	return os.WriteFile(kubeletDefaultFileDs, contents, 0644)
}

// processDefaultJob processes a single default scrape job
func processDefaultJob(job *scrapeConfigs.ScrapeJob) {
	if job.ScrapeInterval != "" {
		UpdateScrapeIntervalConfig(job.ScrapeConfigDefinitionFile, job.ScrapeInterval)
	}

	if job.KeepListRegex != "" {
		AppendMetricRelabelConfig(job.ScrapeConfigDefinitionFile, job.KeepListRegex)
	}

	if job.ControllerType == scrapeConfigs.ControllerType.DaemonSet {
		if err := UpdatePlaceholders(job.ScrapeConfigDefinitionFile, []string{"NODE_IP", "NODE_NAME"}); err != nil {
			fmt.Printf("Error updating placeholders for DaemonSet: %v\n", err)
		}
	}

	if job.PlaceholderNames != nil {
		if err := UpdatePlaceholders(job.ScrapeConfigDefinitionFile, job.PlaceholderNames); err != nil {
			fmt.Printf("Error updating placeholders: %v\n", err)
		}
	}
}

// populateDefaultPrometheusConfig processes default scrape jobs
func populateDefaultPrometheusConfig() {
	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(strings.ToLower(os.Getenv("CONTROLLER_TYPE")))
	osType := strings.ToLower(os.Getenv("OS_TYPE"))

	for _, job := range scrapeConfigs.DefaultScrapeJobs {
		if job.Enabled && job.ControllerType == currentControllerType && job.OSType == osType {
			processDefaultJob(&job)
			defaultConfigs = append(defaultConfigs, job.ScrapeConfigDefinitionFile)
		}
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
}

// mergeDefaultScrapeConfigs merges multiple YAML files into one config
func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	mergedDefaultConfigs := make(map[interface{}]interface{})

	if len(defaultScrapeConfigs) > 0 {
		mergedDefaultConfigs["scrape_configs"] = make([]interface{}, 0)

		for _, configFile := range defaultScrapeConfigs {
			defaultConfigYaml, err := loadYAMLFromFile(configFile)
			if err != nil {
				log.Printf("Error loading YAML from file %s: %s\n", configFile, err)
				continue
			}

			mergedDefaultConfigs = deepMerge(mergedDefaultConfigs, defaultConfigYaml)
		}
	}

	fmt.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))
	return mergedDefaultConfigs
}

// loadYAMLFromFile loads a YAML file into a map
func loadYAMLFromFile(filename string) (map[interface{}]interface{}, error) {
	var yamlData map[interface{}]interface{}
	if err := loadYAMLData(filename, &yamlData); err != nil {
		return nil, err
	}
	return yamlData, nil
}

// deepMerge merges two maps recursively
func deepMerge(target, source map[interface{}]interface{}) map[interface{}]interface{} {
	for key, sourceValue := range source {
		targetValue, exists := target[key]

		if !exists {
			target[key] = sourceValue
			continue
		}

		targetMap, targetMapOk := targetValue.(map[interface{}]interface{})
		sourceMap, sourceMapOk := sourceValue.(map[interface{}]interface{})

		if targetMapOk && sourceMapOk {
			target[key] = deepMerge(targetMap, sourceMap)
		} else if reflect.TypeOf(targetValue) == reflect.TypeOf(sourceValue) {
			// Both are slices, concatenate them
			if targetSlice, ok := targetValue.([]interface{}); ok {
				if sourceSlice, ok := sourceValue.([]interface{}); ok {
					target[key] = append(targetSlice, sourceSlice...)
				}
			}
		} else {
			// Different types, overwrite with source
			target[key] = sourceValue
		}
	}

	return target
}

// writeDefaultScrapeTargetsFile writes the merged default config to a file
func writeDefaultScrapeTargetsFile(operatorEnabled bool) map[interface{}]interface{} {
	if strings.ToLower(os.Getenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED")) == "false" {
		if operatorEnabled {
			populateDefaultPrometheusConfig() // Simplified by removing redundant function
		} else {
			populateDefaultPrometheusConfig()
		}

		if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
			fmt.Printf("Starting to merge default prometheus config values in collector template as backup\n")
			mergedDefaultConfigYaml, err := yaml.Marshal(mergedDefaultConfigs)
			if err != nil {
				fmt.Printf("Error marshalling merged default prometheus config: %v\n", err)
				return nil
			}

			if err := os.WriteFile(mergedDefaultConfigPath, mergedDefaultConfigYaml, fs.FileMode(0644)); err != nil {
				fmt.Printf("Error writing merged default prometheus config to file: %v\n", err)
				return nil
			}

			return mergedDefaultConfigs
		}
	} else {
		mergedDefaultConfigs = nil
	}

	fmt.Printf("Done creating default targets file\n")
	return nil
}

func setDefaultFileScrapeInterval(scrapeInterval string) {
	defaultFilesArray := []string{
		kubeletDefaultFileRsSimple, kubeletDefaultFileRsAdvanced, kubeletDefaultFileDs,
		kubeletDefaultFileRsAdvancedWindowsDaemonset, coreDNSDefaultFile,
		cadvisorDefaultFileRsSimple, cadvisorDefaultFileRsAdvanced, cadvisorDefaultFileDs,
		kubeProxyDefaultFile, apiserverDefaultFile, kubeStateDefaultFile,
		nodeExporterDefaultFileRsSimple, nodeExporterDefaultFileRsAdvanced, nodeExporterDefaultFileDs,
		prometheusCollectorHealthDefaultFile, windowsExporterDefaultRsSimpleFile, windowsExporterDefaultDsFile,
		windowsKubeProxyDefaultFileRsSimpleFile, windowsKubeProxyDefaultDsFile, podAnnotationsDefaultFile,
		kappieBasicDefaultFileDs, networkObservabilityRetinaDefaultFileDs, networkObservabilityHubbleDefaultFileDs,
		networkObservabilityCiliumDefaultFileDs, acstorMetricsExporterDefaultFile, acstorCapacityProvisionerDefaultFile,
	}

	for _, currentFile := range defaultFilesArray {
		contents, err := os.ReadFile(fmt.Sprintf("%s%s", defaultPromConfigPathPrefix, currentFile))
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", currentFile, err)
			continue
		}

		contents = []byte(strings.Replace(string(contents), "$$SCRAPE_INTERVAL$$", scrapeInterval, -1))

		err = os.WriteFile(currentFile, contents, fs.FileMode(0644))
		if err != nil {
			fmt.Printf("Error writing to file %s: %v\n", currentFile, err)
		}
	}
}

func setLabelLimitsPerScrape(prometheusConfigString string) string {
	var limitedCustomConfig map[interface{}]interface{}

	if err := yaml.Unmarshal([]byte(prometheusConfigString), &limitedCustomConfig); err != nil {
		shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
		return prometheusConfigString
	}

	if limitedCustomConfig == nil || len(limitedCustomConfig) == 0 {
		shared.EchoWarning("Nothing to set for label limits")
		return prometheusConfigString
	}

	limitedCustomScrapes, ok := limitedCustomConfig["scrape_configs"].([]interface{})
	if !ok || len(limitedCustomScrapes) == 0 {
		shared.EchoWarning("No jobs found to set label limits")
		return prometheusConfigString
	}

	for _, scrape := range limitedCustomScrapes {
		if scrapeMap, ok := scrape.(map[interface{}]interface{}); ok {
			scrapeMap["label_limit"] = 63
			scrapeMap["label_name_length_limit"] = 511
			scrapeMap["label_value_length_limit"] = 1023
			shared.EchoVar(fmt.Sprintf("Successfully set label limits in custom scrape config for job %s", scrapeMap["job_name"]), "")
		}
	}
	shared.EchoWarning("Done setting label limits")
	updatedConfig, err := yaml.Marshal(limitedCustomConfig)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error marshalling config: %v", err))
		return prometheusConfigString
	}

	return string(updatedConfig)
}

func setGlobalScrapeConfigInDefaultFilesIfExists(configString string) string {
	var customConfig map[interface{}]interface{}
	if err := yaml.Unmarshal([]byte(configString), &customConfig); err != nil {
		fmt.Println("Error:", err)
		return ""
	}

	// Set scrape interval to 30s for updating the default merged config
	scrapeInterval := defaultScrapeInterval

	if globalConfig, ok := customConfig["global"].(map[interface{}]interface{}); ok {
		if si, ok := globalConfig["scrape_interval"].(string); ok {
			// Validate scrape interval format
			if matched := regexp.MustCompile(`^((\d+y)?(\d+w)?(\d+d)?(\d+h)?(\d+m)?(\d+s)?(\d+ms)?|0)$`).MatchString(si); matched {
				scrapeInterval = si
			} else {
				globalConfig["scrape_interval"] = "1m"
			}
		}
	}

	setDefaultFileScrapeInterval(scrapeInterval)

	updatedConfig, err := yaml.Marshal(customConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}

	return string(updatedConfig)
}

func mergeDefaultAndCustomScrapeConfigs(customPromConfig string, mergedDefaultConfigs map[interface{}]interface{}) {
	var mergedConfigYaml []byte

	if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
		shared.EchoStr("Merging default and custom scrape configs")
		var customPrometheusConfig map[interface{}]interface{}

		if err := yaml.Unmarshal([]byte(customPromConfig), &customPrometheusConfig); err != nil {
			shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
			return
		}

		mergedConfigs := deepMerge(mergedDefaultConfigs, customPrometheusConfig)
		var err error
		if mergedConfigYaml, err = yaml.Marshal(mergedConfigs); err != nil {
			shared.EchoError(fmt.Sprintf("Error marshalling merged configs: %v", err))
			return
		}
	} else {
		shared.EchoWarning("Using only custom scrape config")
		mergedConfigYaml = []byte(customPromConfig)
	}

	if err := os.WriteFile(promMergedConfigPath, mergedConfigYaml, fs.FileMode(0644)); err != nil {
		shared.EchoError(fmt.Sprintf("Error writing merged config: %v", err))
	}
}

// prometheusConfigMerger is the main function that merges configs
func prometheusConfigMerger(operatorEnabled bool) {
	shared.EchoSectionDivider("Start Processing - prometheusConfigMerger")
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	prometheusConfigMap := parseConfigMap()

	if len(prometheusConfigMap) > 0 {
		modifiedPrometheusConfigString := setGlobalScrapeConfigInDefaultFilesIfExists(prometheusConfigMap)
		writeDefaultScrapeTargetsFile(operatorEnabled)
		// Set label limits for every custom scrape job, before merging the default & custom config
		labellimitedconfigString := setLabelLimitsPerScrape(modifiedPrometheusConfigString)
		mergeDefaultAndCustomScrapeConfigs(labellimitedconfigString, mergedDefaultConfigs)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Merging Default and Custom Prometheus Config")
	} else {
		setDefaultFileScrapeInterval(defaultScrapeInterval)
		writeDefaultScrapeTargetsFile(operatorEnabled)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Writing Default Prometheus Config")
	}
}
