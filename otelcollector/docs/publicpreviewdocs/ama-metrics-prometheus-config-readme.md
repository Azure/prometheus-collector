# Configure custom scrape jobs

In order to configure the azure monitor metrics addon to scrape targets other than the default targets, create this [configmap](https://github.com/Azure/prometheus-collector/blob/rashmi/pub-preview-docs/otelcollector/deploy/ama-metric-settings-prometheus-config.yaml) and update the prometheus-config section with your custom prometheus configuration. 
The format specified in the configmap will be the same as a prometheus.yml following the [configuration format](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Currently supported are the following sections:
```yaml
global:
  scrape_interval: <duration>
  scrape_timeout: <duration>
scrape_configs:
  - <scrape_config>
  ...
```
Before applying the configuration as a configmap, it is recommended that you validate it using the 'promconfigvalidator' tool, which is the same tool that is run at the container startup to perform validation of custom configuration. If the config is not valid, then the custom configuration given will not be used by the agent.
Please refer to these instructions on how to run the tool. 

RashmiTBD: Add link to promconfigvalidator instructions here. 


Note that any other unsupported sections need to be removed from the config before applying as a configmap, else the promconfigvalidator tool validation will fail and as a result the custom scrape configuration will not be applied

The `scrape_config` setting `honor_labels` (`false` by default) should be `true` for scrape configs where labels that are normally added by Prometheus, such as `job` and `instance`, are already labels of the scraped metrics and should not be overridden. This is only applicable for cases like [federation](https://prometheus.io/docs/prometheus/latest/federation/) or scraping the [Pushgateway](https://github.com/prometheus/pushgateway), where the scraped metrics already have `job` and `instance` labels. See the [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) for more details.

## Targets
For a Kubernetes cluster, a scrape config can either use `static_configs` or [`kubernetes_sd_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) for specifing or discovering targets.

### Static Config
```yaml
scrape_configs:
  - job_name: example
    - targets: [ '10.10.10.1:9090', '10.10.10.2:9090', '10.10.10.3:9090' ... ]
    - labels: [ label1: value1, label1: value2, ... ]
```

### Kubernetes Service Discovery Config

Targets discovered using [`kubernetes_sd_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) will each have different `__meta_*` labels depending on what role is specified. These can be used in the `relabel_configs` section to filter targets or replace labels for the targets.

See the [Prometheus examples](https://github.com/prometheus/prometheus/blob/main/documentation/examples/prometheus-kubernetes.yml) of scrape configs for a Kubernetes cluster.

### More Examples

Add a new label called `example_label` with value `example_value` to every metric of the job. Use `__address__` as the source label only because that label will always exist.

This example can be used to add a `cluster_id` label to metrics when multiple clusters are sending metrics to the same account.
```yaml
relabel_configs:
- source_labels: [__address__]
  target_label: example_label
  replacement: 'example_value'
```

## Metric Filtering
Metrics are filtered after scraping and before ingestion. Use the `metric_relabel_configs` section for a scrape_config to rename or filter metrics.

### Drop Metrics by Name
Drop the metric named `example_metric_name`
```yaml
metric_relabel_configs:
- source_labels: [__name__]
  action: drop
  regex: 'example_metric_name'
```
### Keep Only Certain Metrics by Name
Keep only the metric named `example_metric_name`
```yaml
metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: 'example_metric_name'
```
Keep only metrics that start with `example_`
```yaml
metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: '(example_.*)'
```
### Rename Metrics
Rename the metric `example_metric_name` to `new_metric_name`
```yaml
metric_relabel_configs:
- source_labels: [__name__]
  action: replace
  regex: 'example_metric_name'
  target_label: __name__
  replacement: 'new_metric_name'
```
### Filter Metrics by Labels
Keep only metrics with where example_label = 'example'
```yaml
metric_relabel_configs:
- source_labels: [example_label]
  action: keep
  regex: 'example'
```
Keep metric only if `example_label` equals `value_1` or `value_2`
```yaml
metric_relabel_configs:
- source_labels: [example_label]
  action: keep
  regex: '(value_1|value_2)'
```
Keep metric only if `example_label_1 = value_1` and `example_label_2 = value_2`
```yaml
metric_relabel_configs:
- source_labels: [example_label_1, example_label_2]
  separator: ';'
  action: keep
  regex: 'value_1;value_2'
```
Keep metric only if `example_label` exists
```yaml
metric_relabel_configs:
- source_labels: [example_label_1]
  action: keep
  regex: '.+'
```
If a job is using [`kubernetes_sd_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) to discover targets, each role has associated `__meta_*` labels for metrics. The `__*` labels are dropped after discovering the targets. To filter by them at the metrics level, first keep them using `relabel_configs` by assigning a label name and then use `metric_relabel_configs` to filter.
```yaml
# Use the kubernetes namespace as a label called 'kubernetes_namespace'
relabel_configs:
- source_labels: [__meta_kubernetes_namespace]
  action: replace
  target_label: kubernetes_namespace
# Keep only metrics with the kubernetes namespace 'default'
metric_relabel_configs:
- source_labels: [kubernetes_namespace]
  action: keep
  regex: 'default'
```

## Advanced Configuration

### Pod Scraping and Pod Annotation Based Scraping
To scrape all pods, include only the last three relabel configs below.

To scrape only certain pods, specify the port, path, and http/https through annotations for the pod and the below job will scrape only the address specified by the annotation:
- `prometheus.io/scrape`: Enable scraping for this pod
- `prometheus.io/scheme`: If the metrics endpoint is secured then you will need to set this to `https` & most likely set the tls config.
- `prometheus.io/path`: If the metrics path is not /metrics, define it with this annotation.
- `prometheus.io/port`: Specify a single, desired port to scrape

  ```yaml
    scrape_configs:
      - job_name: 'kubernetes-pods'
        label_limit: 63
        label_name_length_limit: 511
        label_value_length_limit: 1023

        kubernetes_sd_configs:
        - role: pod

        relabel_configs:
        # Scrape only pods with the annotation: prometheus.io/scrape = true
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
          action: keep
          regex: true

        # If prometheus.io/path is specified, scrape this path instead of /metrics
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
          action: replace
          target_label: __metrics_path__
          regex: (.+)

        # If prometheus.io/port is specified, scrape this port instead of the default
        - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
          action: replace
          regex: ([^:]+)(?::\d+)?;(\d+)
          replacement: $1:$2
          target_label: __address__
        
        # If prometheus.io/scheme is specified, scrape with this scheme instead of http
        - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scheme]
          action: replace
          regex: (http|https)
          target_label: __scheme__

        # Include all pod labels as labels for the metric
        - action: labelmap
          regex: __meta_kubernetes_pod_label_(.+)

        # Include the pod namespace a label for the metric
        - source_labels: [__meta_kubernetes_namespace]
          action: replace
          target_label: kubernetes_namespace

        # Include the pod name as a label for the metric
        - source_labels: [__meta_kubernetes_pod_name]
          action: replace
          target_label: kubernetes_pod_name
  ```

### Reducing Cost
See the [Grafana documentation](https://grafana.com/docs/grafana-cloud/billing-and-usage/prometheus/usage-reduction/) for detailed instructions on using `relabel_configs` and `metric_relabel_configs` for reducing the number of targets scraped and the amount of metrics ingested.
