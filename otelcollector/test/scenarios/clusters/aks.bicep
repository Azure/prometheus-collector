targetScope = 'resourceGroup'

import { nodePool } from 'pools.bicep'

@minLength(1)
@maxLength(48)
param runName string
@minLength(1)
param scenarioId string
param location string
@description('Current regional supported/default version resolved by the runner; no historical version axis.')
@minLength(5)
param kubernetesVersion string
@minLength(1)
param owner string
@secure()
param windowsAdminPassword string = ''
param windowsAdminUsername string = 'scenarioadmin'

@minLength(1)
param nodePools nodePool[]
@allowed(['azure', 'cilium', 'kubenet'])
param dataplane string = 'azure'
@allowed(['overlay', 'classic'])
param azureCniMode string = 'overlay'
param workloadIdentityEnabled bool = true
param privateApi bool = false
param userAssignedIdentity bool = false
param acns bool = false
param privateMonitoring bool = false
@allowed(['candidate-addon', 'customer-helm', 'otlp'])
param installationMode string = 'candidate-addon'
param proxyEnabled bool = false
param proxyCustomCa bool = false
param noProxy string[] = []
@description('Public SSH key for the owned proxy VM; no inbound SSH is opened.')
param proxyAdminSshPublicKey string = ''
param proxyVmSize string = 'Standard_D2s_v4'
@allowed([1])
param proxyVmCount int = 1
@description('Optional DNS servers reachable from the owned VNet; caller owns forwarding of private zones.')
param dnsServers string[] = []
var windows = !empty(filter(nodePools, pool => pool.osType == 'Windows'))
var useUai = userAssignedIdentity || dataplane == 'kubenet'
var clusterName = 'sc-${uniqueString(resourceGroup().id, runName, scenarioId)}'
var tags = {
  owner: owner
  scenario: scenarioId
  run: runName
}
var networkContributor = subscriptionResourceId('Microsoft.Authorization/roleDefinitions', '4d97b98b-1d4f-4787-a291-c67834d212e7')
var effectivePrivateApi = privateApi || proxyEnabled
var effectiveNoProxy = union(
  ['localhost', '127.0.0.1', '169.254.169.254', '168.63.129.16', '10.224.0.0/12', '10.244.0.0/16', '10.0.0.0/16', '.svc', '.cluster.local', '.azmk8s.io'],
  privateMonitoring ? ['.monitor.azure.com', '.opinsights.azure.com', '.agentsvc.azure-automation.net'] : [],
  noProxy
)

// Native assertions remain experimental in Bicep 0.42; ARM allowedValues guard cluster creation instead.
module guards 'guards.bicep' = {
  name: '${clusterName}-guards'
  params: {
    windowsNeedsAzure: any(!windows || (dataplane == 'azure' && !proxyEnabled))
    windowsNeedsPassword: any(!windows || length(windowsAdminPassword) >= 14)
    windows2022Version: any(empty(filter(nodePools, pool => pool.osSKU == 'Windows2022')) || int(split(kubernetesVersion, '.')[1]) < 37)
    windows2025Version: any(empty(filter(nodePools, pool => pool.osSKU == 'Windows2025')) || int(split(kubernetesVersion, '.')[1]) >= 32)
    acnsNeedsAzure: any(!acns || dataplane != 'kubenet')
    proxyNeedsSshKey: any(!proxyEnabled || startsWith(proxyAdminSshPublicKey, 'ssh-'))
    proxyCaNeedsProxy: any(!proxyCustomCa || proxyEnabled)
    proxyNoProxyIsScoped: any(!proxyEnabled || (!contains(join(noProxy, ','), '*') && !contains(noProxy, '0.0.0.0/0') && !contains(noProxy, '::/0')))
    validPoolPlan: any(length(filter(nodePools, pool => pool.mode == 'System' && pool.osType == 'Linux')) >= 1 && length(toObject(nodePools, pool => pool.name)) == length(nodePools) && empty(filter(nodePools, pool => pool.minCount > pool.count || pool.count > pool.maxCount || (!pool.autoscaling && (pool.minCount != pool.count || pool.maxCount != pool.count)) || (pool.osType == 'Windows' && (pool.architecture != 'amd64' || length(pool.name) > 6 || !startsWith(pool.osSKU, 'Windows') || (pool.osSKU == 'Windows2025' && !pool.fips))) || (pool.osType == 'Linux' && startsWith(pool.osSKU, 'Windows')) || (pool.architecture == 'arm64' && pool.fips && pool.osSKU != 'AzureLinux3'))))
    classicNeedsAzure: any(azureCniMode != 'classic' || dataplane == 'azure')
    // Keep Windows FIPS on the declared SKU; expanding this coverage needs explicit regional image/SKU preflight.
    windowsFipsVmSupported: any(empty(filter(nodePools, pool => pool.osType == 'Windows' && pool.fips && pool.vmSize != 'Standard_D4s_v4')))
  }
}

