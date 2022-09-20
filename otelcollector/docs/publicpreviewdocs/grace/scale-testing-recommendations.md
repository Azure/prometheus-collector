# High Scale and Metric Volume

## CPU and Memory

The CPU and memory usage is correlated with the number of bytes of each sample and the number of samples scraped. Below are benchmarks based on the default targets scraped, volume of custom metrics scraped, and number of nodes, pods, and containers. These numbers are meant as a reference rather than a guarantee, since usage can still vary greatly depending on the number of timeseries and bytes per metric.

Note that a very large volume of metrics per pod will require a large enough node to be able to handle the CPU and memory usage required. Below are guidelines on the expected usage. See [Rashmi's doc section] about how to specify a node that the replicaset should run on.

Currently the upper volume limit per pod is around 3-3.5 million samples/min, depending on the number of bytes per sample. This limitation will go away in the future when sharding is added.

The agent consists of a deployment with one replica and daemonset for scraping metrics. The daemonset scrapes any node-level targets such as cAdvisor, kubelet, and node exporter. You can also configure it to scrape any custom targets at the node level with static configs. The replicaset scrapes everything else such as kube-state-metrics or custom scrape jobs that utilize service discovery.

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
