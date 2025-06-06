param azureMonitorWorkspaceResourceId string
param azureMonitorWorkspaceLocation string
param clusterResourceId string
param clusterLocation string
param metricLabelsAllowlist string
param metricAnnotationsAllowList string
param enableWindowsRecordingRules bool
param grafanaResourceId string
param grafanaLocation string
param grafanaSku string
param grafanaAdminObjectId string

@description('A new GUID used to identify the role assignment')
param roleNameGuid string = newGuid()

var azureMonitorWorkspaceSubscriptionId = split(azureMonitorWorkspaceResourceId, '/')[2]
var clusterSubscriptionId = split(clusterResourceId, '/')[2]
var clusterResourceGroup = split(clusterResourceId, '/')[4]
var clusterName = split(clusterResourceId, '/')[8]
var dceName = substring('MSProm-${azureMonitorWorkspaceLocation}-${clusterName}', 0, min(44, length('MSProm-${azureMonitorWorkspaceLocation}-${clusterName}')))
var dcrName = substring('MSProm-${azureMonitorWorkspaceLocation}-${clusterName}', 0, min(64, length('MSProm-${azureMonitorWorkspaceLocation}-${clusterName}')))
var dcraName = 'MSProm-${clusterLocation}-${clusterName}'
var nodeRecordingRuleGroupPrefix = 'NodeRecordingRulesRuleGroup-'
var nodeRecordingRuleGroupName = '${nodeRecordingRuleGroupPrefix}${clusterName}'
var nodeRecordingRuleGroupDescription = 'Node Recording Rules RuleGroup'
var kubernetesRecordingRuleGrouPrefix = 'KubernetesRecordingRulesRuleGroup-'
var kubernetesRecordingRuleGroupName = '${kubernetesRecordingRuleGrouPrefix}${clusterName}'
var kubernetesRecordingRuleGroupDescription = 'Kubernetes Recording Rules RuleGroup'
var nodeRecordingRuleGroupWin = 'NodeRecordingRulesRuleGroup-Win-'
var nodeAndKubernetesRecordingRuleGroupWin = 'NodeAndKubernetesRecordingRulesRuleGroup-Win-'
var nodeRecordingRuleGroupNameWinName = '${nodeRecordingRuleGroupWin}${clusterName}'
var nodeAndKubernetesRecordingRuleGroupWinName = '${nodeAndKubernetesRecordingRuleGroupWin}${clusterName}'
var RecordingRuleGroupDescriptionWin = 'Recording Rules RuleGroup for Win'
var uxRecordingRulesRuleGroup = 'UXRecordingRulesRuleGroup - ${clusterName}'
var uxRecordingRulesRuleGroupDescription = 'UX recording rules for Linux'
var uxRecordingRulesRuleGroupWin = 'UXRecordingRulesRuleGroup-Win - ${clusterName}'
var uxRecordingRulesRuleGroupWinDescription = 'UX recording rules for Windows'
var version = ' - 0.1'

resource dce 'Microsoft.Insights/dataCollectionEndpoints@2022-06-01' = {
  name: dceName
  location: azureMonitorWorkspaceLocation
  kind: 'Linux'
  properties: {
  }
}

resource dcr 'Microsoft.Insights/dataCollectionRules@2022-06-01' = {
  name: dcrName
  location: azureMonitorWorkspaceLocation
  kind: 'Linux'
  properties: {
    dataCollectionEndpointId: dce.id
    dataFlows: [
      {
        destinations: [
          'MonitoringAccount1'
        ]
        streams: [
          'Microsoft-PrometheusMetrics'
        ]
      }
    ]
    dataSources: {
      prometheusForwarder: [
        {
          name: 'PrometheusDataSource'
          streams: [
            'Microsoft-PrometheusMetrics'
          ]
          labelIncludeFilter: {
          }
        }
      ]
    }
    description: 'DCR for Azure Monitor Metrics Profile (Managed Prometheus)'
    destinations: {
      monitoringAccounts: [
        {
          accountResourceId: azureMonitorWorkspaceResourceId
          name: 'MonitoringAccount1'
        }
      ]
    }
  }
}