resource routes 'Microsoft.Network/routeTables@2024-05-01' = if (dataplane == 'kubenet') {
  name: '${clusterName}-routes'
  location: location
  tags: tags
  properties: { disableBgpRoutePropagation: false }
}

resource vnet 'Microsoft.Network/virtualNetworks@2024-05-01' = {
  name: '${clusterName}-vnet'
  location: location
  tags: tags
  properties: {
    addressSpace: { addressPrefixes: ['10.224.0.0/12'] }
    dhcpOptions: { dnsServers: dnsServers }
    subnets: [
      {
        name: 'nodes'
        properties: {
          addressPrefix: '10.224.0.0/16'
          defaultOutboundAccess: false
          routeTable: dataplane == 'kubenet' ? { id: routes!.id } : null
          networkSecurityGroup: proxyEnabled ? { id: nodeEgressNsg!.id } : null
        }
      }
      {
        name: 'proxy'
        properties: {
          addressPrefix: '10.226.0.0/24'
          defaultOutboundAccess: false
          natGateway: proxyEnabled ? { id: proxyNat!.id } : null
          networkSecurityGroup: proxyEnabled ? { id: proxyNsg!.id } : null
        }
      }
      {
        name: 'endpoints'
        properties: {
          addressPrefix: '10.225.0.0/24'
          defaultOutboundAccess: false
          privateEndpointNetworkPolicies: 'Disabled'
        }
      }
    ]
  }
}
var nodeSubnetId = resourceId('Microsoft.Network/virtualNetworks/subnets', vnet.name, 'nodes')
var endpointSubnetId = resourceId('Microsoft.Network/virtualNetworks/subnets', vnet.name, 'endpoints')
var proxySubnetId = resourceId('Microsoft.Network/virtualNetworks/subnets', vnet.name, 'proxy')

resource proxyIp 'Microsoft.Network/publicIPAddresses@2024-05-01' = if (proxyEnabled) {
  name: '${clusterName}-proxy-egress'
  location: location
  tags: tags
  sku: { name: 'Standard' }
  properties: { publicIPAllocationMethod: 'Static' }
}
resource proxyNat 'Microsoft.Network/natGateways@2024-05-01' = if (proxyEnabled) {
  name: '${clusterName}-proxy-nat'
  location: location
  tags: tags
  sku: { name: 'Standard' }
  properties: { publicIpAddresses: [{ id: proxyIp!.id }] }
}
resource proxyNsg 'Microsoft.Network/networkSecurityGroups@2024-05-01' = if (proxyEnabled) {
  name: '${clusterName}-proxy-nsg'
  location: location
  tags: tags
  properties: {
    securityRules: [
      {
        name: 'AllowNodesProxy'
        properties: {
          priority: 100
          access: 'Allow'
          direction: 'Inbound'
          protocol: 'Tcp'
          sourceAddressPrefix: '10.224.0.0/16'
          sourcePortRange: '*'
          destinationAddressPrefix: '10.226.0.4'
          destinationPortRange: '3128'
        }
      }
      {
        name: 'DenyOtherInbound'
        properties: {
          priority: 200
          access: 'Deny'
          direction: 'Inbound'
          protocol: '*'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: '*'
          destinationPortRange: '*'
        }
      }
    ]
  }
}
resource nodeEgressNsg 'Microsoft.Network/networkSecurityGroups@2024-05-01' = if (proxyEnabled) {
  name: '${clusterName}-node-egress-nsg'
  location: location
  tags: tags
  properties: {
    securityRules: [
      {
        name: 'AllowOwnedNetwork'
        properties: {
          priority: 100
          access: 'Allow'
          direction: 'Outbound'
          protocol: '*'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: '10.224.0.0/12'
          destinationPortRange: '*'
        }
      }
      {
        name: 'DenyDirectWeb'
        properties: {
          priority: 200
          access: 'Deny'
          direction: 'Outbound'
          protocol: 'Tcp'
          sourceAddressPrefix: '*'
          sourcePortRange: '*'
          destinationAddressPrefix: 'Internet'
          destinationPortRanges: ['80', '443']
        }
      }
    ]
  }
}

