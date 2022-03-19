# Prometheus metrics in MDM FAQ

Prometheus metrics in MDM is currently available to internal Microsoft teams as a private preview capability. Through 2022, we will learn from our preview customers, continue to add more cpabilities to the preview, and subsequently expand to public preview / GA. 

* [Private Preview - Prerequisites](./PromMDMfaq.md#private-preview---prerequisites)
* [Private Preview - Existing capabilities](./PromMDMfaq.md#private-preview---existing-capabilities)
* [Private Preview - upcoming](./PromMDMfaq.md#private-preview---upcoming)
* [Unsupported capabilities](./PromMDMfaq.md#unsupported-capabilities)
* [Data collection FAQ](./PromMDMfaq.md#data-collection-faq)
* [Query FAQ](./PromMDMfaq.md#promql-query-faq)
* [Grafana FAQ](./PromMDMfaq.md#grafana-faq)
* [Azure Monitor Container Insights and Prometheus MDM](./PromMDMfaq.md#azure-monitor-container-insights-and-prometheus-mdm)
* [Release notes for Prometheus collector agent releases](./PromMDMReleaseNotes.md)
* [Getting support](./PromMDMfaq.md#getting-support)

## Private Preview - Prerequisites

### Who can use Geneva-MDM backed Prometheus?

Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) for updates on the preview, and information about future milestones like public preview and GA. 

### What are some prerequisites to use Geneva-MDM backed Prometheus?

1. MDM account should be in **public cloud** region. We will support all regions in subsequent milestones.
2. Cluster's K8s versions should be > **1.16.x**
3. The MDM certificate should be stored in **Azure key-vault**, we only support Azure key-vault certificate based auth for ingesting metrics into metrics store(UA-MI will be coming soon).
4. The limited preview requires signing up for managed Grafana preview. Please reach out to [AzMonGrafanaTeam@microsoft.com](mailto:AzMonGrafanaTeam@microsoft.com) for instructions on how to sign up for that.

## Private Preview - Existing capabilities

* Ability to ingest all 4 Prometheus metric types, via an agent (Prometheus collector)
* Customizable metric collection config (service discovery supported), with scrape intervals up to 1 sec. 
* Store collected metrics in MDM accounts (currently accounts in shared MDM stamps are supported. Dedicated stamps will be supported by Mar 2022). 
* View ingested metrics in Grafana via a Prometheus data source and run queries using PromQL queries. Grafana will be available as both a managed offering (Azure Grafana Service - preview) or BYO (stand alone Grafana that you manage).

## Private Preview - upcoming

The following functionality will be added to the private preview through 2022

1. **Recording rules** support on raw Prometheus metrics collected.
2. **Alerting support** on Prometheus metrics.
3. **Remote write** data from Prometheus server to MDM account.
4. **up** metric for discovered targets.
5. Prometheus **Operator support**
6. Customizable MDM namespace for Prometheus metrics(currently fixed namespace for all Prom metrics)
7. Querying Prometheus metrics via SDK or KQL-M(currently PromQL only)

## Unsupported capabilities

1. You cannot query Prometheus metrics via Jarvis, we recommend customers to use Azure managed Grafana to access Prometheus metrics.
2. You will not be able to use IFx* libaries for instrumenting Prometheus metrics. For now use Prometheus SDK to instrument your workloads & in future we will support these capabilities via Open Telemetry metrics SDK).
3. We will not support pre-aggregates & composite metrics in Prometheus metrics.

## Data Collection FAQ

### Data Collection Known Issues

1. Metrics with +-Inf and NaN values will be dropped (by design)
2. 'job' and 'instance' labels are reserved and cannot be relabled. If you either try to relabel 'job' & 'instance' labels, or try adding a label called 'job' or 'instance' (through re-labeling or external labels), it will fail the entire scrape output for that job, and no metrics will be ingested for that job. At present there is no fix for this.
3. In the scrape config, `remote_write` and `groups` ( rule groups for recording & alerting rules) sections are un-supported. Please remove them from your custom scrape configuration, or else config validation will fail.

### Data Collection Checks

#### How do I check the prometheus-collector logs?

The prometheus-collector container prints logs at startup and errors from Metrics Extension.

```
kubectl logs $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector) -n <release-namespace>
```

This will have info about:

- What configmap settings were used.
- The result from running the promconfigvalidator check on a custom config:
  ```
  prom-config-validator::Config file provided - /etc/config/settings/prometheus/prometheus-config
  prom-config-validator::Successfully generated otel config
  prom-config-validator::Loading configuration...
  prom-config-validator::Successfully loaded and validated custom prometheus config
  ```

  This means the custom prometheus config is valid. Otherwise, the errors will be printed.

- The metric account names and results of decoding their certificates. 
- The following processes starting up: otelcollector, metricsextension, telegraf, and fluent-bit.
- Any Metrics Extension errors, including authentication, certificate, and ingestion issues.

#### How do I check the Metrics Extension logs?

ME logs are located at the root: `/MetricsExtensionConsoleDebugLog.log`. These are logs at the `INFO` level and include information about metrics received, processed, published, and dropped, as well as any errors. Access either by copying the file from the container:
```
kubectl cp $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector):MetricsExtensionConsoleDebugLog
.log MetricsExtensionConsoleDebugLog.log -n <release-namespace>
```
or exec-ing into the container:
```
kubectl exec -it $(kubectl get pods -n <release-namespace> -o custom-columns=NAME:.metadata.name | grep prometheus-collector) -n <release-namespace> -- bash
```

### Windows support

1. Currently below windows targets are included as default scrape targets, but they are not turned ON by default
   1. Windows exporter - Scraping this target is turned OFF by default. You would need to install Windows exporter manually in every windows host node (or automate installation using DSC in every windows host node in the cluster). See here for more information & tips on this.
   2. Windows kube proxy - Scraping this target is turned OFF by default. This will scrape kube-proxy service running on windows host nodes.
   3. You can see windows v. linux specific targets , and whats turned ON by default [here](~/metrics/prometheus/chartvalues.md)
2. Grafana dashboards for Windows -
      1. At present, 2 Windows exporter dashboards, showing windows node metrics, are included by default.

## PromQL Query FAQ

1. Query durations > 14d are blocked
2. Grafana Template functions
    * label_values(my_label) not supported due to cost of the query on MDM storage
        * Use label_values(my_metric, my_label)
3. Case-sensitivity
    * Due to limitations on MDM store (being case in-sensitive), query will do the following –
       * Any specific casing specified in the query for labels & values (non-regex), will be honored by the query service (meaning results returned will have the same casing)
       * For labels & values not specified in the query (including regex-based value matchers), query service will return results all in lower case

## Grafana FAQ

### Built-in Grafana dashboards have some changes over open-source dashboards: What are those changes?

1. Queries using metrics from recording rules needed to be updated for all Prometheus default dashboards
   * So far, out of the 20 default k8s-Prometheus dashboards, We changed below dashboards which were using recording rules –
      * api-server (1)
      * workloads* (4)
      * node exporter* (3)
      * other k8s mix-ins (3)
2. All mix-in dashboards have cluster-picker hidden, so we had to ‘un-hide’ it
3. Add cluster picker for other dashboards
   * node exporter (3)
   * kube-proxy (1)
   * kube-dns (1)

### Can I bring my own Grafana local instance & use Geneva-MDM backed Prometheus?

Yes, you can use this with your local Grafana instance. However we recommend you use managed Grafana. The managed Grafana instance comes with benefits, such as managed identity support and pre-created dashboards, to name a couple. Please let [AzMonGrafanaTeam@microsoft.com](mailto:AzMonGrafanaTeam@microsoft.com) know if you run into any issues.

## Azure Monitor Container Insights and Prometheus MDM

### I'm already using Azure Monitor container insights. How does this offering relate to Container Insights?

1. Azure monitor container insights is a 3P solution(for external customers) which provides container logs collection(stdout/stderr) & curated experience in Azure portal. Learn more [here](https://docs.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-overview) 
2. Geneva-MDM backed Prometheus runs independently of container insights and collects Prometheus metrics and ingest in MDM account. We have plan to subsequently bring this functionality to Azure monitor container insights.

## Getting Support

* [Teams channel](https://teams.microsoft.com/l/channel/19%3a0ee871c52d1744b0883e2d07f2066df0%40thread.skype/Prometheus%2520metrics%2520in%2520MDM%2520(Limited%2520Preview)?groupId=5658f840-c680-4882-93be-7cc69578f94e&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47)
* [ICM support](https://portal.microsofticm.com/imp/v3/incidents/create?tmpl=hcP1y3)
