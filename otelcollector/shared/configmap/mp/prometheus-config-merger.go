package configmapsettings

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/prometheus-collector/shared"

	"gopkg.in/yaml.v2"
)

const (
	configMapMountPath               = "/etc/config/settings/prometheus/prometheus-config"
	replicasetControllerType         = "replicaset"
	daemonsetControllerType          = "daemonset"
	configReaderSidecarContainerType = "configreadersidecar"
	supportedSchemaVersion           = true
	sendDSUpMetric                   = false
)

var (
	regexHash    = make(map[string]string)
	intervalHash = make(map[string]string)
)

var mergedDefaultConfigs map[interface{}]interface{}

func parseConfigMap() string {
	defer func() {
		if r := recover(); r != nil {
			shared.EchoError(fmt.Sprintf("Recovered from panic: %v\n", r))
		}
	}()

	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		shared.EchoWarning("Custom prometheus config does not exist, using only default scrape targets if they are enabled")
		return ""
	}

	config, err := os.ReadFile(configMapMountPath)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Exception while parsing configmap for prometheus config: %s. Custom prometheus config will not be used. Please check configmap for errors", err))
		return ""
	}

	// shared.EchoVar("Successfully parsed configmap for prometheus config", string(config))
	return string(config)
}

func loadRegexHash() {
	data, err := os.ReadFile(regexHashFile)
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
		return
	}

	err = yaml.Unmarshal(data, &regexHash)
	if err != nil {
		fmt.Printf("Exception in loadRegexHash for prometheus config: %v. Keep list regexes will not be used\n", err)
	}
}

func loadIntervalHash() {
	data, err := os.ReadFile(intervalHashFile)
	if err != nil {
		fmt.Printf("Exception in loadIntervalHash for prometheus config: %v. Scrape interval will not be used\n", err)
		return
	}

	err = yaml.Unmarshal(data, &intervalHash)
	if err != nil {
		fmt.Printf("Exception in loadIntervalHash for prometheus config: %v. Scrape interval will not be used\n", err)
	}
}

func isConfigReaderSidecar() bool {
	containerType := os.Getenv("CONTAINER_TYPE")
	if containerType != "" {
		currentContainerType := strings.ToLower(strings.TrimSpace(containerType))
		if currentContainerType == configReaderSidecarContainerType {
			return true
		}
	}
	return false
}

func UpdateScrapeIntervalConfig(yamlConfigFile string, scrapeIntervalSetting string) {
	fmt.Printf("Updating scrape interval config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("Error unmarshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
		return
	}

	// Update scrape interval config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				fmt.Printf("scrapeInterval %s\n", scrapeIntervalSetting)
				scfgMap["scrape_interval"] = scrapeIntervalSetting
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithScrapeConfig, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML for %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = os.WriteFile(yamlConfigFile, []byte(cfgYamlWithScrapeConfig), fs.FileMode(0644))
		if err != nil {
			fmt.Printf("Error writing to file %s: %v. The scrape interval will not be updated\n", yamlConfigFile, err)
			return
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML. The scrape interval will not be updated.\n")
	}
}

func AppendMetricRelabelConfig(yamlConfigFile, keepListRegex string) error {
	fmt.Printf("Starting to append keep list regex or minimal ingestion regex to %s\n", yamlConfigFile)

	content, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		return fmt.Errorf("error reading config file %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("error unmarshalling YAML for %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	keepListMetricRelabelConfig := map[string]interface{}{
		"source_labels": []interface{}{"__name__"},
		"action":        "keep",
		"regex":         keepListRegex,
	}

	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for i, scfg := range scrapeConfigs {
			// Ensure scfg is a map with string keys
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				// Convert to map[string]interface{}
				stringScfgMap := make(map[string]interface{})
				for k, v := range scfgMap {
					if key, ok := k.(string); ok {
						stringScfgMap[key] = v
					} else {
						return fmt.Errorf("encountered non-string key in scrape config map: %v", k)
					}
				}

				// Update or add metric_relabel_configs
				if metricRelabelCfgs, ok := stringScfgMap["metric_relabel_configs"].([]interface{}); ok {
					stringScfgMap["metric_relabel_configs"] = append(metricRelabelCfgs, keepListMetricRelabelConfig)
				} else {
					stringScfgMap["metric_relabel_configs"] = []interface{}{keepListMetricRelabelConfig}
				}

				// Convert back to map[interface{}]interface{} for YAML marshalling
				interfaceScfgMap := make(map[interface{}]interface{})
				for k, v := range stringScfgMap {
					interfaceScfgMap[k] = v
				}

				// Update the scrape_configs list
				scrapeConfigs[i] = interfaceScfgMap
			}
		}

		// Write updated scrape_configs back to config
		config["scrape_configs"] = scrapeConfigs
	} else {
		fmt.Println("No 'scrape_configs' found in the YAML. Skipping updates.")
		return nil
	}

	cfgYamlWithMetricRelabelConfig, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshalling YAML for %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	if err := os.WriteFile(yamlConfigFile, cfgYamlWithMetricRelabelConfig, os.ModePerm); err != nil {
		return fmt.Errorf("error writing to file %s: %v. The keep list regex will not be used", yamlConfigFile, err)
	}

	return nil
}