module azuremonitormetrics_dcra_clusterResourceId './nested_azuremonitormetrics_dcra_clusterResourceId.bicep' = {
  name: 'azuremonitormetrics-dcra-${uniqueString(clusterResourceId)}'
  scope: resourceGroup(clusterSubscriptionId, clusterResourceGroup)
  params: {
    resourceId_Microsoft_Insights_dataCollectionRules_variables_dcrName: dcr.id
    variables_clusterName: clusterName
    variables_dcraName: dcraName
    clusterLocation: clusterLocation
  }
  dependsOn: [
    dce

  ]
}

module azuremonitormetrics_profile_clusterResourceId './nested_azuremonitormetrics_profile_clusterResourceId.bicep' = {
  name: 'azuremonitormetrics-profile--${uniqueString(clusterResourceId)}'
  scope: resourceGroup(clusterSubscriptionId, clusterResourceGroup)
  params: {
    variables_clusterName: clusterName
    clusterLocation: clusterLocation
    metricLabelsAllowlist: metricLabelsAllowlist
    metricAnnotationsAllowList: metricAnnotationsAllowList
  }
  dependsOn: [
    azuremonitormetrics_dcra_clusterResourceId
  ]
}

resource nodeRecordingRuleGroup 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: nodeRecordingRuleGroupName
  location: azureMonitorWorkspaceLocation
  properties: {
    description: '${nodeRecordingRuleGroupDescription}${version}'
    scopes: [azureMonitorWorkspaceResourceId,clusterResourceId]
    enabled: true
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
        record: 'instance:node_num_cpu:sum'
        expression: 'count without (cpu, mode) (  node_cpu_seconds_total{job="node",mode="idle"})'
      }
      {
        record: 'instance:node_cpu_utilisation:rate5m'
        expression: '1 - avg without (cpu) (  sum without (mode) (rate(node_cpu_seconds_total{job="node", mode=~"idle|iowait|steal"}[5m])))'
      }
      {
        record: 'instance:node_load1_per_cpu:ratio'
        expression: '(  node_load1{job="node"}/  instance:node_num_cpu:sum{job="node"})'
      }
      {
        record: 'instance:node_memory_utilisation:ratio'
        expression: '1 - (  (    node_memory_MemAvailable_bytes{job="node"}    or    (      node_memory_Buffers_bytes{job="node"}      +      node_memory_Cached_bytes{job="node"}      +      node_memory_MemFree_bytes{job="node"}      +      node_memory_Slab_bytes{job="node"}    )  )/  node_memory_MemTotal_bytes{job="node"})'
      }
      {
        record: 'instance:node_vmstat_pgmajfault:rate5m'
        expression: 'rate(node_vmstat_pgmajfault{job="node"}[5m])'
      }
      {
        record: 'instance_device:node_disk_io_time_seconds:rate5m'
        expression: 'rate(node_disk_io_time_seconds_total{job="node", device!=""}[5m])'
      }
      {
        record: 'instance_device:node_disk_io_time_weighted_seconds:rate5m'
        expression: 'rate(node_disk_io_time_weighted_seconds_total{job="node", device!=""}[5m])'
      }
      {
        record: 'instance:node_network_receive_bytes_excluding_lo:rate5m'
        expression: 'sum without (device) (  rate(node_network_receive_bytes_total{job="node", device!="lo"}[5m]))'
      }
      {
        record: 'instance:node_network_transmit_bytes_excluding_lo:rate5m'
        expression: 'sum without (device) (  rate(node_network_transmit_bytes_total{job="node", device!="lo"}[5m]))'
      }
      {
        record: 'instance:node_network_receive_drop_excluding_lo:rate5m'
        expression: 'sum without (device) (  rate(node_network_receive_drop_total{job="node", device!="lo"}[5m]))'
      }
      {
        record: 'instance:node_network_transmit_drop_excluding_lo:rate5m'
        expression: 'sum without (device) (  rate(node_network_transmit_drop_total{job="node", device!="lo"}[5m]))'
      }
    ]
  }
}

