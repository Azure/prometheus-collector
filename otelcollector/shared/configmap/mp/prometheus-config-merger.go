package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	configMapMountPath                           = "/etc/config/settings/prometheus/prometheus-config"
	promMergedConfigPath                         = "/opt/promMergedConfig.yml"
	mergedDefaultConfigPath                      = "/opt/defaultsMergedConfig.yml"
	replicasetControllerType                     = "replicaset"
	daemonsetControllerType                      = "daemonset"
	supportedSchemaVersion                       = true
	defaultPromConfigPathPrefix                  = "/opt/microsoft/otelcollector/default-prom-configs/"
	regexHashFile                                = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	sendDSUpMetric                               = false
	intervalHashFile                             = "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash"
	kubeletDefaultFileRsSimple                   = defaultPromConfigPathPrefix + "kubeletDefaultRsSimple.yml"
	kubeletDefaultFileRsAdvanced                 = defaultPromConfigPathPrefix + "kubeletDefaultRsAdvanced.yml"
	kubeletDefaultFileDs                         = defaultPromConfigPathPrefix + "kubeletDefaultDs.yml"
	kubeletDefaultFileRsAdvancedWindowsDaemonset = defaultPromConfigPathPrefix + "kubeletDefaultRsAdvancedWindowsDaemonset.yml"
	coreDNSDefaultFile                           = defaultPromConfigPathPrefix + "corednsDefault.yml"
	cadvisorDefaultFileRsSimple                  = defaultPromConfigPathPrefix + "cadvisorDefaultRsSimple.yml"
	cadvisorDefaultFileRsAdvanced                = defaultPromConfigPathPrefix + "cadvisorDefaultRsAdvanced.yml"
	cadvisorDefaultFileDs                        = defaultPromConfigPathPrefix + "cadvisorDefaultDs.yml"
	kubeProxyDefaultFile                         = defaultPromConfigPathPrefix + "kubeproxyDefault.yml"
	apiserverDefaultFile                         = defaultPromConfigPathPrefix + "apiserverDefault.yml"
	kubeStateDefaultFile                         = defaultPromConfigPathPrefix + "kubestateDefault.yml"
	nodeExporterDefaultFileRsSimple              = defaultPromConfigPathPrefix + "nodeexporterDefaultRsSimple.yml"
	nodeExporterDefaultFileRsAdvanced            = defaultPromConfigPathPrefix + "nodeexporterDefaultRsAdvanced.yml"
	nodeExporterDefaultFileDs                    = defaultPromConfigPathPrefix + "nodeexporterDefaultDs.yml"
	prometheusCollectorHealthDefaultFile         = defaultPromConfigPathPrefix + "prometheusCollectorHealth.yml"
	windowsExporterDefaultRsSimpleFile           = defaultPromConfigPathPrefix + "windowsexporterDefaultRsSimple.yml"
	windowsExporterDefaultDsFile                 = defaultPromConfigPathPrefix + "windowsexporterDefaultDs.yml"
	windowsKubeProxyDefaultFileRsSimpleFile      = defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsSimple.yml"
	windowsKubeProxyDefaultDsFile                = defaultPromConfigPathPrefix + "windowskubeproxyDefaultDs.yml"
	podAnnotationsDefaultFile                    = defaultPromConfigPathPrefix + "podannotationsDefault.yml"
	windowsKubeProxyDefaultRsAdvancedFile        = defaultPromConfigPathPrefix + "windowskubeproxyDefaultRsAdvanced.yml"
	kappieBasicDefaultFileDs                     = defaultPromConfigPathPrefix + "kappieBasicDefaultDs.yml"
	networkObservabilityRetinaDefaultFileDs      = defaultPromConfigPathPrefix + "networkobservabilityRetinaDefaultDs.yml"
	networkObservabilityHubbleDefaultFileDs      = defaultPromConfigPathPrefix + "networkobservabilityHubbleDefaultDs.yml"
	networkObservabilityCiliumDefaultFileDs      = defaultPromConfigPathPrefix + "networkobservabilityCiliumDefaultDs.yml"
)

