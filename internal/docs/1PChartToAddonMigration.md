# Instructions for migrating from Helm chart (Prometheus-collector) to AKS addon for Managed Prometheus

## step 1 : Create Azure monitoring workspace using internal documentation [here](https://eng.ms/docs/products/geneva/metrics/prometheus/mac)
  
Note this step is necessary, in the sense, that if you create AMW using Azure portal, internal only features like ICM integration will not work.
    
## step 2 : Delete Prometheus-collecotr HELM chart and enable metrics addon in your AKS cluster. See external [documentation](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-enable?tabs=azure-portal) for enabling addon in your cluster.
  
    You can enable thrugh CLI/UX/ARM as specified in the above documentation link.

## step 3 : Tune collection settings (optional, as needed)
    
Prometheus collector chart by default scrapes more targets and hence collects more Prometheus metrics from AKS clusters. Add-on collects a subset of metrics that are being collected by Helm chart based deployment. If you want to enable other targets for addon , please follow the steps [here](https://github.com/Azure/prometheus-collector/blob/vishwa/1paddon/GeneratedMonitoringArtifacts/non-default/README.md).
    

# FAQs

## 1) Does the scrape configuration provided using configmap to the HELM chart work with addon ?

Same config map containing scrape configurtion will work with addon. But the name of the configmap(s) are different between HELM chart and addon. See below . Also note that the addon config map(s) *must* be in kube-system namespace , where the addon runs.


| HELM Chart                           | Addon | 
| -----------------------                   |-------------| 
|<helm_release_name>-prometheus-config               | ama-metrics-prometheus-config    |
|<helm_release_name>-prometheus-config-node               | ama-metrics-prometheus-config-node       |

In addition to the above scrape configuration configmaps, some of the HELM chart parameters (like enabling/disabling predefined targets) acn be specified through `ama-metrics-settings-configmap` with the addon. See [here](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-scrape-configuration#metrics-addon-settings-configmap) for more details.


## 2) Will i be able to change Pod resources (CPU & MEM) in the new addon model ?

You cannot change the resource requests & limits for the addon. The defaults for addon are set to high values (for our replica limits(cpu,mem) are set to (7 cores, 14Gi), and for daemonset limits(cpu,mem) are set to (200m, 1Gi) ). If you were using more than these limits for your HELM chart, please contact us at ciprometheus@microsoft.com with your subscriptionID(s) of your AKS cluster(s) and the new limits, and we can increase them as appropriate.

## 3) Where can i see the ingestion utilization/limits for my AMW, and how do i put a request to increase it ?

You can see your quota/usage for ingestion from the `Metrics` menu for your Azure Monitor workspace in the Azure Portal. Check your geneva account's current usage. All AMW comes with default ingestion limits of `1 million events/min` and `1 million time-series per 12 hours`. If your current geneva account's Prometheus usage is more than that, you can file a support ticket from the Azure Monitor Workspace's `Support Request` menu in the portal to increase the limits.

## 4) Can i run the HELM chart & the addon side-by-side ?
    
Side by side scenario is neither tested not supported, as it will cause duplicate time-series to be ingested. Please uninstall the Helm chart before enabliong addon.

## 5) Can i run addon on non-AKS clusters ?

Addon is only for AKS clusters. If you are currently using Helm chart for monitoring a non-AKS cluster, please reach out to us (ciprometheus@microsoft.com).

## 6) Can the Kubernetes namespace in which the addon runs be changed ?

Addon is managed by MSFT and i can only run on `kube-system` namespace.