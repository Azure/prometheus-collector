# TSG: Pods Not Created / Addon Not Deploying

When `ama-metrics` pods don't exist at all:

1. **Check if monitoring addon is enabled** — run `tsg_config`, check "Addon Enabled in AKS Profile". If `metricsEnabled == false`, the addon isn't enabled. Customer needs to enable via `az aks update --enable-azure-monitor-metrics`
2. **Check cluster PUT failures** — if addon is enabled but pods don't exist, cluster PUT calls may be timing out. Transfer to `Azure Kubernetes Service/RP Triage` for cluster provisioning issues
3. **Customer documentation:**
   - [Enable monitoring for AKS clusters](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable)
   - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
4. **Check for DCRA (Data Collection Rule Association)** — the DCRA links the DCR to the cluster. If missing, metrics won't flow. Check via Azure Portal → AKS cluster → Monitoring → Data Collection Rules
4. **Check webhook/admission controller** — if the cluster has restrictive admission policies (OPA Gatekeeper, Kyverno), they may block ama-metrics pod creation. Check for denied admission events
