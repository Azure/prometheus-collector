package ccpconfigmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

var (
	configMapMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
)

// parseConfigMapForKeepListRegex extracts the control plane metrics keep list from metricsConfigBySection.
func parseConfigMapForKeepListRegex(metricsConfigBySection map[string]map[string]string, schemaVersion string) {
	fmt.Printf("parseConfigMapForKeepListRegex::schemaVersion: %s\n", schemaVersion)

	// Get minimal ingestion profile setting
	sectionName := "default-targets-metrics-keep-list"
	keyName := "minimalingestionprofile"
	if schemaVersion == shared.SchemaVersion.V2 {
		sectionName = "minimal-ingestion-profile"
		keyName = "enabled"
	}

	minimalProfileEnabled := true // Default to true if not found
	if settings, ok := metricsConfigBySection[sectionName]; ok {
		if minimalProfile, ok := settings[keyName]; ok {
			fmt.Printf("parseConfigMapForKeepListRegex::Found %s: %s\n", keyName, minimalProfile)
			minimalProfileEnabled, err := strconv.ParseBool(minimalProfile)
			if err != nil {
				fmt.Printf("parseConfigMapForKeepListRegex::Error parsing minimal ingestion profile: %v\n", err)
			} else {
				fmt.Printf("parseConfigMapForKeepListRegex::Parsed %s as: %t\n", keyName, minimalProfileEnabled)
			}

			if minimalProfileEnabled {
				fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is true or not set, appending minimal metrics")
			} else {
				fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is false, appending values")
			}
		} else {
			fmt.Printf("parseConfigMapForKeepListRegex::%s not found, setting default to true\n", keyName)
		}
	} else {
		fmt.Printf("parseConfigMapForKeepListRegex::%s section not found, setting default to true\n", sectionName)
	}

	// Set keeplist from configmap and with or without minimal ingestion profile
	settings, _ := metricsConfigBySection["default-targets-metrics-keep-list"]
	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		if schemaVersion == shared.SchemaVersion.V1 {
			jobName = "controlplane-" + jobName // Prefix for v1 schema
		}

		if setting, ok := settings[jobName]; ok {
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
			job.CustomerKeepListRegex += "|" + job.KeepListRegex
		} else {
			fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is false, appending values")
		}
	}
}

// tomlparserCCPTargetsMetricsKeepList processes the configuration and writes it to a file.
func tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	if configSchemaVersion == shared.SchemaVersion.V1 || configSchemaVersion == shared.SchemaVersion.V2 {
		fmt.Printf("tomlparserCCPTargetsMetricsKeepList::Processing with schema version: %s\n", configSchemaVersion)
	} else {
		if _, err := os.Stat(configMapMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
		metricsConfigBySection = map[string]map[string]string{}
	}

	parseConfigMapForKeepListRegex(metricsConfigBySection, configSchemaVersion)

	// Write settings to a YAML file.
	data := map[string]string{}
	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		envVarName := fmt.Sprintf("CONTROLPLANE_%s_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		data[envVarName] = job.CustomerKeepListRegex
	}
	fmt.Printf("tomlparserCCPTargetsMetricsKeepList::Final data to write: %+v\n", data)

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	fmt.Println("End default-targets-metrics-keep-list Processing")
}
