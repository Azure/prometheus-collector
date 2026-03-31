# TSG: Firewall / Network Connectivity / Private Link / AMPLS Issues

**Applies to:** Any cluster where outbound connectivity to Azure Monitor endpoints is blocked — including **ARC/Azure Local clusters behind customer firewalls**, AKS clusters with restrictive NSGs, private-link-enabled clusters, and AMPLS configurations.

**How to detect — the "TokenConfig.json" error chain:**

This is one of the most common patterns. When AMCS endpoints are unreachable (firewall, network policy, private link misconfiguration), the pod enters a restart loop with this characteristic error chain visible in `tsg_errors` and `tsg_logs`:

1. **`TokenConfig.json does not exist`** — logged every 15-30s in ReplicaSet/DaemonSet logs. MDSD/MA cannot download this file from AMCS because the endpoint is unreachable
2. **`AmcsTokenStore.cpp(54): Token config file is not...`** — MetricsExtension (ME) cannot initialize because the AMCS token store was never populated
3. **`MetricsExtensionService.cpp(213): Failed...`** — ME fails to start entirely because it has no authentication tokens
4. **Liveness probe HTTP 503: `"No configuration present for the AKS resource"`** — since ME never starts, the health endpoint returns 503
5. **Container killed & restarted** by kubelet after 3 consecutive failed probes (period=15s, failure=3). Restart count climbs to hundreds/thousands over days
6. **OtelCollector: `Exporting failed... connection refused on 127.0.0.1:55680`** — OtelCollector tries to export scraped metrics to ME's local OTLP endpoint, but ME is not listening. Data is dropped silently
7. **DCR/DCE/AMCS Configuration Errors: thousands per 6-hour window** — massive error volume in `tsg_errors` confirms persistent auth/config failure

**Key insight:** The OtelCollector "connection refused" errors look like an OtelCollector bug but are actually a SYMPTOM of ME not running. Always check the ME and MDSD errors first — they reveal the true root cause (missing TokenConfig.json → blocked endpoints).

**⚠️ Primary indicator for private clusters — MDSD 403 "Data collection endpoint must be used to access configuration over private link":**

This specific MDSD error means the cluster is private and AMCS **requires** a DCE (Data Collection Endpoint) to serve configuration, but no DCE is configured. This is now surfaced in `tsg_triage` as "⚠️ Missing DCE for Private Cluster (AMCS 403)". When you see this error:

1. **Confirm the cluster is private** — `tsg_triage` → "⚠️ Private Cluster Check (definitive)" uses `ManagedClusterSnapshot.privateLinkProfile.enablePrivateCluster` (Private V1) or `privateConnectProfile.enabled` (Private V2) — these booleans are the authoritative source, NOT the FQDN
2. **Check if a DCE exists** — `tsg_triage` → "Internal DCE and DCR Ids" will be empty
3. **Check if a DCR exists** — `tsg_triage` → "Data Collection Rules Associated with Cluster" will be empty
4. **Check if an AMW exists in the subscription** — `tsg_triage` → "Azure Monitor Workspace(s) in Subscription (fallback)" may show an AMW was created but never linked
5. **Root cause:** The addon was enabled but DCE + DCR + DCRA were never provisioned. This is an incomplete onboarding — typically the ARM template / Bicep / Terraform that creates these resources was never run, or only the AMW was created
6. **Fix:** Create DCE (in same region as cluster) + DCR + DCRA linking the cluster to the AMW. For private clusters, the DCE must also be linked to an Azure Monitor Private Link Scope (AMPLS)

**Investigation steps:**

Run `tsg_errors`, look for the error chain above, private link errors, and DNS errors. Then:

1. **Check for the TokenConfig.json error chain** — if `tsg_logs` shows repeated `TokenConfig.json does not exist` and `tsg_errors` shows `AmcsTokenStore` + `MetricsExtensionService` failures, the issue is blocked AMCS endpoints. Proceed to firewall rules below
2. **Check if cluster is ARC / Azure Local** — ARM resource ID containing `Microsoft.Kubernetes/connectedclusters` means this is an ARC cluster. ARC clusters run on-premises behind customer-managed firewalls, making blocked endpoints the most common root cause for pod restart issues
3. **DCE region mismatch** — DCE must be in same region as AKS cluster. If AKS and AMW are in different regions, create a new DCE in the AKS cluster's region
4. **DCE not linked to AMPLS** — check DCE Network Isolation settings, ensure correct Azure Monitor Private Link Scope is selected
5. **Firewall rules** — ensure outbound on port 443 is allowed to:
   - `*.ods.opinsights.azure.com`, `*.oms.opinsights.azure.com`
   - `*.monitoring.azure.com`, `*.metrics.ingest.monitor.azure.com`
   - `*.ingest.monitor.azure.com`, `login.microsoftonline.com`
   - `global.handler.control.monitor.azure.com`
   - `<cluster-region>.handler.control.monitor.azure.com`
6. **Validate connectivity** from a pod: `curl -sv https://global.handler.control.monitor.azure.com`
7. **After fixing** — delete the ama-metrics pods to force fresh config download. TokenConfig.json should appear within 2-3 minutes if endpoints are reachable
8. **Customer documentation for private link setups** — always share these docs:
   - [Configure Azure Private Link for Azure Monitor](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-configure?tabs=portal)
   - [Connect VMs and Kubernetes to Azure Monitor Private Link](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-vm-kubernetes?tabs=portal)
   - [Connect Azure Monitor Workspace to a Private Link](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-azure-monitor-workspace)
9. **Customer documentation for firewall / network** — always share these docs:
   - [Network firewall requirements for monitoring Kubernetes](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-firewall)
   - [AKS outbound network and FQDN rules](https://learn.microsoft.com/en-us/azure/aks/outbound-rules-control-egress)
   - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
