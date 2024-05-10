package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

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

func (fcl *FilesystemConfigLoader) ParseConfigMapForDefaultScrapeSettings() (map[string]string, error) {
	config := make(map[string]string)
	// Set default values
	config["controlplane-apiserver"] = "true"
	config["controlplane-cluster-autoscaler"] = "false"
	config["controlplane-kube-scheduler"] = "false"
	config["controlplane-kube-controller-manager"] = "false"
	config["controlplane-etcd"] = "true"

	if _, err := os.Stat(fcl.ConfigMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap for ccp default scrape settings not mounted, using defaults")
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
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	fmt.Println("using configmap for ccp scrape settings...")
	return config, nil
}

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

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %s", err)
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

func (c *Configurator) ConfigureDefaultScrapeSettings() {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	fmt.Printf("Start prometheus-collector-settings Processing\n")

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings, err := c.ConfigLoader.ParseConfigMapForDefaultScrapeSettings()
		if err == nil && len(configMapSettings) > 0 {
			c.ConfigParser.PopulateSettingValues(configMapSettings)
		}
	} else {
		defaultSettings, err := c.ConfigLoader.SetDefaultScrapeSettings()
		if err == nil && len(defaultSettings) > 0 {
			c.ConfigParser.PopulateSettingValues(defaultSettings)
		}
		fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
	}

	if mac := os.Getenv("MAC"); mac != "" && strings.TrimSpace(mac) == "true" {
		clusterArray := strings.Split(strings.TrimSpace(os.Getenv("CLUSTER")), "/")
		c.ConfigParser.ClusterAlias = clusterArray[len(clusterArray)-1]
	} else {
		c.ConfigParser.ClusterAlias = os.Getenv("CLUSTER")
	}

	if c.ConfigParser.ClusterAlias != "" && len(c.ConfigParser.ClusterAlias) > 0 {
		// replace all non alpha-numeric characters with "_"  -- this is to ensure that all down stream places where this is used (like collector, telegraf config etc are keeping up with sanity)
		c.ConfigParser.ClusterAlias = regexp.MustCompile(`[^0-9a-zA-Z]+`).ReplaceAllString(c.ConfigParser.ClusterAlias, "_")
		c.ConfigParser.ClusterAlias = strings.Trim(c.ConfigParser.ClusterAlias, "_")
		fmt.Printf("After replacing non-alpha-numeric characters with '_': %s\n", c.ConfigParser.ClusterAlias)
	}

	err := c.ConfigWriter.WriteDefaultScrapeSettingsToFile(c.ConfigFilePath, c.ConfigParser)
	if err != nil {
		fmt.Printf("%v\n", err)
		return
	}

	fmt.Printf("End prometheus-collector-settings Processing\n")
}

func tomlparserCCPDefaultScrapeSettings() {
	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: "/etc/config/settings/default-scrape-settings-enabled"},
		ConfigWriter:   &FileConfigWriter{Config: map[string]string{}},
		ConfigFilePath: "/opt/microsoft/configmapparser/config_default_scrape_settings_env_var",
		ConfigParser:   &ConfigProcessor{},
	}

	fmt.Println("Start ccp-default-scrape-settings Processing")
	configurator.ConfigureDefaultScrapeSettings()
	fmt.Println("End ccp-default-scrape-settings Processing")
}
