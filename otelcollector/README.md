- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)


# Build Instructions

#### Step 1 : cd into ```otelcollector/opentelemetry-collector-builder``` directory
#### Step 2 : make
#### Step 3 : cd into ```otelcollector``` directory and do ```docker build -t  <myregistry>/<myrepository>:<myimagetag> .```
Example : 
```docker build -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:myprometheuscollector-1 .```
#### Step 4 : docker push <myregistry>/<myrepository>:<myimagetag> (after successfully logging into registry/repository)
Example : 
```docker push containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/prometheus-collector/images:myprometheuscollector-1```

# Release Process

## Github Workflows
Each commit to a PR for `main` or a merge into `main` generates a build. 
  - Container image tag and Helm chart version/tag are the same. It is a semVer with the following taxonomy : `{helm chart version}-branch-{Pacific time date}-{commit id}`
  - Each merge commit is tagged with: `v{helm chart version}-branch-{Pacific time date}-{commit id}` (same as container image tag & helm chart semver)
  
Each merge into `main` will push the image to the public mcr and deploy to the dev cluster.


## Release Process
- **PR 1**: Bump the version in the VERSION file following semantic versioning.
    - If you know your PR with the last feature changes will be the last one before the release, you can do this then.
    - **Build 1**: The `values.yaml` and `Chart.yaml` templates for the HELM chart will automatically be replaced with the image tag and the HELM chart version during the workflow build.
- **PR 2**: Get the commit SHA from the commit used for **Build 1** and run `./release.sh` script: 

  - ```
    commit_id=<commit id> ./release.sh
    ```
  - This changes the image and helm chart tags in all the README files that contain it.
- **PR 3**: Make a PR to update the [Geneva docs](https://msazure.visualstudio.com/One/_git/EngSys-MDA-GenevaDocs?path=%2Fdocumentation%2Fmetrics%2FPrometheus&version=GBmaster&_a=contents) with any changes made in `/otelcollector/deploy/eng.ms/docs/Prometheus`
- **Deploy to Prod Cluster**: Go to the Actions tab in Github and run the workflow `deploy-to-prod.yml` to deploy the release to the prod cluster by specifing the tag in the format of `{helm chart version}-branch-{Pacific time date}-{commit id}`.