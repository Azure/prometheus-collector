# TSG: Duplicate Label Errors (kube-state-metrics)

When `kube-state-metrics` scraping produces duplicate label errors:

1. **Check `Kube-State-Metrics Labels Allow List`** — run `tsg_config`. If customer set `metricLabelsAllowlist` to `[*]` (all labels), Kubernetes labels may conflict with Prometheus labels (e.g., `pod`, `namespace` exist as both KSM metric labels and Kubernetes object labels)
2. **Check for double-scraping** — customer may have both default KSM scraping enabled AND a custom job scraping the same KSM endpoint with different label handling
3. **Resolution** — either narrow the `metricLabelsAllowlist` to specific needed labels, or use `metric_relabel_configs` to rename/drop conflicting labels
4. **Customer documentation:**
   - [Customize scraping of Prometheus metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration)