func AppendRelabelConfig(yamlConfigFile string, relabelConfig []map[string]interface{}, keepRegex string) {
	fmt.Printf("Adding relabel config for %s\n", yamlConfigFile)

	// Read YAML config file
	data, err := os.ReadFile(yamlConfigFile)
	if err != nil {
		fmt.Printf("Error reading config file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Unmarshal YAML data
	var config map[string]interface{}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("Error unmarshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
		return
	}

	// Append relabel config for keep list to each scrape config
	if scrapeConfigs, ok := config["scrape_configs"].([]interface{}); ok {
		for _, scfg := range scrapeConfigs {
			if scfgMap, ok := scfg.(map[interface{}]interface{}); ok {
				relabelCfgs, exists := scfgMap["relabel_configs"]
				if !exists {
					scfgMap["relabel_configs"] = relabelConfig
				} else if relabelCfgsSlice, ok := relabelCfgs.([]interface{}); ok {
					for _, rc := range relabelConfig {
						relabelCfgsSlice = append(relabelCfgsSlice, rc)
					}
					scfgMap["relabel_configs"] = relabelCfgsSlice
				}
			}
		}

		// Marshal updated config back to YAML
		cfgYamlWithRelabelConfig, err := yaml.Marshal(config)
		if err != nil {
			fmt.Printf("Error marshalling YAML for %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}

		// Write updated YAML back to file
		err = os.WriteFile(yamlConfigFile, []byte(cfgYamlWithRelabelConfig), fs.FileMode(0644))
		if err != nil {
			fmt.Printf("Error writing to file %s: %v. The relabel config will not be added\n", yamlConfigFile, err)
			return
		}
	} else {
		fmt.Printf("No 'scrape_configs' found in the YAML. The relabel config will not be added.\n")
	}
}

func populateDefaultPrometheusConfig() {
	defaultConfigs := []string{}
	currentControllerType := strings.TrimSpace(strings.ToLower(os.Getenv("CONTROLLER_TYPE")))

	// Default values
	advancedMode := false
	windowsDaemonset := false

	// Get current mode (advanced or not...)
	currentMode := strings.TrimSpace(strings.ToLower(os.Getenv("MODE")))
	if currentMode == "advanced" {
		advancedMode = true
	}

	// Get if windowsdaemonset is enabled or not (i.e., WINMODE env = advanced or not...)
	winMode := strings.TrimSpace(strings.ToLower(os.Getenv("WINMODE")))
	if winMode == "advanced" {
		windowsDaemonset = true
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		kubeletMetricsKeepListRegex, exists := regexHash["KUBELET_METRICS_KEEP_LIST_REGEX"]
		kubeletScrapeInterval := intervalHash["KUBELET_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			if !advancedMode {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsSimple, kubeletScrapeInterval)
				if exists && kubeletMetricsKeepListRegex != "" {
					fmt.Printf("Using regex for Kubelet: %s\n", kubeletMetricsKeepListRegex)
					AppendMetricRelabelConfig(kubeletDefaultFileRsSimple, kubeletMetricsKeepListRegex)
				}
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsSimple)
			} else if windowsDaemonset && sendDSUpMetric {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsAdvancedWindowsDaemonset, kubeletScrapeInterval)
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsAdvancedWindowsDaemonset)
			} else if sendDSUpMetric {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsAdvanced, kubeletScrapeInterval)
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsAdvanced)
			}
		} else {
			if advancedMode && (windowsDaemonset || strings.ToLower(os.Getenv("OS_TYPE")) == "linux") {
				UpdateScrapeIntervalConfig(kubeletDefaultFileDs, kubeletScrapeInterval)
				if exists && kubeletMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(kubeletDefaultFileDs, kubeletMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(kubeletDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$OS_TYPE$$", os.Getenv("OS_TYPE")))
					err = os.WriteFile(kubeletDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, kubeletDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		corednsMetricsKeepListRegex, exists := regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"]
		corednsScrapeInterval, intervalExists := intervalHash["COREDNS_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(coreDNSDefaultFile, corednsScrapeInterval)
		}
		if exists && corednsMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(coreDNSDefaultFile, corednsMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, coreDNSDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		cadvisorMetricsKeepListRegex, exists := regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"]
		cadvisorScrapeInterval, intervalExists := intervalHash["CADVISOR_SCRAPE_INTERVAL"]
		if intervalExists {
			if currentControllerType == replicasetControllerType {
				if !advancedMode {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileRsSimple, cadvisorScrapeInterval)
					if exists && cadvisorMetricsKeepListRegex != "" {
						AppendMetricRelabelConfig(cadvisorDefaultFileRsSimple, cadvisorMetricsKeepListRegex)
					}
					defaultConfigs = append(defaultConfigs, cadvisorDefaultFileRsSimple)
				} else if sendDSUpMetric {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileRsAdvanced, cadvisorScrapeInterval)
					defaultConfigs = append(defaultConfigs, cadvisorDefaultFileRsAdvanced)
				}
			} else {
				if advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileDs, cadvisorScrapeInterval)
					if exists && cadvisorMetricsKeepListRegex != "" {
						AppendMetricRelabelConfig(cadvisorDefaultFileDs, cadvisorMetricsKeepListRegex)
					}
					contents, err := os.ReadFile(cadvisorDefaultFileDs)
					if err == nil {
						contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
						contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
						err = os.WriteFile(cadvisorDefaultFileDs, contents, 0644)
						if err == nil {
							defaultConfigs = append(defaultConfigs, cadvisorDefaultFileDs)
						}
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		kubeproxyMetricsKeepListRegex, exists := regexHash["KUBEPROXY_METRICS_KEEP_LIST_REGEX"]
		kubeproxyScrapeInterval, intervalExists := intervalHash["KUBEPROXY_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(kubeProxyDefaultFile, kubeproxyScrapeInterval)
		}
		if exists && kubeproxyMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(kubeProxyDefaultFile, kubeproxyMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, kubeProxyDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		apiserverMetricsKeepListRegex, exists := regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"]
		apiserverScrapeInterval, intervalExists := intervalHash["APISERVER_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(apiserverDefaultFile, apiserverScrapeInterval)
		}
		if exists && apiserverMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(apiserverDefaultFile, apiserverMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, apiserverDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		kubestateMetricsKeepListRegex, exists := regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"]
		kubestateScrapeInterval, intervalExists := intervalHash["KUBESTATE_SCRAPE_INTERVAL"]
		log.Printf("path %s: %s\n", "kubeStateDefaultFile", kubeStateDefaultFile)

		if intervalExists {
			UpdateScrapeIntervalConfig(kubeStateDefaultFile, kubestateScrapeInterval)
		}
		if exists && kubestateMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(kubeStateDefaultFile, kubestateMetricsKeepListRegex)
		}
		contents, err := os.ReadFile(kubeStateDefaultFile)
		if err == nil {
			contents = []byte(strings.ReplaceAll(string(contents), "$$KUBE_STATE_NAME$$", os.Getenv("KUBE_STATE_NAME")))
			contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
			err = os.WriteFile(kubeStateDefaultFile, contents, 0644)
			if err == nil {
				defaultConfigs = append(defaultConfigs, kubeStateDefaultFile)
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		nodeexporterMetricsKeepListRegex, exists := regexHash["NODEEXPORTER_METRICS_KEEP_LIST_REGEX"]
		nodeexporterScrapeInterval := intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			if advancedMode && sendDSUpMetric {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileRsAdvanced, nodeexporterScrapeInterval)
				contents, err := os.ReadFile(nodeExporterDefaultFileRsAdvanced)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_NAME$$", os.Getenv("NODE_EXPORTER_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
					err = os.WriteFile(nodeExporterDefaultFileRsAdvanced, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileRsAdvanced)
					}
				}
			} else if !advancedMode {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileRsSimple, nodeexporterScrapeInterval)
				if exists && nodeexporterMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(nodeExporterDefaultFileRsSimple, nodeexporterMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(nodeExporterDefaultFileRsSimple)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_NAME$$", os.Getenv("NODE_EXPORTER_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
					err = os.WriteFile(nodeExporterDefaultFileRsSimple, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileRsSimple)
					}
				}
			}
		} else {
			if advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileDs, nodeexporterScrapeInterval)
				if exists && nodeexporterMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(nodeExporterDefaultFileDs, nodeexporterMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(nodeExporterDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_TARGETPORT$$", os.Getenv("NODE_EXPORTER_TARGETPORT")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(nodeExporterDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		kappiebasicMetricsKeepListRegex, exists := regexHash["KAPPIEBASIC_METRICS_KEEP_LIST_REGEX"]
		kappiebasicScrapeInterval := intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			// Do nothing - Kappie is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" {
				UpdateScrapeIntervalConfig(kappieBasicDefaultFileDs, kappiebasicScrapeInterval)
				if exists && kappiebasicMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(kappieBasicDefaultFileDs, kappiebasicMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(kappieBasicDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(kappieBasicDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, kappieBasicDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityRetinaMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityRetinaScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Retina is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityRetinaDefaultFileDs, networkobservabilityRetinaScrapeInterval)
				}
				if exists && networkobservabilityRetinaMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityRetinaDefaultFileDs, networkobservabilityRetinaMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityRetinaDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityRetinaDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityRetinaDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityHubbleMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityHubbleScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Hubble is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityHubbleDefaultFileDs, networkobservabilityHubbleScrapeInterval)
				}
				if exists && networkobservabilityHubbleMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityHubbleDefaultFileDs, networkobservabilityHubbleMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityHubbleDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityHubbleDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityHubbleDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityCiliumMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityCiliumScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Cilium is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityCiliumDefaultFileDs, networkobservabilityCiliumScrapeInterval)
				}
				if exists && networkobservabilityCiliumMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityCiliumDefaultFileDs, networkobservabilityCiliumMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityCiliumDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityCiliumDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityCiliumDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		prometheusCollectorHealthInterval, intervalExists := intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(prometheusCollectorHealthDefaultFile, prometheusCollectorHealthInterval)
		}
		defaultConfigs = append(defaultConfigs, prometheusCollectorHealthDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		winexporterMetricsKeepListRegex, exists := regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"]
		windowsexporterScrapeInterval, intervalExists := intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType && !advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsExporterDefaultRsSimpleFile, windowsexporterScrapeInterval)
			}
			if exists && winexporterMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsExporterDefaultRsSimpleFile, winexporterMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsExporterDefaultRsSimpleFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsExporterDefaultRsSimpleFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsExporterDefaultRsSimpleFile)
				}
			}
		} else if currentControllerType == daemonsetControllerType && advancedMode && windowsDaemonset && strings.ToLower(os.Getenv("OS_TYPE")) == "windows" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsExporterDefaultDsFile, windowsexporterScrapeInterval)
			}
			if exists && winexporterMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsExporterDefaultDsFile, winexporterMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsExporterDefaultDsFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsExporterDefaultDsFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsExporterDefaultDsFile)
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		winkubeproxyMetricsKeepListRegex, exists := regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"]
		windowskubeproxyScrapeInterval, intervalExists := intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"]
		if currentControllerType == replicasetControllerType && !advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsKubeProxyDefaultFileRsSimpleFile, windowskubeproxyScrapeInterval)
			}
			if exists && winkubeproxyMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsKubeProxyDefaultFileRsSimpleFile, winkubeproxyMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsKubeProxyDefaultFileRsSimpleFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsKubeProxyDefaultFileRsSimpleFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsKubeProxyDefaultFileRsSimpleFile)
				}
			}
		} else if currentControllerType == daemonsetControllerType && advancedMode && windowsDaemonset && strings.ToLower(os.Getenv("OS_TYPE")) == "windows" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsKubeProxyDefaultDsFile, windowskubeproxyScrapeInterval)
			}
			if exists && winkubeproxyMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsKubeProxyDefaultDsFile, winkubeproxyMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsKubeProxyDefaultDsFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsKubeProxyDefaultDsFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsKubeProxyDefaultDsFile)
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		if podannotationNamespacesRegex, exists := os.LookupEnv("AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"); exists {
			podannotationMetricsKeepListRegex := regexHash["POD_ANNOTATION_METRICS_KEEP_LIST_REGEX"]
			podannotationScrapeInterval, intervalExists := intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"]

			if intervalExists {
				UpdateScrapeIntervalConfig(podAnnotationsDefaultFile, podannotationScrapeInterval)
			}
			if podannotationMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(podAnnotationsDefaultFile, podannotationMetricsKeepListRegex)
			}
			// Trim the first and last escaped quotes if they exist
			if len(podannotationNamespacesRegex) > 1 && podannotationNamespacesRegex[0] == '"' && podannotationNamespacesRegex[len(podannotationNamespacesRegex)-1] == '"' {
				podannotationNamespacesRegex = podannotationNamespacesRegex[1 : len(podannotationNamespacesRegex)-1]
			}
			// Additional trim to remove single quotes if present
			podannotationNamespacesRegex = strings.Trim(podannotationNamespacesRegex, "'")

			if podannotationNamespacesRegex != "" {
				relabelConfig := []map[string]interface{}{
					{"source_labels": []string{"__meta_kubernetes_namespace"}, "action": "keep", "regex": podannotationNamespacesRegex},
				}
				AppendRelabelConfig(podAnnotationsDefaultFile, relabelConfig, podannotationNamespacesRegex)
			}
			defaultConfigs = append(defaultConfigs, podAnnotationsDefaultFile)
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_ACSTORCAPACITYPROVISIONER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		acstorCapacityProvisionerKeepListRegex, exists := regexHash["ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX"]
		acstorCapacityProvisionerScrapeInterval, intervalExists := intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(acstorCapacityProvisionerDefaultFile, acstorCapacityProvisionerScrapeInterval)
		}
		if exists && acstorCapacityProvisionerKeepListRegex != "" {
			AppendMetricRelabelConfig(acstorCapacityProvisionerDefaultFile, acstorCapacityProvisionerKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, acstorCapacityProvisionerDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_ACSTORMETRICSEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
		acstorMetricsExporterKeepListRegex, exists := regexHash["ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX"]
		acstorMetricsExporterScrapeInterval, intervalExists := intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(acstorMetricsExporterDefaultFile, acstorMetricsExporterScrapeInterval)
		}
		if exists && acstorMetricsExporterKeepListRegex != "" {
			AppendMetricRelabelConfig(acstorMetricsExporterDefaultFile, acstorMetricsExporterKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, acstorMetricsExporterDefaultFile)
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
	// if mergedDefaultConfigs != nil {
	// 	fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	// }
}

func populateDefaultPrometheusConfigWithOperator() {
	defaultConfigs := []string{}

	envControllerType := os.Getenv("CONTROLLER_TYPE")
	currentControllerType := ""
	if envControllerType != "" {
		currentControllerType = strings.TrimSpace(strings.ToLower(envControllerType))
	}

	// Default values
	advancedMode := false
	windowsDaemonset := false

	envMode := os.Getenv("MODE")
	currentMode := "default"
	if envMode != "" {
		currentMode = strings.TrimSpace(strings.ToLower(envMode))
	}
	if currentMode == "advanced" {
		advancedMode = true
	}

	// Get if windowsdaemonset is enabled or not (i.e., WINMODE env = advanced or not...)
	winMode := "default"
	if envWinMode := os.Getenv("WINMODE"); envWinMode != "" {
		winMode = strings.TrimSpace(strings.ToLower(envWinMode))
	}
	if winMode == "advanced" {
		windowsDaemonset = true
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBELET_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		kubeletMetricsKeepListRegex, exists := regexHash["KUBELET_METRICS_KEEP_LIST_REGEX"]
		kubeletScrapeInterval := intervalHash["KUBELET_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			if !advancedMode {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsSimple, kubeletScrapeInterval)
				if exists && kubeletMetricsKeepListRegex != "" {
					fmt.Printf("Using regex for Kubelet: %s\n", kubeletMetricsKeepListRegex)
					AppendMetricRelabelConfig(kubeletDefaultFileRsSimple, kubeletMetricsKeepListRegex)
				}
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsSimple)
			} else if windowsDaemonset && sendDSUpMetric {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsAdvancedWindowsDaemonset, kubeletScrapeInterval)
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsAdvancedWindowsDaemonset)
			} else if sendDSUpMetric {
				UpdateScrapeIntervalConfig(kubeletDefaultFileRsAdvanced, kubeletScrapeInterval)
				defaultConfigs = append(defaultConfigs, kubeletDefaultFileRsAdvanced)
			}
		} else {
			if advancedMode && currentControllerType == daemonsetControllerType && (windowsDaemonset || strings.ToLower(os.Getenv("OS_TYPE")) == "linux") {
				UpdateScrapeIntervalConfig(kubeletDefaultFileDs, kubeletScrapeInterval)
				if exists && kubeletMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(kubeletDefaultFileDs, kubeletMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(kubeletDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$OS_TYPE$$", os.Getenv("OS_TYPE")))
					err = os.WriteFile(kubeletDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, kubeletDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_COREDNS_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		corednsMetricsKeepListRegex, exists := regexHash["COREDNS_METRICS_KEEP_LIST_REGEX"]
		corednsScrapeInterval, intervalExists := intervalHash["COREDNS_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(coreDNSDefaultFile, corednsScrapeInterval)
		}
		if exists && corednsMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(coreDNSDefaultFile, corednsMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, coreDNSDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CADVISOR_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		cadvisorMetricsKeepListRegex, exists := regexHash["CADVISOR_METRICS_KEEP_LIST_REGEX"]
		cadvisorScrapeInterval, intervalExists := intervalHash["CADVISOR_SCRAPE_INTERVAL"]
		if intervalExists {
			if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
				if !advancedMode {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileRsSimple, cadvisorScrapeInterval)
					if exists && cadvisorMetricsKeepListRegex != "" {
						AppendMetricRelabelConfig(cadvisorDefaultFileRsSimple, cadvisorMetricsKeepListRegex)
					}
					defaultConfigs = append(defaultConfigs, cadvisorDefaultFileRsSimple)
				} else if sendDSUpMetric {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileRsAdvanced, cadvisorScrapeInterval)
					defaultConfigs = append(defaultConfigs, cadvisorDefaultFileRsAdvanced)
				}
			} else {
				if advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" && currentControllerType == daemonsetControllerType {
					UpdateScrapeIntervalConfig(cadvisorDefaultFileDs, cadvisorScrapeInterval)
					if exists && cadvisorMetricsKeepListRegex != "" {
						AppendMetricRelabelConfig(cadvisorDefaultFileDs, cadvisorMetricsKeepListRegex)
					}
					contents, err := os.ReadFile(cadvisorDefaultFileDs)
					if err == nil {
						contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
						contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
						err = os.WriteFile(cadvisorDefaultFileDs, contents, 0644)
						if err == nil {
							defaultConfigs = append(defaultConfigs, cadvisorDefaultFileDs)
						}
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBEPROXY_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		kubeproxyMetricsKeepListRegex, exists := regexHash["KUBEPROXY_METRICS_KEEP_LIST_REGEX"]
		kubeproxyScrapeInterval, intervalExists := intervalHash["KUBEPROXY_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(kubeProxyDefaultFile, kubeproxyScrapeInterval)
		}
		if exists && kubeproxyMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(kubeProxyDefaultFile, kubeproxyMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, kubeProxyDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_APISERVER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		apiserverMetricsKeepListRegex, exists := regexHash["APISERVER_METRICS_KEEP_LIST_REGEX"]
		apiserverScrapeInterval, intervalExists := intervalHash["APISERVER_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(apiserverDefaultFile, apiserverScrapeInterval)
		}
		if exists && apiserverMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(apiserverDefaultFile, apiserverMetricsKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, apiserverDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KUBESTATE_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		kubestateMetricsKeepListRegex, exists := regexHash["KUBESTATE_METRICS_KEEP_LIST_REGEX"]
		kubestateScrapeInterval, intervalExists := intervalHash["KUBESTATE_SCRAPE_INTERVAL"]

		if intervalExists {
			UpdateScrapeIntervalConfig(kubeStateDefaultFile, kubestateScrapeInterval)
		}
		if exists && kubestateMetricsKeepListRegex != "" {
			AppendMetricRelabelConfig(kubeStateDefaultFile, kubestateMetricsKeepListRegex)
		}
		contents, err := os.ReadFile(kubeStateDefaultFile)
		if err == nil {
			contents = []byte(strings.ReplaceAll(string(contents), "$$KUBE_STATE_NAME$$", os.Getenv("KUBE_STATE_NAME")))
			contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
			err = os.WriteFile(kubeStateDefaultFile, contents, 0644)
			if err == nil {
				defaultConfigs = append(defaultConfigs, kubeStateDefaultFile)
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NODEEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		nodeexporterMetricsKeepListRegex, exists := regexHash["NODEEXPORTER_METRICS_KEEP_LIST_REGEX"]
		nodeexporterScrapeInterval := intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			if advancedMode && sendDSUpMetric {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileRsAdvanced, nodeexporterScrapeInterval)
				contents, err := os.ReadFile(nodeExporterDefaultFileRsAdvanced)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_NAME$$", os.Getenv("NODE_EXPORTER_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
					err = os.WriteFile(nodeExporterDefaultFileRsAdvanced, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileRsAdvanced)
					}
				}
			} else if !advancedMode {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileRsSimple, nodeexporterScrapeInterval)
				if exists && nodeexporterMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(nodeExporterDefaultFileRsSimple, nodeexporterMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(nodeExporterDefaultFileRsSimple)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_NAME$$", os.Getenv("NODE_EXPORTER_NAME")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE")))
					err = os.WriteFile(nodeExporterDefaultFileRsSimple, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileRsSimple)
					}
				}
			}
		} else {
			if advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" && currentControllerType == daemonsetControllerType {
				UpdateScrapeIntervalConfig(nodeExporterDefaultFileDs, nodeexporterScrapeInterval)
				if exists && nodeexporterMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(nodeExporterDefaultFileDs, nodeexporterMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(nodeExporterDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_EXPORTER_TARGETPORT$$", os.Getenv("NODE_EXPORTER_TARGETPORT")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(nodeExporterDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, nodeExporterDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_KAPPIEBASIC_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		kappiebasicMetricsKeepListRegex, exists := regexHash["KAPPIEBASIC_METRICS_KEEP_LIST_REGEX"]
		kappiebasicScrapeInterval := intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			// Do nothing - Kappie is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if currentControllerType == daemonsetControllerType && advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" {
				UpdateScrapeIntervalConfig(kappieBasicDefaultFileDs, kappiebasicScrapeInterval)
				if exists && kappiebasicMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(kappieBasicDefaultFileDs, kappiebasicMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(kappieBasicDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(kappieBasicDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, kappieBasicDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYRETINA_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityRetinaMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYRETINA_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityRetinaScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Retina is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityRetinaDefaultFileDs, networkobservabilityRetinaScrapeInterval)
				}
				if exists && networkobservabilityRetinaMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityRetinaDefaultFileDs, networkobservabilityRetinaMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityRetinaDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityRetinaDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityRetinaDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYHUBBLE_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityHubbleMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYHUBBLE_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityHubbleScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Hubble is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityHubbleDefaultFileDs, networkobservabilityHubbleScrapeInterval)
				}
				if exists && networkobservabilityHubbleMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityHubbleDefaultFileDs, networkobservabilityHubbleMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityHubbleDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityHubbleDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityHubbleDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_NETWORKOBSERVABILITYCILIUM_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		networkobservabilityCiliumMetricsKeepListRegex, exists := regexHash["NETWORKOBSERVABILITYCILIUM_METRICS_KEEP_LIST_REGEX"]
		networkobservabilityCiliumScrapeInterval, intervalExists := intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"]
		if isConfigReaderSidecar() || currentControllerType == replicasetControllerType {
			// Do nothing - Network observability Cilium is not supported to be scrapped automatically outside ds.
			// If needed, the customer can disable this ds target and enable rs scraping through custom config map
		} else {
			if advancedMode && strings.ToLower(os.Getenv("MAC")) == "true" && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
				if intervalExists {
					UpdateScrapeIntervalConfig(networkObservabilityCiliumDefaultFileDs, networkobservabilityCiliumScrapeInterval)
				}
				if exists && networkobservabilityCiliumMetricsKeepListRegex != "" {
					AppendMetricRelabelConfig(networkObservabilityCiliumDefaultFileDs, networkobservabilityCiliumMetricsKeepListRegex)
				}
				contents, err := os.ReadFile(networkObservabilityCiliumDefaultFileDs)
				if err == nil {
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
					contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
					err = os.WriteFile(networkObservabilityCiliumDefaultFileDs, contents, 0644)
					if err == nil {
						defaultConfigs = append(defaultConfigs, networkObservabilityCiliumDefaultFileDs)
					}
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		prometheusCollectorHealthInterval, intervalExists := intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"]
		if intervalExists {
			UpdateScrapeIntervalConfig(prometheusCollectorHealthDefaultFile, prometheusCollectorHealthInterval)
		}
		defaultConfigs = append(defaultConfigs, prometheusCollectorHealthDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_WINDOWSEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		winexporterMetricsKeepListRegex, exists := regexHash["WINDOWSEXPORTER_METRICS_KEEP_LIST_REGEX"]
		windowsexporterScrapeInterval, intervalExists := intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"]
		// Not adding the isConfigReaderSidecar check instead of replicaset check since this is legacy 1P chart path and not relevant anymore.
		if currentControllerType == replicasetControllerType && !advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsExporterDefaultRsSimpleFile, windowsexporterScrapeInterval)
			}
			if exists && winexporterMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsExporterDefaultRsSimpleFile, winexporterMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsExporterDefaultRsSimpleFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsExporterDefaultRsSimpleFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsExporterDefaultRsSimpleFile)
				}
			}
		} else if currentControllerType == daemonsetControllerType && advancedMode && windowsDaemonset && strings.ToLower(os.Getenv("OS_TYPE")) == "windows" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsExporterDefaultDsFile, windowsexporterScrapeInterval)
			}
			if exists && winexporterMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsExporterDefaultDsFile, winexporterMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsExporterDefaultDsFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsExporterDefaultDsFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsExporterDefaultDsFile)
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_WINDOWSKUBEPROXY_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" {
		winkubeproxyMetricsKeepListRegex, exists := regexHash["WINDOWSKUBEPROXY_METRICS_KEEP_LIST_REGEX"]
		windowskubeproxyScrapeInterval, intervalExists := intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"]
		// Not adding the isConfigReaderSidecar check instead of replicaset check since this is legacy 1P chart path and not relevant anymore.
		if currentControllerType == replicasetControllerType && !advancedMode && strings.ToLower(os.Getenv("OS_TYPE")) == "linux" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsKubeProxyDefaultFileRsSimpleFile, windowskubeproxyScrapeInterval)
			}
			if exists && winkubeproxyMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsKubeProxyDefaultFileRsSimpleFile, winkubeproxyMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsKubeProxyDefaultFileRsSimpleFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsKubeProxyDefaultFileRsSimpleFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsKubeProxyDefaultFileRsSimpleFile)
				}
			}
		} else if currentControllerType == daemonsetControllerType && advancedMode && windowsDaemonset && strings.ToLower(os.Getenv("OS_TYPE")) == "windows" {
			if intervalExists {
				UpdateScrapeIntervalConfig(windowsKubeProxyDefaultDsFile, windowskubeproxyScrapeInterval)
			}
			if exists && winkubeproxyMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(windowsKubeProxyDefaultDsFile, winkubeproxyMetricsKeepListRegex)
			}
			contents, err := os.ReadFile(windowsKubeProxyDefaultDsFile)
			if err == nil {
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_IP$$", os.Getenv("NODE_IP")))
				contents = []byte(strings.ReplaceAll(string(contents), "$$NODE_NAME$$", os.Getenv("NODE_NAME")))
				err = os.WriteFile(windowsKubeProxyDefaultDsFile, contents, 0644)
				if err == nil {
					defaultConfigs = append(defaultConfigs, windowsKubeProxyDefaultDsFile)
				}
			}
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_POD_ANNOTATION_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		if podannotationNamespacesRegex, exists := os.LookupEnv("AZMON_PROMETHEUS_POD_ANNOTATION_NAMESPACES_REGEX"); exists {
			podannotationMetricsKeepListRegex := regexHash["POD_ANNOTATION_METRICS_KEEP_LIST_REGEX"]
			podannotationScrapeInterval, intervalExists := intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"]

			if intervalExists {
				UpdateScrapeIntervalConfig(podAnnotationsDefaultFile, podannotationScrapeInterval)
			}
			if podannotationMetricsKeepListRegex != "" {
				AppendMetricRelabelConfig(podAnnotationsDefaultFile, podannotationMetricsKeepListRegex)
			}
			// Trim the first and last escaped quotes if they exist
			if len(podannotationNamespacesRegex) > 1 && podannotationNamespacesRegex[0] == '"' && podannotationNamespacesRegex[len(podannotationNamespacesRegex)-1] == '"' {
				podannotationNamespacesRegex = podannotationNamespacesRegex[1 : len(podannotationNamespacesRegex)-1]
			}
			// Additional trim to remove single quotes if present
			podannotationNamespacesRegex = strings.Trim(podannotationNamespacesRegex, "'")

			if podannotationNamespacesRegex != "" {
				relabelConfig := []map[string]interface{}{
					{"source_labels": []string{"__meta_kubernetes_namespace"}, "action": "keep", "regex": podannotationNamespacesRegex},
				}
				AppendRelabelConfig(podAnnotationsDefaultFile, relabelConfig, podannotationNamespacesRegex)
			}
			defaultConfigs = append(defaultConfigs, podAnnotationsDefaultFile)
		}
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_ACSTORCAPACITYPROVISIONER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		acstorCapacityProvisionerKeepListRegex, exists := regexHash["ACSTORCAPACITYPROVISONER_KEEP_LIST_REGEX"]
		acstorCapacityProvisionerScrapeInterval, intervalExists := intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"]
		log.Printf("path %s: %s\n", "acstorCapacityProvisionerDefaultFile", acstorCapacityProvisionerDefaultFile)
		if intervalExists {
			UpdateScrapeIntervalConfig(acstorCapacityProvisionerDefaultFile, acstorCapacityProvisionerScrapeInterval)
		}
		if exists && acstorCapacityProvisionerKeepListRegex != "" {
			AppendMetricRelabelConfig(acstorCapacityProvisionerDefaultFile, acstorCapacityProvisionerKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, acstorCapacityProvisionerDefaultFile)
	}

	if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_ACSTORMETRICSEXPORTER_SCRAPING_ENABLED"); exists && strings.ToLower(enabled) == "true" && (isConfigReaderSidecar() || currentControllerType == replicasetControllerType) {
		acstorMetricsExporterKeepListRegex, exists := regexHash["ACSTORMETRICSEXPORTER_KEEP_LIST_REGEX"]
		acstorMetricsExporterScrapeInterval, intervalExists := intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"]
		log.Printf("path %s: %s\n", "acstorMetricsExporterDefaultFile", acstorMetricsExporterDefaultFile)
		if intervalExists {
			UpdateScrapeIntervalConfig(acstorMetricsExporterDefaultFile, acstorMetricsExporterScrapeInterval)
		}
		if exists && acstorMetricsExporterKeepListRegex != "" {
			AppendMetricRelabelConfig(acstorMetricsExporterDefaultFile, acstorMetricsExporterKeepListRegex)
		}
		defaultConfigs = append(defaultConfigs, acstorMetricsExporterDefaultFile)
	}

	mergedDefaultConfigs = mergeDefaultScrapeConfigs(defaultConfigs)
	// if mergedDefaultConfigs != nil {
	// 	fmt.Printf("Merged default scrape targets: %v\n", mergedDefaultConfigs)
	// }
}

