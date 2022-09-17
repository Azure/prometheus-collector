# Scraping Metrics at High Scale

## Advanced Mode: Scraping custom targets with the Daemonset pods

When you follow the instructions [here](https://github.com/Azure/prometheus-collector/blob/rashmi/pub-preview-docs/otelcollector/docs/publicpreviewdocs/ama-metrics-prometheus-config-readme.md) to scrape custom targets, the scraping is done by the ama-metrics replicaset pod.

* Custom scrape targets can be off-loaded to the daemonset. A configmap similar to the regular configmap can be created to have static scrape configs on each node. This configmap should have the name `<helm release name>-prometheus-config-node` for scrape targets for Linux and `<helm release name>-prometheus-config-node-windows` for scrape targets for Windows. Note that the scrape config should only target a single node; otherwise each node will try to scrape all targets. The node-exporter config is a good example of using the `$NODE_IP` environment variable (already set for every prometheus-collector pod) to target a specific endpoint on the node:

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