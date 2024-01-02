package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

func (fcl *FilesystemConfigLoader) ParseConfigMapForDefaultScrapeSettings() (map[string]string, error) {
	config := make(map[string]string)

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap for ccp default scrape settings not mounted, using defaults")
		return config, nil
	}

	content, err := ioutil.ReadFile(fcl.ConfigMapMountPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config map file: %s", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	return config, nil
}

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing ccp default scrape settings environment variables: %s", err)
	}
	defer file.Close()

	for key, value := range fcw.Config {
		_, err := file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			return fmt.Errorf("error writing to file: %s", err)
		}
	}

	return nil
}

func (c *Configurator) ConfigureDefaultScrapeSettings() {
	configMapSettings, err := c.ConfigLoader.ParseConfigMapForDefaultScrapeSettings()
	if err != nil {
		fmt.Printf("error parsing config map: %v\n", err)
		return
	}

	if len(configMapSettings) > 0 {
		err := c.ConfigWriter.WriteDefaultScrapeSettingsToFile(c.ConfigFilePath)
		if err != nil {
			fmt.Printf("error writing default scrape settings to file: %v\n", err)
			return
		}
	} else {
		fmt.Println("configmap for ccp default scrape settings not found or empty, using defaults")
	}
}

func tomlparserCCPDefaultScrapeSettings() {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/default-scrape-settings-enabled"},
		ConfigWriter:   &FileConfigWriter{Config: map[string]string{}},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var",
	}

	fmt.Println("Start ccp-default-scrape-settings Processing")
	configurator.ConfigureDefaultScrapeSettings()
	fmt.Println("End ccp-default-scrape-settings Processing")
}
