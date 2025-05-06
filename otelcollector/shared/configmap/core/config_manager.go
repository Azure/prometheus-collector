package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus-collector/otelcollector/shared/configmap/defaultscrapeconfigs"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

// Constants for configuration paths
const (
	DefaultConfigPathPrefix = "/opt/microsoft/otelcollector/default-prom-configs/"
	MergedDefaultConfigPath = "/opt/defaultsMergedConfig.yml"
)

// ConfigProcessor defines a function signature for processing different config types
type ConfigProcessor func(jobName string, job *defaultscrapeconfigs.DefaultScrapeJob, value string) (string, error)

// ConfigManager manages the configuration processing
type ConfigManager struct {
	controllerType         string
	osType                 string
	isOperatorMode         bool
	isAdvancedMode         bool
	envVars                map[string]string
	scrapeJobs             map[string]defaultscrapeconfigs.DefaultScrapeJob
	configMapPath          string
	outputPath             string
	templatePath           string
	regexHashPath          string
	intervalHashPath       string
	configType             string
	schemaVersion          string
	metricsConfigBySection map[string]map[string]string
}

// NewConfigManager creates a new config manager instance
func NewConfigManager(configType string, schemaVersion string, metricsConfigBySection map[string]map[string]string) *ConfigManager {
	// Determine which job set to use based on config type
	var jobs map[string]defaultscrapeconfigs.DefaultScrapeJob
	if configType == "controlplane" {
		jobs = defaultscrapeconfigs.ControlPlaneDefaultScrapeJobs
	} else {
		jobs = defaultscrapeconfigs.DefaultScrapeJobs
	}

	return &ConfigManager{
		controllerType:         strings.TrimSpace(os.Getenv("CONTROLLER_TYPE")),
		osType:                 strings.ToLower(os.Getenv("OS_TYPE")),
		isOperatorMode:         os.Getenv("AZMON_OPERATOR_ENABLED") == "true" || os.Getenv("CONTAINER_TYPE") == "ConfigReaderSidecar",
		isAdvancedMode:         os.Getenv("MODE") == "advanced",
		envVars:                make(map[string]string),
		scrapeJobs:             jobs,
		configMapPath:          "/etc/config/settings",
		templatePath:           "/opt/microsoft/otelcollector/collector-config-template.yml",
		regexHashPath:          "/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash",
		intervalHashPath:       "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash",
		configType:             configType,
		schemaVersion:          schemaVersion,
		metricsConfigBySection: metricsConfigBySection,
	}
}

// getOutputPath constructs the appropriate output path based on config type and section
func (cm *ConfigManager) getOutputPath(baseName string) string {
	prefix := ""
	if cm.configType == "controlplane" {
		prefix = "config_ccp_"
	} else {
		prefix = "config_"
	}

	return filepath.Join("/opt/microsoft/configmapparser", prefix+baseName)
}

// getSectionName determines the section name based on schema version and config type
func (cm *ConfigManager) getSectionName(schemaVersion, baseSection string) string {
	if schemaVersion == "v2" && cm.configType == "controlplane" {
		return "controlplane-metrics"
	}
	return baseSection
}

// getJobName handles schema version differences in job naming
func (cm *ConfigManager) getJobName(schemaVersion, key string) string {
	jobName := key
	if schemaVersion == "v2" {
		// For v2, strip the "controlplane-" prefix for control plane jobs
		jobName = strings.TrimPrefix(jobName, "controlplane-")
	}
	return jobName
}

