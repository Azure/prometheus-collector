# Scraping Metrics at High Scale

## Advanced Mode: Scraping custom targets with the Daemonset pods

When you follow the instructions [here](https://github.com/Azure/prometheus-collector/blob/temp/documentation/otelcollector/docs/publicpreviewdocs/rashmi/ama-metrics-prometheus-config-readme.md) to scrape custom targets, the scraping is done by the ama-metrics replicaset pod.

* For a cluster with a large number of nodes and pods running on it, custom scrape targets can be off-loaded to the daemonset. A [configmap](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-prometheus-config-node-configmap.yaml) similar to the regular configmap can be created to have static scrape configs on each node. Note that the scrape config should only target a single node and not try to use service discovery; otherwise each node will try to scrape all targets. The node-exporter config is a good example of using the `$NODE_IP` environment variable (already set for every prometheus-collector container) to target a specific endpoint on the node:

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