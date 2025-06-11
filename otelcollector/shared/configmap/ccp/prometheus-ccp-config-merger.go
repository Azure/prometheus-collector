package ccpconfigmapsettings

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/prometheus-collector/shared"

	"gopkg.in/yaml.v2"
)

const (
	mergedDefaultConfigPath     = "/opt/defaultsMergedConfig.yml"
	replicasetControllerType    = "replicaset"
	defaultPromConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
)

var mergedDefaultConfigs map[interface{}]interface{}

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

	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(strings.ToLower(os.Getenv("CONTROLLER_TYPE")))

	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		if job.Enabled && job.ControllerType == currentControllerType {
			fmt.Printf("%s job enabled\n", jobName)

			if job.CustomerKeepListRegex != "" {
				fmt.Printf("Using regex for %s: %s\n", jobName, job.CustomerKeepListRegex)
				appendMetricRelabelConfig(job.ScrapeConfigDefinitionFile, job.CustomerKeepListRegex)
			}

			contents, err := os.ReadFile(job.ScrapeConfigDefinitionFile)
			if err == nil {
				for _, envVarName := range job.PlaceholderNames {
					contents = []byte(strings.Replace(string(contents), fmt.Sprintf("$$%s$$", envVarName), os.Getenv(envVarName), -1))
					os.WriteFile(job.ScrapeConfigDefinitionFile, contents, fs.FileMode(0644))
				}
			}
			defaultConfigs = append(defaultConfigs, job.ScrapeConfigDefinitionFile)
		}
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
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

			mergedDefaultConfigs = DeepMerge(mergedDefaultConfigs, defaultConfigYaml)
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

func DeepMerge(target, source map[interface{}]interface{}) map[interface{}]interface{} {
	for key, sourceValue := range source {
		targetValue, exists := target[key]

		if !exists {
			target[key] = sourceValue
			continue
		}

		targetMap, targetMapOk := targetValue.(map[interface{}]interface{})
		sourceMap, sourceMapOk := sourceValue.(map[interface{}]interface{})

		if targetMapOk && sourceMapOk {
			target[key] = DeepMerge(targetMap, sourceMap)
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
	for _, job := range shared.ControlPlaneDefaultScrapeJobs {
		currentFile := job.ScrapeConfigDefinitionFile
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
