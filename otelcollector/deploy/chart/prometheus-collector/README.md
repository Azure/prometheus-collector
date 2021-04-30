# prometheus-collector HELM chart

![Version: 0.0.2](https://img.shields.io/badge/Version-0.0.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.2](https://img.shields.io/badge/AppVersion-0.0.2-informational?style=flat-square)

A Helm chart for collecting Prometheus metrics in Kubernetes clusters and ingestion to Azure Metrics Account(s)

## Requirements

Kubernetes: `>=1.16.0-0`

## Pre-requisites

- **Step 1** : Create MDM Metric Account(s) & obtain Pfx certificate for each of the MDM Metric account(s)
    You can configure prometheus-collector to ingest different metrics into different MDM account(s). You will need to create atleast one MDM account (to use as default metric account) and have the name of that default MDM account. You also will need pfx certificate for each of the MDM accounts to which you will be configuring prometheus-collector to ingest metrics. See [configuration.md](../configuration.md) for more information about how to configure a metric account per scrape job (to ingest metrics from that scrape job to a specified metric account. If No metric account is specified as part of prometheus configuration for any scrape job, metrics produced by that scrape job will be ingested into the default metrics account specified as parameter)

- **Step 2** : Upload Pfx certificate(s) to Azure KeyVault
    Azure KeyVault is the only supported way for prometheus-collector to consume authentication certificates for ingesting into metric store account(s). Create an Azure KeyVault (if there is not one already that you can use to store certificate(s) ). Import certificate(s) from previous step  (pfx is required) per metric account into the KeyVault (ensure private key is exportable for the pfx certificate when importing into KeyVault, which is default behavior when importing pfx certificates into Azure KeyVauly).You will need the below information from this step -
     - KeyVaultName
     - KeyVault TenantId
     - Certificate Name (for each of the account's Pfx certificate that you uploaded to KeyVault in this step)

- **Step 3** : Provide access to KeyVault using service principal
    Prometheus-collector will need a service principal and secret to access key vault and pull the certificate(s) to use for ingesting metrics into MDM account(s). For this purpose, you will need to create/use a service principal and do the following -
     - 3.1) Create a new service principal & secret (or) use an existing service principal with its secret
     - 3.2) For the KeyVault resource, grant 'Key Vault Secrets User' built-in role for your service principal (from step 3.1)
     - 3.3) Copy the service principal app/clientid & its secret

## Install

- **Step 4** : Install csi driver & secrets store provider for azure KeyVault in your cluster
```shell 
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts 
helm install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --namespace <my_any_namespace> --create-namespace
```
  **Example** :-
```shell
helm repo add csi-secrets-store-provider-azure https://raw.githubusercontent.com/Azure/secrets-store-csi-driver-provider-azure/master/charts
helm install csi csi-secrets-store-provider-azure/csi-secrets-store-provider-azure --namespace csi --create-namespace
```

- **Step 5** : Pull, Export & Install prometheus-collector chart in your cluster
```shell
helm chart pull mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-0.0.2
helm chart export mcr.microsoft.com/azuremonitor/containerinsights/cidev:prometheus-collector-chart-0.0.2 .

helm upgrade --install <chart_release_name> ./prometheus-collector --set azureKeyVault.name='**' --set azureKeyVault.pfxCertNames='{**,**}' --set azureKeyVault.tenantId='**' --set clusterName='**' --set azureMetricAccount.defaultAccountName='**' --set azureKeyVault.clientId='**' --set azureKeyVault.clientSecret='****' --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector --set azureKeyVault.name='containerinsightstest1kv' --set azureKeyVault.pfxCertNames='{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}' --set azureKeyVault.tenantId='72f988bf-****-41af-****-2d7cd011db47' --set clusterName='mydevcluster' --set azureMetricAccount.defaultAccountName='containerinsightsgenevaaccount1' --set azureKeyVault.clientId='70937f05-****-4fc0-****-de917f2a9402' --set azureKeyVault.clientSecret='**********************************' --namespace=prom-collector --create-namespace
```
- **Step 6** : [Optional] - Apply prometheus configuration as configmap
  If you have prometheus config as .yml, you can apply it as config map using the below command. 
  
    **Tip** - If you have your own prometheus yaml scrape configuration and want to use that without having to paste into the configmap, rename your config file   to ```prometheus-config``` and run:**
```shell
kubectl create configmap <chart_release_name>-prometheus-config --from-file=prometheus-config -n <same_namespace_as_collector_namespace>
```
  **Example** :- [Note the release name 'my-collector-dev-release-' used a prefix to the configmap name below, and also config map should be created in the same namespace (ex;- prom-collector in this example) into which prometheus-collector chart was installed in step-5 above]
```shell
kubectl create configmap my-collector-dev-release-prometheus-config --from-file=prometheus-config -n prom-collector
```  
  **Tip** - We will validate provided prometheus configuration using [promtool](https://github.com/prometheus/prometheus/tree/main/cmd/promtool), an official commandline prometheus tool, with the command below]
```shell
    promtool check config <config_file_name>
```

 

## Chart Values

| Key | Type | Required | Default | Description |
|-----|------|----------|---------|-------------|
| azureKeyVault.name | string | <mark>`Required`</mark> | `""` | name of the azure key vault resource |
| azureKeyVault.clientId | string | <mark>`Required`</mark> | `""` | clientid for a service principal that has access to read the Pfx certificates from keyvault specified above |
| azureKeyVault.clientSecret | string | <mark>`Required`</mark> | `""` | client secret for the above service principal |
| azureKeyVault.pfxCertNames | list of comma seperated strings | <mark>`Required`</mark> | `{}` | name of the Pfx certificate(s) - one per metric account |
| azureKeyVault.tenantId | string | <mark>`Required`</mark> | `""` | tenantid for the azure key vault resource |
| azureMetricAccount.defaultAccountName | string | <mark>`Required`</mark> | `""` | default metric account name to ingest metrics into. This will be the account used if metric itself does not have account 'hinting' label. The certificate for this account should be specified in one of the further arguments below here |
| clusterName | string | <mark>`Required`</mark> | `""` | name of the k8s cluster. This will be added as a 'cluster' label for every metric scraped |
| image.pullPolicy | string | Optional | `"IfNotPresent"` |  |
| image.repository | string | Optional | `"mcr.microsoft.com/azuremonitor/containerinsights/cidev"` |  |
| image.tag | string | Optional | `"prometheus-collector-0420-2"` |  |
| internalSettings.intEnvironment | bool | Optional | `false` | do not use any of the internal settings. This is for testing purposes |
| resources.limits.cpu | string | Optional | `2` |  |
| resources.limits.memory | string | Optional | `"2Gi"` |  |
| resources.requests.cpu | string | Optional | `"250m"` |  |
| resources.requests.memory | string | Optional | `"1Gi"` |  |
| scrapeTargets.coreDns | bool | Optional | `true` | when true, automatically scrape coredns service in the k8s cluster without any additional scrape config |
| scrapeTargets.kubelet | bool | Optional | `true` | when true, automatically scrape kubelet in every node in the k8s cluster without any additional scrape config |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.5.0](https://github.com/norwoodj/helm-docs/releases/v1.5.0)