resource kubernetesRecordingRuleGroup 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: kubernetesRecordingRuleGroupName
  location: azureMonitorWorkspaceLocation
  properties: {
    description: '${kubernetesRecordingRuleGroupDescription}${version}'
    scopes: [azureMonitorWorkspaceResourceId,clusterResourceId]
    enabled: true
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
        record: 'node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate'
        expression: 'sum by (cluster, namespace, pod, container) (  irate(container_cpu_usage_seconds_total{job="cadvisor", image!=""}[5m])) * on (cluster, namespace, pod) group_left(node) topk by (cluster, namespace, pod) (  1, max by(cluster, namespace, pod, node) (kube_pod_info{node!=""}))'
      }
      {
        record: 'node_namespace_pod_container:container_memory_working_set_bytes'
        expression: 'container_memory_working_set_bytes{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))'
      }
      {
        record: 'node_namespace_pod_container:container_memory_rss'
        expression: 'container_memory_rss{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))'
      }
      {
        record: 'node_namespace_pod_container:container_memory_cache'
        expression: 'container_memory_cache{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))'
      }
      {
        record: 'node_namespace_pod_container:container_memory_swap'
        expression: 'container_memory_swap{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))'
      }
      {
        record: 'cluster:namespace:pod_memory:active:kube_pod_container_resource_requests'
        expression: 'kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))'
      }
      {
        record: 'namespace_memory:kube_pod_container_resource_requests:sum'
        expression: 'sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))'
      }
      {
        record: 'cluster:namespace:pod_cpu:active:kube_pod_container_resource_requests'
        expression: 'kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))'
      }
      {
        record: 'namespace_cpu:kube_pod_container_resource_requests:sum'
        expression: 'sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))'
      }
      {
        record: 'cluster:namespace:pod_memory:active:kube_pod_container_resource_limits'
        expression: 'kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))'
      }
      {
        record: 'namespace_memory:kube_pod_container_resource_limits:sum'
        expression: 'sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))'
      }
      {
        record: 'cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits'
        expression: 'kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) ( (kube_pod_status_phase{phase=~"Pending|Running"} == 1) )'
      }
      {
        record: 'namespace_cpu:kube_pod_container_resource_limits:sum'
        expression: 'sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))'
      }
      {
        record: 'namespace_workload_pod:kube_pod_owner:relabel'
        expression: 'max by (cluster, namespace, workload, pod) (  label_replace(    label_replace(      kube_pod_owner{job="kube-state-metrics", owner_kind="ReplicaSet"},      "replicaset", "$1", "owner_name", "(.*)"    ) * on(replicaset, namespace) group_left(owner_name) topk by(replicaset, namespace) (      1, max by (replicaset, namespace, owner_name) (        kube_replicaset_owner{job="kube-state-metrics"}      )    ),    "workload", "$1", "owner_name", "(.*)"  ))'
        labels: {
          workload_type: 'deployment'
        }
      }
      {
        record: 'namespace_workload_pod:kube_pod_owner:relabel'
        expression: 'max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="DaemonSet"},    "workload", "$1", "owner_name", "(.*)"  ))'
        labels: {
          workload_type: 'daemonset'
        }
      }
      {
        record: 'namespace_workload_pod:kube_pod_owner:relabel'
        expression: 'max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="StatefulSet"},    "workload", "$1", "owner_name", "(.*)"  ))'
        labels: {
          workload_type: 'statefulset'
        }
      }
      {
        record: 'namespace_workload_pod:kube_pod_owner:relabel'
        expression: 'max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="Job"},    "workload", "$1", "owner_name", "(.*)"  ))'
        labels: {
          workload_type: 'job'
        }
      }
      {
        record: ':node_memory_MemAvailable_bytes:sum'
        expression: 'sum(  node_memory_MemAvailable_bytes{job="node"} or  (    node_memory_Buffers_bytes{job="node"} +    node_memory_Cached_bytes{job="node"} +    node_memory_MemFree_bytes{job="node"} +    node_memory_Slab_bytes{job="node"}  )) by (cluster)'
      }
      {
        record: 'cluster:node_cpu:ratio_rate5m'
        expression: 'sum(rate(node_cpu_seconds_total{job="node",mode!="idle",mode!="iowait",mode!="steal"}[5m])) by (cluster) /count(sum(node_cpu_seconds_total{job="node"}) by (cluster, instance, cpu)) by (cluster)'
      }
    ]
  }
}

