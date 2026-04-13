# TSG: Remote Write Issues

1. Check if Prometheus version ≥ v2.45 (managed identity) or ≥ v2.48 (Entra ID app auth)
2. HTTP 403 → check `Monitoring Metrics Publisher` role on DCR (takes ~30 min to propagate)
3. No data flowing → `kubectl describe pod <prometheus-pod>`, check MSI assignment
4. Container restart loop → verify `AZURE_CLIENT_ID` and `IDENTITY_TYPE` env vars
5. If MDM ingestion issue → transfer to `Geneva Monitoring/Observability T1 Support (Not Live Site)`
6. **Customer documentation:**
   - [Remote write to Azure Monitor (overview)](https://learn.microsoft.com/en-us/azure/azure-monitor/metrics/prometheus-remote-write)
   - [Remote write using managed identity](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-remote-write-managed-identity)
   - [Remote write using Microsoft Entra authentication](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-remote-write-active-directory)
