package ccpconfigmapsettings

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
	controlplaneNodeAutoProvisioningFile  = defaultPromConfigPathPrefix + "controlplane_node_auto_provisioning.yml"
	controlplaneEtcdDefaultFile           = defaultPromConfigPathPrefix + "controlplane_etcd.yml"
)

var (
	regexHash    = make(map[string]string)
	intervalHash = make(map[string]string)
)

var mergedDefaultConfigs map[interface{}]interface{}

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
		for i, scfg := range scrapeConfigs {
			// Ensure scfg is a map with string keys
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				stringScfgMap := make(map[string]interface{})
				for k, v := range scfgMap {
					if key, ok := k.(string); ok {
						stringScfgMap[key] = v
					} else {
						fmt.Printf("Encountered non-string key in scrape config map: %v\n", k)
						return
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

		// Marshal the updated config to YAML
		cfgYamlWithMetricRelabelConfig, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML for %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
			return
		}

		// Write the updated YAML back to the file
		if err := os.WriteFile(yamlConfigFile, cfgYamlWithMetricRelabelConfig, fs.FileMode(0644)); err != nil {
			fmt.Printf("Error writing to file %s: %v. The keep list regex will not be used\n", yamlConfigFile, err)
			return
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

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_NODE_AUTO_PROVISIONING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneNodeAutoProvisioningKeepListRegex, exists := regexHash["CONTROLPLANE_NODE_AUTO_PROVISIONING_KEEP_LIST_REGEX"]
		if exists && controlplaneNodeAutoProvisioningKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneNodeAutoProvisioningFile, controlplaneNodeAutoProvisioningKeepListRegex)
		}
		contents, err := os.ReadFile(controlplaneNodeAutoProvisioningFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = os.WriteFile(controlplaneNodeAutoProvisioningFile, contents, fs.FileMode(0644))
		}
		defaultConfigs = append(defaultConfigs, controlplaneNodeAutoProvisioningFile)
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
	// if mergedDefaultConfigs != nil {
	// 	fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	// }
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
		controlplaneClusterAutoscalerFile, controlplaneNodeAutoProvisioningFile, controlplaneEtcdDefaultFile,
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

func prometheusCcpConfigMerger() {
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	setDefaultFileScrapeInterval("30s")
	writeDefaultScrapeTargetsFile()
	fmt.Printf("Done creating default targets file\n")
}
