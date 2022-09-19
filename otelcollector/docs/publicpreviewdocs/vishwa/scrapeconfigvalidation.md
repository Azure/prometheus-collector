## Scrape Configuration

Azure Monitor Prometheus agent does not understand/process operator CRDs (like PodMonitor, ServiceMonitor,...) for scrape configuration, but instead uses the native Prometheus configuration (yaml) as defined in Prometheus [configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config). Below are instructions how to provide custom scrape configuration for Azure Monitor Prometheus agent.

# Custom scrape configuration

In addition to the default scrape targets that Azure Monitor Prometheus agent scrapes by default, you can also provide additional scrape config to the agent thru a configmap. Below steps explains how to do it.

step-1 : Create a Prometheus scrape configuration file named 'prometheus-config'. See [here](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/docs/scrapeconfig/SCRAPECONFIG.md) for some samples/tips on authoring scrape config for Prometheus. You can also refer to [Prometheus.io](https://prometheus.io/) scrape configuration [reference](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config)

In prometheus-config, configuration file, add any custom scrape jobs. See the [Prometheus configuration docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) for more information. Your config file will list the scrape configs under the section `scrape_configs` and can optionally use the `global` section for setting the global `scrape_interval`, `scrape_timeout`, and `evaluation_interval`. Note:- Changes to global section (ex;- scrape interval) will impact default config and custom config. See a sample scrape config file [here](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/docs/scrapeconfig/samples/prometheus-config) and also below.

```yaml
global:
  evaluation_interval: 60s
  scrape_interval: 60s
scrape_configs:
- job_name: node
  scrape_interval: 30s
  scheme: http
  kubernetes_sd_configs:
    - role: endpoints
      namespaces:
        names:
        - node-exporter
  relabel_configs:
    - source_labels: [__meta_kubernetes_endpoints_name]
      action: keep
      regex: "dev-cluster-node-exporter-release-prometheus-node-exporter"
    - source_labels: [__metrics_path__]
      regex: (.*)
      target_label: metrics_path
    - source_labels: [__meta_kubernetes_endpoint_node_name]
      regex: (.*)
      target_label: instance

- job_name: kube-state-metrics
  scrape_interval: 30s
  static_configs:
    - targets: ['dev-cluster-kube-state-metrics-release.kube-state-metrics.svc.cluster.local:8080']
    
- job_name: prometheus_ref_app
  scheme: http
  kubernetes_sd_configs:
    - role: service
  relabel_configs:
    - source_labels: [__meta_kubernetes_service_name]
      action: keep
      regex: "prometheus-reference-service"
```

Step-2 : Validate the scrape config file using `promconfigvalidator` tool.

Once you have a custom  Prometheus scrape configuration, you can use our tool (promconfigvalidator) to validate your config, before creating it as a configmap that the agent addon can consume. promconfigvalidator tool is inside our addon container. You can use any of the ama-metrics-node-* pods in kube-system namespace in your cluster, to download the tool for validation. You woudl use `kubectl cp` for downloding the tool & its config as shown below.

Get the tool and config template from inside one of the ama-metrics containers -

```shell
    for podname in $(kubectl get pods -l rsName=ama-metrics -n=kube-system -o json | jq -r '.items[].metadata.name'); do kubectl cp -n=kube-system "${podname}":/opt/promconfigvalidator ./promconfigvalidator/promconfigvalidator;  kubectl cp -n=kube-system "${podname}":/opt/microsoft/otelcollector/collector-config-template.yml ./promconfigvalidator/collector-config-template.yml; done
```


Now you can validate the prometheus configuration using the promconfigvalidator tool that was downloaded using above instructions. This same tool is used by the agent to validate the config given to it thru the configmap. If the config is not valid, then the custom configuration given will not be used by the agent.

NOTE: This tool is supported only for Linux platform.


```shell
    ./promconfigvalidator --config "<full-path-to-prometheus-config-file>" --otelTemplate "collector-config-template-path"
```
This by default generates the merged configuration file 'merged-otel-config.yaml' if no paramater is provided using the optional --output paramater. Please do not use/give this merged file as config to the metrics collector agent, as this is only used for tool validation/debugging purposes.

Step-3 : Apply the config file as a config map `ama-metrics-prometheus-config` to the cluster in `kube-system` namespace.
Note: ensure the config file is named as 'prometheus-metrics' before running the below command as it uses file name as config map settign name.
You can also see a full 

```shell
kubectl create configmap ama-metrics-prometheus-config --from-file="full-path-to-prometheus-config-file" -n kube-system
```

The above will create a config map `ama-metrics-prometheus-config` in `kube-system` namespace, after which the azure monitor metrics pod will re-start to apply new config. You can look at any errors in config processing/merging by lookign at logs of the pod.

Also see a sample config-map [here](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/docs/scrapeconfig/samples/prometheus-config-configmap.yaml)