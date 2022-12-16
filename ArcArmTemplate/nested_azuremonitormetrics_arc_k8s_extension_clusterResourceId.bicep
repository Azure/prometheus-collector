param reference_parameters_azureMonitorWorkspaceResourceId_2015_03_20_customerId object
param clusterLocation string
param azureMonitorWorkspaceResourceId string
param clusterResourceId string

resource azuremonitor_metrics 'Microsoft.KubernetesConfiguration/extensions@2021-09-01' = {
  scope: 'Microsoft.Kubernetes/connectedClusters/${split(clusterResourceId, '/')[8]}'
  name: 'azuremonitor-metrics'
  location: clusterLocation
  identity: {
    type: 'SystemAssigned'
  }
  properties: {
    extensionType: 'Microsoft.AzureMonitor.Metrics'
    configurationSettings: {
    }
    configurationProtectedSettings: {
      'amalogs.secret.wsid': reference_parameters_azureMonitorWorkspaceResourceId_2015_03_20_customerId.customerId
      'amalogs.secret.key': listKeys(azureMonitorWorkspaceResourceId, '2015-03-20').primarySharedKey
    }
    autoUpgradeMinorVersion: true
    releaseTrain: 'Stable'
    scope: {
      cluster: {
        releaseNamespace: 'azuremonitor-metrics'
      }
    }
  }
}