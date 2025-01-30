package ccpconfigmapsettings

import (
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/prometheus-collector/shared"
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
	controlplaneApiserverMinMac                                            = "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_request_sli_duration_seconds_count|apiserver_request_sli_duration_seconds_sum|process_start_time_seconds|apiserver_request_duration_seconds_count|apiserver_request_duration_seconds_sum|apiserver_storage_list_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests"
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

// parseConfigMapForKeepListRegex extracts the control plane metrics keep list from parsedData.
func parseConfigMapForKeepListRegex(parsedData map[string]map[string]string, schemaVersion string) map[string]interface{} {
	configMap := make(map[string]interface{})

	if schemaVersion == "v1" {
		// For v1, control plane jobs are under "default-targets-metrics-keep-list" with "controlplane-" prefix
		if settings, ok := parsedData["default-targets-metrics-keep-list"]; ok {
			for key, value := range settings {
				if strings.HasPrefix(key, "controlplane-") {
					configMap[key] = value
				}
			}
		}
	} else if schemaVersion == "v2" {
		// For v2, control plane jobs are under "controlplane-metrics" without "controlplane-" prefix
		if settings, ok := parsedData["controlplane-metrics"]; ok {
			// Map v2 keys to v1 keys
			v2ToV1KeyMap := map[string]string{
				"apiserver":               "controlplane-apiserver",
				"cluster-autoscaler":      "controlplane-cluster-autoscaler",
				"kube-scheduler":          "controlplane-kube-scheduler",
				"kube-controller-manager": "controlplane-kube-controller-manager",
				"etcd":                    "controlplane-etcd",
			}
			for key, value := range settings {
				if v1Key, ok := v2ToV1KeyMap[key]; ok {
					configMap[v1Key] = value
				}
			}
		}
	}

	return configMap
}

// populateSettingValuesFromConfigMap populates settings from the parsed configuration.
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
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneKubeControllerManagerRegex: %s\n", regexValues.ControlplaneKubeControllerManager)
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneKubeSchedulerRegex: %s\n", regexValues.ControlplaneKubeScheduler)
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneApiserverRegex: %s\n", regexValues.ControlplaneApiserver)
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneClusterAutoscalerRegex: %s\n", regexValues.ControlplaneClusterAutoscaler)
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneEtcdRegex: %s\n", regexValues.ControlplaneEtcd)
	fmt.Printf("populateSettingValuesFromConfigMap::minimalIngestionProfile: %s\n", regexValues.MinimalIngestionProfile)

	return regexValues, nil // Return regex values and nil error if everything is valid
}

// populateRegexValuesWithMinimalIngestionProfile updates regex values based on minimal ingestion profile.
func populateRegexValuesWithMinimalIngestionProfile(regexValues RegexValues) {
	fmt.Println("populateRegexValuesWithMinimalIngestionProfile::minimalIngestionProfile:", regexValues.MinimalIngestionProfile)

	if regexValues.MinimalIngestionProfile == "false" {
		controlplaneKubeControllerManagerRegex += regexValues.ControlplaneKubeControllerManager
		controlplaneKubeSchedulerRegex += regexValues.ControlplaneKubeScheduler
		controlplaneApiserverRegex += regexValues.ControlplaneApiserver
		controlplaneClusterAutoscalerRegex += regexValues.ControlplaneClusterAutoscaler
		controlplaneEtcdRegex += regexValues.ControlplaneEtcd

		// Print the updated regex strings after appending values
		fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Regex Strings for CCP targets: collecting ONLY below metrics for targets")
		fmt.Println("ControlplaneKubeControllerManagerRegex:", controlplaneKubeControllerManagerRegex)
		fmt.Println("ControlplaneKubeSchedulerRegex:", controlplaneKubeSchedulerRegex)
		fmt.Println("ControlplaneApiserverRegex:", controlplaneApiserverRegex)
		fmt.Println("ControlplaneClusterAutoscalerRegex:", controlplaneClusterAutoscalerRegex)
		fmt.Println("ControlplaneEtcdRegex:", controlplaneEtcdRegex)
	} else { // else accounts for "true" and any other values including "nil" (meaning no configmap or no minimal setting in the configmap)
		controlplaneKubeControllerManagerRegex += regexValues.ControlplaneKubeControllerManager + "|" + controlplaneKubeControllerManagerMinMac
		controlplaneKubeSchedulerRegex += regexValues.ControlplaneKubeScheduler + "|" + controlplaneKubeSchedulerMinMac
		controlplaneApiserverRegex += regexValues.ControlplaneApiserver + "|" + controlplaneApiserverMinMac
		controlplaneClusterAutoscalerRegex += regexValues.ControlplaneClusterAutoscaler + "|" + controlplaneClusterAutoscalerMinMac
		controlplaneEtcdRegex += regexValues.ControlplaneEtcd + "|" + controlplaneEtcdMinMac
	}
}

// tomlparserCCPTargetsMetricsKeepList processes the configuration and writes it to a file.
func tomlparserCCPTargetsMetricsKeepList(parsedData map[string]map[string]string) {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	var regexValues RegexValues

	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		configMapSettings := parseConfigMapForKeepListRegex(parsedData, configSchemaVersion)
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
