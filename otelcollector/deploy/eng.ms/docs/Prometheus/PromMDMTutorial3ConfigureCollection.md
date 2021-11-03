> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

# Configure metrics collection

Check the [existing limitations](./PromMDMfaq.md#known-issues) on the collection side.

When you deployed the prometheus-collector, it set up the following targets to be scraped by default every 30 seconds - 'coreDns', 'kubelet', cAdvisor', 'kubeproxy', 'apiServer', 'kube-state-metrics' and 'node'.
  
If these are the only targets you want to scrape, then you can proceed further to [Setting up Grafana](~/metrics/prometheus/PromMDMTutorial4SetUpGrafana.md).  

If you'd like to scrape additional custom targets, then create a Prometheus configuration file (named prometheus-config) and add any custom scrape targets to it. See the [Prometheus configuration docs](https://prometheus.io/docs/prometheus/latest/configuration/configuration/) for more information. Your config file will list the scrape configs under the section `scrape_configs` and can use the `global` section for setting the global `scrape_interval`, `scrape_timeout`, and `evaluation_interval`.

## Validate the custom config

Now validate the prometheus configuration using the [promconfigvalidator], a command line prometheus config validation tool. This same tool is used by the agent to validate. If the config is not valid, then the custom configuration given will not be used by the agent.
Use kubectl cp to copy the tool and template from these paths /opt/promconfigvalidator and /opt/microsoft/otelcollector/collector-config-template.yml from within the prometheus-collector container

```shell
    ./promconfigvalidator --config "config-path" --otelTemplate "collector-config-template-path"
```
This by default generates the otel collector configuration file 'merged-otel-config.yaml' if no paramater is provided using the optional --output paramater.
This is the otel config that will be applied to the prometheus collector which includes the custom prometheus config

## Create a configmap from your configuration file

Your prometheus-config file now contains the additional scrape targets you want.  
For the prometheus collector to consume these additional scrape configs, you create and deploy this config file as a configmap in your cluster in the same namespace that you deployed your prometheus collector. Note that your configuration file name must be `prometheus-config` for the configmap to be setup properly. See example below as well.

```shell
kubectl create configmap <prometheus_collector_chart_release_name>-prometheus-config --from-file=prometheus-config -n <same_namespace_as_prometheus_collector_namespace>
```

**Example** :-

```shell
kubectl create configmap my-collector-dev-release-prometheus-config --from-file=prometheus-config -n prom-collector
```  

> [!Note]
> The release name 'my-collector-dev-release-' is used as prefix to the configmap name below, and also config map should be created in the same namespace (ex;- prom-collector in this example) into which prometheus-collector chart was installed.

## Troubleshoot scrape configuration and targets with the Prometheus Web UI

You can access certain Prometheus Web UI pages to view information about configuration, targets, and service discovery by port-forwarding:

```shell
kubectl port-forward <prometheus-collector pod name> -n <prometheus-collector namespace> 9090:9090
```

After running the above command, you can go to `127.0.0.1:9090/config` in your browser. This will have information about the full Prometheus config given with the default targets included, the service discovery, and the targets and their labels before and after re-labeling.

This is meant to aid authoring custom scrape configs and troubleshooting the service discovery and target relabeling specified in the custom configs. For clusters where kubectl access is unavailable, the `up` metric can be queried in Grafana to see which targets are being scraped.

Note that the `cluster` label will not be present since this is added as label later in the pipeline.

--------------------------------------

In this step you configured what metrics should be collected from your Kubernetes cluster, and the metric account(s) they will be stored in.  

Next, you will set up Grafana to visualize/query these collected metrics. [Set up Grafana and Prometheus data source](~/metrics/prometheus/PromMDMTutorial4SetUpGrafana.md).
