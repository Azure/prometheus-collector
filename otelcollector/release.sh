#!/bin/bash

if [ -z "$commit_id" ]
then
  echo "commit_id is not set. Re-run as: commit_id=<number> ./release.sh"
  exit
fi

if [ ${#commit_id} -lt 8 ]
then
  echo "commit_id is less than 8 characters. Re-run with longer commit id."
fi

if [ ${#commit_id} -gt 8 ]
then
  commit_id=$(echo $commit_id | cut -c 1-8)
  echo "truncating commit_id to: ${commit_id}"
fi

version=$(cat ./VERSION)
date=$(TZ=America/Los_Angeles date +%m-%d-%Y)

image_regex="prometheus-collector-main-[a-zA-Z0-9.-]\+"
image_replacement="prometheus-collector-main-$version-$date-$commit_id"
echo "Replacing the image tag with:      $image_replacement"
sed -i "s/$image_regex/$image_replacement/g" ./deploy/prometheus-collector.yaml
sed -i "s/$image_regex/$image_replacement/g" ./deploy/chart/prometheus-collector/README.md
sed -i "s/$image_regex/$image_replacement/g" ./deploy/eng.ms/docs/Prometheus/chartvalues.md

helm_regex="prometheus-collector-main-[a-zA-Z0-9.-]\+"
helm_replacement="prometheus-collector-chart-main-$version-$date-$commit_id"
echo "Replacing the HELM chart tag with: $helm_replacement"
sed -i "s/$helm_regex/$helm_replacement/g" ./deploy/eng.ms/docs/Prometheus/PromMDMTutorial2DeployAgentHELM.md
sed -i "s/$helm_regex/$helm_replacement/g" ./deploy/chart/prometheus-collector/README.md
