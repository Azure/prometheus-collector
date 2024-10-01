# Instructions for taking newer versions for our dependent charts

We have dependency on `kube-state-metrics` and `prometheus-node-exporter` external charts. The source for both the dependent charts are under otelcollector/deploy/dependentcharts in respective folders.

We will take periodic updated charts (and images) for these dependencies. Below is the outline for steps involved in updating these dependencies to a later version. MSFT OSS Upstream team will produce safe images for each release for the above 2 projects. We (Container Insights team) will consume that image and produce charts for these 2 projects.

## Check for updated versions in the below repos for these charts
 - [Kube-state-metrics](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-state-metrics)
 - [Prometheus node exporter](https://github.com/prometheus-community/helm-charts/blob/main/charts/prometheus-node-exporter/)

Note: You can update 1 chart or both the charts, depending on what needs to be updated/refreshed.

Chart version and image version are different. You can check the latest chart & image versions for kube-state-metrics [here](https://github.com/kubernetes/kube-state-metrics/) and for Prometheus-node-exporter [here](https://github.com/prometheus/node_exporter/). Every chart version goes hand-in-hand with the image version released along with it. So please don't change the image version used by any given chart version. The only change we need to make in the chart is to pick up the image from MSFT OSS container registry (the same image version as in the chart version we pick).

The OSS MSFT repository for kube-state-metrics is [here](https://azcuindexer.azurewebsites.net/repositories/oss/kubernetes/kube-state-metrics) and for Prometheus-node-exporter is [here](https://azcuindexer.azurewebsites.net/repositories/oss/prometheus/node-exporter). Ensure that the image tag used in the chart, is indeed available in MSFT OSS container repository for the corresponding chart.

## Node Exporter Chart
This chart is only used for our Arc agent. AKS now handles and owns node-exporter installation/upgrades.
  
1. Create a branch for chart update only. Copy the new node-exporter chart into `otelcollector/deploy/dependentcharts/prometheus-node-exporter`.
2. Trigger chart build & push through the Github action `build-and-push-dependent-helm-charts`. The parameter is 1 chart name. i.e `prometheus-node-exporter`. Currently, these charts will be packaged and pushed to our cidev ACR repository, which will be reconciled with MCR. The image tag will be the chart version in the branch
3. Once the chart is pushed to MCR, update `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/Chart-template.yaml` with the correct dependent chart version. Include this in the branch and PR with the node-exporter chart changes.
4. Test that upgrading the helm chart from the existing version to the new one succeeds without conflicts. You may need to revert the `selector labels` if these were changed. 
5. Update the following variables in the [release pipeline](https://github-private.visualstudio.com/azure/_releaseDefinition?definitionId=79&_a=definition-variables):
    - `NEChartTag` - with the new chart version
    - `PushNewNEChart` - to true for the said release (remember to set it back to false after the release is done!)

## Kube-State-Metrics Chart
`Kube-state-metrics` is now included directly in our chart templates so that it replicates what we have in the AKS-RP repo. This will be used by both AKS and Arc.

The relevant files are prefixed with `ama-metrics-ksm` in `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates`.

1. Check if any changes in the `kube-state-metrics` chart are relevant to be added to our templates. Selector labels should not be changed to prevent upgrade issues.
2. Change the `KubeStateMetrics.ImageTag` value to the corresponding version to the chart in `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml`. This tag is different from the chart version.
3. Create a PR with these template changes.
4. Test and release as usual.
