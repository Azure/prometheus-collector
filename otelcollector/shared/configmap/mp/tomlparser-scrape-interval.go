package configmapsettings

import (
	"fmt"
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

func getParsedDataValue(parsedData map[string]map[string]string, key string) string {
	if value, exists := parsedData["default-targets-scrape-interval-settings"][key]; exists {
		return checkDuration(value)
	}
	return defaultScrapeInterval
}

func processConfigMap(parsedData map[string]map[string]string) map[string]string {
	intervalHash := make(map[string]string)

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")

	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		// Use parsedData instead of reading from file
		intervalHash["KUBELET_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "kubelet")
		intervalHash["COREDNS_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "coredns")
		intervalHash["CADVISOR_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "cadvisor")
		intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "kubeproxy")
		intervalHash["APISERVER_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "apiserver")
		intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "kubestate")
		intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "nodeexporter")
		intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "windowsexporter")
		intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "windowskubeproxy")
		intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "prometheuscollectorhealth")
		intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "podannotations")
		intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "kappiebasic")
		intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "networkobservabilityRetina")
		intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "networkobservabilityHubble")
		intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "networkobservabilityCilium")
		intervalHash["ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "acstor-capacity-provisioner")
		intervalHash["ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL"] = getParsedDataValue(parsedData, "acstor-metrics-exporter")

		return intervalHash
	}

	// Set each value in intervalHash to "30s" from default
	keys := []string{
		"KUBELET_SCRAPE_INTERVAL", "COREDNS_SCRAPE_INTERVAL", "CADVISOR_SCRAPE_INTERVAL",
		"KUBEPROXY_SCRAPE_INTERVAL", "APISERVER_SCRAPE_INTERVAL", "KUBESTATE_SCRAPE_INTERVAL",
		"NODEEXPORTER_SCRAPE_INTERVAL", "WINDOWSEXPORTER_SCRAPE_INTERVAL",
		"WINDOWSKUBEPROXY_SCRAPE_INTERVAL", "PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL",
		"POD_ANNOTATION_SCRAPE_INTERVAL", "KAPPIEBASIC_SCRAPE_INTERVAL",
		"NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL", "NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL",
		"NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL", "ACSTORCAPACITYPROVISIONER_SCRAPE_INTERVAL",
		"ACSTORMETRICSEXPORTER_SCRAPE_INTERVAL",
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

func tomlparserScrapeInterval(parsedData map[string]map[string]string) {
	shared.EchoSectionDivider("Start Processing - tomlparserScrapeInterval")
	intervalHash := processConfigMap(parsedData)
	err := writeIntervalHashToFile(intervalHash, scrapeIntervalEnvVarPath)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
	shared.EchoSectionDivider("End Processing - tomlparserScrapeInterval")
}
