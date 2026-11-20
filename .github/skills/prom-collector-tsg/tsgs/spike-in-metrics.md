# TSG: Spike in Metrics Ingested

Run `tsg_workload`, `tsg_config`, `tsg_mdm_throttling`, and `tsg_metric_insights`. Then:

1. **Check MDM throttling first** — extract `MDMAccountName` from `tsg_triage`, then run `tsg_mdm_throttling`. If event volume or time series utilization is > 80%, the spike may be causing throttling and metric loss
2. **Identify top offending metrics** — run `tsg_metric_insights` with the same `MDMAccountName`. Check "Top 20 Highest Cardinality Metrics" and "Metrics with High Dimension Cardinality" to find which metrics/jobs are causing the spike
3. **Check scrape samples per job over time** — run `tsg_workload`, check "Scrape Samples Per Job Over Time". If a specific job's `max_samples` changed, that job is the source. Common causes: AKS upgrade shipped a new node exporter version (check "AKS Upgrade History" in triage), new PodMonitor/ServiceMonitor added, pod annotation scraping enabled for a high-churn namespace
4. **Check ME throughput by pod type** — run `tsg_workload`, check "ME Throughput by Pod Type Over Time". If throughput is flat but TS is growing, the spike is from **label churn** (new pod/container names creating new TS in MDM) or the "floodgate effect" after a quota increase (see Missing Metrics TSG step 16)
5. Customer can run PromQL: `sum_over_time(scrape_samples_post_metric_relabeling) by (job)` to see if new jobs were added or existing jobs increased
6. Most common cause: **Network Observability** metrics increase with cluster traffic
7. **Reduction options:**
   - Default metrics: use `ama-metrics-settings-configmap` to change targets, metrics, scrape frequency
   - Custom metrics: use `relabel_configs`/`metric_relabel_configs` to filter, increase `scrape_interval`
   - Reducing labels reduces time series count; reducing scrape interval reduces sample count
8. **Customer documentation:**
   - [Customize scraping of Prometheus metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration)
   - [Default Prometheus metrics configuration](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default)
   - [Azure Monitor workspace scaling best practices](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/azure-monitor-workspace-scaling-best-practice)
   - [Azure Monitor service limits](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/service-limits)