// processConfiguration is a generic configuration processor
func (cm *ConfigManager) processConfiguration(
	outputFileSuffix string,
	baseSection string,
	keyGenerator func(string) string,
	defaultValueGetter func(defaultscrapeconfigs.DefaultScrapeJob) string,
	valueProcessor ConfigProcessor,
	isYamlOutput bool,
) error {
	// Get output path
	outputPath := cm.getOutputPath(outputFileSuffix)

	// Map to store settings
	configValues := make(map[string]string)

	// Set defaults first
	for jobName, job := range cm.scrapeJobs {
		key := keyGenerator(jobName)
		configValues[key] = defaultValueGetter(job)
	}

	// Override with values from config map if schema version is valid
	if cm.schemaVersion == "v1" || cm.schemaVersion == "v2" {
		// Get the appropriate section name
		sectionName := cm.getSectionName(cm.schemaVersion, baseSection)

		if settings, ok := cm.metricsConfigBySection[sectionName]; ok {
			// Handle special cases first if this is the metrics keep list section
			if baseSection == "default-targets-metrics-keep-list" && settings["minimalingestionprofile"] == "false" {
				// Special case for minimal ingestion profile
				for jobName, job := range cm.scrapeJobs {
					// Clear minimal keep list regex if minimal profile is disabled
					job.MinimalKeepListRegex = ""
					cm.scrapeJobs[jobName] = job
				}
			}

			// Process all settings
			for key, value := range settings {
				// Skip the minimal ingestion profile key for keep list settings
				if baseSection == "default-targets-metrics-keep-list" && key == "minimalingestionprofile" {
					continue
				}

				// Get actual job name accounting for schema version
				jobName := cm.getJobName(cm.schemaVersion, key)

				// Process if job exists
				if job, exists := cm.scrapeJobs[jobName]; exists {
					// Process the value
					if processedValue, err := valueProcessor(jobName, &job, value); err == nil {
						configKey := keyGenerator(jobName)
						configValues[configKey] = processedValue

						// Update the job in the map
						cm.scrapeJobs[jobName] = job
					}
				}
			}
		}
	}

	// Add any post-processing special cases
	if baseSection == "default-scrape-settings-enabled" {
		// Check if no defaults are enabled
		noDefaultsEnabled := true
		for _, job := range cm.scrapeJobs {
			if job.ControllerType == cm.controllerType && job.OSType == cm.osType && job.Enabled {
				noDefaultsEnabled = false
				break
			}
		}

		// Add the no-defaults flag
		configValues["AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED"] = fmt.Sprintf("%t", noDefaultsEnabled)
	}

	// Write to file with appropriate method
	if isYamlOutput {
		return shared.writeYamlToFile(outputPath, configValues)
	}
	return shared.WriteEnvVarsToFile(outputPath, configValues)
}

// ProcessDefaultScrapeSettings processes the default scrape settings
func (cm *ConfigManager) ProcessDefaultScrapeSettings() error {
	return cm.processConfiguration(
		cm.metricsConfigBySection,
		"default_scrape_settings_env_var",
		"default-scrape-settings-enabled",
		func(jobName string) string {
			return fmt.Sprintf("AZMON_PROMETHEUS_%s_SCRAPING_ENABLED", strings.ToUpper(jobName))
		},
		func(job defaultscrapeconfigs.DefaultScrapeJob) string {
			return fmt.Sprintf("%t", job.Enabled)
		},
		func(jobName string, job *defaultscrapeconfigs.DefaultScrapeJob, value string) (string, error) {
			job.Enabled = (value == "true")
			return value, nil
		},
		false, // Write as env vars, not YAML
	)
}

// ProcessMetricsKeepList processes the metrics keep list settings
func (cm *ConfigManager) ProcessMetricsKeepList() error {
	return cm.processConfiguration(
		cm.metricsConfigBySection,
		"def_targets_metrics_keep_list_hash",
		"default-targets-metrics-keep-list",
		func(jobName string) string {
			return fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))
		},
		func(job defaultscrapeconfigs.DefaultScrapeJob) string {
			// Combine minimal and customer regexes
			finalRegex := ""
			if job.MinimalKeepListRegex != "" {
				finalRegex = job.MinimalKeepListRegex
			}
			if job.CustomerKeepListRegex != "" {
				if finalRegex != "" {
					finalRegex += "|"
				}
				finalRegex += job.CustomerKeepListRegex
			}
			return finalRegex
		},
		func(jobName string, job *defaultscrapeconfigs.DefaultScrapeJob, value string) (string, error) {
			// Validate regex
			if !shared.isValidRegex(value) {
				return "", fmt.Errorf("invalid regex for %s: %s", jobName, value)
			}

			// Store customer regex
			job.CustomerKeepListRegex = value

			// Combine minimal and customer regexes
			finalRegex := ""
			if job.MinimalKeepListRegex != "" {
				finalRegex = job.MinimalKeepListRegex
			}
			if value != "" {
				if finalRegex != "" {
					finalRegex += "|"
				}
				finalRegex += value
			}

			// Store the final regex
			job.KeepListRegex = finalRegex

			return finalRegex, nil
		},
		true, // Write as YAML
	)
}

