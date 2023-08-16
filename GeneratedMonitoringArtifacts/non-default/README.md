# Turning ON scraping for non-default targets in Azure Montitor metrics addon (AKS clusters)

> NOTE: Doing the below, will increase metrics volume collected from your cluster(s) and ingested into Azure Monitor Workspace(s). Please ensure you have enough quotas in your Azure Monitor Workspace.   Refer [here](https://learn.microsoft.com/en-us/azure/azure-monitor/service-limits#prometheus-metrics), for default quotas & limits.

Azure monitor metrics addon by default collects minimal amount of metrics from Kubernetes clusters to send to Azure Managed Prometheus service. See [here](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-scrape-default) on what is collected by default using addon.

Azure Monitor metrics addon has pre-built configurations to discover & scrape more targets in a Kubernetes cluster. Below sections explain how to turn them ON and consume those metrics, with a few steps.

## Kubernetes API-Server

`kube-api-server` job is turned OFF by default. To collect API-server metrics, do the following -

1. Enable apiserver scraping by specifiying `kube-apiserver = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping apiserver every 30s.
2. Add more metrics to be collected by the `apiserver` target by specifiying  below -
   `kube-apiserver = "apiserver_request_slo_duration_seconds_bucket|apiserver_request_slo_duration_seconds_sum|apiserver_request_slo_duration_seconds_count"` under `default-targets-metrics-keep-list` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap
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

## Kubernetes mixin

By default Azure Managed Prometheus collects metrics used by Kubernetes mixins  and also auto configures few dashboards & recording rules from Kubernetes mixins. In addition to that, you can configure it to collect all other remaining metrics used by Kubernetes mixin usig the steps below.
1. Add more metrics to be collected by the `kubelet` target by specifiying  below -
   `kubelet = "kubelet_volume_stats_capacity_bytes|kubelet_volume_stats_available_bytes|kubelet_volume_stats_inodes_used|kubelet_volume_stats_inodes"` under `default-targets-metrics-keep-list` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap
2. Import all other Kubernetes mixin dashboards from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/kubernetes) into your Grafana instance

## Node-exporter (Linux) mixin

By default Azure Managed Prometheus collects metrics used by node-exporter(Linux) mixins  and also auto configures few dashboards & recording rules from node-exporter(Linux) mixins. In addition to that you can utilize addiitonal dashboards provided by node-exporter mixin usig the steps below.
1. Import all other Kubernetes mixin dashboards from [here](https://github.com/Azure/prometheus-collector/tree/main/GeneratedMonitoringArtifacts/non-default/node-exporter) into your Grafana instance


## Kube-scheduler

`kube-scheduler` job is turned OFF by default. To collect kube-scheduler metrics, do the following -

1. Enable kube-scheduler scraping by specifying `kube-scheduler = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping kube-scheduler every 30s.

## Kube-controller-manager

`kube-controller-manager` job is turned OFF by default. To collect kube-controller-manager metrics, do the following -

1. Enable kube-controller-manager scraping by specifying `kube-controller-manager = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping kube-controller-manager every 30s.

## Cluster Autoscaler

`cluster-autoscaler` job is turned OFF by default. To collect cluster-autoscaler metrics, do the following -

1. Enable cluster-autoscaler scraping by specifying `cluster-autoscaler = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping cluster-autoscaler every 30s.

## Etcd

`etcd` job is turned OFF by default. To collect etcd metrics, do the following -

1. Enable etcd scraping by specifying `etcd = true` under `default-scrape-settings-enabled` in the [settings](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-settings-configmap.yaml) configmap. This will enable scraping etcd every 30s.

> NOTE: You can find settings config map with all the changes above [here](https://github.com/Azure/prometheus-collector/blob/main/GeneratedMonitoringArtifacts/non-default/ama-metrics-settings-configmap.yaml), in case if you just want to use it readily.