module proxyHost 'proxy.bicep' = [for i in range(0, proxyVmCount): if (proxyEnabled) {
  name: '${clusterName}-proxy-${i}'
  params: {
    name: '${clusterName}-proxy'
    location: location
    tags: tags
    subnetId: proxySubnetId
    vmSize: proxyVmSize
    adminSshPublicKey: proxyAdminSshPublicKey
    customCa: proxyCustomCa
  }
  dependsOn: [guards]
}]

resource identity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = if (useUai) {
  name: '${clusterName}-identity'
  location: location
  tags: tags
}

resource uaiNetworkRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (useUai) {
  name: guid(resourceGroup().id, identity!.id, networkContributor)
  properties: {
    principalId: identity!.properties.principalId
    principalType: 'ServicePrincipal'
    roleDefinitionId: networkContributor
  }
}

resource cluster 'Microsoft.ContainerService/managedClusters@2025-07-01' = {
  name: clusterName
  location: location
  tags: tags
  sku: { name: 'Base', tier: 'Free' }
  identity: useUai ? {
    type: 'UserAssigned'
    userAssignedIdentities: { '${identity!.id}': {} }
  } : { type: 'SystemAssigned' }
  properties: {
    dnsPrefix: clusterName
    nodeResourceGroup: '${clusterName}-nodes'
    kubernetesVersion: kubernetesVersion
    enableRBAC: true
    oidcIssuerProfile: { enabled: workloadIdentityEnabled }
    securityProfile: { workloadIdentity: { enabled: workloadIdentityEnabled } }
    autoUpgradeProfile: {
      upgradeChannel: 'stable'
      nodeOSUpgradeChannel: 'NodeImage'
    }
    apiServerAccessProfile: {
      enablePrivateCluster: effectivePrivateApi
      enablePrivateClusterPublicFQDN: false
    }
    azureMonitorProfile: { metrics: { enabled: false } }
    windowsProfile: windows ? {
      adminUsername: windowsAdminUsername
      adminPassword: windowsAdminPassword
      enableCSIProxy: true
    } : null
    httpProxyConfig: proxyEnabled ? {
      httpProxy: proxyHost[0]!.outputs.endpoint
      httpsProxy: proxyHost[0]!.outputs.endpoint
      noProxy: effectiveNoProxy
      trustedCa: proxyCustomCa ? proxyHost[0]!.outputs.trustedCa : null
    } : null
    networkProfile: {
      networkPlugin: dataplane == 'kubenet' ? 'kubenet' : 'azure'
      networkPluginMode: dataplane != 'kubenet' && azureCniMode == 'overlay' ? 'overlay' : null
      networkDataplane: dataplane == 'kubenet' ? null : dataplane
      networkPolicy: dataplane == 'cilium' ? 'cilium' : null
      outboundType: 'loadBalancer'
      loadBalancerSku: 'standard'
      loadBalancerProfile: { managedOutboundIPs: { count: 1 } }
      podCidr: dataplane == 'kubenet' || azureCniMode == 'overlay' ? '10.244.0.0/16' : null
      serviceCidr: '10.0.0.0/16'
      dnsServiceIP: '10.0.0.10'
      advancedNetworking: {
        enabled: acns
        observability: { enabled: acns }
        security: { enabled: acns && dataplane == 'cilium' }
      }
    }
    agentPoolProfiles: [for pool in nodePools: {
      name: pool.name
      count: pool.count
      vmSize: pool.vmSize
      mode: pool.mode
      osType: pool.osType
      osSKU: pool.osSKU
      enableFIPS: pool.fips
      type: 'VirtualMachineScaleSets'
      vnetSubnetID: nodeSubnetId
      osDiskType: 'Managed'
      osDiskSizeGB: 128
      enableNodePublicIP: false
      maxPods: azureCniMode == 'classic' ? 30 : null
      enableAutoScaling: pool.autoscaling
      minCount: pool.autoscaling ? pool.minCount : null
      maxCount: pool.autoscaling ? pool.maxCount : null
      upgradeSettings: { maxSurge: pool.maxSurge }
      nodeLabels: {
        'scenario-pool': pool.name
        'scenario-fips': string(pool.fips)
      }
      tags: tags
    }]
  }
  dependsOn: [guards, uaiNetworkRole]
}

