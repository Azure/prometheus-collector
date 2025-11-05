#!/bin/bash
set -x
$x &> /dev/null
results_dir="${RESULTS_DIR:-/tmp/results}"

validateArcConfTestParameters() {
  if [ -z $SUBSCRIPTION_ID ]; then
     echo "ERROR: parameter SUBSCRIPTION_ID is required." > ${results_dir}/error
  fi

  if [ -z $RESOURCE_GROUP ]; then
    echo "ERROR: parameter RESOURCE_GROUP is required." > ${results_dir}/error
  fi

  if [ -z $CLUSTER_NAME ]; then
    echo "ERROR: parameter CLUSTER_NAME is required." > ${results_dir}/error
  fi
}

login_to_azure() {
  if [[ -v SYSTEM_ACCESSTOKEN && -n "$SYSTEM_ACCESSTOKEN" &&
         -v SERVICE_CONNECTION_ID && -n "$SERVICE_CONNECTION_ID" &&
         -v SYSTEM_OIDCREQUESTURI && -n "$SYSTEM_OIDCREQUESTURI" ]]; then
    export OIDC_REQUEST_URL="${SYSTEM_OIDCREQUESTURI}?api-version=7.1&serviceConnectionId=${SERVICE_CONNECTION_ID}"
    echo "OIDC_REQUEST_URL= $OIDC_REQUEST_URL"

    FED_TOKEN=$(curl -s -H "Content-Length: 0" -H "Content-Type: application/json" -H "Authorization: Bearer $SYSTEM_ACCESSTOKEN" -X POST $OIDC_REQUEST_URL | jq -r '.oidcToken')
    echo "FED_TOKEN= $FED_TOKEN"

    echo "logging in using Federated Identity"
    az login --service-principal -u $FED_CLIENT_ID  --tenant $TENANT_ID --allow-no-subscriptions --federated-token $FED_TOKEN 2> ${results_dir}/error || python3 setup_failure_handler.py
    echo "setting subscription: ${SUBSCRIPTION_ID} as default subscription"
    az account set -s $SUBSCRIPTION_ID
  elif [[ -v WORKLOAD_CLIENT_ID && -n "$WORKLOAD_CLIENT_ID" ]]; then
    echo "logging in using Workload Identity"
    az login --identity --username $WORKLOAD_CLIENT_ID
    echo "setting subscription: ${SUBSCRIPTION_ID} as default subscription"
    az account set -s $SUBSCRIPTION_ID
  else
    echo "ERROR: Unable to login to Azure. Missing Federated Identity or Workload Identity parameters." > ${results_dir}/error
  fi
}

addArcConnectedK8sExtension() {
  echo "adding Arc K8s connectedk8s extension"
  az extension add --name connectedk8s 2> ${results_dir}/error
}

waitForResourcesReady() {
  ready=false
  max_retries=60
  sleep_seconds=10
  NAMESPACE=$1
  RESOURCETYPE=$2
  RESOURCE=$3
  # if resource not specified, set to --all
  if [ -z $RESOURCE ]; then
      RESOURCE="--all"
  fi
  for i in $(seq 1 $max_retries)
  do
  allPodsAreReady=$(kubectl wait --for=condition=Ready ${RESOURCETYPE} ${RESOURCE} --namespace ${NAMESPACE})
  if [ $? -ne 0 ]; then
      echo "waiting for the resource:${RESOURCE} of the type:${RESOURCETYPE} in namespace:${NAMESPACE} to be ready state, iteration:${i}"
      sleep ${sleep_seconds}
  else
      echo "resource:${RESOURCE} of the type:${RESOURCETYPE} in namespace:${NAMESPACE} in ready state"
      ready=true
      break
  fi
  done

  echo "waitForResourcesReady state: $ready"
}

waitForArcK8sClusterCreated() {
  connectivityState=false
  max_retries=60
  sleep_seconds=10
  for i in $(seq 1 $max_retries)
  do
    echo "iteration: ${i}, clustername: ${CLUSTER_NAME}, resourcegroup: ${RESOURCE_GROUP}"
    clusterState=$(az connectedk8s show --name $CLUSTER_NAME --resource-group $RESOURCE_GROUP --query connectivityStatus -o json)
    clusterState=$(echo $clusterState | tr -d '"' | tr -d '"\r\n')
    echo "cluster current state: ${clusterState}"
    if [ ! -z "$clusterState" ]; then
        if [[ ("${clusterState}" == "Connected") || ("${clusterState}" == "Connecting") ]]; then
          connectivityState=true
          break
        fi
    fi
    sleep ${sleep_seconds}
  done
  echo "Arc K8s cluster connectivityState: $connectivityState"
}

addArcK8sCLIExtension() {
  if [ ! -z "$K8S_EXTENSION_WHL_URL" ]; then
    echo "adding Arc K8s k8s-extension cli extension from whl file path ${K8S_EXTENSION_WHL_URL}"
    az extension add --source $K8S_EXTENSION_WHL_URL -y
  else
    echo "adding Arc K8s k8s-extension cli extension"
    az extension add --name k8s-extension
  fi
}

