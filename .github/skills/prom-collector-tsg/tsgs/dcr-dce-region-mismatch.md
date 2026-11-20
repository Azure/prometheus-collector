# TSG: DCR/DCE Region Mismatch or Missing

Run `tsg_triage`, check DCR and DCE configuration. Then:

> **⚠️ CRITICAL: Check if the cluster is private link FIRST.**
> Run `tsg_triage` → "AKS Cluster Network Settings" and look for `apiServerAccessProfile_enablePrivateCluster = true` or a `privateFQDN`.
> **Private link clusters are the #1 cause of missing DCR/DCE/DCRA.** When the cluster is private:
> - The AKS RP may fail to call the Monitor RP to create DCR/DCE during addon enablement (AMCS endpoint unreachable from private network)
> - The addon pods cannot reach AMCS to download DCR config → `TokenConfig.json missing` / DCR errors
> - The DCE **must** be in the same region as the AKS cluster AND accessible via private endpoint
> - Even if DCR/DCE exist, the DCRA may fail silently if private DNS resolution doesn't work
>
> If the cluster IS private link and DCR/DCE are missing, the most likely root cause is that **addon enablement partially failed** — the addon was installed on the cluster but the DCR/DCE/DCRA provisioning step failed silently due to private network restrictions. See also: TSG → Firewall / Network / Private Link.

1. **No DCR/DCE found in triage** — the monitoring addon is running but no DCR/DCE/DCRA exists. This means either they were never created or they were deleted
2. **Random region name in DCR/DCE** — when AKS and AMW are in different regions, the system may create DCR/DCE resources in an unexpected region. The DCE MUST be in the same region as the AKS cluster if it's a private link cluster
3. **Fix for region mismatch** — customer should create a new DCE in the AKS cluster's region and update the DCRA to point to it
4. **Validation** — check `tsg_triage` → "Internal DCE and DCR Ids" to see which DCE region is being used

**When DCR/DCE is missing, investigate with ARM logs:**

5. **Determine the correct ARM regional cluster** based on the AKS cluster's region (see `reference.md` → "Querying ARM Deployment Logs" for region mapping). Use `ARMPRODSEA` for Asia/Pacific, `ARMPRODEUS` for Americas, `ARMPRODWEU` for Europe
6. **Check if cluster is private link** — query `tsg_triage` → "AKS Cluster Network Settings". Private link clusters commonly have DCR/DCE provisioning failures during addon enablement
7. **Check if addon was ever enabled** — query `tsg_query` with the "Managed Clusters PUT Operations" query from `armInvestigation` category. If no `managedClusters` PUT exists in 30 days, the addon was enabled >30 days ago (beyond ARM retention)
8. **Check if DCR/DCE/DCRA were created** — query `tsg_query` with "Microsoft.Insights PUT/DELETE Operations". If zero PUT results, DCR/DCE/DCRA were never created for this subscription
9. **Check if AMW exists** — query "AMW All Operations" to see if there's an Azure Monitor Workspace. If only GETs (no PUTs/DELETEs), AMW exists but was created before the 30d ARM window
10. **Check if DCR/DCE/DCRA were deleted** — query the "All Subscription DELETEs on Microsoft.Insights" query. Note: DCR/DCE/DCRA can be in **any resource group** in the subscription, not necessarily the AKS cluster's RG. Examine `parentResource` — DELETEs on `microsoft.compute/virtualmachines` are VM-level DCRAs (unrelated to AKS), while DELETEs on `microsoft.containerservice/managedclusters` are AKS-related
11. **Check ARM outgoing requests** — query "ARM Outgoing Requests to Insights RP" to see if the AKS RP ever tried to call the Monitor RP to create DCR/DCE
12. **Resolution:**
    - If **private link + DCR/DCE never created**: ensure private endpoints exist for AMCS, re-enable monitoring via `az aks enable-addons --addon monitoring`, verify DCR/DCE/DCRA creation succeeded
    - If **DCR/DCE never created (non-private)**: re-enable monitoring via Azure portal or `az aks enable-addons --addon monitoring`
    - If **DCR/DCE deleted**: recreate the DCR/DCE/DCRA manually or re-enable the addon
13. **Customer documentation for private link clusters** — always share these docs when the cluster is private link:
    - [Configure Azure Private Link for Azure Monitor](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-configure?tabs=portal)
    - [Connect VMs and Kubernetes to Azure Monitor Private Link](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-vm-kubernetes?tabs=portal)
    - [Connect Azure Monitor Workspace to a Private Link](https://learn.microsoft.com/en-us/azure/azure-monitor/fundamentals/private-link-azure-monitor-workspace)
14. **Customer documentation for DCR/DCE configuration** — always share these docs:
    - [Data collection rules overview](https://learn.microsoft.com/en-us/azure/azure-monitor/data-collection/data-collection-rule-overview)
    - [Data collection endpoints overview](https://learn.microsoft.com/en-us/azure/azure-monitor/data-collection/data-collection-endpoint-overview)
    - [Enable monitoring for AKS clusters](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable)
