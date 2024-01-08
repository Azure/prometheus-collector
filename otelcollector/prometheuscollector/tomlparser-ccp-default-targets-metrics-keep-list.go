package main

// import (
// 	"fmt"
// 	"io/ioutil"
// 	"os"
// 	"strings"
// 	"gopkg.in/yaml.v2"
// )

// var (
// 	configMapMountPath                     = "/etc/config/settings/default-targets-metrics-keep-list"
// 	configSchemaVersion, minimalIngestionProfile string
// 	controlplaneApiserverRegex, controlplaneClusterAutoscalerRegex string
// 	controlplaneKubeSchedulerRegex, controlplaneKubeControllerManagerRegex string
// 	controlplaneEtcdRegex                  string
// 	LOGGING_PREFIX                         = "default-scrape-keep-lists"
// )

// func parseConfigMap() map[string]interface{} {
// 	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
// 		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
// 		return nil
// 	}

// 	content, err := ioutil.ReadFile(configMapMountPath)
// 	if err != nil {
// 		fmt.Printf("Exception while parsing config map for default-targets-metrics-keep-list: %v, using defaults, please check config map for errors\n", err)
// 		return nil
// 	}

// 	tree, err := toml.Load(string(content))
// 	if err != nil {
// 		fmt.Printf("Error parsing TOML: %v\n", err)
// 		return nil
// 	}

// 	configMap := make(map[string]interface{})

// 	// Extract values from the TOML tree and populate the configMap
// 	// Replace 'get' methods with the appropriate way to extract values based on your TOML structure
// 	// Example:
// 	// configMap["controlplane-kube-controller-manager"] = tree.Get("controlplane-kube-controller-manager").(string)
// 	// ... and so on ...

// 	return configMap
// }

// func isValidRegex(input string) bool {
// 	_, err := regexp.Compile(input)
// 	return err == nil
// }

// func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}) {
// 	controlplaneKubeControllerManagerRegex = parsedConfig["controlplane-kube-controller-manager"].(string)
// 	controlplaneKubeSchedulerRegex = parsedConfig["controlplane-kube-scheduler"].(string)
// 	controlplaneApiserverRegex = parsedConfig["controlplane-apiserver"].(string)
// 	controlplaneClusterAutoscalerRegex = parsedConfig["controlplane-cluster-autoscaler"].(string)
// 	controlplaneEtcdRegex = parsedConfig["controlplane-etcd"].(string)

// 	// Validate regex values
// 	if !isValidRegex(controlplaneKubeControllerManagerRegex) {
// 		fmt.Println("Invalid regex for controlplane-kube-controller-manager:", controlplaneKubeControllerManagerRegex)
// 		controlplaneKubeControllerManagerRegex = ""
// 	}
// 	if !isValidRegex(controlplaneKubeSchedulerRegex) {
// 		fmt.Println("Invalid regex for controlplane-kube-scheduler:", controlplaneKubeSchedulerRegex)
// 		controlplaneKubeSchedulerRegex = ""
// 	}
// 	if !isValidRegex(controlplaneApiserverRegex) {
// 		fmt.Println("Invalid regex for controlplane-apiserver:", controlplaneApiserverRegex)
// 		controlplaneApiserverRegex = ""
// 	}
// 	if !isValidRegex(controlplaneClusterAutoscalerRegex) {
// 		fmt.Println("Invalid regex for controlplane-cluster-autoscaler:", controlplaneClusterAutoscalerRegex)
// 		controlplaneClusterAutoscalerRegex = ""
// 	}
// 	if !isValidRegex(controlplaneEtcdRegex) {
// 		fmt.Println("Invalid regex for controlplane-etcd:", controlplaneEtcdRegex)
// 		controlplaneEtcdRegex = ""
// 	}

// 	// Logging the values being set
// 	fmt.Printf("controlplaneKubeControllerManagerRegex: %s\n", controlplaneKubeControllerManagerRegex)
// 	fmt.Printf("controlplaneKubeSchedulerRegex: %s\n", controlplaneKubeSchedulerRegex)
// 	fmt.Printf("controlplaneApiserverRegex: %s\n", controlplaneApiserverRegex)
// 	fmt.Printf("controlplaneClusterAutoscalerRegex: %s\n", controlplaneClusterAutoscalerRegex)
// 	fmt.Printf("controlplaneEtcdRegex: %s\n", controlplaneEtcdRegex)
// }

// func populateRegexValuesWithMinimalIngestionProfile() {
// 	if minimalIngestionProfile == "true" {
// 		controlplaneKubeControllerManagerRegex += "|" + "Your Minimal MAC Value"
// 		controlplaneKubeSchedulerRegex += "|" + "Your Minimal MAC Value"
// 		controlplaneApiserverRegex += "|" + "Your Minimal MAC Value"
// 		controlplaneClusterAutoscalerRegex += "|" + "Your Minimal MAC Value"
// 		controlplaneEtcdRegex += "|" + "Your Minimal MAC Value"
// 	}
// }

// func tomlparserCCPTargetsMetricsKeepList() {
// 	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
// 	fmt.Println("Start default-targets-metrics-keep-list Processing")

// 	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
// 		configMapSettings := parseConfigMap()
// 		if configMapSettings != nil {
// 			populateSettingValuesFromConfigMap(configMapSettings)
// 		}
// 	} else {
// 		if _, err := os.Stat(configMapMountPath); err == nil {
// 			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
// 		}
// 	}

// 	populateRegexValuesWithMinimalIngestionProfile()

// 	// Write settings to a YAML file.
// 	data := map[string]string{
// 		"CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX": controlplaneKubeControllerManagerRegex,
// 		"CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX":           controlplaneKubeSchedulerRegex,
// 		"CONTROLPLANE_APISERVER_KEEP_LIST_REGEX":                 controlplaneApiserverRegex,
// 		"CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX":        controlplaneClusterAutoscalerRegex,
// 		"CONTROLPLANE_ETCD_KEEP_LIST_REGEX":                      controlplaneEtcdRegex,
// 	}

// 	out, err := yaml.Marshal(data)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		return
// 	}

// 	err = ioutil.WriteFile("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", out, 0644)
// 	if err != nil {
// 		fmt.Printf("Exception while writing to file: %v\n", err)
// 		return
// 	}

// 	fmt.Println("End default-targets-metrics-keep-list Processing")
// }
