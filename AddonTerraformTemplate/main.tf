resource "azurerm_resource_group" "rg" {
  location = var.resource_group_location
  name     = "defaultPrometheusOnboardingResourceGroup"
}

resource "azurerm_kubernetes_cluster" "k8s" {
  location            = azurerm_resource_group.rg.location
  name                = var.cluster_name
  resource_group_name = azurerm_resource_group.rg.name

  dns_prefix = var.dns_prefix
  tags = {
    Environment = "Development"
  }

  default_node_pool {
    name       = "agentpool"
    vm_size    = "Standard_D2_v2"
    node_count = var.agent_count
  }

  monitor_metrics {
    annotations_allowed = var.metric_annotations_allowlist
    labels_allowed      = var.metric_labels_allowlist
  }

  network_profile {
    network_plugin    = "kubenet"
    load_balancer_sku = "standard"
  }

  identity {
    type = "SystemAssigned"
  }
}

resource "azurerm_monitor_workspace" "amw" {
  name                = var.monitor_workspace_name
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
}

resource "azurerm_monitor_data_collection_endpoint" "dce" {
  name                = substr("MSProm-${azurerm_resource_group.rg.location}-${var.cluster_name}", 0, min(44, length("MSProm-${azurerm_resource_group.rg.location}-${var.cluster_name}")))
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  kind                = "Linux"
}

# Logic to determine region mismatch
locals {
  dce_region_mismatch = var.cluster_region != var.amw_region
}

# Create another DCE if the regions don't match and is_private_cluster is true
resource "azurerm_monitor_data_collection_endpoint" "dce_mismatch" {
  count               = (local.dce_region_mismatch && var.is_private_cluster) ? 1 : 0
  name                = substr("MSProm-PL-${azurerm_resource_group.rg.location}-${var.cluster_name}", 0, min(44, length("MSProm-PL-${azurerm_resource_group.rg.location}-${var.cluster_name}")))
  resource_group_name = azurerm_resource_group.rg.name
  location            = var.cluster_region
  kind                = "Linux"
}

resource "azurerm_monitor_data_collection_rule" "dcr" {
  name                        = substr("MSProm-${azurerm_resource_group.rg.location}-${var.cluster_name}", 0, min(64, length("MSProm-${azurerm_resource_group.rg.location}-${var.cluster_name}")))
  resource_group_name         = azurerm_resource_group.rg.name
  location                    = azurerm_resource_group.rg.location
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.dce.id
  kind                        = "Linux"

  destinations {
    monitor_account {
      monitor_account_id = azurerm_monitor_workspace.amw.id
      name               = "MonitoringAccount1"
    }
  }

  data_flow {
    streams      = ["Microsoft-PrometheusMetrics"]
    destinations = ["MonitoringAccount1"]
  }

  data_sources {
    prometheus_forwarder {
      streams = ["Microsoft-PrometheusMetrics"]
      name    = "PrometheusDataSource"
    }
  }

  description = "DCR for Azure Monitor Metrics Profile (Managed Prometheus)"
  depends_on = [
    azurerm_monitor_data_collection_endpoint.dce
  ]
}

resource "azurerm_monitor_data_collection_rule_association" "dcra" {
  name                    = "MSProm-${azurerm_resource_group.rg.location}-${var.cluster_name}"
  target_resource_id      = azurerm_kubernetes_cluster.k8s.id
  data_collection_rule_id = azurerm_monitor_data_collection_rule.dcr.id
  description             = "Association of data collection rule. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_rule.dcr
  ]
}


# This association is required only if the cluster is private and the DCE region differs from the cluster region.
# In this case, we associate the cluster with the DCE created in the cluster's region (dce_mismatch).
# If there is no region mismatch, this resource is not created.
resource "azurerm_monitor_data_collection_rule_association" "dcra_mismatch" {
  count                       = (local.dce_region_mismatch && var.is_private_cluster) ? 1 : 0
  target_resource_id          = azurerm_kubernetes_cluster.k8s.id
  data_collection_endpoint_id = azurerm_monitor_data_collection_endpoint.dce_mismatch[0].id
  description                 = "Association of data collection endpoint for private link clusters. Deleting this association will break the data collection for this AKS Cluster."
  depends_on = [
    azurerm_monitor_data_collection_endpoint.dce_mismatch
  ]
}