func mergeDefaultScrapeConfigs(defaultScrapeConfigs []string) map[interface{}]interface{} {
	mergedDefaultConfigs := make(map[interface{}]interface{})

	if len(defaultScrapeConfigs) > 0 {
		mergedDefaultConfigs["scrape_configs"] = make([]interface{}, 0)

		for _, defaultScrapeConfig := range defaultScrapeConfigs {
			defaultConfigYaml, err := loadYAMLFromFile(defaultScrapeConfig)
			if err != nil {
				log.Printf("Error loading YAML from file %s: %s\n", defaultScrapeConfig, err)
				continue
			}

			mergedDefaultConfigs = deepMerge(mergedDefaultConfigs, defaultConfigYaml)
		}
	}

	fmt.Printf("Done merging %d default prometheus config(s)\n", len(defaultScrapeConfigs))

	return mergedDefaultConfigs
}

func loadYAMLFromFile(filename string) (map[interface{}]interface{}, error) {
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var yamlData map[interface{}]interface{}
	err = yaml.Unmarshal(fileContent, &yamlData)
	if err != nil {
		return nil, err
	}

	return yamlData, nil
}

// This needs unit tests

func deepMerge(target, source map[interface{}]interface{}) map[interface{}]interface{} {
	for key, sourceValue := range source {
		targetValue, exists := target[key]

		if !exists {
			target[key] = sourceValue
			continue
		}

		targetMap, targetMapOk := targetValue.(map[interface{}]interface{})
		sourceMap, sourceMapOk := sourceValue.(map[interface{}]interface{})

		if targetMapOk && sourceMapOk {
			target[key] = deepMerge(targetMap, sourceMap)
		} else if reflect.TypeOf(targetValue) == reflect.TypeOf(sourceValue) {
			// Both are slices, concatenate them
			if targetSlice, targetSliceOk := targetValue.([]interface{}); targetSliceOk {
				if sourceSlice, sourceSliceOk := sourceValue.([]interface{}); sourceSliceOk {
					target[key] = append(targetSlice, sourceSlice...)
				}
			}
		} else {
			// If types are different, simply overwrite with the source value
			target[key] = sourceValue
		}
	}

	return target
}