resource nodeRecordingRuleGroupNameWin 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: nodeRecordingRuleGroupNameWinName
  location: azureMonitorWorkspaceLocation
  properties: {
    description: '${RecordingRuleGroupDescriptionWin}${version}'
    scopes: [azureMonitorWorkspaceResourceId,clusterResourceId]
    enabled: enableWindowsRecordingRules
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
        record: 'node:windows_node:sum'
        expression: 'count (windows_system_boot_time_timestamp_seconds{job="windows-exporter"})'
      }
      {
        record: 'node:windows_node_num_cpu:sum'
        expression: 'count by (instance) (sum by (instance, core) (windows_cpu_time_total{job="windows-exporter"}))'
      }
      {
        record: ':windows_node_cpu_utilisation:avg5m'
        expression: '1 - avg(rate(windows_cpu_time_total{job="windows-exporter",mode="idle"}[5m]))'
      }
      {
        record: 'node:windows_node_cpu_utilisation:avg5m'
        expression: '1 - avg by (instance) (rate(windows_cpu_time_total{job="windows-exporter",mode="idle"}[5m]))'
      }
      {
        record: ':windows_node_memory_utilisation:'
        expression: '1 -sum(windows_memory_available_bytes{job="windows-exporter"})/sum(windows_os_visible_memory_bytes{job="windows-exporter"})'
      }
      {
        record: ':windows_node_memory_MemFreeCached_bytes:sum'
        expression: 'sum(windows_memory_available_bytes{job="windows-exporter"} + windows_memory_cache_bytes{job="windows-exporter"})'
      }
      {
        record: 'node:windows_node_memory_totalCached_bytes:sum'
        expression: '(windows_memory_cache_bytes{job="windows-exporter"} + windows_memory_modified_page_list_bytes{job="windows-exporter"} + windows_memory_standby_cache_core_bytes{job="windows-exporter"} + windows_memory_standby_cache_normal_priority_bytes{job="windows-exporter"} + windows_memory_standby_cache_reserve_bytes{job="windows-exporter"})'
      }
      {
        record: ':windows_node_memory_MemTotal_bytes:sum'
        expression: 'sum(windows_os_visible_memory_bytes{job="windows-exporter"})'
      }
      {
        record: 'node:windows_node_memory_bytes_available:sum'
        expression: 'sum by (instance) ((windows_memory_available_bytes{job="windows-exporter"}))'
      }
      {
        record: 'node:windows_node_memory_bytes_total:sum'
        expression: 'sum by (instance) (windows_os_visible_memory_bytes{job="windows-exporter"})'
      }
      {
        record: 'node:windows_node_memory_utilisation:ratio'
        expression: '(node:windows_node_memory_bytes_total:sum - node:windows_node_memory_bytes_available:sum) / scalar(sum(node:windows_node_memory_bytes_total:sum))'
      }
      {
        record: 'node:windows_node_memory_utilisation:'
        expression: '1 - (node:windows_node_memory_bytes_available:sum / node:windows_node_memory_bytes_total:sum)'
      }
      {
        record: 'node:windows_node_memory_swap_io_pages:irate'
        expression: 'irate(windows_memory_swap_page_operations_total{job="windows-exporter"}[5m])'
      }
      {
        record: ':windows_node_disk_utilisation:avg_irate'
        expression: 'avg(irate(windows_logical_disk_read_seconds_total{job="windows-exporter"}[5m]) + irate(windows_logical_disk_write_seconds_total{job="windows-exporter"}[5m]))'
      }
      {
        record: 'node:windows_node_disk_utilisation:avg_irate'
        expression: 'avg by (instance) ((irate(windows_logical_disk_read_seconds_total{job="windows-exporter"}[5m]) + irate(windows_logical_disk_write_seconds_total{job="windows-exporter"}[5m])))'
      }
    ]
  }
}

