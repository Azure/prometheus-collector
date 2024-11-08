package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"io/fs"

	"github.com/pelletier/go-toml"
	"github.com/prometheus-collector/shared"
	scrapeConfigs "github.com/prometheus-collector/defaultscrapeconfigs"
	"gopkg.in/yaml.v2"
)

const (
	defaultScrapeInterval = "30s"
)

var (
	MATCHER = regexp.MustCompile(`^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$`)
)

func parseConfigMapForScrapeSettings() *toml.Tree {
	config, err := toml.LoadFile(configMapScrapeIntervalMountPath)
	if err != nil {
		fmt.Printf("Error parsing config map: %v\n", err)
		return nil
	}
	return config
}

func checkDuration(duration string) string {
	if !MATCHER.MatchString(duration) || duration == "" {
		return defaultScrapeInterval
	}
	return duration
}

func getConfigStringValue(configMapSettings *toml.Tree, key string) string {
	if configMapSettings == nil {
		return ""
	}
	valueInterface := configMapSettings.Get(key)
	if valueInterface == nil {
		return ""
	}
	value, ok := valueInterface.(string)
	if !ok {
		return ""
	}
	return value
}

func processConfigMap() map[string]string {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	intervalHash := make(map[string]string)
	var configMapSettings *toml.Tree

	err := os.Stat(configMapScrapeIntervalMountPath)
	if os.IsNotExist(err)
		fmt.Printf("configmap prometheus-collector-configmap for default-targets-scrape-interval-settings not mounted, using defaults")
	} else if err == nil && configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings = parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
		} else {
			fmt.Printf("Error parsing config map, scrape interval settings is empty. Using default scrape interval settings\n")
		}
	}

	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		if configMapSettings != nil {
			interval := checkDuration(getConfigStringValue(configMapSettings, jobName))
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

	err = os.WriteFile(filePath, []byte(out), fs.FileMode(0644))
	if err != nil {
		return err
	}
	return nil
}

func tomlparserScrapeInterval() {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	intervalHash := processConfigMap()
	err := writeIntervalHashToFile(intervalHash, scrapeIntervalEnvVarPath)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