func writeDefaultScrapeTargetsFile(operatorEnabled bool) map[interface{}]interface{} {
	noDefaultScrapingEnabled := os.Getenv("AZMON_PROMETHEUS_NO_DEFAULT_SCRAPING_ENABLED")
	if noDefaultScrapingEnabled != "" && strings.ToLower(noDefaultScrapingEnabled) == "false" {
		loadRegexHash()
		loadIntervalHash()
		if operatorEnabled {
			populateDefaultPrometheusConfigWithOperator()
		} else {
			populateDefaultPrometheusConfig()
		}
		if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
			fmt.Printf("Starting to merge default prometheus config values in collector template as backup\n")
			mergedDefaultConfigYaml, err := yaml.Marshal(mergedDefaultConfigs)
			if err != nil {
				fmt.Printf("Error marshalling merged default prometheus config: %v\n", err)
				return nil
			}

			err = os.WriteFile(mergedDefaultConfigPath, mergedDefaultConfigYaml, fs.FileMode(0644))
			if err != nil {
				fmt.Printf("Error writing merged default prometheus config to file: %v\n", err)
				return nil
			}

			return mergedDefaultConfigs
		}
	} else {
		mergedDefaultConfigs = nil
	}
	fmt.Printf("Done creating default targets file\n")
	return nil
}

