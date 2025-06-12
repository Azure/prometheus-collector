package configmapsettings

import (
	"fmt"
	"os"
	"prometheus-collector/otelcollector/shared"
	"regexp"
	"strconv"
	"strings"
)

var NoDefaultsEnabled bool

func PopulateSettingValues(metricsConfigBySection map[string]map[string]string, schemaVersion string) error {
	configSectionName := "default-scrape-settings-enabled"
	if schemaVersion == shared.SchemaVersion.V2 {
		configSectionName = "default-targets-scrape-enabled"
	}
	settings, ok := metricsConfigBySection[configSectionName]
	if !ok {
		fmt.Println("ParseConfigMapForDefaultScrapeSettings::No default-targets-scrape-enabled section found, using defaults")
		return nil
	}

	for jobName, job := range shared.DefaultScrapeJobs {
		if setting, ok := settings[jobName]; ok {
			var err error
			job.Enabled, err = strconv.ParseBool(setting)
			if err != nil {
				return fmt.Errorf("ParseConfigMapForDefaultScrapeSettings::Error parsing value for %s: %v", jobName, err)
			}
			if job.Enabled {
				NoDefaultsEnabled = false
			}

			fmt.Printf("ParseConfigMapForDefaultScrapeSettings::Job: %s, Enabled: %t\n", jobName, job.Enabled)
		}
	}

	// Check if no defaults are enabled
	controllerType := os.Getenv("CONTROLLER_TYPE")
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	NoDefaultsEnabled = true

	for _, job := range shared.DefaultScrapeJobs {
		if job.ControllerType == controllerType &&
			job.OSType == osType &&
			job.Enabled {
			NoDefaultsEnabled = false
			break
		}
	}

	if NoDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled\n")
	}

	return nil
}

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	for jobName, job := range shared.DefaultScrapeJobs {
		file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_%s_SCRAPING_ENABLED=%v\n", strings.ToUpper(jobName), job.Enabled))
	}
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", NoDefaultsEnabled))

	return nil
}

func (c *Configurator) ConfigureDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	fmt.Println("Start prometheus-collector-settings Processing")

	// Load default settings based on the schema version
	var err error
	if configSchemaVersion == shared.SchemaVersion.V1 || configSchemaVersion == shared.SchemaVersion.V2 {
		fmt.Println("ConfigureDefaultScrapeSettings::Loading settings from config map")
	} else {
		// Initialize with an empty metrics config map if none is provided
		fmt.Println("ConfigureDefaultScrapeSettings::Loading default settings")
		metricsConfigBySection = make(map[string]map[string]string)
	}

	// Populate and print setting values
	err = PopulateSettingValues(metricsConfigBySection, configSchemaVersion)
	if err != nil {
		fmt.Printf("Error loading default settings: %v\n", err)
		return
	}

	// Set cluster alias
	if mac := os.Getenv("MAC"); mac != "" && strings.TrimSpace(mac) == "true" {
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		c.ConfigParser.ClusterAlias = clusterArray[len(clusterArray)-1]
	} else {
		c.ConfigParser.ClusterAlias = os.Getenv("CLUSTER")
	}

	if c.ConfigParser.ClusterAlias != "" && len(c.ConfigParser.ClusterAlias) > 0 {
		c.ConfigParser.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(c.ConfigParser.ClusterAlias, "_")
		c.ConfigParser.ClusterAlias = strings.Trim(c.ConfigParser.ClusterAlias, "_")
		fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", c.ConfigParser.ClusterAlias)
	}

	// Write default scrape settings to file
	err = c.ConfigWriter.WriteDefaultScrapeSettingsToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		fmt.Printf("Error writing default scrape settings to file: %v\n", err)
		return
	}

	fmt.Printf("End prometheus-collector-settings Processing\n")
}

func tomlparserDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	configLoaderPath := defaultSettingsMountPath
	if configSchemaVersion == shared.SchemaVersion.V2 {
		configLoaderPath = defaultSettingsMountPathv2
	}

	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: configLoaderPath},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: defaultSettingsEnvVarPath,
		ConfigParser:   &ConfigProcessor{},
	}

	configurator.ConfigureDefaultScrapeSettings(metricsConfigBySection, configSchemaVersion)
}
