#!/bin/bash
#
# Execute this directly in Azure Cloud Shell (https://shell.azure.com) by pasting (SHIFT+INS on Windows, CTRL+V on Mac or Linux)
# the following line (beginning with curl...) at the command prompt and then replacing the args:
# curl https://raw.githubusercontent.com/microsoft/Docker-Provider/prometheus-collector/prometheus-collector/Onboarding/Onboarding-script.sh
# Also download the ARM template that creates MAC, Data Collection Rules and Custom role to query data using grafana
# This script configures required artifacts for MAC and Grafana usage
# Azure CLI:  https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest
#
#   [Required]  ${1}  subscriptionId             SubscriptionId where resources(MAC, DCR, Grafana) are created
#   [Required]  ${2}  resourceGroup              Resource group where resources(MAC, DCR, Grafana) are created
#   [Required]  ${3}  monitoringAccountName      Name of the Monitoring Account that will be created
#   [Required]  ${4}  grafanaName                Name of the Grafana instance that will be created
#   [Required]  ${5}  azureRegion                Region where resources(MAC, DCR, Grafana) are created
#   [Required]  ${6}  aksResourceId              Azure resource id of the AKS Cluster ("/subscriptions/subid/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername")

#
# For example:
#
# bash Onboarding-script.sh "00000000-0000-0000-0000-000000000000" "my-rg" "my-mac-account" "my-grafana-instance" "eastus" "/subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername"
#
echo "subscriptionId"= ${1}
echo "resourceGroup" = ${2}
echo "monitoringAccountName"= ${3}
echo "grafanaName"= ${4}
echo "azureRegion" = ${5}
echo "aksResourceId" = ${6}

echo "Checking azure cli version"
currentVer=$(az version --query '"azure-cli"')
requiredVer="2.30.0"
 if [ "$(printf '%s\n' "$requiredVer" "$currentVer" | sort -V | head -n1)" = "$requiredVer" ]; then 
        echo "az cli version greater than or equal to required version - ${requiredVer}, continuing..."
 else
        echo "az cli version lower than required version- ${requiredVer}, please upgrade to a version > 2.30.0"
        exit 1
 fi

az login

subscriptionId=${1}
resourceGroup=${2}
monitoringAccountName=${3}
grafanaName=${4}
azureRegion=${5}
aksResourceId=${6}

