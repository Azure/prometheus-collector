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
	configMapMountPath                                                                                    = "/etc/config/settings/default-targets-metrics-keep-list"
	configSchemaVersion, minimalIngestionProfile                                                          string
	controlplaneApiserverRegex, controlplaneClusterAutoscalerRegex, controlplaneNodeAutoProvisioningRegex string
	controlplaneKubeSchedulerRegex, controlplaneKubeControllerManagerRegex                                string
	controlplaneEtcdRegex                                                                                 string
	controlplaneKubeControllerManagerMinMac                                                               = "rest_client_request_duration_seconds|rest_client_requests_total|workqueue_depth"
	controlplaneKubeSchedulerMinMac                                                                       = "scheduler_pending_pods|scheduler_unschedulable_pods|scheduler_pod_scheduling_attempts|scheduler_queue_incoming_pods_total|scheduler_preemption_attempts_total|scheduler_preemption_victims|scheduler_scheduling_attempt_duration_seconds|scheduler_schedule_attempts_total|scheduler_pod_scheduling_duration_seconds"
	controlplaneApiserverMinMac                                                                           = "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_request_sli_duration_seconds_count|apiserver_request_sli_duration_seconds_sum|process_start_time_seconds|apiserver_request_duration_seconds_count|apiserver_request_duration_seconds_sum|apiserver_storage_list_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests"
	controlplaneClusterAutoscalerMinMac                                                                   = "rest_client_requests_total|cluster_autoscaler_((last_activity|cluster_safe_to_autoscale|scale_down_in_cooldown|scaled_up_nodes_total|unneeded_nodes_count|unschedulable_pods_count|nodes_count))|cloudprovider_azure_api_request_(errors|duration_seconds_(bucket|count))"
	controlplaneNodeAutoProvisioningMinMac                                                                = ""
	controlplaneEtcdMinMac                                                                                = "etcd_server_has_leader|rest_client_requests_total|etcd_mvcc_db_total_size_in_bytes|etcd_mvcc_db_total_size_in_use_in_bytes|etcd_server_slow_read_indexes_total|etcd_server_slow_apply_total|etcd_network_client_grpc_sent_bytes_total|etcd_server_heartbeat_send_failures_total"
)

