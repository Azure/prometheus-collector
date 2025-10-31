package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"strings"

	"github.com/prometheus-collector/shared"
	cmcommon "github.com/prometheus-collector/shared/configmap/common"
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
		NoDefaultsEnabled = false
		return nil
	}

	if err := cmcommon.UpdateJobEnablement(settings, shared.ControlPlaneDefaultScrapeJobs, schemaVersion, func(jobName string, schema string) string {
		if schema == shared.SchemaVersion.V1 {
			return "controlplane-" + jobName
		}
		return jobName
	}); err != nil {
		return fmt.Errorf("ParseConfigMapForDefaultScrapeSettings::%w", err)
	}

	NoDefaultsEnabled = cmcommon.DetermineNoDefaultsEnabled(
		shared.ControlPlaneDefaultScrapeJobs,
		os.Getenv("CONTROLLER_TYPE"),
		os.Getenv("CONTAINER_TYPE"),
		strings.ToLower(os.Getenv("OS_TYPE")),
	)
	if NoDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled\n")
	}
	return nil
}

// WriteDefaultScrapeSettingsToFile writes the configuration settings to a file.
func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	fmt.Printf("WriteDefaultScrapeSettingsToFile::Writing settings to file: %s\n", filename)

	formatter := func(jobName string) string {
		return strings.ToUpper(jobName) + "_ENABLED"
	}

	if err := cmcommon.WriteEnabledEnvFile(filename, shared.ControlPlaneDefaultScrapeJobs, formatter, NoDefaultsEnabled); err != nil {
		return err
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
	c.ConfigParser.ClusterAlias = cmcommon.ComputeClusterAlias(os.Getenv("CLUSTER"), os.Getenv("MAC"))
	if c.ConfigParser.ClusterAlias != "" {
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

	configLoaderPath := defaultSettingsMountPath
	if schemaVersion == shared.SchemaVersion.V2 {
		configLoaderPath = defaultSettingsMountPathv2
	}

	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: configLoaderPath},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: defaultSettingsEnvVarPath,
		ConfigParser:   &ConfigProcessor{},
	}

	configurator.ConfigureDefaultScrapeSettings(metricsConfigBySection, schemaVersion)
	fmt.Println("tomlparserCCPDefaultScrapeSettings::End ccp-default-scrape-settings Processing")
}
