# Prometheus metrics in MDM FAQ

* [Prerequisites](./PromMDMfaq.md#prerequisites)
* [Azure Monitor Container Insights](./PromMDMfaq.md#azure-monitor-container-insights)
* [Coming Soon](./PromMDMfaq.md#coming-soon)
* [Limitations](./PromMDMfaq.md#known-issues)
* [Grafana questions](./PromMDMfaq.md#grafana-questions)
* [Checks](./PromMDMfaq.md#checks)
* [Release notes for Prometheus collector agent releases](./PromMDMReleaseNotes.md)

## Prerequisites

### Who can use Geneva-MDM backed Prometheus?

Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) for updates on this feature, including when this will roll out more broadly in Public Preview.

### What are some prerequisites to use Geneva-MDM backed Prometheus?

1. MDM account should be in **public cloud** region. We will support all regions soon.
2. Cluster's K8s versions should be > **1.16.x**
3. The MDM certificate should be stored in **Azure key-vault**, we only support Azure key-vault certificate based auth for ingesting metrics into metrics store(UA-MI will be coming soon).
4. The limited preview requires signing up for managed Grafana preview. Please reach out to [AzMonGrafanaTeam@microsoft.com](mailto:AzMonGrafanaTeam@microsoft.com) for instructions on how to sign up for that.

## Azure Monitor Container Insights

### I'm already using Azure Monitor container insights, how this is related with Azure Monitor container insights?

1. Azure monitor container insights is a 3P solution(for external customers) which provides container logs collection(stdout/stderr) & curated experience in Azure portal. Learn more [here](https://docs.microsoft.com/en-us/azure/azure-monitor/containers/container-insights-overview) 
2. Geneva-MDM backed Prometheus runs independently of container insights and collects Prometheus metrics and ingest in MDM account. We have plan to bring this functionality by end of 2021 to Azure monitor container insights.

## Coming Soon

### What are some of the limitations that are coming soon?

1. **Recording rules** support on raw Prometheus metrics collected.
2. **Alerting support** on Prometheus metrics.
3. Customizable MDM namespace for Prometheus metrics(currently fixed namespace for all Prom metrics)
4. Querying Prometheus metrics via SDK or KQL-M(currently PromQL only)
5. **Remote write** data from Prometheus server to MDM account.
6. No **up** metric for discovered targets.
7. Prometheus **Operator support**

## Known issues

### Data collection

1. Metrics with +-Inf and NaN values will be dropped (by design)
2. 'job' and 'instance' labels are reserved and cannot be relabled. If you either try to relabel 'job' & 'instance' labels, or try adding a label called 'job' or 'instance' (through re-labeling or external labels), it will fail the entire scrape output for that job, and no metrics will be ingested for that job. This is because the otelcollector's prometheus receiver tries to look up the target based on the new labels and fails to find them resulting in a runtime error. At present there is no fix for this. 
3. In the scrape config, `remote_write` and `groups` ( rule groups for recording & alerting rules) sections are un-supported. Please remove them from your custom scrape configuration, or else config validation will fail.


### Query

1. Query durations > 14d are blocked
2. Grafana Template functions
    * label_values(my_label) not supported due to cost of the query on MDM storage
        * Use label_values(my_metric, my_label)
3. Case-sensitivity
    * Due to limitations on MDM store (being case in-sensitive), query will do the following –
       * Any specific casing specified in the query for labels & values (non-regex), will be honored by the query service (meaning results returned will have the same casing)
       * For labels & values not specified in the query (including regex-based value matchers), query service will return results all in lower case

### Unsupported capabilities

1. You cannot query Prometheus metrics via Jarvis, we recommend customers to use Azure managed Grafana to access Prometheus metrics.
2. You will not be able to use IFx* libaries for instrumenting Prometheus metrics. For now use Prometheus SDK to instrument your workloads & in future we will support these capabilities in OTel(Open Telemetry SDK).
3. We will not support pre-aggregates & composite metrics in Prometheus metrics.

## Grafana questions

### These inbuilt Grafana dashboard have some changes than open-source dashboard: What are those changes?

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

## Checks

### How do I check the prometheus-collector logs?

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

### How do I check the Metrics Extension logs?

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
