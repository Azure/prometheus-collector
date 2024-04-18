package ccpconfigmapsettings

import (
	"fmt"
	"io/fs"
	"os"

	// "prometheus-collector/shared"
	"github.com/prometheus-collector/shared"

	"strings"

	"github.com/pelletier/go-toml"
	"gopkg.in/yaml.v2"
)

var (
	configMapMountPath                                                     = "/etc/config/settings/default-targets-metrics-keep-list"
	configSchemaVersion, minimalIngestionProfile                           string
	controlplaneApiserverRegex, controlplaneClusterAutoscalerRegex         string
	controlplaneKubeSchedulerRegex, controlplaneKubeControllerManagerRegex string
	controlplaneEtcdRegex                                                  string
	controlplaneKubeControllerManagerMinMac                                = "rest_client_request_duration_seconds|rest_client_requests_total|workqueue_depth"
	controlplaneKubeSchedulerMinMac                                        = "scheduler_pending_pods|scheduler_unschedulable_pods|scheduler_pod_scheduling_attempts|scheduler_queue_incoming_pods_total|scheduler_preemption_attempts_total|scheduler_preemption_victims|scheduler_scheduling_attempt_duration_seconds|scheduler_schedule_attempts_total|scheduler_pod_scheduling_duration_seconds"
	controlplaneApiserverMinMac                                            = "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_request_sli_duration_seconds_bucket|apiserver_request_sli_duration_seconds_count|apiserver_request_sli_duration_seconds_sum|process_start_time_seconds|apiserver_request_duration_seconds_bucket|apiserver_request_duration_seconds_count|apiserver_request_duration_seconds_sum|apiserver_storage_list_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests"
	controlplaneClusterAutoscalerMinMac                                    = "rest_client_requests_total|cluster_autoscaler_((last_activity|cluster_safe_to_autoscale|scale_down_in_cooldown|scaled_up_nodes_total|unneeded_nodes_count|unschedulable_pods_count|nodes_count))|cloudprovider_azure_api_request_(errors|duration_seconds_(bucket|count))"
	controlplaneEtcdMinMac                                                 = "etcd_server_has_leader|rest_client_requests_total|etcd_mvcc_db_total_size_in_bytes|etcd_mvcc_db_total_size_in_use_in_bytes|etcd_server_slow_read_indexes_total|etcd_server_slow_apply_total|etcd_network_client_grpc_sent_bytes_total|etcd_server_heartbeat_send_failures_total"
)

// getStringValue checks the type of the value and returns it as a string if possible.
func getStringValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v) // Convert boolean to string representation
	default:
		// Handle other types if needed
		return fmt.Sprintf("%v", v) // Convert any other type to its default string representation
	}
}

func parseConfigMapForKeepListRegex() map[string]interface{} {
	if _, err := os.Stat(configMapMountPath); os.IsNotExist(err) {
		fmt.Println("configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
		return nil
	}

	content, err := os.ReadFile(configMapMountPath)
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
	configMap["controlplane-kube-controller-manager"] = getStringValue(tree.Get("controlplane-kube-controller-manager"))
	configMap["controlplane-kube-scheduler"] = getStringValue(tree.Get("controlplane-kube-scheduler"))
	configMap["controlplane-apiserver"] = getStringValue(tree.Get("controlplane-apiserver"))
	configMap["controlplane-cluster-autoscaler"] = getStringValue(tree.Get("controlplane-cluster-autoscaler"))
	configMap["controlplane-etcd"] = getStringValue(tree.Get("controlplane-etcd"))
	configMap["minimalingestionprofile"] = getStringValue(tree.Get("minimalingestionprofile"))

	return configMap
}

func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}) (RegexValues, error) {
	regexValues := RegexValues{
		ControlplaneKubeControllerManager: getStringValue(parsedConfig["controlplane-kube-controller-manager"]),
		ControlplaneKubeScheduler:         getStringValue(parsedConfig["controlplane-kube-scheduler"]),
		ControlplaneApiserver:             getStringValue(parsedConfig["controlplane-apiserver"]),
		ControlplaneClusterAutoscaler:     getStringValue(parsedConfig["controlplane-cluster-autoscaler"]),
		ControlplaneEtcd:                  getStringValue(parsedConfig["controlplane-etcd"]),
		MinimalIngestionProfile:           getStringValue(parsedConfig["minimalingestionprofile"]),
	}

	// Validate regex values
	if regexValues.ControlplaneKubeControllerManager != "" && !shared.IsValidRegex(regexValues.ControlplaneKubeControllerManager) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-kube-controller-manager: %s", regexValues.ControlplaneKubeControllerManager)
	}
	if regexValues.ControlplaneKubeScheduler != "" && !shared.IsValidRegex(regexValues.ControlplaneKubeScheduler) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-kube-scheduler: %s", regexValues.ControlplaneKubeScheduler)
	}
	if regexValues.ControlplaneApiserver != "" && !shared.IsValidRegex(regexValues.ControlplaneApiserver) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-apiserver: %s", regexValues.ControlplaneApiserver)
	}
	if regexValues.ControlplaneClusterAutoscaler != "" && !shared.IsValidRegex(regexValues.ControlplaneClusterAutoscaler) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-cluster-autoscaler: %s", regexValues.ControlplaneClusterAutoscaler)
	}
	if regexValues.ControlplaneEtcd != "" && !shared.IsValidRegex(regexValues.ControlplaneEtcd) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-etcd: %s", regexValues.ControlplaneEtcd)
	}
	if regexValues.MinimalIngestionProfile != "" && !shared.IsValidRegex(regexValues.MinimalIngestionProfile) {
		return regexValues, fmt.Errorf("invalid regex for MinimalIngestionProfile: %s", regexValues.MinimalIngestionProfile)
	}

	// Logging the values being set
	fmt.Printf("controlplaneKubeControllerManagerRegex: %s\n", regexValues.ControlplaneKubeControllerManager)
	fmt.Printf("controlplaneKubeSchedulerRegex: %s\n", regexValues.ControlplaneKubeScheduler)
	fmt.Printf("controlplaneApiserverRegex: %s\n", regexValues.ControlplaneApiserver)
	fmt.Printf("controlplaneClusterAutoscalerRegex: %s\n", regexValues.ControlplaneClusterAutoscaler)
	fmt.Printf("controlplaneEtcdRegex: %s\n", regexValues.ControlplaneEtcd)
	fmt.Printf("minimalIngestionProfile: %s\n", regexValues.MinimalIngestionProfile)

	return regexValues, nil // Return regex values and nil error if everything is valid
}

