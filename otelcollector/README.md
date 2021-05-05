Main branch builds:![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push) PR builds: ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)

# Build Instructions

#### Step 1 : cd into ```otelcollector/opentelemetry-collector-builder``` directory
#### Step 2 : make
#### Step 3 : cd into ```otelcollector``` directory and do ```docker build -t  <myregistry>/<myrepository>:<myimagetag> .```
Example : 
```docker build -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev:myprometheuscollector-1 --build-arg IMAGE_TAG=myprometheuscollector-1 .```
#### Step 4 : docker push <myregistry>/<myrepository>:<myimagetag> (after successfully logging into registry/repository)
Example : 
```docker push -t containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev:myprometheuscollector-1```
