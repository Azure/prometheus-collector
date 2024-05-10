You can deploy the templates using a command like :

```az deployment group create -g <resource_group> -n <deployment_name> --template-file ./FullAzureMonitorMetricsProfile.bicep --parameters ./FullAzureMonitorMetricsProfileParameters.json```


In order to deploy recommended metric alerts through template, deploy using command like:

```az deployment group create -g <resource_group> -n <deployment_name> --template-file .\recommendedMetricAlerts.bicep --parameters .\recommendedMetricAlertsProfileParameters.json```

**NOTE**

- Please download all files under AddonBicepTemplate folder before running the Bicep template.
- Please edit the FullAzureMonitorMetricsProfileParameters.json file appropriately before running the Bicep template
- Users with 'User Access Administrator' role in the subscription  of the AKS cluster can be able to enable 'Monitoring Data Reader' role directly by deploying the template.
- Please add in any existing azureMonitorWorkspaceIntegrations values to the grafana resource before running the template otherwise the older values will get deleted and replaced with what is there in the template at the time of deployment
- Please edit the grafanaSku parameter if you are using a non standard SKU.
- Please run this template in the Grafana Resources RG.
