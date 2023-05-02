## Dev Build
- The azure-pipeline-build.yml file will build and push an Arc extension chart that uses the same container image as AKS. This chart is `mcr.microsoft.com/azuremonitor/containerinsights/cidev/ama-metrics-arc`.
- You can follow the same backdoor deployment instructions as AKS for 24 hours for your own testing. Afterwards, the actual extension will need to be enabled and disabled again to get the token refreshed.

## Dev CI/CD
- The chart is then deployed to the CI/CD cluster [`ci-dev-arc-wcus`](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-dev-arc-wcus/overview) upon any merge to `main`.
- This is not deployed through the backdoor but to a separate release train for our extension using a PUT call and a service principal registered with our extension. This is because deploying through the backdoor will stop working after 24 hours due to the needed MSI token refresh.
- The extension then needs to be upgraded to the explicit version since the semver major, minor, and patch numbers will still be the same for merges between releases.
- Metrics sent to [`ci-dev-arc-amw`](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/microsoft.monitor/accounts/ci-dev-arc-amw/resourceOverviewId) and are viewed through [this Grafana instance](https://ci-dev-aks-eus-graf-aje3bpf7d3ctc9h6.eus.grafana.azure.com/d/efa86fd1d0c121a26444b636a3f56738/kubernetes-compute-resources-cluster?orgId=1&refresh=1m&var-datasource=Managed_Prometheus_ci-dev-arc-amw&var-cluster=ci-dev-arc-wcus), the same as our dev AKS 3P cluster.

## Prod Build
- Our release pipeline will build the ARC chart: `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/ama-metrics-arc`. This is then deployed to the CI/CD cluster [`ci-prod-arc-wcus`](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-prod-arc-wcus/overview).

## Release
- The release uses the same method of PUT calls as the CI/CD clusters, but will use the `stable` release train and rollout to all regions using SDP.
- We will manually trigger the first rollout to canary. The pipeline will wait 24 hours and repeat to each stage of regions.