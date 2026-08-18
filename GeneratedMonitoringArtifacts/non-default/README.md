# Turning ON scraping for non-default targets in Azure Montitor metrics addon (AKS clusters)

> NOTE: Doing the below, will increase metrics volume collected from your cluster(s) and ingested into Azure Monitor Workspace(s). Please ensure you have enough quotas in your Azure Monitor Workspace.   Refer [here](https://learn.microsoft.com/en-us/azure/azure-monitor/service-limits#prometheus-metrics), for default quotas & limits.

Azure monitor metrics addon by default collects minimal amount of metrics from Kubernetes clusters to send to Azure Managed Prometheus service. See [here](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-scrape-default) on what is collected by default using addon.

Azure Monitor metrics addon has pre-built configurations to discover & scrape more targets in a Kubernetes cluster. Below sections explain how to turn them ON and consume those metrics, with a few steps.

## Kubernetes API-Server

`kube-api-server` job is turned OFF by default. To collect API-server metrics, do the following -

1. Enable apiserver scraping by specifiying `apiserver = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping apiserver every 30s.
2. Add more metrics to be collected by the `apiserver` target by specifiying  below -
   `apiserver = "apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count"` under `default-targets-metrics-keep-list` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap
3. Import the pre-defined recording rules for apiserver from the template [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/api-server)
4. Import the apiserver Grafana dashboard from [here](https://github.com/Azure/prometheus-collector/tree/vishwa/1paddon/GeneratedMonitoringArtifacts/non-default/api-server) into your Grafana instance

## Kube-proxy

`kubeproxy` job is turned OFF by default. To collect API-server metrics, do the following -

1. Enable kubeproxy scraping by specifying `kubeproxy = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping kubeproxy every 30s.
2. Import the kubeproxy Grafana dashboard from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/kubeproxy) into your Grafana instance

## coredns

`coredns` job is turned OFF by default. To collect API-server metrics, do the following -

1. Enable coredns scraping by specifying `coredns = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping coredns every 30s.
2. Import the coredns Grafana dashboard from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/coredns) into your Grafana instance

## Control plane etcd

The `controlplane-etcd` job is collected by default for AKS clusters that expose control-plane metrics, so no `default-scrape-settings-enabled` / keep-list change is required for the metrics used below (`etcd_mvcc_db_total_size_in_bytes`, `etcd_mvcc_db_total_size_in_use_in_bytes`, `etcd_server_has_leader` are in the control-plane minimal-ingestion keep list). The recording rules and dashboard exist to make the numbers partition-aware.

On **hyperscale** control planes etcd is split into **6 independent partitions** (`etcd`, `etcd-events`, `etcd-leases`, `etcd-nodes`, `etcd-pods`, `etcd-secrets`), each with **3 Raft replicas**. The scrape job emits one flat series per member, labelled only by `instance` (the pod name, e.g. `etcd-nodes-<ccp>-<rand>`) with **no** partition label, so a metric such as `etcd_mvcc_db_total_size_in_bytes` arrives as up to 18 flat series. The two dimensions need **opposite** aggregation:

- The **6 partitions are disjoint** keyspaces → they **SUM** (total DB size).
- The **3 replicas** within a partition are Raft **duplicates** of the same data → they must be **collapsed with `max`**, never summed.

So `sum(etcd_mvcc_db_total_size_in_bytes)` over-reports ~3x and `max(...)` under-reports. The artifacts derive an `etcd_partition` label from `instance` with `label_replace`, do **max-over-replicas first**, then **sum-over-partitions**. Non-hyperscale clusters have a single `etcd` partition and the same math is correct.

> Regex note: in the `label_replace` alternation `^(etcd-events|etcd-leases|etcd-nodes|etcd-pods|etcd-secrets|etcd2|etcd)-.*` the specific `etcd-<name>` branches MUST precede the bare `etcd` branch. Prometheus uses Go RE2 leftmost-first alternation, so a leading bare `etcd` would match `etcd-nodes-...` and wrongly capture just `etcd`. Preserve this ordering everywhere.

1. Import the pre-defined recording rules for etcd from the template [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/etcd) (`etcd-RecordingRules.json`) into your Azure Monitor Workspace.
2. Import the etcd Grafana dashboard (`etcd.json`) from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/etcd) into your Grafana instance. The dashboard prefers the recorded rules above but falls back to the raw `label_replace` expression (via `or`) so it renders even if the rule group is not deployed.

## Kubernetes mixin

By default Azure Managed Prometheus collects metrics used by Kubernetes mixins  and also auto configures few dashboards & recording rules from Kubernetes mixins. In addition to that, you can configure it to collect all other remaining metrics used by Kubernetes mixin usig the steps below.
1. Add more metrics to be collected by the `kubelet` target by specifiying  below -
   `kubelet = "kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_available_bytes|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes"` under `default-targets-metrics-keep-list` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap
2. Import all other Kubernetes mixin dashboards from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/kubernetes) into your Grafana instance

## Node-exporter (Linux) mixin

By default Azure Managed Prometheus collects metrics used by node-exporter(Linux) mixins  and also auto configures few dashboards & recording rules from node-exporter(Linux) mixins. In addition to that you can utilize addiitonal dashboards provided by node-exporter mixin usig the steps below.
1. Import all other Kubernetes mixin dashboards from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/node-exporter) into your Grafana instance
   

> NOTE: You can find settings config map with all the changes above [here](https://github.com/Azure/prometheus-collector/blob/main/GeneratedMonitoringArtifacts/non-default/ama-metrics-settings-configmap.yaml), in case if you just want to use it readily.