macAccountLength=${#3}

# Checking for length of MAC name, since it is used in dc artifacts creation (max is 44)
if [ $macAccountLength -gt 24 ]
then
    echo "Monitoring account name is longer than 24 characters, please use a shorter name, exiting."
    exit 1
fi

trimmedRegion=$(echo $azureRegion | sed 's/ //g' | awk '{print tolower($0)}')
echo $trimmedRegion
if [ $trimmedRegion != "eastus2euap" ] && [ $trimmedRegion != "eastus" ] && [ $trimmedRegion != "eastus2" ] && [ $trimmedRegion != "westeurope" ]
then
    echo "azureRegion not in a supported region - eastus, eastus2, westeurope"
    exit 1
fi

aksResourceSplitarray=($(echo $aksResourceId | tr "/" "\n"))
aksResourceIdLength=${#aksResourceSplitarray[@]}

if [ $aksResourceIdLength != 8 ]
then
    echo "Incorrect AKS Resource ID specified, please specify an id in this format - /subscriptions/00000000-0000-0000-0000-000000000000/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername"
    exit 1
fi

echo "Getting AKS Subscription id and resource group for DCR association..."
aksSubId=${aksResourceSplitarray[1]}
aksRgName=${aksResourceSplitarray[3]}
echo "AKSSubId: $aksSubId"
echo "AKSRg: $aksRgName"

#az login
az extension add -n amg
az account set -s $subscriptionId

az group create --location $trimmedRegion --name $resourceGroup
if [ $? -ne 0 ]
then
    echo "Unable to create resource group"
    exit 1
fi

# Creating role definition
echo "Creating role definition to be able to read data from MAC"
az deployment sub create --location $trimmedRegion --name role-def-$trimmedRegion --template-file RoleDefinition.json

if [ $? -ne 0 ]
then
    echo "Unable to create custom role definition"
    exit 1
fi

#Template to create all resources required for MAC ingestion e2e
echo "Creating all resources required for MAC ingestion"
az deployment group create --resource-group $resourceGroup --template-file RootTemplate.json \
--parameters monitoringAccountName=$monitoringAccountName monitoringAccountLocation=$trimmedRegion \
targetAKSResource=$aksResourceId AKSSubId=$aksSubId AKSRg=$aksRgName

if [ $? -ne 0 ]
then
    echo "Unable to create resources required for MAC ingestion"
    exit 1
fi

echo "Creating Grafana instance, if it doesnt exist: $grafanaName" 
if [ $trimmedRegion == "eastus2" ]
then
    echo "Using EastUS for Grafana instance creation since EASTUS2 is not supported"
    grafanalocation="eastus"
else
    echo "Using $trimmedRegion for Grafana instance creation"
    grafanalocation=$trimmedRegion
fi

# Creating grafana instance
az grafana create -g $resourceGroup -n $grafanaName -l $grafanalocation
if [ $? -ne 0 ]
then
    echo "Unable to create Grafana instance"
    exit 1
fi
echo "Grafana instance created successfully - $grafanaName"

grafanaSmsi=$(az grafana show -g $resourceGroup -n $grafanaName --query 'identity.principalId')
echo "Got System Assigned Identity for Grafana instance: $grafanaSmsi"
echo "Removing quotes from MSI"
grafanaSmsi=$(echo $grafanaSmsi | tr -d '"')
echo "Removing carriage returns from MSI"
grafanaSmsi=$(echo $grafanaSmsi | tr -d '\r')


macId=$(az resource show -g $resourceGroup -n $monitoringAccountName --resource-type "Microsoft.Monitor/Accounts" --query 'id')
echo "Got MAC id: $macId"
echo "Removing quotes from MAC Id"
macId=$(echo $macId | tr -d '"')
echo "Removing carriage returns for MAC Id"
macId=$(echo $macId | tr -d '\r')

# Creating role assignment
echo "Assigning MAC reader role to grafana's system assigned MSI"
az role assignment create --assignee-object-id $grafanaSmsi --assignee-principal-type ServicePrincipal --scope $macId --role "Monitoring Data Reader-"${subscriptionId}

if [ $? -ne 0 ]
then
    echo "Unable to create role assignment to query MAC from Grafana instance"
    exit 1
fi

promQLEndpoint=$(az resource show -g $resourceGroup -n $monitoringAccountName --resource-type "Microsoft.Monitor/Accounts" --query 'properties.metrics.prometheusQueryEndpoint')
echo "PromQLEndpoint: $promQLEndpoint"

macPromDataSourceConfig='{
    "id": 6,
    "uid": "prometheus-mac",
    "orgId": 1,
    "name": "Monitoring Account",
    "type": "prometheus",
    "typeLogoUrl": "",
    "access": "proxy",
    "url": PROM_QL_PLACEHOLDER,
    "password": "",
    "user": "",
    "database": "",
    "basicAuth": false,
    "basicAuthUser": "",
    "basicAuthPassword": "",
    "withCredentials": false,
    "isDefault": true,
    "jsonData": {
        "azureAuth": true,
        "azureCredentials": {
            "authType": "msi"
        },
        "azureEndpointResourceId": "https://prometheus.monitor.azure.com",
        "httpMethod": "POST",
        "httpHeaderName1": "x-ms-use-new-mdm-namespace"
    },
    "secureJsonData": {
        "httpHeaderValue1": "true"
    },
    "version": 1,
    "readOnly": false
}'


populatedMACPromDataSourceConfig=${macPromDataSourceConfig//PROM_QL_PLACEHOLDER/$promQLEndpoint}

az grafana data-source create -n $grafanaName --definition "$populatedMACPromDataSourceConfig" 
# Not adding exit here since if this script is rerun it can fail with data source exists error, which is benign and can be ignored

echo "Downloading dashboards package"
wget https://github.com/microsoft/Docker-Provider/raw/prometheus-collector/prometheus-collector/dashboards.tar.gz
tar -zxvf dashboards.tar.gz 

#for 1p this folder already exists, it will fail with a 409 conflict, but its okay to move on
echo "Creating folder for dashboards"
az grafana folder create -g $resourceGroup -n $grafanaName --title "Azure Monitor Container Insights"

echo "Importing dashboards into Azure Monitor Container Insights folder in Grafana instance"
for FILE in dashboards/*.json; do
    az grafana dashboard import -g $resourceGroup -n $grafanaName --overwrite --definition $FILE --folder "Azure Monitor Container Insights"
done;

echo "Onboarding was completed successfully, please deploy the prometheus-collector helm chart for data collection using the helm command below."
echo "Please ensure to set the right cluster context before running the helm install command - See Step #2 in the instructions on how to set this."
echo "helm upgrade --install prometheus-collector-release ./prometheus-collector-3.2.0-main-05-24-2022-0c3a87bc.tgz --dependency-update --set useMonitoringAccount=true --set azureResourceId=\"$aksResourceId\" --set azureResourceRegion=\"$trimmedRegion\" --set mode.advanced=true --namespace=\"kube-system\" --create-namespace"

