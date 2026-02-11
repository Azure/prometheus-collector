# Troubleshoot Guide for Azure Monitor Metrics (Managed Prometheus)


# Troubleshooting script

Prequisites:
- For AKS/Arc, Collect ResourceId of the cluster
- Powershell and kubectl (if running locally)

# AKS/Arc

You can use the troubleshooting script provided [here](https://raw.githubusercontent.com/Azure/prometheus-collector/main/internal/scripts/troubleshoot/TroubleshootError.ps1) to diagnose the problem.

Steps:
- Open powershell using the [cloudshell](https://docs.microsoft.com/en-us/azure/cloud-shell/overview) in the azure portal.
- Make sure your az account subscription is pointing to the subscription of the cluster.
- If running an Arc cluster, make sure the kubeconfig/kubectl context is already pointing to the cluster

> Note: This script supported on any Powershell supported environment: Windows and Non-Windows.
 For Linux, refer [Install-Powershell-On-Linux](https://docs.microsoft.com/en-us/powershell/scripting/install/installing-powershell-core-on-linux?view=powershell-7) and
 For Mac OS, refer [install-powershell-core-on-mac](https://docs.microsoft.com/en-us/powershell/scripting/install/installing-powershell-core-on-macos?view=powershell-7) how to install powershell
- Make sure that you're using powershell (selected by default)
- Run the following command to change home directory - `cd ~`
- Run the following command to download the script - `curl -LO https://raw.githubusercontent.com/Azure/prometheus-collector/arc-troubleshoot/internal/scripts/troubleshoot/TroubleshootError.ps1`

> Note: In some versions of Powershell above CURL command may not work in such cases, you can try  `curl https://raw.githubusercontent.com/Azure/prometheus-collector/main/internal/scripts/troubleshoot/TroubleshootError.ps1`


- Run the following command to execute the script - `./TroubleshootError.ps1 -ClusterResourceId <resourceIdoftheCluster>`
    > Note: For AKS, resourceIdoftheCluster should be in this format `/subscriptions/<subId>/resourceGroups/<rgName>/providers/Microsoft.ContainerService/managedClusters/<clusterName>`.
- This script will generate a TroubleshootDump.txt and a ZIP file called debugLogs.zip which collects detailed information about container health onboarding.
- Please [create a support ticket](https://azure.microsoft.com/en-us/support/create-ticket) in Azure and send these two files alongwith it. We will respond back to you.
