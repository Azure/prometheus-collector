# Scraping Metrics at High Scale

## Advanced Mode: Scraping with a Daemonset

* For a cluster with a large number of nodes and pods running on it and the default scrape targets enabled, `advanced` mode is recommended. When deploying the helm chart, add the parameter `--set mode.advanced=true`. This runs a daemonset to scrape all node-wide targets such as cAdvisor, kubelet, and node-exporter, and also the replicaset as usual to still scrape all other targets. This off-loads some of the work to each node running a prometheus-collector instance instead of a single pod scraping everything.
* Custom scrape targets can also be off-loaded to the daemonset. A configmap similar to the regular configmap can be created to have static scrape configs on each node. This configmap should have the name `<helm release name>-prometheus-config-node` for scrape targets for Linux and `<helm release name>-prometheus-config-node-windows` for scrape targets for Windows. Note that the scrape config should only target a single node; otherwise each node will try to scrape all targets. The node-exporter config is a good example of using the `$NODE_IP` environment variable (already set for every prometheus-collector pod) to target a specific endpoint on the node:

  ```yaml
  - job_name: node
    scrape_interval: 30s
    scheme: http
    metrics_path: /metrics
    relabel_configs:
    - source_labels: [__metrics_path__]
      regex: (.*)
      target_label: metrics_path
    - source_labels: [__address__]
      replacement: '$NODE_NAME'
      target_label: instance
    static_configs:
    - targets: ['$NODE_IP:9100']
  ```

* Custom scrape targets can follow the same format using `static_configs` with targets using the `$NODE_IP` environment variable and specifying the port to scrape. Each pod of the daemonset will take the config and scrape and send the metrics for that node.
* Add the configmap by creating your Prometheus config in a file called `prometheus-config` and run below for Linux scrape targets:
  ```
  kubectl create configmap <prometheus collector chart release name>-prometheus-config-node --from-file=prometheus-config -n <same namespace as prometheus collector namespace>
  ```

* Repeat the same steps for the Windows scrape targets by creating your Prometheus config in a file called `prometheus-config` and running the following command:

  ```bash
  kubectl create configmap <prometheus collector chart release name>-prometheus-config-node-windows --from-file=prometheus-config -n <same namespace as prometheus collector namespace>
  ```

**<em>Note that the file name has to be prometheus-config for the --from-file parameter since we rely on the data in the configmap to be prometheus-config</em>**

## Scraping with Multiple Prometheus-Collector Instances
* Even with off-loading some jobs to a daemonset using advanced mode, there still may be an extremely high load of metrics being scraped from the replicaset. This requires multiple deployments of the prometheus-collector and multiple corresponding custom scrape configs in configmaps, with different jobs split up between configmaps.
* A single instance can handle up to around `2.7 million timeseries per minute` and `4 GB of timeseries per minute`. After this, multiple instances will need to be used with scrape jobs split between them in the custom configmaps.
* Note this is also the same limit that one daemonset pod (Linux or Windows) can handle.

## Viewing The Volume of Timeseries Scraped and Sent
* To know how many timeseries and bytes you are sending, you can check usage by instance in the `Prometheus-Collector Health` default dashboard. This shows the historical number of timeseries and bytes that have been scraped and sent.
* The variable selectors can be adjusted to view the total timeseries and bytes scraped for the whole cluster, for an individual release, the replicaset and daemonset, and individual nodes. To view if you are close to the single instance limit of 2.7 million timeseries per minute and 4 GB of timeseries per minute, select the release name for that instance and `replicaset` as the `controller_type`.
* If the amount of metrics sent is already high enough that it may be over the limit, you can also port-forward to check the number of timeseries and bytes the instance is sending for that previous minute.

  ```bash
  kubectl port-forward <prometheus-collector replicaset pod name> -n <prometheus-collector pod namespace> 2234:2234
  ```

  Curl `http://127.0.0.1:2234/metrics` to see the volume metrics for that minute.
* The metrics are:

  | Name | Description
  | --- | --- |
  | `timeseries_received_per_minute` | Number of timseries scraped
  | `timeseries_sent_per_minute`  | Number of timeseries sent to storage
  | `bytes_sent_per_minute` | Number of bytes of timeseries sent to storage

* The metric `timeseries_received_per_minute` may not exactly equal `timeseries_sent_per_minute` in the same minute. However, if there is a large difference and `timeseries_received_per_minute` is over 2.7 million timeseries per minute, not all your timeseries may be sending, and you will need to deploy multiple instances of the prometheus-collector.

## Configuring Multiple Instances

* Follow the regular HELM deployment instructions for the first instance with advanced mode enabled and whichever default scrape targets you wish to have enabled or disabled. Custom scrape targets will be in the configmap `<chart release name>-prometheus-config` as usual.
* Deploy the HELM chart a second time with a different chart release name, advanced mode not enabled, and all default scrape targets disabled. The default scrape targets need to be disabled or else node-exporter and kube-state-metrics will be installed again and all the default metrics will be sent from both instances. Additional custom scrape targets should be in the configmap `<chart release name 2>-prometheus-config`.
* For example, if the first helm chart install command was:

  ```bash
  helm upgrade --install <chart_release_name_1> <chart>.tgz --version <chart_semver> --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.clientId="**" --set azureKeyVault.clientSecret="****" --set mode.advanced=true --set windowsDaemonset=true --namespace=<my_prom_collector_namespace> --create-namespace
  ```

  Then the second should be similar to:

  ```bash
  helm upgrade --install <chart_release_name_2> <chart.tgz> --version <chart_semver> --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.clientId="**" --set azureKeyVault.clientSecret="****" --set scrapeTargets.coreDns=false --set scrapeTargets.kubelet=false --set scrapeTargets.cAdvisor=false -- set scrapeTargets.kubeProxy=false --set scrapeTargets.apiServer=false --set scrapeTargets.kubeState=false --set scrapeTargets.nodeExporter=false --namespace=<my_prom_collector_namespace> --create-namespace
  ```

