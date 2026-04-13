# TSG: Windows Pod Restarts (ama-metrics-win)

1. Check pod logs for `TokenConfig.json not found`
2. If liveness probe shows `MetricsExtension not running (configuration exists)` — MA/MDSD was slow downloading TokenConfig.json from AMCS
3. **Resolution:** escalate to AMCS team with the DCR ID (get from `tsg_triage` → `Internal DCE and DCR Ids`)
4. **Customer documentation:**
   - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
   - [Network firewall requirements for monitoring Kubernetes](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-firewall)
