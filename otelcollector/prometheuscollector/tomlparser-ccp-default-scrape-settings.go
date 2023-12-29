package main

// import (
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"strings"
// )

// // ConfigLoader is an interface for loading configurations.
// type ConfigLoader interface {
// 	ParseConfigMap() (map[string]string, error)
// 	ParseConfigMap2() (map[string]string, error)
// }

// func (fcl *FilesystemConfigLoader) ParseConfigMap() (map[string]string, error) {
// 	config := make(map[string]string)

// 	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
// 		fmt.Printf("configmap for ccp default scrape settings not mounted, using defaults\n")
// 		return config, nil
// 	}

// 	content, err := ioutil.ReadFile(fcl.ConfigMapMountPath)
// 	if err != nil {
// 		fmt.Printf("Error reading config map file: %s, using defaults, please check config map for errors\n", err)
// 		return nil, err
// 	}

// 	lines := strings.Split(string(content), "\n")
// 	for _, line := range lines {
// 		parts := strings.SplitN(line, "=", 2)
// 		if len(parts) == 2 {
// 			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
// 		}
// 	}

// 	return config, nil
// }

// // ConfigWriter is an interface for writing configurations to a file.
// type ConfigWriter interface {
// 	WriteConfigToFile1(filename string) error
// 	WriteConfigToFile2(filename string) error
// }

// func (fcw *FileConfigWriter) WriteConfigToFile2(filename string) error {
// 	file, err := os.Create(filename)
// 	if err != nil {
// 		return fmt.Errorf("Exception while opening file for writing ccp default scrape settings environment variables: %s", err)
// 	}
// 	defer file.Close()

// 	for key, value := range fcw.Config {
// 		file.WriteString(fmt.Sprintf("%s=%s\n", key, value))
// 	}

// 	return nil
// }

// func (c *Configurator) Configure2() {
// 	configMapSettings, err := c.ConfigLoader.ParseConfigMap()
// 	if err == nil && len(configMapSettings) > 0 {
// 		err := c.ConfigWriter.WriteConfigToFile2(c.ConfigFilePath)
// 		if err != nil {
// 			fmt.Printf("%v\n", err)
// 			return
// 		}
// 	} else {
// 		fmt.Printf("Configmap for ccp default scrape settings not found or empty, using defaults\n")
// 	}
// }

// func tomlparserCCPDefaultScrapeSettings() {
// 	configurator := &Configurator{
// 		ConfigLoader: &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/ccp-default-scrape-settings"},
// 		ConfigWriter: &FileConfigWriter{},
// 		ConfigFilePath: "/opt/microsoft/configmapparser/config_ccp_default_scrape_settings_env_var",
// 	}

// 	fmt.Printf("Start ccp-default-scrape-settings Processing\n")

// 	configurator.Configure2()

// 	fmt.Printf("End ccp-default-scrape-settings Processing\n")
// }
