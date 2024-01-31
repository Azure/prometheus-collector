### **Step 1: Enable the Container Insights Add-On**
Utilize the Azure Portal to enable the add-on. Follow the instructions provided in the [documentation](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable?tabs=cli#enable-full-monitoring-with-azure-portal).
We need this step because we need to get the secret created for the addon-token-adapter to serve, which is created when the monitoring addon is enabled. The Managed Prometheus addon also uses this secret for its functioning.

### **Step 2: Deploy ARM templates for configuration** 
Instructions on how to deploy ARM template -
https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable?tabs=arm#enable-prometheus-and-grafana

In the ARM template, comment out the section that enables the addon (with name - **"azuremonitormetrics-profile-"**(Lines 188 to 228), please comment the section acccordingly if template is updated)

### **Step 3: Go to addon-chart directory**
```
cd prometheus-collector\otelcollector\deploy\addon-chart
```
### **Step 4: Update the chart/values file accordingly based on what needs to be tested with your backdoor deployment**

Charts and Values for the addon are in the folder azure-monitor-metrics-addon/

Values.yaml has some settings that need to be replaced, that are specific to your cluster. Please replace them before installing the helm chart.
 - global.commonGlobals.Region
 - global.commonGlobals.Customer.AzureResourceID


i.e. replace ${AKS_REGION} with somethinng similar to "eastus" and ${AKS_RESOURCE_ID} with "/subscriptions/{sub_id}/resourceGroups/{rg_name}/providers/Microsoft.ContainerService/managedClusters/{cluster_name}

### **Step 6: Install Helm chart**
```
helm install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/values.yaml
```

### **Step 7: Uninstall helm chart**
```
helm uninstall ama-metrics
```