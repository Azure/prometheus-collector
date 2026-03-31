// Auto-generated from Azure Managed Prometheus TSGs ADX dashboard
// Dashboard ID: 94da59c1-df12-4134-96bb-82c6b32e6199
export const QUERIES = {
    triage: [
        {
            name: "Version",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "ClusterCoreCapacity"
| extend addonversion=tostring(customDimensions.agentversion)
| summarize dcount(tostring(customDimensions.cluster)) by addonversion, bin(timestamp, totimespan(Interval))
| order by timestamp desc 
| top 1 by timestamp`,
        },
        {
            name: "Component Versions (ME, OtelCollector, Golang, Prometheus)",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "ME_VERSION" or message has "OTEL_VERSION" or message has "GOLANG_VERSION" or message has "PROMETHEUS_VERSION"
| extend controllertype=tostring(customDimensions.controllertype)
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend meVersion = extract(@"ME_VERSION=([^\\s]+)", 1, cleaned)
| extend otelVersion = extract(@"OTEL_VERSION=([^\\s]+)", 1, cleaned)
| extend golangVersion = extract(@"GOLANG_VERSION=([^\\s]+)", 1, cleaned)
| extend promVersion = extract(@"PROMETHEUS_VERSION=([^\\s]+)", 1, cleaned)
| where isnotempty(meVersion) or isnotempty(otelVersion) or isnotempty(golangVersion) or isnotempty(promVersion)
| summarize LastSeen=max(timestamp) by meVersion, otelVersion, golangVersion, promVersion, controllertype
| order by LastSeen desc`,
        },
        {
            name: "Cluster Region",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "ClusterCoreCapacity"
| extend region=tostring(customDimensions.Region)
| project region
| take 1
//| extend interval=(_endTime - _startTime) / 4
//| summarize dcount(tostring(customDimensions.cluster)) by region, bin(timestamp, interval)
//| order by timestamp`,
        },
        {
            name: "AKS Cluster ID",
            datasource: "PrometheusAppInsights",
            kql: `print AKSClusterID`,
        },
        {
            name: "Azure Monitor Workspace",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| distinct AzureMonitorWorkspace=AMWAccountResourceId, MDMAccountName, Location, DCRId
| extend AzureMonitorWorkspaceName=tostring(split(AzureMonitorWorkspace, '/')[8])`,
        },
        {
            name: "MDM Account ID",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| distinct AzureMonitorWorkspace=AMWAccountResourceId, MDMAccountName, Location, DCRId
| extend AzureMonitorWorkspaceName=tostring(split(AzureMonitorWorkspace, '/')[8])`,
        },
        {
            name: "MDM Stamp",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| distinct AzureMonitorWorkspace=AMWAccountResourceId, MDMAccountName, Location, DCRId, MDMStampName`,
        },
        {
            name: "Azure Monitor Workspace Region",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| distinct Location`,
        },
        {
            name: "⚠️ Private Cluster Check (definitive — from ManagedClusterSnapshot)",
            datasource: "AKS",
            kql: `// DEFINITIVE private cluster check — uses subscription+clusterName, no CCP ID required
// privateLinkProfile.enablePrivateCluster = Private V1 (legacy)
// privateConnectProfile.enabled + !enablePublicEndpoint = Private V2 (private connect)
ManagedClusterSnapshot
| where TIMESTAMP > ago(7d)
| where subscription == '_subscriptionId'
| where clusterName has '_clusterName'
| top 1 by TIMESTAMP desc
| extend isPrivateV1 = coalesce(tobool(privateLinkProfile.enablePrivateCluster), false)
| extend isPrivateConnect = coalesce(tobool(privateConnectProfile.enabled), false)
| extend isPrivateV2 = isPrivateConnect and not(coalesce(tobool(privateConnectProfile.enablePublicEndpoint), false))
| extend isPrivateCluster = isPrivateV1 or isPrivateV2
| extend isNetworkIsolated = outboundType contains "none" or outboundType contains "block"
| extend hasHttpProxy = isnotempty(httpProxyConfig) and httpProxyConfig != "na"
| extend privateType = case(isPrivateV1, "Private V1 (privateLinkProfile)", isPrivateV2, "Private V2 (privateConnect)", "Not Private")
| project clusterName, ['Is Private Cluster']=isPrivateCluster, ['Private Type']=privateType, ['Network Isolated']=isNetworkIsolated, ['Has HTTP Proxy']=hasHttpProxy, privateDNSZone, publicNetworkAcess, azurePortalFQDN`,
        },
        {
            name: "Internal DCE and DCR Ids",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "ClusterCoreCapacity"
| take 1
| project Ids=split(tostring(customDimensions.DCRId), ";")
| mv-expand Ids
//| extend interval=(_endTime - _startTime) / 4
//| summarize dcount(tostring(customDimensions.cluster)) by addonversion, bin(timestamp, interval)
//| order by timestamp`,
        },
        {
            name: "⚠️ Missing DCE for Private Cluster (AMCS 403)",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| mv-expand message = split(message, "\\n") to typeof(string)
| where message contains 'Data collection endpoint must be used to access configuration over private link.'
| summarize HitCount=count(), FirstSeen=min(timestamp), LastSeen=max(timestamp) by controllertype=tostring(customDimensions.controllertype)
| extend Status="❌ MISSING DCE — private cluster requires a Data Collection Endpoint for AMCS access. Create a DCE and link it via DCRA."`,
        },
        {
            name: "Token Adapter Health",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend tadapterh=toint(customDimensions.tadapterh)
| extend tadapterf=toint(customDimensions.tadapterf)
| extend Values = pack(
                        "Token Adapter Healthy After _ Seconds", tadapterh,
                        "Token Adapter Unhealthy After _ Seconds", tadapterf
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=toint(Values[1])`,
        },
        {
            name: "Data Collection Rules Associated with Cluster",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId`,
        },
        {
            name: "Azure Monitor Workspace(s) from Scrape Config Routing",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| extend metricsAccountName=tostring(customDimensions.metricsAccountName)
| distinct metricsAccountName`,
        },
        {
            name: "Azure Monitor Workspace(s)",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId, Location, DCRId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| distinct AzureMonitorWorkspace=AMWAccountResourceId, MDMAccountName, Location, DCRId
`,
        },
        {
            name: "Azure Monitor Workspace(s) in Subscription (fallback)",
            datasource: "AMWInfo",
            kql: `// Falls back to subscription-level search when no DCR is linked to the cluster
AzureMonitorWorkspaceStatsDaily
| where Timestamp > ago(30d)
| where SubscriptionGuid == extract("/subscriptions/([^/]+)", 1, _cluster)
| distinct AMWAccountResourceId, AMWAccountUniqueName, MDMAccountName, Location, AMWCreationTime`,
        },
        {
            name: "AKS Cluster Network Settings",
            datasource: "AKS",
            kql: `// let globalFrom = _startTime;
// let globalTo = _endTime;
// let mcs = ManagedClusterSnapshot
// | where PreciseTimeStamp between (globalFrom .. globalTo)
// | where id =~ _cluster
// | extend managedClusterSKUTier = iff(isempty(sku.tier), "free", tolower(sku.tier))
// | extend k8sCurrentVersion = tostring(orchestratorProfile.orchestratorVersion)
// | extend mcsLoadBalancerProfile = todynamic(LoadBalancerProfile)
// | extend mcsSupportPlan = tostring(parse_json(orchestratorProfile).supportPlan)
// | summarize
//     mcsRows = count(),
//     free = countif(managedClusterSKUTier == "free"),
//     paid = countif(managedClusterSKUTier == "paid"),
//     premium = countif(managedClusterSKUTier == "premium"),
//     arg_min(PreciseTimeStamp, started = managedClusterSKUTier),
//     arg_max(PreciseTimeStamp, ended = managedClusterSKUTier),
//     arg_min(PreciseTimeStamp, minK8sVsn = k8sCurrentVersion),
//     arg_max(PreciseTimeStamp, maxK8sVsn = k8sCurrentVersion),
//     underlays = make_set(UnderlayName),
//     arg_max(PreciseTimeStamp, *), // this sucks for perf, but need to finish query first
//     lastSeen = max(PreciseTimeStamp)
// | extend transition = case (
//     started == 'free' and ended == 'paid', 'free -> paid',
//     started == 'paid' and ended == 'free', 'paid -> free',
//     started == 'free' and ended == started and paid > 0, 'free -> paid -> free',
//     started == 'paid' and ended == started and free > 0, 'paid -> free -> paid',
//     // lts handling
//     started == 'free' and ended == 'premium', 'free -> premium',
//     started == 'paid' and ended == 'premium', 'paid -> premium',
//     started == 'premium' and ended == 'free', 'premium -> free',
//     started == 'premium' and ended == 'paid', 'premium -> paid',
//     started == 'premium' and ended == started and paid > 0, 'premium -> paid -> premium',
//     started == 'premium' and ended == started and free > 0, 'premium -> free -> premium',
//     started
// )
// | extend k8sTransition = case (
//     minK8sVsn != maxK8sVsn, strcat(minK8sVsn, ' -> ' , maxK8sVsn),
//     maxK8sVsn
// )
// | project-away *1, *2, *3, *4, free, paid, started, ended, minK8sVsn, maxK8sVsn, TIMESTAMP
// | project-away LoadBalancerProfile
// ;
// mcs
// | extend clusterName = name
// | extend clusterBirthdate = todatetime(createdTime)
// | extend k8sCurrentVersion = k8sCurrentVersion
// | extend addonProfiles = tostring(addonProfiles)
// | extend isAutoScalingCluster = isAutoscalingCluster
// | extend isClusterAvailable = mcsRows > 0
// | extend isMSICluster = isnotempty(MSIProfile)
// | extend isPrivateCluster = iif(privateLinkProfile == "na" and privateConnectProfile == "na", false, true)
// | extend managedClusterSKUTier = managedClusterSKUTier
// | extend createdApiVersion = tostring(createApiVersion)
// | extend enableSecureKubelet = tostring(orchestratorProfile.kubernetesConfig.enableSecureKubelet)
// | extend lastStateChange = todatetime(powerState.lastStateChange)
// | extend powerState = tostring(powerState.code)
// | extend upgradeChannel = tostring(coalesce(tostring(autoUpgradeProfile.upgradeChannel), "none"))
// | extend oschannelenum = toint(coalesce(toint(autoUpgradeProfile.NodeOSUpgradeChannel), 0))
// | extend osUpgradechannel = case(oschannelenum == 0, "unspecified",
//     oschannelenum == 1 , "Unmanaged",
//     oschannelenum == 2, "None",
//     oschannelenum == 3, "SecurityPatch",
//     oschannelenum == 4, "NodeImage",
//     "Unknown")
// | extend nodeResourceGroupProfile = column_ifexists("nodeResourceGroupProfile", dynamic({"restrictionLevel":0}))
// | extend armResourceId = id
// | extend tags = todynamic(tags)
// | extend environment = Environment
// | extend supportPlan = iff(mcsSupportPlan == "2", "AKS Long-Term Support", "KubernetesOfficial")
// | project
//     lastSeen, region, clusterName, clusterVersion, clusterBirthdate, k8sCurrentVersion,
//     azurePortalFQDN, provisioningState, resourceName, underlayName, UnderlayName, underlays, pod, containerID,
//     container, RPTenant, Underlay, hostMachine, Host,  agentNodeCount,
//     customerPodCount, kubeSystemPodCount, underlayPodsNodes, addonProfiles, UnderlayClass, NodePoolResourceGroup,
//     NodePoolResourceGroupMCM, isAAD, isAutoScalingCluster, isClusterAvailable, isMSICluster,
//     isPrivateCluster,  managedClusterSKUTier,transition, k8sTransition,
//     clusterBlobJson, createdApiVersion, maxPodsPerNode, apiServerAuthorizedIPRanges,
//     deallocationTime, agentPoolProfiles, enableRbac, enableSecureKubelet, powerState, lastStateChange,
//     upgradeChannel, osUpgradechannel, hcpControlPlaneID, slbBackendPoolType, safeguardsProfile,
//     underlayId, genevaEndpoint, armResourceId, fleetMembershipProfile, fleetProfile, fleet_customize_ccm, fleet_resourceId, containerAppsEnvironmentId, environment, supportPlan

let local_clusterVersion = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
set best_effort=true;
let mcmBase = materialize(
    ManagedClusterMonitoring
    | where hcpControlPlaneID == local_clusterVersion
    | where entitytype == 'managedcluster'
    | top 1 by PreciseTimeStamp desc
    | extend messageJSON = parse_json(coalesce(msg, log))
    | extend messageJSON = iff(isnotempty(messageJSON.msg), parse_json(tostring(messageJSON.msg)), messageJSON)
    | extend cluster_id = hcpControlPlaneID
    | project cluster_id, messageJSON
);
// Migrated most of these to cluster snapshot
let mcm = mcmBase
| extend publicNetworkAccessDisabled = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "disabled", true, false)
| extend publicNetworkAccessSecuredByPerimeter = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "securedbyperimeter", true, false)
| extend nrgLockdownRestrictionLevel = tostring(messageJSON.nodeResourceGroupLockdownProfile.restrictionLevel)
| extend nrgLockdownRestrictionLevel = iif(isempty(nrgLockdownRestrictionLevel), "1", nrgLockdownRestrictionLevel)
| extend isAzureServiceMesh = iff(tostring(messageJSON.serviceMeshProfile.mode) == "1", true, false)
| extend isIMDSRestrictionEnabled = iff(tostring(messageJSON.NetworkProfile.podLinkLocalAccess) == "None", true, false)
| project publicNetworkAccessDisabled, publicNetworkAccessSecuredByPerimeter, nrgLockdownRestrictionLevel, 
    isAzureServiceMesh, isIMDSRestrictionEnabled
| project features = bag_pack(
    "Has Limited Network Access", publicNetworkAccessDisabled or publicNetworkAccessSecuredByPerimeter,
    "Has IMDS Restriction", isIMDSRestrictionEnabled
)
| mv-expand bagexpansion=array features
| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
let clusterSnapshot = ManagedClusterSnapshot
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where cluster_id == local_clusterVersion
| top 1 by PreciseTimeStamp desc 
| project cluster_id, StorageProfile, nodeProvisioningProfile, orchestratorProfile, outboundType, azureMonitorProfile
    , CustomerProvidedKubenetRouteTableID, autoUpgradeProfile, securityProfile, oidcProfile, privateLinkProfile, privateConnectProfile
    , addonProfiles, workloadAutoScalerProfile, fleetProfile, fleetMembershipProfile, enableNamespaceResources, apiServerAuthorizedIPRanges
    , sku, diskEncryptionSetID, metricsProfile, ingressProfile, LoadBalancerProfile, staticEgressGatewayProfile, extendedLocation
    , httpProxyConfig, isControlPlaneAZEnabled
| extend upgradeChannel = tostring(autoUpgradeProfile.upgradeChannel)
| extend upgradeChannel = iif(isempty(upgradeChannel), "none", upgradeChannel)
| extend isPrivateV1 = coalesce(tobool(privateLinkProfile.enablePrivateCluster), false)
| extend isPrivateConnect = coalesce(tobool(privateConnectProfile.enabled), false)
| extend isPrivateV2 = isPrivateConnect and not(coalesce(tobool(privateConnectProfile.enablePublicEndpoint), false))
| extend isPrivateCluster = isPrivateV1 or isPrivateV2
| extend isOverlayVPAEnabled = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.enabled)
| extend isAddonAutoscaling = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.addonAutoscaling == "2")
| extend isAzureCNIOverlay = tobool(orchestratorProfile.kubernetesConfig.networkPluginMode == "overlay")
| extend networkPolicy = tostring(orchestratorProfile.kubernetesConfig.networkPolicy)
// migrated from mcmBase
| project features = bag_pack (
    //"Azure Monitor Metrics", tobool(azureMonitorProfile.metrics.enabled),
    //"Auto Upgrade", upgradeChannel != "none",
    "Is Network Isolated Cluster", outboundType contains "none" or outboundType contains "block",
    "Is Private Cluster", isPrivateCluster,
    //"Network Observability (Retina)", tobool(orchestratorProfile.kubernetesConfig.containerNetworkMonitoring.enabled),
    "Has HTTP Proxy", isnotempty(httpProxyConfig) and httpProxyConfig != 'na'
)
| mv-expand bagexpansion=array features
| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
let bbm = BlackboxMonitoringActivity
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where ccpNamespace == local_clusterVersion
| top 1 by PreciseTimeStamp desc
| extend enableAzureKeyvaultSecretsProviderAddon=parse_json(addonProfiles).azureKeyvaultSecretsProvider.enabled
| project isAAD, isAutoScalingCluster, isAzureCNI, isClusterAvailable, isMSICluster,
    isStandardLB, hasTiller, isPLSPoolingEnabled, isManagedAAD, isAzureRBACEnabled, isAzureKeyVaultKms,
    virtualKubelet, enableAzureKeyvaultSecretsProviderAddon, hasAADPI, hasManagedAADPI
