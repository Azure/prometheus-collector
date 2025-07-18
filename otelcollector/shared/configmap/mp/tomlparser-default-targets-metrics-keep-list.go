package configmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

// populateKeepList initializes the regex keep list with values from metricsConfigBySection.
func populateKeepList(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) error {
	keeplist := metricsConfigBySection["default-targets-metrics-keep-list"]

	minimalProfileEnabled := true
	switch configSchemaVersion {
	case shared.SchemaVersion.V1:
		minimalProfileEnabledBool, err := strconv.ParseBool(keeplist["minimalingestionprofile"])
		if err != nil {
			fmt.Println("Invalid value for minimalingestionprofile in v1:", err.Error())
			metricsConfigBySection = map[string]map[string]string{}
		} else {
			minimalProfileEnabled = minimalProfileEnabledBool
			fmt.Println("populateKeepList::Minimal ingestion profile enabled:", minimalProfileEnabled)
		}
	case shared.SchemaVersion.V2:
		minimalProfileEnabledBool, err := strconv.ParseBool(metricsConfigBySection["minimal-ingestion-profile"]["enabled"])
		if err != nil {
			fmt.Printf("Invalid value for minimal-ingestion-profile in v2: %s", metricsConfigBySection["minimal-ingestion-profile"]["enabled"])
			metricsConfigBySection = map[string]map[string]string{}
		} else {
			minimalProfileEnabled = minimalProfileEnabledBool
			fmt.Println("populateKeepList::Minimal ingestion profile enabled:", minimalProfileEnabled)
		}
	default:
		fmt.Printf("Unsupported/missing config schema version - '%s', using defaults\n", configSchemaVersion)
		metricsConfigBySection = map[string]map[string]string{}
	}

	for jobName, job := range shared.DefaultScrapeJobs {
		job.KeepListRegex = ""
		if setting, ok := keeplist[jobName]; ok {
			fmt.Printf("parseConfigMapForKeepListRegex::Adding key: %s, value: %s\n", jobName, setting)
			if !shared.IsValidRegex(setting) {
				fmt.Printf("parseConfigMapForKeepListRegex::Invalid regex for job %s: %s\n", jobName, setting)
				continue // Skip invalid regex
			}
			job.CustomerKeepListRegex = setting
			fmt.Printf("populateSettingValuesFromConfigMap::%s: %s\n", jobName, job.CustomerKeepListRegex)
		}

		if minimalProfileEnabled {
			fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is true or not set, appending minimal metrics")
			job.KeepListRegex = "|" + job.MinimalKeepListRegex
			fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal Keep List Regex:", job.MinimalKeepListRegex)
		} else {
			fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is false, using configmap values")
		}

		job.KeepListRegex = job.CustomerKeepListRegex + job.KeepListRegex
	}

	fmt.Printf("Parsed config map for default-targets-metrics-keep-list successfully\n")
	return nil
}

func tomlparserTargetsMetricsKeepList(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	shared.EchoSectionDivider("Start Processing - tomlparserTargetsMetricsKeepList")

	err := populateKeepList(metricsConfigBySection, configSchemaVersion)
	if err != nil {
		fmt.Printf("Error populating keep list: %s\n", err.Error())
	}
	// Write settings to a YAML file
	data := map[string]string{}
	for jobName, job := range shared.DefaultScrapeJobs {
		data[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = job.KeepListRegex
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile(configMapKeepListEnvVarPath, []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	shared.EchoSectionDivider("End Processing - tomlparserTargetsMetricsKeepList")
}
