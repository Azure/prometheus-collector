package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

// ConfigLoader is an interface for loading configurations.
type ConfigLoader interface {
	ParseConfigMap() (map[string]string, error)
}

// FilesystemConfigLoader implements ConfigLoader for file-based configuration loading.
type FilesystemConfigLoader struct {
	ConfigMapMountPath string
}

func (fcl *FilesystemConfigLoader) ParseConfigMap() (map[string]string, error) {
	config := make(map[string]string)

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Printf("configmap for ccp default scrape settings not mounted, using defaults\n")
		return config, nil
	}

	content, err := ioutil.ReadFile(fcl.ConfigMapMountPath)
	if err != nil {
		fmt.Printf("Error reading config map file: %s, using defaults, please check config map for errors\n", err)
		return nil, err
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

// ConfigWriter is an interface for writing configurations to a file.
type ConfigWriter interface {
	WriteConfigToFile(filename string) error
}

// FileConfigWriter implements ConfigWriter for writing configurations to a file.
type FileConfigWriter struct {
	Config map[string]string
}

func (fcw *FileConfigWriter) WriteConfigToFile(filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing ccp default scrape settings environment variables: %s", err)
	}
	defer file.Close()

	for key, value := range fcw.Config {
		file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
	}

	return nil
}

// Configurator is responsible for configuring the application.
type Configurator struct {
	ConfigLoader   ConfigLoader
	ConfigWriter   ConfigWriter
	ConfigFilePath string
}

func (c *Configurator) Configure() {
	configMapSettings, err := c.ConfigLoader.ParseConfigMap()
	if err == nil && len(configMapSettings) > 0 {
		err := c.ConfigWriter.WriteConfigToFile(c.ConfigFilePath)
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
	} else {
		fmt.Printf("Configmap for ccp default scrape settings not found or empty, using defaults\n")
	}
}

func main() {
	configurator := &Configurator{
		ConfigLoader: &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/ccp-default-scrape-settings"},
		ConfigWriter: &FileConfigWriter{},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_ccp_default_scrape_settings_env_var",
	}

	fmt.Printf("Start ccp-default-scrape-settings Processing\n")

	configurator.Configure()

	fmt.Printf("End ccp-default-scrape-settings Processing\n")
}