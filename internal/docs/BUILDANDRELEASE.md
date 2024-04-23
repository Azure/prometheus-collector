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
    - Add the latest `addon-token-adapter-linux` and `addon-token-adapter-windows` versions in the values-template.yaml file by checking the version [here](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=%2Fccp%2Fcharts%2Fkube-control-plane%2Ftemplates%2F_images.tpl&_a=contents&version=GBmaster).
    - If you know your PR with the last feature changes will be the last one before the release, you can do this then.
    - **Build 1**: The `values.yaml` and `Chart.yaml` templates for the HELM chart will automatically be replaced with the image tag and the HELM chart version during the CI/CD build.
- **Release**: Create a release in [ADO](https://github-private.visualstudio.com/azure/_release?_a=releases&view=mine&definitionId=79).
    - Select `Create release`, then choose the build version which should be the same as the image tag.
    - This pushes the linux, windows, and chart builds to the prod ACR which is synchronized with the prod MCR.
    - Once pushed, you can manually start the `Deploy to prod clusters` stage to deploy the image to our prod clusters.
- **E2E Conformance Tests**: Ask for our conformance tests to be run in the [Arc Conformance teams channel](https://teams.microsoft.com/l/channel/19%3arlnJ5tIxEMP-Hhe-pRPPp9C6iYQ1CwAelt4zTqyC_NI1%40thread.tacv2/General?groupId=a077ab34-99ea-490c-b204-358d31c24fbe&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47).
- **PR 2**: Get the chart semver or container image tag from the commit used for **Build 1** and update the release notes with the changelog. Link to a similar PR [here](https://github.com/Azure/prometheus-collector/pull/298)
- **PR 3**: Make a PR to update the [Geneva docs](https://msazure.visualstudio.com/One/_git/EngSys-MDA-GenevaDocs?path=%2Fdocumentation%2Fmetrics%2FPrometheus&version=GBmaster&_a=contents) with any changes made in `/otelcollector/deploy/eng.ms/docs/Prometheus`
- **PR 4**: Make changes in AgentBaker for this new image version. Link to similar PR [here](https://github.com/Azure/AgentBaker/pull/2285/files)
- **PR 5**: Update prometheus-addon image in AKS-RP. 
First update the files here - https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-linux.yaml
https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-windows.yaml
https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-ksm.yaml 

  And then generate the _images.tpl file as described here - 
https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/toolkit/versioning/README.md&version=GBmaster&line=51&lineEnd=52&lineStartColumn=1&lineEndColumn=1&lineStyle=plain&_a=contents

  Link to similar PR [here](https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp/pullrequest/8675121)
    - To generate snapshots(required when you update the image and/or chart) –
        - [Re-Render Test Snapshots](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/tests/addon-adapter-charts&version=GBmaster&_a=contents&anchor=re-render-test-snapshots)
        - [Re-Render Addon Chart Snapshots](https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/tests/addon-charts/README.md&version=GBmaster&_a=contents)
- **Arc**: Start Arc release to Canary regions. The new version will be automatically deployed to each region batch every 24 hours.
