package ccpconfigmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// SetDefaultScrapeSettings sets the default values for control plane scrape settings.
func (fcl *FilesystemConfigLoader) SetDefaultScrapeSettings() (map[string]string, error) {
	config := make(map[string]string)
	// Set default values
	config["controlplane-apiserver"] = "true"
	config["controlplane-cluster-autoscaler"] = "false"
	config["controlplane-kube-scheduler"] = "false"
	config["controlplane-kube-controller-manager"] = "false"
	config["controlplane-etcd"] = "true"
	return config, nil
}

// ParseConfigMapForDefaultScrapeSettings extracts the control plane scrape settings from parsedData.
func (fcl *FilesystemConfigLoader) ParseConfigMapForDefaultScrapeSettings(parsedData map[string]map[string]string, schemaVersion string) (map[string]string, error) {
	config := make(map[string]string)
	// Set default values
	config["controlplane-apiserver"] = "true"
	config["controlplane-cluster-autoscaler"] = "false"
	config["controlplane-kube-scheduler"] = "false"
	config["controlplane-kube-controller-manager"] = "false"
	config["controlplane-etcd"] = "true"

	// Override defaults with values from parsedData
	if schemaVersion == "v1" {
		// For v1, control plane jobs are under "default-scrape-settings-enabled" with "controlplane-" prefix
		if settings, ok := parsedData["default-scrape-settings-enabled"]; ok {
			for key, value := range settings {
				if strings.HasPrefix(key, "controlplane-") {
					config[key] = value
				}
			}
		}
	} else if schemaVersion == "v2" {
		// For v2, control plane jobs are under "controlplane-metrics" without "controlplane-" prefix
		if settings, ok := parsedData["controlplane-metrics"]; ok {
			// Map v2 keys to v1 keys
			v2ToV1KeyMap := map[string]string{
				"apiserver":               "controlplane-apiserver",
				"cluster-autoscaler":      "controlplane-cluster-autoscaler",
				"kube-scheduler":          "controlplane-kube-scheduler",
				"kube-controller-manager": "controlplane-kube-controller-manager",
				"etcd":                    "controlplane-etcd",
			}
			for key, value := range settings {
				if v1Key, ok := v2ToV1KeyMap[key]; ok {
					config[v1Key] = value
				}
			}
		}
	}

	fmt.Println("Using configmap for ccp scrape settings...")
	return config, nil
}

// PopulateSettingValues populates settings from the parsed configuration.
func (cp *ConfigProcessor) PopulateSettingValues(parsedConfig map[string]string) {
	if val, ok := parsedConfig["controlplane-kube-controller-manager"]; ok && val != "" {
		cp.ControlplaneKubeControllerManager = val
		fmt.Printf("config::Using scrape settings for controlplane-kube-controller-manager: %v\n", cp.ControlplaneKubeControllerManager)
	}

	if val, ok := parsedConfig["controlplane-kube-scheduler"]; ok && val != "" {
		cp.ControlplaneKubeScheduler = val
		fmt.Printf("config::Using scrape settings for controlplane-kube-scheduler: %v\n", cp.ControlplaneKubeScheduler)
	}

	if val, ok := parsedConfig["controlplane-apiserver"]; ok && val != "" {
		cp.ControlplaneApiserver = val
		fmt.Printf("config::Using scrape settings for controlplane-apiserver: %v\n", cp.ControlplaneApiserver)
	}

	if val, ok := parsedConfig["controlplane-cluster-autoscaler"]; ok && val != "" {
		cp.ControlplaneClusterAutoscaler = val
		fmt.Printf("config::Using scrape settings for controlplane-cluster-autoscaler: %v\n", cp.ControlplaneClusterAutoscaler)
	}

	if val, ok := parsedConfig["controlplane-etcd"]; ok && val != "" {
		cp.ControlplaneEtcd = val
		fmt.Printf("config::Using scrape settings for controlplane-etcd: %v\n", cp.ControlplaneEtcd)
	}

	if os.Getenv("MODE") == "" && strings.ToLower(strings.TrimSpace(os.Getenv("MODE"))) == "advanced" {
		controllerType := os.Getenv("CONTROLLER_TYPE")
		if controllerType == "ReplicaSet" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" &&
			cp.ControlplaneKubeControllerManager == "" && cp.ControlplaneKubeScheduler == "" &&
			cp.ControlplaneApiserver == "" && cp.ControlplaneClusterAutoscaler == "" && cp.ControlplaneEtcd == "" {
			cp.NoDefaultsEnabled = true
		}
	} else if cp.ControlplaneKubeControllerManager == "" && cp.ControlplaneKubeScheduler == "" &&
		cp.ControlplaneApiserver == "" && cp.ControlplaneClusterAutoscaler == "" && cp.ControlplaneEtcd == "" {
		cp.NoDefaultsEnabled = true
	}

	if cp.NoDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled")
	}
}

// WriteDefaultScrapeSettingsToFile writes the configuration settings to a file.
func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_CONTROLLER_MANAGER_ENABLED=%v\n", cp.ControlplaneKubeControllerManager))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CONTROLPLANE_KUBE_SCHEDULER_ENABLED=%v\n", cp.ControlplaneKubeScheduler))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CONTROLPLANE_APISERVER_ENABLED=%v\n", cp.ControlplaneApiserver))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CONTROLPLANE_CLUSTER_AUTOSCALER_ENABLED=%v\n", cp.ControlplaneClusterAutoscaler))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED=%v\n", cp.ControlplaneEtcd))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", cp.NoDefaultsEnabled))

	return nil
}

// ConfigureDefaultScrapeSettings processes the configuration and writes it to a file.
func (c *Configurator) ConfigureDefaultScrapeSettings(parsedData map[string]map[string]string) {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Printf("ConfigureDefaultScrapeSettings getenv:configSchemaVersion: %s\n", configSchemaVersion)

	fmt.Printf("Start prometheus-collector-settings Processing\n")

	// Load default settings based on the schema version
	var defaultSettings map[string]string
	var err error
	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		defaultSettings, err = c.ConfigLoader.ParseConfigMapForDefaultScrapeSettings(parsedData, configSchemaVersion)
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
		// Replace all non-alphanumeric characters with "_"
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

// TomlparserCCPDefaultScrapeSettings initializes the configurator and processes the configuration.
func tomlparserCCPDefaultScrapeSettings(parsedData map[string]map[string]string) {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/default-scrape-settings-enabled"},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var",
		ConfigParser:   &ConfigProcessor{},
	}

	fmt.Println("Start ccp-default-scrape-settings Processing")
	configurator.ConfigureDefaultScrapeSettings(parsedData)
	fmt.Println("End ccp-default-scrape-settings Processing")
}
