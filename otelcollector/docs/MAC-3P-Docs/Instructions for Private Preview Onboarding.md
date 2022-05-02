# Instructions for Private Preview onboarding

**1. The steps need to executed in this order. If not, step 1 has to be rerun after executing step 2**

**2. Aks cluster needs to be same region as the onboarding resources being created**

**3. Regions available – EastUS2, EastUS, WestEurope**

**4. This script does not create an AKS cluster, you will need to pass the id of an existing AKS cluster to the script**

**5. az cli version needs to be >= 2.30.0**

**6. Ingesting data from one cluster to multiple MAC accounts (multi homing) is not supported for private preview.**

### **Step 1**: Download and run the script (Recommended to use Azure cloud shell) to create the required resources for metric ingestion and query. Please note the script takes about 10 to 15 minutes because of the various resource creation templates. Please make sure the session doesn’t timeout so that you don’t miss out on the errors. 
1.	Download the Onboarding-script.sh, RootTemplate.json and RoleDefinition.json using the following links -

        wget https://raw.githubusercontent.com/microsoft/Docker-Provider/prometheus-collector/prometheus-collector/MAC-3P-Docs/Onboarding-script.sh
<br/> 

        wget https://raw.githubusercontent.com/microsoft/Docker-Provider/prometheus-collector/prometheus-collector/MAC-3P-Docs/RoleDefinition.json
<br/>

        wget https://raw.githubusercontent.com/microsoft/Docker-Provider/prometheus-collector/prometheus-collector/MAC-3P-Docs/RootTemplate.json

2.	Update the script to have execute permissions. 
3.	Run the bash script with the parameters specified for the below in double quotes("")
    - Subscriptionid – subscription where the onboarding resources are created
    - ResourceGroupname – Resource group where the onboarding resources are created. (Rg is automatically created if the provided rg doesn’t exist)
    - MonitoringAccountName – Account where the metrics will be sent (This is created if it doesn’t exist already)
    - Grafana Instance Name – Grafana instance that will be used to query the metrics (This is created if it doesn’t exist already)
    - Azure Region – Location/region where the resources are created. (EastUS2, Eastus and WestEurope are the supported regions)
    - AKS Resource Id – AKS resource from which the metrics need to be collected.

To run the script execute - 

    bash Onboarding-script.sh "<sub-id>" "<rg-name>" "<mac-name>" "<grafana-instance-name>" "<location/region>" "<aks-resource-id>"

Ex: bash Onboarding-script.sh "00000000-0000-0000-0000-000000000000" "rg-name" "mac-name" "grafana-name" "eastus2" "/subscriptions/subid/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername"


4.	Check to make sure the script run completed without any errors. This step also outputs the helm command that needs to be used in Step 2.


### **Step 2**: Install the helm chart on the AKS cluster to collect metrics and send them to the MAC account

**Requires helm version  >= v3.7.0. If you are using Azure cloud shell, it by default has v3.4.0. Please download the latest version by following instructions here https://helm.sh/docs/intro/install/#from-the-binary-releases.
Cloud shell doesn’t let you replace the exe in location /usr/local/bin/helm.
You can instead run the helm commands from the path ~/linux-amd64. Prefix the helm with "./" for it to pick up helm from this folder.**

**Ex - ./helm pull oci://mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector --version 3.1.0-main-04-29-2022-0a7092d3**

1.	Download the helm chart - 

        set HELM_EXPERIMENTAL_OCI=1

        helm pull oci://mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector --version 3.1.0-main-04-29-2022-0a7092d3

2.  Install the helm chart with the following parameters  -
    
        helm upgrade --install <release-name> ./prometheus-collector-3.1.0-main-04-29-2022-0a7092d3.tgz --dependency-update --set useMonitoringAccount=true --set azureResourceId="<aks-resource-id>" --set azureResourceRegion="<aks-resource-location>" --set mode.advanced=true --namespace="kube-system" --create-namespace


Ex - helm upgrade --install my-collector-dev-release ./prometheus-collector-3.1.0-main-04-29-2022-0a7092d3.tgz --dependency-update --set useMonitoringAccount=true --set azureResourceId="/subscriptions/subid/resourcegroups/rg-name/providers/Microsoft.ContainerService/managedClusters/clustername" --set azureResourceRegion="eastus2" --set mode.advanced=true --namespace="kube-system" --create-namespace

    Please make sure you have set the right context for the AKS cluster you want to install the chart on
    - az aks get-credentials -g <aks-rg-name> -n <aks-cluster-name> (This gets the cluster kubeconfig and sets it as the current context)
    - or, If you already have the kubeconfig downloaded, set it as current-context 
        kubectl config use-context <aks-cluster-name>
    - Make sure you have the right context set by running
        kubectl cluster-info


### **Step 3**: Navigate to the Grafana UX. An initial set of default dashboards are created under the folder  ‘Azure Monitor Container Insights’. Browse through these dashboards by picking the Monitoring Account data source to see the cluster you just started collecting metrics from.

1. Go to aka.ms/ags/portal/prod
2. Navigate to the newly created grafana instance with the script
3. Click on the **Endpoint** url in the Overview blade to access the Grafana UX.
--------------------------------------