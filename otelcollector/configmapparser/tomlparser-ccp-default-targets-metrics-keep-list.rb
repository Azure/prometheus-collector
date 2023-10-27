#!/usr/local/bin/ruby
# frozen_string_literal: true

require "tomlrb"
if (!ENV["OS_TYPE"].nil? && ENV["OS_TYPE"].downcase == "linux")
  require "re2"
end
require "yaml"
require_relative "ConfigParseErrorLogger"
require_relative "tomlparser-utils"

LOGGING_PREFIX = "default-scrape-keep-lists"

@configMapMountPath = "/etc/config/settings/default-targets-metrics-keep-list"
@configVersion = ""
@configSchemaVersion = ""

@controlplane_apiserver_regex = ""
@controlplane_cluster_autoscaler_regex = ""
@controlplane_kube_scheduler_regex = ""
@controlplane_kube_controller_manager_regex = ""
@controlplane_etcd_regex = ""

@controlplane_apiserver_minimal_mac = "apiserver_request_total|apiserver_cache_list_fetched_objects_total|apiserver_cache_list_returned_objects_total|apiserver_flowcontrol_demand_seats_average|apiserver_flowcontrol_current_limit_seats|apiserver_flowcontrol_rejected_requests_total|apiserver_request_sli_duration_seconds|process_start_time_seconds|apiserver_request_duration_seconds|apiserver_storage_fetched_objects_total|apiserver_storage_list_returned_objects_total|apiserver_current_inflight_requests"
@controlplane_cluster_autoscaler_minimal_mac = "rest_client_requests_total|cluster_autoscaler_((last_activity|cluster_safe_to_autoscale|failed_scale_ups_total|scale_down_in_cooldown|scaled_up_nodes_total|unneeded_nodes_count|unschedulable_pods_count|nodes_count))|cloudprovider_azure_api_request_(errors|duration_seconds_(bucket|count))"
@controlplane_kube_scheduler_minimal_mac = "scheduler_pending_pods|scheduler_unschedulable_pods|scheduler_pod_scheduling_attempts|scheduler_queue_incoming_pods_total|scheduler_preemption_attempts_total|scheduler_preemption_victims|scheduler_scheduling_attempt_duration_seconds|Scheduler_schedule_attempts_total|scheduler_pod_scheduling_duration_seconds"
@controlplane_kube_controller_manager_minimal_mac = "rest_client_requests_duration_seconds_bucket|rest_client_requests_total|workqueue_depth|node_collector_evictions_total"
@controlplane_etcd_minimal_mac = "etcd_memory_in_bytes|etcd_cpu_in_cores|etcd_db_limit_in_bytes|etcd_db_max_size_in_bytes|etcd_db_fragmentation_rate|etcd_db_total_object_count|etcd_db_top_N_object_counts_by_type|etcd_db_top_N_object_size_by_type|etcd2_enabled"

@minimalIngestionProfile = ENV["MINIMAL_INGESTION_PROFILE"]

# Use parser to parse the configmap toml file to a ruby structure
def parseConfigMap
  begin
    # Check to see if config map is created
    if (File.file?(@configMapMountPath))
      parsedConfig = Tomlrb.load_file(@configMapMountPath, symbolize_keys: true)
      return parsedConfig
    else
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "configmap prometheus-collector-configmap for default-targets-metrics-keep-list not mounted, using defaults")
      return nil
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while parsing config map for default-targets-metrics-keep-list: #{errorStr}, using defaults, please check config map for errors")
    return nil
  end
end

# Use the ruby structure created after config parsing to set the right values to be used for otel collector settings
def populateSettingValuesFromConfigMap(parsedConfig)
  begin
    controlplane_kube_controller_manager_regex = parsedConfig[:"controlplane_kube_controller_manager"]
    if !controlplane_kube_controller_manager_regex.nil? && controlplane_kube_controller_manager_regex.kind_of?(String)
      if !controlplane_kube_controller_manager_regex.empty?
        if isValidRegex(controlplane_kube_controller_manager_regex) == true
          @controlplane_kube_controller_manager_regex = controlplane_kube_controller_manager_regex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for controlplane-kube-controller-manager")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for controlplane-kube-controller-manager")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "controlplane_kube_controller_manager either not specified or not of type string")
    end
  
    controlplane_kube_scheduler_regex = parsedConfig[:"controlplane_kube_scheduler"]
    if !controlplane_kube_scheduler_regex.nil? && controlplane_kube_scheduler_regex.kind_of?(String)
      if !controlplane_kube_scheduler_regex.empty?
        if isValidRegex(controlplane_kube_scheduler_regex) == true
          @controlplane_kube_scheduler_regex = controlplane_kube_scheduler_regex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for controlplane-apiserver")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for controlplane-apiserver")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "controlplane_kube_scheduler either not specified or not of type string")
    end
  
    controlplane_apiserver_regex = parsedConfig[:"controlplane_apiserver"]
    if !controlplane_apiserver_regex.nil? && controlplane_apiserver_regex.kind_of?(String)
      if !controlplane_apiserver_regex.empty?
        if isValidRegex(controlplane_apiserver_regex) == true
          @controlplane_apiserver_regex = controlplane_apiserver_regex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for controlplane-apiserver")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for controlplane-apiserver")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "controlplane_apiserver either not specified or not of type string")
    end
  
    controlplane_cluster_autoscaler_regex = parsedConfig[:"controlplane_cluster_autoscaler"]
    if !controlplane_cluster_autoscaler_regex.nil? && controlplane_cluster_autoscaler_regex.kind_of?(String)
      if !controlplane_cluster_autoscaler_regex.empty?
        if isValidRegex(controlplane_cluster_autoscaler_regex) == true
          @controlplane_cluster_autoscaler_regex = controlplane_cluster_autoscaler_regex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for controlplane-cluster-autoscaler")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for controlplane-cluster-autoscaler")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "controlplane_cluster_autoscaler_regex either not specified or not of type string")
    end
  
    controlplane_etcd_regex = parsedConfig[:"controlplane_etcd"]
    if !controlplane_etcd_regex.nil? && controlplane_etcd_regex.kind_of?(String)
      if !controlplane_etcd_regex.empty?
        if isValidRegex(controlplane_etcd_regex) == true
          @controlplane_etcd_regex = controlplane_etcd_regex
          ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap metrics keep list regex for controlplane-etcd")
        else
          ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Invalid keep list regex for controlplane-etcd")
        end
      end
    else
      ConfigParseErrorLogger.logError(LOGGING_PREFIX, "controlplane_etcd_regex either not specified or not of type string")
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while reading config map settings for default targets metrics keep list - #{errorStr}, using defaults, please check config map for errors")
  end

  
  ConfigParseErrorLogger.log(LOGGING_PREFIX, "Reading configmap setting for minimalingestionprofile")
  minimalIngestionProfileSetting = parsedConfig[:minimalingestionprofile]
  if !minimalIngestionProfileSetting.nil?
    @minimalIngestionProfile = minimalIngestionProfileSetting.to_s.downcase
    ConfigParseErrorLogger.log(LOGGING_PREFIX, "Using configmap setting for minimalIngestionProfile -> #{@minimalIngestionProfile}")
  end