resource nodeAndKubernetesRecordingRuleGroupNameWin 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: nodeAndKubernetesRecordingRuleGroupWinName
  location: azureMonitorWorkspaceLocation
  properties: {
    description: '${RecordingRuleGroupDescriptionWin}${version}'
    scopes: [azureMonitorWorkspaceResourceId,clusterResourceId]
    enabled: enableWindowsRecordingRules
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
        record: 'node:windows_node_filesystem_usage:'
        expression: 'max by (instance,volume)((windows_logical_disk_size_bytes{job="windows-exporter"} - windows_logical_disk_free_bytes{job="windows-exporter"}) / windows_logical_disk_size_bytes{job="windows-exporter"})'
      }
      {
        record: 'node:windows_node_filesystem_avail:'
        expression: 'max by (instance, volume) (windows_logical_disk_free_bytes{job="windows-exporter"} / windows_logical_disk_size_bytes{job="windows-exporter"})'
      }
      {
        record: ':windows_node_net_utilisation:sum_irate'
        expression: 'sum(irate(windows_net_bytes_total{job="windows-exporter"}[5m]))'
      }
      {
        record: 'node:windows_node_net_utilisation:sum_irate'
        expression: 'sum by (instance) ((irate(windows_net_bytes_total{job="windows-exporter"}[5m])))'
      }
      {
        record: ':windows_node_net_saturation:sum_irate'
        expression: 'sum(irate(windows_net_packets_received_discarded_total{job="windows-exporter"}[5m])) + sum(irate(windows_net_packets_outbound_discarded_total{job="windows-exporter"}[5m]))'
      }
      {
        record: 'node:windows_node_net_saturation:sum_irate'
        expression: 'sum by (instance) ((irate(windows_net_packets_received_discarded_total{job="windows-exporter"}[5m]) + irate(windows_net_packets_outbound_discarded_total{job="windows-exporter"}[5m])))'
      }
      {
        record: 'windows_pod_container_available'
        expression: 'windows_container_available{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'windows_container_total_runtime'
        expression: 'windows_container_cpu_usage_seconds_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'windows_container_memory_usage'
        expression: 'windows_container_memory_usage_commit_bytes{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'windows_container_private_working_set_usage'
        expression: 'windows_container_memory_usage_private_working_set_bytes{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'windows_container_network_received_bytes_total'
        expression: 'windows_container_network_receive_bytes_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'windows_container_network_transmitted_bytes_total'
        expression: 'windows_container_network_transmit_bytes_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)'
      }
      {
        record: 'kube_pod_windows_container_resource_memory_request'
        expression: 'max by (namespace, pod, container) (kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}) * on(container,pod,namespace) (windows_pod_container_available)'
      }
      {
        record: 'kube_pod_windows_container_resource_memory_limit'
        expression: 'kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"} * on(container,pod,namespace) (windows_pod_container_available)'
      }
      {
        record: 'kube_pod_windows_container_resource_cpu_cores_request'
        expression: 'max by (namespace, pod, container) ( kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}) * on(container,pod,namespace) (windows_pod_container_available)'
      }
      {
        record: 'kube_pod_windows_container_resource_cpu_cores_limit'
        expression: 'kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"} * on(container,pod,namespace) (windows_pod_container_available)'
      }
      {
        record: 'namespace_pod_container:windows_container_cpu_usage_seconds_total:sum_rate'
        expression: 'sum by (namespace, pod, container) (rate(windows_container_total_runtime{}[5m]))'
      }
    ]
  }
}

