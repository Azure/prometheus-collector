#!/bin/bash

if [ -z "$workflow_run" ]
then
      echo "workflow_run is not set. Re-run as: workflow_run=<number> ./release.sh"
      exit
fi

version=$(cat ./VERSION)
date=$(TZ=America/Los_Angeles date +%m-%d-%Y)

image_replacement="prometheus-collector-main-$date-$workflow_run"
echo "Replacing the image tag with:      $image_replacement"
sed -i "s/prometheus-collector-main-[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+/$image_replacement/g" ./deploy/prometheus-collector.yaml
sed -i "s/prometheus-collector-main-[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+/$image_replacement/g" ./deploy/chart/prometheus-collector/README.md
sed -i "s/prometheus-collector-main-[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+/$image_replacement/g" ./deploy/eng.ms/docs/Prometheus/chartvalues.md

helm_replacement="prometheus-collector-chart-main-$version-$date-$workflow_run"
echo "Replacing the HELM chart tag with: $helm_replacement"
sed -i "s/prometheus-collector-chart-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+/$helm_replacement/g" ./deploy/eng.ms/docs/Prometheus/PromMDMTutorial2DeployAgentHELM.md
sed -i "s/prometheus-collector-chart-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+/$helm_replacement/g" ./deploy/chart/prometheus-collector/README.md