package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	scrapeConfigs "github.com/prometheus-collector/defaultscrapeconfigs"
)

func (fcl *FilesystemConfigLoader) SetDefaultScrapeSettings() (map[string]string, error) {
	config := make(map[string]string)
	for jobName := range scrapeConfigs.DefaultScrapeJobs {
		config[jobName] = strconv.FormatBool(scrapeConfigs.DefaultScrapeJobs[jobName].Enabled)
	}
	config["noDefaultsEnabled"] = "false"

	return config, nil
}

func (fcl *FilesystemConfigLoader) ParseConfigMapForDefaultScrapeSettings() (map[string]string, error) {
	config, err := fcl.SetDefaultScrapeSettings()

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap for default scrape settings not mounted, using defaults")
		return config, nil
	}

	content, err := os.ReadFile(string(fcl.ConfigMapMountPath))
	if err != nil {
		return config, fmt.Errorf("using default values, error reading config map file: %s", err)
	}

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

func (cp *ConfigProcessor) PopulateSettingValues(parsedConfig map[string]string) {
	for scrapeJobName := range scrapeConfigs.DefaultScrapeJobs {
		isEnabledString := parsedConfig[scrapeJobName]
		isEnabled, err := strconv.ParseBool(isEnabledString)
		if err != nil {
			fmt.Printf("config::Error converting %s to bool: %v\n", isEnabledString, err)
			continue
		}
		job := scrapeConfigs.DefaultScrapeJobs[scrapeJobName]
		job.Enabled = isEnabled
		scrapeConfigs.DefaultScrapeJobs[scrapeJobName] = job
		fmt.Printf("config::Using configmap setting for if %s is enabled: %v\n", scrapeJobName, isEnabled)
	}

	controllerType := os.Getenv("CONTROLLER_TYPE")
	osType := strings.ToLower(os.Getenv("OS_TYPE"))
	cp.NoDefaultsEnabled = areNoDefaultsEnabled(controllerType, osType)

	if cp.NoDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled")
	}
}

func areNoDefaultsEnabled(controllerType, osType string) bool {
	noDefaultsEnabled := true
	for jobName := range scrapeConfigs.DefaultScrapeJobs {
		if scrapeConfigs.DefaultScrapeJobs[jobName].ControllerType == controllerType &&
			scrapeConfigs.DefaultScrapeJobs[jobName].OSType == osType &&
			scrapeConfigs.DefaultScrapeJobs[jobName].Enabled == true {
			noDefaultsEnabled = false
			continue
		}
	}
	return noDefaultsEnabled
}

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	for jobName := range scrapeConfigs.DefaultScrapeJobs {
		file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_%s_SCRAPING_ENABLED=%v\n", strings.ToUpper(scrapeConfigs.DefaultScrapeJobs[jobName].JobName), DefaultScrapeJobs[jobName].Enabled))
	}
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", cp.NoDefaultsEnabled))

	return nil
}

func (c *Configurator) ConfigureDefaultScrapeSettings() {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	fmt.Printf("Start prometheus-collector-settings Processing\n")

	// Load default settings based on the schema version
	var defaultSettings map[string]string
	var err error
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		defaultSettings, err = c.ConfigLoader.ParseConfigMapForDefaultScrapeSettings()
	} else {
		defaultSettings, err = c.ConfigLoader.SetDefaultScrapeSettings()
	}

	if err != nil {
		fmt.Printf("Error loading default settings: %v\n", err)
		return
	}

	// Populate and print setting values
	c.ConfigParser.PopulateSettingValues(defaultSettings)

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

func tomlparserDefaultScrapeSettings() {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: defaultSettingsMountPath},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: defaultSettingsEnvVarPath,
		ConfigParser:   &ConfigProcessor{},
	}

	configurator.ConfigureDefaultScrapeSettings()
}