resource "azurerm_dashboard_grafana" "grafana" {
  name                = var.grafana_name
  resource_group_name = azurerm_resource_group.rg.name
  location            = var.grafana_location
  grafana_major_version = var.grafana_version

  identity {
    type = "SystemAssigned"
  }

  azure_monitor_workspace_integrations {
    resource_id = azurerm_monitor_workspace.amw.id
  }
}

resource "azurerm_role_assignment" "datareaderrole" {
  scope              = azurerm_monitor_workspace.amw.id
  role_definition_id = "/subscriptions/${split("/", azurerm_monitor_workspace.amw.id)[2]}/providers/Microsoft.Authorization/roleDefinitions/b0d8363b-8ddd-447d-831f-62ca05bff136"
  principal_id       = azurerm_dashboard_grafana.grafana.identity.0.principal_id
}


resource "azurerm_monitor_alert_prometheus_rule_group" "node_recording_rules_rule_group" {
  name                = "NodeRecordingRulesRuleGroup-${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "Node Recording Rules Rule Group"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [azurerm_monitor_workspace.amw.id,azurerm_kubernetes_cluster.k8s.id]

  rule {
    enabled    = true
    record     = "instance:node_num_cpu:sum"
    expression = <<EOF
count without (cpu, mode) (  node_cpu_seconds_total{job="node",mode="idle"})
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_cpu_utilisation:rate5m"
    expression = <<EOF
1 - avg without (cpu) (  sum without (mode) (rate(node_cpu_seconds_total{job="node", mode=~"idle|iowait|steal"}[5m])))
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_load1_per_cpu:ratio"
    expression = <<EOF
(  node_load1{job="node"}/  instance:node_num_cpu:sum{job="node"})
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_memory_utilisation:ratio"
    expression = <<EOF
1 - (  (    node_memory_MemAvailable_bytes{job="node"}    or    (      node_memory_Buffers_bytes{job="node"}      +      node_memory_Cached_bytes{job="node"}      +      node_memory_MemFree_bytes{job="node"}      +      node_memory_Slab_bytes{job="node"}    )  )/  node_memory_MemTotal_bytes{job="node"})
EOF
  }
  rule {
    enabled = true

    record     = "instance:node_vmstat_pgmajfault:rate5m"
    expression = <<EOF
rate(node_vmstat_pgmajfault{job="node"}[5m])
EOF
  }
  rule {
    enabled    = true
    record     = "instance_device:node_disk_io_time_seconds:rate5m"
    expression = <<EOF
rate(node_disk_io_time_seconds_total{job="node", device!=""}[5m])
EOF
  }
  rule {
    enabled    = true
    record     = "instance_device:node_disk_io_time_weighted_seconds:rate5m"
    expression = <<EOF
rate(node_disk_io_time_weighted_seconds_total{job="node", device!=""}[5m])
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_network_receive_bytes_excluding_lo:rate5m"
    expression = <<EOF
sum without (device) (  rate(node_network_receive_bytes_total{job="node", device!="lo"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_network_transmit_bytes_excluding_lo:rate5m"
    expression = <<EOF
sum without (device) (  rate(node_network_transmit_bytes_total{job="node", device!="lo"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_network_receive_drop_excluding_lo:rate5m"
    expression = <<EOF
sum without (device) (  rate(node_network_receive_drop_total{job="node", device!="lo"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "instance:node_network_transmit_drop_excluding_lo:rate5m"
    expression = <<EOF
sum without (device) (  rate(node_network_transmit_drop_total{job="node", device!="lo"}[5m]))
EOF
  }
}

resource "azurerm_monitor_alert_prometheus_rule_group" "kubernetes_recording_rules_rule_group" {
  name                = "KubernetesRecordingRulesRuleGroup-${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "Kubernetes Recording Rules Rule Group"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [azurerm_monitor_workspace.amw.id,azurerm_kubernetes_cluster.k8s.id]

  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate"
    expression = <<EOF
sum by (cluster, namespace, pod, container) (  irate(container_cpu_usage_seconds_total{job="cadvisor", image!=""}[5m])) * on (cluster, namespace, pod) group_left(node) topk by (cluster, namespace, pod) (  1, max by(cluster, namespace, pod, node) (kube_pod_info{node!=""}))
EOF
  }
  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_working_set_bytes"
    expression = <<EOF
container_memory_working_set_bytes{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))
EOF
  }
  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_rss"
    expression = <<EOF
container_memory_rss{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))
EOF
  }
  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_cache"
    expression = <<EOF
container_memory_cache{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))
EOF
  }
  rule {
    enabled    = true
    record     = "node_namespace_pod_container:container_memory_swap"
    expression = <<EOF
container_memory_swap{job="cadvisor", image!=""}* on (namespace, pod) group_left(node) topk by(namespace, pod) (1,  max by(namespace, pod, node) (kube_pod_info{node!=""}))
EOF
  }
  rule {
    enabled    = true
    record     = "cluster:namespace:pod_memory:active:kube_pod_container_resource_requests"
    expression = <<EOF
kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_memory:kube_pod_container_resource_requests:sum"
    expression = <<EOF
sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))
EOF
  }
  rule {
    enabled    = true
    record     = "cluster:namespace:pod_cpu:active:kube_pod_container_resource_requests"
    expression = <<EOF
kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_cpu:kube_pod_container_resource_requests:sum"
    expression = <<EOF
sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))
EOF
  }
  rule {
    enabled    = true
    record     = "cluster:namespace:pod_memory:active:kube_pod_container_resource_limits"
    expression = <<EOF
kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) (  (kube_pod_status_phase{phase=~"Pending|Running"} == 1))
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_memory:kube_pod_container_resource_limits:sum"
    expression = <<EOF
sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))
EOF
  }
  rule {
    enabled    = true
    record     = "cluster:namespace:pod_cpu:active:kube_pod_container_resource_limits"
    expression = <<EOF
kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}  * on (namespace, pod, cluster)group_left() max by (namespace, pod, cluster) ( (kube_pod_status_phase{phase=~"Pending|Running"} == 1) )
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_cpu:kube_pod_container_resource_limits:sum"
    expression = <<EOF
sum by (namespace, cluster) (    sum by (namespace, pod, cluster) (        max by (namespace, pod, container, cluster) (          kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"}        ) * on(namespace, pod, cluster) group_left() max by (namespace, pod, cluster) (          kube_pod_status_phase{phase=~"Pending|Running"} == 1        )    ))
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<EOF
max by (cluster, namespace, workload, pod) (  label_replace(    label_replace(      kube_pod_owner{job="kube-state-metrics", owner_kind="ReplicaSet"},      "replicaset", "$1", "owner_name", "(.*)"    ) * on(replicaset, namespace) group_left(owner_name) topk by(replicaset, namespace) (      1, max by (replicaset, namespace, owner_name) (        kube_replicaset_owner{job="kube-state-metrics"}      )    ),    "workload", "$1", "owner_name", "(.*)"  ))
EOF
    labels = {
      workload_type = "deployment"
    }
  }
  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<EOF
max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="DaemonSet"},    "workload", "$1", "owner_name", "(.*)"  ))
EOF
    labels = {
      workload_type = "daemonset"
    }
  }
  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<EOF
max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="StatefulSet"},    "workload", "$1", "owner_name", "(.*)"  ))
EOF
    labels = {
      workload_type = "statefulset"
    }
  }
  rule {
    enabled    = true
    record     = "namespace_workload_pod:kube_pod_owner:relabel"
    expression = <<EOF
max by (cluster, namespace, workload, pod) (  label_replace(    kube_pod_owner{job="kube-state-metrics", owner_kind="Job"},    "workload", "$1", "owner_name", "(.*)"  ))
EOF
    labels = {
      workload_type = "job"
    }
  }
  rule {
    enabled    = true
    record     = ":node_memory_MemAvailable_bytes:sum"
    expression = <<EOF
sum(  node_memory_MemAvailable_bytes{job="node"} or  (    node_memory_Buffers_bytes{job="node"} +    node_memory_Cached_bytes{job="node"} +    node_memory_MemFree_bytes{job="node"} +    node_memory_Slab_bytes{job="node"}  )) by (cluster)
EOF
  }
  rule {
    enabled    = true
    record     = "cluster:node_cpu:ratio_rate5m"
    expression = <<EOF
sum(rate(node_cpu_seconds_total{job="node",mode!="idle",mode!="iowait",mode!="steal"}[5m])) by (cluster) /count(sum(node_cpu_seconds_total{job="node"}) by (cluster, instance, cpu)) by (cluster)
EOF
  }
}