## Setting Custom CPU and Memory Limits (Helm Chart)

The CPU and memory needed are correlated with the number of bytes each timeseries sent is and how many timeseries there are.

### Agent in Non-Advanced mode

* Some reference measurements recorded using just the default scrape targets and non-advanced mode are:

  | Timeseries Sent / Minute | GB Sent / Minute | Node Count | Pod Count | Prometheus-Collector CPU Usage (cores) |Prometheus-Collector Memory Usage (GB)
  | --- | --- | --- | --- | --- | --- |
  | 2.58 million | 3.1 | 240 | 1500 | 3.45 | 8.45 |
  | 2.84 million | 3.5 | 240 | 2000 | 3.51 | 9.39 |
  | 3.03 million | 3.7 | 265 | 2000 | 4.07 | 10.76 |

  The number of `Timeseries Sent / Minute` and `GB Sent / Minute` can be compared with your volume to set the CPU and Memory limits for your prometheus-collector deployments.

### Windows Daemonset (per agent pod)

* For each Windows daemonset pod, the limit of the number of timeseries and bytes is the same as above. The volume sent from a single daemonset pod can be viewed in the `Prometheus-Collector Health` dashboard by filtering by a specific Windows node.
* Below is the performance of one daemonset pod that is scraping only the default scrape targets for one node, based on the pod and container count for that node:

  | Timeseries Sent / Minute | Bytes Sent / Minute | Pod Count | Container Count | Prometheus-Collector CPU Usage |Prometheus-Collector Memory Usage
  | --- | --- | --- | --- | --- | --- |
  | 33,200 | 11.9 MB | 8 | 15 | 70 mc | 238 MB |
  | 46,100 | 16.5 MB | 14 | 27 | 81 mc | 367 MB |
* Below is the performance of one daemonset pod for a high volume of custom metrics from a single Windows node:

  | Timeseries Sent / Minute | Bytes Sent / Minute | Prometheus-Collector CPU Usage |Prometheus-Collector Memory Usage
  | --- | --- | --- | --- |
  | 1.05 million | 409 MB | 0.69 cores | 1.60 GB |
  | 1.65 million | 641 MB | 1.23 cores | 2.41 GB |
  | 2.0 million | 803 MB | 1.65 cores | 2.92 GB |

### Adjusting the Requests and Limits

* The requests and limits of the pods can be adjusted by setting values in the HELM chart for the replicaset and Linux/Windows daemonsets:

  ```yaml
  resources:
    deployment:
      limits:
        cpu: 4
        memory: 7Gi
      requests:
        cpu: 1
        memory: 2Gi
    daemonSet:
      limits:
        cpu: 1
        memory: 2Gi
      requests:
        cpu: 500m
        memory: 1Gi
    daemonSetWindows:
     limits:
       cpu: 500m
       memory: 500Mi
  ```

  These can be adjusted by specifying these chart values such as `--set resources.deployment.limits.cpu=5` and `--set resources.deployment.limits.memory=11GB` in the HELM upgrade/install command.

## CPU and Memory Limits (Addon)

The CPU and memory usage is correlated with the number of bytes of each sample and the number of samples scraped. Below are benchmarks based on the default targets scraped, volume of custom metrics scraped, and number of nodes, pods, and containers. These numbers are meant as a reference rather than a guarantee, since usage can still vary greatly depending on the number of timeseries and bytes per metric.

Note that a very large volume of metrics will require a large enough node to be able to handle the CPU and memory usage required. Below are guidelines on the expected usage.

Currently the upper volume limit is around 3-3.5 million samples/min, depending on the number of bytes per sample. This limitation will go away in the future with sharding.

### Replicaset in Small vs Large Cluster

  Scrape Targets | Samples Sent / Minute | Node Count | Pod Count | Prometheus-Collector CPU Usage (cores) |Prometheus-Collector Memory Usage (bytes)
  | --- | --- | --- | --- | --- | --- |
  | default targets | 11,344 | 3 | 40 | 12.9 mc | 148 Mi |
  | default targets | 260,000  | 340 | 13000 | 1.10 c | 1.70 GB |
  | default targets + custom targets | 3.56 million | 340 | 13000 | 5.13 c | 9.52 GB |

### Daemonset in Small Cluster vs Large Cluster

  Scrape Targets | Samples Sent / Minute Total | Samples Sent / Minute / Pod |  Node Count | Pod Count | Prometheus-Collector CPU Usage Total (cores) |Prometheus-Collector Memory Usage Total (bytes) | Prometheus-Collector CPU Usage / Pod (cores) |Prometheus-Collector Memory Usage / Pod (bytes)
  | --- | --- | --- | --- | -- | --- | --- | --- | --- |
  | default targets | 9,858 | 3,327 | 3 | 40 | 41.9 mc | 581 Mi | 14.7 mc | 189 Mi |
  | default targets | 2.3 million | 14,400 | 340 | 13000 | 805 mc | 305.34 GB | 2.36 mc | 898 Mi |

  For additional custom metrics, the single pod will behave the same as the replicaset pod depending on the volume of custom metrics.