// SAI is allocated by AKS creation; grant subsequent subnet/LB operations on the owned VNet.
resource saiNetworkRole 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (!useUai) {
  name: guid(vnet.id, cluster.id, networkContributor)
  scope: vnet
  properties: {
    principalId: cluster.identity.principalId!
    principalType: 'ServicePrincipal'
    roleDefinitionId: networkContributor
  }
}

resource workspace 'Microsoft.Monitor/accounts@2023-04-03' = {
  name: '${clusterName}-amw'
  location: location
  tags: tags
  properties: { publicNetworkAccess: privateMonitoring ? 'Disabled' : 'Enabled' }
}

resource dce 'Microsoft.Insights/dataCollectionEndpoints@2023-03-11' = {
  name: 'MSProm-${location}-${clusterName}'
  location: location
  tags: tags
  kind: 'Linux'
  properties: {
    networkAcls: { publicNetworkAccess: privateMonitoring ? 'Disabled' : 'Enabled' }
  }
}

resource dcr 'Microsoft.Insights/dataCollectionRules@2023-03-11' = {
  name: 'MSProm-${location}-${clusterName}'
  location: location
  tags: tags
  kind: 'Linux'
  properties: {
    dataCollectionEndpointId: dce.id
    dataSources: {
      prometheusForwarder: [{
        name: 'PrometheusDataSource'
        streams: ['Microsoft-PrometheusMetrics']
        labelIncludeFilter: {}
      }]
    }
    destinations: {
      monitoringAccounts: [{ name: 'MonitoringAccount1', accountResourceId: workspace.id }]
    }
    dataFlows: [{
      streams: ['Microsoft-PrometheusMetrics']
      destinations: ['MonitoringAccount1']
    }]
  }
}

resource ruleAssociation 'Microsoft.Insights/dataCollectionRuleAssociations@2023-03-11' = {
  name: 'prometheus-${uniqueString(cluster.id)}'
  scope: cluster
  properties: { dataCollectionRuleId: dcr.id }
}

resource otlpDcr 'Microsoft.Insights/dataCollectionRules@2024-03-11' = if (scenarioId == 'otlp') {
  name: 'otlp-${clusterName}'
  location: location
  tags: tags
  properties: {
    dataCollectionEndpointId: dce.id
    dataSources: any({
      otelMetrics: [{ name: 'otlpMetricsDataSource', streams: ['Custom-Metrics-otlp'] }]
    })
    destinations: {
      monitoringAccounts: [{ name: 'MonitoringAccount1', accountResourceId: workspace.id }]
    }
    dataFlows: [{
      streams: ['Custom-Metrics-otlp']
      destinations: ['MonitoringAccount1']
    }]
  }
}

resource otlpAssociation 'Microsoft.Insights/dataCollectionRuleAssociations@2023-03-11' = if (scenarioId == 'otlp') {
  name: 'otlp-${uniqueString(cluster.id)}'
  scope: cluster
  properties: { dataCollectionRuleId: otlpDcr!.id }
}

