package common

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
)

// JobKeyFormatter is used to translate a job name into the key expected in the
// customer-provided config map based on the schema version.
type JobKeyFormatter func(jobName string, schemaVersion string) string

// EnvVarFormatter generates the environment variable name that should be
// written for a given job.
type EnvVarFormatter func(jobName string) string

// UpdateJobEnablement reads enablement flags from the provided settings map and
// applies them to the supplied job catalog. The map is expected to contain the
// job name (formatted through jobKeyFormatter) as the key.
func UpdateJobEnablement(settings map[string]string, jobs map[string]*shared.DefaultScrapeJob, schemaVersion string, jobKeyFormatter JobKeyFormatter) error {
	if settings == nil {
		return nil
	}

	for jobName, job := range jobs {
		key := jobKeyFormatter(jobName, schemaVersion)
		if key == "" {
			key = jobName
		}

		if value, ok := settings[key]; ok {
			enabled, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("common.UpdateJobEnablement::error parsing value for %s: %w", key, err)
			}
			job.Enabled = enabled
		}
	}

	return nil
}

// DetermineNoDefaultsEnabled inspects the supplied jobs and returns true when
// none of them is enabled for the effective controller and OS.
func DetermineNoDefaultsEnabled(jobs map[string]*shared.DefaultScrapeJob, controllerType, containerType, osType string) bool {
	effectiveController := controllerType
	if strings.EqualFold(containerType, shared.ControllerType.ConfigReaderSidecar) {
		effectiveController = shared.ControllerType.ReplicaSet
	}

	for _, job := range jobs {
		if job.ControllerType == effectiveController && strings.EqualFold(job.OSType, osType) && job.Enabled {
			return false
		}
	}
	return true
}

// ComputeClusterAlias mirrors the cluster alias normalization logic used by
// both CCP and MP variants.
func ComputeClusterAlias(cluster, macEnv string) string {
	trimmedCluster := strings.TrimSpace(cluster)
	if strings.EqualFold(strings.TrimSpace(macEnv), "true") {
		segments := strings.Split(trimmedCluster, "/")
		trimmedCluster = segments[len(segments)-1]
	}

	if trimmedCluster == "" {
		return ""
	}

	alias := regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(trimmedCluster, "_")
	return strings.Trim(alias, "_")
}

// WriteEnabledEnvFile persists the job enablement flags along with the
// NoDefaultsEnabled marker.
func WriteEnabledEnvFile(filePath string, jobs map[string]*shared.DefaultScrapeJob, envVarFormatter EnvVarFormatter, noDefaults bool) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("common.WriteEnabledEnvFile::opening file failed: %w", err)
	}
	defer file.Close()

	for jobName, job := range jobs {
		if _, err := fmt.Fprintf(file, "AZMON_PROMETHEUS_%s=%v\n", envVarFormatter(jobName), job.Enabled); err != nil {
			return fmt.Errorf("common.WriteEnabledEnvFile::writing flag for %s failed: %w", jobName, err)
		}
	}
	if _, err := fmt.Fprintf(file, "AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", noDefaults); err != nil {
		return fmt.Errorf("common.WriteEnabledEnvFile::writing no defaults flag failed: %w", err)
	}

	return nil
}
