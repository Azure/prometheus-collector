#!/bin/bash
export HELM_EXPERIMENTAL_OCI=1

RELEASE_TRAINS_PATH='"'$(echo "$RELEASE_TRAINS_STABLE_PATH" | sed 's/,/","/g')'"'
REGIONS_BATCH='"'$(echo "$REGISTER_REGIONS_BATCH" | sed 's/,/","/g')'"'
IS_CUSTOMER_HIDDEN=$IS_CUSTOMER_HIDDEN
CHART_VERSION=${CHART_VERSION}

PACKAGE_CONFIG_NAME="${PACKAGE_CONFIG_NAME:-microsoft.azuremonitor.containers-prom030823}"
API_VERSION="${API_VERSION:-2021-05-01}"
METHOD="${METHOD:-put}"
REGISTRY_PATH="https://mcr.microsoft.com/azuremonitor/containerinsights/ciprod/ama-metrics-arc"

CANARY_BATCH="\"eastus2euap\",\"centraluseuap\"
SMALL_REGION="$CANARY_BATCH,\"westcentralus\""
MEDIUM_REGION="$SMALL_REGION,\"westeurope\""
LARGE_REGION="$MEDIUM_REGION,\"westus2\""
BATCH_1_REGIONS="$LARGE_REGION,\"eastus\",\"southcentralus\",\"uksouth\",\"southeastasia\",\"koreacentral\",\"centralus\",\"japaneast\""
BATCH_2_REGIONS="$BATCH_1_REGIONS,\"australiaeast\",\"northeurope\",\"eastus2\",\"francecentral\",\"westus\",\"northcentralus\",\"eastasia\",\"westus3\""

RELEASE_TRAIN_DEV="\"dev\""
RELEASE_TRAIN_STAGING="$RELEASE_TRAIN_DEV,\"staging\""
RELEASE_TRAIN_STABLE="$RELEASE_TRAIN_STAGING,\"stable\""

if [ -z "$REGIONS_BATCH" ]; then
    echo "-e error release regions must be provided "
    exit 1
fi
if [ -z "$IS_CUSTOMER_HIDDEN" ]; then
    echo "-e error is_customer_hidden must be provided "
    exit 1
fi
if [ -z "$CHART_VERSION" ]; then
    echo "-e error chart version must be provided "
    exit 1
fi

echo "Pulling chart from MCR:${REGISTRY_PATH}"
helm chart pull ${REGISTRY_PATH}:${CHART_VERSION}
if [ $? -eq 0 ]; then
  echo "Pulling chart from MCR:${REGISTRY_PATH}:${CHART_VERSION} completed successfully."
else
  echo "-e error Pulling chart from MCR:${REGISTRY_PATH}:${CHART_VERSION} failed. Please review Ev2 pipeline logs for more details on the error."
  exit 1
fi   

echo "Start arc extension release stage ${RELEASE_STAGE}, REGISTER_REGIONS is $REGISTER_REGIONS_BATCH, RELEASE_TRAINS are $RELEASE_TRAINS_PREVIEW_PATH, $RELEASE_TRAINS_STABLE_PATH, PACKAGE_CONFIG_NAME is $PACKAGE_CONFIG_NAME, API_VERSION is $API_VERSION, METHOD is $METHOD"

# Create JSON request body
cat <<EOF > "request.json"
{
    "artifactEndpoints": [
        {
            "Regions": [
                $REGISTER_REGIONS_BATCH
            ],
            "Releasetrains": [
                "$RELEASE_TRAINS_PATH"
            ],
            "FullPathToHelmChart": "$REGISTRY_PATH",
            "ExtensionUpdateFrequencyInMinutes": 60,
            "IsCustomerHidden": $IS_CUSTOMER_HIDDEN,
            "ReadyforRollout": true,
            "RollbackVersion": null,
            "PackageConfigName": "$PACKAGE_CONFIG_NAME"
        },
    ]
}
EOF

# Send Request
SUBSCRIPTION=${ADMIN_SUBSCRIPTION_ID}
RESOURCE_AUDIENCE=${RESOURCE_AUDIENCE}

echo "Request parameter preparation, SUBSCRIPTION is $SUBSCRIPTION, RESOURCE_AUDIENCE is $RESOURCE_AUDIENCE, CHART_VERSION is $CHART_VERSION, SPN_CLIENT_ID is $SPN_CLIENT_ID, SPN_TENANT_ID is $SPN_TENANT_ID"

# msi is not supported yet since msi always linked to an Azure Resource
echo "Login cli using spn"
az login --service-principal --username=${SPN_CLIENT_ID} --password=${SPN_SECRET} --tenant=${SPN_TENANT_ID}
if [ $? -eq 0 ]; then
  echo "Logged in successfully with spn"
else
  echo "-e error failed to login to az with managed identity credentials"
  exit 1
fi    

ACCESS_TOKEN=$(az account get-access-token --resource $RESOURCE_AUDIENCE --query accessToken -o json)
if [ $? -eq 0 ]; then
  echo "get access token from resource:$RESOURCE_AUDIENCE successfully."
else
  echo "-e error get access token from resource:$RESOURCE_AUDIENCE failed. Please review Ev2 pipeline logs for more details on the error."
  exit 1
fi   
ACCESS_TOKEN=$(echo $ACCESS_TOKEN | tr -d '"' | tr -d '"\r\n')

ARC_API_URL="https://eastus2euap.dp.kubernetesconfiguration.azure.com"
EXTENSION_NAME="microsoft.azuremonitor.containers.metrics"

echo "start send request"
az rest --method $METHOD --headers "{\"Authorization\": \"Bearer $ACCESS_TOKEN\", \"Content-Type\": \"application/json\"}" --body @request.json --uri $ARC_API_URL/subscriptions/$SUBSCRIPTION/extensionTypeRegistrations/$EXTENSION_NAME/versions/$CHART_VERSION?api-version=$API_VERSION
if [ $? -eq 0 ]; then
  echo "arc extension registered successfully"
else
  echo "-e error failed to register arc extension"
  exit 1
fi