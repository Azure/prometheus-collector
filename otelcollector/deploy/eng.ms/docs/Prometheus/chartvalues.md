# Chart Values for Prometheus-collector

| Key | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| azureKeyVault.name | string | <mark>`Required`</mark> | `""` | name of the azure key vault resource |
| azureKeyVault.clientId | string | <mark>`Required`</mark> | `""` | clientid for a service principal that has access to read the Pfx certificates from keyvault specified above |
| azureKeyVault.clientSecret | string | <mark>`Required`</mark> | `""` | client secret for the above service principal |
| azureKeyVault.pfxCertNames | list of comma seperated strings | <mark>`Required`</mark> | `"{}"` | name of the Pfx certificate(s) - one per metric account |
| azureKeyVault.tenantId | string | <mark>`Required`</mark> | `""` | tenantid for the azure key vault resource |
| azureMetricAccount.defaultAccountName | string | <mark>`Required`</mark> | `""` | default metric account name to ingest metrics into. This will be the account used if metric itself does not have account 'hinting' label. The certificate for this account should be specified in one of the further arguments below here |
| clusterName | string | <mark>`Required`</mark> | `""` | name of the k8s cluster. This will be added as a 'cluster' label for every metric scraped |
| image.pullPolicy | string | Optional | `"IfNotPresent"` |  |
| image.repository | string | Optional | `"mcr.microsoft.com/azuremonitor/containerinsights/cidev"` |  |
| image.tag | string | Optional | `"prometheus-collector-main-0.0.5-09-25-2021-e1c22c83"` |  |
| internalSettings.intEnvironment | bool | Optional | `false` | do not use any of the internal settings. This is for testing purposes |
| mode.advanced | bool | Optional | `false` | if mode.advanced==true (default is false), then it will deploy a daemonset in addition to replica, and move some of the default node targets (kubelet, cadvisor & nodeexporter) to daemonset. On bigger clusters (> 50+ nodes and > 1500+ pods), it is highly recommended to set this to `true`, as this will distribute the metric volumes to individual nodes as nodes & pods scale out & grow. Note:- When this is set to `true`, the `up` metric for the node target will be generated from the replica, so when the node (and daemonset in the node) becomes unvailable), the target availability can still be tracked.
| resources.deployment.limits.cpu | string | Optional | `4` |  |
| resources.deployment.limits.memory | string | Optional | `"7Gi"` |  |
| resources.deployment.requests.cpu | string | Optional | `"1"` |  |
| resources.deployment.requests.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.limits.cpu | string | Optional | `1` |  |
| resources.daemonSet.limits.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.requests.cpu | string | Optional | `"500m"` |  |
| resources.daemonSet.requests.memory | string | Optional | `"1Gi"` |  |
| updateStrategy.daemonSet.maxUnavailable | string | Optional | `"1"` | This can be a number or percentage of pods |
| scrapeTargets.coreDns | bool | Optional | `true` | when true, automatically scrape coredns service in the k8s cluster without any additional scrape config |
| scrapeTargets.kubelet | bool | Optional | `true` | when true, automatically scrape kubelet in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.cAdvisor | bool | Optional | `true` | when true, automatically scrape cAdvisor in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeProxy | bool | Optional | `true` | `linux only` - when true, automatically scrape kube-proxy in every linux node discovered in the k8s cluster without any additional scrape config |
| scrapeTargets.apiServer | bool | Optional | `true` | when true, automatically scrape the kubernetes api server in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeState | bool | Optional | `true` | when true, automatically install kube-state-metrics and scrape kube-state-metrics in the k8s cluster without any additional scrape config |
| scrapeTargets.nodeExporter | bool | Optional | `true` | `linux only` - when true, automatically install prometheus-node-exporter in every linux node in the k8s cluster and scrape node metrics without any additional scrape config |
| scrapeTargets.prometheusCollectorHealth | bool | Optional | `true` | when true, automatically scrape info about the Prometheus-Collector such as the amount and size of timeseries scraped |
| scrapeTargets.windowsExporter | bool | Optional | `false` | `windows only` - when true, will scrape windows node exporter in every windows node discovered in the cluster, without requiring any additional scrape configuration. Note:- Windows-exporter is not installed by this tool on windows node(s). You would need to install it by yourselves, before turning this ON |
| scrapeTargets.windowsKubeProxy | bool | Optional | `false` | `windows only` - when true, will scrape windows node's kubeproxy service, without requiring any additional scrape configuration, in every windows node discovered in the cluster. Note:- Windows kube-proxy metrics will soon be enabled on windows nodes for AKS clusters |

----------------------------------------------