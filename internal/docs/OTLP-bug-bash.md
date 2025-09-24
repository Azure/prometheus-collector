<!-- 1. Create AKS cluster in canary region -->

## Setup (Done for Bug Bash Subscription)
### Public and Private Preview:
1. Register the features on the subscription:
    ``` sh
    az feature register --namespace  "Microsoft.Insights" --name "Amcs20240311"
    az provider register --namespace "Microsoft.Insights"

    az feature register --namespace "Microsoft.ContainerService" --name "AzureMonitorAppMonitoringPreview"
    az provider register --namespace "Microsoft.ContainerService"
    ```
### Private Preview Only:
2. Subscription needs to be allow-listed on AppInsights backend.
3. Logs and Metrics AKS image toggles applied to subscriptions or clusters.
4. App Monitoring AKS preview toggle applied to subscriptions or clusters.

## CLI Setup (Bug Bash Only)
These instructions are taken from the AKS guide [here](https://dev.azure.com/msazure/CloudNativeCompute/_wiki/wikis/CloudNativeCompute.wiki/358311/AZCLI-Coding-Handbook?anchor=setup#pre-steps---install-python-and-set-up-a-virtual-environment).
1. Install python version >= 3.7
2. Create a virtual env: `python3 -m venv azenv`
3. Activate the virtual environment via `source azenv/bin/activate` (from ubuntu bash) or `.\azenv\Scripts\activate` (from windows cmd).
4. Download the wheel file from [here](https://github.com/Azure/prometheus-collector/releases/download/untagged-c29ccdfeea74c1c6bb3e/aks_preview-18.0.0b39-py2.py3-none-any.whl).
4. Install azure cli and aks-preview extension with pre-release changes.
   ```sh
    pip install --upgrade pip
    pip install azure-cli
    pip install azure_cli_core
    az extension remove aks-preview
    az extension add --source aks_preview-18.0.0b39-py2.py3-none-any.whl -y
   ```
5. When done with testing, exit the virtual environment via `deactivate`.

## App Insights Creation
1. Create the managed Application Insights resource in eastus2euap. This creates the managaged DCR.
    ``` sh
    az rest --method put \
        --url "https://management.azure.com/subscriptions/<subId>/resourceGroups/<rgName>/providers/microsoft.insights/components/<aiResourceName>?api-version=2025-01-23-preview" \
        --body '{
        "location": "eastus2euap",
        "kind": "web",
        "properties": {
                "Application_Type": "web",
                "Flow_Type": "Bluefield",
                "Request_Source": "rest",
                "AzureMonitorWorkspaceIngestionMode": "Enabled",
                "CustomMetricsExclusivelyToAzureMonitorWorkspace": false
            }
        }'
    ```
2. After the cluster has been created or if using an existing cluster, associate Managed DCR to the AKS cluster
    ``` sh
    az monitor data-collection rule association create --name "otel-test-ai" --rule-id "/subscriptions/<subscriptionId>/resourceGroups/<managedResourceGroup>/providers/microsoft.insights/dataCollectionRules/<dcrName> " --resource "/subscriptions/<subscriptionId>/resourcegroups/<resourceGroup>/providers/Microsoft.ContainerService/managedClusters/<clusterName>"
    ```

## Greenfield Scenarios
- All settings:
    ```sh
    az group create --name <resource-group> --location eastus2euap 
    az aks create -g <resource-group> -n <cluster-name> --location eastus2euap --generate-ssh-keys --enable-azure-monitor-app-monitoring --enable-azure-monitor-metrics --enable-opentelemetry-metrics --enable-addons monitoring --enable-opentelemetry-logs --opentelemetry-metrics-port 23450 --opentelemetry-logs-port 23451  --workspace-resource-id "/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourcegroups/grace-eastus2euap/providers/microsoft.operationalinsights/workspaces/grace-eastus2euap" --node-vm-size Standard_DS2_v2
    ```
- Extra settings for `enable-azure-monitor-metrics` like `--azure-monitor-workspace` work.
- Extra settings for `enable-addons monitoring` like `--workspace-resource-id` work.
## Brownfield Scenarios
- All settings:
    ```sh
    az aks addon enable -a monitoring
    az aks update -g grace-win -n grace-win --enable-azure-monitor-app-monitoring --enable-azure-monitor-metrics --enable-opentelemetry-metrics --enable-opentelemetry-logs --opentelemetry-metrics-port 23450 --opentelemetry-logs-port 23451
    ```
- With addons already enabled:
    ```sh
    az aks update -g <resource-group> -n <cluster-name> --enable-opentelemetry-metrics --enable-opentelemetry-logs
    ```
- Extra settings for `enable-azure-monitor-metrics` like `--azure-monitor-workspace` work.
- Extra settings for `enable monitoring` like `--workspace-resource-id` work.

## Cluster Profile
The output should contain settings for:
```
  "azureMonitorProfile": {
    "appMonitoring": {
      "autoInstrumentation": {
        "enabled": true
      },
      "openTelemetryLogs": {
        "enabled": true,
        "port": null
      },
      "openTelemetryMetrics": {
        "enabled": true,
        "port": null
      }
    },
    "containerInsights": {
      "disableCustomMetrics": null,
      "disablePrometheusMetricsScraping": null,
      "enabled": true,
      "logAnalyticsWorkspaceResourceId": "/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourcegroups/grace-eastus2euap/providers/microsoft.operationalinsights/workspaces/grace-eastus2euap",
      "syslogPort": null
    },
    "metrics": {
      "enabled": true,
      "kubeStateMetrics": {
        "metricAnnotationsAllowList": "",
        "metricLabelsAllowlist": ""
      }
    }
  },
```

## App Instrumentation
Follow the onboarding for Auto-Instrumentation from the docs [here](https://learn.microsoft.com/en-us/azure/azure-monitor/app/kubernetes-codeless#namespace-wide-onboarding).

## Consumption
1. Go to the managed log analytics workspace and query the tables:
    - OTelLogs
    - OTelSpans
    - OTelEvents
    - OTelResources
2. Go to the managed azure monitor workspace and query with PromQL