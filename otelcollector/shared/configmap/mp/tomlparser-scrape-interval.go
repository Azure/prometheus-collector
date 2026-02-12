package configmapsettings

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

const (
	defaultScrapeInterval = "30s"
)

var (
	MATCHER = regexp.MustCompile(`^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$`)
)

func checkDuration(duration string) string {
	if !MATCHER.MatchString(duration) || duration == "" {
		return defaultScrapeInterval
	}
	return duration
}

func processConfigMap(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {

	if configSchemaVersion == shared.SchemaVersion.Nil {
		log.Printf("Setting default scrape interval (%s) for all jobs as no config map is present \n", defaultScrapeInterval)
		metricsConfigBySection = map[string]map[string]string{}
	}

	// Process all default jobs
	for jobName, job := range shared.DefaultScrapeJobs {
		if value, exists := metricsConfigBySection["default-targets-scrape-interval-settings"][jobName]; exists {
			interval := checkDuration(value)
			if interval != "" {
				job.ScrapeInterval = interval
			}
		}
	}
}

func writeIntervalHashToFile(intervalHash map[string]string, filePath string) error {
	out, err := yaml.Marshal(intervalHash)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, out, 0644)
}

func tomlparserScrapeInterval(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	processConfigMap(metricsConfigBySection, configSchemaVersion)

	data := map[string]string{}
	for jobName, job := range shared.DefaultScrapeJobs {
		data[fmt.Sprintf("%s_SCRAPE_INTERVAL", strings.ToUpper(jobName))] = job.ScrapeInterval
	}
	err := writeIntervalHashToFile(data, scrapeIntervalEnvVarPath)
	if err != nil {
		log.Printf("Error writing to file: %v\n", err)
	}
	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