// ProcessScrapeIntervals processes the scrape interval settings
func (cm *ConfigManager) ProcessScrapeIntervals() error {
	return cm.processConfiguration(
		cm.metricsConfigBySection,
		"def_targets_scrape_intervals_hash",
		"default-targets-scrape-interval-settings",
		func(jobName string) string {
			return fmt.Sprintf("%s_SCRAPE_INTERVAL", strings.ToUpper(jobName))
		},
		func(job defaultscrapeconfigs.DefaultScrapeJob) string {
			return job.ScrapeInterval
		},
		func(jobName string, job *defaultscrapeconfigs.DefaultScrapeJob, value string) (string, error) {
			// Validate duration format
			if !shared.isValidDuration(value) {
				return "", fmt.Errorf("invalid duration for %s: %s", jobName, value)
			}

			// Store scrape interval
			job.ScrapeInterval = value

			return value, nil
		},
		true, // Write as YAML
	)
}

// MergePrometheusConfigs merges the prometheus configurations
func (cm *ConfigManager) MergePrometheusConfigs() error {
	// Set output paths
	var defaultConfigsPath string
	if cm.configType == "controlplane" {
		defaultConfigsPath = "/opt/ccp-defaultsMergedConfig.yml"
	} else {
		defaultConfigsPath = "/opt/defaultsMergedConfig.yml"
	}

	// Load regex and interval hash maps
	regexHash := shared.loadHashMap(cm.regexHashPath)
	intervalHash := shared.loadHashMap(cm.intervalHashPath)

	// Generate default configurations
	defaultConfigs := []string{}

	// Process each enabled job
	for _, job := range cm.scrapeJobs {
		// Skip if not enabled or doesn't match current controller type and OS
		if !job.Enabled || job.ControllerType != cm.controllerType || job.OSType != cm.osType {
			continue
		}

		// Get job settings
		jobName := job.JobName
		scrapeInterval := job.ScrapeInterval
		keepListRegex := job.KeepListRegex

		// Get config file path
		configFile := filepath.Join(DefaultConfigPathPrefix, job.ScrapeConfigDefinitionFile)

		// Update the config file with scrape interval and keep list regex
		if err := cm.updateConfigFile(configFile, jobName, scrapeInterval, keepListRegex); err != nil {
			shared.EchoError(fmt.Sprintf("Error updating config file %s: %v", configFile, err))
			continue
		}

		// Add to default configs list
		defaultConfigs = append(defaultConfigs, configFile)
	}

	// Merge default configurations
	if len(defaultConfigs) > 0 {
		mergedConfig := cm.mergeConfigs(defaultConfigs)
		if mergedConfig == nil {
			return fmt.Errorf("failed to merge default configs")
		}

		// Write merged config to file
		if err := shared.writeYamlToFile(defaultConfigsPath, mergedConfig); err != nil {
			return fmt.Errorf("failed to write merged config: %v", err)
		}
	}

	return nil
}

// ValidateAndApplyConfig validates and applies the prometheus configuration
func (cm *ConfigManager) ValidateAndApplyConfig() error {
	// Set paths based on config type
	prefix := ""
	if cm.configType == "controlplane" {
		prefix = "ccp-"
	}

	inputConfigPath := fmt.Sprintf("/opt/%sdefaultsMergedConfig.yml", prefix)
	outputConfigPath := fmt.Sprintf("/opt/%scollector-config-with-defaults.yml", prefix)
	defaultOutputPath := fmt.Sprintf("/opt/microsoft/otelcollector/%scollector-config-default.yml", prefix)

	// Set default flags
	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "false", true)
	shared.SetEnvAndSourceBashrcOrPowershell("CONFIG_VALIDATOR_RUNNING_IN_AGENT", "true", true)

	// Try custom config for non-control plane
	if cm.configType != "controlplane" && shared.FileExists("/opt/promMergedConfig.yml") {
		if err := validateConfig("/opt/promMergedConfig.yml", "/opt/microsoft/otelcollector/collector-config.yml", cm.templatePath); err == nil {
			shared.SetEnvAndSourceBashrcOrPowershell("AZMON_SET_GLOBAL_SETTINGS", "true", true)
			return nil
		}

		shared.EchoError("Prometheus custom config validation failed. The custom config will not be used")
		shared.SetEnvAndSourceBashrcOrPowershell("AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG", "true", true)
	}

	// Fall back to default config when needed
	if shared.FileExists(inputConfigPath) {
		if err := validateConfig(inputConfigPath, outputConfigPath, cm.templatePath); err == nil {
			if err := shared.CopyFile(outputConfigPath, defaultOutputPath); err != nil {
				return fmt.Errorf("error copying default config: %v", err)
			}
		} else {
			shared.EchoError("Prometheus default scrape config validation failed. No scrape configs will be used")
		}
	}

	shared.SetEnvAndSourceBashrcOrPowershell("AZMON_USE_DEFAULT_PROMETHEUS_CONFIG", "true", true)
	return nil
}

