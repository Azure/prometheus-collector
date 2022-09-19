# Prometheus Configuration Tips

## Configuration File
The configuration format is the same as the [Prometheus configuration file](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Currently supported are the following sections:
```yaml
global:
  scrape_interval: <duration>
  scrape_timeout: <duration>
  external_labels:
    <labelname1>: <labelvalue>
    <labelname2>: <labelvalue>
scrape_configs:
  - <scrape_config1>
  - <scrape_config2>
```

Note that any other unsupported sections need to be removed from the config before applying as a configmap; otherwise the custom configuration will fail validation and will not be applied.

## Scrape Configs
The currently supported methods of target discovery for a [scrape config](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) are either [`static_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#static_config) or [`kubernetes_sd_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) for specifing or discovering targets.

### Static Config

A static config has a list of static targets and any additional labels to add to them.

```yaml
scrape_configs:
  - job_name: example
    - targets: [ '10.10.10.1:9090', '10.10.10.2:9090', '10.10.10.3:9090' ... ]
    - labels: [ label1: value1, label1: value2, ... ]
```

### Kubernetes Service Discovery Config

Targets discovered using [`kubernetes_sd_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#kubernetes_sd_config) will each have different `__meta_*` labels depending on what role is specified. These can be used in the `relabel_configs` section to filter targets or replace labels for the targets.

See the [Prometheus examples](https://github.com/prometheus/prometheus/blob/main/documentation/examples/prometheus-kubernetes.yml) of scrape configs for a Kubernetes cluster.

## Relabel Configs
The `relabel_configs` section is applied at the time of target discovery and applies to each target for the job. Below are examples showing ways to use `relabel_configs`.

### Adding a Label
Add a new label called `example_label` with value `example_value` to every metric of the job. Use `__address__` as the source label only because that label will always exist. This will add the label for every target of the job.

```yaml
relabel_configs:
- source_labels: [__address__]
  target_label: example_label
  replacement: 'example_value'
```

### Use Kubernetes Service Discovery Labels

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

### Job and Instance Relabeling
The `job` and `instance` label values can be changed based on the source label, just like any other label.

```yaml
# Replace the job name with the pod label 'k8s app'
relabel_configs:
- source_labels: [__meta_kubernetes_pod_label_k8s_app]
  target_label: job

# Replace the instance name with the node name. This is helpful to replace a node IP
# and port with a value that is more readable
relabel_configs:
- source_labels: [__meta_kubernetes_node_name]]
  target_label: instance
```

## Metric Relabel Configs
Metric relabel configs are applied after scraping and before ingestion. Use the `metric_relabel_configs` section to filter metrics after scraping. Below are examples of how to do so.

### Drop Metrics by Name

```yaml
# Drop the metric named 'example_metric_name'
metric_relabel_configs:
- source_labels: [__name__]
  action: drop
  regex: 'example_metric_name'
```

### Keep Only Certain Metrics by Name

```yaml
# Keep only the metric named 'example_metric_name'
metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: 'example_metric_name'
```

```yaml
# Keep only metrics that start with 'example_'
metric_relabel_configs:
- source_labels: [__name__]
  action: keep
  regex: '(example_.*)'
```

### Rename Metrics
*Note that metric renaming is not supported.*

### Filter Metrics by Labels

```yaml
# Keep only metrics with where example_label = 'example'
metric_relabel_configs:
- source_labels: [example_label]
  action: keep
  regex: 'example'
```

```yaml
# Keep metrics only if `example_label` equals `value_1` or `value_2`
metric_relabel_configs:
- source_labels: [example_label]
  action: keep
  regex: '(value_1|value_2)'
```

```yaml
# Keep metric only if `example_label_1 = value_1` and `example_label_2 = value_2`
metric_relabel_configs:
- source_labels: [example_label_1, example_label_2]
  separator: ';'
  action: keep
  regex: 'value_1;value_2'
```

```yaml
# Keep metric only if `example_label` exists as a label
metric_relabel_configs:
- source_labels: [example_label_1]
  action: keep
  regex: '.+'
```

## Pod Annotation Based Scraping

If you are currently using Azure Monitor Container Insights Prometheus scraping with the setting `monitor_kubernetes_pods = true`, adding this job to your custom config will allow you to scrape the same pods and metrics.

The scrape config below uses the `__meta_*` labels added from the `kubernetes_sd_configs` for the `pod` role to filter for pods with certain annotations.

To scrape certain pods, specify the port, path, and scheme through annotations for the pod and the below job will scrape only the address specified by the annotation:
- `prometheus.io/scrape`: Enable scraping for this pod
- `prometheus.io/scheme`: If the metrics endpoint is secured then you will need to set this to `https` & most likely set the tls config.
- `prometheus.io/path`: If the metrics path is not /metrics, define it with this annotation.
- `prometheus.io/port`: Specify a single, desired port to scrape

```yaml
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
      replacement: $1:$2
      target_label: __address__
        
    # If prometheus.io/scheme is specified, scrape with this scheme instead of http
    - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scheme]
      action: replace
      regex: (http|https)
      target_label: __scheme__

    # Include the pod namespace as a label for each metric
    - source_labels: [__meta_kubernetes_namespace]
      action: replace
      target_label: kubernetes_namespace

    # Include the pod name as a label for each metric
    - source_labels: [__meta_kubernetes_pod_name]
      action: replace
      target_label: kubernetes_pod_name
    
    # [Optional] Include all pod labels as labels for each metric
    - action: labelmap
      regex: __meta_kubernetes_pod_label_(.+)
