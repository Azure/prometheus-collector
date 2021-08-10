- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)


# Build Instructions

#### Step 1 : cd into ```otelcollector/opentelemetry-collector-builder``` directory
#### Step 2 : make
#### Step 3 : cd into ```otelcollector``` directory and do ```docker build -t  <myregistry>/<myrepository>:<myimagetag> .```
Example : 
```docker build -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev:myprometheuscollector-1 --build-arg IMAGE_TAG=myprometheuscollector-1 .```
#### Step 4 : docker push <myregistry>/<myrepository>:<myimagetag> (after successfully logging into registry/repository)
Example : 
```docker push containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev:myprometheuscollector-1```

# Release Process

## Github Workflows
Each commit to a PR for `main` or a merge into `main` generates a build. 
  - Image tag is `prometheus-collector-{branch name}-{helm chart version}-{Pacific time date}-{commit id}`
  - Helm chart image tag is `prometheus-collector-chart-{branch name}-{helm chart version}-{Pacific time date}-{commit id}`
  - Each merge commit is tagged with: `v{helm chart version}-{Pacific time date}-{commit id}`


## Release Pull Requests
- **PR 1**: Bump the version in the VERSION file following semantic versioning.
    - If you know your PR with the last feature changes will be the last one before the release, you can do this then.
    - **Build 1**: The `values.yaml` and `Chart.yaml` templates for the HELM chart will automatically be replaced with the image tag and the HELM chart version during the workflow build.
- **PR 2**: Get the commit SHA from the commit used for **Build 1** and run `./release.sh` script: 

  - ```
    commit_id=<commit id> ./release.sh
    ```
  - This changes the image and helm chart tags in all the README files that contain it.
- **PR 3**: Make a PR to update the Geneva docs with any changes made in `/otelcollector/deploy/eng.ms/docs/Prometheus`
