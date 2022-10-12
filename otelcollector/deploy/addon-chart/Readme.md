cd prometheus-collector\otelcollector\deploy\addon-chart
helm install ama-metrics azure-monitor-metrics-addon/ --values azure-monitor-metrics-addon/Values.yaml

helm uninstall ama-metrics