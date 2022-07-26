# Setup windows dashboards with recording rules for your cluster

## Pre-requisite 

- Setup your subscription to be able to use alerts using the following [documentation](https://eng.ms/docs/products/geneva/metrics/prometheus/prommdmtutorial7setupalerts)

- Please also run this script in a location where you have access to the az cli.

- Please install windows-exporter on your AKS nodes. To do so you'll need to ssh/RDP into your AKS nodes using the following [documentation](https://docs.microsoft.com/en-us/azure/aks/node-access) and then install windows exporter using the following [instructions](https://github.com/prometheus-community/windows_exporter#installation)

The steps will look something like this:

1. SSH into each individual AKS node
2. Run powershell
3. Call `Invoke-WebRequest -Uri https://github.com/microsoft/Docker-Provider/releases/download/windows-exporter-releases/windows_exporter-0.16.0-amd64.3.msi -OutFile c:\test.msi`
4. Call `msiexec /i C:\test.msi ENABLED_COLLECTORS="[defaults],process,container,tcp,os,memory" /quiet`
5. Call `netstat -na | findstr 9182` to see wether the exporter is running as expected.

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

`Note: This script uses the az cli for logging in and deploying the recording rules to the cluster. Please use the latest version of the az cli for the best experience.`

The dashboards can be found checked in the `\otelcollector\deploy\dashboard\windows\recording-rules` folder.

Please import the below mentioned files to see the metrics from the recording rules deployed using the script.

-   Kubernetes _ Compute Resources _ Cluster(Windows)
-   Kubernetes _ Compute Resources _ Cluster(Windows)
-   Kubernetes / Compute Resources / Pod(Windows)
-   Kubernetes / USE Method / Cluster(Windows)
-   Kubernetes / USE Method / Node(Windows)


If you're account is already added in the cimonitoring grafana instance then you can also use the following link to view the dashboards once the script succeeds:

https://cimonitoring.scus.azgrafana.io/dashboards/f/xtltXN_7k/kaveesh-test-3
