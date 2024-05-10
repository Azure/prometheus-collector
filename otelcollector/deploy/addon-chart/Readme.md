### **Step 1: Enable and Disable Azure Monitor Metrics addon**
Use cli to enable the addon and then disable the addon.  
**Enable addon** - 
```
az aks update --enable-azuremonitormetrics -n <cluster-name> -g <cluster-resource-group>
```

**Disable addon** - 
```
az aks update --disable-azuremonitormetrics -n <cluster-name> -g <cluster-resource-group>
```
We need this step because we need to get the secret created for the addon-token-adapter to serve, which is only created when the addon is enabled.


### **Step 2: Deploy ARM templates for configuration** 
Instructions on how to deploy ARM template -
https://learn.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-prometheus-metrics-addon?tabs=resource-manager#download-and-edit-template-and-parameter-file

In the ARM template, comment out the section that enables the addon (with name - **"azuremonitormetrics-profile-"**(Lines 147 to 187), please comment the section acccordingly if template is updated)

### **Step 3: Go to addon-chart directory**
```
cd prometheus-collector\otelcollector\deploy\addon-chart
```
### **Step 4: Update the chart/values file accordingly based on what needs to be tested with your backdoor deployment**

Charts and Values for the addon are in the folder azure-monitor-metrics-addon/

Values.yaml has some settings that need to be replaced, that are specific to your cluster. Please replace them before installing the helm chart.
 - global.commonGlobals.Region
 - global.commonGlobals.Customer.AzureResourceID

 Additionally, AzureMonitorMetrics.AddonTokenAdapter.ImageRepository and AzureMonitorMetrics.AddonTokenAdapter.ImageTag, should also be updated to the latest from the
 AKS-RP repo so that the backdoor deployment gets the latest image that is deployed by the AKS-RP

Repository: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml&version=GBrashmi/prom-addon-arm64&line=136&lineEnd=136&lineStartColumn=56&lineEndColumn=85&lineStyle=plain&_a=contents
ImageTag: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/kube-control-plane/templates/_images.tpl&version=GBrashmi/prom-addon-arm64&line=530&lineEnd=530&lineStartColumn=28&lineEndColumn=53&lineStyle=plain&_a=contents


### **Step 5: Test out with Operator mode turned on**

This is an interim step needed until the operator changes roll out globally to make sure the changes work well with the new mode.
The value TargetAllocatorEnabled is set to false by default, this needs to be set to true to test out with the operator mode turned on.

### **Step 6: Install Helm chart**
```
helm install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/values.yaml
```

### **Step 7: Uninstall helm chart**
```
helm uninstall ama-metrics
```