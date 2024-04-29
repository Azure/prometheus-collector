package shared

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
)

const (
	mergedDefaultConfigPath               = "/opt/defaultsMergedConfig.yml"
	replicasetControllerType              = "replicaset"
	defaultPromConfigPathPrefix           = "/opt/microsoft/otelcollector/default-prom-configs/"
	regexHashFile                         = "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash"
	controlplaneApiserverDefaultFile      = defaultPromConfigPathPrefix + "controlplane_apiserver.yml"
	controlplaneKubeSchedulerDefaultFile  = defaultPromConfigPathPrefix + "controlplane_kube_scheduler.yml"
	controlplaneKubeControllerManagerFile = defaultPromConfigPathPrefix + "controlplane_kube_controller_manager.yml"
	controlplaneClusterAutoscalerFile     = defaultPromConfigPathPrefix + "controlplane_cluster_autoscaler.yml"
	controlplaneEtcdDefaultFile           = defaultPromConfigPathPrefix + "controlplane_etcd.yml"
)

var defaultFilesArray = []string{
	controlplaneApiserverDefaultFile, controlplaneKubeSchedulerDefaultFile, controlplaneKubeControllerManagerFile,
	controlplaneClusterAutoscalerFile, controlplaneEtcdDefaultFile,
}

var (
	regexHash    = make(map[string]string)
	intervalHash = make(map[string]string)
)

var mergedDefaultConfigs map[interface{}]interface{}

func loadRegexHash() {
	data, err := os.ReadFile(getPath(regexHashFile))
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
		return
	}

	err = yaml.Unmarshal(data, &regexHash)
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
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

func populateDefaultPrometheusConfig() {
	loadRegexHash()

	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(strings.ToLower(os.Getenv("CONTROLLER_TYPE")))

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		fmt.Println("Kube Controller Manager enabled.")
		defaultConfigs = appendDefaultConfig(controlplaneKubeControllerManagerFile, "CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX", &defaultConfigs)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		defaultConfigs = appendDefaultConfig(controlplaneKubeSchedulerDefaultFile, "CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX", &defaultConfigs)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		defaultConfigs = appendDefaultConfig(controlplaneApiserverDefaultFile, "CONTROLPLANE_APISERVER_KEEP_LIST_REGEX", &defaultConfigs)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		defaultConfigs = appendDefaultConfig(controlplaneClusterAutoscalerFile, "CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX", &defaultConfigs)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		defaultConfigs = appendDefaultConfig(controlplaneEtcdDefaultFile, "CONTROLPLANE_ETCD_KEEP_LIST_REGEX", &defaultConfigs)
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
	if mergedDefaultConfigs != nil {
		fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	}
}

func appendDefaultConfig(file, keepListName string, defaultConfigs *[]string) []string {
	defaultConfigFile := getPath(file)
	keepListRegex, exists := regexHash[keepListName]
	if exists && keepListRegex != "" {
		appendMetricRelabelConfig(defaultConfigFile, keepListRegex)
	}
	contents, err := os.ReadFile(defaultConfigFile)
	if err == nil {
		contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
		err = os.WriteFile(defaultConfigFile, contents, fs.FileMode(0644))
	}
	return append(*defaultConfigs, defaultConfigFile)
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
	fileContent, err := os.ReadFile(getPath(filename))
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
				err = os.WriteFile(getPath(mergedDefaultConfigPath), []byte(mergedDefaultConfigYaml), fs.FileMode(0644))
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
	for _, configFile := range defaultFilesArray {
		currentFile := getPath(configFile)
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

func prometheusCcpConfigMerger() {
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	setDefaultFileScrapeInterval("30s")
	writeDefaultScrapeTargetsFile()
	fmt.Printf("Done creating default targets file\n")
}
