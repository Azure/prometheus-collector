# Configure metrics collection

# open question - should we doc windows targets?

## Default targets
Below is a list of all the default targets which the Azure Monitor Metrics addon can scrape by default. 
The table below also lists the ones that are enabled to be scraped by default (every 30 seconds).

| Key | Type | Default | Description |
|-----|------|----------|-------------|
| kubelet | bool | `true` | when true, automatically scrape kubelet in every node in the k8s cluster without any additional scrape config |
| cadvisor | bool | `true` | `linux only` - when true, automatically scrape cAdvisor in every node in the k8s cluster without any additional scrape config |
| kubestate | bool | `true` | when true, automatically scrape kube-state-metrics in the k8s cluster (installed as a part of the addon) without any additional scrape config |
| nodeexporter | bool | `true` | `linux only`- when true, automatically scrape node metrics without any additional scrape config |
| coredns | bool | `false` | when true, automatically scrape coredns service in the k8s cluster without any additional scrape config |
| kubeproxy | bool | `false` | `linux only` - when true, automatically scrape kube-proxy in every linux node discovered in the k8s cluster without any additional scrape config |
| apiserver | bool | `false` | when true, automatically scrape the kubernetes api server in the k8s cluster without any additional scrape config |
| prometheuscollectorhealth | bool | `false` | when true, automatically scrape info about the prometheus-collector container such as the amount and size of timeseries scraped |
| scrapeTargets.windowsExporter | bool | `false` | when true, will scrape windows node exporter in every windows node discovered in the cluster, without requiring any additional scrape configuration. Note:- Windows-exporter is not installed by the azure monitor metrics addon on windows node(s). You would need to install it by yourselves, before turning this ON |
| scrapeTargets.windowsKubeProxy | bool | `false` | when true, will scrape windows node's kubeproxy service, without requiring any additional scrape configuration, in every windows node discovered in the cluster. Note:- Windows kube-proxy metrics will soon be enabled on windows nodes for AKS clusters |

If you wish to turn on the scraping of the default targets which are not enabled by default, you can create this [configmap](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/deploy/ama-metrics-settings-configmap.yaml) (or edit if you have already created it) and update the targets listed under
'default-scrape-settings-enabled' to true.


## Cluser Alias
The cluster label appended to every timeseries scraped will use the last part of the full ARM resourceID.
ex - if this is the full ARM resourceID - "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername", the cluster label is 'clustername'. 

If you wish to override the cluster label in the time-series scraped, you can update the setting 'cluster_alias' to any string under 'prometheus-collector-settings', in this [configmap](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/deploy/ama-metrics-settings-configmap.yaml). You can either create this configmap or edit if you have already created one. 

The new label will also show up in the grafana instance in the cluster dropdown instead of the default one.
>Note - only alpha-numeric characters are allowed, everything else will be replaced with _ . This is to ensure that different components that consume this label (otel collector, telegraf etc..) will all adhere to the basic alphanumeric + _ convention.