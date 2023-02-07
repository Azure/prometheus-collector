You can deploy the templates using a command like :

```az deployment group create -g <resource_group> -n <deployment_name> --template-file .\FullAzureMonitorMetricsProfile.json --parameters .\FullAzureMonitorMetricsProfileParameters.json```

**NOTE**

- Please edit the FullAzureMonitorMetricsProfileParameters.json file appropriately before running the ARM tempalte
- Users with 'User Access Administrator' role in the subscription  of the AKS cluster can be able to enable 'Monitoring Data Reader' role directly by deploying the template.
- Please add in any existing azureMonitorWorkspaceIntegrations values to the grafana resource before running the template otherwise the older values will get deleted and replaced with what is there in the template at the time of deployment
- Please edit the grafanaSku parameter if you are using a non standard SKU.
- Please run this template in the Grafana Resources RG.
