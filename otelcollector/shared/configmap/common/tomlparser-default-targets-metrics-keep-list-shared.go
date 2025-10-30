package common

import (
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

// KeepListOptions describes how to interpret keep list data for a given variant.
type KeepListOptions struct {
	Jobs                      map[string]*shared.DefaultScrapeJob
	MetricsConfig             map[string]map[string]string
	SchemaVersion             string
	KeepListSection           string
	MinimalProfileSection     string
	MinimalProfileKeyV1       string
	MinimalProfileKeyV2       string
	OutputPath                string
	EnvVarFormatter           EnvVarFormatter
	JobKeyFormatter           JobKeyFormatter
	MinimalProfileDefaultBool bool
}

// MinimalProfileSetting extracts the value that indicates whether the minimal
// ingestion profile should be enforced. The config map layout differs between
// schema versions, so the caller provides both the section and key to read. If
// the value cannot be parsed, defaultValue is returned.
func MinimalProfileSetting(metricsConfig map[string]map[string]string, sectionName, keyName string, defaultValue bool) bool {
	section, ok := metricsConfig[sectionName]
	if !ok {
		return defaultValue
	}

	value, ok := section[strings.ToLower(keyName)]
	if !ok {
		value, ok = section[keyName]
		if !ok {
			return defaultValue
		}
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

// PopulateKeepLists updates the job catalog with customer-provided and minimal
// keep list expressions and writes the resulting values to a YAML file that can
// be consumed by downstream components.
func PopulateKeepLists(opts KeepListOptions) error {
	keeplist := opts.MetricsConfig[opts.KeepListSection]
	minimalProfileEnabled := opts.MinimalProfileDefaultBool

	switch shared.ParseSchemaVersion(opts.SchemaVersion) {
	case shared.SchemaVersion.V1:
		minimalProfileEnabled = MinimalProfileSetting(opts.MetricsConfig, opts.KeepListSection, opts.MinimalProfileKeyV1, opts.MinimalProfileDefaultBool)
	case shared.SchemaVersion.V2:
		minimalProfileEnabled = MinimalProfileSetting(opts.MetricsConfig, opts.MinimalProfileSection, opts.MinimalProfileKeyV2, opts.MinimalProfileDefaultBool)
	default:
		// leave the default value when schema is unknown
	}

	for jobName, job := range opts.Jobs {
		job.CustomerKeepListRegex = ""
		job.KeepListRegex = ""

		key := opts.JobKeyFormatter(jobName, opts.SchemaVersion)
		if keeplist != nil {
			if value, ok := keeplist[key]; ok {
				if !shared.IsValidRegex(value) {
					continue
				}
				job.CustomerKeepListRegex = value
			}
		}

		regexParts := []string{job.CustomerKeepListRegex}
		if minimalProfileEnabled {
			regexParts = append(regexParts, job.MinimalKeepListRegex)
		}
		job.KeepListRegex = strings.Join(regexParts, "|")
	}

	data := map[string]string{}
	for jobName, job := range opts.Jobs {
		envName := opts.EnvVarFormatter(jobName)
		if envName == "" {
			continue
		}
		data[envName] = job.KeepListRegex
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("common.PopulateKeepLists::marshal failed: %w", err)
	}

	if err := os.WriteFile(opts.OutputPath, out, fs.FileMode(0644)); err != nil {
		return fmt.Errorf("common.PopulateKeepLists::write file failed: %w", err)
	}

	return nil
}
