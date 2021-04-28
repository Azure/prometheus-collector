# Prometheus Configuration
## Configuration File
The format specified in the configmap will be the same as a prometheus.yml following the [configuration format](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Currently supported are the following sections:
```yaml
global:
  scrape_interval: <duration>
  scrape_timeout: <duration>
  external_labels:
    [ <labelname>: <labelvalue> ... ]
scrape_configs:
  - <scrape_config>
  ...
```

Note when using the `replacement` field for `relabel_configs` or `metric_relabel_configs`, the `$` operator for a regex match group reference needs to be replaced with `$$` so that environment variable replacement does not happen. This is different from a configuration directly for Prometheus.

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

### Multiple metric accounts

To route metric(s) to different account(s), use the target re-labeling config to provide a pre-defined label ```microsoft_metrics_account```

This example has two scrape jobs that is configured to woute metrics from each of these job to a different metrics account. 
**Note** If no account name is specified using the pre-defined label for any scraped metric, those metrics will be routed to the 'default' metric account specified in the config map. To over-ride the default account, you can use the re-labeling per job or even for specific metric(s), in the scrape config.

```yaml
global:
  evaluation_interval: 60s
  scrape_interval: 60s
scrape_configs:
- job_name: prometheus_ref_app
  scheme: http
  kubernetes_sd_configs:
    - role: service
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      action: keep
      regex: "prometheus-reference-service"
    - source_labels: [__address__]
      target_label: microsoft_metrics_account
      action: replace
      replacement: "containerinsightsgenevaaccount"
- job_name: "kubernetes-kubelet"
  scheme: https
  tls_config:
    ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    insecure_skip_verify: true
  bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
  kubernetes_sd_configs:
    - role: node
  relabel_configs:
  - action: labelmap
    regex: __meta_kubernetes_node_label_(.+)
  - target_label: __address__
    replacement: kubernetes.default.svc:443
  - target_label: __scheme__
    replacement: https
  - source_labels: [__meta_kubernetes_node_name]
    regex: (.+)
    target_label: __metrics_path__
    replacement: /api/v1/nodes/$${1}/proxy/metrics
  - source_labels: [__address__]
    target_label: microsoft_metrics_account
    action: replace
    replacement: "containerinsightsgenevaaccount2"
```

#### More Examples

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
  global:
    scrape_interval: 5s
    evaluation_interval: 5s
    scrape_configs:
      - job_name: 'kubernetes-pods'

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
          replacement: $$1:$$2
          target_label: __address__
        
        # If prometheus.io/port is specified, scrape this port instead of the default
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
