---
applyTo: "**/deploy/**/*.yaml,**/deploy/**/*.yml,**/chart/**,**/addon-chart/**"
description: "Kubernetes and Helm chart conventions for prometheus-collector deployments."
---

# Kubernetes & Helm Conventions

- Helm chart templates use `-template.yaml` suffix (e.g., `Chart-template.yaml`, `values-template.yaml`) — these are generated at build time.
- DaemonSet is the primary deployment model for node-level metric collection.
- Deployment is used for the centralized target allocator and ReplicaSet mode.
- RBAC permissions are defined per chart — verify `clusterrole` and `clusterrolebinding` when adding new scrape targets.
- ConfigMaps store Prometheus scrape configurations and collector settings.
- Custom Resources (PodMonitor, ServiceMonitor) are used for operator-based target discovery.
- Test cluster manifests go under `otelcollector/test/test-cluster-yamls/`.
- Dependent charts (node-exporter, kube-state-metrics) are in `otelcollector/deploy/dependentcharts/`.