func setDefaultFileScrapeInterval(scrapeInterval string) {
	defaultFilesArray := []string{
		kubeletDefaultFileRsSimple, kubeletDefaultFileRsAdvanced, kubeletDefaultFileDs,
		kubeletDefaultFileRsAdvancedWindowsDaemonset, coreDNSDefaultFile,
		cadvisorDefaultFileRsSimple, cadvisorDefaultFileRsAdvanced, cadvisorDefaultFileDs,
		kubeProxyDefaultFile, apiserverDefaultFile, kubeStateDefaultFile,
		nodeExporterDefaultFileRsSimple, nodeExporterDefaultFileRsAdvanced, nodeExporterDefaultFileDs,
		prometheusCollectorHealthDefaultFile, windowsExporterDefaultRsSimpleFile, windowsExporterDefaultDsFile,
		windowsKubeProxyDefaultFileRsSimpleFile, windowsKubeProxyDefaultDsFile, podAnnotationsDefaultFile,
		kappieBasicDefaultFileDs, networkObservabilityRetinaDefaultFileDs, networkObservabilityHubbleDefaultFileDs,
		networkObservabilityCiliumDefaultFileDs, acstorMetricsExporterDefaultFile, acstorCapacityProvisionerDefaultFile,
	}

	for _, currentFile := range defaultFilesArray {
		contents, err := os.ReadFile(fmt.Sprintf("%s%s", defaultPromConfigPathPrefix, currentFile))
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", currentFile, err)
			continue
		}

		contents = []byte(strings.Replace(string(contents), "$$SCRAPE_INTERVAL$$", scrapeInterval, -1))

		err = os.WriteFile(currentFile, contents, fs.FileMode(0644))
		if err != nil {
			fmt.Printf("Error writing to file %s: %v\n", currentFile, err)
		}
	}
}

