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

# Release Process

## Azure Pipelines
Each commit to a PR for `main` or a merge into `main` generates a build with [Azure Pipelines](https://github-private.visualstudio.com/azure/_build?definitionId=440). 

The following are the formats for the image tags, helm chart versions, and git tags:
  - Linux image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  - Windows image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}-win`
  - Chart version/image tag: `{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  - Git Tag: `v{helm chart semver}-{branch}-{date in pacific timezone}-{commit id}`
  
Each merge into `main` will push the image to the public mcr and deploy to the dev clusters. If the deployed pods do not have a `Running` state within the 5 minute timeout, the deploy step will fail, so we will know the clusters did not get updated. The reason for why the deployment failed would need to be investigated and fixed. Then you can go back to the build and click `Rerun failed jobs` to try to re-deploy without having to re-build and push the images.

## Release Process
- **PR 1**: Bump the version in the VERSION file following semantic versioning.
    - If you know your PR with the last feature changes will be the last one before the release, you can do this then.
    - **Build 1**: The `values.yaml` and `Chart.yaml` templates for the HELM chart will automatically be replaced with the image tag and the HELM chart version during the CI/CD build.
- **Release**: Create a release in [ADO](https://github-private.visualstudio.com/azure/_release?_a=releases&view=mine&definitionId=79).
    - Select `Create release`, then choose the build version which should be the same as the image tag.
    - This pushes the linux, windows, and chart builds to the prod ACR which is synchronized with the prod MCR.
    - Once pushed, the chart is deployed on the prod clusters.
- **PR 2**: Get the chart semver or container image tag from the commit used for **Build 1**, and also for the previously released version and run `./release.sh` script: 

  - ```
    previous_semver=<semver_for_currently_relased_chart> current_semver=<semver_for_to_be_relased_chart_from_pr_1_above> ./release.sh
    ex;- previous_semver=0.0.5-main-10-11-2021-4a9de500 current_semver=0.0.6-main-10-12-2021-5c34d764 ./release.sh
    ```
  - This changes the image and helm chart tags in all the README files that contain it.
- **PR 3**: Make a PR to update the [Geneva docs](https://msazure.visualstudio.com/One/_git/EngSys-MDA-GenevaDocs?path=%2Fdocumentation%2Fmetrics%2FPrometheus&version=GBmaster&_a=contents) with any changes made in `/otelcollector/deploy/eng.ms/docs/Prometheus`