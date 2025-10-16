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

var (
	configMapMountPath               = "/etc/config/settings/prometheus/prometheus-config"
	replicasetControllerType         = "replicaset"
	daemonsetControllerType          = "daemonset"
	configReaderSidecarContainerType = "configreadersidecar"
	scrapeConfigDefinitionPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
)

var mergedDefaultConfigs = make(map[interface{}]interface{})

func parseConfigMap() string {
	defer func() {
		if r := recover(); r != nil {
			shared.EchoError(fmt.Sprintf("Recovered from panic: %v\n", r))
		}
	}()

	log.Println("Starting to parse prometheus configmap", configMapMountPath)
	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		shared.EchoWarning("Custom prometheus config does not exist, using only default scrape targets if they are enabled")
		return ""
	}

	config, err := os.ReadFile(configMapMountPath)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Exception while parsing configmap for prometheus config: %s. Custom prometheus config will not be used. Please check configmap for errors", err))
		return ""
	}

	return string(config)
}

func isConfigReaderSidecar() bool {
	containerType := os.Getenv("CONTAINER_TYPE")
	if containerType != "" {
		currentContainerType := strings.ToLower(strings.TrimSpace(containerType))
		if currentContainerType == configReaderSidecarContainerType {
			return true
		}
	}
	return false
}

