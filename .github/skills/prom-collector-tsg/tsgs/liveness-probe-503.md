# TSG: Liveness Probe Failures (503)

Run `tsg_errors`, check "Liveness Probe Logs". Then:

1. **HTTP 503 from liveness probe** — this means ME (MetricsExtension) is not ready. Common causes:
   - TokenConfig.json not yet downloaded from AMCS (slow AMCS response, especially on cold start)
   - DCR/DCE misconfiguration preventing config download
   - Network policy blocking egress to AMCS endpoints
2. **Check auth errors** — run `tsg_errors`, look for `DCR/DCE/AMCS Configuration Errors`. If "Configuration not found", the DCR may be deleted or DCE endpoint is wrong
3. **Transient vs persistent** — if liveness probes fail only during pod startup (first 30-60s) then succeed, this is normal cold-start behavior. If persistent, there's a config or network issue
4. **Gov cloud / sovereign** — gov cloud clusters (`*.cx.aks.containerservice.azure.us`) have different AMCS endpoints. Verify the DCE region matches the cluster region
5. **Customer documentation:**
   - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
   - [Network firewall requirements for monitoring Kubernetes](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-firewall)