// getStringValue checks the type of the value and returns it as a string if possible.
func getStringValue(value interface{}) string {
	if value == nil {
		return ""
	}
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

// parseConfigMapForKeepListRegex extracts the control plane metrics keep list from metricsConfigBySection.
func parseConfigMapForKeepListRegex(metricsConfigBySection map[string]map[string]string, schemaVersion string) map[string]interface{} {
	configMap := make(map[string]interface{})
	// set default to true in case there is no configmap
	configMap["minimalingestionprofile"] = "true"

	fmt.Printf("parseConfigMapForKeepListRegex::schemaVersion: %s\n", schemaVersion)

	if schemaVersion == "v1" {
		fmt.Println("parseConfigMapForKeepListRegex::Processing v1 schema")
		// For v1, control plane jobs are under "default-targets-metrics-keep-list" with "controlplane-" prefix
		if settings, ok := metricsConfigBySection["default-targets-metrics-keep-list"]; ok {
			fmt.Println("parseConfigMapForKeepListRegex::Found default-targets-metrics-keep-list section")
			for key, value := range settings {
				if strings.HasPrefix(key, "controlplane-") {
					fmt.Printf("parseConfigMapForKeepListRegex::Adding key: %s, value: %s\n", key, value)
					configMap[key] = value
				}
			}
		}

		// Handle minimalingestionprofile for v1
		if settings, ok := metricsConfigBySection["default-targets-metrics-keep-list"]; ok {
			if minimalProfile, ok := settings["minimalingestionprofile"]; ok {
				fmt.Printf("parseConfigMapForKeepListRegex::Found minimalingestionprofile: %s\n", minimalProfile)
				configMap["minimalingestionprofile"] = minimalProfile
			} else {
				fmt.Println("parseConfigMapForKeepListRegex::minimalingestionprofile not found, setting default to true")
				configMap["minimalingestionprofile"] = "true" // Setting the default value
			}
		} else {
			fmt.Println("parseConfigMapForKeepListRegex::default-targets-metrics-keep-list section not found, setting default to true")
			configMap["minimalingestionprofile"] = "true" // Setting the default value
		}
	} else if schemaVersion == "v2" {
		fmt.Println("parseConfigMapForKeepListRegex::Processing v2 schema")
		// For v2, control plane jobs are under "controlplane-metrics" without "controlplane-" prefix
		if settings, ok := metricsConfigBySection["default-targets-metrics-keep-list"]; ok {
			fmt.Println("parseConfigMapForKeepListRegex::Found default-targets-metrics-keep-list section")
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
					fmt.Printf("parseConfigMapForKeepListRegex::Adding key: %s, value: %s\n", v1Key, value)
					configMap[v1Key] = value
				}
			}

			// Handle minimal-ingestion-profile for v2
			if minimalProfileSection, ok := metricsConfigBySection["minimal-ingestion-profile"]; ok {
				if enabledValue, ok := minimalProfileSection["enabled"]; ok {
					fmt.Printf("parseConfigMapForKeepListRegex::Found minimal-ingestion-profile enabled: %s\n", enabledValue)
					configMap["minimalingestionprofile"] = enabledValue
				} else {
					fmt.Println("parseConfigMapForKeepListRegex::minimal-ingestion-profile enabled not found, setting default to true")
					configMap["minimalingestionprofile"] = "true" // Setting the default value
				}
			} else {
				fmt.Println("parseConfigMapForKeepListRegex::minimal-ingestion-profile section not found, setting default to true")
				configMap["minimalingestionprofile"] = "true" // Setting the default value
			}

		}
	}

	fmt.Printf("parseConfigMapForKeepListRegex::Final configMap: %+v\n", configMap)
	return configMap
}

