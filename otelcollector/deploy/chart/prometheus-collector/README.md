- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)

# prometheus-collector HELM chart

![Version: 0.0.2](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for collecting Prometheus metrics in Kubernetes clusters and ingestion to Azure Metrics Account(s)

## Requirements

Kubernetes: `>=1.16.0-0`

## Pre-requisites

- **Step 0** : Tools
    You will need [kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/) and [helm client tool(v3 or later)](https://helm.sh/docs/intro/install/) to continue this deployment.

    ```Note: Its recommended to use linux/WSL in windows to deploy with the below steps. Though windows command shell/powershell should work, we haven't fully tested with them. If you find any bug, please let us know (askcoin@microsoft.com)```

- **Step 1** : Create MDM Metric Account(s) & obtain Pfx certificate for each of the MDM Metric account(s)
    You can configure prometheus-collector to ingest different metrics into different MDM account(s). You will need to create atleast one MDM account (to use as default metric account) and have the name of that default MDM account. You also will need pfx certificate for each of the MDM accounts to which you will be configuring prometheus-collector to ingest metrics. See [configuration.md](../../../configuration.md) for more information about how to configure a metric account per scrape job (to ingest metrics from that scrape job to a specified metric account. If No metric account is specified as part of prometheus configuration for any scrape job, metrics produced by that scrape job will be ingested into the default metrics account specified as parameter)

- **Step 2** : Upload/provision certificate(s) for your metric store account(s) in Azure KeyVault
  Azure KeyVault is the only supported way for this prometheus-collector to read authentication certificates for ingesting into metric store account(s). Create an Azure KeyVault (if there is not one already that you can use to store certificate(s) ). Import/create certificate(s) (private key should be exportable) per metric account into the KeyVault (ensure private key is exportable for the certificate), and update the secretProviderClass.yaml with the below (and save the secretProviderClass.yaml file)
     - KeyVaultName
     - KeyVault TenantId
     - Certificate Name (for each of the account's certificate (thats exportable with private key) that you uploaded to KeyVault in this step)

- **Step 3** : Provide access to KeyVault using service principal
    Prometheus-collector will need a service principal and secret to access key vault and pull the certificate(s) to use for ingesting metrics into MDM account(s). For this purpose, you will need to create/use a service principal and do the following -
     - 3.1) Create a new service principal & secret (or) use an existing service principal with its secret
     - 3.2) For the KeyVault resource, grant 'Key Vault Secrets User' built-in role for your service principal (from step 3.1)
     - 3.3) Copy the service principal app/clientid & its secret

## Install

- **Step 4** : Install csi driver & secrets store provider for azure KeyVault in your cluster
```shell 
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts 
helm upgrade --install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --set secrets-store-csi-driver.enableSecretRotation=true --namespace <my_any_namespace> --create-namespace
```
  **Example** :-
```shell
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts
helm upgrade --install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --set secrets-store-csi-driver.enableSecretRotation=true --namespace csi --create-namespace
```

