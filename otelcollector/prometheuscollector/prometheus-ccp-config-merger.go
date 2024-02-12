package main

import (
	"fmt"
	"io/ioutil"
	"os"
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

var (
	regexHash    = make(map[string]string)
	intervalHash = make(map[string]string)
)

var mergedDefaultConfigs map[interface{}]interface{}

func loadRegexHash() {
	data, err := ioutil.ReadFile(regexHashFile)
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

	content, err := ioutil.ReadFile(yamlConfigFile)
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
			if err := ioutil.WriteFile(yamlConfigFile, cfgYamlWithMetricRelabelConfig, 0644); err != nil {
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
		kubeControllerManagerMetricsKeepListRegex, exists := regexHash["CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX"]
		if exists && kubeControllerManagerMetricsKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneKubeControllerManagerFile, kubeControllerManagerMetricsKeepListRegex)
		}
		contents, err := ioutil.ReadFile(controlplaneKubeControllerManagerFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = ioutil.WriteFile(controlplaneKubeControllerManagerFile, contents, 0644)
		}
		defaultConfigs = append(defaultConfigs, controlplaneKubeControllerManagerFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneKubeSchedulerKeepListRegex, exists := regexHash["CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX"]
		if exists && controlplaneKubeSchedulerKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneKubeSchedulerDefaultFile, controlplaneKubeSchedulerKeepListRegex)
		}
		contents, err := ioutil.ReadFile(controlplaneKubeSchedulerDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = ioutil.WriteFile(controlplaneKubeSchedulerDefaultFile, contents, 0644)
		}
		defaultConfigs = append(defaultConfigs, controlplaneKubeSchedulerDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneApiserverKeepListRegex, exists := regexHash["CONTROLPLANE_APISERVER_KEEP_LIST_REGEX"]
		if exists && controlplaneApiserverKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneApiserverDefaultFile, controlplaneApiserverKeepListRegex)
		}
		contents, err := ioutil.ReadFile(controlplaneApiserverDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = ioutil.WriteFile(controlplaneApiserverDefaultFile, contents, 0644)
		}
		defaultConfigs = append(defaultConfigs, controlplaneApiserverDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneClusterAutoscalerKeepListRegex, exists := regexHash["CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX"]
		if exists && controlplaneClusterAutoscalerKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneClusterAutoscalerFile, controlplaneClusterAutoscalerKeepListRegex)
		}
		contents, err := ioutil.ReadFile(controlplaneClusterAutoscalerFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = ioutil.WriteFile(controlplaneClusterAutoscalerFile, contents, 0644)
		}
		defaultConfigs = append(defaultConfigs, controlplaneClusterAutoscalerFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		controlplaneEtcdKeepListRegex, exists := regexHash["CONTROLPLANE_ETCD_KEEP_LIST_REGEX"]
		if exists && controlplaneEtcdKeepListRegex != "" {
			appendMetricRelabelConfig(controlplaneEtcdDefaultFile, controlplaneEtcdKeepListRegex)
		}
		contents, err := ioutil.ReadFile(controlplaneEtcdDefaultFile)
		if err == nil {
			contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
			err = ioutil.WriteFile(controlplaneEtcdDefaultFile, contents, 0644)
		}
		defaultConfigs = append(defaultConfigs, controlplaneEtcdDefaultFile)
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
	if mergedDefaultConfigs != nil {
		fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	}
}

func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	for _, defaultScrapeConfig := range defaultScrapeConfigs {
		data, err := ioutil.ReadFile(defaultScrapeConfig)
		if err != nil {
			fmt.Printf("Error reading default scrape config %s: %v. No default scrape targets will be included\n", defaultScrapeConfig, err)
			return nil
		}

		config := make(map[interface{}]interface{})
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			fmt.Printf("Error unmarshalling YAML for default scrape config %s: %v. No default scrape targets will be included\n", defaultScrapeConfig, err)
			return nil
		}

		fmt.Printf("Merging config:  %v\n", config)
		mergedDefaultConfigs = mergeYAML(mergedDefaultConfigs, config)
	}
	fmt.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))
	return mergedDefaultConfigs
}

func mergeYAML(dest, src map[interface{}]interface{}) map[interface{}]interface{} {
	for key, srcVal := range src {
		destVal, exists := dest[key]
		if exists {
			srcMap, isMap := srcVal.(map[interface{}]interface{})
			destMap, isDestMap := destVal.(map[interface{}]interface{})
			if isMap && isDestMap {
				dest[key] = mergeYAML(destMap, srcMap)
			} else {
				dest[key] = srcVal
			}
		} else {
			dest[key] = srcVal
		}
	}
	return dest
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
				err = ioutil.WriteFile(mergedDefaultConfigPath, mergedDefaultConfigYaml, 0644)
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
		contents, err := ioutil.ReadFile(currentFile)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", currentFile, err)
			continue
		}

		contents = []byte(strings.Replace(string(contents), "$$SCRAPE_INTERVAL$$", scrapeInterval, -1))

		err = ioutil.WriteFile(currentFile, contents, 0644)
		if err != nil {
			fmt.Printf("Error writing to file %s: %v\n", currentFile, err)
		}
	}
}

func prometheusCcpConfigMerger() {
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	setDefaultFileScrapeInterval("30s")
	writeDefaultScrapeTargetsFile()
}
