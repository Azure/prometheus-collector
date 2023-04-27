param clusterLocation string

@description('Resource Id of the Azure Arc Connected Cluster')
param clusterResourceId string

resource aksCluster 'Microsoft.Kubernetes/connectedClusters@2022-10-01-preview' existing = {
  name: split(clusterResourceId, '/')[8]
}

resource azuremonitor_metrics 'Microsoft.KubernetesConfiguration/extensions@2021-09-01' = {
  scope: aksCluster
  name: 'azuremonitor-metrics'
  location: clusterLocation
  identity: {
    type: 'SystemAssigned'
  }
  properties: {
    extensionType: 'Microsoft.AzureMonitor.Containers.Metrics'
    configurationSettings: {
    }
    configurationProtectedSettings: {
    }
    autoUpgradeMinorVersion: true
    releaseTrain: 'Dev'
    scope: {
      cluster: {
        releaseNamespace: 'kube-system'
      }
    }
  }
}
