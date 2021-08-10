#!/bin/bash

if [ -z "$commit_id" ]
then
      echo "commit_id is not set. Re-run as: commit_id=<number> ./release.sh"
      exit
fi

version=$(cat ./VERSION)
date=$(TZ=America/Los_Angeles date +%m-%d-%Y)

image_replacement="prometheus-collector-main-$version-$date-$commit_id"
echo "Replacing the image tag with:      $image_replacement"
sed -i "s/prometheus-collector-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[a-zA-Z0-9]\+/$image_replacement/g" ./deploy/prometheus-collector.yaml
sed -i "s/prometheus-collector-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[a-zA-Z0-9]\+/$image_replacement/g" ./deploy/chart/prometheus-collector/README.md
sed -i "s/prometheus-collector-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[a-zA-Z0-9]\+/$image_replacement/g" ./deploy/eng.ms/docs/Prometheus/chartvalues.md

helm_replacement="prometheus-collector-chart-main-$version-$date-$commit_id"
echo "Replacing the HELM chart tag with: $helm_replacement"
sed -i "s/prometheus-collector-chart-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[a-zA-Z0-9]\+/$helm_replacement/g" ./deploy/eng.ms/docs/Prometheus/PromMDMTutorial2DeployAgentHELM.md
sed -i "s/prometheus-collector-chart-main-[0-9]\+.[0-9]\+.[0-9]\+-[0-9]\+-[0-9]\+-[0-9]\+-[a-zA-Z0-9]\+/$helm_replacement/g" ./deploy/chart/prometheus-collector/README.md