// Helper function to validate config
func validateConfig(configPath, outputPath, templatePath string) error {
	return shared.StartCommandAndWait(
		"/opt/promconfigvalidator",
		"--config", configPath,
		"--output", outputPath,
		"--otelTemplate", templatePath,
	)
}

// Helper functions

// updateConfigFile updates a config file with scrape interval and keep list regex
func (cm *ConfigManager) updateConfigFile(path, jobName, scrapeInterval, keepListRegex string) error {
	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse YAML
	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return err
	}

	// Update scrape interval
	if scrapeInterval != "" {
		if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
			for _, cfg := range scrapeConfigs {
				if scrapeMap, ok := cfg.(map[string]interface{}); ok {
					scrapeMap["scrape_interval"] = scrapeInterval
				}
			}
		}
	}

	// Add metric relabel configs for keep list regex if provided
	if keepListRegex != "" {
		if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
			for _, cfg := range scrapeConfigs {
				if scrapeMap, ok := cfg.(map[string]interface{}); ok {
					// Create metric relabel config for keep list regex
					metricRelabel := map[string]interface{}{
						"source_labels": []string{"__name__"},
						"action":        "keep",
						"regex":         keepListRegex,
					}

					// Add to existing relabel configs or create new array
					if relabelConfigs, ok := scrapeMap["metric_relabel_configs"].([]interface{}); ok {
						scrapeMap["metric_relabel_configs"] = append(relabelConfigs, metricRelabel)
					} else {
						scrapeMap["metric_relabel_configs"] = []interface{}{metricRelabel}
					}
				}
			}
		}
	}

	// Replace placeholders with environment variables
	for _, job := range cm.scrapeJobs {
		if job.JobName == jobName && len(job.PlaceholderNames) > 0 {
			yamlStr, err := yaml.Marshal(config)
			if err != nil {
				return err
			}

			// Replace each placeholder
			contentStr := string(yamlStr)
			for _, placeholder := range job.PlaceholderNames {
				envValue := os.Getenv(placeholder)
				contentStr = strings.ReplaceAll(contentStr, "$$"+placeholder+"$$", envValue)
			}

			// Write back to file
			return os.WriteFile(path, []byte(contentStr), 0644)
		}
	}

	// Write updated config back to file
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(path, yamlData, 0644)
}

// mergeConfigs merges multiple config files into a single configuration
func (cm *ConfigManager) mergeConfigs(configFiles []string) map[string]interface{} {
	mergedConfig := make(map[string]interface{})

	for _, file := range configFiles {
		config, err := shared.readYaml(file)
		if err != nil {
			shared.EchoError(fmt.Sprintf("Error reading config file %s: %v", file, err))
			continue
		}

		// Merge with accumulating result
		mergedConfig = cm.deepMerge(mergedConfig, config)
	}

	return mergedConfig
}

// deepMerge performs a deep merge of two maps
func (cm *ConfigManager) deepMerge(target, source map[string]interface{}) map[string]interface{} {
	for key, sourceValue := range source {
		if targetValue, exists := target[key]; exists {
			// If both values are maps, merge them recursively
			if sourceMap, ok := sourceValue.(map[string]interface{}); ok {
				if targetMap, ok := targetValue.(map[string]interface{}); ok {
					target[key] = cm.deepMerge(targetMap, sourceMap)
					continue
				}
			}

			// If both are slices, append them
			if sourceSlice, ok := sourceValue.([]interface{}); ok {
				if targetSlice, ok := targetValue.([]interface{}); ok {
					target[key] = append(targetSlice, sourceSlice...)
					continue
				}
			}
		}

		// Otherwise just set the value
		target[key] = sourceValue
	}

	return target
}
