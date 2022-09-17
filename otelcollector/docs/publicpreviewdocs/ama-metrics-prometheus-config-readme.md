# Configure custom scrape jobs

In order to configure the azure monitor metrics addon to scrape targets which is custom to your environment, create this [configmap](https://github.com/Azure/prometheus-collector/blob/rashmi/pub-preview-docs/otelcollector/deploy/ama-metric-settings-prometheus-config.yaml) and update the prometheus-config section with your custom prometheus configuration. 
The format specified in the configmap will be the same as a prometheus.yml following the [configuration format](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#configuration-file). Currently supported are the following sections:
```yaml
global:
  scrape_interval: <duration>
  scrape_timeout: <duration>
scrape_configs:
  - <scrape_config>
  ...
```
refer - https://raw.githubusercontent.com/Azure/prometheus-collector/main/otelcollector/docs/scrapeconfig/SCRAPECONFIG.md?token=GHSAT0AAAAAABR2DGC5P4PEB35E6LSSXLSMYZFK3KQ to conitnue

