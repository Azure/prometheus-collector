
# Enable Azure Monitor Metrics for existing Azure Kubernetes Service (AKS) cluster
This article describes how to set up Container insights to monitor managed Kubernetes cluster hosted on [Azure Kubernetes Service](https://learn.microsoft.com/en-us/azure/aks/) that have already been deployed in your subscription.

The following resource providers must be registered in the subscription of the AKS cluster and the Azure Monitor Workspace

- Microsoft.ContainerService 
- Microsoft.Insights
- Microsoft.AlertsManagement

For more information, see [Register resource provider](https://learn.microsoft.com/en-us/azure/azure-resource-manager/management/resource-providers-and-types#register-resource-provider).

## CLI

> [!NOTE]
> Azure CLI version 2.41.0 or higher is required for this feature.


The `az extension add --name aks-preview` extension needs to be installed for access to this feature. For more information on how to install an az cli extension, see [Use and manage extensions with the Azure CLI](https://learn.microsoft.com/en-us/cli/azure/azure-cli-extensions-overview)


The following step enables monitoring of your AKS cluster using Azure CLI. In this example, you are not required to pre-create or specify an existing Azure Monitor Workspace. This command simplifies the process for you by creating a default Azure Monitor Workspace in the default resource group of the AKS cluster subscription if one does not already exist in the region.  The default workspace created resembles the format of *DefaultAzureMonitorWorkspace-\<Region>*.

`
az aks update --enable-azuremonitormetrics -n MyExistingManagedCluster -g MyExistingManagedClusterRG
`

The output will contain the following:

<pre>
"azureMonitorProfile": {
    "metrics": {
      "enabled": true,
      "kubeStateMetrics": {
        "metricAnnotationsAllowList": "",
        "metricLabelsAllowlist": ""
      }
    }
  }
</pre>

### Integrate with an existing Azure Monitor workspace

If you would rather integrate with an existing Azure Monitor workspace, perform the following steps to first identify the full resource ID of your Azure Monitor workspace required for the `--azure-monitor-workspace-resource-id` parameter, and then run the command to enable the Azure Monitor Metrics profile against the specified workspace.

1. List all the subscriptions that you have access to using the following command:

    `az account list --all -o table`

    The output will resemble the following:

    <pre>
    Name                                  CloudName    SubscriptionId                        State    IsDefault
    ------------------------------------  -----------  ------------------------------------  -------  -----------
    Microsoft Azure                       AzureCloud   00000000-0000-0000-0000-000000000000  Enabled  True
    </pre>

    Copy the value for **SubscriptionId**.

2. Switch to the subscription hosting the Azure Monitor workspace using the following command:

    `az account set -s <subscriptionId of the workspace>`

3. The following example displays the list of workspaces in your subscriptions in the default JSON format.

    `az resource list --resource-type Microsoft.Monitor/Accounts -o json`

    In the output, find the workspace name, and then copy the full resource ID of that Azure Monitor workspace under the field **id**.

4. Switch to the subscription hosting the cluster using the following command:

    `az account set -s <subscriptionId of the cluster>`

5. Run the following command to enable the Azure Monitor Metrics add-on, replacing the value for the `--azure-monitor-workspace-resource-id` parameter. The string value must be within the double quotes:

    `az aks update --enable-azuremonitormetrics -n ExistingManagedCluster -g ExistingManagedClusterRG --azure-monitor-workspace-resource-id "/subscriptions/<SubscriptionId>/resourceGroups/<ResourceGroupName>/providers/Microsoft.Monitor/Accounts/<WorkspaceName>"`

    The output will contain the following:

    <pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "",
            "metricLabelsAllowlist": ""
          }
        }
      }
    </pre>

### Integrate with an existing Azure Managed Grafana

You can also link your Azure Monitor Workspace account with an Azure Managed Grafana workspace for viewing the metrics and you will also get 9 dashboards available out of the box with the data source already configured for the particular Azure Monitor Workspace following the format *Managed_Prometheus_\<Azure_Monitor_Workspace_name>*.

