# Setup windows dashboards with recording rules for your cluster

## Pre-requisite 

Setup your subscription to be able to use alerts using the following [documentation](https://eng.ms/docs/products/geneva/metrics/prometheus/prommdmtutorial7setupalerts)

Please also run this script in a location where you have access to the az cli.

## Steps to deploy

Replace the proper values in the `values.json` file present in the templates folder.

Below is a sample list of values:

```
{
    "MACLocation": "{eastus2euap||northcentralus}",
    "mac": "/subscriptions/{sub_id}}/resourcegroups/{resource_group}/providers/microsoft.monitor/accounts/{account_name}",
    "cluster": "{cluster_name}"
}
```


Run the `.\create_rules.ps1` script to deploy the recording rules to your cluster.

`Note: This script uses the az cli for logging in and deploying the recording rules to the cluster.`

The dashboards can be found checked in the `\otelcollector\deploy\dashboard\dashboards` folder.

Please import the below mentioned files to see the metrics from the recording rules deployed using the script.

-   k8s-resources-windows-cluster
-   k8s-resources-windows-namespace
-   k8s-resources-windows-pod



If you're account is already added in the cimonitoring grafana instance then you can also use the following link to view the dashboards once the script succeeds:

https://cimonitoring.scus.azgrafana.io/dashboards/f/yLVAyAP7k/kaveesh-test
