- **Main branch builds:** ![Builds on main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event=push)

- **PR builds:** ![PRs to main branch](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-image-and-chart.yml/badge.svg?branch=main&event!=push)

# prometheus-collector HELM chart

![Version: 0.0.2](https://img.shields.io/badge/Version-0.0.1-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for collecting Prometheus metrics in Kubernetes clusters and ingestion to Azure Metrics Account(s)

## Requirements

Kubernetes: `>=1.16.0-0`

## Pre-requisites

- **Step 0** : Tools
    You will need [kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/) and [helm client tool(v3.7.0 or later - see below note)](https://helm.sh/docs/intro/install/) to continue this deployment. 

    ```Note: Our charts will not work on HELM clients < 3.7.0```

    ```Note: Its recommended to use linux/WSL in windows to deploy with the below steps. Though windows command shell/powershell should work, we haven't fully tested with them. If you find any bug, please let us know (askcoin@microsoft.com)```

- **Step 1** : Create MDM Metric Account(s) & obtain Pfx certificate for each of the MDM Metric account(s)
    You can configure prometheus-collector to ingest different metrics into different MDM account(s). You will need to create atleast one MDM account (to use as default metric account) and have the name of that default MDM account. You also will need pfx certificate for each of the MDM accounts to which you will be configuring prometheus-collector to ingest metrics. See [configuration.md](../../../configuration.md) for more information about how to configure a metric account per scrape job (to ingest metrics from that scrape job to a specified metric account. If No metric account is specified as part of prometheus configuration for any scrape job, metrics produced by that scrape job will be ingested into the default metrics account specified as parameter)

- **Step 2** : Upload/provision certificate(s) for your metric store account(s) in Azure KeyVault
  Azure KeyVault is the only supported way for this prometheus-collector to read authentication certificates for ingesting into metric store account(s). Create an Azure KeyVault (if there is not one already that you can use to store certificate(s) ). Import/create certificate(s) (private key should be exportable) per metric account into the KeyVault (ensure private key is exportable for the certificate), and update the secretProviderClass.yaml with the below (and save the secretProviderClass.yaml file)
     - KeyVaultName
     - KeyVault TenantId
     - Certificate Name (for each of the account's certificate (thats exportable with private key) that you uploaded to KeyVault in this step)

- **Step 3** : Provide access to KeyVault using service principal or MSI
    Service Principal:
        As one of the methods to fetch certificates from Azure Keyvault, Prometheus-collector needs a service principal and secret to access key vault and pull the certificate(s) to use for ingesting metrics into MDM account(s). For this purpose, you will need to create/use a service principal and do the following -
     - 3.1) Create a new service principal & secret (or) use an existing service principal with its secret
     - 3.2) For the KeyVault resource, grant 'Key Vault Secrets User' built-in role for your service principal (from step 3.1)
     - 3.3) Copy the service principal app/clientid & its secret
    Managed Identity:
        Prometheus collector also supports both User Assigned Managed Identity & System Assigned Managed Identity to access key vault and pull the certificate(s) to use for ingesting metrics into MDM. For this, you will need to grant access to the appropriate managed identity used by/in your kubernetes cluster(s) to the Azure Key Vault. Check [here](https://docs.microsoft.com/en-us/azure/aks/csi-secrets-store-identity-access) for instructions how to grant access to an identity for your key vault.


    

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

- **Step 5** : Pull & Install prometheus-collector chart in your cluster
```shell
helm pull oci://mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector --version 2.0.0-main-03-17-2022-dfef2a5d
```

If using Service principal:
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.clientId="**" --set azureKeyVault.clientSecret="****" --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Service principal)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.clientId="70937f05-****-4fc0-****-de917f2a9402" --set azureKeyVault.clientSecret="**********************************" --namespace=prom-collector --create-namespace
```

If using Managed Identity (User Assigned): [See specifically, azureKeyVault.useManagedIdentity & azureKeyVault.userAssignedIdentityID parameters below]
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.useManagedIdentity=true --set azureKeyVault.userAssignedIdentityID="59677e05-****-4ea1-****-ed976f2b2049" --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Managed identity-user defined)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.useManagedIdentity=true --set azureKeyVault.userAssignedIdentityID="59677e05-****-4ea1-****-ed976f2b2049" --namespace=prom-collector --create-namespace
```

