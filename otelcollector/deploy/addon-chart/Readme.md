### **Step 1: Go to addon-chart directory**
cd prometheus-collector\otelcollector\deploy\addon-chart

### **Step 2: Update the chart/values file accordingly based on what needs to be tested with your backdoor deployment**

Charts and Values for the addon are in the folder azure-monitor-metrics-addon/

Values.yaml has some settings that need to be replaced, that are specific to your cluster. Please replace them before installing the helm chart.
 - global.commonGlobals.Region
 - global.commonGlobals.Customer.AzureResourceID

 Additionally, AzureMonitorMetrics.AddonTokenAdapter.ImageRepository and AzureMonitorMetrics.AddonTokenAdapter.ImageTag, should also be updated to the latest from the
 AKS-RP repo so that the backdoor deployment gets the latest image that is deployed by the AKS-RP

Repository: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml&version=GBrashmi/prom-addon-arm64&line=136&lineEnd=136&lineStartColumn=56&lineEndColumn=85&lineStyle=plain&_a=contents
ImageTag: https://msazure.visualstudio.com/CloudNativeCompute/_git/aks-rp?path=/ccp/charts/kube-control-plane/templates/_images.tpl&version=GBrashmi/prom-addon-arm64&line=530&lineEnd=530&lineStartColumn=28&lineEndColumn=53&lineStyle=plain&_a=contents

### **Step 3: Install Helm chart**
helm install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/Values.yaml

### **Step 4: Uninstall helm chart**
helm uninstall ama-metrics