//| project features = bag_pack(
    //"Auto Scale", isAutoScalingCluster, 
    //"Cluster Available", isClusterAvailable,
    //"MSI", isMSICluster
//)
//| mv-expand bagexpansion=array features
//| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
union mcm, clusterSnapshot
| extend State = coalesce(State, "false")
| order by FeatureName asc`,
        },
        {
            name: "AKS Cluster Addons Enabled",
            datasource: "AKS",
            kql: `// let globalFrom = _startTime;
// let globalTo = _endTime;
// let mcs = ManagedClusterSnapshot
// | where PreciseTimeStamp between (globalFrom .. globalTo)
// | where id =~ _cluster
// | extend managedClusterSKUTier = iff(isempty(sku.tier), "free", tolower(sku.tier))
// | extend k8sCurrentVersion = tostring(orchestratorProfile.orchestratorVersion)
// | extend mcsLoadBalancerProfile = todynamic(LoadBalancerProfile)
// | extend mcsSupportPlan = tostring(parse_json(orchestratorProfile).supportPlan)
// | summarize
//     mcsRows = count(),
//     free = countif(managedClusterSKUTier == "free"),
//     paid = countif(managedClusterSKUTier == "paid"),
//     premium = countif(managedClusterSKUTier == "premium"),
//     arg_min(PreciseTimeStamp, started = managedClusterSKUTier),
//     arg_max(PreciseTimeStamp, ended = managedClusterSKUTier),
//     arg_min(PreciseTimeStamp, minK8sVsn = k8sCurrentVersion),
//     arg_max(PreciseTimeStamp, maxK8sVsn = k8sCurrentVersion),
//     underlays = make_set(UnderlayName),
//     arg_max(PreciseTimeStamp, *), // this sucks for perf, but need to finish query first
//     lastSeen = max(PreciseTimeStamp)
// | extend transition = case (
//     started == 'free' and ended == 'paid', 'free -> paid',
//     started == 'paid' and ended == 'free', 'paid -> free',
//     started == 'free' and ended == started and paid > 0, 'free -> paid -> free',
//     started == 'paid' and ended == started and free > 0, 'paid -> free -> paid',
//     // lts handling
//     started == 'free' and ended == 'premium', 'free -> premium',
//     started == 'paid' and ended == 'premium', 'paid -> premium',
//     started == 'premium' and ended == 'free', 'premium -> free',
//     started == 'premium' and ended == 'paid', 'premium -> paid',
//     started == 'premium' and ended == started and paid > 0, 'premium -> paid -> premium',
//     started == 'premium' and ended == started and free > 0, 'premium -> free -> premium',
//     started
// )
// | extend k8sTransition = case (
//     minK8sVsn != maxK8sVsn, strcat(minK8sVsn, ' -> ' , maxK8sVsn),
//     maxK8sVsn
// )
// | project-away *1, *2, *3, *4, free, paid, started, ended, minK8sVsn, maxK8sVsn, TIMESTAMP
// | project-away LoadBalancerProfile
// ;
// mcs
// | extend clusterName = name
// | extend clusterBirthdate = todatetime(createdTime)
// | extend k8sCurrentVersion = k8sCurrentVersion
// | extend addonProfiles = tostring(addonProfiles)
// | extend isAutoScalingCluster = isAutoscalingCluster
// | extend isClusterAvailable = mcsRows > 0
// | extend isMSICluster = isnotempty(MSIProfile)
// | extend isPrivateCluster = iif(privateLinkProfile == "na" and privateConnectProfile == "na", false, true)
// | extend managedClusterSKUTier = managedClusterSKUTier
// | extend createdApiVersion = tostring(createApiVersion)
// | extend enableSecureKubelet = tostring(orchestratorProfile.kubernetesConfig.enableSecureKubelet)
// | extend lastStateChange = todatetime(powerState.lastStateChange)
// | extend powerState = tostring(powerState.code)
// | extend upgradeChannel = tostring(coalesce(tostring(autoUpgradeProfile.upgradeChannel), "none"))
// | extend oschannelenum = toint(coalesce(toint(autoUpgradeProfile.NodeOSUpgradeChannel), 0))
// | extend osUpgradechannel = case(oschannelenum == 0, "unspecified",
//     oschannelenum == 1 , "Unmanaged",
//     oschannelenum == 2, "None",
//     oschannelenum == 3, "SecurityPatch",
//     oschannelenum == 4, "NodeImage",
//     "Unknown")
// | extend nodeResourceGroupProfile = column_ifexists("nodeResourceGroupProfile", dynamic({"restrictionLevel":0}))
// | extend armResourceId = id
// | extend tags = todynamic(tags)
// | extend environment = Environment
// | extend supportPlan = iff(mcsSupportPlan == "2", "AKS Long-Term Support", "KubernetesOfficial")
// | project
//     lastSeen, region, clusterName, clusterVersion, clusterBirthdate, k8sCurrentVersion,
//     azurePortalFQDN, provisioningState, resourceName, underlayName, UnderlayName, underlays, pod, containerID,
//     container, RPTenant, Underlay, hostMachine, Host,  agentNodeCount,
//     customerPodCount, kubeSystemPodCount, underlayPodsNodes, addonProfiles, UnderlayClass, NodePoolResourceGroup,
//     NodePoolResourceGroupMCM, isAAD, isAutoScalingCluster, isClusterAvailable, isMSICluster,
//     isPrivateCluster,  managedClusterSKUTier,transition, k8sTransition,
//     clusterBlobJson, createdApiVersion, maxPodsPerNode, apiServerAuthorizedIPRanges,
//     deallocationTime, agentPoolProfiles, enableRbac, enableSecureKubelet, powerState, lastStateChange,
//     upgradeChannel, osUpgradechannel, hcpControlPlaneID, slbBackendPoolType, safeguardsProfile,
//     underlayId, genevaEndpoint, armResourceId, fleetMembershipProfile, fleetProfile, fleet_customize_ccm, fleet_resourceId, containerAppsEnvironmentId, environment, supportPlan

let local_clusterVersion = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
set best_effort=true;
let mcmBase = materialize(
    ManagedClusterMonitoring
    | where hcpControlPlaneID == local_clusterVersion
    | where entitytype == 'managedcluster'
    | top 1 by PreciseTimeStamp desc
    | extend messageJSON = parse_json(coalesce(msg, log))
    | extend messageJSON = iff(isnotempty(messageJSON.msg), parse_json(tostring(messageJSON.msg)), messageJSON)
    | extend cluster_id = hcpControlPlaneID
    | project cluster_id, messageJSON
);
// Migrated most of these to cluster snapshot
// let mcm = mcmBase
// | extend publicNetworkAccessDisabled = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "disabled", true, false)
// | extend publicNetworkAccessSecuredByPerimeter = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "securedbyperimeter", true, false)
// | extend nrgLockdownRestrictionLevel = tostring(messageJSON.nodeResourceGroupLockdownProfile.restrictionLevel)
// | extend nrgLockdownRestrictionLevel = iif(isempty(nrgLockdownRestrictionLevel), "1", nrgLockdownRestrictionLevel)
// | extend isAzureServiceMesh = iff(tostring(messageJSON.serviceMeshProfile.mode) == "1", true, false)
// | extend isIMDSRestrictionEnabled = iff(tostring(messageJSON.NetworkProfile.podLinkLocalAccess) == "None", true, false)
// | project publicNetworkAccessDisabled, publicNetworkAccessSecuredByPerimeter, nrgLockdownRestrictionLevel, 
//     isAzureServiceMesh, isIMDSRestrictionEnabled
// | project features = bag_pack(
//     "LimitedNetworkAccess", publicNetworkAccessDisabled or publicNetworkAccessSecuredByPerimeter,
//     "IMDS Restriction", isIMDSRestrictionEnabled
// )
// | mv-expand bagexpansion=array features
// | project FeatureName = tostring(features[0]), State = tostring(features[1])
// ;
let clusterSnapshot = ManagedClusterSnapshot
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where cluster_id == local_clusterVersion
| top 1 by PreciseTimeStamp desc 
| project cluster_id, StorageProfile, nodeProvisioningProfile, orchestratorProfile, outboundType, azureMonitorProfile
    , CustomerProvidedKubenetRouteTableID, autoUpgradeProfile, securityProfile, oidcProfile, privateLinkProfile, privateConnectProfile
    , addonProfiles, workloadAutoScalerProfile, fleetProfile, fleetMembershipProfile, enableNamespaceResources, apiServerAuthorizedIPRanges
    , sku, diskEncryptionSetID, metricsProfile, ingressProfile, LoadBalancerProfile, staticEgressGatewayProfile, extendedLocation
    , httpProxyConfig, isControlPlaneAZEnabled
| extend upgradeChannel = tostring(autoUpgradeProfile.upgradeChannel)
| extend upgradeChannel = iif(isempty(upgradeChannel), "none", upgradeChannel)
| extend isPrivateV1 = coalesce(tobool(privateLinkProfile.enablePrivateCluster), false)
| extend isPrivateConnect = coalesce(tobool(privateConnectProfile.enabled), false)
| extend isPrivateV2 = isPrivateConnect and not(coalesce(tobool(privateConnectProfile.enablePublicEndpoint), false))
| extend isPrivateCluster = isPrivateV1 or isPrivateV2
| extend isOverlayVPAEnabled = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.enabled)
| extend isAddonAutoscaling = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.addonAutoscaling == "2")
| extend isAzureCNIOverlay = tobool(orchestratorProfile.kubernetesConfig.networkPluginMode == "overlay")
| extend networkPolicy = tostring(orchestratorProfile.kubernetesConfig.networkPolicy)
// migrated from mcmBase
| project features = bag_pack (
    "Azure Monitor Metrics", tobool(azureMonitorProfile.metrics.enabled),
    //"Auto Upgrade", upgradeChannel != "none",
    //"Network Isolated Cluster", outboundType contains "none" or outboundType contains "block",
    //"Private Cluster", isPrivateCluster
    "Network Observability", tobool(orchestratorProfile.kubernetesConfig.containerNetworkMonitoring.enabled),
    //"HTTP Proxy", isnotempty(httpProxyConfig) and httpProxyConfig != 'na',
    "Azure Defender",tobool(securityProfile.azureDefender.enabled)
)
| mv-expand bagexpansion=array features
| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
// let bbm = BlackboxMonitoringActivity
// | where PreciseTimeStamp between (queryFrom .. queryTo)
// | where ccpNamespace == local_clusterVersion
// | top 1 by PreciseTimeStamp desc
// | extend enableAzureKeyvaultSecretsProviderAddon=parse_json(addonProfiles).azureKeyvaultSecretsProvider.enabled
// | project isAAD, isAutoScalingCluster, isAzureCNI, isClusterAvailable, isMSICluster,
//     isStandardLB, hasTiller, isPLSPoolingEnabled, isManagedAAD, isAzureRBACEnabled, isAzureKeyVaultKms,
//     virtualKubelet, enableAzureKeyvaultSecretsProviderAddon, hasAADPI, hasManagedAADPI
//| project features = bag_pack(
    //"Auto Scale", isAutoScalingCluster, 
    //"Cluster Available", isClusterAvailable,
    //"MSI", isMSICluster
//)
//| mv-expand bagexpansion=array features
//| project FeatureName = tostring(features[0]), State = tostring(features[1])
// ;
union clusterSnapshot
| extend State = coalesce(State, "false")
| order by FeatureName asc`,
        },
        {
            name: "AKS Cluster Settings",
            datasource: "AKS",
            kql: `// let globalFrom = _startTime;
// let globalTo = _endTime;
// let mcs = ManagedClusterSnapshot
// | where PreciseTimeStamp between (globalFrom .. globalTo)
// | where id =~ _cluster
// | extend managedClusterSKUTier = iff(isempty(sku.tier), "free", tolower(sku.tier))
// | extend k8sCurrentVersion = tostring(orchestratorProfile.orchestratorVersion)
// | extend mcsLoadBalancerProfile = todynamic(LoadBalancerProfile)
// | extend mcsSupportPlan = tostring(parse_json(orchestratorProfile).supportPlan)
// | summarize
//     mcsRows = count(),
//     free = countif(managedClusterSKUTier == "free"),
//     paid = countif(managedClusterSKUTier == "paid"),
//     premium = countif(managedClusterSKUTier == "premium"),
//     arg_min(PreciseTimeStamp, started = managedClusterSKUTier),
//     arg_max(PreciseTimeStamp, ended = managedClusterSKUTier),
//     arg_min(PreciseTimeStamp, minK8sVsn = k8sCurrentVersion),
//     arg_max(PreciseTimeStamp, maxK8sVsn = k8sCurrentVersion),
//     underlays = make_set(UnderlayName),
//     arg_max(PreciseTimeStamp, *), // this sucks for perf, but need to finish query first
//     lastSeen = max(PreciseTimeStamp)
// | extend transition = case (
//     started == 'free' and ended == 'paid', 'free -> paid',
//     started == 'paid' and ended == 'free', 'paid -> free',
//     started == 'free' and ended == started and paid > 0, 'free -> paid -> free',
//     started == 'paid' and ended == started and free > 0, 'paid -> free -> paid',
//     // lts handling
//     started == 'free' and ended == 'premium', 'free -> premium',
//     started == 'paid' and ended == 'premium', 'paid -> premium',
//     started == 'premium' and ended == 'free', 'premium -> free',
//     started == 'premium' and ended == 'paid', 'premium -> paid',
//     started == 'premium' and ended == started and paid > 0, 'premium -> paid -> premium',
//     started == 'premium' and ended == started and free > 0, 'premium -> free -> premium',
//     started
// )
// | extend k8sTransition = case (
//     minK8sVsn != maxK8sVsn, strcat(minK8sVsn, ' -> ' , maxK8sVsn),
//     maxK8sVsn
// )
// | project-away *1, *2, *3, *4, free, paid, started, ended, minK8sVsn, maxK8sVsn, TIMESTAMP
// | project-away LoadBalancerProfile
// ;
// mcs
// | extend clusterName = name
// | extend clusterBirthdate = todatetime(createdTime)
// | extend k8sCurrentVersion = k8sCurrentVersion
// | extend addonProfiles = tostring(addonProfiles)
// | extend isAutoScalingCluster = isAutoscalingCluster
// | extend isClusterAvailable = mcsRows > 0
// | extend isMSICluster = isnotempty(MSIProfile)
// | extend isPrivateCluster = iif(privateLinkProfile == "na" and privateConnectProfile == "na", false, true)
// | extend managedClusterSKUTier = managedClusterSKUTier
// | extend createdApiVersion = tostring(createApiVersion)
// | extend enableSecureKubelet = tostring(orchestratorProfile.kubernetesConfig.enableSecureKubelet)
// | extend lastStateChange = todatetime(powerState.lastStateChange)
// | extend powerState = tostring(powerState.code)
// | extend upgradeChannel = tostring(coalesce(tostring(autoUpgradeProfile.upgradeChannel), "none"))
// | extend oschannelenum = toint(coalesce(toint(autoUpgradeProfile.NodeOSUpgradeChannel), 0))
// | extend osUpgradechannel = case(oschannelenum == 0, "unspecified",
//     oschannelenum == 1 , "Unmanaged",
//     oschannelenum == 2, "None",
//     oschannelenum == 3, "SecurityPatch",
//     oschannelenum == 4, "NodeImage",
//     "Unknown")
// | extend nodeResourceGroupProfile = column_ifexists("nodeResourceGroupProfile", dynamic({"restrictionLevel":0}))
// | extend armResourceId = id
// | extend tags = todynamic(tags)
// | extend environment = Environment
// | extend supportPlan = iff(mcsSupportPlan == "2", "AKS Long-Term Support", "KubernetesOfficial")
// | project
//     lastSeen, region, clusterName, clusterVersion, clusterBirthdate, k8sCurrentVersion,
//     azurePortalFQDN, provisioningState, resourceName, underlayName, UnderlayName, underlays, pod, containerID,
//     container, RPTenant, Underlay, hostMachine, Host,  agentNodeCount,
//     customerPodCount, kubeSystemPodCount, underlayPodsNodes, addonProfiles, UnderlayClass, NodePoolResourceGroup,
//     NodePoolResourceGroupMCM, isAAD, isAutoScalingCluster, isClusterAvailable, isMSICluster,
//     isPrivateCluster,  managedClusterSKUTier,transition, k8sTransition,
//     clusterBlobJson, createdApiVersion, maxPodsPerNode, apiServerAuthorizedIPRanges,
//     deallocationTime, agentPoolProfiles, enableRbac, enableSecureKubelet, powerState, lastStateChange,
//     upgradeChannel, osUpgradechannel, hcpControlPlaneID, slbBackendPoolType, safeguardsProfile,
//     underlayId, genevaEndpoint, armResourceId, fleetMembershipProfile, fleetProfile, fleet_customize_ccm, fleet_resourceId, containerAppsEnvironmentId, environment, supportPlan

