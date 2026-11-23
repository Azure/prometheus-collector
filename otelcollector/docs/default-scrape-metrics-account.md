# Route built-in metrics with a metrics account label

Set `default-scrape-metrics-account` in `ama-metrics-settings-configmap` to supply
a fallback `microsoft_metrics_account` for samples from enabled built-in scrape jobs.
The setting is an optional string. Empty or omitted disables it; there is no
separate enable switch.

The key is directly under `data` in **both v1 and v2**, including when cluster and
control-plane settings use separate v2 sections:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  schema-version: v2
  default-scrape-metrics-account: "service"
```

Add the key to your existing settings ConfigMap to retain your other settings.
The shipped examples use `default-scrape-metrics-account: ""`.

## Behavior

- Applies to every enabled built-in cluster and control-plane scrape job,
  including Linux and Windows jobs, annotation-based scraping, and jobs generated
  by the target allocator's configuration reader.
- Appends a metric relabel rule that fills a missing or empty account label.
  Samples with an existing nonempty account retain it. Prometheus-generated
  series such as `up` bypass metric relabeling and receive no fallback.
- Preserves existing discovery, authentication, intervals, and metric filtering.
- Applies before custom scrape jobs are merged. Custom scrape configurations,
  ServiceMonitors, and PodMonitors retain their own labeling.
- Accepts only ASCII letters, digits, underscores, periods, colons, and hyphens.
  An invalid value logs a warning and disables the generated assignment.
- Removing or clearing the setting restores normal built-in labeling when
  configuration is regenerated. It does not enable disabled scrape jobs.

Existing receiver external-label behavior still applies: avoid a conflicting
`global.external_labels.microsoft_metrics_account` in custom configuration.

## Match the value in a DCR

The metric-side label is `microsoft_metrics_account`. The corresponding DCR
filter key has a different name, `microsoft_metrics_include_label`:

```json
"prometheusForwarder": [
  {
    "name": "PrometheusDataSource",
    "streams": ["Microsoft-PrometheusMetrics"],
    "labelIncludeFilter": {
      "microsoft_metrics_include_label": "service"
    }
  }
]
```

Configure and associate the DCR separately, using a value accepted by the DCR
filter. This setting adds the label; it does not create a DCR or select a
workspace by resource ID. The label/filter pairing follows the
[Azure Monitor multi-workspace routing guide](https://learn.microsoft.com/azure/azure-monitor/containers/prometheus-metrics-multiple-workspaces).
