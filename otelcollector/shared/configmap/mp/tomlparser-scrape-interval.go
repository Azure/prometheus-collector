package configmapsettings

import (
	"log"
	"os"
	"regexp"
	"strings"

	"io/fs"

	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

const (
	defaultScrapeInterval = "30s"
)

var (
	MATCHER = regexp.MustCompile(`^((([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?|0)$`)
)

func checkDuration(duration string) string {
	if !MATCHER.MatchString(duration) || duration == "" {
		return defaultScrapeInterval
	}
	return duration
}

func getParsedDataValue(metricsConfigBySection map[string]map[string]string, key string) string {
	if value, exists := metricsConfigBySection["default-targets-scrape-interval-settings"][key]; exists {
		return checkDuration(value)
	}
	return defaultScrapeInterval
}

func processConfigMap(metricsConfigBySection map[string]map[string]string) map[string]string {
	intervalHash := make(map[string]string)

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		// Use metricsConfigBySection instead of reading from file
		intervalHash["KUBELET_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "kubelet")
		intervalHash["COREDNS_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "coredns")
		intervalHash["CADVISOR_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "cadvisor")
		intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "kubeproxy")
		intervalHash["APISERVER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "apiserver")
		intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "kubestate")
		intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "nodeexporter")
		intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "windowsexporter")
		intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "windowskubeproxy")
		intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "prometheuscollectorhealth")
		intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "podannotations")
		intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "kappiebasic")
		intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "networkobservabilityRetina")
		intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "networkobservabilityHubble")
		intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "networkobservabilityCilium")
		intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "acstor-capacity-provisioner")
		intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "acstor-metrics-exporter")
		intervalHash["LOCALCSIDRIVER_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "local-csi-driver")
		intervalHash["ZTUNNEL_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "ztunnel")
		intervalHash["ISTIOCNI_SCRAPE_INTERVAL"] = getParsedDataValue(metricsConfigBySection, "istio-cni")

		return intervalHash
	}

	log.Printf("Setting default scrape interval (%s) for all jobs as no config map is present \n", defaultScrapeInterval)
	// Set each value in intervalHash to "30s" from default
	keys := []string{
		"KUBELET_SCRAPE_INTERVAL", "COREDNS_SCRAPE_INTERVAL", "CADVISOR_SCRAPE_INTERVAL",
		"KUBEPROXY_SCRAPE_INTERVAL", "APISERVER_SCRAPE_INTERVAL", "KUBESTATE_SCRAPE_INTERVAL",
		"NODEEXPORTER_SCRAPE_INTERVAL", "WINDOWSEXPORTER_SCRAPE_INTERVAL",
		"WINDOWSKUBEPROXY_SCRAPE_INTERVAL", "PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL",
		"POD_ANNOTATION_SCRAPE_INTERVAL", "KAPPIEBASIC_SCRAPE_INTERVAL",
		"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL", "NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL",
		"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL", "ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL",
		"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL", "LOCALCSIDRIVER_SCRAPE_INTERVAL",
		"ZTUNNEL_SCRAPE_INTERVAL", "ISTIOCNI_SCRAPE_INTERVAL",
	}

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
		return err
	}
	return nil
}

func tomlparserScrapeInterval(metricsConfigBySection map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	intervalHash := processConfigMap(metricsConfigBySection)
	err := writeIntervalHashToFile(intervalHash, scrapeIntervalEnvVarPath)
	if err != nil {
		log.Printf("Error writing to file: %v\n", err)
		return
	}
	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