func mergeDefaultAndCustomScrapeConfigs(customPromConfig string, mergedDefaultConfigs map[interface{}]interface{}) {
	var mergedConfigYaml []byte

	if mergedDefaultConfigs != nil && len(mergedDefaultConfigs) > 0 {
		shared.EchoStr("Merging default and custom scrape configs")
		var customPrometheusConfig map[interface{}]interface{}
		err := yaml.Unmarshal([]byte(customPromConfig), &customPrometheusConfig)
		if err != nil {
			shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
			return
		}

		var mergedConfigs map[interface{}]interface{}
		if customPrometheusConfig["scrape_configs"] != nil {
			mergedConfigs = deepMerge(mergedDefaultConfigs, customPrometheusConfig)
		} else {
			mergedConfigs = mergedDefaultConfigs
		}

		mergedConfigYaml, err = yaml.Marshal(mergedConfigs)
		if err != nil {
			shared.EchoError(fmt.Sprintf("Error marshalling merged configs: %v", err))
			return
		}

		shared.EchoStr("Done merging default scrape config(s) with custom prometheus config, writing them to file")
	} else {
		shared.EchoWarning("The merged default scrape config is nil or empty, using only custom scrape config")
		mergedConfigYaml = []byte(customPromConfig)
	}

	err := os.WriteFile(promMergedConfigPath, mergedConfigYaml, fs.FileMode(0644))
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error writing merged config to file: %v", err))
		return
	}
}