- **Step 5** : Pull, Export & Install prometheus-collector chart in your cluster
```shell
helm chart pull mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-main-0.0.1-07-29-2021-34a65d59
helm chart export mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-main-0.0.1-07-29-2021-34a65d59 .
helm dependency update ./prometheus-collector

helm upgrade --install <chart_release_name> ./prometheus-collector --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.clientId="**" --set azureKeyVault.clientSecret="****" --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.clientId="70937f05-****-4fc0-****-de917f2a9402" --set azureKeyVault.clientSecret="**********************************" --namespace=prom-collector --create-namespace
```
- **Step 6** : [Optional] - Apply aditional prometheus scrape configuration as configmap
  Any additional prometheus scrape configuration (for your applications/services/other exporters etc..), you can author the config apply it as config map using the below instructions. See the provided sample prometheus scrape config [prometheus-config](../sample-scrape-configs/prometheus-config) as an example.
  
  Rename your config file to ```prometheus-config``` (no extension for the file)  and validate it using [promtool](https://github.com/prometheus/prometheus/tree/main/cmd/promtool), an official commandline prometheus tool, with the command below]

```shell
    promtool check config <path_to_prometheus-config>
```

  Now apply your ```prometheus-config``` as configmap using below.

```shell
kubectl create configmap <chart_release_name>-prometheus-config --from-file=prometheus-config -n <same_namespace_as_collector_namespace>
```
  **Example** :- [Note the sample release name 'my-collector-dev-release-' is used as prefix to the configmap name below, and also config map should be created in the same namespace (ex;- prom-collector in this example) into which prometheus-collector chart was installed in step-5 above]
```shell
kubectl create configmap my-collector-dev-release-prometheus-config --from-file=prometheus-config -n prom-collector
```  

--- 

## Chart Values

| Key | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| azureKeyVault.name | string | <mark>`Required`</mark> | `""` | name of the azure key vault resource |
| azureKeyVault.clientId | string | <mark>`Required`</mark> | `""` | clientid for a service principal that has access to read the Pfx certificates from keyvault specified above |
| azureKeyVault.clientSecret | string | <mark>`Required`</mark> | `""` | client secret for the above service principal |
| azureKeyVault.pfxCertNames | list of comma seperated strings | <mark>`Required`</mark> | `"{}"` | name of the Pfx certificate(s) - one per metric account |
| azureKeyVault.tenantId | string | <mark>`Required`</mark> | `""` | tenantid for the azure key vault resource |
| azureMetricAccount.defaultAccountName | string | <mark>`Required`</mark> | `""` | default metric account name to ingest metrics into. This will be the account used if metric itself does not have account 'hinting' label. The certificate for this account should be specified in one of the further arguments below here |
| clusterName | string | <mark>`Required`</mark> | `""` | name of the k8s cluster. This will be added as a 'cluster' label for every metric scraped |
| image.pullPolicy | string | Optional | `"IfNotPresent"` |  |
| image.repository | string | Optional | `"mcr.microsoft.com/azuremonitor/containerinsights/cidev"` |  |
| image.tag | string | Optional | `"prometheus-collector-main-07-28-2021-55fb08c2"` |  |
| internalSettings.intEnvironment | bool | Optional | `false` | do not use any of the internal settings. This is for testing purposes |
| mode.advanced | bool | Optional | `false` | if mode.advanced==true (default is false), then it will deploy a daemonset in addition to replica, and move some of the default node targets (kubelet, cadvisor & nodeexporter) to daemonset. On bigger clusters (> 50+ nodes and > 1500+ pods), it is highly recommended to set this to `true`, as this will distribute the metric volumes to individual nodes as nodes & pods scale out & grow. Note:- When this is set to `true`, the `up` metric for the node target will be generated from the replica, so when the node (and daemonset in the node) becomes unvailable), the target availability can still be tracked.
| resources.deployment.limits.cpu | string | Optional | `4` |  |
| resources.deployment.limits.memory | string | Optional | `"7Gi"` |  |
| resources.deployment.requests.cpu | string | Optional | `"1"` |  |
| resources.deployment.requests.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.limits.cpu | string | Optional | `1` |  |
| resources.daemonSet.limits.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.requests.cpu | string | Optional | `"500m"` |  |
| resources.daemonSet.requests.memory | string | Optional | `"1Gi"` |  |
| scrapeTargets.coreDns | bool | Optional | `true` | when true, automatically scrape coredns service in the k8s cluster without any additional scrape config |
| scrapeTargets.kubelet | bool | Optional | `true` | when true, automatically scrape kubelet in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.cAdvisor | bool | Optional | `true` | when true, automatically scrape cAdvisor in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeProxy | bool | Optional | `true` | when true, automatically scrape kube-proxy in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.apiServer | bool | Optional | `true` | when true, automatically scrape the kubernetes api server in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeState | bool | Optional | `true` | when true, automatically install kube-state-metrics and scrape kube-state-metrics in the k8s cluster without any additional scrape config |
| scrapeTargets.nodeExporter | bool | Optional | `true` | when true, automatically install prometheus-node-exporter in every node in the k8s cluster and scrape node metrics without any additional scrape config |


----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.5.0](https://github.com/norwoodj/helm-docs/releases/v1.5.0)