func populateRegexValuesWithMinimalIngestionProfile(regexValues RegexValues) {
	if regexValues.MinimalIngestionProfile == "true" {
		controlplaneKubeControllerManagerRegex += regexValues.ControlplaneKubeControllerManager + "|" + controlplaneKubeControllerManagerMinMac
		controlplaneKubeSchedulerRegex += regexValues.ControlplaneKubeScheduler + "|" + controlplaneKubeSchedulerMinMac
		controlplaneApiserverRegex += regexValues.ControlplaneApiserver + "|" + controlplaneApiserverMinMac
		controlplaneClusterAutoscalerRegex += regexValues.ControlplaneClusterAutoscaler + "|" + controlplaneClusterAutoscalerMinMac
		controlplaneEtcdRegex += regexValues.ControlplaneEtcd + "|" + controlplaneEtcdMinMac

		// Print the updated regex strings after appending values
		// Only log this in debug mode
		// fmt.Println("Updated Regex Strings After Appending:")
		// fmt.Println("ControlplaneKubeControllerManagerRegex:", controlplaneKubeControllerManagerRegex)
		// fmt.Println("ControlplaneKubeSchedulerRegex:", controlplaneKubeSchedulerRegex)
		// fmt.Println("ControlplaneApiserverRegex:", controlplaneApiserverRegex)
		// fmt.Println("ControlplaneClusterAutoscalerRegex:", controlplaneClusterAutoscalerRegex)
		// fmt.Println("ControlplaneEtcdRegex:", controlplaneEtcdRegex)
	} else {
		fmt.Println("minimalIngestionProfile:", regexValues.MinimalIngestionProfile)
	}
}

func tomlparserCCPTargetsMetricsKeepList() {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	var regexValues RegexValues

	if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v1" {
		configMapSettings := parseConfigMapForKeepListRegex()
		if configMapSettings != nil {
			var err error
			regexValues, err = populateSettingValuesFromConfigMap(configMapSettings) // Capture the returned RegexValues
			if err != nil {
				fmt.Printf("Error populating setting values: %v\n", err)
				return
			}
		}
	} else {
		if _, err := os.Stat(configMapMountPath); err == nil {
			fmt.Printf("Unsupported/missing config schema version - '%s', using defaults, please use supported schema version\n", configSchemaVersion)
		}
	}

	populateRegexValuesWithMinimalIngestionProfile(regexValues) // Pass the captured regexValues

	// Write settings to a YAML file.
	data := map[string]string{
		"CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX": controlplaneKubeControllerManagerRegex,
		"CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX":          controlplaneKubeSchedulerRegex,
		"CONTROLPLANE_APISERVER_KEEP_LIST_REGEX":               controlplaneApiserverRegex,
		"CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX":      controlplaneClusterAutoscalerRegex,
		"CONTROLPLANE_ETCD_KEEP_LIST_REGEX":                    controlplaneEtcdRegex,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	err = os.WriteFile("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", []byte(out), fs.FileMode(0644))
	if err != nil {
		fmt.Printf("Exception while writing to file: %v\n", err)
		return
	}

	fmt.Println("End default-targets-metrics-keep-list Processing")
}
