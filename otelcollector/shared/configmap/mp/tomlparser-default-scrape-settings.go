package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (fcl *FilesystemConfigLoader) SetDefaultScrapeSettings() (map[string]string, error) {
	config := make(map[string]string)
	// Set default values
	config["kubelet"] = "true"
	config["coredns"] = "false"
	config["cadvisor"] = "true"
	config["kubeproxy"] = "false"
	config["apiserver"] = "false"
	config["kubestate"] = "true"
	config["nodeexporter"] = "true"
	config["prometheuscollectorhealth"] = "false"
	config["windowsexporter"] = "false"
	config["windowskubeproxy"] = "false"
	config["kappiebasic"] = "true"
	config["networkobservabilityRetina"] = "true"
	config["networkobservabilityHubble"] = "true"
	config["networkobservabilityCilium"] = "true"
	config["noDefaultsEnabled"] = "false"
	config["acstor-capacity-provisioner"] = "true"
	config["acstor-metrics-exporter"] = "true"

	return config, nil
}

func (fcl *FilesystemConfigLoader) ParseConfigMapForDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string, schemaVersion string) (map[string]string, error) {
	config := make(map[string]string)
	// Set default values
	config["kubelet"] = "true"
	config["coredns"] = "false"
	config["cadvisor"] = "true"
	config["kubeproxy"] = "false"
	config["apiserver"] = "false"
	config["kubestate"] = "true"
	config["nodeexporter"] = "true"
	config["prometheuscollectorhealth"] = "false"
	config["windowsexporter"] = "false"
	config["windowskubeproxy"] = "false"
	config["kappiebasic"] = "true"
	config["networkobservabilityRetina"] = "true"
	config["networkobservabilityHubble"] = "true"
	config["networkobservabilityCilium"] = "true"
	config["noDefaultsEnabled"] = "false"
	config["acstor-capacity-provisioner"] = "true"
	config["acstor-metrics-exporter"] = "true"

	configSectionName := "default-scrape-settings-enabled"
	if schemaVersion == "v2" {
		configSectionName = "default-targets-scrape-enabled"
	}
	// Override defaults with values from metricsConfigBySection
	if settings, ok := metricsConfigBySection[configSectionName]; ok {
		for key, value := range settings {
			if _, ok := config[key]; ok {
				config[key] = value
			}
		}
	}

	fmt.Println("Using configmap for default scrape settings...")
	return config, nil
}