func UpdateScrapeIntervalConfig(yamlConfigFile, scrapeIntervalSetting string) {
	log.Printf("Updating scrape interval config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		log.Printf("Error reading config file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Error unmarshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Update scrape interval config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				log.Printf("scrapeInterval %s\n", scrapeIntervalSetting)
				scfgMap["scrape_interval"] = scrapeIntervalSetting
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithScrapeConfig, err := yaml.Marshal(config)
		if err != nil {
			log.Printf("Error marshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = os.WriteFile(yamlConfigFile, []byte(cfgYamlWithScrapeConfig), fs.FileMode(0644))
		if err != nil {
			log.Printf("Error writing to file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}
	} else {
		log.Printf("No 'scrape_configs' found in the YAML. The scrape interval will not be updated.\n")
	}
}

func AppendMetricRelabelConfig(yamlConfigFile, keepListRegex string) error {
	log.Printf("Starting to append keep list regex or minimal ingestion regex to %s\n", yamlConfigFile)

	content, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("error reading config file %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("error unmarshalling YAML for %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	keepListMetricRelabelConfig := map[string]interface{}{
		"source_labels": []interface{}{"__name__"},
		"action":        "keep",
		"regex":         keepListRegex,
	}

	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for i, scfg := range scrapeConfigs {
			// Ensure scfg is a map with string keys
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				stringScfgMap := make(map[string]interface{})
				for k, v := range scfgMap {
					if key, ok := k.(string); ok {
						stringScfgMap[key] = v
					} else {
						return fmt.Errorf("encountered non-string key in scrape config map: %v", k)
					}
				}

				// Update or add metric_relabel_configs
				if metricRelabelCfgs, ok := stringScfgMap["metric_relabel_configs"].([]interface{}); ok {
					stringScfgMap["metric_relabel_configs"] = append(metricRelabelCfgs, keepListMetricRelabelConfig)
				} else {
					stringScfgMap["metric_relabel_configs"] = []interface{}{keepListMetricRelabelConfig}
				}

				// Convert back to map[interface{}]interface{} for YAML marshalling
				interfaceScfgMap := make(map[interface{}]interface{})
				for k, v := range stringScfgMap {
					interfaceScfgMap[k] = v
				}

				// Update the scrape_configs list
				scrapeConfigs[i] = interfaceScfgMap
			}
		}

		// Write updated scrape_configs back to config
		config["scrape_configs"] = scrapeConfigs
	} else {
		log.Println("No 'scrape_configs' found in the YAML. Skipping updates.")
		return nil
	}

	cfgYamlWithMetricRelabelConfig, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshalling YAML for %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	if err := os.WriteFile(yamlConfigFile, cfgYamlWithMetricRelabelConfig, os.ModePerm); err != nil {
		return fmt.Errorf("error writing to file %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	return nil
}

func AppendRelabelConfig(yamlConfigFile string, relabelConfig []map[string]interface{}, keepRegex string) {
	log.Printf("Adding relabel config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		log.Printf("Error reading config file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Error unmarshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Append relabel config for keep list to each scrape config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				relabelCfgs, exists := scfgMap["relabel_configs"]
				if !exists {
					scfgMap["relabel_configs"] = relabelConfig
				} else if relabelCfgsSlice, ok := relabelCfgs.([]interface{}); ok {
					for _, rc := range relabelConfig {
						relabelCfgsSlice = append(relabelCfgsSlice, rc)
					}
					scfgMap["relabel_configs"] = relabelCfgsSlice
				}
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithRelabelConfig, err := yaml.Marshal(config)
		if err != nil {
			log.Printf("Error marshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = os.WriteFile(yamlConfigFile, []byte(cfgYamlWithRelabelConfig), fs.FileMode(0644))
		if err != nil {
			log.Printf("Error writing to file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}
	} else {
		log.Printf("No 'scrape_configs' found in the YAML. The relabel config will not be added.\n")
	}
}

func populateDefaultPrometheusConfig() {
	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(os.Getenv("CONTROLLER_TYPE"))
	if isConfigReaderSidecar() {
		currentControllerType = shared.ControllerType.ReplicaSet
	}
	osType := strings.ToLower(os.Getenv("OS_TYPE"))

	for _, job := range shared.DefaultScrapeJobs {
		if job.Enabled && job.ControllerType == currentControllerType && job.OSType == osType {
			processDefaultJob(job)
			defaultConfigs = append(defaultConfigs, scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile)
		}
	}
	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
}

func UpdatePlaceholders(yamlConfigFile string, placeholders []string) error {
	contents, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return err
	}

	for _, placeholder := range placeholders {
		contents = []byte(strings.ReplaceAll(string(contents), fmt.Sprintf("$$%s$$", placeholder), os.Getenv(placeholder)))
	}

	return os.WriteFile(yamlConfigFile, contents, 0644)
}

func processDefaultJob(job *shared.DefaultScrapeJob) {
	if job.ScrapeInterval != "" {
		UpdateScrapeIntervalConfig(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, job.ScrapeInterval)
	}

	if job.KeepListRegex != "" {
		AppendMetricRelabelConfig(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, job.KeepListRegex)
	}

	if job.ControllerType == shared.ControllerType.DaemonSet {
		if err := UpdatePlaceholders(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, []string{"NODE_IP", "NODE_NAME"}); err != nil {
			log.Printf("Error updating placeholders for DaemonSet: %v\n", err)
		}
	}

	if job.PlaceholderNames != nil {
		if err := UpdatePlaceholders(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, job.PlaceholderNames); err != nil {
			log.Printf("Error updating placeholders: %v\n", err)
		}
	}
}

func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	if len(defaultScrapeConfigs) > 0 {
		mergedDefaultConfigs["scrape_configs"] = make([]interface{}, 0)

		for _, defaultScrapeConfig := range defaultScrapeConfigs {
			defaultConfigYaml, err := loadYAMLFromFile(defaultScrapeConfig)
			if err != nil {
				log.Printf("Error loading YAML from file %s: %s\n", defaultScrapeConfig, err)
				continue
			}

			mergedDefaultConfigs = deepMerge(mergedDefaultConfigs, defaultConfigYaml)
		}
	}

	log.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))

	return mergedDefaultConfigs
}

func loadYAMLFromFile(filename string) (map[interface{}]interface{}, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var yamlData map[interface{}]interface{}
	err = yaml.Unmarshal(fileContent, &yamlData)
	if err != nil {
		return nil, err
	}

	return yamlData, nil
}

// This needs unit tests

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
			if targetSlice, targetSliceOk := targetValue.([]interface{}); targetSliceOk {
				if sourceSlice, sourceSliceOk := sourceValue.([]interface{}); sourceSliceOk {
					target[key] = append(targetSlice, sourceSlice...)
				}
			}
		} else {
			// If types are different, simply overwrite with the source value
			target[key] = sourceValue
		}
	}

	return target
}

func writeDefaultScrapeTargetsFile(operatorEnabled bool) map[interface{}]interface{} {
	noDefaultScrapingEnabled := os.Getenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED")
	log.Println("No Default Scraping Enabled:", noDefaultScrapingEnabled)
	if noDefaultScrapingEnabled != "true" {
		populateDefaultPrometheusConfig()
		if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
			log.Printf("Starting to merge default prometheus config values in collector template as backup\n")
			mergedDefaultConfigYaml, err := yaml.Marshal(mergedDefaultConfigs)
			if err != nil {
				log.Printf("Error marshalling merged default prometheus config: %v\n", err)
				return nil
			}

			err = os.WriteFile(mergedDefaultConfigPath, mergedDefaultConfigYaml, fs.FileMode(0644))
			if err != nil {
				log.Printf("Error writing merged default prometheus config to file: %v\n", err)
				return nil
			}

			return mergedDefaultConfigs
		}
	} else {
		mergedDefaultConfigs = make(map[interface{}]interface{})
	}
	log.Printf("Done creating default targets file\n")
	return nil
}

func setDefaultFileScrapeInterval(scrapeInterval string) {
	for _, job := range shared.DefaultScrapeJobs {
		if job.ScrapeInterval != "" {
			scrapeInterval = job.ScrapeInterval
		}
		if job.ScrapeConfigDefinitionFile != "" {
			currentFile := scrapeConfigDefinitionPathPrefix + job.ScrapeConfigDefinitionFile
			contents, err := os.ReadFile(currentFile)
			if err != nil {
				log.Printf("Error reading file %s: %v\n", currentFile, err)
				continue
			}
			contents = []byte(strings.Replace(string(contents), "$$SCRAPE_INTERVAL$$", scrapeInterval, -1))
			err = os.WriteFile(currentFile, contents, fs.FileMode(0644))
			if err != nil {
				log.Printf("Error writing to file %s: %v\n", currentFile, err)
			}
		}
	}
}

func mergeDefaultAndCustomScrapeConfigs(customPromConfig string, mergedDefaultConfigs map[interface{}]interface{}) {
	var mergedConfigYaml []byte

	if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
		shared.EchoStr("Merging default and custom scrape configs")
		var customPrometheusConfig map[interface{}]interface{}
		err := yaml.Unmarshal([]byte(customPromConfig), &customPrometheusConfig)
		if err != nil {
			shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
			return
		}

		var mergedConfigs map[interface{}]interface{}
		if customPrometheusConfig["scrape_configs"] != nil {
			mergedConfigs = deepMerge(mergedDefaultConfigs, customPrometheusConfig)
		} else {
			delete(customPrometheusConfig, "scrape_configs")
			mergedConfigs = deepMerge(mergedDefaultConfigs, customPrometheusConfig)
		}

		mergedConfigYaml, err = yaml.Marshal(mergedConfigs)
		if err != nil {
			shared.EchoError(fmt.Sprintf("Error marshalling merged configs: %v", err))
			return
		}

		shared.EchoStr("Done merging default scrape config(s) with custom prometheus config, writing them to file")
	} else {
		shared.EchoWarning("The merged default scrape config is nil or empty, using only custom scrape config")
		mergedConfigYaml = []byte(customPromConfig)
	}

	err := os.WriteFile(promMergedConfigPath, mergedConfigYaml, fs.FileMode(0644))
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error writing merged config to file: %v", err))
		return
	}
}

