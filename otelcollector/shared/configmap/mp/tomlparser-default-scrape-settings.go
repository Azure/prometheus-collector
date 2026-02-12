package configmapsettings

import (
	"fmt"
	"log"
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
		log.Println("ParseConfigMapForDefaultScrapeSettings::No default-targets-scrape-enabled section found, using defaults")
		NoDefaultsEnabled = cmcommon.DetermineNoDefaultsEnabled(
			shared.DefaultScrapeJobs,
			os.Getenv("CONTROLLER_TYPE"),
			os.Getenv("CONTAINER_TYPE"),
			strings.ToLower(os.Getenv("OS_TYPE")),
		)
		if NoDefaultsEnabled {
			log.Printf("No default scrape configs enabled\n")
		}
		return nil
	}

	if err := cmcommon.UpdateJobEnablement(settings, shared.DefaultScrapeJobs, schemaVersion, func(name string, schema string) string {
		return name
	}); err != nil {
		return fmt.Errorf("ParseConfigMapForDefaultScrapeSettings::%w", err)
	}

	NoDefaultsEnabled = cmcommon.DetermineNoDefaultsEnabled(
		shared.DefaultScrapeJobs,
		os.Getenv("CONTROLLER_TYPE"),
		os.Getenv("CONTAINER_TYPE"),
		strings.ToLower(os.Getenv("OS_TYPE")),
	)
	if NoDefaultsEnabled {
		log.Printf("No default scrape configs enabled\n")
	}

	return nil
}

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	formatter := func(jobName string) string {
		return strings.ToUpper(jobName) + "_SCRAPING_ENABLED"
	}

	if err := cmcommon.WriteEnabledEnvFile(filename, shared.DefaultScrapeJobs, formatter, NoDefaultsEnabled); err != nil {
		return err
	}

	log.Println("No default scrape configs enabled:", NoDefaultsEnabled)
	return nil
}

func (c *Configurator) ConfigureDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, configSchemaVersion string) {
	log.Println("Start prometheus-collector-settings Processing")

	// Load default settings based on the schema version
	var err error
	if configSchemaVersion == shared.SchemaVersion.V1 || configSchemaVersion == shared.SchemaVersion.V2 {
		log.Println("ConfigureDefaultScrapeSettings::Loading settings from config map")
	} else {
		// Initialize with an empty metrics config map if none is provided
		log.Println("ConfigureDefaultScrapeSettings::Loading default settings")
		metricsConfigBySection = make(map[string]map[string]string)
	}

	// Populate and print setting values
	err = PopulateSettingValues(metricsConfigBySection, configSchemaVersion)
	if err != nil {
		log.Printf("Error loading default settings: %v\n", err)
		return
	}

	// Set cluster alias
	c.ConfigParser.ClusterAlias = cmcommon.ComputeClusterAlias(os.Getenv("CLUSTER"), os.Getenv("MAC"))
	if c.ConfigParser.ClusterAlias != "" {
		log.Printf("After replacing non-alpha-numeric characters with '_': %s\n", c.ConfigParser.ClusterAlias)
	}

	// Write default scrape settings to file
	err = c.ConfigWriter.WriteDefaultScrapeSettingsToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		log.Printf("Error writing default scrape settings to file: %v\n", err)
		return
	}

	log.Printf("End prometheus-collector-settings Processing\n")
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
