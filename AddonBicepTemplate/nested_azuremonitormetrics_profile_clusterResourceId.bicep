param variables_clusterName string
param clusterLocation string
param metricLabelsAllowlist string
param metricAnnotationsAllowList string
param enableControlPlaneMetrics bool

resource variables_cluster 'Microsoft.ContainerService/managedClusters@2024-09-01' = {
  name: variables_clusterName
  location: clusterLocation
  properties: {
    azureMonitorProfile: {
      metrics: {
        enabled: true
        kubeStateMetrics: {
          metricLabelsAllowlist: metricLabelsAllowlist
          metricAnnotationsAllowList: metricAnnotationsAllowList
        }
        // controlPlane is supported by the AKS RP but not yet in the published Bicep
        // type for this apiVersion; suppress the type-check warning until types catch up.
        #disable-next-line BCP037
        controlPlane: {
          enabled: enableControlPlaneMetrics
        }
      }
    }
  }
}
