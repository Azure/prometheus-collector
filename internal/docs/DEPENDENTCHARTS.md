- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)


# Instructions for taking newer versions for our dependent charts

We have dependency on kube-state-metrics and prometheus-node-exporter external charts. The source for both the dependent charts are under otelcollector/deploy/dependentcharts in respective folders.

We will take periodic updated charts (and images) for these dependencies. Below is the outline for steps involved in updating these dependencies to a later version. MSFT OSS Upstream team will produce safe images for each release for the above 2 projects. We (Container Insights team) will consume that image and produce charts for these 2 projects.

#### Step 1 : Check and look for updated versions in the below repos for these charts -
 - [Kube-state-metrics](https://github.com/prometheus-community/helm-charts/blob/main/charts/kube-state-metrics)
 - [Prometheus node exporter](https://github.com/prometheus-community/helm-charts/blob/main/charts/prometheus-node-exporter/)

Note: You can update 1 chart or both the charts, depending on what needs to be updated/refreshed.

Chart version and image version are different. You can check the latest chart & image versions for kube-state-metrics [here](https://github.com/kubernetes/kube-state-metrics/) and for Prometheus-node-exporter [here](https://github.com/prometheus/node_exporter/). Every chart version goes hand-in-hand with the image version released along with it. So please don't change the image version used by any given chart version. The only change we need to make in the chart is to pick up the image from MSFT OSS container registry (the same image version as in the chart version we pick).

The OSS MSFT repository for kube-state-metrics is [here](https://azcuindexer.azurewebsites.net/repositories/oss/kubernetes/kube-state-metrics) and for Prometheus-node-exporter is [here](https://azcuindexer.azurewebsites.net/repositories/oss/prometheus/node-exporter). Ensure that the image tag used in the chart, is indeed available in MSFT OSS container repository for the corresponding chart.

After taking the latest chart(s), only change required is changing the default value for `image.repository` in values.yaml to the below -

kube-state-metrics       : `mcr.microsoft.com/oss/kubernetes/kube-state-metrics`
prometheus-node-exporter : `mcr.microsoft.com/oss/prometheus/node-exporter`
  
#### Step 2 : Create a PR for chart update only. Please keep this PR seperate from other changes.
#### Step 3 : After PR is approved and merged, trigger chart build & push thru the action `build-and-push-dependent-helm-charts`. The parameter is 1 chart name. i.e `prometheus-node-exporter` or `kube-state-metrics` depending on what is being updated/refreshed. If you want to update both, you would trigger this action twice (one after another). Currently, these charts will be packaged and pushed to our cidev ACR repository, which will be reconciled with MCR.
#### Step 4 : Update 'build-and-push-image-and-chart' workflow to scan for the updated images thru trivy
#### Step 5 : Once dependent chart(s) is/are packaged and pushed to our mcr, update our Prometheus collector charts' Chart-template.yaml with the correct chart version(s) for the dependency(ies) updated, and creatre a PR.


>Update the following variables in the [release pipeline](https://github-private.visualstudio.com/azure/_releaseDefinition?definitionId=79&_a=definition-variables)
>   a. KSMChartTag - with the new version
>   b. NEChartTag - with the new version
>   c. PushNewKSMChart - to true for the said release (remember to set it back to false after the release is done!)
>   d. PushNewNEChart - to true for the said release (remember to set it back to false after the release is done!)
