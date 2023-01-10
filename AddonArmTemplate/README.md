You can deploy the templates using a command like :

```az deployment group create -g <resource_group> -n <deployment_name> --template-file .\FullAzureMonitorMetricsProfile.json --parameters .\FullAzureMonitorMetricsProfileParameters.json```

**NOTE**

- Please edit the FullAzureMonitorMetricsProfileParameters,json file appropriately before running the ARM tempalte
- Please add in any existing azureMonitorWorkspaceIntegrations values to the grafana resource before running the template otherwise the older values will get deleted and replaced with what is there in the template at the time of deployment
- Please edit the grafanaSku parameter if you are using a non standard SKU.
- Please assign the role 'Monitoring Data Reader' to the Grafana MSI on the Azure Monitor Workspace resource so that it can read data for displaying the charts
- Please run this template in the Grafana Resources RG.
