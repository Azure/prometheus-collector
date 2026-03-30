# TSG: Missing Metrics

Run `tsg_triage`, `tsg_config`, `tsg_workload`. Then:

1. **Check addon is enabled** — run `tsg_config`, check "Addon Enabled in AKS Profile". If `metricsEnabled == false`, the monitoring addon isn't enabled. Customer needs `az aks update --enable-azure-monitor-metrics`
2. **Check for OOMKill / pod crashes first** — missing metrics are often a SYMPTOM of pod OOMKills. If App Insights queries return "path does not exist", the addon is crash-looping and not sending telemetry at all. Go directly to `tsg_pods` for pod restart analysis and follow the Pod Restarts TSG below
3. **Check if the metric name exists in the account** — run `tsg_metric_insights` with the `MDMAccountName` from triage. The "View All Metric Names" panel returns every metric name ever ingested (180-day lookback). If the missing metric is NOT in this list, it was never successfully ingested — focus on scrape config, keep list regex, or config validation errors. If it IS in the list, the metric was ingested at some point — the issue may be throttling, intermittent scrape failures, or dimension fragmentation (see step 4)
4. **Check AMW quota and MDM throttling** — this is the most common cause of missing metrics at scale:
   - Run `tsg_triage` → extract `MDMAccountName` from "MDM Account ID" result
   - Run `tsg_mdm_throttling` with that `monitoringAccount` to check for throttling
   - If `ThrottledClientMetricCount > 0` → incoming events are being rejected by Geneva. Customer is hitting their account ingestion rate limit
   - If `ThrottledTimeSeriesCount > 0` → MStore is throttling time series. Customer has too many unique metric+label combinations
   - If `DroppedClientMetricCount > 0` → events are being dropped before ingestion
   - If `MStoreDroppedSamplesCount > 0` → samples are being lost in MStore
   - If event volume utilization is > 80% → approaching limit, will start throttling soon
   - If time series utilization is > 80% → approaching limit, need to reduce cardinality
   - **Resolution for throttling**: escalate to `Geneva Monitoring/MDM-Support-Manageability-Tier2` for quota increase, or help customer reduce cardinality via `metric_relabel_configs`
4. **Check ME ingestion success rate** — run `tsg_workload`, check "ME Ingestion Success Rate". If `successRate < 99%`, ME is dropping significant metrics. Cross-reference with ME queue sizes and drops
5. **Check auth issues** — look for `DCR/DCE/AMCS Configuration Errors`, `Liveness Probe Logs` with "No configuration present", `MDSD Errors`, `MetricsExtension Errors`. If `tsg_logs` shows repeated `TokenConfig.json does not exist`, this is the firewall/blocked endpoints pattern — see TSG: Firewall / Network Connectivity. If errors mention "private link is needed", also see that TSG
6. **Check if DCR/AMW association exists** — if `tsg_triage` → "Azure Monitor Workspace" returns empty AND `TokenConfig.json does not exist` is in logs, the DCRA may be missing entirely. Query the `AMWInfo` data source by subscription to verify: `AzureMonitorMetricsDCRDaily | where Timestamp > ago(30d) | where ParentResourceId contains "<SUBSCRIPTION_ID>" | distinct ParentResourceId, DCRId, AzureMonitorWorkspaceResourceId`. If the cluster is absent, the addon was enabled without a proper DCR/DCRA. Use the `ARMProd` data source to check ARM deployment history and confirm when/how the addon was enabled — see `reference.md` → "Querying ARM Deployment Logs". **Fix:** re-enable with an explicit AMW: `az aks update --enable-azure-monitor-metrics --azure-monitor-workspace-resource-id <AMW_ID>`
6. **Check CPU/memory** — if resources are very high, pods may be overwhelmed. Check if samples per minute per ReplicaSet exceed ~3.5 million
7. **Check ME queue/drops** — run `tsg_workload`, look at `ReplicaSet Samples Dropped` and queue sizes. If growing, need HPA or more shards
8. **Default metrics missing** — run `tsg_config`, check if default scrape config is enabled and metric is in `Default Targets KeepListRegex`. Customer may need to add metric to keep list
9. **Custom metrics missing** — run `tsg_config` and check these queries in order:
   - **"Invalid Custom Prometheus Config"** — if `true`, the customer's configmap has errors. Check **"Custom Config Validation Errors"** for the specific error (common: `not a valid duration string: "30"` — missing unit suffix; `found multiple scrape configs with job name` — duplicate job names; `unsupported features: rule_files` — rule_files not supported in configmap)
   - **"Custom Config YAML Error Lines"** — when the error is `yaml: unmarshal errors`, this query extracts every individual line number and field error (e.g. `line 136: field action not found in type kubernetes.plain`). This pinpoints exactly where the YAML is malformed — typically `metric_relabel_configs` fields (`action`, `regex`, `source_labels`, `target_label`, `replacement`) placed at the wrong indentation level inside `kubernetes_sd_configs`
   - **"Custom Config OTel Loading Errors"** — captures the full OTel collector configuration loading error chain (`Cannot load configuration: cannot unmarshal the configuration: decoding failed...`). This shows the higher-level error when the Prometheus config can't be loaded into the OTel receiver, complementing the line-level errors above
   - **"Custom Config Validation Status"** — shows per-pod whether config was loaded (`OK`), rejected (`INVALID`), or absent (`NO_CUSTOM_CONFIG`). If DaemonSet shows `NO_CUSTOM_CONFIG` but ReplicaSet shows `OK`, that's expected (DaemonSet uses separate `-node` configmap)
   - **"ReplicaSet ConfigMap Jobs"** and **"Custom Scrape Jobs from Startup Logs"** — verify the customer's job names appear. The startup logs query uses a wider time window since it only captures pod restarts. If empty, retry with `timeRange='30d'`
   - Check job also appears in PodMonitors or ServiceMonitors if using operator-based discovery
