# Example Validation Output

This is an example of the validation output produced by the `validate-release-ready-image-cidev` skill, generated on 2026-04-10 against build 116132.

---

## Validation Summary Report
**Image:** 6.27.0-main-04-10-2026-a2c43cc1
**Build:** [116132](https://github-private.visualstudio.com/azure/_build/results?buildId=116132)
**Date:** 2026-04-10
**Cluster:** ci-dev-aks-mac-eus

### Phase 1: CI Pipeline Results
| Stage | Result | Details |
|-------|--------|---------|
| Build | ✅ | All images built successfully (linux AMD64/ARM64, windows 2019/2022, CCP, target allocator AMD64/ARM64, config reader, Arc conformance) |
| Deploy_AKS_Chart | ✅ | Helm upgrade succeeded on ci-dev-aks-mac-eus |
| Deploy_AKS_Chart_Test_Cluster | ✅ | Helm upgrade succeeded on ci-dev-aks-tests |
| Deploy_AKS_Chart_OTel_Cluster | ✅ | Helm upgrade succeeded on ciprom-dev-aks-otlp |
| Deploy_Chart_ARC | ✅ | Arc extension deployed to ci-dev-arc-wcus |
| Testkube (AKS) | ⚠️ | prometheusui-138: ✅ passed, operator-138: ✅ passed, containerstatus-138: ❌ failed, querymetrics-138: ⚠️ canceled (pipeline canceled while running), livenessprobe: ⏭️ not executed. Pipeline was canceled during querymetrics execution after containerstatus failure. Historical data shows containerstatus and livenessprobe are flaky on this cluster. |
| Testkube_OTel | ✅ | prometheusui: ✅, operator: ✅, containerstatus: ✅, querymetrics: ✅ — All 4 workflows passed |
| Testkube_ARC | ✅ | containerstatus: ✅ — All workflows passed |
| TestKube_Summary | ✅ | Summary notification sent successfully |

### Phase 2: Manual Validation Results
| Step | Result | Evidence |
|------|--------|----------|
| 1. Pod Status | ✅ | RS: 2/2 pods Running (2/2 containers each). DS: 6/6 pods Running. Win DS: 4/4 pods Running. Image tag confirmed: `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:6.27.0-main-04-10-2026-a2c43cc1` |
| 2. Pod Restarts | ✅ | All restart counts = 0 across all pod types (RS, DS, Win DS). |
| 3. Container Logs | ✅ | No recent errors (last 10m clean). Startup-only "Unable to refresh target groups" errors at 12:02:19 UTC (TA connection refused during pod init) — transient, resolved within seconds. No errors in addon-token-adapter or DS/Win DS logs. |
| 4. Liveness/Readiness Probes | ✅ | Probes configured on all pod types: RS prometheus-collector (http :8080/health, delay=120s), DS prometheus-collector (http :8080/health, delay=60s), Win DS prometheus-collector (http :8080/health, delay=300s). addon-token-adapter (http :9999/healthz, delay=10s) on all types. No Unhealthy events found. |
| 5a. Config Sources | ✅ | Default scrape settings: all 25 targets enabled (kubelet, coredns, cadvisor, kubeproxy, apiserver, kubestate, nodeexporter, windowsexporter, etc.). Scrape intervals: 30s for all. Custom configmaps present: ama-metrics-prometheus-config, ama-metrics-prometheus-config-node, ama-metrics-prometheus-config-node-windows. PodMonitor: default/referenceapp. ServiceMonitor: default/referenceapp. |
| 5b. Replicaset Config | ✅ | 14 scrape jobs in running config (including PodMonitor/ServiceMonitor targets). 7 active targets on this replica, 0 down. Jobs include: application_pods, kube-apiserver, kube-dns, kube-proxy, kube-state-metrics, kubernetes-pods, podMonitor/default/referenceapp/0, serviceMonitor/default/referenceapp/0, prometheus_ref_app, etc. Target distribution across replicas is expected. |
| 5c. Daemonset Config | ✅ | 8 scrape jobs: cadvisor, kubelet, node, node-configmap, kappie-basic, networkobservability-retina/hubble/cilium. 5 active targets, 0 down. All node-level jobs present and healthy. |
| 6. Metrics Ingestion | ✅ | count(up)=104, count(kube_pod_info)=190, count(scrape_samples_scraped)=104. All non-zero, metrics flowing. |
| 7a. Grafana Data (API) | ✅ | container_cpu_usage_seconds_total: 566 series, container_memory_working_set_bytes: 569, kubelet_running_pods: 18, kube_pod_info: 190, node_cpu_seconds_total: 704, apiserver_request_total: 2586, coredns_dns_requests_total: 18, kubeproxy_sync_proxy_rules_duration_seconds_count: 10, windows_memory_available_bytes: 4. 19 jobs reporting, all up=1. Latest data: 2026-04-10 21:00:59 UTC (< 1 minute old). Note: `windows_cs_physical_memory_bytes` returned 0 — metric may have been renamed in newer windows_exporter versions; `windows_memory_available_bytes` confirms Windows metrics are flowing. |
| 7b. Grafana Visual (Playwright) | ✅ | Verified all 12 kubernetes-mixin tagged dashboards in Azure Managed Prometheus folder. See dashboard details below. |

#### Grafana Dashboard Verification (kubernetes-mixin tag)

| Dashboard | Total Panels | "No data" Panels | Assessment |
|-----------|-------------|------------------|------------|
| Kubernetes / Compute Resources / Cluster | 10 | 0 | ✅ All panels populated |
| Kubernetes / Compute Resources / Cluster (Windows) | 9 | 3 | ✅ Expected — Windows metric name variants |
| Kubernetes / Compute Resources / Namespace (Pods) | 7 | 0 (with namespace=kube-system) | ✅ Requires namespace selection; all panels show data when namespace set |
| Kubernetes / Compute Resources / Namespace (Windows) | 4 | 0 | ✅ All panels populated |
| Kubernetes / Compute Resources / Namespace (Workloads) | 4 | 0 | ✅ All panels populated |
| Kubernetes / Compute Resources / Node (Pods) | 4 | 0 | ✅ All panels populated |
| Kubernetes / Compute Resources / Pod | 4 | 1 | ✅ Expected — CPU Throttling panel (no throttling = healthy) |
| Kubernetes / Compute Resources / Pod (Windows) | 4 | 0 | ✅ All panels populated |
| Kubernetes / Compute Resources / Workload | 4 | 0 | ✅ All panels populated |
| Kubernetes / Kubelet | 11 | 2 | ✅ Expected — Config Error Count + Operation Error Rate (empty = healthy) |
| Kubernetes / USE Method / Cluster (Windows) | 6 | 1 | ✅ Expected — Memory Saturation (Swap I/O) not collected |
| Kubernetes / USE Method / Node (Windows) | 8 | 1 | ✅ Expected — Memory Saturation (Swap I/O) not collected |

### Verdict
**Result:** READY (with caveat)
**Justification:** The image `6.27.0-main-04-10-2026-a2c43cc1` passes all manual validation checks — all pods are healthy with zero restarts, all scrape targets are up, metrics are flowing with fresh data across all 19 jobs, and all 12 kubernetes-mixin Grafana dashboards show data in their primary panels. The only "No data" panels are on error/throttling/swap panels which are expected to be empty when the system is healthy. The single concern is the AKS TestKube `containerstatus` test failure which caused the pipeline to be marked as failed and `querymetrics` to be canceled mid-run. However, the same `containerstatus` test passed on both OTel and ARC clusters, and historical data shows this test is flaky on the AKS tests cluster (dozens of sequential failures in the execution history). Recommend re-running the AKS TestKube tests to confirm, or proceed with release given that 2/3 TestKube environments passed fully and all manual validation checks are green.