resource uxRecordingRulesRuleGroup 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: uxRecordingRulesRuleGroup
  location: azureMonitorWorkspaceLocation
  properties: {
    description: uxRecordingRulesRuleGroupDescription
    scopes: [
      azureMonitorWorkspaceResourceId
      clusterResourceId
    ]
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
        record: 'ux:pod_cpu_usage:sum_irate'
        expression: '''(sum by (namespace, pod, cluster, microsoft_resourceid) (
    irate(container_cpu_usage_seconds_total{container != "", pod != "", job = "cadvisor"}[5m])
)) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))'''
      }
      {
        record: 'ux:controller_cpu_usage:sum_irate'
        expression: '''sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_cpu_usage:sum_irate
)
'''
      }
      {
        record: 'ux:pod_workingset_memory:sum'
        expression: '''(
        sum by (namespace, pod, cluster, microsoft_resourceid) (
        container_memory_working_set_bytes{container != "", pod != "", job = "cadvisor"}
        )
    ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))'''
      }
      {
        record: 'ux:controller_workingset_memory:sum'
        expression: '''sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_workingset_memory:sum
)'''
      }
      {
        record: 'ux:pod_rss_memory:sum'
        expression: '''(
        sum by (namespace, pod, cluster, microsoft_resourceid) (
        container_memory_rss{container != "", pod != "", job = "cadvisor"}
        )
    ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))'''
      }
      {
        record: 'ux:controller_rss_memory:sum'
        expression: '''sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_rss_memory:sum
)'''
      }
      {
        record: 'ux:pod_container_count:sum'
        expression: '''sum by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
((
sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"})
or sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_init_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"})
)
* on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(
max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
    kube_pod_info{pod != "", job = "kube-state-metrics"}
)
)
)

)'''
      }
      {
        record: 'ux:controller_container_count:sum'
        expression: '''sum by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (
ux:pod_container_count:sum
)'''
      }
      {
        record: 'ux:pod_container_restarts:max'
        expression: '''max by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
((
max by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_container_status_restarts_total{container != "", pod != "", job = "kube-state-metrics"})
or sum by (container, pod, namespace, cluster, microsoft_resourceid) (kube_pod_init_status_restarts_total{container != "", pod != "", job = "kube-state-metrics"})
)
* on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(
max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
    kube_pod_info{pod != "", job = "kube-state-metrics"}
)
)
)

)'''
      }
      {
        record: 'ux:controller_container_restarts:max'
        expression: '''max by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (
ux:pod_container_restarts:max
)'''
      }
      {
        record: 'ux:pod_resource_limit:sum'
        expression: '''(sum by (cluster, pod, namespace, resource, microsoft_resourceid) (
(
    max by (cluster, microsoft_resourceid, pod, container, namespace, resource)
     (kube_pod_container_resource_limits{container != "", pod != "", job = "kube-state-metrics"})
)
)unless (count by (pod, namespace, cluster, resource, microsoft_resourceid)
    (kube_pod_container_resource_limits{container != "", pod != "", job = "kube-state-metrics"})
!= on (pod, namespace, cluster, microsoft_resourceid) group_left()
 sum by (pod, namespace, cluster, microsoft_resourceid)
 (kube_pod_container_info{container != "", pod != "", job = "kube-state-metrics"}) 
)

)* on (namespace, pod, cluster, microsoft_resourceid) group_left (node, created_by_kind, created_by_name)
(
    kube_pod_info{pod != "", job = "kube-state-metrics"}
)'''
      }
      {
        record: 'ux:controller_resource_limit:sum'
        expression: '''sum by (cluster, namespace, created_by_name, created_by_kind, node, resource, microsoft_resourceid) (
ux:pod_resource_limit:sum
)'''
      }
      {
        record: 'ux:controller_pod_phase_count:sum'
        expression: '''sum by (cluster, phase, node, created_by_kind, created_by_name, namespace, microsoft_resourceid) ( (
(kube_pod_status_phase{job="kube-state-metrics",pod!=""})
 or (label_replace((count(kube_pod_deletion_timestamp{job="kube-state-metrics",pod!=""}) by (namespace, pod, cluster, microsoft_resourceid) * count(kube_pod_status_reason{reason="NodeLost", job="kube-state-metrics"} == 0) by (namespace, pod, cluster, microsoft_resourceid)), "phase", "terminating", "", ""))) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(
max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
kube_pod_info{job="kube-state-metrics",pod!=""}
)
)
)'''
      }
      {
        record: 'ux:cluster_pod_phase_count:sum'
        expression: '''sum by (cluster, phase, node, namespace, microsoft_resourceid) (
ux:controller_pod_phase_count:sum
)'''
      }
      {
        record: 'ux:node_cpu_usage:sum_irate'
        expression: '''sum by (instance, cluster, microsoft_resourceid) (
(1 - irate(node_cpu_seconds_total{job="node", mode="idle"}[5m]))
)'''
      }
      {
        record: 'ux:node_memory_usage:sum'
        expression: '''sum by (instance, cluster, microsoft_resourceid) ((
node_memory_MemTotal_bytes{job = "node"}
- node_memory_MemFree_bytes{job = "node"} 
- node_memory_cached_bytes{job = "node"}
- node_memory_buffers_bytes{job = "node"}
))'''
      }
      {
        record: 'ux:node_network_receive_drop_total:sum_irate'
        expression: '''sum by (instance, cluster, microsoft_resourceid) (irate(node_network_receive_drop_total{job="node", device!="lo"}[5m]))'''
      }
      {
        record: 'ux:node_network_transmit_drop_total:sum_irate'
        expression: '''sum by (instance, cluster, microsoft_resourceid) (irate(node_network_transmit_drop_total{job="node", device!="lo"}[5m]))'''
      }
    ]
  }
}

