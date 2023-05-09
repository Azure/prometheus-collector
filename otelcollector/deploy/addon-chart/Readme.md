### **Step 1: Enable and Disable Azure Monitor Metrics addon**
Use cli to enable the addon and then disable the addon. This creates the MSI token provided by AKS and only needs to be done once. We need this step because we need to get the secret created for the addon-token-adapter to serve, which is only created when the addon is enabled.
**Enable addon** - 
```
az aks update --enable-azuremonitormetrics -n <cluster-name> -g <cluster-resource-group>
```

**Disable addon** - 
```
az aks update --disable-azuremonitormetrics -n <cluster-name> -g <cluster-resource-group>
```

### **Step 2: Deploy ARM templates for configuration but without azuremonitormetrics-profile** 
Instructions on how to deploy ARM template -
https://learn.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-prometheus-metrics-addon?tabs=resource-manager#download-and-edit-template-and-parameter-file

**In the ARM template, comment out the section that enables the addon (with name - **"azuremonitormetrics-profile-"**(Lines 147 to 187), please comment the section acccordingly if template is updated)**

### **Step 3: Clone repository and go to addon-chart directory**
```
cd prometheus-collector\otelcollector\deploy\addon-chart
```
### **Step 4: Update the chart/values file accordingly based on what needs to be tested with your backdoor deployment**

Charts and Values for the addon are in the folder azure-monitor-metrics-addon/

Values.yaml has some settings that need to be replaced, that are specific to your cluster. Please replace them before installing the helm chart.
 - global.commonGlobals.Region
 - global.commonGlobals.Customer.AzureResourceID

### **Step 5: Install Helm chart**
```
helm install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/values.yaml
```

### **Step 6: Uninstall helm chart**
```
helm uninstall ama-metrics
```