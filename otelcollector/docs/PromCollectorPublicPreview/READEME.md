You can deploy the templates using a command like :

```az deployment group create -g <resource_group> -n <deployment_name> --template-file .\FullAzureMonitorMetricsProfile.json --parameters .\FullAzureMonitorMetricsProfileParameters.json```

**NOTE**

- Please edit the FullAzureMonitorMetricsProfileParameters,json file appropriately before running the ARM tempalte
- Please add in any existing azureMonitorWorkspaceIntegrations values to the grafana resource before running the template otherwise the older values will get deleted and replaced with what is there in the template at the time of deployment
