#!/bin/bash
export HELM_EXPERIMENTAL_OCI=1

# Define regions
REGION1='"centraluseuap","eastus2euap"'
REGION2='"westcentralus","francecentral"'
REGION3='"uksouth"'
REGION4='"westeurope"'
BATCH1='"westus2","centralus","southeastasia","southcentralus","australiaeast","japaneast","koreacentral"'
BATCH2='"eastus2","northeurope","westus","eastasia","westus3","northcentralus"'
BATCH3='"eastus","canadacentral","centralindia","canadaeast","norwayeast","germanywestcentral","australiasoutheast","switzerlandnorth","francesouth","swedencentral","germanynorth"'
BATCH4='"southindia","ukwest","southafricanorth","japanwest","koreasouth","brazilsouth","uaenorth","jioindiawest","norwaywest"'
BATCH5='"italynorth","israelcentral","polandcentral","australiacentral","australiacentral2","southafricawest","westindia","switzerlandwest","spaincentral","indonesiacentral","taiwannorth","mexicocentral","newzealandnorth","brazilsoutheast"'
BATCH6='"chilecentral","malaysiawest","austriaeast","belgiumcentral","denmarkeast","israelnorthwest","malaysiasouth"'
FAIRFAX='"usgovvirginia","usgovtexas","usgovarizona"'
MOONCAKE='"chinaeast2","chinanorth2","chinaeast3","chinanorth3"'

# Determine the location
if [ "$LOCATION" = "centraluseuap" ]; then
    echo "Registering in centraluseuap, eastus2euap"
    REGIONS_LIST="$REGION1"
elif [ "$LOCATION" = "westcentralus" ]; then
    echo "Registering in westcentralus, francecentral"
    REGIONS_LIST="$REGION2"
elif [ "$LOCATION" = "uksouth" ]; then
    echo "Registering in uksouth"
    REGIONS_LIST="$REGION3"
elif [ "$LOCATION" = "westeurope" ]; then
    echo "Registering in westeurope"
    REGIONS_LIST="$REGION4"
elif [ "$LOCATION" = "westus2" ]; then
    echo "Registering in batch1"
    REGIONS_LIST="$BATCH1"
elif [ "$LOCATION" = "eastus2" ]; then
    echo "Registering in batch2"
    REGIONS_LIST="$BATCH2"
elif [ "$LOCATION" = "eastus" ]; then
    echo "Registering in batch3"
    REGIONS_LIST="$BATCH3"
elif [ "$LOCATION" = "southindia" ]; then
    echo "Registering in all batch4"
    REGIONS_LIST="$BATCH4"
elif [ "$LOCATION" = "italynorth" ]; then
    echo "Registering in all batch5"
    REGIONS_LIST="$BATCH5"
elif [ "$LOCATION" = "chilecentral" ]; then
    echo "Registering in all batch6"
    REGIONS_LIST="$BATCH6"
elif [ "$LOCATION" = "usgovvirginia" ]; then
    echo "Registering in Fairfax regions"
    REGIONS_LIST="$FAIRFAX"
elif [ "$LOCATION" = "chinaeast2" ]; then
    echo "Registering in Mooncake regions"
    REGIONS_LIST="$MOONCAKE"
else
    echo "Invalid location, not part of SDP regions. Exiting."
    exit 1
fi

if [ -z "$REGIONS_LIST" ]; then
    echo "-e error release regions must be provided "
    exit 1
fi
if [ -z $EXTENSION_NAME ]; then
    echo "-e error extension name must be provided "
    exit 1
fi
if [ -z "$PACKAGE_CONFIG_NAME" ]; then
    echo "-e error package config name must be provided "
    exit 1
fi
if [ -z "$HELM_CHART_ENDPOINT" ]; then
    echo "-e error helm chart endpoint must be provided "
    exit 1
fi
if [ -z "$IS_CUSTOMER_HIDDEN" ]; then
    echo "-e error is_customer_hidden must be provided "
    exit 1
fi
if [ -z "$ARC_API_URL" ]; then
    echo "-e error arc api url must be provided "
    exit 1
fi
if [ -z "$API_VERSION" ]; then
    echo "-e error api version must be provided "
    exit 1
fi
if [ -z "$SUBSCRIPTION" ]; then
    echo "-e error subscription must be provided "
    exit 1
fi
if [ -z "$RELEASE_TRAIN" ]; then
    echo "-e error release train must be provided "
    exit 1
fi
if [ -z "$CHART_VERSION" ]; then
    echo "-e error chart version must be provided "
    exit 1
fi
if [ -z "$METHOD" ]; then
    echo "-e error method must be provided "
    exit 1
fi
if [ -z "$RESOURCE_AUDIENCE" ]; then
    echo "-e error resource audience must be provided "
    exit 1
fi

RELEASE_TRAIN_PIPELINE="\"pipeline\""
RELEASE_TRAIN_STAGING="$RELEASE_TRAIN_PIPELINE,\"staging\""
RELEASE_TRAIN_STABLE="$RELEASE_TRAIN_STAGING,\"stable\""
RELEASE_TRAINS="$RELEASE_TRAIN_STABLE"

echo "Pulling chart from MCR:${HELM_CHART_ENDPOINT} with version ${CHART_VERSION}"
helm pull oci://${HELM_CHART_ENDPOINT} --version ${CHART_VERSION}
if [ $? -eq 0 ]; then
  echo "Pulling chart from ${HELM_CHART_ENDPOINT}:${CHART_VERSION} completed successfully."
else
  echo "-e error Pulling chart from ${HELM_CHART_ENDPOINT}:${CHART_VERSION} failed. Please review Ev2 pipeline logs for more details on the error."
  exit 1
fi

echo "Start arc extension release. REGISTER_REGIONS is $REGIONS_LIST, RELEASE_TRAINS are $RELEASE_TRAINS, PACKAGE_CONFIG_NAME is $PACKAGE_CONFIG_NAME, API_VERSION is $API_VERSION, METHOD is $METHOD"


# Create JSON request body
cat <<EOF > "request.json"
[
    {
        "Regions": [$REGIONS_LIST],
        "Releasetrains": [
            "$RELEASE_TRAINS"
        ],
        "FullPathToHelmChart": "$HELM_CHART_ENDPOINT",
        "ExtensionUpdateFrequencyInMinutes": 60,
        "autoUpdateImagePath": null,
        "IsCustomerHidden": $IS_CUSTOMER_HIDDEN,
        "ReadyforRollout": true,
        "RollbackVersion": null,
        "PackageConfigName": "$PACKAGE_CONFIG_NAME"
    }
]
EOF

echo "Request JSON:"
cat request.json

# Send Request
echo "Request parameter preparation, SUBSCRIPTION is $SUBSCRIPTION, RESOURCE_AUDIENCE is $RESOURCE_AUDIENCE, CHART_VERSION is $CHART_VERSION"

# Retries needed due to: https://stackoverflow.microsoft.com/questions/195032
n=0
signInExitCode=-1
until [ "$n" -ge 5 ]
do
   az login --identity --allow-no-subscriptions && signInExitCode=0 && break
   n=$((n+1))
   sleep 15
done

if [ $signInExitCode -eq 0 ]; then
  echo "Logged in successfully"
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
