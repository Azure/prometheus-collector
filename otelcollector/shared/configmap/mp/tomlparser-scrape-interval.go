package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"io/fs"

	"github.com/pelletier/go-toml"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

const (
	defaultScrapeInterval = "30s"
)

var (
	MATCHER = regexp.MustCompile(`^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$`)
)

func parseConfigMapForScrapeSettings() *toml.Tree {
	config, err := toml.LoadFile(configMapScrapeIntervalMountPath)
	if err != nil {
		fmt.Printf("Error parsing config map: %v\n", err)
		return nil
	}
	return config
}

func checkDuration(duration string) string {
	if !MATCHER.MatchString(duration) || duration == "" {
		return defaultScrapeInterval
	}
	return duration
}

func getConfigStringValue(configMapSettings *toml.Tree, key string) string {
	if configMapSettings == nil {
		return ""
	}
	valueInterface := configMapSettings.Get(key)
	if valueInterface == nil {
		return ""
	}
	value, ok := valueInterface.(string)
	if !ok {
		return ""
	}
	return value
}

func processConfigMap() map[string]string {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	intervalHash := make(map[string]string)

	// Handle version v1
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
			// Process v1 scrape intervals
			intervalHash["KUBELET_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "kubelet"))
			intervalHash["COREDNS_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "coredns"))
			intervalHash["CADVISOR_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "cadvisor"))
			intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "kubeproxy"))
			intervalHash["APISERVER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "apiserver"))
			intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "kubestate"))
			intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "nodeexporter"))
			intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "windowsexporter"))
			intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "windowskubeproxy"))
			intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "prometheuscollectorhealth"))
			intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "podannotations"))
			intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "kappiebasic"))
			intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "networkobservabilityRetina"))
			intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "networkobservabilityHubble"))
			intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "networkobservabilityCilium"))
			intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "acstor-capacity-provisioner"))
			intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "acstor-metrics-exporter"))
			return intervalHash
		} else {
			fmt.Printf("Error parsing v1 config map. Using default scrape interval settings\n")
		}
	}

	// Handle version v2
	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v2" {
		// Process v2 scrape intervals for both data plane and control plane
		configMapSettings := parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
			// Data plane metrics
			intervalHash["KUBELET_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.kubelet"))
			intervalHash["COREDNS_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.coredns"))
			intervalHash["CADVISOR_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.cadvisor"))
			intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.kubeproxy"))
			intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.kubestate"))
			intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.nodeexporter"))
			intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.windowsexporter"))
			intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.windowskubeproxy"))
			intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.podannotations"))
			intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.kappiebasic"))
			intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.networkobservabilityRetina"))
			intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.networkobservabilityHubble"))
			intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.networkobservabilityCilium"))
			intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.acstor-capacity-provisioner"))
			intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "dataplane-metrics.acstor-metrics-exporter"))

			// // Control plane metrics
			// intervalHash["APISERVER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "controlplane-metrics.apiserver"))
			// intervalHash["CLUSTERAUTOSCALER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "controlplane-metrics.cluster-autoscaler"))
			// intervalHash["KUBESCHEDULER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "controlplane-metrics.kube-scheduler"))
			// intervalHash["KUBECONTROLLER_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "controlplane-metrics.kube-controller-manager"))
			// intervalHash["ETCD_SCRAPE_INTERVAL"] = checkDuration(getConfigStringValue(configMapSettings, "controlplane-metrics.etcd"))
		}
	}

	// Fallback if no config map or version match
	if _, err := os.Stat(configMapScrapeIntervalMountPath); os.IsNotExist(err) {
		fmt.Printf("Config map for default-targets-scrape-interval-settings not mounted, using defaults\n")
	}
	keys := []string{
		"KUBELET_SCRAPE_INTERVAL", "COREDNS_SCRAPE_INTERVAL", "CADVISOR_SCRAPE_INTERVAL",
		"KUBEPROXY_SCRAPE_INTERVAL", "APISERVER_SCRAPE_INTERVAL", "KUBESTATE_SCRAPE_INTERVAL",
		"NODEEXPORTER_SCRAPE_INTERVAL", "WINDOWSEXPORTER_SCRAPE_INTERVAL",
		"WINDOWSKUBEPROXY_SCRAPE_INTERVAL", "PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL",
		"POD_ANNOTATION_SCRAPE_INTERVAL", "KAPPIEBASIC_SCRAPE_INTERVAL", "NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL",
		"NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL", "NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL", "ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL",
		"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL",
	}
	fmt.Printf("Setting default scrape interval (%s) for all jobs as no config map is present\n", defaultScrapeInterval)
	for _, key := range keys {
		intervalHash[key] = defaultScrapeInterval
	}

	return intervalHash
}

func writeIntervalHashToFile(intervalHash map[string]string, filePath string) error {
	out, err := yaml.Marshal(intervalHash)
	if err != nil {
		return err
	}

	err = os.WriteFile(filePath, []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Error writing intervals to file %s: %v\n", filePath, err)
		return err
	}
	return nil
}

func tomlparserScrapeInterval() {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	intervalHash := processConfigMap()
	err := writeIntervalHashToFile(intervalHash, scrapeIntervalEnvVarPath)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
