# TSG: Control Plane Metrics

Run `tsg_control_plane`. Then:

1. Check AMW quota and OOM issues first
2. Check ASI page (requires VPN): `https://azureserviceinsights.trafficmanager.net/search/services/AKS?searchText={_cluster}` → Addons → Monitoring. If `ama-metrics-ccp` pod OOMing → transfer to AKS RP team
3. Verify ConfigMap formatting: `default-targets-metrics-keep-list`, `minimal-ingestion-profile`, `default-scrape-settings-enabled`
4. Isolate: set some node metrics to `true` and confirm they flow — determines if issue is control-plane-specific
5. Check Metrics Explorer for ingestion rate changes after config changes
6. **Customer documentation:**
   - [Monitor AKS control plane metrics](https://learn.microsoft.com/en-us/azure/aks/control-plane-metrics-monitor)
   - [Enable monitoring for AKS clusters](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable)