var (
	regexHash    = make(map[string]string)
	intervalHash = make(map[string]string)
)

var mergedDefaultConfigs map[interface{}]interface{}

func parseConfigMap() string {
	defer func() {
		if r := recover(); r != nil {
			echoError(fmt.Sprintf("Recovered from panic: %v\n", r))
		}
	}()

	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		echoWarning("Custom prometheus config does not exist, using only default scrape targets if they are enabled")
		return ""
	}

	config, err := os.ReadFile(configMapMountPath)
	if err != nil {
		echoError(fmt.Sprintf("Exception while parsing configmap for prometheus config: %s. Custom prometheus config will not be used. Please check configmap for errors", err))
		return ""
	}

	echoVar("Successfully parsed configmap for prometheus config", string(config))
	return string(config)
}

func loadRegexHash() {
	data, err := os.ReadFile(regexHashFile)
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
		return
	}

	err = yaml.Unmarshal(data, &regexHash)
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
	}
}

func loadIntervalHash() {
	data, err := os.ReadFile(intervalHashFile)
	if err != nil {
		fmt.Printf("Exception in loadIntervalHash for prometheus config: %v. Scrape interval will not be used\n", err)
		return
	}

	err = yaml.Unmarshal(data, &intervalHash)
	if err != nil {
		fmt.Printf("Exception in loadIntervalHash for prometheus config: %v. Scrape interval will not be used\n", err)
	}
}

func UpdateScrapeIntervalConfig(yamlConfigFile string, scrapeIntervalSetting string) {
	fmt.Printf("Updating scrape interval config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := ioutil.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("Error unmarshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Update scrape interval config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[string]interface{}); ok {
				if scrapeInterval, exists := scfgMap["scrape_interval"]; exists {
					fmt.Printf("scrapeInterval %s\n", scrapeInterval)
					scfgMap["scrape_interval"] = scrapeIntervalSetting
				}
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithScrapeConfig, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = ioutil.WriteFile(yamlConfigFile, cfgYamlWithScrapeConfig, 0644)
		if err != nil {
			fmt.Printf("Error writing to file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML. The scrape interval will not be updated.\n")
	}
}

func appendMetricRelabelConfig(yamlConfigFile, keepListRegex string) {
	fmt.Printf("Adding keep list regex or minimal ingestion regex for %s\n", yamlConfigFile)

	content, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
		return
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		fmt.Printf("Error unmarshalling YAML for %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
		return
	}

	keepListMetricRelabelConfig := map[string]interface{}{
		"source_labels": []interface{}{"__name__"},
		"action":        "keep",
		"regex":         keepListRegex,
	}

	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[string]interface{}); ok {
				if metricRelabelCfgs, ok := scfgMap["metric_relabel_configs"].([]interface{}); ok {
					scfgMap["metric_relabel_configs"] = append(metricRelabelCfgs, keepListMetricRelabelConfig)
				} else {
					scfgMap["metric_relabel_configs"] = []interface{}{keepListMetricRelabelConfig}
				}
			}
		}

		if cfgYamlWithMetricRelabelConfig, err := yaml.Marshal(config); err == nil {
			if err := os.WriteFile(yamlConfigFile, []byte(cfgYamlWithMetricRelabelConfig), fs.FileMode(0644)); err != nil {
				fmt.Printf("Error writing to file %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
			}
		} else {
			fmt.Printf("Error marshalling YAML for %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML. The keep list regex will not be used.\n")
	}
}

