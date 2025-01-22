# Increasing Azure Monitor Workspace ingestion limits with ARM API update

## Introduction
Azure Monitor Workspaces or AMW are containers that store data collected by Azure Monitor managed service for Prometheus. An AMW instance has certain limits on how much data it can ingest. These limits are set by default, but they can be customized by the customer by creating a support ticket. For more details on these limits, see [Azure Monitor service limits](https://learn.microsoft.com/azure/azure-monitor/service-limits#prometheus-metrics)

Customers can now update the ingestion limits for their AMW instance using the Azure Resource Manager (ARM) API.

Few additional details about this update:
- Customers can request for an increase in limit from 1 Mn events/min or active TS to up to 20 Mn events/min or active TS with an API update through cli or through ARM update. For limits above 20 mn, customers will need to create a support ticket.
  - Customers can request upto 2 Mn and get approved - no usage will be checked.
  - If customers request beyond 2 Mn, we will check usage for at least 50%, i.e. if customer's AMW is at 5 Mn, they can request for increase upto 10Mn. Customers can request up to 20 Mn.
  - For requests beyond 20 Mn, please create a support ticket.
- Customers can request an increase for an existing AMW instance. We are not supporting creation of AMW with increased limits. Creation of AMW will always apply the default limits. This is because we want to support increasing the limits based on certain heuristics/usage.

This document explains how to use the ARM API to update the data ingestion limits of your Azure Monitor Workspaces. 

## Prerequisites

- An Azure subscription with one or more Azure Monitor Workspaces
- A command-line tool to run the ARM template commands, such as Azure PowerShell, or Azure CLI


### Step 1: Share the subscription ID

Submit the form [here](https://forms.microsoft.com/r/8P9F2GS7k4) to share the subscription ID with us in order to enable the feature for your subscription. It will take a while for the feature to be enabled, and we will follow up via email as soon as it is ready for your subscription. After the feature is enabled, you can try the preview for any Azure Monitor Workspace instance in that subscription.

### Step 2: Download the ARM templates and update the parameters

Download the ARM template files ([AMWLimitIncrease-Template.json](./AMWLimitIncrease-Template.json) and [AMWLimitIncrease-Parameters.json](./AMWLimitIncrease-Parameters.json) ) and update the Parameters.json file with the AMW name, location and required ingestion limits (maximum is 20 Mn).

### Step 3: Execute the ARM update

Run the below commands from the downloaded ARM templates folder:

For Azure CLI:

```azurecli
az login
az account set --subscription <subscriptionId>
az deployment group create --name AmwLimits --resource-group <resourceGroupName>   --template-file AMWLimitIncrease-Template.json --parameters AMWLimitIncrease-Parameters.json
```

For Azure Powershell:

```
Connect-AzAccount
New-AzResourceGroupDeployment -Name AmwLimits -ResourceGroupName  <resourceGroupName> -TemplateFile AMWLimitIncrease-Template.json -TemplateParameterFile AMWLimitIncrease-Parameters.json
```

### Step 4: Verify if the limits are updated

To verify if the limits are updated successfully, you can go to the Azure portal, navigate to the Azure Monitor Workspace -> Metrics explorer and then verify if the updated limits are applied to the “Active Time Series Limit” and “Events per minute Ingested Limit”.