createArcAMAMetricsExtension() {
  echo "iteration: ${i}, clustername: ${CLUSTER_NAME}, resourcegroup: ${RESOURCE_GROUP}"
  installState=$(az k8s-extension show  --cluster-name $CLUSTER_NAME --resource-group $RESOURCE_GROUP  --cluster-type connectedClusters --name azuremonitor-metrics --query provisioningState -o json)
  installState=$(echo $installState | tr -d '"' | tr -d '"\r\n')
  echo "extension install state: ${installState}"
  if [ ! -z "$installState" ]; then
      if [ "${installState}" == "Succeeded" ]; then
        installedState=true
        return
      fi
  fi

  echo "creating extension type: Microsoft.AzureMonitor.Containers.Metrics"
  basicparameters="--cluster-name $CLUSTER_NAME --resource-group $RESOURCE_GROUP --cluster-type connectedClusters --extension-type Microsoft.AzureMonitor.Containers.Metrics --scope cluster --name azuremonitor-metrics --auto-upgrade-minor-version false"
  if [ ! -z "$AMA_METRICS_ARC_RELEASE_TRAIN" ]; then
      basicparameters="$basicparameters  --release-train $AMA_METRICS_ARC_RELEASE_TRAIN"
  fi
  if [ ! -z "$AMA_METRICS_ARC_VERSION" ]; then
      basicparameters="$basicparameters  --version $AMA_METRICS_ARC_VERSION"
  fi
  
  az k8s-extension create $basicparameters
}

showArcAMAMetricsExtension() {
  echo "Arc AMA Metrics extension status"
  az k8s-extension show  --cluster-name $CLUSTER_NAME --resource-group $RESOURCE_GROUP  --cluster-type connectedClusters --name azuremonitor-metrics
}

waitForAMAMetricsExtensionInstalled() {
  installedState=false
  max_retries=60
  sleep_seconds=10
  for i in $(seq 1 $max_retries)
  do
    echo "iteration: ${i}, clustername: ${CLUSTER_NAME}, resourcegroup: ${RESOURCE_GROUP}"
    installState=$(az k8s-extension show  --cluster-name $CLUSTER_NAME --resource-group $RESOURCE_GROUP  --cluster-type connectedClusters --name azuremonitor-metrics --query provisioningState -o json)
    installState=$(echo $installState | tr -d '"' | tr -d '"\r\n')
    echo "extension install state: ${installState}"
    if [ ! -z "$installState" ]; then
        if [ "${installState}" == "Succeeded" ]; then
          installedState=true
          break
        fi
    fi
    sleep ${sleep_seconds}
  done
}

getAMAMetricsAMWQueryEndpoint() {
  amw=$(az k8s-extension show --cluster-name ${CLUSTER_NAME} --resource-group ${RESOURCE_GROUP} --cluster-type connectedClusters --name azuremonitor-metrics --query configurationSettings -o json)
  echo "Azure Monitor Metrics extension amw: $amw"
  amw=$(echo $amw | tr -d '"\r\n {}')
  amw="${amw##*:}"
  echo "extension amw: ${amw}"
  queryEndpoint=$(az monitor account show --ids ${amw} --query "metrics.prometheusQueryEndpoint" -o json | tr -d '"\r\n')
  echo "queryEndpoint: ${queryEndpoint}"
  export AMW_QUERY_ENDPOINT=$queryEndpoint
}

deleteArcAMAMetricsExtension() {
  az k8s-extension delete --name azuremonitor-metrics \
  --cluster-type connectedClusters \
  --cluster-name $CLUSTER_NAME \
  --resource-group $RESOURCE_GROUP --yes
}

# saveResults prepares the results for handoff to the Sonobuoy worker.
# See: https://github.com/vmware-tanzu/sonobuoy/blob/master/docs/plugins.md
saveResults() {
  cd ${results_dir}

    # Sonobuoy worker expects a tar file.
  tar czf results.tar.gz *

  # Signal to the worker that we are done and where to find the results.
  printf ${results_dir}/results.tar.gz > ${results_dir}/done
}

# Ensure that we tell the Sonobuoy worker we are done regardless of results.
trap saveResults EXIT

validateArcConfTestParameters

login_to_azure

addArcConnectedK8sExtension

waitForResourcesReady azure-arc pods

waitForArcK8sClusterCreated

addArcK8sCLIExtension

createArcAMAMetricsExtension

showArcAMAMetricsExtension

waitForAMAMetricsExtensionInstalled

getAMAMetricsAMWQueryEndpoint

sleep 5m
cd ginkgo-test-binaries
files=("containerstatus.test" "prometheusui.test" "operator.test" "querymetrics.test" "livenessprobe.test")
for file in "${files[@]}"; do
  AMW_QUERY_ENDPOINT=$AMW_QUERY_ENDPOINT ginkgo -p -r --junit-report=${results_dir}/results-$file.xml --keep-going --label-filter='!/./ || arc-extension' -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com" "$file"
done
cd ..

deleteArcAMAMetricsExtension