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

func processConfigMap() map[string]string {
	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-scrape-interval-settings Processing")

	intervalHash := make(map[string]string)

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForScrapeSettings()
		if configMapSettings != nil {
			if kubeletInterval := configMapSettings.Get("kubelet").(string); kubeletInterval != "" {
				intervalHash["KUBELET_SCRAPE_INTERVAL"] = checkDuration(kubeletInterval)
			}
			if corednsInterval := configMapSettings.Get("coredns").(string); corednsInterval != "" {
				intervalHash["COREDNS_SCRAPE_INTERVAL"] = checkDuration(corednsInterval)
			}
			if cadvisorInterval := configMapSettings.Get("cadvisor").(string); cadvisorInterval != "" {
				intervalHash["CADVISOR_SCRAPE_INTERVAL"] = checkDuration(cadvisorInterval)
			}
			if kubeproxyInterval := configMapSettings.Get("kubeproxy").(string); kubeproxyInterval != "" {
				intervalHash["KUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(kubeproxyInterval)
			}
			if apiserverInterval := configMapSettings.Get("apiserver").(string); apiserverInterval != "" {
				intervalHash["APISERVER_SCRAPE_INTERVAL"] = checkDuration(apiserverInterval)
			}
			if kubestateInterval := configMapSettings.Get("kubestate").(string); kubestateInterval != "" {
				intervalHash["KUBESTATE_SCRAPE_INTERVAL"] = checkDuration(kubestateInterval)
			}
			if nodeexporterInterval := configMapSettings.Get("nodeexporter").(string); nodeexporterInterval != "" {
				intervalHash["NODEEXPORTER_SCRAPE_INTERVAL"] = checkDuration(nodeexporterInterval)
			}
			if windowsexporterInterval := configMapSettings.Get("windowsexporter").(string); windowsexporterInterval != "" {
				intervalHash["WINDOWSEXPORTER_SCRAPE_INTERVAL"] = checkDuration(windowsexporterInterval)
			}
			if windowskubeproxyInterval := configMapSettings.Get("windowskubeproxy").(string); windowskubeproxyInterval != "" {
				intervalHash["WINDOWSKUBEPROXY_SCRAPE_INTERVAL"] = checkDuration(windowskubeproxyInterval)
			}
			if prometheusCollectorInterval := configMapSettings.Get("prometheuscollectorhealth").(string); prometheusCollectorInterval != "" {
				intervalHash["PROMETHEUS_COLLECTOR_HEALTH_SCRAPE_INTERVAL"] = checkDuration(prometheusCollectorInterval)
			}
			if podAnnotationInterval := configMapSettings.Get("podannotations").(string); podAnnotationInterval != "" {
				intervalHash["POD_ANNOTATION_SCRAPE_INTERVAL"] = checkDuration(podAnnotationInterval)
			}
			if kappieBasicInterval := configMapSettings.Get("kappiebasic").(string); kappieBasicInterval != "" {
				intervalHash["KAPPIEBASIC_SCRAPE_INTERVAL"] = checkDuration(kappieBasicInterval)
			}
			if networkObservabilityRetinaInterval := configMapSettings.Get("networkobservabilityretina").(string); networkObservabilityRetinaInterval != "" {
				intervalHash["NETWORKOBSERVABILITYRETINA_SCRAPE_INTERVAL"] = checkDuration(networkObservabilityRetinaInterval)
			}
			if networkObservabilityHubbleInterval := configMapSettings.Get("networkobservabilityhubble").(string); networkObservabilityHubbleInterval != "" {
				intervalHash["NETWORKOBSERVABILITYHUBBLE_SCRAPE_INTERVAL"] = checkDuration(networkObservabilityHubbleInterval)
			}
			if networkObservabilityCiliumInterval := configMapSettings.Get("networkobservabilitycilium").(string); networkObservabilityCiliumInterval != "" {
				intervalHash["NETWORKOBSERVABILITYCILIUM_SCRAPE_INTERVAL"] = checkDuration(networkObservabilityCiliumInterval)
			}
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