func (cp *ConfigProcessor) PopulateSettingValues(parsedConfig map[string]string) {
	if val, ok := parsedConfig["kubelet"]; ok && val != "" {
		cp.Kubelet = val
		fmt.Printf("config::Using scrape settings for kubelet: %v\n", cp.Kubelet)
	}

	if val, ok := parsedConfig["coredns"]; ok && val != "" {
		cp.Coredns = val
		fmt.Printf("config::Using scrape settings for coredns: %v\n", cp.Coredns)
	}

	if val, ok := parsedConfig["cadvisor"]; ok && val != "" {
		cp.Cadvisor = val
		fmt.Printf("config::Using scrape settings for cadvisor: %v\n", cp.Cadvisor)
	}

	if val, ok := parsedConfig["kubeproxy"]; ok && val != "" {
		cp.Kubeproxy = val
		fmt.Printf("config::Using scrape settings for kubeproxy: %v\n", cp.Kubeproxy)
	}

	if val, ok := parsedConfig["apiserver"]; ok && val != "" {
		cp.Apiserver = val
		fmt.Printf("config::Using scrape settings for apiserver: %v\n", cp.Apiserver)
	}

	if val, ok := parsedConfig["kubestate"]; ok && val != "" {
		cp.Kubestate = val
		fmt.Printf("config::Using scrape settings for kubestate: %v\n", cp.Kubestate)
	}

	if val, ok := parsedConfig["nodeexporter"]; ok && val != "" {
		cp.NodeExporter = val
		fmt.Printf("config::Using scrape settings for nodeexporter: %v\n", cp.NodeExporter)
	}

	if val, ok := parsedConfig["prometheuscollectorhealth"]; ok && val != "" {
		cp.PrometheusCollectorHealth = val
		fmt.Printf("config::Using scrape settings for prometheuscollectorhealth: %v\n", cp.PrometheusCollectorHealth)
	}

	if val, ok := parsedConfig["windowsexporter"]; ok && val != "" {
		cp.Windowsexporter = val
		fmt.Printf("config::Using scrape settings for windowsexporter: %v\n", cp.Windowsexporter)
	}

	if val, ok := parsedConfig["windowskubeproxy"]; ok && val != "" {
		cp.Windowskubeproxy = val
		fmt.Printf("config::Using scrape settings for windowskubeproxy: %v\n", cp.Windowskubeproxy)
	}

	if val, ok := parsedConfig["kappiebasic"]; ok && val != "" {
		cp.Kappiebasic = val
		fmt.Printf("config::Using scrape settings for kappiebasic: %v\n", cp.Kappiebasic)
	}

	if val, ok := parsedConfig["networkobservabilityRetina"]; ok && val != "" {
		cp.NetworkObservabilityRetina = val
		fmt.Printf("config::Using scrape settings for networkobservabilityRetina: %v\n", cp.NetworkObservabilityRetina)
	}

	if val, ok := parsedConfig["networkobservabilityHubble"]; ok && val != "" {
		cp.NetworkObservabilityHubble = val
		fmt.Printf("config::Using scrape settings for networkobservabilityHubble: %v\n", cp.NetworkObservabilityHubble)
	}

	if val, ok := parsedConfig["networkobservabilityCilium"]; ok && val != "" {
		cp.NetworkObservabilityCilium = val
		fmt.Printf("config::Using scrape settings for networkobservabilityCilium: %v\n", cp.NetworkObservabilityCilium)
	}

	if val, ok := parsedConfig["acstor-capacity-provisioner"]; ok && val != "" {
		cp.AcstorCapacityProvisioner = val
		fmt.Printf("config:: Using scrape settings for acstor-capacity-provisioner: %v\n", cp.AcstorCapacityProvisioner)
	}

	if val, ok := parsedConfig["acstor-metrics-exporter"]; ok && val != "" {
		cp.AcstorMetricsExporter = val
		fmt.Printf("config:: Using scrape settings for acstor-metrics-exporter: %v\n", cp.AcstorMetricsExporter)
	}

	if os.Getenv("MODE") == "" && strings.ToLower(strings.TrimSpace(os.Getenv("MODE"))) == "advanced" {
		controllerType := os.Getenv("CONTROLLER_TYPE")
		if controllerType == "ReplicaSet" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" &&
			cp.Kubelet == "" && cp.Cadvisor == "" &&
			cp.NodeExporter == "" && cp.PrometheusCollectorHealth == "" && cp.Kappiebasic == "" {
			cp.NoDefaultsEnabled = true
		}
	} else if cp.Kubelet == "" && cp.Cadvisor == "" &&
		cp.NodeExporter == "" && cp.PrometheusCollectorHealth == "" && cp.Kappiebasic == "" {
		cp.NoDefaultsEnabled = true
	}

	if cp.NoDefaultsEnabled {
		fmt.Printf("No default scrape configs enabled")
	}
}

func (fcw *FileConfigWriter) WriteDefaultScrapeSettingsToFile(filename string, cp *ConfigProcessor) error {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("Exception while opening file for writing prometheus-collector config environment variables: %s", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED=%v\n", cp.Kubelet))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED=%v\n", cp.Coredns))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED=%v\n", cp.Cadvisor))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED=%v\n", cp.Kubeproxy))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED=%v\n", cp.Apiserver))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED=%v\n", cp.Kubestate))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED=%v\n", cp.NodeExporter))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED=%v\n", cp.PrometheusCollectorHealth))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED=%v\n", cp.Windowsexporter))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED=%v\n", cp.Windowskubeproxy))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED=%v\n", cp.Kappiebasic))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED=%v\n", cp.NetworkObservabilityRetina))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED=%v\n", cp.NetworkObservabilityHubble))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED=%v\n", cp.NetworkObservabilityCilium))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED=%v\n", cp.NoDefaultsEnabled))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_ACSTORCAPACITYPROVISIONER_SCRAPING_ENABLED=%v\n", cp.AcstorCapacityProvisioner))
	file.WriteString(fmt.Sprintf("AZMON_PROMETHEUS_ACSTORMETRICSEXPORTER_SCRAPING_ENABLED=%v\n", cp.AcstorMetricsExporter))

	return nil
}

func (c *Configurator) ConfigureDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string) {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	fmt.Printf("Start prometheus-collector-settings Processing\n")

	// Load default settings based on the schema version
	var defaultSettings map[string]string
	var err error
	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		defaultSettings, err = c.ConfigLoader.ParseConfigMapForDefaultScrapeSettings(metricsConfigBySection, configSchemaVersion)
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

func tomlparserDefaultScrapeSettings(metricsConfigBySection map[string]map[string]string) {

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	configLoaderPath := defaultSettingsMountPath
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v2" {
		configLoaderPath = defaultSettingsMountPathv2
	}

	configurator := &Configurator{
		ConfigLoader:   &FilesystemConfigLoader{ConfigMapMountPath: configLoaderPath},
		ConfigWriter:   &FileConfigWriter{},
		ConfigFilePath: defaultSettingsEnvVarPath,
		ConfigParser:   &ConfigProcessor{},
	}

	configurator.ConfigureDefaultScrapeSettings(metricsConfigBySection)
}