func setLabelLimitsPerScrape(prometheusConfigString string) string {
	customConfig := prometheusConfigString

	var limitedCustomConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(customConfig), &limitedCustomConfig)
	if err != nil {
		shared.EchoError(fmt.Sprintf("Error unmarshalling custom config: %v", err))
		return prometheusConfigString
	}

	if limitedCustomConfig != nil && len(limitedCustomConfig) > 0 {
		limitedCustomScrapes, _ := limitedCustomConfig["scrape_configs"].([]interface{})
		if limitedCustomScrapes != nil && len(limitedCustomScrapes) > 0 {
			for _, scrape := range limitedCustomScrapes {
				scrapeMap, _ := scrape.(map[interface{}]interface{})
				scrapeMap["label_limit"] = 63
				scrapeMap["label_name_length_limit"] = 511
				scrapeMap["label_value_length_limit"] = 1023
				shared.EchoVar(fmt.Sprintf("Successfully set label limits in custom scrape config for job %s", scrapeMap["job_name"]), "")
			}
			shared.EchoWarning("Done setting label limits for custom scrape config ...")
			updatedConfig, err := yaml.Marshal(limitedCustomConfig)
			if err != nil {
				shared.EchoError(fmt.Sprintf("Error marshalling custom config: %v", err))
				return prometheusConfigString
			}
			return string(updatedConfig)
		} else {
			shared.EchoWarning("No Jobs found to set label limits while processing custom scrape config")
			return prometheusConfigString
		}
	} else {
		shared.EchoWarning("Nothing to set for label limits while processing custom scrape config")
		return prometheusConfigString
	}
}