let local_clusterVersion = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
set best_effort=true;
let mcmBase = materialize(
    ManagedClusterMonitoring
    | where hcpControlPlaneID == local_clusterVersion
    | where entitytype == 'managedcluster'
    | top 1 by PreciseTimeStamp desc
    | extend messageJSON = parse_json(coalesce(msg, log))
    | extend messageJSON = iff(isnotempty(messageJSON.msg), parse_json(tostring(messageJSON.msg)), messageJSON)
    | extend cluster_id = hcpControlPlaneID
    | project cluster_id, messageJSON
);
// Migrated most of these to cluster snapshot
// let mcm = mcmBase
// | extend publicNetworkAccessDisabled = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "disabled", true, false)
// | extend publicNetworkAccessSecuredByPerimeter = iff(tolower(tostring(messageJSON.properties.PublicNetworkAccess)) == "securedbyperimeter", true, false)
// | extend nrgLockdownRestrictionLevel = tostring(messageJSON.nodeResourceGroupLockdownProfile.restrictionLevel)
// | extend nrgLockdownRestrictionLevel = iif(isempty(nrgLockdownRestrictionLevel), "1", nrgLockdownRestrictionLevel)
// | extend isAzureServiceMesh = iff(tostring(messageJSON.serviceMeshProfile.mode) == "1", true, false)
// | extend isIMDSRestrictionEnabled = iff(tostring(messageJSON.NetworkProfile.podLinkLocalAccess) == "None", true, false)
// | project publicNetworkAccessDisabled, publicNetworkAccessSecuredByPerimeter, nrgLockdownRestrictionLevel, 
//     isAzureServiceMesh, isIMDSRestrictionEnabled
// | project features = bag_pack(
//     "LimitedNetworkAccess", publicNetworkAccessDisabled or publicNetworkAccessSecuredByPerimeter,
//     "IMDS Restriction", isIMDSRestrictionEnabled
// )
// | mv-expand bagexpansion=array features
// | project FeatureName = tostring(features[0]), State = tostring(features[1])
// ;
let clusterSnapshot = ManagedClusterSnapshot
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where cluster_id == local_clusterVersion
| top 1 by PreciseTimeStamp desc 
| project cluster_id, StorageProfile, nodeProvisioningProfile, orchestratorProfile, outboundType, azureMonitorProfile
    , CustomerProvidedKubenetRouteTableID, autoUpgradeProfile, securityProfile, oidcProfile, privateLinkProfile, privateConnectProfile
    , addonProfiles, workloadAutoScalerProfile, fleetProfile, fleetMembershipProfile, enableNamespaceResources, apiServerAuthorizedIPRanges
    , sku, diskEncryptionSetID, metricsProfile, ingressProfile, LoadBalancerProfile, staticEgressGatewayProfile, extendedLocation
    , httpProxyConfig, isControlPlaneAZEnabled
| extend upgradeChannel = tostring(autoUpgradeProfile.upgradeChannel)
| extend upgradeChannel = iif(isempty(upgradeChannel), "none", upgradeChannel)
| extend isPrivateV1 = coalesce(tobool(privateLinkProfile.enablePrivateCluster), false)
| extend isPrivateConnect = coalesce(tobool(privateConnectProfile.enabled), false)
| extend isPrivateV2 = isPrivateConnect and not(coalesce(tobool(privateConnectProfile.enablePublicEndpoint), false))
| extend isPrivateCluster = isPrivateV1 or isPrivateV2
| extend isOverlayVPAEnabled = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.enabled)
| extend isAddonAutoscaling = tobool(workloadAutoScalerProfile.verticalPodAutoscaler.addonAutoscaling == "2")
| extend isAzureCNIOverlay = tobool(orchestratorProfile.kubernetesConfig.networkPluginMode == "overlay")
| extend networkPolicy = tostring(orchestratorProfile.kubernetesConfig.networkPolicy)
// migrated from mcmBase
| project features = bag_pack (
    //"Azure Monitor Metrics", tobool(azureMonitorProfile.metrics.enabled),
    "Auto Upgrade Enabled", upgradeChannel != "none"
    //"Network Isolated Cluster", outboundType contains "none" or outboundType contains "block",
    //"Private Cluster", isPrivateCluster
    //"Network Observability (Retina)", tobool(orchestratorProfile.kubernetesConfig.containerNetworkMonitoring.enabled)
    //"HTTP Proxy", isnotempty(httpProxyConfig) and httpProxyConfig != 'na'
)
| mv-expand bagexpansion=array features
| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
let bbm = BlackboxMonitoringActivity
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where ccpNamespace == local_clusterVersion
| top 1 by PreciseTimeStamp desc
| extend enableAzureKeyvaultSecretsProviderAddon=parse_json(addonProfiles).azureKeyvaultSecretsProvider.enabled
| project isAAD, isAutoScalingCluster, isAzureCNI, isClusterAvailable, isMSICluster,
    isStandardLB, hasTiller, isPLSPoolingEnabled, isManagedAAD, isAzureRBACEnabled, isAzureKeyVaultKms,
    virtualKubelet, enableAzureKeyvaultSecretsProviderAddon, hasAADPI, hasManagedAADPI
| project features = bag_pack(
    "Auto Scale Enabled", isAutoScalingCluster
    //"Cluster Available", isClusterAvailable,
    //"MSI", isMSICluster
)
| mv-expand bagexpansion=array features
| project FeatureName = tostring(features[0]), State = tostring(features[1])
;
union bbm, clusterSnapshot
| extend State = coalesce(State, "false")
| order by FeatureName asc`,
        },
        {
            name: "AKS Cluster State",
            datasource: "AKS",
            kql: `let local_clusterVersion = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
BlackboxMonitoringActivity
| where PreciseTimeStamp between (queryFrom .. queryTo)
| where ccpNamespace == local_clusterVersion
| top 1 by PreciseTimeStamp desc
| project state`,
        },
        {
            name: "CCP Cluster ID",
            datasource: "AKS",
            kql: `let resourceid = split(_cluster, "/");
let subscription = tostring(resourceid[2]);
let resourceGroup = tostring(resourceid[4]);
ManagedClusterMonitoring
    | where TIMESTAMP > _startTime and TIMESTAMP < _endTime
    | where resourceGroupName =~ resourceGroup
    | where subscriptionID == subscription
    | where entitytype == 'managedcluster'
    | top 1 by PreciseTimeStamp desc
    | extend messageJSON = parse_json(coalesce(msg, log))
    | extend messageJSON = iff(isnotempty(messageJSON.msg), parse_json(tostring(messageJSON.msg)), messageJSON)
    | extend clusterid = tostring(messageJSON.containerService.id)
    | where clusterid =~ _cluster
    | extend cluster_id = hcpControlPlaneID
    | project cluster_id, subscription, resourceGroup`,
        },
        {
            name: "CCP Cluster ID (AgentPoolSnapshot fallback)",
            datasource: "AKS",
            kql: `AgentPoolSnapshot
| where resource_id =~ _cluster
| where TIMESTAMP > ago(7d)
| where isnotempty(cluster_id)
| top 1 by TIMESTAMP desc
| project cluster_id`,
        },
        {
            name: "Node Pool Capacity",
            datasource: "AKS",
            kql: `AgentPoolSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| summarize arg_max(PreciseTimeStamp, *) by name
