package main

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

func processConfigMap() map[string]string {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-scrape-interval-settings Processing")

	intervalHash := make(map[string]string)

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
			intervalHash["KUBELET_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("kubelet").(string))
			intervalHash["COREDNS_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("coredns").(string))
			intervalHash["CADVISOR_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("cadvisor").(string))
			intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("kubeproxy").(string))
			intervalHash["APISERVER_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("apiserver").(string))
			intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("kubestate").(string))
			intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("nodeexporter").(string))
			intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("windowsexporter").(string))
			intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("windowskubeproxy").(string))
			intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("prometheuscollectorhealth").(string))
			intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("podannotations").(string))
			intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("kappiebasic").(string))
			intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("networkobservabilityretina").(string))
			intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("networkobservabilityhubble").(string))
			intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = checkDuration(configMapSettings.Get("networkobservabilitycilium").(string))
		}
	} else {
		if _, err := os.Stat(configMapScrapeIntervalMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	fmt.Println("End default-targets-scrape-interval-settings Processing")
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