10. **Recording rule metrics missing** — run `tsg_config`, check "Recording Rules Configured" to confirm rules exist. Check scrape frequency vs recording rules evaluation interval (e.g. 1m rule interval with 2m scrape interval causes gaps). Transfer to `Azure Log Search Alerts/Prometheus Alerts` if needed
11. **Target distribution imbalance** — run `tsg_workload`, check "Target Allocator Distribution". If `targets_per_collector` is uneven or very high, some collectors may be overloaded. Check "Exporter Send Failures" for `send_failed_metric_points > 0` — indicates ME/MDM ingestion failures
12. **Check event timeline** — run `tsg_workload`, check "Event Timeline" to correlate config changes, restarts, and error spikes. Look for patterns like "config change → error spike → restart"
13. **Multi-AMW routing** — if the cluster has multiple AMWs associated (check `tsg_triage` → "AMW Configuration"):
    - All scrape jobs route to the **default AMW** unless explicitly configured otherwise via the `microsoft_metrics_account` label
    - **DCR-level routing vs label routing**: DCR associations route default scrape targets (kubelet, cadvisor, node-exporter, KSM) to the AMW linked in the DCR. The `microsoft_metrics_account` label on PodMonitor/ServiceMonitor relabeling routes those specific metrics to a named AMW
    - Cross-subscription ingestion IS supported without additional RBAC configuration — the DCRA handles auth automatically
    - If customer says metrics are missing, first ask **which AMW** they are querying and **which metrics** are missing (default targets vs pod monitor custom metrics)
    - **Correct PodMonitor relabeling** (per [MS docs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-send-to-multiple-azure-monitor-workspaces)):
      ```yaml
      relabelings:
        - action: replace
          replacement: <amw-name>  # must match an AMW associated with THIS cluster
          targetLabel: microsoft_metrics_account
      ```
    - **Common misconfiguration: wrong AMW name** — if `microsoft_metrics_account` points to an AMW not associated with the cluster (e.g. using a prod AMW name on a dev cluster), ME will try to publish to that endpoint and **fail** with HTTP 500 / connection timeouts. Metrics are silently dropped. Diagnose with:
      ```
      tsg_query(datasource: "PrometheusAppInsights", kql: "traces | where message has 'AggregatedMetricsPublisher' and message has 'publication failed' | project timestamp, message | order by timestamp desc | take 10", cluster: "...", timeRange: "24h", outputFile: "/tmp/me-routing-errors.json", outputFormat: "json")
      ```
      The ME error pattern is: `AggregatedMetricsPublisher _Default[wrong-account.region.metrics.ingest.monitor.azure.com]` — the bracketed endpoint reveals which AMW ME is trying to reach
    - **Cross-cluster AMW lookup** — to find which cluster an AMW is actually associated with:
      ```
      tsg_query(datasource: "AMWInfo", kql: "search '<amw-name>' | where $table == 'AzureMonitorMetricsDCRDaily' | project ParentResourceId | distinct ParentResourceId", outputFile: "/tmp/amw-cluster.json", outputFormat: "json")
      ```
      Use `outputFile` to avoid ARM ID truncation in tabular output
14. **Pod restarts causing gaps** — see Pod Restarts TSG above
15. **Check AKS upgrade / node image change** — run `tsg_triage`, check "AKS Upgrade History" to see if the cluster was recently upgraded (version changes with timestamps). An AKS version upgrade changes the node image, which includes a new `node_exporter` version. A new node exporter can expose different/additional metrics causing TS explosion, or break scraping entirely (up=0). Signs: `tsg_scrape_health` shows `node` job degraded/down, `tsg_workload` → "Node Exporter Sample Count Trend" shows `max_samples` changing (e.g. 2028→2065 = new version exposing 37 new metrics passing the keep-list). Also check "Scrape Samples Per Job Over Time" — if total scrape samples are flat but TS is growing, label churn (pod/container name turnover) is the cause. If `node` job has 0% success rate after upgrade, the new node exporter version broke scraping — escalate to AKS team
16. **Time series explosion after quota increase** — if MDM auto-quota increased and TS count is growing exponentially, this is the "floodgate effect": the quota was throttling events, masking the true TS count. When the throttle lifts, MStore discovers all previously-dropped label combinations as new time series. Check `tsg_workload` → "ME Throughput by Pod Type Over Time" — if ME throughput is flat but TS is growing, the growth is from MDM catching up, not from new data. The TS count will plateau once MStore has registered all unique label combos. Watch that it stays under the TS limit

**Ask customer to check Prometheus UI:**
- `kubectl port-forward <ama-metrics-pod> 9090` then check `/config` (scrape config present?), `/targets` (targets up?)
- If target is down: error message has details. If `node-exporter` down → transfer to AKS team. If `kube-state-metrics` down → check `ama-metrics-ksm` pod logs
15. **Customer documentation:**
    - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
    - [Customize scraping of Prometheus metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration)
    - [Default Prometheus metrics configuration](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default)
    - [Enable monitoring for AKS clusters](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable)
