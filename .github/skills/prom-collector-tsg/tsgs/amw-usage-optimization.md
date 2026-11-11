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

## Compare observed queries with ingestion (internal telemetry)

Use `tsg_query` with datasource `MetricInsights` (`metricsinsightsUX`). Replace
`<MDM_ACCOUNT>` with the monitoring account, not the workspace ARM ID. Replace
the UTC day placeholders below with explicit dates; export full results with
`outputFile`, rather than comparing truncated previews.

These query forms were executed successfully. `StartTime`/`EndTime` were daily
buckets: the inclusive `between` range selects day starts from `<FIRST_DAY_UTC>`
through `<LAST_DAY_UTC>`, covering through midnight after the last day.

```kusto
QueryInsightsUsage
| where monitoringAccount == '<MDM_ACCOUNT>' and metricNamespace == 'customdefault'
    and metricName startswith 'kube_'
    and StartTime between (datetime(<FIRST_DAY_UTC>) .. datetime(<LAST_DAY_UTC>))
| summarize records=count() by metricName
| order by records desc
```

```kusto
QueryInsightsCount
| where monitoringAccount == '<MDM_ACCOUNT>' and metricNamespace == 'customdefault'
    and metricName startswith 'kube_'
    and StartTime between (datetime(<FIRST_DAY_UTC>) .. datetime(<LAST_DAY_UTC>))
| summarize total=sum(TotalQueries), rows=count() by metricName
| order by total desc
```

For ingestion on one complete UTC day:

```kusto
StorageInsightsUsageV2
| where monitoringAccount == '<MDM_ACCOUNT>' and metricNamespace == 'customdefault'
    and metricName startswith 'kube_' and StartTime == datetime(<DAY_UTC>)
| summarize daily=max(DailyTimeSeriesCount) by metricName
| order by daily desc
```

```kusto
EventStatsInsightsV2
| where monitoringAccount == '<MDM_ACCOUNT>' and metricNamespace == 'customdefault'
    and metricName startswith 'kube_' and CollectionStartDate == datetime(<DAY_UTC>)
| project metricName, AvgEventRate, P90EventRate, totalEventCount
| order by totalEventCount desc
```

Compare metric-name sets, preserving these limits:

- `records` counts usage-table rows, not query frequency. `total` sums
  `TotalQueries` per metric across rows; multiple metrics can share a query,
  so summing across metrics does not count distinct queries.
- `daily` is the maximum observed daily series count among dimension variants,
  not their sum. `totalEventCount` is the daily event count. Verify rate units
  independently; do not infer them from field names or assume P90 shares the
  average's units.
- These are account-wide observations, including observed query-service and
  recording-rule activity, not per-cluster usage. `kube_` is a name-prefix
  proxy, not verified attribution to a KSM job. Unobserved metric names are
  candidates for investigation, **not evidence that they are unused or safe
  to drop**. Confirm dashboards, alerts, recording rules, and other consumers
  before recommending a keep-list change.
