package ccpconfigmapsettings

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
)

var mergedDefaultConfigs map[interface{}]interface{}

func appendMetricRelabelConfig(yamlConfigFile, keepListRegex string) {
	if err := cmcommon.AppendMetricRelabelConfig(yamlConfigFile, keepListRegex, log.Printf); err != nil {
		log.Printf("Error updating metric relabel config for %s: %v\n", yamlConfigFile, err)
	}
}

func populateDefaultPrometheusConfig() {

	defaultConfigs := []string{}
	currentControllerType := os.Getenv("CONTROLLER_TYPE")

	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		if job.Enabled && job.ControllerType == currentControllerType {
			fmt.Printf("%s job enabled\n", jobName)

			if job.CustomerKeepListRegex != "" {
				fmt.Printf("Using regex for %s: %s\n", jobName, job.CustomerKeepListRegex)
				appendMetricRelabelConfig(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, job.CustomerKeepListRegex)
			}

			if err := cmcommon.ReplacePlaceholders(scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile, job.PlaceholderNames); err != nil {
				log.Printf("Error replacing placeholders for %s: %v\n", jobName, err)
			}
			defaultConfigs = append(defaultConfigs, scrapeConfigDefinitionPathPrefix+job.ScrapeConfigDefinitionFile)
		}
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
}

func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	merged := make(map[interface{}]interface{})

	if len(defaultScrapeConfigs) > 0 {
		merged["scrape_configs"] = make([]interface{}, 0)

		for _, defaultScrapeConfig := range defaultScrapeConfigs {
			defaultConfigYaml, err := cmcommon.LoadYAMLFromFile(defaultScrapeConfig)
			if err != nil {
				log.Printf("Error loading YAML from file %s: %s\n", defaultScrapeConfig, err)
				continue
			}

			merged = cmcommon.DeepMerge(merged, defaultConfigYaml)
		}
	}

	fmt.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))

	return merged
}

func writeDefaultScrapeTargetsFile() {
	fmt.Printf("Start Updating Default Prometheus Config\n")
	noDefaultScrapingEnabled := os.Getenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED")
	if noDefaultScrapingEnabled != "" && strings.ToLower(noDefaultScrapingEnabled) == "false" {
		populateDefaultPrometheusConfig()
		if len(mergedDefaultConfigs) > 0 {
			fmt.Printf("Starting to merge default prometheus config values in collector template as backup\n")
			if err := cmcommon.WriteYAML(mergedDefaultConfigPath, mergedDefaultConfigs); err != nil {
				fmt.Printf("Error writing merged default prometheus config to file: %v\n", err)
			}
		}
	} else {
		mergedDefaultConfigs = make(map[interface{}]interface{})
	}
}

func setDefaultFileScrapeInterval(scrapeInterval string) {
	for _, job := range shared.ControlPlaneDefaultScrapeJobs {
		currentFile := scrapeConfigDefinitionPathPrefix + job.ScrapeConfigDefinitionFile
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