func AppendRelabelConfig(yamlConfigFile string, relabelConfig []interface{}, keepRegex string) {
	fmt.Printf("Adding relabel config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := ioutil.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("Error unmarshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Append relabel config for keep list to each scrape config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[string]interface{}); ok {
				relabelCfgs, exists := scfgMap["relabel_configs"]
				if !exists {
					scfgMap["relabel_configs"] = relabelConfig
				} else if relabelCfgsSlice, ok := relabelCfgs.([]interface{}); ok {
					scfgMap["relabel_configs"] = append(relabelCfgsSlice, relabelConfig...)
				}
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithRelabelConfig, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = ioutil.WriteFile(yamlConfigFile, cfgYamlWithRelabelConfig, 0644)
		if err != nil {
			fmt.Printf("Error writing to file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML. The relabel config will not be added.\n")
	}
}

func populateDefaultPrometheusConfig() {
	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(strings.ToLower(os.Getenv("CONTROLLER_TYPE")))

	// Default values
	advancedMode := false
	windowsDaemonset := false

	// Get current mode (advanced or not...)
	currentMode := strings.TrimSpace(strings.ToLower(os.Getenv("MODE")))
	if currentMode == "advanced" {
		advancedMode = true
	}

	// Get if windowsdaemonset is enabled or not (i.e., WINMODE env = advanced or not...)
	winMode := strings.TrimSpace(strings.ToLower(os.Getenv("WINMODE")))
	if winMode == "advanced" {
		windowsDaemonset = true
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		fmt.Println("Kube Controller Manager enabled.")
		kubeControllerManagerMetricsKeepListRegex, exists := regexHash["CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX"]
		if exists && kubeControllerManagerMetricsKeepListRegex != "" {
			fmt.Printf("Using regex for Kube Controller Manager: %s\n", kubeControllerManagerMetricsKeepListRegex)
			appendMetricRelabelConfig(controlplaneKubeControllerManagerFile, kubeControllerManagerMetricsKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneKubeControllerManagerFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneKubeControllerManagerFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneKubeControllerManagerFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneKubeSchedulerKeepListRegex, exists := regexHash["CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX"]
		if exists && controlplaneKubeSchedulerKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneKubeSchedulerDefaultFile, controlplaneKubeSchedulerKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneKubeSchedulerDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneKubeSchedulerDefaultFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneKubeSchedulerDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneApiserverKeepListRegex, exists := regexHash["CONTROLPLANE_APISERVER_KEEP_LIST_REGEX"]
		if exists && controlplaneApiserverKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneApiserverDefaultFile, controlplaneApiserverKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneApiserverDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneApiserverDefaultFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneApiserverDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneClusterAutoscalerKeepListRegex, exists := regexHash["CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX"]
		if exists && controlplaneClusterAutoscalerKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneClusterAutoscalerFile, controlplaneClusterAutoscalerKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneClusterAutoscalerFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneClusterAutoscalerFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneClusterAutoscalerFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneEtcdKeepListRegex, exists := regexHash["CONTROLPLANE_ETCD_KEEP_LIST_REGEX"]
		if exists && controlplaneEtcdKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneEtcdDefaultFile, controlplaneEtcdKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneEtcdDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneEtcdDefaultFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneEtcdDefaultFile)
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
	if mergedDefaultConfigs != nil {
		fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	}
}

func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	mergedDefaultConfigs := make(map[interface{}]interface{})

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

	fmt.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))

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

func writeDefaultScrapeTargetsFile() {
	fmt.Printf("Start Updating Default Prometheus Config\n")
	noDefaultScrapingEnabled := os.Getenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED")
	if noDefaultScrapingEnabled != "" && strings.ToLower(noDefaultScrapingEnabled) == "false" {
		loadRegexHash()
		populateDefaultPrometheusConfig()
		if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
			fmt.Printf("Starting to merge default prometheus config values in collector template as backup\n")
			mergedDefaultConfigYaml, err := yaml.Marshal(mergedDefaultConfigs)
			if err == nil {
				err = os.WriteFile(mergedDefaultConfigPath, []byte(mergedDefaultConfigYaml), fs.FileMode(0644))
				if err != nil {
					fmt.Printf("Error writing merged default prometheus config to file: %v\n", err)
				}
			} else {
				fmt.Printf("Error marshalling merged default prometheus config: %v\n", err)
			}
		}
	} else {
		mergedDefaultConfigs = nil
	}
	fmt.Printf("Done creating default targets file\n")
}

