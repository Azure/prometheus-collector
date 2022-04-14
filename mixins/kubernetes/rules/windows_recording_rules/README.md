# Setup windows dashboards with recording rules for your cluster

## Pre-requisite 

Setup your subscription to be able to use alerts using the following [documentation](https://eng.ms/docs/products/geneva/metrics/prometheus/prommdmtutorial7setupalerts)

Please also run this script in a location where you have access to the az cli.

## Steps to deploy

Replace the proper values in the `values.json` file present in the templates folder.

Below is a sample list of values:

```
{
    "location": "{eastus2euap||northcentralus}",
    "mac": "/subscriptions/{sub_id}}/resourcegroups/{resource_group}/providers/microsoft.monitor/accounts/{account_name}",
    "cluster": "{cluster_name}"
}
```


Run the `.\create_rules.ps1` script to deploy the recording rules to your cluster.

`Note: This script uses the az cli for logging in and deploying the recording rules to the cluster.`

The dashboards can be found checked in the `.\dashboards` folder. Please import them into your own grafana instance to start using them.


If you're account is already added in the cimonitoring grafana instance then you can also use the following link to view the dashboards once the script succeeds:

https://cimonitoring.scus.azgrafana.io/dashboards/f/yLVAyAP7k/kaveesh-test
