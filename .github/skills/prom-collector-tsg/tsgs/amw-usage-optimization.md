# TSG: AMW Usage Optimization

When customer asks about reducing Azure Monitor Workspace costs:

1. **Identify top volume drivers** — run `tsg_metric_insights` to see which metrics have the most time series and highest sample rates
2. **Reduce default metrics** — use `ama-metrics-settings-configmap` to disable unused default targets (e.g., `kube-proxy`, `core-dns` if not needed)
3. **Set keep lists** — configure `default-targets-metrics-keep-list` to only ingest needed metrics from each target
4. **Increase scrape intervals** — change from default 15s/30s to 60s for non-critical targets via `default-scrape-settings-enabled`
5. **Reduce cardinality** — use `metric_relabel_configs` to drop high-cardinality labels. Check `tsg_metric_insights` → "Metrics with High Dimension Cardinality"
6. **Enable minimal ingestion profile** — set `minimal-ingestion-profile: true` in settings configmap to only ingest a curated set of metrics
7. **Customer documentation:**
   - [Azure Monitor workspace overview](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/azure-monitor-workspace-overview)
   - [Azure Monitor workspace scaling best practices](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/azure-monitor-workspace-scaling-best-practice)
   - [Azure Monitor service limits](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/service-limits)
   - [Customize scraping of Prometheus metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration)
   - [Default Prometheus metrics configuration](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default)
   - [Azure Monitor Prometheus overview](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/prometheus-metrics-overview)
