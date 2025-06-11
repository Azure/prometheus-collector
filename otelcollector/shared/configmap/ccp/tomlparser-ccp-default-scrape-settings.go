package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus-collector/shared"
)

var NoDefaultsEnabled bool

// ParseConfigMapForDefaultScrapeSettings extracts the control plane scrape settings from metricsConfigBySection.
func PopulateSettingValues(metricsConfigBySection map[string]map[string]string, schemaVersion string) error {
	settings, ok := metricsConfigBySection["default-targets-scrape-enabled"]
	if !ok {
		fmt.Println("ParseConfigMapForDefaultScrapeSettings::No default-targets-scrape-enabled section found, using defaults")
		return nil
	}

	NoDefaultsEnabled = true
	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		if schemaVersion == shared.SchemaVersion.V1 {
			jobName = "controlplane-" + jobName // Prefix for v1 schema
		}

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
	if NoDefaultsEnabled {
		fmt.Println("PopulateSettingValues::No default scrape configs enabled")
	}
	return nil
}

// WriteDefaultScrapeSettingsToFile writes the configuration settings to a file.
func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	fmt.Printf("WriteDefaultScrapeSettingsToFile::Writing settings to file: %s\n", filename)

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	for jobName, job := range shared.ControlPlaneDefaultScrapeJobs {
		file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_%s_ENABLED=%v\n", strings.ToUpper(jobName), job.Enabled))
	}

	fmt.Println("WriteDefaultScrapeSettingsToFile::Settings written to file successfully")
	return nil
}

// ConfigureDefaultScrapeSettings processes the configuration and writes it to a file.
func (c *Configurator) ConfigureDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	fmt.Printf("ConfigureDefaultScrapeSettings::Config schema version: %s\n", configSchemaVersion)
	fmt.Println("ConfigureDefaultScrapeSettings::Start prometheus-collector-settings Processing")

	// Load default settings based on the schema version
	if configSchemaVersion == shared.SchemaVersion.V1 || configSchemaVersion == shared.SchemaVersion.V2 {
		fmt.Println("ConfigureDefaultScrapeSettings::Loading settings from config map")
	} else {
		// Initialize with an empty metrics config map if none is provided
		fmt.Println("ConfigureDefaultScrapeSettings::Loading default settings")
		metricsConfigBySection = make(map[string]map[string]string)
	}

	// Populate and print setting values
	err := PopulateSettingValues(metricsConfigBySection, configSchemaVersion)
	if err != nil {
		fmt.Println(err.Error())
	}

	// Set cluster alias
	if mac := os.Getenv("MAC"); mac != "" && strings.TrimSpace(mac) == "true" {
		fmt.Println("ConfigureDefaultScrapeSettings::MAC environment variable is true")
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		c.ConfigParser.ClusterAlias = clusterArray[len(clusterArray)-1]
	} else {
		c.ConfigParser.ClusterAlias = os.Getenv("CLUSTER")
	}

	if c.ConfigParser.ClusterAlias != "" && len(c.ConfigParser.ClusterAlias) > 0 {
		fmt.Printf("ConfigureDefaultScrapeSettings::Original cluster alias: %s\n", c.ConfigParser.ClusterAlias)
		// Replace all non-alphanumeric characters with "_"
		c.ConfigParser.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(c.ConfigParser.ClusterAlias, "_")
		c.ConfigParser.ClusterAlias = strings.Trim(c.ConfigParser.ClusterAlias, "_")
		fmt.Printf("ConfigureDefaultScrapeSettings::Sanitized cluster alias: %s\n", c.ConfigParser.ClusterAlias)
	}

	// Write default scrape settings to file
	err = c.ConfigWriter.WriteDefaultScrapeSettingsToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		fmt.Printf("ConfigureDefaultScrapeSettings::Error writing default scrape settings to file: %v\n", err)
		return
	}

	fmt.Println("ConfigureDefaultScrapeSettings::End prometheus-collector-settings Processing")
}

// TomlparserCCPDefaultScrapeSettings initializes the configurator and processes the configuration.
func tomlparserCCPDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, schemaVersion string) {
	fmt.Println("tomlparserCCPDefaultScrapeSettings::Start ccp-default-scrape-settings Processing")

	configLoaderPath := "/etc/config/settings/default-targets-scrape-enabled"
	if schemaVersion == shared.SchemaVersion.V2 {
		configLoaderPath = "/etc/config/settings/default-targets-scrape-enabled"
	}

	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: configLoaderPath},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var",
		ConfigParser:   &ConfigProcessor{},
	}

	configurator.ConfigureDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	fmt.Println("tomlparserCCPDefaultScrapeSettings::End ccp-default-scrape-settings Processing")
}