end

# -------Apply profile for ingestion--------
# Logical OR-ing profile regex with customer provided regex
# so the theory here is --
# if customer provided regex is valid, our regex validation for that will pass, a OR of customer provided regex with our minimal profile regex would be a valid regex as well, so we dont check again for the wholistic validation of merged regex
# if customer provided regex is invalid, our regex validation for customer provided regex will fail, and if minimal ingestion profile is enabled, we will use that and ignore customer provided one
def populateRegexValuesWithMinimalIngestionProfile
  begin
    if @minimalIngestionProfile == "true"
      ConfigParseErrorLogger.log(LOGGING_PREFIX, "Populating regex with customer  + default values for minimal ingestion profile")
      @controlplane_kube_controller_manager_regex = @controlplane_kube_controller_manager_regex + "|" + @controlplane_kube_controller_manager_minimal_mac
      @controlplane_kube_scheduler_regex = @controlplane_kube_scheduler_regex + "|" + @controlplane_kube_scheduler_minimal_mac
      @controlplane_apiserver_regex = @controlplane_apiserver_regex + "|" + @controlplane_apiserver_minimal_mac
      @controlplane_cluster_autoscaler_regex = @controlplane_cluster_autoscaler_regex + "|" + @controlplane_cluster_autoscaler_minimal_mac
      @controlplane_etcd_regex = @controlplane_etcd_regex + "|" + @controlplane_etcd_minimal_mac
    end
  rescue => errorStr
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while populating regex values with minimal ingestion profile - #{errorStr}, skipping applying minimal ingestion profile regexes")
  end
end

# ----End applying profile for ingestion--------

@configSchemaVersion = ENV["AZMON_AGENT_CFG_SCHEMA_VERSION"]
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "Start default-targets-metrics-keep-list Processing")
if !@configSchemaVersion.nil? && !@configSchemaVersion.empty? && @configSchemaVersion.strip.casecmp("v1") == 0 #note v1 is the only supported schema version, so hardcoding it
  configMapSettings = parseConfigMap
  if !configMapSettings.nil?
    populateSettingValuesFromConfigMap(configMapSettings)
  end
else
  if (File.file?(@configMapMountPath))
    ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Unsupported/missing config schema version - '#{@configSchemaVersion}' , using defaults, please use supported schema version")
  end
end

# Populate the regex values after reading the configmap settings
populateRegexValuesWithMinimalIngestionProfile

# Write the settings to file, so that they can be set as environment variables
file = File.open("/opt/microsoft/configmapparser/config_def_targets_metrics_keep_list_hash", "w")

regexHash = {}
regexHash["CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX"] = @controlplane_kube_controller_manager_regex
regexHash["CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX"] = @controlplane_kube_scheduler_regex
regexHash["CONTROLPLANE_APISERVER_KEEP_LIST_REGEX"] = @controlplane_apiserver_regex
regexHash["CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX"] = @controlplane_cluster_autoscaler_regex
regexHash["CONTROLPLANE_ETCD_KEEP_LIST_REGEX"] = @controlplane_etcd_regex

if !file.nil?
  # Close file after writing regex keep list hash
  # Writing it as yaml as it is easy to read and write hash
  file.write(regexHash.to_yaml)
  file.close
else
  ConfigParseErrorLogger.logError(LOGGING_PREFIX, "Exception while opening file for writing default-targets-metrics-keep-list regex config hash")
end
ConfigParseErrorLogger.logSection(LOGGING_PREFIX, "End default-targets-metrics-keep-list Processing")
