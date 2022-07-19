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
        - <my-node-exporter-namespace>
  relabel_configs:
    - source_labels: [__meta_kubernetes_endpoints_name]
      action: keep
      regex: "<my-node-exporter-release-name>-prometheus-node-exporter"
    - source_labels: [__metrics_path__]
      regex: (.*)
      target_label: metrics_path
    - source_labels: [__meta_kubernetes_endpoint_node_name]
      regex: (.*)
      target_label: instance
- job_name: kube-state-metrics
  scrape_interval: 30s
  static_configs:
    - targets: ['<my-kube-state-metrics-release-name>.<my-kube-state-metrics-namespace>.svc.cluster.local:8080']
```