resource uxRecordingRulesRuleGroupWin 'Microsoft.AlertsManagement/prometheusRuleGroups@2023-03-01' = {
  name: uxRecordingRulesRuleGroupWin
  location: azureMonitorWorkspaceLocation
  properties: {
    description: uxRecordingRulesRuleGroupWinDescription
    scopes: [
      azureMonitorWorkspaceResourceId
      clusterResourceId
    ]
    enabled: enableWindowsRecordingRules
    clusterName: clusterName
    interval: 'PT1M'
    rules: [
      {
            "record": "ux:pod_cpu_usage_windows:sum_irate",
            "expression": "sum by (cluster, pod, namespace, node, created_by_kind, created_by_name, microsoft_resourceid) (\n\t(\n\t\tmax by (instance, container_id, cluster, microsoft_resourceid) (\n\t\t\tirate(windows_container_cpu_usage_seconds_total{ container_id != \"\", job = \"windows-exporter\"}[5m])\n\t\t) * on (container_id, cluster, microsoft_resourceid) group_left (container, pod, namespace) (\n\t\t\tmax by (container, container_id, pod, namespace, cluster, microsoft_resourceid) (\n\t\t\t\tkube_pod_container_info{container != \"\", pod != \"\", container_id != \"\", job = \"kube-state-metrics\"}\n\t\t\t)\n\t\t)\n\t) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)\n\t(\n\t\tmax by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (\n\t\t  kube_pod_info{ pod != \"\", job = \"kube-state-metrics\"}\n\t\t)\n\t)\n)"
          },
          {
            "record": "ux:controller_cpu_usage_windows:sum_irate",
            "expression": "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (\nux:pod_cpu_usage_windows:sum_irate\n)\n"
          },
          {
            "record": "ux:pod_workingset_memory_windows:sum",
            "expression": "sum by (cluster, pod, namespace, node, created_by_kind, created_by_name, microsoft_resourceid) (\n\t(\n\t\tmax by (instance, container_id, cluster, microsoft_resourceid) (\n\t\t\twindows_container_memory_usage_private_working_set_bytes{ container_id != \"\", job = \"windows-exporter\"}\n\t\t) * on (container_id, cluster, microsoft_resourceid) group_left (container, pod, namespace) (\n\t\t\tmax by (container, container_id, pod, namespace, cluster, microsoft_resourceid) (\n\t\t\t\tkube_pod_container_info{container != \"\", pod != \"\", container_id != \"\", job = \"kube-state-metrics\"}\n\t\t\t)\n\t\t)\n\t) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)\n\t(\n\t\tmax by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (\n\t\t  kube_pod_info{ pod != \"\", job = \"kube-state-metrics\"}\n\t\t)\n\t)\n)"
          },
          {
            "record": "ux:controller_workingset_memory_windows:sum",
            "expression": "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (\nux:pod_workingset_memory_windows:sum\n)"
          },
          {
            "record": "ux:node_cpu_usage_windows:sum_irate",
            "expression": "sum by (instance, cluster, microsoft_resourceid) (\n(1 - irate(windows_cpu_time_total{job=\"windows-exporter\", mode=\"idle\"}[5m]))\n)"
          },
          {
            "record": "ux:node_memory_usage_windows:sum",
            "expression": "sum by (instance, cluster, microsoft_resourceid) ((\nwindows_os_visible_memory_bytes{job = \"windows-exporter\"}\n- windows_memory_available_bytes{job = \"windows-exporter\"}\n))"
          },
          {
            "record": "ux:node_network_packets_received_drop_total_windows:sum_irate",
            "expression": "sum by (instance, cluster, microsoft_resourceid) (irate(windows_net_packets_received_discarded_total{job=\"windows-exporter\", device!=\"lo\"}[5m]))"
          },
          {
            "record": "ux:node_network_packets_outbound_drop_total_windows:sum_irate",
            "expression": "sum by (instance, cluster, microsoft_resourceid) (irate(windows_net_packets_outbound_discarded_total{job=\"windows-exporter\", device!=\"lo\"}[5m]))"
          }
    ]
  }
}

