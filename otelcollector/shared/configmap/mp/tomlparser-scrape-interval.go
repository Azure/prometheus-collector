package configmapsettings

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"io/fs"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

const (
	configMapScrapeIntervalMountPath = "/etc/config/settings/default-targets-scrape-interval-settings"
	defaultScrapeInterval            = "30s"
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
	if !MATCHER.MatchString(duration) {
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
	fmt.Println("Start default-targets-scrape-interval-settings")

	intervalHash := make(map[string]string)

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
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
		}
	} else {
		if _, err := os.Stat(configMapScrapeIntervalMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	fmt.Println("End default-targets-scrape-interval-settings")
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

func tomlparserScrapeInterval() {
	intervalHash := processConfigMap()
	err := writeIntervalHashToFile(intervalHash, "/opt/microsoft/configmapparser/config_def_targets_scrape_intervals_hash")
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
}