| project name, mode, currentNodes=size, vmSize, osType, orchestratorVersion,
    enableAutoScaling, maxCount=toint(maxCount), minCount=toint(minCount), provisioningState,
    isFull=iff(enableAutoScaling == true, size >= toint(maxCount), false)`,
        },
        {
            name: "Node Conditions (Memory/Disk/PID Pressure)",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where objectRef.resource == "nodes"
| where verb in ("patch", "update")
| extend node = tostring(objectRef.name)
| summarize arg_max(PreciseTimeStamp, *) by node
| mv-apply condition = responseObject.status.conditions on (
    where condition.type in ("MemoryPressure", "DiskPressure", "PIDPressure", "Ready")
    | project conditionType = tostring(condition.type), conditionStatus = tostring(condition.status), reason = tostring(condition.reason), message = tostring(condition.message), lastTransition = todatetime(condition.lastTransitionTime)
)
| extend nodepool = tostring(responseObject.metadata.labels.['agentpool'])
| extend hasIssue = (conditionType == "Ready" and conditionStatus != "True") or (conditionType != "Ready" and conditionStatus == "True")
| project PreciseTimeStamp, node, nodepool, conditionType, conditionStatus, reason, message, lastTransition, hasIssue
| order by hasIssue desc, node asc, conditionType asc`,
        },
        {
            name: "Node Allocatable Resources",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where objectRef.resource == "nodes"
| where verb in ("patch", "update")
| extend node = tostring(objectRef.name)
| summarize arg_max(PreciseTimeStamp, *) by node
| extend nodepool = tostring(responseObject.metadata.labels.['agentpool'])
| extend allocatable_memory = tostring(responseObject.status.allocatable.memory)
| extend allocatable_cpu = tostring(responseObject.status.allocatable.cpu)
| extend capacity_memory = tostring(responseObject.status.capacity.memory)
| extend capacity_cpu = tostring(responseObject.status.capacity.cpu)
| extend allocatable_pods = tostring(responseObject.status.allocatable.pods)
| extend capacity_pods = tostring(responseObject.status.capacity.pods)
| project PreciseTimeStamp, node, nodepool, allocatable_memory, capacity_memory, allocatable_cpu, capacity_cpu, allocatable_pods, capacity_pods`,
        },
        {
            name: "AgentPool Autoscaling History",
            datasource: "AKS",
            kql: `AgentPoolSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| extend enableAutoScaling = coalesce(enableAutoScaling, false)
| summarize arg_max(PreciseTimeStamp, *) by name, bin(PreciseTimeStamp, 1h)
| project PreciseTimeStamp, name, mode, currentNodes=size, vmSize,
    enableAutoScaling, minCount=toint(minCount), maxCount=toint(maxCount), provisioningState,
    isFull=iff(enableAutoScaling == true, size >= toint(maxCount), false)
| order by name asc, PreciseTimeStamp asc`,
        },
        {
            name: "AKS Upgrade History",
            datasource: "AKS",
            kql: `AgentPoolSnapshot
| where resource_id =~ _cluster
| where TIMESTAMP > ago(48h)
| summarize min_ts=min(TIMESTAMP), max_ts=max(TIMESTAMP), latest_size=arg_max(TIMESTAMP, size).size by name, orchestratorVersion
| order by name asc, min_ts asc`,
        },
        {
            name: "Node Pool Versions (resource_id fallback)",
            datasource: "AKS",
            kql: `AgentPoolSnapshot
| where resource_id =~ _cluster
| where TIMESTAMP > ago(48h)
| summarize arg_max(TIMESTAMP, *) by name
| project TIMESTAMP, name, orchestratorVersion, osSku, currentNodes=size, distroVersion, imageRef
| order by name asc`,
        },
        {
            name: "Cluster Overview (subscription+name lookup, no CCP required)",
            datasource: "AKS",
            kql: `// Direct lookup — works even when CCP cluster ID resolution fails
ManagedClusterSnapshot
| where TIMESTAMP > ago(7d)
| where subscription == '_subscriptionId'
| where clusterName has '_clusterName'
| top 1 by TIMESTAMP desc
| extend k8sVersion = tostring(orchestratorProfile.orchestratorVersion)
| extend skuTier = iff(isempty(sku.tier), "free", tolower(sku.tier))
| extend isPrivateV1 = coalesce(tobool(privateLinkProfile.enablePrivateCluster), false)
| extend isPrivateConnect = coalesce(tobool(privateConnectProfile.enabled), false)
| extend isPrivateCluster = isPrivateV1 or isPrivateConnect
| extend upgradeChannel = coalesce(tostring(autoUpgradeProfile.upgradeChannel), "none")
| extend azureMonitorMetricsEnabled = tobool(azureMonitorProfile.metrics.enabled)
| project clusterName, location, k8sVersion, skuTier, isPrivateCluster, upgradeChannel, azureMonitorMetricsEnabled, azurePortalFQDN, createdTime`,
        },
    ],
    errors: [
        {
            name: "DCR/DCE/AMCS Configuration Errors Found",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| mv-expand message = split(message, "\\n") to typeof(string)
| where message contains 'TokenConfig.json does not exist' or message contains 'No configuration present for the AKS resource' or message contains 'InvalidAccess' or message contains 'Data collection endpoint must be used to access configuration over private link.'
| extend controllertype=tostring(customDimensions.controllertype)
| make-series count_=count() default=0 on timestamp in range(_startTime, _endTime, totimespan(Interval)) by controllertype
| mvexpand timestamp, count_
| extend timestamp=todatetime(timestamp), count_=toint(count_)`,
        },
        {
            name: "ContainerLog Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend os = tostring(customDimensions.osType)
| where tostring(customDimensions.tag) == "prometheus.log.prometheuscollectorcontainer"
| mv-expand message = split(message, "\\n") to typeof(string) 
| where (message contains 'error' or message contains 'E!' or message contains 'warning::Custom prometheus config does not exist') and message !contains "\\"filepath\\":\\"/"
| project timestamp, controllertype=tostring(customDimensions.controllertype), os, message
| order by timestamp`,
        },
        {
            name: "OtelCollector Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where message contains '/opt/microsoft/otelcollector/collector-log.txt'
| mv-expand message = split(message, "\\n") to typeof(string) 
| extend json=parse_json(message)
| where isnotnull(json.filepath)
| project timestamp, controllertype=tostring(customDimensions.controllertype), msg=tostring(json.msg), err=tostring(json.err), component=strcat(json.name, " ", json.kind), caller=tostring(json.caller), stacktrace=tostring(json.stacktrace)
| summarize count() by timestamp, controllertype, msg, err, component, caller, stacktrace
| order by timestamp`,
        },
        {
            name: "MetricsExtension Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime and timestamp < _endTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message contains '\\"filepath\\":\\"/MetricsExtensionConsoleDebugLog.log\\"'
| mv-expand message = split(message, "\\n") to typeof(string) 
| extend json=parse_json(message)
| where isnotempty(json.message)
| project timestamp, controllertype=tostring(customDimensions.controllertype), level=json.level, message=json.message
| order by timestamp`,
        },
        {
            name: "MDSD Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where message contains '"filepath":"/opt/microsoft/linuxmonagent/mdsd.err"'
| mv-expand message = split(message, "\\n") to typeof(string) 
| extend json=parse_json(message)
| where isnotnull(json.log)
| project timestamp, controllertype=tostring(customDimensions.controllertype), log=json.log
| order by timestamp`,
        },
        {
            name: "AddonTokenAdapter Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where customDimensions.tag == 'prometheus.log.addontokenadapter'
| order by timestamp`,
        },
        {
            name: "TargetAllocator Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where customDimensions.tag contains "prometheus.log.targetallocator.tacontainer"
| where message contains "error"
| order by timestamp`,
        },
        {
            name: "ConfigReader errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where customDimensions.tag == "prometheus.log.targetallocator.configreader"
| mv-expand message = split(message, "\\n") to typeof(string) 
| where message has "error" or message has "Exception"`,
        },
        {
            name: "DNS Resolution Issues",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend controllertype=tostring(customDimensions.controllertype)
| where message contains 'Temporary failure in name resolution' or message contains 'Error resolving address' or message contains 'Error resolving address'
| project timestamp, controllertype, message
| order by timestamp`,
        },
        {
            name: "Private Link Issues",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend os = tostring(customDimensions.osType)
| mv-expand message = split(message, "\\n") to typeof(string) 
| extend controllertype=tostring(customDimensions.controllertype)
| where message contains 'Data collection endpoint must be used to access configuration over private link.'
| project timestamp, controllertype, os, message
| order by timestamp`,
        },
        {
            name: "Private Link Issues by Nodepool, Node, and Pod",
            datasource: "PrometheusAppInsights",
            kql: `let restartingPods = traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend os = tostring(customDimensions.osType)
| mv-expand message = split(message, "\\n") to typeof(string)
| where message contains 'Data collection endpoint must be used to access configuration over private link.'
| extend controllertype=tostring(customDimensions.controllertype)
| extend node = tostring(customDimensions.computer)
| extend podname = tostring(customDimensions.podname)
| project timestamp, controllertype, os, node, podname, message
| summarize count() by controllertype, os, node, podname, bin(timestamp, totimespan(Interval))
| extend count= iff(count_ > 0, 1, 0)
| order by timestamp;
customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| extend controllertype=tostring(customDimensions.controllertype)
| extend node = tostring(customDimensions.computer)
| extend podname = tostring(customDimensions.podname)
| summarize count() by controllertype, node, podname, bin(timestamp, totimespan(Interval))
| join kind=leftouter restartingPods on node, podname, controllertype, timestamp
| extend issue=iff(controllertype1 != "", 1, 0)
| extend nodepool=trim(@"-vmss[\\d\\w]+", node)
//| where issue == 0
//| where nodepool contains 'system'
//| summarize count() by issue, nodepool, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Liveness Probe Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend controllertype=tostring(customDimensions.controllertype)
| extend podname=tostring(customDimensions.podname)
| mv-expand message = split(message, "\\n") to typeof(string) 
| where message contains 'Health check failed'
| project timestamp, controllertype, podname, message
| summarize count() by timestamp, controllertype, podname, message
| order by timestamp`,
        },
    ],
    config: [
        {
            name: "Invalid Custom Prometheus Config",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| extend invalidPromConfig=tobool(customDimensions.InvalidCustomPrometheusConfig)
| extend apiserver=tostring(customDimensions.ApiServerKeepListRegex)
| extend cadvisor=tostring(customDimensions.CAdvisorKeepListRegex)
| extend coredns=tostring(customDimensions.CoreDNSKeepListRegex)
| extend kappie=tostring(customDimensions.KappieBasicKeepListRegex)
| extend kubeproxy=tostring(customDimensions.KappieBasicKeepListRegex)
| project invalidPromConfig`,
        },
        {
            name: "ReplicaSet Scrape Configs Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend apiServer=tobool(customDimensions.defaultscrapeapiserver)
| extend collectorHealth=tobool(customDimensions.defaultscrapecollectorhealth)
| extend coreDns=tobool(customDimensions.defaultscrapecoreDns)
| extend kubeProxy=tobool(customDimensions.defaultscrapekubeproxy)
| extend kubeState=tobool(customDimensions.defaultscrapekubestate)
| extend podAnnotations=tobool(customDimensions.defaultscrapepodannotations)
| extend Values = pack(
                        "Kube-State-Metrics", kubeState,
                        "Kube Proxy", kubeProxy,
                        "API Server", apiServer,
                        "Core DNS", coreDns,
                        "Prometheus Collector Health", collectorHealth,
                        "Pod Annotations", podAnnotations
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "Linux DaemonSet Scrape Configs Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend debugModeEnabled=tobool(customDimensions.debugmodeenabled)
| extend hpaEnabled=tobool(customDimensions.collectorHpaEnabled)
| extend apiServer=tobool(customDimensions.defaultscrapeapiserver)
| extend cadvisor=tobool(customDimensions.defaultscrapecadvisor)
| extend collectorHealth=tobool(customDimensions.defaultscrapecollectorhealth)
| extend coreDns=tobool(customDimensions.defaultscrapecoreDns)
| extend kapppieBasic=tobool(customDimensions.defaultscrapekappiebasic)
| extend kubelet=tobool(customDimensions.defaultscrapekubelet)
| extend kubeProxy=tobool(customDimensions.defaultscrapekubeproxy)
| extend kubeState=tobool(customDimensions.defaultscrapekubestate)
| extend nodeExporter=tobool(customDimensions.defaultscrapenodeexporter)
| extend podAnnotations=tobool(customDimensions.defaultscrapepodannotations)
| extend windowsExporter=tobool(customDimensions.defaultscrapewindowsexporter)
| extend windowsKubeProxy=tobool(customDimensions.defaultscrapewindowskubeproxy)
| extend httpproxyenabled=tobool(customDimensions.httpproxyenabled)
| extend defaultscrapenetworkobservabilityRetina = tobool(customDimensions.defaultscrapenetworkobservabilityRetina)
| extend defaultscrapenetworkobservabilityHubble = tobool(customDimensions.defaultscrapenetworkobservabilityHubble)
| extend defaultscrapenetworkobservabilityCilium = tobool(customDimensions.defaultscrapenetworkobservabilityCilium)
| extend Values = pack(
                        "cAdvisor", cadvisor,
                        "Kubelet", kubelet,
                        "Node Exporter", nodeExporter,
                        "Kappie Basic", kapppieBasic,
                        "Retina", defaultscrapenetworkobservabilityRetina,
                        "Hubble", defaultscrapenetworkobservabilityHubble,
                        "Cilium", defaultscrapenetworkobservabilityCilium

                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "Windows DaemonSet Scrape Configs Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend windowsExporter=tobool(customDimensions.defaultscrapewindowsexporter)
| extend windowsKubeProxy=tobool(customDimensions.defaultscrapewindowskubeproxy)
| extend Values = pack(
                        "Windows Exporter", windowsExporter,
                        "Windows KubeProxy", windowsKubeProxy
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "Horizontal Pod Auto-Scaling (HPA) Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend hpaEnabled=tobool(customDimensions.collectorHpaEnabled)
| extend Values = pack(
                        "hpa", hpaEnabled
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "Debug Mode Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend debugModeEnabled=tobool(customDimensions.debugmodeenabled)
| extend Values = pack("debugMode", debugModeEnabled
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "HTTP Proxy Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend httpproxyenabled=tobool(customDimensions.httpproxyenabled)
| extend Values = pack(
                        "HTTP Proxy", httpproxyenabled
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "ReplicaSet ConfigMap Jobs",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend type = iff(job_name startswith "serviceMonitor", '"ServiceMonitor"', iff(job_name startswith "podMonitor", '"PodMonitor"', '"Configmap"'))
| where type == '"Configmap"'
| distinct job_name`,
        },
        {
            name: "Custom Config Validation Status",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "prom-config-validator"
| where message has_any ("validation failed", "No custom prometheus config", "No custom config via configmap", "validation succeeded", "Config file provided")
| extend controllertype=tostring(customDimensions.controllertype)
| extend podname=tostring(customDimensions.podname)
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend status = case(
    cleaned has "custom config validation failed", "INVALID - custom config rejected",
    cleaned has "No custom prometheus config found", "NO_CUSTOM_CONFIG - using defaults only",
    cleaned has "No custom config via configmap", "NO_CONFIG - no custom or default configs",
    cleaned has "default scrape config validation failed", "FATAL - even defaults failed",
    cleaned has "default scrape config validation succeeded", "FALLBACK - custom rejected, using defaults",
    cleaned has "Config file provided - /opt/promMergedConfig.yml", "OK - merged config loaded",
    cleaned has "Config file provided - /opt/defaultsMergedConfig.yml", "OK - defaults config loaded",
    "UNKNOWN")
| summarize LastSeen=max(timestamp), Count=count() by status, controllertype, podname
| order by controllertype asc, status asc`,
        },
        {
            name: "Custom Config Validation Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "prom-config-validator" and message has "validation failed"
| extend controllertype=tostring(customDimensions.controllertype)
| extend podname=tostring(customDimensions.podname)
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend failIdx = indexof(cleaned, "Prometheus custom config validation failed")
| extend preContext = substring(cleaned, max_of(failIdx - 2000, 0), 2000)
| extend errorDetail = extract(@"(?:failed to unmarshal yaml to prometheus config object: |unsupported features: |Generating otel config failed: |no space left on device|Invalid configuration: )(.{0,300})", 0, preContext)
| extend errorDetail = iff(isempty(errorDetail), extract(@"(Cannot load configuration:.*)", 1, preContext), errorDetail)
| where isnotempty(errorDetail)
| summarize LastSeen=max(timestamp), Count=count() by errorDetail=substring(errorDetail, 0, 300), controllertype
| order by Count desc`,
        },
        {
            name: "Custom Config YAML Error Lines",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "prom-config-validator" and message has "validation failed"
| where message has "unmarshal"
| extend controllertype=tostring(customDimensions.controllertype)
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend errorLines = extract_all(@"line (\\d+): (field \\S+ not found in type \\S+)", cleaned)
| mv-expand errorLines
| project timestamp, controllertype, lineNum = tostring(errorLines[0]), errorMsg = tostring(errorLines[1])
| distinct lineNum, errorMsg, controllertype
| order by toint(lineNum) asc`,
        },
        {
            name: "Custom Config OTel Loading Errors",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "Cannot load configuration"
| extend controllertype=tostring(customDimensions.controllertype)
| extend podname=tostring(customDimensions.podname)
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend idx = indexof(cleaned, "Cannot load configuration")
| extend errorBlock = substring(cleaned, idx, 500)
| summarize LastSeen=max(timestamp), Count=count() by errorBlock=substring(errorBlock, 0, 500), controllertype
| order by Count desc`,
        },
        {
            name: "Custom Scrape Jobs from Startup Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > _startTime
| where tostring(customDimensions.cluster) =~ _cluster
| where message has "label limits in custom scrape config for job"
| extend cleaned = replace_regex(message, @"\\x1b\\[[0-9;]*m", "")
| extend jobs = extract_all(@"label limits in custom scrape config for job ([a-zA-Z0-9_\\-]+)", cleaned)
| mv-expand job = jobs to typeof(string)
| extend controllertype=tostring(customDimensions.controllertype)
| extend podname=tostring(customDimensions.podname)
| summarize LastSeen=max(timestamp), PodCount=dcount(podname) by job, controllertype
| order by controllertype asc, job asc`,
        },
        {
            name: "ReplicaSet PodMonitors",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend type = iff(job_name startswith "serviceMonitor", '"ServiceMonitor"', iff(job_name startswith "podMonitor", '"PodMonitor"', '"Configmap"'))
| where type == '"PodMonitor"'
| distinct job_name`,
        },
        {
            name: "ReplicaSet ServiceMonitors",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend type = iff(job_name startswith "serviceMonitor", '"ServiceMonitor"', iff(job_name startswith "podMonitor", '"PodMonitor"', '"Configmap"'))
| where type == '"ServiceMonitor"'
| distinct job_name`,
        },
        {
            name: "Default Targets KeepListRegex",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| order by timestamp
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| take 1
| extend invalidPromConfig=tobool(customDimensions.InvalidCustomPrometheusConfig)
| extend apiserver=tostring(customDimensions.ApiServerKeepListRegex)
| extend cadvisor=tostring(customDimensions.CAdvisorKeepListRegex)
| extend coredns=tostring(customDimensions.CoreDNSKeepListRegex)
| extend kappie=tostring(customDimensions.KappieBasicKeepListRegex)
| extend kubeproxy=tostring(customDimensions.KubeProxyKeepListRegex)
| extend kubestate=tostring(customDimensions.KubeStateKeepListRegex)
| extend kubelet=tostring(customDimensions.KubeletKeepListRegex)
| extend nodeexporter=tostring(customDimensions.NodeExporterKeepListRegex)
| extend windowsexporter=tostring(customDimensions.WinExporterKeepListRegex)
| extend windowskubeproxy=tostring(customDimensions.WinKubeProxyKeepListRegex)
| extend acstorcapacityprovisioner=tostring(customDimensions.AcstorCapacityProvisionerRegex)
| extend acstormetricsexporter=tostring(customDimensions.AcstorMetricsExporterRegex)
| extend NetworkObservabilityCiliumScrape=tostring(customDimensions.NetworkObservabilityCiliumScrapeRegex)
| extend NetworkObservabilityHubbleScrape=tostring(customDimensions.NetworkObservabilityHubbleScrapeRegex)
| extend NetworkObservabilityRetinaScrape=tostring(customDimensions.NetworkObservabilityRetinaScrapeRegex)
| extend Values = pack(
                        "API Server", apiserver,
                        "cAdvisor", cadvisor,
                        "Core DNS", coredns,
                        "Kappie Basic", kappie,
                        "Kube Proxy", kubeproxy,
                        "Kube-State-Metrics", kubestate,
                        "Kubelet", kubelet,
                        "Node Exporter", nodeexporter,
                        "Windows Exporter", windowsexporter,
                        "AcStor Capacity Provisioner", acstorcapacityprovisioner,
                        "AcStor Metrics Exporter", acstormetricsexporter,
                        "Network Observability Cilium", NetworkObservabilityCiliumScrape,
                        "Network Observability Hubbble", NetworkObservabilityHubbleScrape,
                        "Network Observability Retina", NetworkObservabilityRetinaScrape
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])

// | project kubestate
// | take 1`,
        },
        {
            name: "Default Targets Scrape Interval",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| order by timestamp
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| take 1
| extend apiserver=tostring(customDimensions.ApiServerScrapeInterval)
| extend cadvisor=tostring(customDimensions.CAdvisorScrapeInterval)
| extend coredns=tostring(customDimensions.CoreDNSScrapeInterval)
| extend kappie=tostring(customDimensions.KappieBasicScrapeInterval)
| extend kubeproxy=tostring(customDimensions.KubeProxyScrapeInterval)
| extend kubestate=tostring(customDimensions.KubeStateScrapeInterval)
| extend kubelet=tostring(customDimensions.KubeletScrapeInterval)
| extend nodeexporter=tostring(customDimensions.NodeExporterScrapeInterval)
| extend podannotations=tostring(customDimensions.PodAnnotationScrapeInterval)
| extend promhealth=tostring(customDimensions.PromHealthScrapeInterval)
| extend windowsexporter=tostring(customDimensions.WinExporterScrapeInterval)
| extend windowskubeproxy=tostring(customDimensions.WinKubeProxyScrapeInterval)
| extend acstorcapacityprovisioner=tostring(customDimensions.AcstorCapacityProvisionerScrapeInterval)
| extend acstormetricsexporter=tostring(customDimensions.AcstorMetricsExporterScrapeInterval)
| extend NetworkObservabilityCiliumScrapeInterval=tostring(customDimensions.NetworkObservabilityCiliumScrapeInterval)
| extend NetworkObservabilityHubbleScrapeInterval=tostring(customDimensions.NetworkObservabilityHubbleScrapeInterval)
| extend NetworkObservabilityRetinaScrapeInterval=tostring(customDimensions.NetworkObservabilityRetinaScrapeInterval)
| extend Values = pack(
                        "API Server", apiserver,
                        "cAdvisor", cadvisor,
                        "Core DNS", coredns,
                        "Kappie Basic", kappie,
                        "Kube Proxy", kubeproxy,
                        "Kube-State-Metrics", kubestate,
                        "Kubelet", kubelet,
                        "Node Exporter", nodeexporter,
                        "Pod Annotations", podannotations,
                        "Windows Exporter", windowsexporter,
                        "AcStor Capacity Provisioner", acstorcapacityprovisioner,
                        "AcStor Metrics Exporter", acstormetricsexporter,
                        "Network Observability Cilium", NetworkObservabilityCiliumScrapeInterval,
                        "Network Observability Hubbble", NetworkObservabilityHubbleScrapeInterval,
                        "Network Observability Retina", NetworkObservabilityRetinaScrapeInterval
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])`,
        },
        {
            name: "Minimal Ingestion Profile Enabled",
            datasource: "AKS CCP",
            kql: `AMAMetricsConfigmapWatcher
| where PreciseTimeStamp > _startTime and PreciseTimeStamp < _endTime
| where ccpNamespace == AKSClusterID
| where configmap != "na"
| project PreciseTimeStamp, file, msg, configmap
| order by PreciseTimeStamp
| top 1 by PreciseTimeStamp
| project configmap
| mv-apply e = extract_all(@"([\\w-]+) = (\\w*)", dynamic([1,2]), tostring(configmap["default-targets-metrics-keep-list"]))
on (
    extend name = tostring(e[0])
    | where name == "minimalingestionprofile"
    | extend value = tostring(e[1])
)`,
        },
        {
            name: "OTLP Metrics Enabled",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend otlpenabled=tobool(customDimensions.otlpenabled)
| extend Values = pack(
                        "otlpenabled", otlpenabled
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tobool(Values[1])`,
        },
        {
            name: "Cluster Alias",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend calias=tostring(customDimensions.calias)
| extend Values = pack(
                        "calias", calias
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])`,
        },
        {
            name: "Cluster Label",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend clabel=tostring(customDimensions.clabel)
| extend Values = pack(
                        "clabel", clabel
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])`,
        },
        {
            name: "ConfigMap Version",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend settingsconfigschemaversion=tostring(customDimensions.settingsconfigschemaversion)
| extend Values = pack(
                        "settingsconfigschemaversion", settingsconfigschemaversion
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])`,
        },
        {
            name: "Pod Annotations Namespace Regex",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| order by timestamp
| take 1
| extend podannotationns=tostring(customDimensions.podannotationns)
| extend Values = pack(
                        "Pod Annotations Namespace Regex", podannotationns
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])`,
        },
        {
            name: "ReplicaSet Targets Discovered (Pre-Filtering) per Job",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend type = iff(job_name startswith "serviceMonitor", '"ServiceMonitor"', iff(job_name startswith "podMonitor", '"PodMonitor"', '"Configmap"'))
//| where type == '"Configmap"'
| summarize value=avg(value) by bin(timestamp, totimespan(Interval)), job_name`,
        },
        {
            name: "Kube-State-Metrics Labels Allow List",
            datasource: "AKS",
            kql: `ManagedClusterSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| top 1 by PreciseTimeStamp desc 
| project azureMonitorProfile
| project tostring(azureMonitorProfile.metrics.kubeStateMetrics.metricLabelsAllowlist)
//| project tostring(azureMonitorProfile.metrics.kubeStateMetrics.metricAnnotationsAllowList)`,
        },
        {
            name: "Kube-State-Metrics Annotations Allow List",
            datasource: "AKS",
            kql: `ManagedClusterSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| top 1 by PreciseTimeStamp desc 
| project azureMonitorProfile
//| project tostring(azureMonitorProfile.metrics.kubeStateMetrics.metricLabelsAllowlist)
| project tostring(azureMonitorProfile.metrics.kubeStateMetrics.metricAnnotationsAllowList)`,
        },
        {
            name: "Recording Rules Configured",
            datasource: "AMWInfo",
            kql: `AzureMonitorMetricsDCRDaily
| where (Timestamp > ago(7d)) or (Timestamp >= _startTime and Timestamp <= _endTime)
| where ParentResourceId =~ _cluster
| extend AMWAccountResourceId=AzureMonitorWorkspaceResourceId
| distinct AMWAccountResourceId
| join kind=innerunique AzureMonitorWorkspaceStatsDaily on AMWAccountResourceId
| project AMWAccountResourceId, MDMAccountName
| extend hasRecordingRules = "Check Azure Portal → AMW → Prometheus Rule Groups"
| project AMWAccountResourceId, MDMAccountName, hasRecordingRules`,
        },
        {
            name: "Addon Enabled in AKS Profile",
            datasource: "AKS",
            kql: `ManagedClusterSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| top 1 by PreciseTimeStamp desc
| extend metricsEnabled = isnotempty(azureMonitorProfile) and tobool(azureMonitorProfile.metrics.enabled)
| extend containerInsightsEnabled = isnotempty(addonProfiles.omsagent) and tobool(addonProfiles.omsagent.enabled)
| project metricsEnabled, containerInsightsEnabled, azureMonitorProfile`,
        },
    ],
    workload: [
        {
            name: "Replica count",
            datasource: "PrometheusAppInsights",
            kql: `let query=customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend podname=tostring(customDimensions.podname)
| summarize replica_pod_count=dcount(podname) by bin(timestamp, totimespan("5m"));
query
| summarize max(replica_pod_count) by bin(timestamp, totimespan(Interval))
| union (query | summarize min(replica_pod_count) by bin(timestamp, totimespan(Interval)))`,
        },
        {
            name: "Max Replica Count",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend podname=tostring(customDimensions.podname)
| summarize replica_pod_count=dcount(podname) by bin(timestamp, totimespan(5m))
| summarize max(replica_pod_count)`,
        },
        {
            name: "DaemonSet Pods Count",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "DaemonSet"
|extend podname=tostring(customDimensions.podname)
| summarize pod_count=dcount(podname) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Samples per Minute",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
//| where tostring(customDimensions.controllertype) == "DaemonSet"
//| where tostring(customDimensions.osType) == "windows"
| extend podname=tostring(customDimensions.podname)
| where podname contains 'win'
| where name == "meMetricsProcessedCount" or name == "meMetricsReceivedCount"
| summarize value=sum(value) by bin(timestamp, totimespan(Interval)), name, podname`,
        },
        {
            name: "Total ReplicaSet Samples per Minute",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount" or name == "meMetricsReceivedCount"
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by name, pod, bin(timestamp, totimespan(Interval))
| summarize value=sum(value) by name, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Max Replica Samples Dropped",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsDroppedCount"
| extend aggregatedSamplesDropped = toint(customDimensions.aggregatedMetricsDropped)
| extend currentQueueSize = toint(customDimensions.currentQueueSize)
| extend pod=tostring(customDimensions.podname)
| project timestamp, pod, samplesDropped=value, aggregatedSamplesDropped, currentQueueSize
| summarize max(samplesDropped) by pod`,
        },
        {
            name: "Replicaset Samples Dropped by ME",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsDroppedCount"
| extend aggregatedSamplesDropped = toint(customDimensions.aggregatedMetricsDropped)
| extend currentQueueSize = toint(customDimensions.currentQueueSize)
| extend pod=tostring(customDimensions.podname)
| project timestamp, pod, samplesDropped=value, aggregatedSamplesDropped, currentQueueSize
| summarize argmax(samplesDropped, aggregatedSamplesDropped, currentQueueSize) by pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "DaemonSet Samples Dropped",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
| where name == "meMetricsDroppedCount"
| extend aggregatedSamplesDropped = toint(customDimensions.aggregatedMetricsDropped)
| extend currentQueueSize = toint(customDimensions.currentQueueSize)
| extend pod=tostring(customDimensions.podname)
| project timestamp, pod, samplesDropped=value, aggregatedSamplesDropped, currentQueueSize
| summarize argmax(samplesDropped, aggregatedSamplesDropped, currentQueueSize) by pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "P95 CPU (millicores)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "ksmUsage"
| extend value = value / 1000000000 * 1000
| extend pod=tostring(customDimensions.PodRefName)
| summarize value=round(percentile(value, 95), 2) by pod, name, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "P95 Memory (MB)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "ksmUsage"
| extend memory = toint(customDimensions.MemKsmRssBytes)
| extend value = memory / 1000000
| extend pod=tostring(customDimensions.PodRefName)
| summarize value=round(percentile(value, 95), 2) by bin(timestamp, totimespan(Interval)), pod`,
        },
        {
            name: "Replicaset ME Queue Size",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsDroppedCount"
| extend aggregatedSamplesDropped = toint(customDimensions.aggregatedMetricsDropped)
| extend currentQueueSize = toint(customDimensions.currentQueueSize)
| extend pod=tostring(customDimensions.podname)
| project timestamp, pod, currentQueueSize
| summarize max(currentQueueSize) by pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "ReplicaSet OtelCollector Queue Size",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
//| distinct name
| where name == "prometheus_otelcol_exporter_queue_size"
| summarize round(sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "DaemonSet OtelCollector Queue Size",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
//| distinct name
| where name == "prometheus_otelcol_exporter_queue_size"
| summarize round(sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "ReplicaSet OtelCollector Export to ME Failed",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
//| distinct name
| where name == "prometheus_otelcol_exporter_send_failed_metric_points"
| summarize round(sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "DaemonSet OtelCollector Export to ME Failed",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
//| distinct name
| where name == "prometheus_otelcol_exporter_send_failed_metric_points"
| summarize round(sum=sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "ReplicaSet OtelCollector Receiver Metrics Refused",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
//| distinct name
| where name == "prometheus_otelcol_receiver_refused_metric_points"
| summarize round(sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "DaemonSet OtelCollector Receiver Metrics Refused",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
//| distinct name
| where name == "prometheus_otelcol_receiver_refused_metric_points"
| summarize round(sum=sum(value)) by bin(timestamp, totimespan(Interval)), pod=tostring(customDimensions.podname)`,
        },
        {
            name: "Number of Collectors Discovered",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_collectors_discovered"
| summarize round(number_of_collectors=avg(value)) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Number of Scrape Jobs",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend podname=tostring(customDimensions.podname)
| extend type = iff(job_name startswith "serviceMonitor", '"ServiceMonitor"', iff(job_name startswith "podMonitor", '"PodMonitor"', '"Configmap"'))
| summarize count=dcount(job_name) by  bin(timestamp, totimespan(Interval)), type`,
        },
        {
            name: "Targets Per Replica Pod",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
//| distinct name
| where name == "target_allocator_opentelemetry_allocator_targets_per_collector"
| summarize round(targets=avg(value)) by bin(timestamp, totimespan(Interval)), collector=tostring(customDimensions.collector_name)`,
        },
        {
            name: "Unassigned Targets",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
// distinct name
| where name == "target_allocator_opentelemetry_allocator_targets_unassigned"
| summarize round(targets=avg(value)) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Total prometheus_sd_http_failures",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "prometheus_prometheus_sd_http_failures_total"
| summarize total_sd_http_failures=sum(value) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Prometheus Receiver ---> Target Allocator Error Count",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where message contains '/opt/microsoft/otelcollector/collector-log.txt'
| mv-expand message = split(message, "\\n") to typeof(string) 
| extend json=parse_json(message)
| where isnotnull(json.filepath)
| project timestamp, controllertype=tostring(customDimensions.controllertype), msg=tostring(json.msg), err=json.err, component=strcat(json.name, " ", json.kind), caller=json.caller, stacktrace=json.stacktrace
| where msg contains 'Failed to retrieve job list' or msg contains 'Unable to refresh target groups' or msg contains 'Get "http://ama-metrics-'
| extend Path = iff(msg contains "Failed to retrieve job list", '"/jobs"', iff(msg contains "Unable to refresh target groups", '"/targets"', '"/scrape_configs"'))
| make-series ErrorCount=count() default=0 on timestamp in range(ago(_endTime - _startTime), now(), totimespan(Interval)) by Path
//| mvexpand timestamp, ErrorCount
//| extend timestamp=todatetime(timestamp), ErrorCount=toint(ErrorCount)
//| summarize ErrorCount=count() by Reason, controllertype, bin(timestamp, 5m)`,
        },
        {
            name: "Kube-state-metrics Version",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "otelcollector_cpu_usage_095"
| extend kubestateversion=tostring(customDimensions.kubestateversion)
| order by timestamp
| take 1`,
        },
        {
            name: "ReplicaSet Samples per Account per Replica",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| extend mdmAccount = tostring(customDimensions.metricsAccountName)
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by mdmAccount, pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "ReplicaSet Samples per Account",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| extend mdmAccount = tostring(customDimensions.metricsAccountName)
//| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by mdmAccount, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "DaemonSet Samples per Account per Pod",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
| where name == "meMetricsProcessedCount"
| extend mdmAccount = tostring(customDimensions.metricsAccountName)
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by mdmAccount, pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "ReplicaSet Samples per Minute per Replica",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsReceivedCount"
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by pod`,
        },
        {
            name: "OpenTelemetryCollector P95 CPU % cores",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend pod=tostring(customDimensions.podname)
| summarize value=percentile(value, 100) by pod, bin(timestamp, totimespan(Interval))

// customMetrics
// | where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
// | where timestamp >= _startTime
// | where timestamp < _endTime
// | where tolower(customDimensions.macmode) == "true"
// | extend cluster=tostring(customDimensions.cluster)
// |extend controllertype=tostring(customDimensions.controllertype)
// | extend    AKSregion=tostring(customDimensions.Region)
// | where tolower(controllertype) == "replicaset"
// | summarize x=percentile(value,100) by bin(timestamp, 1h), AKSregion, name,cluster
// |summarize y=sum(x) by bin(timestamp,1h), AKSregion, cluster
// |summarize CPU=percentile(y,95) by bin(timestamp,1h), AKSregion`,
        },
        {
            name: "OpenTelemetryCollector P95 Memory GB",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_memory_rss_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, totimespan(Interval))
`,
        },
        {
            name: "MetricsExtension P95 CPU % cores",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, name, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "MetricsExtension P95 Memory GB",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension_memory_rss_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, totimespan(Interval))
`,
        },
        {
            name: "OpenTelemetryCollector P95 CPU mc",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "win_proc_Percent_Processor_Time_095"
| extend instance=tostring(customDimensions.instance)
| where instance contains 'otelcollector'
| project timestamp, value, name, podname=tostring(customDimensions.podname), instance=tostring(customDimensions.instance)
| summarize value=round(percentile(value, 100), 2) by bin(timestamp, totimespan(Interval)), podname, instance
| join kind=fullouter (
customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "DaemonSet"
| extend os=tostring(customDimensions.osType)
| where os == "windows"
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, totimespan(Interval)), os
) on timestamp
| extend timestamp = iff(timestamp != "", timestamp, timestamp1)
| extend podname = iff(podname != "", podname, pod)
| extend value = iff(isnotnull(value), value, value1)`,
        },
        {
            name: "MetricsExtension P95 CPU mc",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "win_proc_Percent_Processor_Time_095"
| extend instance=tostring(customDimensions.instance)
| where instance contains 'MetricsExtension'
| project timestamp, value, name, podname=tostring(customDimensions.podname), instance=tostring(customDimensions.instance)
| summarize value=round(percentile(value, 100), 2) by bin(timestamp, totimespan(Interval)), podname, instance
| join kind=fullouter (
customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "metricsextension.native_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "DaemonSet"
| extend os=tostring(customDimensions.osType)
| where os == "windows"
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, totimespan(Interval)), os
) on timestamp
| extend timestamp = iff(timestamp != "", timestamp, timestamp1)
| extend podname = iff(podname != "", podname, pod)
| extend value = iff(isnotnull(value), value, value1)`,
        },
        {
            name: "P95 CPU Config Reloader (millicores)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "cnfgRdrCPUUsage"
//| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000 * 1000
| project timestamp, value, name
| extend interval=(_endTime - _startTime) / 4
| summarize value=round(percentile(value, 95), 2) by bin(timestamp, totimespan(Interval)), name`,
        },
        {
            name: "P95 CPU Target Allocator (millicores)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "taCPUUsage"
//| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000 * 1000
| project timestamp, value, name
| extend interval=(_endTime - _startTime) / 4
| summarize value=round(percentile(value, 95), 2) by bin(timestamp, totimespan(Interval)), name`,
        },
        {
            name: "P95 Mem Config Reloader (MB)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "cnfgRdrCPUUsage"
| extend memory = toint(customDimensions.cnfgRdrMemRssBytes)
//| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = memory / 1000000
| project timestamp, value
| extend interval=(_endTime - _startTime) / 4
| summarize value=round(percentile(value, 95), 2) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "P95 Mem Target Allocator (MB)",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "taCPUUsage"
| extend memory = toint(customDimensions.taMemRssBytes)
//| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = memory / 1000000
| project timestamp, value
| extend interval=(_endTime - _startTime) / 4
| summarize value=round(percentile(value, 95), 2) by bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "DaemonSet Samples per Minute per Pod",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where tostring(customDimensions.controllertype) == "DaemonSet"
| where tostring(customDimensions.osType) == "linux"
| where name == "meMetricsProcessedCount" or name == "meMetricsReceivedCount"
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by name, pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Total P95 CPU per Replica",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_cpu_usage_095" or name == "metricsextension_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, name, bin(timestamp, totimespan(Interval))
| summarize value=sum(value) by pod, bin(timestamp, totimespan(Interval))`,
        },
        {
            name: "Total P95 Memory GB per Replica",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "otelcollector_memory_rss_095" or name == "metricsextension_memory_rss_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, totimespan(Interval))
| summarize value=sum(value) by pod, bin(timestamp, totimespan(Interval))
`,
        },
        {
            name: "Node Pools",
            datasource: "AKS",
            kql: `set best_effort=true;
AgentPoolSnapshot
| where PreciseTimeStamp between (_startTime .. _endTime)
| where cluster_id == AKSClusterID
| summarize arg_max(PreciseTimeStamp, *) by tolower(id)
| extend osSKUSanitised= iff(isnotempty(osSku) and osSku != 'na', osSku, osType)
| extend enableAutoScaling = coalesce(enableAutoScaling, false)
| extend max_pods = tostring(kubernetesConfig.kubeletConfig['--max-pods'])
| extend availabilityProfile = extractjson('$.availabilityProfile', log)
| extend kubeReserved = tostring(kubernetesConfig.kubeletConfig["--kube-reserved"])
| extend osSku = osSKUSanitised
| extend nodeImageRefe = tostring(agentPoolVersionProfile.nodeImageReference.id)
| extend imageDefinitionName = split(nodeImageRefe, "/")[10]
| extend imageVersion = tostring(split(nodeImageRefe, '/')[-1])
| extend manual = parse_json(log).virtualMachinesProfile.scale.manual
| mv-apply manual on (
    summarize manual_count = sum(toint(manual.count)), manual_vmsizes = make_list(manual.vm_sizes)
)
| extend size = max_of(size, manual_count)
| extend vmSize = iff(isnotempty(vmSize), vmSize, manual_vmsizes)
| extend vmssName = tostring(split(vmssID, "/")[-1])
| project PreciseTimeStamp, name, namespace, clusterName, vmssName, mode, enableAutoScaling, isAutoscalingCluster, size, provisioningState, 
    osType, vmSize, storageProfile, distro, max_pods, orchestratorVersion, availabilityProfile, log, maxCount, minCount, kubeReserved,
    osDiskSizeGB, osSku, imageDefinitionName, imageVersion, configurationVersion, nodeImageRefe, latestOperationId, scaleDownMode, availabilityZones,
    kubeletDiskType, scaleSetPriority, createdTime
| extend zones = array_strcat(availabilityZones, ', ')
| extend disk = strcat(storageProfile, ' (', osDiskSizeGB, ' GB)')
| extend level = iff(provisioningState =~ "failed", "error", '')
| extend scaleSetPriority = iff(scaleSetPriority == 'na', '', scaleSetPriority)
| order by mode asc
| project name, vmssName, mode, osType, vmSize, size, distro, provisioningState, createdTime`,
        },
        {
            name: "System Nodepool Nodes Status",
            datasource: "AKS CCP",
            kql: `let InjectBase10_Temp = (T:(*)) {
    let hextra_length = 6;
    let charList = "0123456789abcdefghijklmnopqrstuvwxyz";
    T
    | extend base36 = column_ifexists('base36', '')
    | extend profile = column_ifexists('availabilityProfile', '')
    | extend hexatridecimal = iff(profile =~ 'AvailabilitySet', 'n/a', substring(base36, strlen(base36) - hextra_length, strlen(base36)))
    | extend parts = split(base36, '-')
    | extend ss_name = case(
        // Linux
        base36 contains "vmss", tostring(substring(base36, 0, indexof(base36, 'vmss') + 4)),
        // Availability Set
        profile =~ 'AvailabilitySet', strcat(parts[0], "-", parts[1], "-", parts[2]),
        // Windows
        substring(base36, 0, strlen(base36) - hextra_length)
    )
    | extend reversed = reverse(hexatridecimal)
    | extend power_0 = toint(indexof(charList, substring(reversed, 0, 1))) * pow(36, 0)
    | extend power_1 = toint(indexof(charList, substring(reversed, 1, 1))) * pow(36, 1)
    | extend power_2 = toint(indexof(charList, substring(reversed, 2, 1))) * pow(36, 2)
    | extend power_3 = toint(indexof(charList, substring(reversed, 3, 1))) * pow(36, 3)
    | extend power_4 = toint(indexof(charList, substring(reversed, 4, 1))) * pow(36, 4)
    | extend power_5 = toint(indexof(charList, substring(reversed, 5, 1))) * pow(36, 5)
    | extend sum_of_powers = toint(power_0 + power_1 + power_2 + power_3 + power_4 + power_5)
    | extend base10 = case(
        profile =~ 'AvailabilitySet', base36,
        profile =~ 'VirtualMachines', base36,
        isnotempty(hexatridecimal), strcat(ss_name, '_', sum_of_powers),
        ''
    )
    | project-away reversed, power_0, power_1, power_2, power_3, power_4, power_5, sum_of_powers, parts, profile
};
let queryNodePoolSet = tostring(_SystemNodepool);
let querySubscriptionId = _AKSClusterSub;
let queryManagedResourceGroupName = _AKSClusterNodeRG;
KubeAudit
| where PreciseTimeStamp between(_startTime .. _endTime) 
| where cluster_id == AKSClusterID and objectRef.resource == 'nodes'
| where verb in ('patch', 'update') and level !in ('Metadata')
| extend node = tostring(objectRef.name)
| summarize hint.num_partitions = 24 hint.strategy=shuffle hint.shufflekey=node
    PreciseTimeStamp = max(PreciseTimeStamp),
    take_any(cluster_id, objectRef, requestObject, responseStatus),
    take_anyif(responseObject, responseObject != 'na')
    by node
| mv-apply condition = coalesce(responseObject.status.conditions, dynamic([{"type":"Ready","status":"Unknown"}])) on 
(
    where condition.type == "Ready" | project status = tostring(condition.status)
)
| project 
    PreciseTimeStamp,
    ccpNamespace = cluster_id,
    name=tostring(objectRef.name),
    status=tostring(iff(status == "True", "Ready", "NotReady")),
    roles=tostring(responseObject.metadata.labels.['kubernetes.azure.com/role']),
    age=tostring(PreciseTimeStamp-responseObject.metadata.creationTimestamp),
    version=tostring(responseObject.status.nodeInfo.kubeletVersion),
    os_image=tostring(responseObject.status.nodeInfo.osImage),
    kernel_version=tostring(responseObject.status.nodeInfo.kernelVersion),
    container_runtime_version=tostring(responseObject.status.nodeInfo.containerRuntimeVersion),
    metadata=responseObject.metadata,
    response_spec=responseObject.spec,
    response_status=responseObject.status
| where metadata.labels.agentpool in (queryNodePoolSet) or name in (queryNodePoolSet)
| extend base36 = name
| invoke InjectBase10_Temp()
| extend instance = column_ifexists("base10", '')
| join kind=leftouter (
    cluster("azcrpbifollower.kusto.windows.net").database("bi_allprod").VMScaleSetVMInstance
    | where PreciseTimeStamp between(_startTime.._endTime)
        and SubscriptionId =~ querySubscriptionId 
        and tolower(ResourceGroupName) contains tolower(queryManagedResourceGroupName)
        and tolower(VMScaleSetName) has_any(_SystemNodepool)
    | extend instance = tolower(strcat(VMScaleSetName, '_', InstanceIdString ))
    | extend managedresourcegroupName = tolower(queryManagedResourceGroupName)
    | extend querynodepoolset = tostring(_SystemNodepool)
    | summarize arg_max(PreciseTimeStamp, VMScaleSetVMInstanceId) by instance
    | project instance, vm_id = VMScaleSetVMInstanceId
) on instance
| summarize
    Ready = countif(status == 'Ready'),
    NotReady = countif(status in ('NotReady', 'Unknown')),
    arg_max(PreciseTimeStamp, Current = status, instance, vm_id)
    by name
| project
    //Value = iif(Current == "Ready", 0, 100),
    Health = iif(Current == "Ready", "Healthy", "Degraded"),
    Node = name,
    //ColumnLabel = PreciseTimeStamp,
    Nodepool = instance
    //VMId = vm_id
`,
        },
        {
            name: "HPA Status",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has 'ama-metrics' and requestObject has 'HorizontalPodAutoscaler'
| extend ro = parse_json(requestObject)
| where ro.kind == "HorizontalPodAutoscaler"
| extend hpaName = tostring(ro.metadata.name)
| extend minReplicas = toint(ro.spec.minReplicas)
| extend maxReplicas = toint(ro.spec.maxReplicas)
| extend currentReplicas = toint(ro.status.currentReplicas)
| extend desiredReplicas = toint(ro.status.desiredReplicas)
| summarize arg_max(PreciseTimeStamp, *) by hpaName
| extend atLimit = currentReplicas >= maxReplicas
| project PreciseTimeStamp, hpaName, minReplicas, maxReplicas, currentReplicas, desiredReplicas, atLimit`,
        },
        {
            name: "HPA Scaling Metric and Oscillation",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where tostring(objectRef) has 'ama-metrics-hpa'
| where verb == 'update'
| extend reqObj = todynamic(requestObject)
| extend desiredReplicas = toint(reqObj.status.desiredReplicas), currentReplicas = toint(reqObj.status.currentReplicas)
| extend specMetrics = tostring(reqObj.spec.metrics), currentMetrics = tostring(reqObj.status.currentMetrics)
| where isnotempty(currentReplicas)
| summarize min_replicas=min(currentReplicas), max_replicas=max(currentReplicas), avg_replicas=round(avg(currentReplicas),1), samples=count() by bin(PreciseTimeStamp, 15m)
| order by PreciseTimeStamp asc`,
        },
        {
            name: "HPA Metric Configuration",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where tostring(objectRef) has 'ama-metrics-hpa'
| where verb == 'update'
| extend reqObj = todynamic(requestObject)
| extend specMetrics = tostring(reqObj.spec.metrics)
| where isnotempty(specMetrics)
| summarize arg_max(PreciseTimeStamp, specMetrics)
| project PreciseTimeStamp, specMetrics`,
        },
        {
            name: "Cluster Autoscaler Scale Decisions",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
ClusterAutoscaler
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where log has 'ScaleUp' or log has 'ScaleDown' or log has 'scale_up' or log has 'scale_down' or log has 'unschedulable' or log has 'Unschedulable' or log has 'removing node'
| where not(log has 'No unschedulable')
| where not(log has 'deletion timestamp')
| project PreciseTimeStamp, logSnippet=substring(log, 0, 300)
| order by PreciseTimeStamp desc
| take 50`,
        },
        {
            name: "Cluster Autoscaler No Unschedulable Count",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
ClusterAutoscaler
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where log has 'No unschedulable pods'
| summarize count_no_unschedulable=count(), first_seen=min(PreciseTimeStamp), last_seen=max(PreciseTimeStamp)`,
        },
        {
            name: "Pod Resource Limits",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has "ama-metrics" and requestObject has "limits"
| extend ro = parse_json(requestObject)
| where ro.kind == "Pod"
| mv-expand c = ro.spec.containers
| where tostring(c.name) == "prometheus-collector"
| extend memLimit = tostring(c.resources.limits.memory)
| extend cpuLimit = tostring(c.resources.limits.cpu)
| extend memReq = tostring(c.resources.requests.memory)
| extend cpuReq = tostring(c.resources.requests.cpu)
| extend podname = tostring(ro.metadata.name)
| extend controllertype = iff(podname contains "node", "DaemonSet", "ReplicaSet")
| summarize arg_max(PreciseTimeStamp, *) by controllertype
| project controllertype, cpuReq, cpuLimit, memReq, memLimit`,
        },
        {
            name: "Target Allocator Distribution",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name in ("target_allocator_opentelemetry_allocator_targets_per_collector", "target_allocator_opentelemetry_allocator_targets", "target_allocator_opentelemetry_allocator_collectors_allocatable")
| summarize avgVal=round(avg(value),1), maxVal=round(max(value),1), lastVal=round(arg_max(timestamp, value),1) by name, bin(timestamp, totimespan(Interval))
| project timestamp, metric=name, avgVal, maxVal
| order by timestamp desc, metric asc`,
        },
        {
            name: "Exporter Send Failures",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name in ("prometheus_otelcol_exporter_send_failed_metric_points", "prometheus_otelcol_receiver_refused_metric_points")
| extend controllertype = tostring(customDimensions.controllertype)
| summarize totalFailed=sum(value) by name, controllertype, bin(timestamp, totimespan(Interval))
| where totalFailed > 0
| project timestamp, metric=name, controllertype, totalFailed
| order by timestamp desc`,
        },
        {
            name: "ME Ingestion Success Rate",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name in ("meMetricsProcessedCount", "meMetricsSentToBlobCount", "meMetricsDroppedCount")
| extend controllertype = tostring(customDimensions.controllertype)
| summarize 
    processed=sumif(value, name == "meMetricsProcessedCount"),
    dropped=sumif(value, name == "meMetricsDroppedCount"),
    sentToBlob=sumif(value, name == "meMetricsSentToBlobCount")
    by controllertype, bin(timestamp, totimespan(Interval))
| extend successRate = iff(processed > 0, round(100.0 * (processed - dropped) / processed, 2), 100.0)
| project timestamp, controllertype, processed, dropped, sentToBlob, successRate
| order by timestamp desc`,
        },
        {
            name: "Event Timeline (Config, Restarts, Errors)",
            datasource: "PrometheusAppInsights",
            kql: `let configChanges = traces
| where timestamp > ago(_endTime - _startTime)
| where customDimensions.cluster =~ _cluster
| where message has "configmap" or message has "Config file provided" or message has "custom config" or message has "Settings configmap"
| where message !has "No custom prometheus config"
| summarize count() by bin(timestamp, totimespan(Interval))
| extend eventType = "ConfigChange", detail = strcat("config events: ", count_);
let restartEvents = customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "PodRestartCount"
| extend pod = tostring(customDimensions.podname)
| summarize restarts = max(value) by pod, bin(timestamp, totimespan(Interval))
| summarize totalRestarts = sum(restarts) by bin(timestamp, totimespan(Interval))
| where totalRestarts > 0
| extend eventType = "PodRestart", detail = strcat("restarts: ", totalRestarts);
let errorSpikes = traces
| where timestamp > ago(_endTime - _startTime)
| where customDimensions.cluster =~ _cluster
| where message has "error" or message has "Error" or message has "fail" or message has "WARN"
| summarize errorCount = count() by bin(timestamp, totimespan(Interval))
| where errorCount > 0
| extend eventType = "ErrorOrWarning", detail = strcat("errors/warnings: ", errorCount);
configChanges | union restartEvents | union errorSpikes
| project timestamp, eventType, detail
| order by timestamp asc`,
        },
        {
            name: "DaemonSet Per-Pod Sample Rate Variance",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where customDimensions.cluster =~ _cluster
| where customDimensions.controllerType == "DaemonSet"
| where name == "meMetricsProcessedCount"
| extend pod = tostring(customDimensions.podname)
| summarize avgSamplesPerMin = avg(value) by pod
| summarize minRate = min(avgSamplesPerMin), maxRate = max(avgSamplesPerMin), avgRate = avg(avgSamplesPerMin), podCount = count(), variance = round((max(avgSamplesPerMin) - min(avgSamplesPerMin)) / avg(avgSamplesPerMin) * 100, 1)
| extend highVariance = variance > 100`,
        },
        {
            name: "DaemonSet Per-Pod Sample Rate Distribution",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where customDimensions.cluster =~ _cluster
| where customDimensions.controllerType == "DaemonSet"
| where name == "meMetricsProcessedCount"
| extend pod = tostring(customDimensions.podname)
| summarize avgSamplesPerMin = round(avg(value), 0) by pod
| order by avgSamplesPerMin desc
| take 20`,
        },
        {
            name: "Scrape Samples Per Job Over Time",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "prometheus_scrape_samples_post_metric_relabeling"
| extend job = tostring(customDimensions.job)
| summarize avg_samples=round(avg(value), 0), max_samples=max(value) by bin(timestamp, totimespan(Interval)), job
| order by timestamp asc, job asc`,
        },
        {
            name: "ME Throughput by Pod Type Over Time",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "meMetricsProcessedCount"
| extend podname = tostring(customDimensions.podname)
| extend podtype = case(
    podname startswith "ama-metrics-win", "windows-ds",
    podname startswith "ama-metrics-node", "linux-ds",
    "replicaset")
| summarize total=sum(value) by bin(timestamp, totimespan(Interval)), podtype
| order by timestamp asc, podtype asc`,
        },
        {
            name: "Node Exporter Sample Count Trend",
            datasource: "PrometheusAppInsights",
            kql: `customMetrics
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where name == "prometheus_scrape_samples_post_metric_relabeling"
| extend job = tostring(customDimensions.job)
| where job == "node"
| where value > 0
| summarize count_nonzero=count(), avg_samples=round(avg(value), 0), max_samples=max(value), p50=round(percentile(value, 50), 0), p95=round(percentile(value, 95), 0) by bin(timestamp, totimespan(Interval))
| order by timestamp asc`,
        },
    ],
    pods: [
        {
            name: "Latest Pod Restarts",
            datasource: "AKS",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
cluster('akshuba.centralus').database('AKSccplogs').KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace and requestObject has 'terminated'
| mv-expand cs = requestObject.status.containerStatuses
| where cs.lastState.terminated.reason !in ('', 'Completed')
| extend pod = tostring(objectRef.name)
| where pod contains 'ama-metrics'
| project 
    PreciseTimeStamp, 
    container = tostring(cs.name),
    reason = tostring(cs.lastState.terminated.reason),
    exitCode = tostring(cs.lastState.terminated.exitCode),
    image = tostring(cs.image),
    containerID = tostring(cs.containerID),
    pod,
    ns = tostring(objectRef.namespace),
    restartCount = toint(cs.restartCount),
    startedAt = todatetime(cs.lastState.terminated.startedAt),
    finishedAt = todatetime(cs.lastState.terminated.finishedAt),
    message = tostring(cs.lastState.terminated.message),
    state = todynamic(cs.state),
    username = tostring(user.username),
    userAgent = tostring(userAgent)
| summarize PreciseTimeStamp = arg_max(PreciseTimeStamp, *) by pod, container
| project 
    PreciseTimeStamp, 
    pod,
    container,
    reason,
    message,
    restartCount,
    startedAt,
    finishedAt,
    exitCode,
    state`,
        },
        {
            name: "Pod Restarts During Interval",
            datasource: "AKS",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
cluster('akshuba.centralus').database('AKSccplogs').KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace and requestObject has 'terminated'
| mv-expand cs = requestObject.status.containerStatuses
| where cs.lastState.terminated.reason !in ('', 'Completed')
| extend pod = tostring(objectRef.name)
| where pod contains 'ama-metrics'
| project 
    PreciseTimeStamp, 
    container = tostring(cs.name),
    reason = tostring(cs.lastState.terminated.reason),
    exitCode = tostring(cs.lastState.terminated.exitCode),
    image = tostring(cs.image),
    containerID = tostring(cs.containerID),
    pod,
    ns = tostring(objectRef.namespace),
    restartCount = toint(cs.restartCount),
    startedAt = todatetime(cs.lastState.terminated.startedAt),
    finishedAt = todatetime(cs.lastState.terminated.finishedAt),
    message = tostring(cs.lastState.terminated.message),
    state = todynamic(cs.state),
    username = tostring(user.username),
    userAgent = tostring(userAgent)
//| summarize PreciseTimeStamp = arg_max(PreciseTimeStamp, *) by pod, container, bin(PreciseTimeStamp, totimespan(Interval))
| project 
    PreciseTimeStamp, 
    pod,
    container,
    reason,
    message,
    restartCount,
    startedAt,
    finishedAt,
    exitCode,
    state
| summarize count() by pod, container, finishedAt, reason, message, bin(PreciseTimeStamp, totimespan(Interval))`,
        },
        {
            name: "AKS Addon Pod Restart Count and Reason",
            datasource: "AKS",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
cluster('akshuba.centralus').database('AKSccplogs').KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace and requestObject has 'terminated'
| mv-expand cs = requestObject.status.containerStatuses
| where cs.lastState.terminated.reason !in ('', 'Completed')
| extend pod = tostring(objectRef.name)
| where pod contains 'ama-metrics'
| extend podName = iff(pod contains "ama-metrics-win-node", "ama-metrics-win-node",
    iff(pod contains "ama-metrics-node", "ama-metrics-node",
        iff(pod contains 'ama-metrics-operator-targets', "ama-metrics-operator-targets",
        "ama-metrics"
    )))
| project 
    PreciseTimeStamp, 
    container = tostring(cs.name),
    reason = tostring(cs.lastState.terminated.reason),
    exitCode = tostring(cs.lastState.terminated.exitCode),
    image = tostring(cs.image),
    containerID = tostring(cs.containerID),
    pod,
    podName,
    ns = tostring(objectRef.namespace),
    restartCount = toint(cs.restartCount),
    startedAt = todatetime(cs.lastState.terminated.startedAt),
    finishedAt = todatetime(cs.lastState.terminated.finishedAt),
    message = tostring(cs.lastState.terminated.message),
    state = todynamic(cs.state),
    username = tostring(user.username),
    userAgent = tostring(userAgent)
| where finishedAt > _startTime and finishedAt < _endTime
| project 
    PreciseTimeStamp, 
    pod,
    podName,
    container,
    reason,
    message,
    restartCount,
    startedAt,
    finishedAt,
    exitCode,
    state
| summarize count() by podName, container, reason, message, bin(finishedAt, totimespan(Interval))`,
        },
        {
            name: "Pod Restart Detail by Pod",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has "ama-metrics" or requestObject has "prometheus-collector"
| extend ro = parse_json(requestObject)
| where ro.kind == "Pod"
| mv-expand cs = ro.status.containerStatuses
| where tostring(cs.name) == "prometheus-collector"
| extend podName = tostring(ro.metadata.name)
| extend restartCount = toint(cs.restartCount)
| extend ready = tobool(cs.ready)
| extend reason = coalesce(tostring(cs.state.waiting.reason), tostring(cs.state.terminated.reason), tostring(cs.lastState.terminated.reason), "Running")
| extend exitCode = coalesce(toint(cs.state.terminated.exitCode), toint(cs.lastState.terminated.exitCode))
| extend controllertype = iff(podName contains "node", "DaemonSet", "ReplicaSet")
| summarize arg_max(PreciseTimeStamp, *) by podName
| project PreciseTimeStamp, podName, controllertype, restartCount, ready, reason, exitCode
| order by controllertype asc, restartCount desc`,
        },
        {
            name: "DaemonSet Pod Count by Status",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has "ama-metrics-node"
| extend ro = parse_json(requestObject)
| where ro.kind == "Pod"
| extend podName = tostring(ro.metadata.name)
| extend phase = tostring(ro.status.phase)
| summarize arg_max(PreciseTimeStamp, *) by podName
| summarize Running=countif(phase == "Running"), Pending=countif(phase == "Pending"), Failed=countif(phase == "Failed"), Succeeded=countif(phase == "Succeeded"), Unknown=countif(phase != "Running" and phase != "Pending" and phase != "Failed" and phase != "Succeeded"), Total=count()`,
        },
        {
            name: "Pod to Node Mapping",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has "ama-metrics"
| extend ro = parse_json(responseObject)
| where ro.kind == "Pod"
| extend podName = tostring(ro.metadata.name)
| where podName startswith "ama-metrics-" and podName !contains "operator" and podName !contains "ksm" and podName !contains "config"
| extend nodeName = tostring(ro.spec.nodeName)
| where isnotempty(nodeName)
| extend phase = tostring(ro.status.phase)
| extend controllertype = iff(podName contains "node", "DaemonSet", "ReplicaSet")
| summarize arg_max(PreciseTimeStamp, *) by podName
| project PreciseTimeStamp, podName, controllertype, nodeName, phase
| summarize podCount=count(), pods=make_list(podName) by controllertype, nodeName
| order by controllertype asc, podCount desc`,
        },
        {
            name: "System Pool Node Resources",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where objectRef.resource == "nodes"
| where verb in ("patch", "update")
| extend node = tostring(objectRef.name)
| extend nodepool = tostring(responseObject.metadata.labels.['agentpool'])
| extend roles = tostring(responseObject.metadata.labels.['kubernetes.azure.com/role'])
| where roles == "agent" or nodepool has "sys"
| summarize arg_max(PreciseTimeStamp, *) by node
| mv-apply condition = responseObject.status.conditions on (
    where condition.type in ("MemoryPressure", "Ready")
    | project conditionType = tostring(condition.type), conditionStatus = tostring(condition.status)
)
| extend allocatable_memory = tostring(responseObject.status.allocatable.memory)
| extend capacity_memory = tostring(responseObject.status.capacity.memory)
| extend allocatable_pods = tostring(responseObject.status.allocatable.pods)
| project PreciseTimeStamp, node, nodepool, conditionType, conditionStatus, allocatable_memory, capacity_memory, allocatable_pods`,
        },
        {
            name: "Node Status Timeline",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where objectRef.resource == "nodes"
| where verb in ("patch", "update")
| extend node = tostring(objectRef.name)
| extend nodepool = tostring(responseObject.metadata.labels.['agentpool'])
| mv-apply condition = responseObject.status.conditions on (
    where condition.type == "Ready"
    | project readyStatus = tostring(condition.status), reason = tostring(condition.reason), lastTransition = todatetime(condition.lastTransitionTime)
)
| project PreciseTimeStamp, node, nodepool, readyStatus, reason, lastTransition
| order by node asc, PreciseTimeStamp asc`,
        },
        {
            name: "Pod Schedule Events",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where requestObject has "ama-metrics"
| where verb == "create" or verb == "update"
| extend ro = parse_json(responseObject)
| where ro.kind == "Pod"
| extend podName = tostring(ro.metadata.name)
| where podName startswith "ama-metrics-" and podName !contains "operator" and podName !contains "ksm"
| extend nodeName = tostring(ro.spec.nodeName)
| where isnotempty(nodeName)
| extend nodepool = tostring(ro.spec.nodeSelector.['agentpool'])
| extend phase = tostring(ro.status.phase)
| project PreciseTimeStamp, verb, podName, nodeName, nodepool, phase
| order by PreciseTimeStamp asc`,
        },
        {
            name: "Cluster Autoscaler Events",
            datasource: "AKS CCP",
            kql: `let queryCcpNamespace = AKSClusterID;
let queryFrom = _startTime;
let queryTo = _endTime;
KubeAudit
| where PreciseTimeStamp between(queryFrom .. queryTo)
| where cluster_id == queryCcpNamespace
| where sourceIPs has "cluster-autoscaler"
    or requestObject has "cluster-autoscaler"
    or userAgent has "cluster-autoscaler"
    or requestObject has "ScaledUpGroup" or requestObject has "ScaleDown"
    or requestObject has "TriggeredScaleUp" or requestObject has "NotTriggerScaleUp"
| extend eventReason = tostring(parse_json(requestObject).reason)
| extend eventMessage = tostring(parse_json(requestObject).message)
| extend eventType = tostring(parse_json(requestObject).type)
| extend involvedObject = tostring(parse_json(requestObject).involvedObject.name)
| project PreciseTimeStamp, verb, eventReason, eventType, involvedObject, eventMessage
| order by PreciseTimeStamp desc`,
        },
    ],
    logs: [
        {
            name: "All ReplicaSet Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend controllertype=tostring(customDimensions.controllertype)
| where controllertype=="ReplicaSet"
| extend podname=tostring(customDimensions.podname)
| project timestamp, podname, message
| order by timestamp`,
        },
        {
            name: "All Linux DaemonSet Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend controllertype=tostring(customDimensions.controllertype)
| where controllertype=="DaemonSet"
| extend os=tostring(customDimensions.osType)
| where os == "linux"
| where tostring(customDimensions.tag) == "prometheus.log.prometheuscollectorcontainer"
| extend podname=tostring(customDimensions.podname)
| project timestamp, podname, message
| order by timestamp`,
        },
        {
            name: "All Windows DaemonSet Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| extend controllertype=tostring(customDimensions.controllertype)
| where controllertype=="DaemonSet"
| extend os=tostring(customDimensions.osType)
| where os == "windows"
| extend podname=tostring(customDimensions.podname)
| project timestamp, podname, message
| order by timestamp`,
        },
        {
            name: "All ConfigReader Logs",
            datasource: "PrometheusAppInsights",
            kql: `traces
| where timestamp > ago(_endTime - _startTime)
| where tostring(customDimensions.cluster) =~ _cluster
| where customDimensions.tag == "prometheus.log.targetallocator.configreader"
| project timestamp, message`,
        },
    ],
    controlPlane: [
        {
            name: "Enabled",
            datasource: "AKS",
            kql: `let queryComponentFrom = _startTime;
let queryComponentTo = _endTime;
let queryClusterVersion = AKSClusterID;
ControlPlaneWrapperSnapshot
| where PreciseTimeStamp between(queryComponentFrom..queryComponentTo)
| where namespace == queryClusterVersion
| where azureMonitorProfile.metrics.enabled == "true" and featureProfile.subscriptionRegisteredFeatures contains "AzureMonitorMetricsControlPlanePreview"
| extend featureEnabledForCluster = featureProfile.subscriptionRegisteredFeatures contains "AzureMonitorMetricsControlPlanePreview"
//| extend featureEnabledForCluster = iff(featureEnabledForCluster == "true", 1, 0)
| summarize arg_max(PreciseTimeStamp, featureEnabledForCluster) by subscription`,
        },
        {
            name: "Jobs Enabled",
            datasource: "AKS CCP",
            kql: `AMAMetricsConfigmapWatcher
| where PreciseTimeStamp > _startTime and PreciseTimeStamp < _endTime
| where ccpNamespace == AKSClusterID
| where configmap != "na"
| project PreciseTimeStamp, file, msg, configmap
| order by PreciseTimeStamp
| top 1 by PreciseTimeStamp
//| extend defaultscrapesettingsenabled = split(tostring(configmap["default-scrape-settings-enabled"]), "\\n")
| project configmap
| mv-apply e = extract_all(@"([\\w-]+) = (\\w+)", dynamic([1,2]), tostring(configmap["default-scrape-settings-enabled"]))
on (
    extend name = tostring(e[0])
    | extend value = tostring(e[1])
)
| where name startswith "controlplane"`,
        },
        {
            name: "Metrics KeepList",
            datasource: "AKS CCP",
            kql: `AMAMetricsConfigmapWatcher
| where PreciseTimeStamp > _startTime and PreciseTimeStamp < _endTime
| where ccpNamespace == AKSClusterID
| where configmap != "na"
| project PreciseTimeStamp, file, msg, configmap
| order by PreciseTimeStamp
| top 1 by PreciseTimeStamp
| project configmap
| mv-apply e = extract_all(@'([\\w-]+) = \\"(\\w*)\\"', dynamic([1,2]), tostring(configmap["default-targets-metrics-keep-list"]))
on (
    extend name = tostring(e[0])
    | extend value = tostring(e[1])
)
| where name startswith "controlplane"`,
        },
        {
            name: "Minimal Ingestion Profile Enabled",
            datasource: "AKS CCP",
            kql: `AMAMetricsConfigmapWatcher
| where PreciseTimeStamp > _startTime and PreciseTimeStamp < _endTime
| where ccpNamespace == AKSClusterID
| where configmap != "na"
| project PreciseTimeStamp, file, msg, configmap
| order by PreciseTimeStamp
| top 1 by PreciseTimeStamp
| project configmap
| mv-apply e = extract_all(@"([\\w-]+) = (\\w*)", dynamic([1,2]), tostring(configmap["default-targets-metrics-keep-list"]))
on (
    extend name = tostring(e[0])
    | where name == "minimalingestionprofile"
    | extend value = tostring(e[1])
)`,
        },
        {
            name: "Configmap Watcher Logs",
            datasource: "AKS CCP",
            kql: `AMAMetricsConfigmapWatcher
| where PreciseTimeStamp > _startTime and PreciseTimeStamp < _endTime
| where ccpNamespace == AKSClusterID
| where configmap != "na"
| project PreciseTimeStamp, file, msg, configmap
| order by PreciseTimeStamp`,
        },
        {
            name: "Prometheus-Collector Stdout Logs",
            datasource: "AKS CCP",
            kql: `let queryComponentFrom = _startTime;
let queryComponentTo = _endTime;
let queryClusterVersion = AKSClusterID;
let amametrics = union isfuzzy=true cluster('akshuba.centralus').database('AKSccplogs').AMAMetrics,
cluster('akshubintv2.eastus').database('AKSccplogs').AMAMetrics
| where PreciseTimeStamp between(queryComponentFrom..queryComponentTo)
| where ccpNamespace == queryClusterVersion
| where log !contains 'HeartbeatSignal	Mark' and log !contains '#Time' and log !contains '#Fields' and log !contains 'AggregatedMetricsPublisher' and log !contains 'Next refresh of MSI token'
| project PreciseTimeStamp, log, container="prometheus-collector";
amametrics
// let configmap_watcher=union isfuzzy=true cluster('akshuba.centralus').database('AKSccplogs').AMAMetricsConfigmapWatcher,
// cluster('akshubintv2.eastus').database('AKSccplogs').AMAMetricsConfigmapWatcher
// | where PreciseTimeStamp between(queryComponentFrom..queryComponentTo)
// | where ccpNamespace == queryClusterVersion
// | where msg !contains 'HeartbeatSignal	Mark' and msg !contains '#Time' and msg !contains '#Fields'
// | project PreciseTimeStamp, log=msg, container="confimap-watcher";
// union amametrics, configmap_watcher
// | sort by PreciseTimeStamp desc
// | take 100`,
        },
        {
            name: "Container Restarts",
            datasource: "AKS Infra",
            kql: `let queryNamespace = AKSClusterID;
ProcessInfo
| where PreciseTimeStamp between(_startTime.._endTime)
| where PodNamespace == queryNamespace
| where PodContainerName in ("configmap-watcher", "prometheus-collector")
| where PodContainerRestartCount > 0
| distinct PodName, PodContainerName, PodContainerStartedAt, PodContainerRestartCount, ImageRepoTags`,
        },
        {
            name: "Max CPU Usage by Container",
            datasource: "AKS Infra",
            kql: `let queryNamespace = AKSClusterID;
ProcessInfo
| where PreciseTimeStamp between(_startTime.._endTime)
| where PodNamespace == queryNamespace
| where PodContainerName in ("configmap-watcher", "prometheus-collector")
| project TIMESTAMP, PodName, PodContainerName, CPUUtil
| summarize cpu=max(CPUUtil) by bin(TIMESTAMP, totimespan(Interval)), PodName, PodContainerName
//| where PodContainerRestartCount > 0
//| distinct PodName, PodContainerName, PodContainerStartedAt, PodContainerRestartCount, ImageRepoTags`,
        },
    ],
    metricInsights: [
        {
            name: "Top Metrics by Time Series Count",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| project
    ['Metric Name']=MetricName,
    ['Dimensions']=Dimensions,
    ['Daily Time Series Count']=DailyTSAcrossAccounts
| order by ['Daily Time Series Count'] desc`,
        },
        {
            name: "Top Metrics by Sample Rate",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| project
    ['Metric Name']=MetricName,
    ['Dimensions']=Dimensions,
    ['Avg Sample Rate']=round(AvgEventRate, 0)
| order by ['Avg Sample Rate'] desc`,
        },
        {
            name: "Full Metric Volume Summary",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| project
    ['Metric Name']=MetricName,
    ['Dimensions']=Dimensions,
    ['Daily Time Series Count']=DailyTSAcrossAccounts,
    ['Avg Sample Rate']=round(AvgEventRate, 0)
| order by ['Daily Time Series Count'] desc`,
        },
        {
            name: "Total Time Series and Events Summary",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| summarize
    ['Total Metrics']=dcount(MetricName),
    ['Total Daily Time Series']=sum(DailyTSAcrossAccounts),
    ['Total Avg Sample Rate']=round(sum(AvgEventRate), 0)`,
        },
        {
            name: "Top 20 Highest Cardinality Metrics",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| summarize
    ['Daily Time Series']=sum(DailyTSAcrossAccounts),
    ['Avg Sample Rate']=round(sum(AvgEventRate), 0),
    ['Dimension Combos']=count()
    by MetricName
| order by ['Daily Time Series'] desc
| take 20`,
        },
        {
            name: "Metrics with High Dimension Cardinality",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| summarize
    ['Unique Dimension Combos']=count(),
    ['Total Daily Time Series']=sum(DailyTSAcrossAccounts),
    ['Avg Sample Rate']=round(sum(AvgEventRate), 0)
    by MetricName
| where ['Unique Dimension Combos'] > 100
| order by ['Unique Dimension Combos'] desc`,
        },
        {
            name: "Volume by Metric Category",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| extend category = case(
    MetricName startswith "istio_", "Istio",
    MetricName startswith "envoy_", "Envoy",
    MetricName startswith "container_network_", "NetworkObservability",
    MetricName startswith "container_", "Container",
    MetricName startswith "node_", "NodeExporter",
    MetricName startswith "kube_", "KubeStateMetrics",
    MetricName startswith "up" or MetricName startswith "scrape_", "ScrapeHealth",
    "Other"
)
| summarize
    ['Metric Count']=dcount(MetricName),
    ['Daily Time Series']=sum(DailyTSAcrossAccounts),
    ['Avg Sample Rate']=round(sum(AvgEventRate), 0)
    by category
| order by ['Daily Time Series'] desc`,
        },
        {
            name: "View All Metric Names",
            datasource: "MetricInsights",
            kql: `let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| distinct MetricName
| order by MetricName asc`,
        },
        {
            name: "Per-Dimension Cardinality Breakdown (Top 10 Metrics)",
            datasource: "MetricInsights",
            kql: `// Shows which dimensions cause cardinality explosion for the top 10 highest-cardinality metrics
let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
let topMetrics = GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| summarize ['Daily TS']=sum(DailyTSAcrossAccounts) by MetricName
| top 10 by ['Daily TS'];
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| where MetricName in ((topMetrics | project MetricName))
| project MetricName, Dimensions, DailyTSAcrossAccounts
| mv-expand Dimensions
| extend DimName = tostring(Dimensions)
| summarize ['Unique Values']=dcount(DimName), ['Total TS']=sum(DailyTSAcrossAccounts) by MetricName, DimName
| order by MetricName asc, ['Unique Values'] desc`,
        },
        {
            name: "Cardinality Trend Over Time (Top 5 Metrics, Last 30d)",
            datasource: "MetricInsights",
            kql: `// Tracks daily time series growth for top 5 metrics to detect cardinality spikes
let _mdmAccount = mdmAccountID;
let _namespace = dynamic(['customdefault']);
EventStatsInsightsV2
| where MonitoringAccount == _mdmAccount
| where MetricNamespace in~ (_namespace)
| where PreciseTimeStamp > ago(30d)
| summarize ['Daily TS']=max(DailyTimeSeriesCount) by MetricName, bin(PreciseTimeStamp, 1d)
| partition hint.strategy=native by MetricName (top 1 by ['Daily TS'])
| top 5 by ['Daily TS']
| project TopMetric=MetricName;
EventStatsInsightsV2
| where MonitoringAccount == _mdmAccount
| where MetricNamespace in~ (_namespace)
| where PreciseTimeStamp > ago(30d)
| where MetricName in ((TopMetric))
| summarize ['Daily TS']=max(DailyTimeSeriesCount) by MetricName, Day=bin(PreciseTimeStamp, 1d)
| order by MetricName asc, Day asc`,
        },
        {
            name: "Metric Dimension Names and Value Counts",
            datasource: "MetricInsights",
            kql: `// For a specific high-cardinality metric, shows each dimension and how many unique values it has
// Useful to identify which label (e.g. pod, container_id, instance) is causing cardinality explosion
let _mdmAccount = mdmAccountID;
let _metric = dynamic(null);
let _namespace = dynamic(['customdefault']);
let _preaggDimensions = dynamic(null);
let _numOfDaysQueryLookBack = 180;
GetPreaggUsageSummaryExploratoryV7(_mdmAccount, _namespace, _metric, _preaggDimensions, false, _numOfDaysQueryLookBack)
| summarize ['Unique Dimension Combos']=count(), ['Total Daily TS']=sum(DailyTSAcrossAccounts), ['Avg Sample Rate']=round(sum(AvgEventRate), 0) by MetricName
| where ['Unique Dimension Combos'] > 50
| extend ['Cardinality Risk'] = case(
    ['Unique Dimension Combos'] > 1000, "🔴 Critical",
    ['Unique Dimension Combos'] > 500, "🟠 High",
    ['Unique Dimension Combos'] > 100, "🟡 Medium",
    "🟢 Low")
| order by ['Unique Dimension Combos'] desc
| take 20`,
        },
    ],
    armInvestigation: [
        {
            name: "ARM PUT Operations by Resource Provider (Subscription Health)",
            datasource: "ARMPRODSEA",
            kql: `// Switch datasource to ARMPRODSEA/ARMPRODEUS/ARMPRODWEU based on cluster region
// Asia/Pacific/UK/Africa → ARMPRODSEA, Americas → ARMPRODEUS, Europe → ARMPRODWEU
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where httpMethod == 'PUT'
| summarize count() by toupper(targetResourceProvider)
| order by count_ desc
| take 30`,
        },
        {
            name: "Managed Clusters PUT Operations (Addon Enablement Check)",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.CONTAINERSERVICE'
| where toupper(targetResourceType) has 'MANAGEDCLUSTERS'
| where httpMethod == 'PUT'
| project TIMESTAMP, httpMethod, httpStatusCode, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "Microsoft.Insights PUT/DELETE Operations (DCR/DCE/DCRA)",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'
| where httpMethod in ('PUT', 'DELETE')
| project TIMESTAMP, httpMethod, httpStatusCode, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "Microsoft.Insights DELETE Details (DCR/DCE/DCRA Deletion)",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'
| where httpMethod == 'DELETE'
| extend resourceGroup = extract(@'/resourcegroups/([^/]+)/', 1, tolower(targetUri))
| extend resourceType = extract(@'/providers/microsoft\.insights/([^/]+)/', 1, tolower(targetUri))
| extend resourceName = extract(@'/providers/microsoft\.insights/[^/]+/([^?]+)', 1, tolower(targetUri))
| project TIMESTAMP, httpMethod, httpStatusCode, resourceGroup, resourceType, resourceName, targetUri, userAgent
| order by TIMESTAMP desc`,
        },
        {
            name: "ContainerService Operations Breakdown",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.CONTAINERSERVICE'
| summarize count() by httpMethod, toupper(targetResourceType)
| order by count_ desc`,
        },
        {
            name: "ARM Outgoing Requests to Insights RP (AKS RP → Monitor RP)",
            datasource: "ARMPRODSEA",
            kql: `HttpOutgoingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'
| project TIMESTAMP, httpMethod, httpStatusCode, targetUri, correlationId
| order by TIMESTAMP desc
| take 50`,
        },
        {
            name: "All Operations on Specific Cluster (Last 30d)",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where targetUri has '_clusterName'
| project TIMESTAMP, httpMethod, httpStatusCode, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "All Subscription DELETEs on Microsoft.Insights (DCR/DCE/DCRA)",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'
| where httpMethod == 'DELETE'
| extend resourceGroup = extract(@'/resourcegroups/([^/]+)/', 1, tolower(targetUri))
| extend resourceType = extract(@'/providers/microsoft\\.insights/([^/]+)/', 1, tolower(targetUri))
| extend resourceName = extract(@'/providers/microsoft\\.insights/[^/]+/([^?]+)', 1, tolower(targetUri))
| extend parentResource = extract(@'/providers/([^/]+/[^/]+/[^/]+)/providers/microsoft\\.insights', 1, tolower(targetUri))
| project TIMESTAMP, httpStatusCode, resourceGroup, resourceType, resourceName, parentResource, targetUri, userAgent
| order by TIMESTAMP desc`,
        },
        {
            name: "AMW (Microsoft.Monitor) All Operations",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.MONITOR'
| extend resourceGroup = extract(@'/resourcegroups/([^/]+)/', 1, tolower(targetUri))
| extend resourceType = extract(@'/providers/microsoft\\.monitor/([^/]+)', 1, tolower(targetUri))
| extend resourceName = extract(@'/providers/microsoft\\.monitor/[^/]+/([^?/]+)', 1, tolower(targetUri))
| summarize count(), methods=make_set(httpMethod), statuses=make_set(httpStatusCode), firstSeen=min(TIMESTAMP), lastSeen=max(TIMESTAMP) by resourceGroup, resourceType, resourceName
| order by count_ desc`,
        },
        {
            name: "AMW (Microsoft.Monitor) PUT/DELETE Operations",
            datasource: "ARMPRODSEA",
            kql: `HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.MONITOR'
| where httpMethod in ('PUT', 'DELETE')
| extend resourceGroup = extract(@'/resourcegroups/([^/]+)/', 1, tolower(targetUri))
| extend resourceType = extract(@'/providers/microsoft\\.monitor/([^/]+)', 1, tolower(targetUri))
| extend resourceName = extract(@'/providers/microsoft\\.monitor/[^/]+/([^?/]+)', 1, tolower(targetUri))
| project TIMESTAMP, httpMethod, httpStatusCode, resourceGroup, resourceType, resourceName, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "DCRA Operations for Cluster (dataCollectionRuleAssociations)",
            datasource: "ARMPRODSEA",
            kql: `// Shows all DCRA operations (PUT/GET/DELETE) targeting this specific cluster
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where tolower(targetUri) has tolower('_clusterName')
| where tolower(targetUri) has 'datacollectionruleassociations'
| extend dcraName = extract(@'/providers/microsoft\.insights/datacollectionruleassociations/([^?/]+)', 1, tolower(targetUri))
| extend httpMethodAndStatus = strcat(httpMethod, ' → ', httpStatusCode)
| project TIMESTAMP, httpMethodAndStatus, dcraName, httpStatusCode, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "DCRA Failed Operations (4xx/5xx errors)",
            datasource: "ARMPRODSEA",
            kql: `// Shows failed DCRA/DCR/DCE operations — creation failures, permission errors, region mismatches
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where toupper(targetResourceProvider) == 'MICROSOFT.INSIGHTS'
| where httpStatusCode >= 400
| extend resourceType = extract(@'/providers/microsoft\.insights/([^/]+)/', 1, tolower(targetUri))
| extend resourceName = extract(@'/providers/microsoft\.insights/[^/]+/([^?]+)', 1, tolower(targetUri))
| extend parentResource = extract(@'/providers/([^/]+/[^/]+/[^/]+)/providers/microsoft\.insights', 1, tolower(targetUri))
| project TIMESTAMP, httpMethod, httpStatusCode, resourceType, resourceName, parentResource, targetUri, userAgent, correlationId
| order by TIMESTAMP desc`,
        },
        {
            name: "DCE Operations in Subscription (dataCollectionEndpoints)",
            datasource: "ARMPRODSEA",
            kql: `// Shows all DCE create/delete operations — critical for private link clusters
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where tolower(targetUri) has 'datacollectionendpoints'
| where httpMethod in ('PUT', 'DELETE')
| extend resourceGroup = extract(@'/resourcegroups/([^/]+)/', 1, tolower(targetUri))
| extend dceName = extract(@'/datacollectionendpoints/([^?/]+)', 1, tolower(targetUri))
| project TIMESTAMP, httpMethod, httpStatusCode, resourceGroup, dceName, targetUri, userAgent
| order by TIMESTAMP desc`,
        },
        {
            name: "DCRA History Timeline (datacollectionruleassociations on cluster)",
            datasource: "ARMPRODSEA",
            kql: `// Full timeline of DCRA create/delete/get operations on the cluster
// Reveals: setup attempts, teardown cycles, 403 permission failures, naming conventions tried
// Critical for multi-AMW routing issues — shows if partner DCRA was deleted or never created
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where tolower(targetUri) has 'datacollectionruleassociations'
| where tolower(targetUri) has '_clusterName'
| where httpStatusCode != -1
| extend assocName = extract(@'(?i)datacollectionruleassociations/([^?/]+)', 1, targetUri)
| summarize
    FirstSeen = min(TIMESTAMP),
    LastSeen = max(TIMESTAMP),
    PutSuccess = countif(httpMethod == 'PUT' and httpStatusCode >= 200 and httpStatusCode < 300),
    PutFailed = countif(httpMethod == 'PUT' and httpStatusCode >= 400),
    DeleteSuccess = countif(httpMethod == 'DELETE' and httpStatusCode >= 200 and httpStatusCode < 300),
    DeleteFailed = countif(httpMethod == 'DELETE' and httpStatusCode >= 400),
    GetCount = countif(httpMethod == 'GET')
    by assocName
| order by LastSeen desc`,
        },
        {
            name: "DCRA Detailed Timeline for Specific Partner",
            datasource: "ARMPRODSEA",
            kql: `// Detailed chronological timeline of ALL DCRA operations on the cluster
// Use to trace exact sequence of create/delete cycles and identify permission issues
// Filter further by adding: | where assocName has '<partner-keyword>'
HttpIncomingRequests
| where TIMESTAMP > ago(30d)
| where subscriptionId == '_subscriptionId'
| where tolower(targetUri) has 'datacollectionruleassociations'
| where tolower(targetUri) has '_clusterName'
| where httpStatusCode != -1
| extend assocName = extract(@'(?i)datacollectionruleassociations/([^?/]+)', 1, targetUri)
| project TIMESTAMP, httpMethod, httpStatusCode, assocName
| order by TIMESTAMP asc`,
        },
    ],
};
/**
 * Replace dashboard parameters in a KQL query with actual values.
 */
export function parameterizeQuery(kql, params) {
    const timeRange = params.timeRange || "24h";
    const interval = params.interval || "6h";
    let q = kql;
    // Replace time parameters — use absolute datetimes when provided, else relative ago()
    if (params.startTime && params.endTime) {
        const start = `datetime("${params.startTime}")`;
        const end = `datetime("${params.endTime}")`;
        q = q.replace(/ago\(_endTime\s*-\s*_startTime\)/g, start);
        q = q.replace(/_startTime/g, start);
        q = q.replace(/_endTime/g, end);
    }
    else {
        q = q.replace(/ago\(_endTime\s*-\s*_startTime\)/g, `ago(${timeRange})`);
        q = q.replace(/_startTime/g, `ago(${timeRange})`);
        q = q.replace(/_endTime/g, "now()");
    }
    q = q.replace(/totimespan\(Interval\)/g, `totimespan(${interval})`);
    q = q.replace(/\bInterval\b/g, `"${interval}"`);
    // Replace cluster parameter — use word boundary to avoid corrupting variables
    // like local_clusterVersion that contain "_cluster" as a substring
    q = q.replace(/(?<![a-zA-Z0-9])_cluster(?![a-zA-Z0-9_])/g, `"${params.cluster}"`);
    // Replace ARM investigation tokens derived from the cluster ARM resource ID
    // Extract subscriptionId and cluster name from: /subscriptions/{sub}/resourceGroups/{rg}/providers/.../managedClusters/{name}
    const subMatch = params.cluster.match(/\/subscriptions\/([^/]+)\//i);
    const nameMatch = params.cluster.match(/\/managedClusters\/([^/]+)$/i);
    if (subMatch) {
        q = q.replace(/'_subscriptionId'/g, `'${subMatch[1]}'`);
    }
    if (nameMatch) {
        q = q.replace(/'_clusterName'/g, `'${nameMatch[1]}'`);
    }
    // Replace MDM account if provided
    if (params.mdmAccountId) {
        q = q.replace(/(?<![a-zA-Z0-9])mdmAccountID(?![a-zA-Z0-9_])/g, `"${params.mdmAccountId}"`);
    }
    // Replace AKS cluster ID if provided
    if (params.aksClusterId) {
        q = q.replace(/(?<![a-zA-Z0-9])AKSClusterID(?![a-zA-Z0-9_])/g, `"${params.aksClusterId}"`);
    }
    return q;
}
//# sourceMappingURL=queries.js.map