# Configure custom scrape jobs

In order to configure the azure monitor metrics addon to scrape targets other than the default targets, create this [configmap](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/configmaps/ama-metrics-prometheus-config-configmap.yaml) and update the prometheus-config section with your custom prometheus configuration. 
The format specified in the configmap will be the same as a prometheus.yml following the [configuration format](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Currently supported are the following sections:
```yaml
global:
  scrape_interval: <duration>
  scrape_timeout: <duration>
scrape_configs:
  - <scrape_config>
  ...
```
Before applying the configuration as a configmap, it is recommended that you validate it using the 'promconfigvalidator' tool, which is the same tool that is run at the container startup to perform validation of custom configuration. If the config is not valid, then the custom configuration given will not be used by the agent.
Please refer to these [instructions](https://github.com/Azure/prometheus-collector/blob/temp/documentation/otelcollector/docs/publicpreviewdocs/vishwa/scrapeconfigvalidation.md) on how to run the tool. 


Note that any other unsupported sections need to be removed from the config before applying as a configmap, else the promconfigvalidator tool validation will fail and as a result the custom scrape configuration will not be applied

The `scrape_config` setting `honor_labels` (`false` by default) should be `true` for scrape configs where labels that are normally added by Prometheus, such as `job` and `instance`, are already labels of the scraped metrics and should not be overridden. This is only applicable for cases like [federation](https://prometheus.io/docs/prometheus/latest/federation/) or scraping the [Pushgateway](https://github.com/prometheus/pushgateway), where the scraped metrics already have `job` and `instance` labels. See the [Prometheus documentation](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#scrape_config) for more details.

For more prometheus configuration tips refer to this [doc](https://github.com/Azure/prometheus-collector/blob/temp/documentation/otelcollector/docs/publicpreviewdocs/grace/custom-config-tips.md)