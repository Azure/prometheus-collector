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

# Github Actions
Each commit to a PR for `main` or a merge into `main` generates a build. 
  - Image tag is `prometheus-collector-{branch name}-{Pacific time date}-{workflow run number}`
  - Helm chart image tag is `prometheus-collector-chart-{branch name}-{helm chart version}-{Pacific time date}-{workflow run number}`
  - The values.yaml and Chart.yaml templates for the HELM chart will automatically be replaced with the above image tag and the HELM chart version.

# Release
1. Commit last changes into repo.
2. Update files for new versioning and commit:
    - Update VERSION file to what the new HELM chart version should be
    - Get what the workflow run number will be for the upcoming PR and run:
      ```
      workflow_run=<workflow run number> ./release.sh
      ```
      This updates the README files where the image and chart tags are mentioned.
3. Make a PR to update the Geneva docs with any changes made in `/otelcollector/deploy/eng.ms/docs/Prometheus`