The following dashboards will be available by default:

1. K8s mix-ins
    1.  Cluster compute resources
    1. Namespace compute resources(Pods)
    1. Namespace compute resources(Workloads)
    1. Node compute resources
    1. Pod compute resources
    1. Workload compute resources
1. Kubelet
1. Node exporter
    1. Nodes
    1. USE Method(Node)

Perform the following steps to first identify the full resource ID of your Azure Managed Grafana required for the `--grafana-resource-id` parameter, and then run the command to enable the Azure Monitor Metrics profile against the specified workspace.

1. List all the subscriptions that you have access to using the following command:

    `az account list --all -o table`

    The output will resemble the following:

    <pre>
    Name                                  CloudName    SubscriptionId                        State    IsDefault
    ------------------------------------  -----------  ------------------------------------  -------  -----------
    Microsoft Azure                       AzureCloud   00000000-0000-0000-0000-000000000000  Enabled  True
    </pre>

    Copy the value for **SubscriptionId**.

2. Switch to the subscription hosting the Azure Managed Grafana using the following command:

    `az account set -s <subscriptionId of grafana instance>`

3. The following example displays the list of grafana instances in your subscriptions in the default JSON format.

    `az resource list --resource-type Microsoft.Dashboard/grafana -o json`

    In the output, find the grafana instance name, and then copy the full resource ID of that Azure Managed Grafana under the field **id**.

4. Switch to the subscription hosting the cluster using the following command:

    `az account set -s <subscriptionId of the cluster>`

5. Run the following command to enable the Azure Monitor Metrics add-on, replacing the value for the `--grafana-resource-id` parameter. The string value must be within the double quotes:

    `az aks update --enable-azuremonitormetrics -n ExistingManagedCluster -g ExistingManagedClusterRG --grafana-resource-id "/subscriptions/<SubscriptionId>/resourceGroups/<ResourceGroupName>/providers/Microsoft.Dashboard/grafana/<grafanaName>"`

    The output will contain the following:

    <pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "",
            "metricLabelsAllowlist": ""
          }
        }
      }
    </pre>

## Optional parameters to setup the metric annotations and metric labels allow lists for kube-state metrics

`--ksm-metric-annotations-allow-list` is a comma-separated list of Kubernetes annotations keys that will be used in the resource's labels metric. By default the metric contains only name and namespace labels. To include additional annotations provide a list of resource names in their plural form and Kubernetes annotation keys you would like to allow for them (Example: `namespaces=[kubernetes.io/team,...],pods=[kubernetes.io/team],...)`. A single '*' can be provided per resource instead to allow any annotations, but that has severe performance implications (Example: `pods=[*]`).

`--ksm-metric-labels-allow-list` is a comma-separated list of additional Kubernetes label keys that will be used in the resource' labels metric. By default the metric contains only name and namespace labels. To include additional labels provide a list of resource names in their plural form and Kubernetes label keys you would like to allow for them (Example: `namespaces=[k8s-label-1,k8s-label-n,...],pods=[app],...)`. A single '*' can be provided per resource instead to allow any labels, but that has severe performance implications (Example: `pods=[*]`).

Run the following command to enable the Azure Monitor Metrics add-on, replacing the value for the `--ksm-metric-labels-allow-list` and/or `--ksm-metric-annotations-allow-list` parameters. The string value must be within the double quotes:
 
`
az aks update --enable-azuremonitormetrics -n ExistingManagedCluster -g ExistingManagedClusterRG --ksm-metric-labels-allow-list "namespaces=[k8s-label-1,k8s-label-n]" --ksm-metric-annotations-allow-list "pods=[k8s-annotation-1,k8s-annotation-n]"
`

The output will contain the following:
<pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "pods=[k8s-annotation-1,k8s-annotation-n]",
            "metricLabelsAllowlist": "namespaces=[k8s-label-1,k8s-label-n]"
          }
        }
      }
