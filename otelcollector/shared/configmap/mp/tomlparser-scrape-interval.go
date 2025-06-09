package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml"
	scrapeConfigs "github.com/prometheus-collector/defaultscrapeconfigs"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

const (
	defaultScrapeInterval = "30s"
)

var (
	durationMatcher = regexp.MustCompile(`^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$`)
)

func loadConfigMapSettings() *toml.Tree {
	config, err := toml.LoadFile(configMapScrapeIntervalMountPath)
	if err != nil {
		fmt.Printf("Error parsing config map: %v\n", err)
		return nil
	}
	return config
}

func validateDuration(duration string) string {
	if duration == "" || !durationMatcher.MatchString(duration) {
		return defaultScrapeInterval
	}
	return duration
}

func getParsedDataValue(metricsConfigBySection map[string]map[string]string, key string) string {
	if value, exists := metricsConfigBySection["default-targets-scrape-interval-settings"][key]; exists {
		return checkDuration(value)
	}
	return defaultScrapeInterval
}

func processConfigMap(metricsConfigBySection map[string]map[string]string) map[string]string {
	intervalHash := make(map[string]string)
	var config *toml.Tree

	// Check config map existence and schema version
	if _, err := os.Stat(configMapScrapeIntervalMountPath); os.IsNotExist(err) {
		fmt.Printf("Config map not mounted, using defaults\n")
	} else if err == nil && strings.TrimSpace(os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")) == "v1" {
		config = loadConfigMapSettings()
		if config == nil {
			fmt.Printf("Error parsing config map. Using default settings\n")
		}
	}

	// Process all default jobs
	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		if config != nil {
			interval := validateDuration(getStringValue(config, jobName))
			if interval != "" {
				job.ScrapeInterval = interval
				scrapeConfigs.DefaultScrapeJobs[jobName] = job
			}
		}
		intervalHash[fmt.Sprintf("%s_SCRAPE_INTERVAL", strings.ToUpper(jobName))] = job.ScrapeInterval
	}

	return intervalHash
}

func writeIntervalHashToFile(intervalHash map[string]string, filePath string) error {
	out, err := yaml.Marshal(intervalHash)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, out, 0644)
}

func tomlparserScrapeInterval(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	intervalHash := processConfigMap(metricsConfigBySection)
	err := writeIntervalHashToFile(intervalHash, scrapeIntervalEnvVarPath)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
	}

	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