resource configurationAssociation 'Microsoft.Insights/dataCollectionRuleAssociations@2023-03-11' = if (privateMonitoring) {
  name: 'configurationAccessEndpoint'
  scope: cluster
  properties: { dataCollectionEndpointId: dce.id }
}

resource ampls 'Microsoft.Insights/privateLinkScopes@2021-07-01-preview' = if (privateMonitoring) {
  name: '${clusterName}-ampls'
  location: 'global'
  tags: tags
  properties: {
    accessModeSettings: {
      ingestionAccessMode: 'PrivateOnly'
      queryAccessMode: 'PrivateOnly'
      exclusions: []
    }
  }
}

resource ingestionScope 'Microsoft.Insights/privateLinkScopes/scopedResources@2021-07-01-preview' = if (privateMonitoring) {
  parent: ampls
  name: 'cluster-dce'
  properties: { linkedResourceId: dce.id }
}

resource workspaceScope 'Microsoft.Insights/privateLinkScopes/scopedResources@2021-07-01-preview' = if (privateMonitoring) {
  parent: ampls
  name: 'workspace-dce'
  properties: { linkedResourceId: workspace.properties.defaultIngestionSettings.dataCollectionEndpointResourceId }
}

resource monitorEndpoint 'Microsoft.Network/privateEndpoints@2024-05-01' = if (privateMonitoring) {
  name: '${clusterName}-monitor-pe'
  location: location
  tags: tags
  properties: {
    subnet: { id: endpointSubnetId }
    privateLinkServiceConnections: [{
      name: 'monitor'
      properties: { privateLinkServiceId: ampls!.id, groupIds: ['azuremonitor'] }
    }]
  }
  dependsOn: [ingestionScope, workspaceScope]
}

resource queryEndpoint 'Microsoft.Network/privateEndpoints@2024-05-01' = if (privateMonitoring) {
  name: '${clusterName}-query-pe'
  location: location
  tags: tags
  properties: {
    subnet: { id: endpointSubnetId }
    privateLinkServiceConnections: [{
      name: 'prometheus'
      properties: { privateLinkServiceId: workspace.id, groupIds: ['prometheusMetrics'] }
    }]
  }
}

var monitorZoneNames = [
  'privatelink.monitor.azure.com'
  'privatelink.oms.opinsights.azure.com'
  'privatelink.ods.opinsights.azure.com'
  'privatelink.agentsvc.azure-automation.net'
  'privatelink.blob.${environment().suffixes.storage}'
]
var zoneNames = concat(monitorZoneNames, ['privatelink.${location}.prometheus.monitor.azure.com'])
resource zones 'Microsoft.Network/privateDnsZones@2020-06-01' = [for name in zoneNames: if (privateMonitoring) {
  name: name
  location: 'global'
  tags: tags
}]
resource links 'Microsoft.Network/privateDnsZones/virtualNetworkLinks@2020-06-01' = [for (name, i) in zoneNames: if (privateMonitoring) {
  parent: zones[i]
  name: clusterName
  location: 'global'
  properties: { registrationEnabled: false, virtualNetwork: { id: vnet.id } }
}]
resource monitorDns 'Microsoft.Network/privateEndpoints/privateDnsZoneGroups@2024-05-01' = if (privateMonitoring) {
  parent: monitorEndpoint
  name: 'monitor'
  properties: {
    privateDnsZoneConfigs: [for (name, i) in monitorZoneNames: {
      name: 'zone-${i}'
      properties: { privateDnsZoneId: zones[i].id }
    }]
  }
}
resource queryDns 'Microsoft.Network/privateEndpoints/privateDnsZoneGroups@2024-05-01' = if (privateMonitoring) {
  parent: queryEndpoint
  name: 'prometheus'
  properties: {
    privateDnsZoneConfigs: [{
      name: 'prometheus'
      properties: { privateDnsZoneId: zones[length(monitorZoneNames)].id }
    }]
  }
}

resource retainCilium 'Microsoft.Authorization/locks@2020-05-01' = if (scenarioId == 'acns-cilium') {
  name: 'retain-acns-cilium'
  properties: {
    level: 'CanNotDelete'
    notes: 'Persistent managed ACNS Cilium scenario; retain for future candidate runs.'
  }
  dependsOn: [cluster, workspace]
}

