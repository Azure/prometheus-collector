# TSG: Pod Restarts and OOMKills

Run `tsg_errors` and `tsg_workload`. Then:

**ama-metrics ReplicaSet:**
1. Check if restarts are due to **authentication/connectivity issues** — run `tsg_errors`, look for `DCR/DCE/AMCS Configuration Errors`, `Liveness Probe Logs` with "No configuration present". Also run `tsg_logs` and check for repeated `TokenConfig.json does not exist`. If present, this is the **firewall/blocked endpoints** pattern — see TSG: Firewall / Network Connectivity below
2. Check if restarts are due to **OOMKilled** — run `tsg_workload`, check P95 CPU/Memory. If OtelCollector + MetricsExtension CPU/Memory is near container limits, pods are resource-starved
3. **Check system pool VM size** — run `tsg_triage`, look at "Node Pool Capacity" for the **System** mode pool. Note the `vmSize` (e.g., Standard_E4s_v5 = 32GB). ReplicaSet pods run exclusively on system pool nodes as a managed addon. Small system pool VMs are the most common cause of OOMKill with high metric volumes
4. **Check HPA status** — run `tsg_workload`, check "HPA Status" for `currentReplicas`, `maxReplicas`, and `atLimit` flag. The HPA automatically scales ReplicaSet pods to handle high metric volumes. If `atLimit == true`, HPA cannot scale further. Min/max replicas are adjustable by patching the `ama-metrics-hpa` HPA object in `kube-system` — see [Autoscaling docs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-autoscaling)
5. **Calculate if system pool can fit HPA replicas** — each ReplicaSet pod has a 14Gi memory limit (check "Pod Resource Limits" to confirm). Calculate: system pool nodes × node memory ÷ 14Gi = max pods. If HPA wants more replicas than the system pool can fit, pods will OOMKill. Example: 4 nodes × Standard_E4s_v5 (32GB) = 128GB → ~9 pods max at 14Gi each
6. **Check pod-to-node placement** — run `tsg_pods`, check "Pod to Node Mapping" and "System Pool Node Resources". Verify ReplicaSet pods are distributed across system pool nodes and that nodes aren't under MemoryPressure
7. **Check metric volume** — run `tsg_metric_insights`. If Istio/Envoy histogram `_bucket` metrics dominate (common: 50-90% of total volume), recommend dropping them via `metric_relabel_configs`. This is the most impactful mitigation
8. **Check pod resource limits** — run `tsg_workload`, check "Pod Resource Limits". ReplicaSet default: 500Mi req / 14Gi limit memory, 150m req / 7 CPU limit
9. **Check scrape interval** — aggressive intervals (e.g. 1s) in `ama-metrics-prometheus-config` configmap cause excessive load
10. **Check for double collection** — customer may have `podannotationnamespaceregex` set in `ama-metrics-settings-configmap` AND custom jobs scraping the same pod annotations
11. **Check relabelings** — ensure customer is using `relabel_configs` and `metric_relabel_configs` to scope scraping
12. **Resolution summary for OOMKills:**
    - **If system pool VMs are small (≤32GB)** → upgrade to larger VM size (Standard_E8s_v5 or larger)
    - **If metric volume is very high (>5M daily TS)** → reduce volume via `metric_relabel_configs` (drop `_bucket` histograms, reduce label cardinality)
    - **If HPA is at limit** → increase `maxReplicas` by patching the HPA: `kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"maxReplicas": <N>}}'`, but ONLY if system pool can accommodate more pods
    - **If system pool is at max nodes** → increase `maxCount` for the system pool autoscaler
13. **Customer documentation:**
    - [Troubleshoot Prometheus metrics collection](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-troubleshoot)
    - [Customize scraping of Prometheus metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration)
    - [Default Prometheus metrics configuration](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default)
