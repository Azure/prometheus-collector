package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
	"github.com/pelletier/go-toml"
)

var (
	configMapMountPath                     = "/etc/config/settings/default-targets-metrics-keep-list"
	configSchemaVersion, minimalIngestionProfile string
	controlplaneApiserverRegex, controlplaneClusterAutoscalerRegex string
	controlplaneKubeSchedulerRegex, controlplaneKubeControllerManagerRegex string
	controlplaneEtcdRegex                  string
	LOGGING_PREFIX                         = "default-scrape-keep-lists"
	controlplaneKubeControllerManagerMinMac = "rest_client_request_duration_seconds|rest_client_requests_total|workqueue_depth"
	controlplaneKubeSchedulerMinMac         = "scheduler_pending_pods|scheduler_unschedulable_pods|scheduler_pod_scheduling_attempts|scheduler_queue_incoming_pods_total|scheduler_preemption_attempts_total|scheduler_preemption_victims|scheduler_scheduling_attempt_duration_seconds|scheduler_schedule_attempts_total|scheduler_pod_scheduling_duration_seconds"
	controlplaneApiserverMinMac             = "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_request_sli_duration_seconds_bucket|apiserver_request_sli_duration_seconds_count|apiserver_request_sli_duration_seconds_sum|process_start_time_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_count|apiserver_request_duration_seconds_sum|apiserver_storage_list_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests"
	controlplaneClusterAutoscalerMinMac     = "rest_client_requests_total|cluster_autoscaler_((last_activity|cluster_safe_to_autoscale|scale_down_in_cooldown|scaled_up_nodes_total|unneeded_nodes_count|unschedulable_pods_count|nodes_count))|cloudprovider_azure_api_request_(errors|duration_seconds_(bucket|count))"
	controlplaneEtcdMinMac                   = "etcd_server_has_leader|rest_client_requests_total|etcd_mvcc_db_total_size_in_bytes|etcd_mvcc_db_total_size_in_use_in_bytes|etcd_server_slow_read_indexes_total|etcd_server_slow_apply_total|etcd_network_client_grpc_sent_bytes_total|etcd_server_heartbeat_send_failures_total"
	
)

func parseConfigMap() map[string]interface{} {
	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
		return nil
	}

	content, err := ioutil.ReadFile(configMapMountPath)
	if err != nil {
		fmt.Printf("Exception while parsing config map for default-targets-metrics-keep-list: %v, using defaults, please check config map for errors\n", err)
		return nil
	}

	tree, err := toml.Load(string(content))
	if err != nil {
		fmt.Printf("Error parsing TOML: %v\n", err)
		return nil
	}

	configMap := make(map[string]interface{})
	configMap["controlplane-kube-controller-manager"] = tree.Get("controlplane-kube-controller-manager").(string)
	configMap["controlplane-kube-scheduler"] = tree.Get("controlplane-kube-scheduler").(string)
	configMap["controlplane-apiserver"] = tree.Get("controlplane-apiserver").(string)
	configMap["controlplane-cluster-autoscaler"] = tree.Get("controlplane-cluster-autoscaler").(string)
	configMap["controlplane-etcd"] = tree.Get("controlplane-etcd").(string)
	configMap["minimalingestionprofile"] = tree.Get("minimalingestionprofile").(string)

	return configMap
}
func isValidRegex(input string) bool {
	_, err := regexp.Compile(input)
	return err == nil
}

func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}) {
	controlplaneKubeControllerManagerRegex = parsedConfig["controlplane-kube-controller-manager"].(string)
	controlplaneKubeSchedulerRegex = parsedConfig["controlplane-kube-scheduler"].(string)
	controlplaneApiserverRegex = parsedConfig["controlplane-apiserver"].(string)
	controlplaneClusterAutoscalerRegex = parsedConfig["controlplane-cluster-autoscaler"].(string)
	controlplaneEtcdRegex = parsedConfig["controlplane-etcd"].(string)

	// Validate regex values
	if !isValidRegex(controlplaneKubeControllerManagerRegex) {
		fmt.Println("Invalid regex for controlplane-kube-controller-manager:", controlplaneKubeControllerManagerRegex)
		controlplaneKubeControllerManagerRegex = ""
	}
	if !isValidRegex(controlplaneKubeSchedulerRegex) {
		fmt.Println("Invalid regex for controlplane-kube-scheduler:", controlplaneKubeSchedulerRegex)
		controlplaneKubeSchedulerRegex = ""
	}
	if !isValidRegex(controlplaneApiserverRegex) {
		fmt.Println("Invalid regex for controlplane-apiserver:", controlplaneApiserverRegex)
		controlplaneApiserverRegex = ""
	}
	if !isValidRegex(controlplaneClusterAutoscalerRegex) {
		fmt.Println("Invalid regex for controlplane-cluster-autoscaler:", controlplaneClusterAutoscalerRegex)
		controlplaneClusterAutoscalerRegex = ""
	}
	if !isValidRegex(controlplaneEtcdRegex) {
		fmt.Println("Invalid regex for controlplane-etcd:", controlplaneEtcdRegex)
		controlplaneEtcdRegex = ""
	}

	// Logging the values being set
	fmt.Printf("controlplaneKubeControllerManagerRegex: %s\n", controlplaneKubeControllerManagerRegex)
	fmt.Printf("controlplaneKubeSchedulerRegex: %s\n", controlplaneKubeSchedulerRegex)
	fmt.Printf("controlplaneApiserverRegex: %s\n", controlplaneApiserverRegex)
	fmt.Printf("controlplaneClusterAutoscalerRegex: %s\n", controlplaneClusterAutoscalerRegex)
	fmt.Printf("controlplaneEtcdRegex: %s\n", controlplaneEtcdRegex)
}

func populateRegexValuesWithMinimalIngestionProfile() {
	if minimalIngestionProfile == "true" {
		controlplaneKubeControllerManagerRegex += "|" + controlplaneKubeControllerManagerMinMac
		controlplaneKubeSchedulerRegex += "|" + controlplaneKubeSchedulerMinMac
		controlplaneApiserverRegex += "|" + controlplaneApiserverMinMac
		controlplaneClusterAutoscalerRegex += "|" + controlplaneClusterAutoscalerMinMac
		controlplaneEtcdRegex += "|" + controlplaneEtcdMinMac
	}
}

func tomlparserCCPTargetsMetricsKeepList() {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMap()
		if configMapSettings != nil {
			populateSettingValuesFromConfigMap(configMapSettings)
		}
	} else {
		if _, err := os.Stat(configMapMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	populateRegexValuesWithMinimalIngestionProfile()

	// Write settings to a YAML file.
	data := map[string]string{
		"CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX":   controlplaneKubeControllerManagerRegex,
		"CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX":            controlplaneKubeSchedulerRegex,
		"CONTROLPLANE_APISERVER_KEEP_LIST_REGEX":                 controlplaneApiserverRegex,
		"CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX":        controlplaneClusterAutoscalerRegex,
		"CONTROLPLANE_ETCD_KEEP_LIST_REGEX":                      controlplaneEtcdRegex,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = ioutil.WriteFile("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", out, 0644)
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	fmt.Println("End default-targets-metrics-keep-list Processing")
}
