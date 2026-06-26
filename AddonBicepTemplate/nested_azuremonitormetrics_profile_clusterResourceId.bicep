param variables_clusterName string
param clusterLocation string
param metricLabelsAllowlist string
param metricAnnotationsAllowList string
param enableControlPlaneMetrics bool

// 2026-04-01 is the latest stable AKS API version and the first GA version to expose
// azureMonitorProfile.metrics.controlPlane. It is newer than the Bicep CLI's bundled
// type catalog, so suppress the "types not available" warning; the compiled ARM is
// correct and deploys fine.
#disable-next-line BCP081
resource variables_cluster 'Microsoft.ContainerService/managedClusters@2026-04-01' = {
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
        controlPlane: {
          enabled: enableControlPlaneMetrics
        }
      }
    }
  }
}