func setLabelLimitsPerScrape(prometheusConfigString string) string {
	customConfig := prometheusConfigString

	var limitedCustomConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(customConfig), &limitedCustomConfig)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
		return prometheusConfigString
	}

	if limitedCustomConfig != nil && len(limitedCustomConfig) > 0 {
		limitedCustomScrapes, _ := limitedCustomConfig["scrape_configs"].([]interface{})
		if limitedCustomScrapes != nil && len(limitedCustomScrapes) > 0 {
			for _, scrape := range limitedCustomScrapes {
				scrapeMap, _ := scrape.(map[interface{}]interface{})
				scrapeMap["label_limit"] = 63
				scrapeMap["label_name_length_limit"] = 511
				scrapeMap["label_value_length_limit"] = 1023
				shared.EchoVar(fmt.Sprintf("Successfully set label limits in custom scrape config for job %s", scrapeMap["job_name"]), "")
			}
			shared.EchoWarning("Done setting label limits for custom scrape config ...")
			updatedConfig, err := yaml.Marshal(limitedCustomConfig)
			if err != nil {
				shared.EchoError(fmt.Sprintf("Error marshalling custom config: %v", err))
				return prometheusConfigString
			}
			return string(updatedConfig)
		} else {
			shared.EchoWarning("No Jobs found to set label limits while processing custom scrape config")
			return prometheusConfigString
		}
	} else {
		shared.EchoWarning("Nothing to set for label limits while processing custom scrape config")
		return prometheusConfigString
	}
}

func setGlobalScrapeConfigInDefaultFilesIfExists(configString string) string {
	var customConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(configString), &customConfig)
	if err != nil {
		log.Println("Error:", err)
		return ""
	}

	// Set scrape interval to 30s for updating the default merged config
	scrapeInterval := "30s"

	globalConfig, globalExists := customConfig["global"].(map[interface{}]interface{})
	if globalExists {
		scrapeInterval, _ = globalConfig["scrape_interval"].(string)

		// Checking to see if the duration matches the pattern specified in the prometheus config
		// Link to documentation with regex pattern -> https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
		matched := regexp.MustCompile(`^((\d+y)?(\d+w)?(\d+d)?(\d+h)?(\d+m)?(\d+s)?(\d+ms)?|0)$`).MatchString(scrapeInterval)
		if !matched {
			// Set default global scrape interval to 1m if it's not in the proper format
			globalConfig["scrape_interval"] = "1m"
			scrapeInterval = "30s"
		}
	}

	setDefaultFileScrapeInterval(scrapeInterval)

	updatedConfig, err := yaml.Marshal(customConfig)
	if err != nil {
		log.Println("Error:", err)
		return ""
	}

	return string(updatedConfig)
}

func prometheusConfigMerger(operatorEnabled bool) {
	shared.EchoSectionDivider("Start Processing - prometheusConfigMerger")
	prometheusConfigMap := parseConfigMap()

	if len(prometheusConfigMap) > 0 {
		modifiedPrometheusConfigString := setGlobalScrapeConfigInDefaultFilesIfExists(prometheusConfigMap)
		writeDefaultScrapeTargetsFile(operatorEnabled)
		// Set label limits for every custom scrape job, before merging the default & custom config
		labellimitedconfigString := setLabelLimitsPerScrape(modifiedPrometheusConfigString)
		mergeDefaultAndCustomScrapeConfigs(labellimitedconfigString, mergedDefaultConfigs)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Merging Default and Custom Prometheus Config")
	} else {
		setDefaultFileScrapeInterval("30s")
		writeDefaultScrapeTargetsFile(operatorEnabled)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Writing Default Prometheus Config")
	}
}