resource "azurerm_monitor_alert_prometheus_rule_group" "node_and_kubernetes_recording_rules_rule_group_win" {
  name                = "NodeAndKubernetesRecordingRulesRuleGroup-Win-${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "Node and Kubernetes Recording Rules Rule Group for Windows Nodes"
  rule_group_enabled  = var.enable_windows_recording_rules
  interval            = "PT1M"
  scopes              = [azurerm_monitor_workspace.amw.id,azurerm_kubernetes_cluster.k8s.id]

  rule {
    enabled    = true
    record     = "node:windows_node_filesystem_usage:"
    expression = <<EOF
max by (instance,volume)((windows_logical_disk_size_bytes{job="windows-exporter"} - windows_logical_disk_free_bytes{job="windows-exporter"}) / windows_logical_disk_size_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_filesystem_avail:"
    expression = <<EOF
max by (instance, volume) (windows_logical_disk_free_bytes{job="windows-exporter"} / windows_logical_disk_size_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_net_utilisation:sum_irate"
    expression = <<EOF
sum(irate(windows_net_bytes_total{job="windows-exporter"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_net_utilisation:sum_irate"
    expression = <<EOF
sum by (instance) ((irate(windows_net_bytes_total{job="windows-exporter"}[5m])))
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_net_saturation:sum_irate"
    expression = <<EOF
sum(irate(windows_net_packets_received_discarded_total{job="windows-exporter"}[5m])) + sum(irate(windows_net_packets_outbound_discarded_total{job="windows-exporter"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_net_saturation:sum_irate"
    expression = <<EOF
sum by (instance) ((irate(windows_net_packets_received_discarded_total{job="windows-exporter"}[5m]) + irate(windows_net_packets_outbound_discarded_total{job="windows-exporter"}[5m])))
EOF
  }
  rule {
    enabled    = true
    record     = "windows_pod_container_available"
    expression = <<EOF
windows_container_available{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "windows_container_total_runtime"
    expression = <<EOF
windows_container_cpu_usage_seconds_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "windows_container_memory_usage"
    expression = <<EOF
windows_container_memory_usage_commit_bytes{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "windows_container_private_working_set_usage"
    expression = <<EOF
windows_container_memory_usage_private_working_set_bytes{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "windows_container_network_received_bytes_total"
    expression = <<EOF
windows_container_network_receive_bytes_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "windows_container_network_transmitted_bytes_total"
    expression = <<EOF
windows_container_network_transmit_bytes_total{job="windows-exporter", container_id != ""} * on(container_id) group_left(container, pod, namespace) max(kube_pod_container_info{job="kube-state-metrics", container_id != ""}) by(container, container_id, pod, namespace)
EOF
  }
  rule {
    enabled    = true
    record     = "kube_pod_windows_container_resource_memory_request"
    expression = <<EOF
max by (namespace, pod, container) (kube_pod_container_resource_requests{resource="memory",job="kube-state-metrics"}) * on(container,pod,namespace) (windows_pod_container_available)
EOF
  }
  rule {
    enabled    = true
    record     = "kube_pod_windows_container_resource_memory_limit"
    expression = <<EOF
kube_pod_container_resource_limits{resource="memory",job="kube-state-metrics"} * on(container,pod,namespace) (windows_pod_container_available)
EOF
  }
  rule {
    enabled    = true
    record     = "kube_pod_windows_container_resource_cpu_cores_request"
    expression = <<EOF
max by (namespace, pod, container) ( kube_pod_container_resource_requests{resource="cpu",job="kube-state-metrics"}) * on(container,pod,namespace) (windows_pod_container_available)
EOF
  }
  rule {
    enabled    = true
    record     = "kube_pod_windows_container_resource_cpu_cores_limit"
    expression = <<EOF
kube_pod_container_resource_limits{resource="cpu",job="kube-state-metrics"} * on(container,pod,namespace) (windows_pod_container_available)
EOF
  }
  rule {
    enabled    = true
    record     = "namespace_pod_container:windows_container_cpu_usage_seconds_total:sum_rate"
    expression = <<EOF
sum by (namespace, pod, container) (rate(windows_container_total_runtime{}[5m]))
EOF
  }
}

resource "azurerm_monitor_alert_prometheus_rule_group" "node_recording_rules_rule_group_win" {
  name                = "NodeRecordingRulesRuleGroup-Win-${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "Node and Kubernetes Recording Rules Rule Group for Windows Nodes"
  rule_group_enabled  = var.enable_windows_recording_rules
  interval            = "PT1M"
  scopes              = [azurerm_monitor_workspace.amw.id,azurerm_kubernetes_cluster.k8s.id]

  rule {
    enabled    = true
    record     = "node:windows_node:sum"
    expression = <<EOF
count (windows_system_boot_time_timestamp_seconds{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_num_cpu:sum"
    expression = <<EOF
count by (instance) (sum by (instance, core) (windows_cpu_time_total{job="windows-exporter"}))
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_cpu_utilisation:avg5m"
    expression = <<EOF
1 - avg(rate(windows_cpu_time_total{job="windows-exporter",mode="idle"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_cpu_utilisation:avg5m"
    expression = <<EOF
1 - avg by (instance) (rate(windows_cpu_time_total{job="windows-exporter",mode="idle"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_memory_utilisation:"
    expression = <<EOF
1 -sum(windows_memory_available_bytes{job="windows-exporter"})/sum(windows_os_visible_memory_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_memory_MemFreeCached_bytes:sum"
    expression = <<EOF
sum(windows_memory_available_bytes{job="windows-exporter"} + windows_memory_cache_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_totalCached_bytes:sum"
    expression = <<EOF
(windows_memory_cache_bytes{job="windows-exporter"} + windows_memory_modified_page_list_bytes{job="windows-exporter"} + windows_memory_standby_cache_core_bytes{job="windows-exporter"} + windows_memory_standby_cache_normal_priority_bytes{job="windows-exporter"} + windows_memory_standby_cache_reserve_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_memory_MemTotal_bytes:sum"
    expression = <<EOF
sum(windows_os_visible_memory_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_bytes_available:sum"
    expression = <<EOF
sum by (instance) ((windows_memory_available_bytes{job="windows-exporter"}))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_bytes_total:sum"
    expression = <<EOF
sum by (instance) (windows_os_visible_memory_bytes{job="windows-exporter"})
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_utilisation:ratio"
    expression = <<EOF
(node:windows_node_memory_bytes_total:sum - node:windows_node_memory_bytes_available:sum) / scalar(sum(node:windows_node_memory_bytes_total:sum))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_utilisation:"
    expression = <<EOF
1 - (node:windows_node_memory_bytes_available:sum / node:windows_node_memory_bytes_total:sum)
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_memory_swap_io_pages:irate"
    expression = <<EOF
irate(windows_memory_swap_page_operations_total{job="windows-exporter"}[5m])
EOF
  }
  rule {
    enabled    = true
    record     = ":windows_node_disk_utilisation:avg_irate"
    expression = <<EOF
avg(irate(windows_logical_disk_read_seconds_total{job="windows-exporter"}[5m]) + irate(windows_logical_disk_write_seconds_total{job="windows-exporter"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "node:windows_node_disk_utilisation:avg_irate"
    expression = <<EOF
avg by (instance) ((irate(windows_logical_disk_read_seconds_total{job="windows-exporter"}[5m]) + irate(windows_logical_disk_write_seconds_total{job="windows-exporter"}[5m])))
EOF
  }
}

resource "azurerm_monitor_alert_prometheus_rule_group" "ux_recording_rules_rule_group" {
  name                = "UXRecordingRulesRuleGroup - ${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "UX recording rules for Linux"
  rule_group_enabled  = true
  interval            = "PT1M"
  scopes              = [azurerm_monitor_workspace.amw.id, azurerm_kubernetes_cluster.k8s.id]

  rule {
    enabled    = true
    record     = "ux:pod_cpu_usage:sum_irate"
    expression = <<EOF
(sum by (namespace, pod, cluster, microsoft_resourceid) (
    irate(container_cpu_usage_seconds_total{container != "", pod != "", job = "cadvisor"}[5m])
)) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_cpu_usage:sum_irate"
    expression = <<EOF
sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_cpu_usage:sum_irate
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:pod_workingset_memory:sum"
    expression = <<EOF
(
        sum by (namespace, pod, cluster, microsoft_resourceid) (
        container_memory_working_set_bytes{container != "", pod != "", job = "cadvisor"}
        )
    ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_workingset_memory:sum"
    expression = <<EOF
sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_workingset_memory:sum
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:pod_rss_memory:sum"
    expression = <<EOF
(
        sum by (namespace, pod, cluster, microsoft_resourceid) (
        container_memory_rss{container != "", pod != "", job = "cadvisor"}
        )
    ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (kube_pod_info{pod != "", job = "kube-state-metrics"}))
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_rss_memory:sum"
    expression = <<EOF
sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (
ux:pod_rss_memory:sum
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:pod_container_count:sum"
    expression = <<EOF
sum by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
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

)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_container_count:sum"
    expression = <<EOF
sum by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (
ux:pod_container_count:sum
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:pod_container_restarts:max"
    expression = <<EOF
max by (node, created_by_name, created_by_kind, namespace, cluster, pod, microsoft_resourceid) (
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

)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_container_restarts:max"
    expression = <<EOF
max by (node, created_by_name, created_by_kind, namespace, cluster, microsoft_resourceid) (
ux:pod_container_restarts:max
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:pod_resource_limit:sum"
    expression = <<EOF
(sum by (cluster, pod, namespace, resource, microsoft_resourceid) (
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
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_resource_limit:sum"
    expression = <<EOF
sum by (cluster, namespace, created_by_name, created_by_kind, node, resource, microsoft_resourceid) (
ux:pod_resource_limit:sum
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:controller_pod_phase_count:sum"
    expression = <<EOF
sum by (cluster, phase, node, created_by_kind, created_by_name, namespace, microsoft_resourceid) ( (
(kube_pod_status_phase{job="kube-state-metrics",pod!=""})
 or (label_replace((count(kube_pod_deletion_timestamp{job="kube-state-metrics",pod!=""}) by (namespace, pod, cluster, microsoft_resourceid) * count(kube_pod_status_reason{reason="NodeLost", job="kube-state-metrics"} == 0) by (namespace, pod, cluster, microsoft_resourceid)), "phase", "terminating", "", ""))) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind)
(
max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
kube_pod_info{job="kube-state-metrics",pod!=""}
)
)
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:cluster_pod_phase_count:sum"
    expression = <<EOF
sum by (cluster, phase, node, namespace, microsoft_resourceid) (
ux:controller_pod_phase_count:sum
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:node_cpu_usage:sum_irate"
    expression = <<EOF
sum by (instance, cluster, microsoft_resourceid) (
(1 - irate(node_cpu_seconds_total{job="node", mode="idle"}[5m]))
)
EOF
  }
  rule {
    enabled    = true
    record     = "ux:node_memory_usage:sum"
    expression = <<EOF
sum by (instance, cluster, microsoft_resourceid) ((
node_memory_MemTotal_bytes{job = "node"}
- node_memory_MemFree_bytes{job = "node"} 
- node_memory_cached_bytes{job = "node"}
- node_memory_buffers_bytes{job = "node"}
))
EOF
  }
  rule {
    enabled    = true
    record     = "ux:node_network_receive_drop_total:sum_irate"
    expression = <<EOF
sum by (instance, cluster, microsoft_resourceid) (irate(node_network_receive_drop_total{job="node", device!="lo"}[5m]))
EOF
  }
  rule {
    enabled    = true
    record     = "ux:node_network_transmit_drop_total:sum_irate"
    expression = <<EOF
sum by (instance, cluster, microsoft_resourceid) (irate(node_network_transmit_drop_total{job="node", device!="lo"}[5m]))
EOF
  }
}

resource "azurerm_monitor_alert_prometheus_rule_group" "ux_recording_rules_rule_group_windows" {
  name                = "UXRecordingRulesRuleGroup-Win - ${var.cluster_name}"
  location            = azurerm_resource_group.rg.location
  resource_group_name = azurerm_resource_group.rg.name
  cluster_name        = var.cluster_name
  description         = "UX recording rules for Windows"
  rule_group_enabled  = var.enable_windows_recording_rules
  interval            = "PT1M"
  scopes              = [
    azurerm_monitor_workspace.amw.id,
    azurerm_kubernetes_cluster.k8s.id
  ]

  rule {
    record     = "ux:pod_cpu_usage_windows:sum_irate"
    expression = <<EOF
sum by (cluster, pod, namespace, node, created_by_kind, created_by_name, microsoft_resourceid) (
  (
    max by (instance, container_id, cluster, microsoft_resourceid) (
      irate(windows_container_cpu_usage_seconds_total{ container_id != "", job = "windows-exporter"}[5m])
    ) * on (container_id, cluster, microsoft_resourceid) group_left (container, pod, namespace) (
      max by (container, container_id, pod, namespace, cluster, microsoft_resourceid) (
        kube_pod_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"}
      )
    )
  ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind) (
    max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
      kube_pod_info{ pod != "", job = "kube-state-metrics"}
    )
  )
)
EOF
  }

  rule {
    record     = "ux:controller_cpu_usage_windows:sum_irate"
    expression = "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (ux:pod_cpu_usage_windows:sum_irate)"
  }

  rule {
    record     = "ux:pod_workingset_memory_windows:sum"
    expression = <<EOF
sum by (cluster, pod, namespace, node, created_by_kind, created_by_name, microsoft_resourceid) (
  (
    max by (instance, container_id, cluster, microsoft_resourceid) (
      windows_container_memory_usage_private_working_set_bytes{ container_id != "", job = "windows-exporter"}
    ) * on (container_id, cluster, microsoft_resourceid) group_left (container, pod, namespace) (
      max by (container, container_id, pod, namespace, cluster, microsoft_resourceid) (
        kube_pod_container_info{container != "", pod != "", container_id != "", job = "kube-state-metrics"}
      )
    )
  ) * on (pod, namespace, cluster, microsoft_resourceid) group_left (node, created_by_name, created_by_kind) (
    max by (node, created_by_name, created_by_kind, pod, namespace, cluster, microsoft_resourceid) (
      kube_pod_info{ pod != "", job = "kube-state-metrics"}
    )
  )
)
EOF
  }

  rule {
    record     = "ux:controller_workingset_memory_windows:sum"
    expression = "sum by (namespace, node, cluster, created_by_name, created_by_kind, microsoft_resourceid) (ux:pod_workingset_memory_windows:sum)"
  }

  rule {
    record     = "ux:node_cpu_usage_windows:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) ((1 - irate(windows_cpu_time_total{job=\"windows-exporter\", mode=\"idle\"}[5m])))"
  }

  rule {
    record     = "ux:node_memory_usage_windows:sum"
    expression = "sum by (instance, cluster, microsoft_resourceid) ((windows_os_visible_memory_bytes{job = \"windows-exporter\"}- windows_memory_available_bytes{job = \"windows-exporter\"}))"
  }

  rule {
    record     = "ux:node_network_packets_received_drop_total_windows:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) (irate(windows_net_packets_received_discarded_total{job=\"windows-exporter\", device!=\"lo\"}[5m]))"
  }

  rule {
    record     = "ux:node_network_packets_outbound_drop_total_windows:sum_irate"
    expression = "sum by (instance, cluster, microsoft_resourceid) (irate(windows_net_packets_outbound_discarded_total{job=\"windows-exporter\", device!=\"lo\"}[5m]))"
  }
}
  