</pre>

## Resource Manager Template

This method includes two JSON templates. One template specifies the configuration to enable monitoring, and the other contains parameter values that you configure to specify the following:

* The Azure Monitor Workspace resource ID.
* The Azure Monitor Workspace location.
* The AKS cluster resource ID.
* The AKS cluster location.
* The Azure Managed Grafana resource ID.
* The Azure Managed Grafana location.
* The Azure Managed Grafana sku.
* Kube state metrics labels allow list (optional string parameter)
* Kube state metrics annotations allow list (optional string parameter)


>[!NOTE]
>The template needs to be deployed in the same resource group as the cluster.


### Prerequisites
The Azure Monitor Workspace and Azure Managed Grafana instance must be created before you deploy the Resource Manager template.

If you're using an existing Azure Managed Grafana instance that already has been linked to an azure monitor workspace while onboarding another cluster, Please get the list of azureMonitorWorkspaceIntegrations via the **Azure Managed Grafana Overview** page for the Azure Managed Grafana instance. Open the JSON View (with API version 2022-08-01) and copy the value of the following field (If it does not exists then the instance has not been linked with any Azure Monitor Workspace)
<pre>
"properties": {
    "grafanaIntegrations": {
            "azureMonitorWorkspaceIntegrations": [
                {
                    "azureMonitorWorkspaceResourceId": "full_resource_id_1"
                },
                {
                    "azureMonitorWorkspaceResourceId": "full_resource_id_2"
                }
            ]
        }
}
</pre>

Please store the values from all the azureMonitorWorkspaceIntegrations for later use.

### Create or download templates

1. Download the template at [temporary link -> move to aka.ms](https://raw.githubusercontent.com/Azure/prometheus-collector/kaveesh/arm_template/otelcollector/docs/PromCollectorPublicPreview/FullAzureMonitorMetricsProfile.json?token=GHSAT0AAAAAABUMNXFSOG3H4LIKOWFQDXUGYZI3L3A) and save it as **existingClusterOnboarding.json**.

2. Download the parameter file at [temporary link -> move to aka.ms](https://raw.githubusercontent.com/Azure/prometheus-collector/kaveesh/arm_template/otelcollector/docs/PromCollectorPublicPreview/FullAzureMonitorMetricsProfileParameters.json?token=GHSAT0AAAAAABUMNXFSPFX66LD2TQTN7LG4YZI3M5A) and save it as **existingClusterParam.json**.

3. Edit the values in the parameter file.

  - For **clusterResourceId** and **clusterLocation**, use the values on the **AKS Overview** page for the AKS cluster.
  - For **azureMonitorWorkspaceResourceId** and **azureMonitorWorkspaceLocation**, use the values on the **Azure Monitor workspace Properties** page for the Azure Monitor workspace. 
  - For **metricLabelsAllowlist**, comma-separated list of Kubernetes labels keys that will be used in the resource's labels metric.
  - For**metricAnnotationsAllowList**, comma-separated list of additional Kubernetes label keys that will be used in the resource' labels metric.
  - For **grafanaResourceId**, **grafanaLocation** and **grafanaSku** , use the values on the **Azure Managed Grafana Overview** page for the Azure Managed Grafana instance (from the **id**, **location** and **sku.name** fields of the JSON View with API version 2022-08-01)
  
4. Open the template file and update the `grafanaIntegrations` property at the end of the file with the values that you stored in the pre-requisite. For e.g. the end result would look something like the following :

<pre>
{
      "type": "Microsoft.Dashboard/grafana",
      "apiVersion": "2022-08-01",
      "name": "[split(parameters('grafanaResourceId'),'/')[8]]",
      "sku": {
        "name": "[parameters('grafanaSku')]"
      },
      "location": "[parameters('grafanaLocation')]",
      "properties": {
        "grafanaIntegrations": {
          "azureMonitorWorkspaceIntegrations": [
            {
                "azureMonitorWorkspaceResourceId": "full_resource_id_1"
            },
            {
                "azureMonitorWorkspaceResourceId": "full_resource_id_2"
            }
            {
              "azureMonitorWorkspaceResourceId": "[parameters('azureMonitorWorkspaceResourceId')]"
            }
          ]
        }
      }
</pre>

### Deploy template

If you are unfamiliar with the concept of deploying resources by using a template, see:

* [Deploy resources with Resource Manager templates and Azure PowerShell](https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/deploy-powershell)
* [Deploy resources with Resource Manager templates and the Azure CLI](https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/deploy-cli)

If you choose to use the Azure CLI, you first need to install and use the CLI locally. You must be running the Azure CLI version 2.0.59 or later. To identify your version, run `az --version`. If you need to install or upgrade the Azure CLI, see [Install the Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli).

#### To deploy with Azure PowerShell:

`
New-AzResourceGroupDeployment -Name OnboardCluster -ResourceGroupName <ResourceGroupName> -TemplateFile .\existingClusterOnboarding.json -TemplateParameterFile .\existingClusterParam.json
`

The configuration change can take a few minutes to complete. When it's completed, a message is displayed that's similar to the following and includes the result:

<pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "",
            "metricLabelsAllowlist": ""
          }
        }
      }
