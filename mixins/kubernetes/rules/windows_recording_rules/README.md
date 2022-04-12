# Deploy recording rules to your cluster


## Pre-requisite 

Setup your subscription to be able to use alerts using the following [documentation](https://eng.ms/docs/products/geneva/metrics/prometheus/prommdmtutorial7setupalerts)

## Steps to deploy

Once you've added the ConfigurationEditor role and setup the feature flag for prometheus alerts on your subscription, replace the values in the `values.json` file present in the templates folder.

`location : The Monitoring Account need to reside in a region supported by the Prometheus alerts and recording rules preview.`

`mac : Full MAC resource ID similar to : /subscriptions/{sub_id}}/resourcegroups/{resource_group}/providers/microsoft.monitor/accounts/{account_name}`

`cluster : Name of the cluster the rules will be filtered to`

Use the `.\create_rules.ps1` script to deploy the recording rules to your cluster.

Please use the following link to view the dashboards once the script succeeds:

https://cimonitoring.scus.azgrafana.io/dashboards/f/yLVAyAP7k/kaveesh-test
