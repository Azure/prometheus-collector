#!/bin/bash
export HELM_EXPERIMENTAL_OCI=1

IS_CUSTOMER_HIDDEN="${IS_CUSTOMER_HIDDEN:-true}"
CHART_VERSION="${CHART_VERSION}"
PACKAGE_CONFIG_NAME="${PACKAGE_CONFIG_NAME:-Microsoft.AzureMonitor.Containers.Metrics-Prom041823}"
API_VERSION="${API_VERSION:-2021-05-01}"
METHOD="${METHOD:-put}"
REGISTRY_PATH="${REGISTRY_PATH:-mcr.microsoft.com/azuremonitor/containerinsights/ciprod/ama-metrics-arc}"
SUBSCRIPTION="${ADMIN_SUBSCRIPTION_ID:-b9842c7c-1a38-4385-8f39-a51314758bcf}"
RESOURCE_AUDIENCE="${RESOURCE_AUDIENCE:-c699bf69-fb1d-4eaf-999b-99e6b2ae4d85}"
SPN_CLIENT_ID="${SPN_CLIENT_ID:-9a4c55e9-576a-450a-88bd-53bd634db38d}"
SPN_TENANT_ID="${SPN_TENANT_ID:-72f988bf-86f1-41af-91ab-2d7cd011db47}"
ARC_API_URL="${ARC_API_URL:-https://eastus2euap.dp.kubernetesconfiguration.azure.com}"
EXTENSION_NAME="${EXTENSION_NAME:-microsoft.azuremonitor.containers.metrics}"

CANARY_BATCH="\"eastus2euap\",\"centraluseuap\""
SMALL_REGION="$CANARY_BATCH,\"westcentralus\""
MEDIUM_REGION="$SMALL_REGION,\"eastus2\""
LARGE_REGION="$MEDIUM_REGION,\"eastus\""
BATCH_1_REGIONS="$LARGE_REGION,\"westus2\",\"southcentralus\",\"southeastasia\",\"koreacentral\",\"centralus\",\"japaneast\",\"australiaeast\""
BATCH_2_REGIONS="$BATCH_1_REGIONS,\"northeurope\",\"uksouth\",\"francecentral\",\"westus\",\"northcentralus\",\"eastasia\",\"westus3\",\"usgovvirginia\""
BATCH_3_REGIONS="$BATCH_2_REGIONS,\"westeurope\",\"canadacentral\",\"canadaeast\",\"australiasoutheast\",\"centralindia\",\"switzerlandnorth\",\"germanywestcentral\",\"norwayeast\",\"germanynorth\""
BATCH_4_REGIONS="$BATCH_3_REGIONS,\"japanwest\",\"ukwest\",\"koreasouth\",\"southafricanorth\",\"southindia\",\"brazilsouth\",\"uaenorth\",\"norwaywest\",\"swedensouth\""

RELEASE_TRAIN_PIPELINE="\"pipeline\""
RELEASE_TRAIN_STAGING="$RELEASE_TRAIN_PIPELINE,\"staging\""
RELEASE_TRAIN_STABLE="$RELEASE_TRAIN_STAGING,\"stable\""
RELEASE_TRAINS="$RELEASE_TRAIN_STABLE"

if [ "$REGIONS_BATCH_NAME" == "canary" ]; then
  REGIONS_BATCH=$CANARY_BATCH
elif [ "$REGIONS_BATCH_NAME" == "small" ]; then
  REGIONS_BATCH=$SMALL_REGION
elif [ "$REGIONS_BATCH_NAME" == "medium" ]; then
  REGIONS_BATCH=$MEDIUM_REGION
elif [ "$REGIONS_BATCH_NAME" == "large" ]; then
  REGIONS_BATCH=$LARGE_REGION
elif [ "$REGIONS_BATCH_NAME" == "batch1" ]; then
  REGIONS_BATCH=$BATCH_1_REGIONS
elif [ "$REGIONS_BATCH_NAME" == "batch2" ]; then
  REGIONS_BATCH=$BATCH_2_REGIONS
elif [ "$REGIONS_BATCH_NAME" == "batch3" ]; then
  REGIONS_BATCH=$BATCH_3_REGIONS
elif [ "$REGIONS_BATCH_NAME" == "batch4" ]; then
  REGIONS_BATCH=$BATCH_4_REGIONS
fi

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
helm pull oci://${REGISTRY_PATH} --version ${CHART_VERSION}
if [ $? -eq 0 ]; then
  echo "Pulling chart from ${REGISTRY_PATH}:${CHART_VERSION} completed successfully."
else
  echo "-e error Pulling chart from ${REGISTRY_PATH}:${CHART_VERSION} failed. Please review Ev2 pipeline logs for more details on the error."
  exit 1
fi

echo "Start arc extension release. REGISTER_REGIONS is $REGIONS_BATCH, RELEASE_TRAINS are $RELEASE_TRAINS, PACKAGE_CONFIG_NAME is $PACKAGE_CONFIG_NAME, API_VERSION is $API_VERSION, METHOD is $METHOD"

# Create JSON request body
cat <<EOF > "request.json"
{
    "artifactEndpoints": [
        {
            "Regions": [
                $REGIONS_BATCH
            ],
            "Releasetrains": [
                $RELEASE_TRAINS
            ],
            "FullPathToHelmChart": "https://${REGISTRY_PATH}",
            "ExtensionUpdateFrequencyInMinutes": 60,
            "IsCustomerHidden": $IS_CUSTOMER_HIDDEN,
            "ReadyforRollout": true,
            "RollbackVersion": null,
            "PackageConfigName": "$PACKAGE_CONFIG_NAME"
        }
    ]
}
EOF

# Send Request
echo "Request parameter preparation, SUBSCRIPTION is $SUBSCRIPTION, RESOURCE_AUDIENCE is $RESOURCE_AUDIENCE, CHART_VERSION is $CHART_VERSION, SPN_CLIENT_ID is $SPN_CLIENT_ID, SPN_TENANT_ID is $SPN_TENANT_ID"

# MSI is not supported
echo "Login cli using spn"
az login --service-principal --username=$SPN_CLIENT_ID --password=${SPN_SECRET} --tenant=$SPN_TENANT_ID
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

echo "start send request"
az rest --method $METHOD --headers "{\"Authorization\": \"Bearer $ACCESS_TOKEN\", \"Content-Type\": \"application/json\"}" --body @request.json --uri $ARC_API_URL/subscriptions/$SUBSCRIPTION/extensionTypeRegistrations/$EXTENSION_NAME/versions/$CHART_VERSION?api-version=$API_VERSION
if [ $? -eq 0 ]; then
  echo "arc extension registered successfully"
else
  echo "-e error failed to register arc extension"
  exit 1
fi