// populateSettingValuesFromConfigMap populates settings from the parsed configuration.
func populateSettingValuesFromConfigMap(parsedConfig map[string]interface{}, schemaVersion string) (RegexValues, error) {
	regexValues := RegexValues{}

	// v2ToV1KeyMap for mapping v2 keys to v1 keys
	v2ToV1KeyMap := map[string]string{
		"apiserver":               "controlplane-apiserver",
		"cluster-autoscaler":      "controlplane-cluster-autoscaler",
		"kube-scheduler":          "controlplane-kube-scheduler",
		"kube-controller-manager": "controlplane-kube-controller-manager",
		"etcd":                    "controlplane-etcd",
		"minimalingestionprofile": "minimalingestionprofile",
	}

	for key, value := range parsedConfig {
		// Map v2 key to v1 if schemaVersion == v2
		if configSchemaVersion != "" && strings.TrimSpace(configSchemaVersion) == "v2" {
			key = v2ToV1KeyMap[key]
		}

		switch key {
		case "controlplane-kube-controller-manager":
			regexValues.ControlplaneKubeControllerManager = getStringValue(value)
		case "controlplane-kube-scheduler":
			regexValues.ControlplaneKubeScheduler = getStringValue(value)
		case "controlplane-apiserver":
			regexValues.ControlplaneApiserver = getStringValue(value)
		case "controlplane-cluster-autoscaler":
			regexValues.ControlplaneClusterAutoscaler = getStringValue(value)
		case "controlplane-etcd":
			regexValues.ControlplaneEtcd = getStringValue(value)
		case "minimalingestionprofile":
			regexValues.MinimalIngestionProfile = getStringValue(value)
		}
	}

	fmt.Printf("populateSettingValuesFromConfigMap::Initial regexValues: %+v\n", regexValues)

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
	if regexValues.ControlplaneNodeAutoProvisioning != "" && !shared.IsValidRegex(regexValues.ControlplaneNodeAutoProvisioning) {
		return regexValues, fmt.Errorf("invalid regex for controlplane-node-auto-provisioning: %s", regexValues.ControlplaneNodeAutoProvisioning)
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
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneNodeAutoProvisioningRegex: %s\n", regexValues.ControlplaneNodeAutoProvisioning)
	fmt.Printf("populateSettingValuesFromConfigMap::controlplaneEtcdRegex: %s\n", regexValues.ControlplaneEtcd)
	fmt.Printf("populateSettingValuesFromConfigMap::minimalIngestionProfile: %s\n", regexValues.MinimalIngestionProfile)

	return regexValues, nil // Return regex values and nil error if everything is valid
}

// populateRegexValuesWithMinimalIngestionProfile updates regex values based on minimal ingestion profile.
func populateRegexValuesWithMinimalIngestionProfile(regexValues RegexValues) {
	fmt.Println("populateRegexValuesWithMinimalIngestionProfile::minimalIngestionProfile:", regexValues.MinimalIngestionProfile)

	if regexValues.MinimalIngestionProfile == "false" {
		fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is false, appending values")
		controlplaneKubeControllerManagerRegex += regexValues.ControlplaneKubeControllerManager
		controlplaneKubeSchedulerRegex += regexValues.ControlplaneKubeScheduler
		controlplaneApiserverRegex += regexValues.ControlplaneApiserver
		controlplaneClusterAutoscalerRegex += regexValues.ControlplaneClusterAutoscaler
		controlplaneNodeAutoProvisioningRegex += regexValues.ControlplaneNodeAutoProvisioning
		controlplaneEtcdRegex += regexValues.ControlplaneEtcd
	} else { // else accounts for "true" and any other values including "nil" (meaning no configmap or no minimal setting in the configmap)
		fmt.Println("populateRegexValuesWithMinimalIngestionProfile::Minimal ingestion profile is true or not set, appending minimal metrics")
		controlplaneKubeControllerManagerRegex += regexValues.ControlplaneKubeControllerManager + "|" + controlplaneKubeControllerManagerMinMac
		controlplaneKubeSchedulerRegex += regexValues.ControlplaneKubeScheduler + "|" + controlplaneKubeSchedulerMinMac
		controlplaneApiserverRegex += regexValues.ControlplaneApiserver + "|" + controlplaneApiserverMinMac
		controlplaneClusterAutoscalerRegex += regexValues.ControlplaneClusterAutoscaler + "|" + controlplaneClusterAutoscalerMinMac
		controlplaneNodeAutoProvisioningRegex += regexValues.ControlplaneNodeAutoProvisioning + "|" + controlplaneNodeAutoProvisioningMinMac
		controlplaneEtcdRegex += regexValues.ControlplaneEtcd + "|" + controlplaneEtcdMinMac
	}
}

// tomlparserCCPTargetsMetricsKeepList processes the configuration and writes it to a file.
func tomlparserCCPTargetsMetricsKeepList(metricsConfigBySection map[string]map[string]string) {
	configSchemaVersion = os.Getenv("AZMON_AGENT_CFG_SCHEMA_VERSION")
	fmt.Println("Start default-targets-metrics-keep-list Processing")

	var regexValues RegexValues

	if configSchemaVersion != "" && (strings.TrimSpace(configSchemaVersion) == "v1" || strings.TrimSpace(configSchemaVersion) == "v2") {
		fmt.Printf("tomlparserCCPTargetsMetricsKeepList::Processing with schema version: %s\n", configSchemaVersion)
		configMapSettings := parseConfigMapForKeepListRegex(metricsConfigBySection, configSchemaVersion)
		if configMapSettings != nil {
			var err error
			regexValues, err = populateSettingValuesFromConfigMap(configMapSettings, configSchemaVersion) // Capture the returned RegexValues
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
		"CONTROLPLANE_NODE_AUTO_PROVISIONING_KEEP_LIST_REGEX":  controlplaneNodeAutoProvisioningMinMac,
		"CONTROLPLANE_ETCD_KEEP_LIST_REGEX":                    controlplaneEtcdRegex,
	}

	fmt.Printf("tomlparserCCPTargetsMetricsKeepList::Final data to write: %+v\n", data)

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