```

## Send Metrics to Multiple Azure Monitor Workspaces

Routing metrics to additional Azure Monitor Workspaces can be done through the creation of additional DCRs. All metrics can be sent to all workspaces or different metrics can be specified to be sent to different workspaces.

### Sending the Same Metrics to Multiple Workspaces

You can create multiple Data Collection Rules that point to the same Data Collection Endpoint for metrics to be sent to additional Azure Monitor Workspaces from the same Kubernetes cluster. Currently, this is only available through ARM template deployments [link to Kaveesh's doc]. In your ARM template, add additional DCRs for your additional Azure Monitor Workspaces:

```json
{
  "type": "Microsoft.Insights/dataCollectionRules",
  "apiVersion": "2021-09-01-preview",
  "name": "[variables('dcrName')]",
  "location": "[parameters('azureMonitorWorkspaceLocation')]",
  "kind": "Linux",
  "properties": {
    "dataCollectionEndpointId": "[resourceId('Microsoft.Insights/dataCollectionEndpoints/', variables('dceName'))]",
    "dataFlows": [
      {
        "destinations": ["MonitoringAccount1"],
        "streams": ["Microsoft-PrometheusMetrics"]
      }
    ],
    "dataSources": {
      "prometheusForwarder": [
        {
          "name": "PrometheusDataSource",
          "streams": ["Microsoft-PrometheusMetrics"],
          "labelIncludeFilter": {}
        }
      ]
    },
    "description": "DCR for Azure Monitor Metrics Profile (Managed Prometheus)",
    "destinations": {
      "monitoringAccounts": [
        {
          "accountResourceId": "[parameters('azureMonitorWorkspaceResourceId')]",
          "name": "MonitoringAccount1"
        }
      ]
    }
  },
  "dependsOn": [
    "[resourceId('Microsoft.Insights/dataCollectionEndpoints/', variables('dceName'))]"
  ]
},
{
  "type": "Microsoft.Insights/dataCollectionRules",
  "apiVersion": "2021-09-01-preview",
  "name": "[variables('dcrName')]",
  "location": "[parameters('azureMonitorWorkspaceLocation')]",
  "kind": "Linux",
  "properties": {
    "dataCollectionEndpointId": "[resourceId('Microsoft.Insights/dataCollectionEndpoints/', variables('dceName'))]",
    "dataFlows": [
      {
        "destinations": ["MonitoringAccount1"],
        "streams": ["Microsoft-PrometheusMetrics"]
      }
    ],
    "dataSources": {
      "prometheusForwarder": [
        {
          "name": "PrometheusDataSource",
          "streams": ["Microsoft-PrometheusMetrics"],
          "labelIncludeFilter": {}
        }
      ]
    },
    "description": "DCR for Azure Monitor Metrics Profile (Managed Prometheus)",
    "destinations": {
      "monitoringAccounts": [
        {
          "accountResourceId": "[parameters('azureMonitorWorkspaceResourceId')]",
          "name": "MonitoringAccount1"
        }
      ]
    }
  },
  "dependsOn": [
    "[resourceId('Microsoft.Insights/dataCollectionEndpoints/', variables('dceName'))]"
  ]
}
```

### Sending Different Metrics to Different Azure Monitor Workspaces

If you want to send some metrics to one Azure Monitor Workspace and other metrics to a different one, follow the above steps to add additional DCRs. The value of `microsoft_metrics_include_label` under the `labelIncludeFilter` in the DCR is the identifier for the workspace. To then configure which metrics are routed to which workspace, you can add an extra pre-defined label, `microsoft_metrics_account` to the metrics. The value should be the same as the corresponding `microsoft_metrics_include_label` in the DCR for that workspace. To add the label to the metrics, you can utilize `relabel_configs` in your scrape config. To send all metrics from one job to a certain workspace, add the following relabel config:

```yaml
relabel_configs:
- source_labels: [__address__]
  target_label: microsoft_metrics_account
  action: replace
  replacement: "MonitoringAccountLabel1"
```

For example, for sending 3 different jobs to 3 different workspaces, in each DCR include:
```json
"labelIncludeFilter": {
  "microsoft_metrics_include_label": "MonitoringAccountLabel1"
}
```

```json
"labelIncludeFilter": {
  "microsoft_metrics_include_label": "MonitoringAccountLabel2"
}
```

```json
"labelIncludeFilter": {
  "microsoft_metrics_include_label": "MonitoringAccountLabel3"
}
```

Then in your scrape config, include the same label value for each:
```yaml
scrape_configs:
- job_name: prometheus_ref_app_1
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: "prometheus-reference-app-1"
    - source_labels: [__address__]
      target_label: microsoft_metrics_account
      action: replace
      replacement: "MonitoringAccountLabel1"
- job_name: prometheus_ref_app_2
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: "prometheus-reference-app-2"
    - source_labels: [__address__]
      target_label: microsoft_metrics_account
      action: replace
      replacement: "MonitoringAccountLabel2"
- job_name: prometheus_ref_app_3
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      action: keep
      regex: "prometheus-reference-app-3"
    - source_labels: [__address__]
      target_label: microsoft_metrics_account
      action: replace
      replacement: "MonitoringAccountLabel3"
```