</pre>

#### To deploy with Azure CLI, run the following commands:

<pre>
az login
az account set --subscription "Subscription Name"
az deployment group create --resource-group <ResourceGroupName> --template-file ./existingClusterOnboarding.json --parameters @./existingClusterParam.json
</pre>

The configuration change can take a few minutes to complete. When it's completed, a message is displayed that's similar to the following and includes the result:

<pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "",
            "metricLabelsAllowlist": ""
          }
        }
      }
</pre>

After you've enabled monitoring, you can view the metrics in the Azure Managed Grafana instance that you've linked or through querying the Azure Monitor Workspace.

## Verify Deployment

Run the following commands to verify that the agent is deployed successfully.

`
kubectl get ds ama-metrics-node --namespace=kube-system
`

The output should resemble the following, which indicates the daemonset was deployed properly:

<pre>
User@aksuser:~$ kubectl get ds ama-metrics-node --namespace=kube-system
NAME               DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
ama-metrics-node   1         1         1       1            1           <none>          10h
</pre>

`
kubectl get rs --namespace=kube-system
`

The output should resemble the following, which indicates the replicaset was deployed properly:

<pre>
User@aksuser:~$kubectl get rs --namespace=kube-system
NAME                            DESIRED   CURRENT   READY   AGE
ama-metrics-5c974985b8          1         1         1       11h
ama-metrics-ksm-5fcf8dffcd      1         1         1       11h
</pre>


## View configuration with CLI

Use the `aks show` command to get details such as is the solution enabled or not and what kube-state metrics annotations and labels are specified

`
az aks show -g <resourceGroupofAKSCluster> -n <nameofAksCluster>
`

After a few minutes, the command completes and returns JSON-formatted information about solution.  The results of the command should show the monitoring add-on profile and resembles the following example output:

<pre>
    "azureMonitorProfile": {
        "metrics": {
          "enabled": true,
          "kubeStateMetrics": {
            "metricAnnotationsAllowList": "",
            "metricLabelsAllowlist": ""
          }
        }
      }
</pre>

## Limitations

- Please update the kube-state metrics Annotations and Labels list with proper formatting and care. There is a limitation in the Resource Manager template deployments right now were we are passing through the exact values into the kube-state metrics pods. If the kuberenetes pods has any issues with malformed parameters and isn't running then the feature will not work as expected.
- A data collection rule, data collection endpoint is created with the name *MSPROM-\<cluster-name\>-\<cluster-region\>*. These names cannot currently be modified.
- One must get the existing azure monitor workspace integrations for a grafana workspace and update the resource manager template with it otherwise it will overwrite and remove the existing integrations from the grafana workspace.
- One can only offboard using the az cli for now.