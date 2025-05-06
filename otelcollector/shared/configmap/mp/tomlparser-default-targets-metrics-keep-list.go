package configmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/pelletier/go-toml"
	scrapeConfigs "github.com/prometheus-collector/defaultscrapeconfigs"
	"github.com/prometheus-collector/shared"
	"gopkg.in/yaml.v2"
)

// getStringValue converts various types to string representation
func getStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseConfigMapForKeepListRegex() {
	if _, err := os.Stat(configMapKeepListMountPath); os.IsNotExist(err) {
		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
		return
	}

	content, err := os.ReadFile(configMapKeepListMountPath)
	if err != nil {
		fmt.Printf("Error while parsing config map for default-targets-metrics-keep-list: %v, using defaults\n", err)
		return
	}

	configSchemaVersion := os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	if configSchemaVersion == "" || strings.TrimSpace(configSchemaVersion) != "v1" {
		fmt.Printf("Unsupported/missing config schema version - '%s', using defaults\n", configSchemaVersion)
		return
	}

	tree, err := toml.Load(string(content))
	if err != nil {
		fmt.Printf("Error parsing TOML: %v\n", err)
		return
	}

	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		customerKeepListRegex := getStringValue(tree.Get(jobName))
		if customerKeepListRegex != "" && !isValidRegex(customerKeepListRegex) {
			fmt.Printf("invalid regex for %s: %s", jobName, customerKeepListRegex)
			customerKeepListRegex = ""
		}
		job.CustomerKeepListRegex = customerKeepListRegex
		scrapeConfigs.DefaultScrapeJobs[jobName] = job
	}

	fmt.Printf("Parsed config map for default-targets-metrics-keep-list successfully\n")
}

func tomlparserTargetsMetricsKeepList() {
	shared.EchoSectionDivider("Start Processing - tomlparserTargetsMetricsKeepList")

	parseConfigMapForKeepListRegex()

	// Write settings to a YAML file
	data := map[string]string{}
	for jobName, job := range scrapeConfigs.DefaultScrapeJobs {
		data[fmt.Sprintf("%s_METRICS_KEEP_LIST_REGEX", strings.ToUpper(jobName))] = job.KeepListRegex
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile(configMapKeepListEnvVarPath, []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	shared.EchoSectionDivider("End Processing - tomlparserTargetsMetricsKeepList")
}