func setDefaultFileScrapeInterval(scrapeInterval string) {
	defaultFilesArray := []string{
		controlplaneApiserverDefaultFile, controlplaneKubeSchedulerDefaultFile, controlplaneKubeControllerManagerFile,
		controlplaneClusterAutoscalerFile, controlplaneEtcdDefaultFile,
	}

	for _, currentFile := range defaultFilesArray {
		contents, err := os.ReadFile(currentFile)
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

func mergeDefaultAndCustomScrapeConfigs(customPromConfig string, mergedDefaultConfigs map[interface{}]interface{}) {
	var mergedConfigYaml string

	if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
		echoVar("Merging default and custom scrape configs", "")
		var customPrometheusConfig map[interface{}]interface{}
		err := yaml.Unmarshal([]byte(customPromConfig), &customPrometheusConfig)
		if err != nil {
			echoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
			return
		}

		mergedConfigs := deepMerge(mergedDefaultConfigs, customPrometheusConfig)
		mergedConfigYaml, err = yaml.Marshal(mergedConfigs)
		if err != nil {
			echoError(fmt.Sprintf("Error marshalling merged configs: %v", err))
			return
		}

		echoVar("Done merging default scrape config(s) with custom prometheus config, writing them to file", "")
	} else {
		echoWarning("The merged default scrape config is nil or empty, using only custom scrape config")
		mergedConfigYaml = customPromConfig
	}

	err := os.WriteFile(promMergedConfigPath, []byte(mergedConfigYaml), fs.FileMode(0644))
	if err != nil {
		echoError(fmt.Sprintf("Error writing merged config to file: %v", err))
		return
	}
}

func setLabelLimitsPerScrape(prometheusConfigString string) string {
	customConfig := prometheusConfigString
	echoVar("setLabelLimitsPerScrape()", "")

	var limitedCustomConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(customConfig), &limitedCustomConfig)
	if err != nil {
		echoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
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
				echoVar(fmt.Sprintf("Successfully set label limits in custom scrape config for job %s", scrapeMap["job_name"]), "")
			}
			echoWarning("Done setting label limits for custom scrape config ...")
			updatedConfig, err := yaml.Marshal(limitedCustomConfig)
			if err != nil {
				echoError(fmt.Sprintf("Error marshalling custom config: %v", err))
				return prometheusConfigString
			}
			return string(updatedConfig)
		} else {
			echoWarning("No Jobs found to set label limits while processing custom scrape config")
			return prometheusConfigString
		}
	} else {
		echoWarning("Nothing to set for label limits while processing custom scrape config")
		return prometheusConfigString
	}
}

func setGlobalScrapeConfigInDefaultFilesIfExists(configString string) string {
	var customConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(configString), &customConfig)
	if err != nil {
		fmt.Println("Error:", err)
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
		fmt.Println("Error:", err)
		return ""
	}

	return string(updatedConfig)
}

func prometheusConfigMerger() {
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	setDefaultFileScrapeInterval("30s")
	writeDefaultScrapeTargetsFile()
	fmt.Printf("Done creating default targets file\n")

	prometheusConfigMap := parseConfigMap()

	if prometheusConfigMap != nil && len(prometheusConfigMap) > 0 {
		modifiedPrometheusConfigString := setGlobalScrapeConfigInDefaultFilesIfExists(prometheusConfigMap)
		writeDefaultScrapeTargetsFile()
		// Set label limits for every custom scrape job, before merging the default & custom config
		labellimitedconfigString := setLabelLimitsPerScrape(modifiedPrometheusConfigString)
		mergeDefaultAndCustomScrapeConfigs(labellimitedconfigString)
	} else {
		setDefaultFileScrapeInterval("30s")
		writeDefaultScrapeTargetsFile()
	}

	fmt.Println("Done Merging Default and Custom Prometheus Config")
}