14. **Detect HPA/OOMKill feedback loop** — run `tsg_workload`, check "HPA Scaling Metric and Oscillation". If replica count oscillates (e.g. 5↔15 repeatedly) rather than climbing steadily to max, this is the OOMKill feedback loop:
    - OOMKills reset pod memory to near-zero → HPA sees low average memory → HPA scales DOWN → fewer pods → higher per-pod load → more OOMKills → repeat
    - **Symptom:** HPA uses `ContainerResource` memory with `AverageValue` target (e.g. 5Gi). Check "HPA Metric Configuration" for the metric type
    - **Evidence:** Cluster autoscaler logs show "No unschedulable pods" (check "Cluster Autoscaler No Unschedulable Count"). Autoscaler never triggers because HPA never requests enough replicas to make pods unschedulable
    - **Fix:** Increase `minReplicas` on the `ama-metrics-hpa` HPA object to force a higher minimum replica count (e.g. 15-20), bypassing HPA's broken scaling signal: `kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"minReplicas": 15}}'` — see [Autoscaling docs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-autoscaling). Also reduce scrape targets to lower per-pod load

**ama-metrics-node DaemonSet (OOM is uncommon but has a specific root cause pattern):**
1. Check for aggressive scrape interval in `ama-metrics-prometheus-config-node`
2. Check if **Advanced Network Observability** is enabled — this can cause high memory usage. Mitigation: increase memory limits via AKS RP toggle
3. **Most common DaemonSet OOM cause: wrong configmap.** Check if the customer put cluster-wide scrape jobs in `ama-metrics-prometheus-config-node` instead of `ama-metrics-prometheus-config`. The node configmap (`-node` suffix) runs on every DaemonSet pod, so cluster-wide targets get scraped N times (once per node) instead of once. This causes massive duplication and OOMKills. **Fix:** move cluster-wide jobs to `ama-metrics-prometheus-config` (ReplicaSet configmap), keep only node-local targets (e.g. kubelet, node-exporter) in the `-node` configmap
4. Check `tsg_config` → look at "Configmaps", "Scrape Configs", and "Custom Scrape Jobs from Startup Logs" to see what jobs are in each configmap. The startup logs query shows which jobs were loaded at pod startup — if DaemonSet shows cluster-wide jobs like `kubernetes-pods` or `kube-state-metrics`, that confirms the wrong-configmap pattern. **Note:** startup logs only appear if pods restarted within the timeRange — use `timeRange='30d'` if needed
5. If DaemonSet pods are OOMing but ReplicaSet pods are healthy, the wrong-configmap pattern is almost certainly the cause

**ama-metrics-operator-targets:**
- Check if service discovery is not scoped to specific namespaces (e.g. kube-api-server endpoints should be scoped to `default` namespace)
- **Stakater Reloader / TLS certificate rotation crash loop** — this is the most common operator-targets failure pattern. Symptoms: thousands of restarts per day, every pod gets a different ReplicaSet name, `tls: bad certificate` errors in TA logs, collectors report "Failed to reach Target Allocator endpoint with HTTPS". The chain:
  1. The addon uses cert-manager-generated TLS secrets (`ama-metrics-operator-targets-server-tls-secret` and `client-tls-secret`) for mTLS between collectors and the target allocator
  2. If the customer has **Stakater Reloader** (or similar secret-watching controller) installed, it detects TLS secret updates and triggers deployment rollouts on `ama-metrics-operator-targets`
  3. The constant rollouts create new pods with new certs, but collector pods still hold stale client certs → TLS handshake failures → TA crashes (exit code 1)
  4. This creates a vicious cycle: secret change → rollout → crash → repeat (5,000+ restarts/day)
  5. **Impact:** Target allocator never stays running → no scrape target distribution → no kube-state-metrics data → recording rules empty. DaemonSet metrics (cadvisor, kubelet, node-exporter) are unaffected since they don't depend on the TA
  - **Detection:** Run `tsg_pods` — if `ama-metrics-operator-targets` has hundreds/thousands of restarts with `reason: Error` and many different ReplicaSet names, this is the pattern. Run `tsg_errors` → check "TargetAllocator Errors" for `tls: bad certificate` messages
  - **Fix (recommended):** Set `https_config = false` in `ama-metrics-settings-configmap` to disable TLS between TA and collectors entirely. This eliminates certificate generation, so there are no secrets for the reloader to trigger on:
    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: ama-metrics-settings-configmap
      namespace: kube-system
    data:
      prometheus-collector-settings: |-
        cluster_alias = ""
        https_config = false
    ```
    This setting is durable — it survives addon upgrades since the configmap is customer-managed. The tradeoff is that TA↔collector traffic is unencrypted within the cluster network (acceptable for most private AKS clusters)
  - **Alternative fix:** Exclude the operator-targets deployment from Stakater Reloader by annotating `reloader.stakater.com/auto=false`, but this annotation may be overwritten on addon upgrades (the deployment is Helm-managed)
  - **After applying either fix:** Delete the current operator-targets pod to force a clean start: `kubectl delete pod -n kube-system -l app.kubernetes.io/name=ama-metrics-operator-targets`