output clusterId string = cluster.id
output clusterName string = cluster.name
output location string = location
output workspaceId string = workspace.id
output queryEndpoint string = workspace.properties.metrics.prometheusQueryEndpoint
output amwQueryEndpoint string = workspace.properties.metrics.prometheusQueryEndpoint
output dcrId string = dcr.id
output dceId string = dce.id
output dceConfigurationEndpoint string = dce.properties.configurationAccess.endpoint
output dceMetricsIngestionEndpoint string = dce.properties.metricsIngestion.endpoint
output configurationDceId string = privateMonitoring ? dce.id : ''
output configurationAssociationId string = privateMonitoring ? configurationAssociation!.id : ''
output ruleAssociationId string = ruleAssociation.id
output workspaceDefaultDceId string = workspace.properties.defaultIngestionSettings.dataCollectionEndpointResourceId
output vnetId string = vnet.id
output subnetId string = nodeSubnetId
output outboundType string = 'loadBalancer'
output amplsId string = privateMonitoring ? ampls!.id : ''
output monitorPrivateEndpointId string = privateMonitoring ? monitorEndpoint!.id : ''
output queryPrivateEndpointId string = privateMonitoring ? queryEndpoint!.id : ''
output privateDnsZoneNames array = privateMonitoring ? zoneNames : []
output expectedPools array = nodePools
output addonEnabled bool = false
output installationMode string = installationMode
output identityType string = useUai ? 'UserAssigned' : 'SystemAssigned'
output oidcIssuerUrl string = workloadIdentityEnabled ? cluster.properties.oidcIssuerProfile!.issuerURL! : ''
output workloadIdentityEnabled bool = workloadIdentityEnabled
output oidcIssuerEnabled bool = workloadIdentityEnabled
output azureCniMode string = dataplane == 'kubenet' ? '' : azureCniMode
output resourceTags object = tags
output privateApiEnabled bool = effectivePrivateApi
output proxyVmId string = proxyEnabled ? proxyHost[0]!.outputs.vmId : ''
output proxyVmName string = proxyEnabled ? proxyHost[0]!.outputs.vmName : ''
output proxyEndpoint string = proxyEnabled ? proxyHost[0]!.outputs.endpoint : ''
output proxyTrustedCa string = proxyEnabled ? proxyHost[0]!.outputs.trustedCa : ''
output proxyCaFingerprintSha256 string = proxyEnabled ? proxyHost[0]!.outputs.caFingerprintSha256 : ''
output proxyBootstrapCommandId string = proxyEnabled ? proxyHost[0]!.outputs.bootstrapCommandId : ''
output proxyAccessLogPath string = proxyEnabled ? '/var/log/squid/access.log' : ''
output proxyCaCertificatePath string = proxyCustomCa ? '/etc/squid/ssl_cert/ca.crt' : ''
output proxyNodeNsgId string = proxyEnabled ? nodeEgressNsg!.id : ''
output proxySubnetId string = proxyEnabled ? proxySubnetId : ''
output proxyNatGatewayId string = proxyEnabled ? proxyNat!.id : ''
output proxyNsgId string = proxyEnabled ? proxyNsg!.id : ''
output proxyEgressPublicIp string = proxyEnabled ? proxyIp!.properties.ipAddress : ''
output proxyNoProxy array = proxyEnabled ? effectiveNoProxy : []
output proxyNoProxyCsv string = proxyEnabled ? join(effectiveNoProxy, ',') : ''
output proxyTlsInterceptionEnabled bool = proxyEnabled && proxyCustomCa
output nodeResourceGroupName string = cluster.properties.nodeResourceGroup
output nodeResourceGroupId string = '${subscription().id}/resourceGroups/${cluster.properties.nodeResourceGroup}'
output amwManagedResourceGroupName string = split(workspace.properties.defaultIngestionSettings.dataCollectionEndpointResourceId, '/')[4]
output amwManagedResourceGroupId string = '${subscription().id}/resourceGroups/${split(workspace.properties.defaultIngestionSettings.dataCollectionEndpointResourceId, '/')[4]}'
