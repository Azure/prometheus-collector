> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

# Deploy agent to Kubernetes cluster for metrics collection

For deploying the metrics collection agent, we will leverage [HELM](https://kubernetes.io/blog/2016/10/helm-charts-making-it-simple-to-package-and-deploy-apps-on-kubernetes/), specifically versions >= 3.7.0. 

```Note: Our charts will not work on HELM clients < 3.7.0```

## Install prometheus-collector chart in your cluster

The prometheus-collector is the name of the agent pod (replica set) that will collect Prometheus metrics from your Kubernetes cluster.

> If you've worked with Geneva Metrics before, you maybe familiar with the Geneva Metrics Extension [ME]. ME will be used for Prometheus collection as well, and is a sub-component of the prometheus-collector

To deploy the agent we will leverage HELM again. At this step you will need to provide the KeyVault certificate information that you saved in the previous step.  The following commands can be used for this. See an example of this below.  

> Note you must set the following environment variable for the below commands to work: HELM_EXPERIMENTAL_OCI=1

```shell
helm pull oci://mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector --version 1.1.2-main-03-07-2022-df71b65a
```

If using Service principal:
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.clientId="**" --set azureKeyVault.clientSecret="****" --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Service principal)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.clientId="70937f05-****-4fc0-****-de917f2a9402" --set azureKeyVault.clientSecret="**********************************" --namespace=prom-collector --create-namespace
```

If using Managed Identity (User Assigned): [See specifically, azureKeyVault.useManagedIdentity & azureKeyVault.userAssignedIdentityID parameters below]
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.useManagedIdentity=true --set azureKeyVault.userAssignedIdentityID="59677e05-****-4ea1-****-ed976f2b2049" --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Managed identity-user defined)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.useManagedIdentity=true --set azureKeyVault.userAssignedIdentityID="59677e05-****-4ea1-****-ed976f2b2049" --namespace=prom-collector --create-namespace
```

If using Managed Identity (System Assigned): [See specifically, azureKeyVault.useManagedIdentity parameter below]
```shell
helm upgrade --install <chart_release_name> ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="**" --set azureKeyVault.pfxCertNames="{**,**}" --set azureKeyVault.tenantId="**" --set clusterName="**" --set azureMetricAccount.defaultAccountName="**" --set azureKeyVault.useManagedIdentity=true --namespace=<my_prom_collector_namespace> --create-namespace
```
  **Example (Managed identity-system)** :-
```shell
helm upgrade --install my-collector-dev-release ./prometheus-collector-1.1.2-main-03-07-2022-df71b65a.tgz --dependency-update --set azureKeyVault.name="containerinsightstest1kv" --set azureKeyVault.pfxCertNames="{containerinsightsgenevaaccount1-pfx,containerinsightsgenevaaccount2-pfx}" --set azureKeyVault.tenantId="72f988bf-****-41af-****-2d7cd011db47" --set clusterName="mydevcluster" --set azureMetricAccount.defaultAccountName="containerinsightsgenevaaccount1" --set azureKeyVault.useManagedIdentity=true --namespace=prom-collector --create-namespace
```

See [chart values for Prometheus-collector](~/metrics/prometheus/chartvalues.md) for additional reference on how to customize more parameters like cpu/memory requests/limits etc..

Note: The deployment will also automatically install [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) and [node exporter](https://github.com/prometheus/node_exporter) which are popular tools to collect infrastructure metrics. These will be shown in the default dashboards that we will look at later in this tutorial. Please check the [chartValues](~/metrics/prometheus/chartvalues.md) page to see how to override the default ports for node exporter.

You can verify that there are not any configuration or authentication issues by looking at the prometheus-collector logs. See the [FAQ](~/metrics/Prometheus/PromMDMfaq.md#how-do-i-check-the-prometheus-collector-logs) for how to do so.

Enabling `--set advanced.mode=true` for large clusters with more than 50 nodes and 1500 pods is highly recommended. See [here](~/metrics/Prometheus/advanced-mode.md) for more information about advanced mode. If the cluster has greater than 25 Windows nodes, enabling advanced mode and `--set windowsDaemonset=true` is recommended. See [here](~/metrics/Prometheus/windows.md) for more information about collecting Windows metrics.

> If you want to have your metrics be sent to multiple metrics accounts, follow the guidelines for [multiple accounts](~/metrics/Prometheus/configuration.md#multiple-metric-accounts) that outlines how Prometheus collector works with multiple metrics accounts.  

--------------------------------------

In this step you deployed the agent and exporters for for collecting metrics from your Kubernetes cluster.  

Next, you will configure what metrics should be collected. [Configure metrics collection](~/metrics/prometheus/PromMDMTutorial3ConfigureCollection.md)