resource grafanaResourceId_8 'Microsoft.Dashboard/grafana@2022-08-01' = {
  name: split(grafanaResourceId, '/')[8]
  sku: {
    name: grafanaSku
  }
  identity: {
    type: 'SystemAssigned'
  }
  location: grafanaLocation
  properties: {
    grafanaIntegrations: {
      azureMonitorWorkspaceIntegrations: [
        {
          azureMonitorWorkspaceResourceId: azureMonitorWorkspaceResourceId
        }
      ]
    }
  }
}

// Add user's as Grafana Admin for the Grafana instance
resource selfRoleAssignmentGrafana 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: roleNameGuid
  scope: grafanaResourceId_8
  properties: {
    roleDefinitionId: '/subscriptions/${azureMonitorWorkspaceSubscriptionId}/providers/Microsoft.Authorization/roleDefinitions/22926164-76b3-42b3-bc55-97df8dab3e41'
    principalId: grafanaAdminObjectId
  }
}

// Provide Grafana access to the AMW instance
module roleAssignmentGrafanaAMW './nested_grafana_amw_role_assignment.bicep' = {
  name: roleNameGuid
  scope: resourceGroup(split(azureMonitorWorkspaceResourceId, '/')[2], split(azureMonitorWorkspaceResourceId, '/')[4])
  params: {
    azureMonitorWorkspaceSubscriptionId: azureMonitorWorkspaceSubscriptionId
    grafanaPrincipalId: reference(grafanaResourceId_8.id, '2022-08-01', 'Full').identity.principalId
  }
}