func setGlobalScrapeConfigInDefaultFilesIfExists(configString string) string {
	var customConfig map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(configString), &customConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}

	// Set scrape interval to 30s for updating the default merged config
	scrapeInterval := "30s"

	globalConfig, globalExists := customConfig["global"].(map[interface{}]interface{})
	if globalExists {
		scrapeInterval, _ = globalConfig["scrape_interval"].(string)

		// Checking to see if the duration matches the pattern specified in the prometheus config
		// Link to documentation with regex pattern -> https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file
		matched := regexp.MustCompile(`^((\d+y)?(\d+w)?(\d+d)?(\d+h)?(\d+m)?(\d+s)?(\d+ms)?|0)$`).MatchString(scrapeInterval)
		if !matched {
			// Set default global scrape interval to 1m if it's not in the proper format
			globalConfig["scrape_interval"] = "1m"
			scrapeInterval = "30s"
		}
	}

	setDefaultFileScrapeInterval(scrapeInterval)

	updatedConfig, err := yaml.Marshal(customConfig)
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}

	return string(updatedConfig)
}

func prometheusConfigMerger(operatorEnabled bool) {
	shared.EchoSectionDivider("Start Processing - prometheusConfigMerger")
	mergedDefaultConfigs = make(map[interface{}]interface{}) // Initialize mergedDefaultConfigs
	prometheusConfigMap := parseConfigMap()

	if len(prometheusConfigMap) > 0 {
		modifiedPrometheusConfigString := setGlobalScrapeConfigInDefaultFilesIfExists(prometheusConfigMap)
		writeDefaultScrapeTargetsFile(operatorEnabled)
		// Set label limits for every custom scrape job, before merging the default & custom config
		labellimitedconfigString := setLabelLimitsPerScrape(modifiedPrometheusConfigString)
		mergeDefaultAndCustomScrapeConfigs(labellimitedconfigString, mergedDefaultConfigs)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Merging Default and Custom Prometheus Config")
	} else {
		setDefaultFileScrapeInterval("30s")
		writeDefaultScrapeTargetsFile(operatorEnabled)
		shared.EchoSectionDivider("End Processing - prometheusConfigMerger, Done Writing Default Prometheus Config")
	}

}
