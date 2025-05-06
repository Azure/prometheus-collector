package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	scrapeConfigs "github.com/prometheus-collector/defaultscrapeconfigs"
)

// GetDefaultSettings returns the default scrape configuration
func GetDefaultSettings() map[string]string {
	config := make(map[string]string)
	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		config[jobName] = strconv.FormatBool(job.Enabled)
	}
	config["noDefaultsEnabled"] = "false"
	return config
}

// ParseConfigMap reads settings from a config map if available, otherwise uses defaults
func (fcl *FilesystemConfigLoader) ParseConfigMap() (map[string]string, error) {
	config := GetDefaultSettings()

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap for default scrape settings not mounted, using defaults")
		return config, nil
	}

	content, err := os.ReadFile(fcl.ConfigMapMountPath)
	if err != nil {
		return config, fmt.Errorf("using default values, error reading config map file: %s", err)
	}

	// Parse config map content
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if _, ok := config[key]; ok {
				config[key] = value
			}
		}
	}

	fmt.Println("using configmap for default scrape settings...")
	return config, nil
}

// ApplySettings applies the parsed configuration to the scrape jobs
func ApplySettings(parsedConfig map[string]string) bool {
	// Apply settings to jobs
	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		if value, exists := parsedConfig[jobName]; exists {
			if enabled, err := strconv.ParseBool(value); err == nil {
				job.Enabled = enabled
				scrapeConfigs.DefaultScrapeJobs[jobName] = job
				fmt.Printf("config::Using configmap setting for if %s is enabled: %v\n", jobName, enabled)
			} else {
				fmt.Printf("config::Error converting %s to bool: %v\n", value, err)
			}
		}
	}

	// Check if no defaults are enabled
	controllerType := os.Getenv("CONTROLLER_TYPE")
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	noDefaultsEnabled := true

	for _, job := range scrapeConfigs.DefaultScrapeJobs {
		if job.ControllerType == controllerType &&
			job.OSType == osType &&
			job.Enabled {
			noDefaultsEnabled = false
			break
		}
	}

	if noDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled\n")
	}

	return noDefaultsEnabled
}

// WriteSettingsToFile writes the current settings to a file
func WriteSettingsToFile(filename string, noDefaultsEnabled bool) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("error opening file: %s", err)
	}
	defer file.Close()

	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_%s_SCRAPING_ENABLED=%v\n",
			strings.ToUpper(job.JobName), job.Enabled))
	}
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", noDefaultsEnabled))

	return nil
}

// ConfigureDefaultScrapeSettings orchestrates the configuration process
func ConfigureDefaultScrapeSettings(configMapPath, outputFilePath string) {
	fmt.Println("Start prometheus-collector-settings Processing")

	var settings map[string]string
	var err error

	loader := &FilesystemConfigLoader{ConfigMapMountPath: configMapPath}

	// Load settings based on schema version
	if configSchema := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION"); configSchema == "v1" {
		settings, err = loader.ParseConfigMap()
	} else {
		settings = GetDefaultSettings()
	}

	if err != nil {
		fmt.Printf("Error loading settings: %v\n", err)
		return
	}

	// Apply settings
	noDefaultsEnabled := ApplySettings(settings)

	// Set cluster alias
	clusterAlias := os.Getenv("CLUSTER")
	if mac := os.Getenv("MAC"); mac == "true" {
		parts := strings.Split(clusterAlias, "/")
		clusterAlias = parts[len(parts)-1]
	}

	if clusterAlias != "" {
		// Sanitize cluster alias
		sanitized := regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(clusterAlias, "_")
		sanitized = strings.Trim(sanitized, "_")
		fmt.Printf("Sanitized cluster alias: %s\n", sanitized)
	}

	// Write settings to file
	if err := WriteSettingsToFile(outputFilePath, noDefaultsEnabled); err != nil {
		fmt.Printf("Error writing settings to file: %v\n", err)
		return
	}

	fmt.Println("End prometheus-collector-settings Processing")
}

func tomlparserDefaultScrapeSettings() {
	ConfigureDefaultScrapeSettings(defaultSettingsMountPath, defaultSettingsEnvVarPath)
}