If using Managed Identity (System Assigned): [See specifically, azureKeyVault.useManagedIdentity parameter below]
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.useManagedIdentity=true --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Managed identity-system)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-2.0.0-main-03-17-2022-dfef2a5d.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.useManagedIdentity=true --namespace=prom-collector --create-namespace
```

- **Step 6** : [Optional] - Apply aditional prometheus scrape configuration as configmap
  Any additional prometheus scrape configuration (for your applications/services/other exporters etc..), you can author the config apply it as config map using the below instructions. See the provided sample prometheus scrape config [prometheus-config](../sample-scrape-configs/prometheus-config) as an example.
  
  Rename your config file to ```prometheus-config``` (no extension for the file)  and validate it using promconfigvalidator, a commandline prometheus config validation tool, with the command below. Copy the tool and template from these paths /opt/promconfigvalidator and /opt/microsoft/otelcollector/collector-config-template.yml from within the prometheus-collector container

```shell
    ./promconfigvalidator --config "config-path" --otelTemplate "collector-config-template-path"
```
  This by default generates the otel collector configuration file 'merged-otel-config.yaml' if no paramater is provided using the optional --output paramater.
  This is the otel config that will be applied to the prometheus collector which includes the custom prometheus config
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
| azureKeyVault.clientId | string | Optional | `""` | clientid for a service principal that has access to read the Pfx certificates from keyvault specified above. Required when using service principal based auth to access keyvault |
| azureKeyVault.clientSecret | string | Optional | `""` | client secret for the above service principal. Required when using service principal |
| azureKeyVault.pfxCertNames | list of comma seperated strings | <mark>`Required`</mark> | `"{}"` | name of the Pfx certificate(s) - one per metric account |
| azureKeyVault.tenantId | string | <mark>`Required`</mark> | `""` | tenantid for the azure key vault resource |
| azureKeyVault.useManagedIdentity | string | Optional | `false` | enable/disable managed identity to access keyvault |
| azureKeyVault.userAssignedIdentityID | string | Optional | `""` | used when useManagedIdentity parameter is set to true. This specifies which user assigned managed identity to use when acccesing keyvault. If you are using a user assigned identity as managed identity, then specify the identity's client id. If empty, AND 'useManagedIdentity' is true, then defaults to use the system assigned identity on the VM |
| azureMetricAccount.defaultAccountName | string | <mark>`Required`</mark> | `""` | default metric account name to ingest metrics into. This will be the account used if metric itself does not have account 'hinting' label. The certificate for this account should be specified in one of the further arguments below here |
| clusterName | string | <mark>`Required`</mark> | `""` | name of the k8s cluster. This will be added as a 'cluster' label for every metric scraped |
| image.pullPolicy | string | Optional | `"IfNotPresent"` |  |
| image.repository | string | Optional | `"mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images"` |  |
| image.tag | string | Optional | `"2.0.0-main-03-17-2022-dfef2a5d"` |  |
| internalSettings.intEnvironment | bool | Optional | `false` | do not use any of the internal settings. This is for testing purposes |
| internalSettings.clusterOverride | bool | Optional | `false` | do not use any of the internal settings. This is for testing purposes for Geneva team |
| mode.advanced | bool | Optional | `false` | if mode.advanced==true (default is false), then it will deploy a daemonset in addition to replica, and move some of the default node targets (kubelet, cadvisor & nodeexporter) to daemonset. On bigger clusters (> 50+ nodes and > 1500+ pods), it is highly recommended to set this to `true`, as this will distribute the metric volumes to individual nodes as nodes & pods scale out & grow. Note:- When this is set to `true`, the `up` metric for the node target will be generated from the replica, so when the node (and daemonset in the node) becomes unvailable), the target availability can still be tracked.
| windowsDaemonset | bool | Optional | `false` | if mode.advanced==true (default is false), and windowsDaemonset==true (default is false) then it will deploy a windows daemonset on windows nodes, and move the default windows node targets (windowsexporter, windows-kube-proxy) to windows daemonset. On bigger windows clusters (> 50+ windows nodes and > 1500+ windows pods), it is highly recommended to set this to `true`, as this will distribute the metric volumes to individual windows nodes, as windows nodes & windows pods scale out & grow. Note:- When this is set to `true`, the `up` metric for the windows node targets will be generated from the replica, so when the windows node (and daemonset in the windows node) becomes unvailable), the target availability can still be tracked. Note:- This setting will be effective only when mode.advanced==true.
| resources.deployment.limits.cpu | string | Optional | `4` |  |
| resources.deployment.limits.memory | string | Optional | `"7Gi"` |  |
| resources.deployment.requests.cpu | string | Optional | `"1"` |  |
| resources.deployment.requests.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.limits.cpu | string | Optional | `1` |  |
| resources.daemonSet.limits.memory | string | Optional | `"2Gi"` |  |
| resources.daemonSet.requests.cpu | string | Optional | `"500m"` |  |
| resources.daemonSet.requests.memory | string | Optional | `"1Gi"` |  |
| updateStrategy.daemonSet.maxUnavailable | string | Optional | `"1"` | This can be a number or percentage of pods |
| scrapeTargets.coreDns | bool | Optional | `true` | when true, automatically scrape coredns service in the k8s cluster without any additional scrape config |
| scrapeTargets.kubelet | bool | Optional | `true` | when true, automatically scrape kubelet in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.cAdvisor | bool | Optional | `true` | when true, automatically scrape cAdvisor in every node in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeProxy | bool | Optional | `true` | `linux only` - when true, automatically scrape kube-proxy in every linux node discovered in the k8s cluster without any additional scrape config |
| scrapeTargets.apiServer | bool | Optional | `true` | when true, automatically scrape the kubernetes api server in the k8s cluster without any additional scrape config |
| scrapeTargets.kubeState | bool | Optional | `true` | when true, automatically install kube-state-metrics and scrape kube-state-metrics in the k8s cluster without any additional scrape config |
| scrapeTargets.nodeExporter | bool | Optional | `true` | `linux only` - when true, automatically install prometheus-node-exporter in every linux node in the k8s cluster and scrape node metrics without any additional scrape config |
| scrapeTargets.prometheusCollectorHealth | bool | Optional | `true` | when true, automatically scrape info about the Prometheus-Collector such as the amount and size of timeseries scraped |
| scrapeTargets.windowsExporter | bool | Optional | `false` | `windows only` - when true, will scrape windows node exporter in every windows node discovered in the cluster, without requiring any additional scrape configuration. Note:- Windows-exporter is not installed by this tool on windows node(s). You would need to install it by yourselves, before turning this ON |
| scrapeTargets.windowsKubeProxy | bool | Optional | `false` | `windows only` - when true, will scrape windows node's kubeproxy service, without requiring any additional scrape configuration, in every windows node discovered in the cluster. Note:- Windows kube-proxy metrics will soon be enabled on windows nodes for AKS clusters |
| keepListRegexes.coreDns | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for the coreDns service
| keepListRegexes.kubelet | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for kubelet
| keepListRegexes.cAdvisor | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for cAdvisor
| keepListRegexes.kubeProxy | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for kube-proxy
| keepListRegexes.apiServer | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for the kubernetes api server
| keepListRegexes.kubeState | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for kube-state metrics
| keepListRegexes.nodeExporter | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for node-exporter
| keepListRegexes.windowsExporter | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for windows exporter
| keepListRegexes.windowsKubeProxy | string | Optional | `""` | when set to a regex string, the collector only collects the metrics whose names match the regex pattern for windows kube-proxy
| prometheus-node-exporter.service.targetPort | INT | Optional | `true` | `linux only` - when a port is specified, node exporter uses this as bind/listen port, both prometheus-node-exporter.service.targetPort and prometheus-node-exporter.service.port should be set for this to work. |
| prometheus-node-exporter.service.port | INT | Optional | `true` | `linux only` - when a port is specified, node exporter uses this as bind/listen port |


----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.5.0](https://github.com/norwoodj/helm-docs/releases/v1.5.0)
