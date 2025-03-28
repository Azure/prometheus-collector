# Build Instructions

## Build Changes Locally

1. From the [otelcollector/opentelemetry-collector-builder](../opentelemetry-collector-builder/) directory, run `make`. For Windows, run `.\makefile_windows.ps1`.
2. cd into the [otelcollector](../) directory and run:

    ```
    docker build -t <myregistry>/<myrepository>:<myimagetag> -f .\build\{linux|windows}\Dockerfile .
    ```

    Example :

    ```
    docker build -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:myprometheuscollector-1 .
    ```
3. Login in registry and run:
    ```
    docker push `<myregistry>/<myrepository>:<myimagetag>` (after successfully logging into registry/repository)
    ```

    Example : 
    ```
    docker push containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:myprometheuscollector-1
    ```

## Build Changes with Azure Pipelines

Edit the file [.pipelines/azure-pipeline-build.yml](../../.pipelines/azure-pipeline-build.yml) to add your branch to trigger a build when you push to it:

```
trigger:
  batch: true
  branches:
    include:
    - main
    - <add your branch here>
```

PRs to the `main` branch are setup to build images with a tag using the branch name, so you can remove the branch added above before making a PR.

The ADO build number is set to be the same as the image tag.

### Golang Version

The default Golang version for the [1ES hosted pools](https://eng.ms/docs/cloud-ai-platform/developer-services/one-engineering-system-1es/1es-docs) is used. To switch from using the default version, add the following task and specfiy the version such as:

```
- task: GoTool@0
  inputs:
    version: '1.18'
```

# Builds and Releases

## Azure Pipelines
Each commit to a PR for `main` or a merge into `main` generates a build with [Azure Pipelines](https://github-private.visualstudio.com/azure/_build?definitionId=440). 

The following are the formats for the image tags, helm chart versions, and git tags:
  - Linux image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  - Windows image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}-win`
  - Chart version/image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  - Git Tag: `v{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  
Each merge into `main` will push the image to the public mcr and deploy to the dev clusters. If the pods fail to deploy properly, then after fixing the issue on the cluster, you can go back to the build and click `Rerun failed jobs` to try to re-deploy without having to re-build and re-push the images.

## Release Process
- **PR 1**: Bump the version in the VERSION file following semantic versioning.
    - Clean the `.trivyignore` file. Run the scan on the image using the Github Action and add back in the still existing vulnerabilities to the file.
    - Add the latest `addon-token-adapter-linux` and `addon-token-adapter-windows` versions in the values-template.yaml file by checking the version [here](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=%2Fccp%2Fcharts%2Fkube-control-plane%2Ftemplates%2F_images.tpl&_a=contents&version=GBmaster).
    - If you know your PR with the last feature changes will be the last one before the release, you can do this then.
    - **Build 1**: The `values.yaml` and `Chart.yaml` templates for the HELM chart will automatically be replaced with the image tag and the HELM chart version during the CI/CD build.
- **Release**: Create a release in [ADO](https://github-private.visualstudio.com/azure/_release?_a=releases&view=mine&definitionId=79).
    - Select `Create release`, then choose the build version which should be the same as the image tag.
    - This pushes the linux, windows, and chart builds to the prod ACR which is synchronized with the prod MCR.
    - Once pushed, you can manually start the `Deploy to prod clusters` stage to deploy the image to our prod clusters.
- **E2E Conformance Tests**: Ask for our conformance tests to be run in the [Arc Conformance teams channel](https://teams.microsoft.com/l/channel/19%3arlnJ5tIxEMP-Hhe-pRPPp9C6iYQ1CwAelt4zTqyC_NI1%40thread.tacv2/General?groupId=a077ab34-99ea-490c-b204-358d31c24fbe&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47). Follow the instructions in the [Arc test README](../../otelcollector/test/arc-conformance/README.md#testing-on-the-arc-conformance-matrix).
- **PR 2**: Get the chart semver or container image tag from the commit used for **Build 1** and update the release notes with the changelog. Link to a similar PR [here](https://github.com/Azure/prometheus-collector/pull/298)
- **PR 3**: Make changes in AgentBaker for this new image version. Link to similar PR [here](https://github.com/Azure/AgentBaker/pull/2285/files)
- **PR 4**: Update prometheus-addon image in AKS-RP. 
First update the files here - https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-linux.yaml
https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-windows.yaml
https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-ksm.yaml 

  And then generate the _images.tpl file as described here - 
https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/README.md&version=GBmaster&line=51&lineEnd=52&lineStartColumn=1&lineEndColumn=1&lineStyle=plain&_a=contents

  Link to similar PR [here](https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp/pullrequest/8675121)
    - To generate snapshots(required when you update the image and/or chart) –
        - [Re-Render Test Snapshots](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/tests/addon-adapter-charts&version=GBmaster&_a=contents&anchor=re-render-test-snapshots)
        - [Re-Render Addon Chart Snapshots](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/tests/addon-charts/README.md&version=GBmaster&_a=contents)
- **Control Plane Step**
  - **PR 5** Toggle Monitoring clusters for Control Plane image. Link to similar PR [here](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp/pullrequest/11017213?_a=files)
  - Once the deployment complete, verify that the latest candidate image is running on the monitoring metrics clusters using the following dashboard [tile](https://dataexplorer.azure.com/dashboards/2ed37a93-2d75-494c-a072-e34fe60dcdd6?p-_startTime=24hours&p-_endTime=now&tile=3298383e-b7f7-409e-aa6f-8a32cf4ad7e4)
  - Verify data flow for CCP metrics, including for non-default components and metrics only enabled by the configmap in the following sub-steps, via the grafana dashboard using these [dashboards](https://mon-graf-metric-westus-f5hvdcaxc3hjdcdm.wus.grafana.azure.com/dashboards/f/cloud-native/azure-kubernetes-service-monitoring)
      - Verify settings configmap updates take effect for control plane, by applying [test config enabling all ccp targets](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/test/test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml)
      - Verify first minimalIngestionProfile scenario by applying [test config enabling all ccp targets, with MIP false, and empty keep list](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-emptykeep.yaml). All the metrics from all targets should have been scraped.
      - Verify second minimalIngestionProfile scenario by applying [test config enabling all ccp targets, with MIP false, and keep list populated](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-emptykeep.yaml). Only the metrics specified in the test config .yaml for each targets' keeplist should be scraped.
  - Post about the candidate image release for the AKS team to update in the [Managed Prometheus (Azure Monitor Metrics addon)](https://teams.microsoft.com/l/channel/19%3Ae9dd234c60824ac5b494dbe3ec7dcb6b%40thread.skype/Managed%20Prometheus%20(Azure%20Monitor%20Metrics%20addon)?groupId=e121dbfd-0ec1-40ea-8af5-26075f6a731b&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47) teams channel
- **🚀 Remote Write Deployment Steps**
  - **Skip if Not Releasing**  
    - If you’re not releasing a new remote write image, leave the default value as is and skip the following steps.  

  - **Get the Latest Image Tag**  
   - Retrieve the latest image tag from the most recent build in the [Remote Write Release Pipeline](https://msazure.visualstudio.com/InfrastructureInsights/_release?_a=releases&view=mine&definitionId=77).  

  - **Update Prometheus Collector Release**  
   - While creating the [Prometheus Collector Prod Release](https://github-private.visualstudio.com/azure/_release?_a=releases&view=mine&definitionId=79), update the release variables with the new candidate image tag.  

  - **Deploy Remote Write Image**  
   - Click **Deploy** in the `Deploy to Remote Write Clusters` step to release the new remote write image to the monitoring cluster.  

  - **Update Pipeline Variables**  
   - After a successful deployment, update the `RemoteWriteTag` variable in the release pipeline with the new remote write release tag.  
- **Arc**: Start Arc release to Canary regions. The new version will be automatically deployed to each region batch every 24 hours.
