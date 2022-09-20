
# How to stop monitoring your Azure Kubernetes Service (AKS) with Azure Monitor Metrics

After you enable monitoring of your AKS cluster, you can stop monitoring the cluster if you decide you no longer want to monitor it. This article shows how to accomplish this using the Azure CLI and Azure Resource Manager templates.

## Azure CLI

Use the [az aks update](https://learn.microsoft.com/en-us/cli/azure/aks?view=azure-cli-latest#az-aks-update) command to disable Azure Monitor Metrics. The command removes the agent from the cluster nodes and deletes the recording rules created for the data being collected from the cluster, it does not remove the DCE, DCR or the data already collected and stored in your Azure Monitor Workspace resource.

`
az aks update --disable-azuremonitormetrics -n MyExistingManagedCluster -g MyExistingManagedClusterRG
`

## Azure Resource Manager template

Please use the az cli for offboarding right now.