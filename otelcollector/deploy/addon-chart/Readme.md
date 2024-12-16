### **Step 1: Enable and Disable Azure Monitor Metrics addon**
Use cli to enable the addon and then disable the addon.  
**Enable addon** - 
```
az aks update --enable-azure-monitor-metrics -n <cluster-name> -g <cluster-resource-group>
```

**Disable addon** - 
```
az aks update --disable-azure-monitor-metrics -n <cluster-name> -g <cluster-resource-group>
```
We need this step because we need to get the secret created for the addon-token-adapter to serve, which is only created when the addon is enabled.


### **Step 2: Deploy ARM templates for configuration** 
Instructions on how to deploy ARM template -
https://learn.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-prometheus-metrics-addon?tabs=resource-manager#download-and-edit-template-and-parameter-file

In the ARM template, comment out the section that enables the addon (with name - **"azureMonitorProfile"**(Lines 156 to 196), please comment the section acccordingly if template is updated)

### **Step 3: Go to addon-chart directory**
```
cd prometheus-collector\otelcollector\deploy\addon-chart
```
### **Step 4: Update the chart/values file accordingly based on what needs to be tested with your backdoor deployment**

Update, local_testing_aks.ps1 within the azure-monitor-metrics-addon/ folder with the apporpritate ImageTag, Cluster Region and Cluster Resource ID (lines 9 to 11). Run the powershell file to generate the Chart and Values from the template files.

If you do not run the script and manually generate the Chart, Values yaml files then please do the following :

Values.yaml has some settings that need to be replaced, that are specific to your cluster. Please replace them before installing the helm chart.
 - global.commonGlobals.Region
 - global.commonGlobals.Customer.AzureResourceID

 Additionally, AzureMonitorMetrics.AddonTokenAdapter.ImageRepository and AzureMonitorMetrics.AddonTokenAdapter.ImageTag, should also be updated to the latest from the
 AKS-RP repo so that the backdoor deployment gets the latest image that is deployed by the AKS-RP

Repository: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml&version=GBrashmi/prom-addon-arm64&line=136&lineEnd=136&lineStartColumn=56&lineEndColumn=85&lineStyle=plain&_a=contents
ImageTag: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/kube-control-plane/templates/_images.tpl&version=GBrashmi/prom-addon-arm64&line=530&lineEnd=530&lineStartColumn=28&lineEndColumn=53&lineStyle=plain&_a=contents

### **Step 5: Install Helm chart**
```
helm upgrade --install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/values.yaml
```

### **Step 6: Uninstall helm chart**
```
helm uninstall ama-metrics
```

# Arc Extension Values
There are a couple Arc-specific settings that customers can use when installing the ama-metrics extension.
- `ClusterDistribution` and `CloudEnvironment` are by default the values that are provided by the Arc infrastructure. These values in our chart allow customers to override them if needed.
- We do not know what Linux distribution the nodes will be using, so we mount both common CA cert directories. Certain node restrictions do not allow directories that do not exist to be created like AKS Edge clusters. In these cases `MountCATrustAnchorsDirectory` and `MountUbuntuCACertDirectory` can be used to set one to `false`.

| Parameter | Description | Default Value | Upstream Arc Cluster Setting |
|-----------|-------------|---------------|---------------|
| `ClusterDistribution` | The distribution of the cluster | `Azure.Cluster.Distribution` | yes |
| `CloudEnvironment` | The cloud environment for the cluster | `Azure.Cluster.Cloud` | yes |
| `MountCATrustAnchorsDirectory` | Whether to mount CA trust anchors directory | `true` | no |
| `MountUbuntuCACertDirectory` | Whether to mount Ubuntu CA certificate directory | `true` unless an `aks_edge` distro | no |
