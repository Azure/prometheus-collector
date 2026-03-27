# Known Issues & FAQ

These are specific known behaviors and past incidents — not troubleshooting workflows, but useful context when a customer reports one of these patterns.

**HPA scaling down unexpectedly** — HPA scaling down is expected behavior when metric volume decreases (e.g., customer deployed a new app version that exposes fewer metrics). Check `tsg_workload` → "HPA Status". Customer can set `minshards` in `ama-metrics-settings-configmap` to prevent scaling below a minimum.

**Inconsistent cAdvisor scrape intervals** — cAdvisor scraping has known inconsistent intervals due to kubelet `/metrics/cadvisor` endpoint latency. Key investigation steps:
1. **Check scrape interval** — run `tsg_config`, look at "Default Targets Scrape Interval". cAdvisor defaults to **15s** — the most aggressive default target (others are 30-60s). This is the primary contributor to timeouts.
2. **Check per-pod sample variance** — run `tsg_workload`, look at "DaemonSet Per-Pod Sample Rate Variance". If `highVariance == true` (>100% difference between min/max pod rates), nodes have very different container counts. Nodes with more containers produce slower cadvisor responses.
3. **Check DaemonSet resource usage** — run `tsg_workload`, look at DaemonSet CPU/memory. If near limits (default: 500m CPU / 1Gi memory), the collector may not have enough resources to maintain consistent scrape timing.
4. **Root cause**: Kubelet's `/metrics/cadvisor` endpoint enumerates cgroup stats for ALL containers on the node — inherently slower than node-exporter (which reads static `/proc` files). When response time exceeds `scrape_timeout` (default 10s), the sample is silently dropped, creating gaps.
5. **Why node-exporter is unaffected**: Node-exporter reads static `/proc` and `/sys` files — near-instant. Kubelet cadvisor queries cgroups for every container — can take seconds on busy nodes.
6. **`scrape_duration_seconds` is NOT in our App Insights telemetry** — customer must verify via `kubectl port-forward <ama-metrics-node-pod> 9090` → query `scrape_duration_seconds{job="cadvisor"}` or check `/targets` page for "Last Scrape Duration".
7. **Recommendations**: Increase `scrape_timeout` for cadvisor to match interval (e.g. 15s), or increase cadvisor `scrape_interval` to 30-60s via `ama-metrics-settings-configmap`. This reduces kubelet load by 2-4x and eliminates most timeouts.
8. **This is a systemic kubelet behavior**, not a collector bug. Affects all clusters but is more pronounced on nodes with many containers (60+ pods/node).

**Post-rollout minimal ingestion profile regression (Aug 2025)** — A past addon release broke minimal ingestion profile logic, causing clusters without ConfigMaps to ingest ALL metrics. Symptoms: sudden CPU spike + ingestion increase after addon update. Workaround: deploy `ama-metrics-settings-configmap` with explicit `minimal-ingestion-profile: true`. If a new version causes similar regression, file Sev2 to `Container Insights/AzureManagedPrometheusAgent`.

**Tolerations blocking node drain** — Older addon versions had tolerations that prevented pod eviction during node drains/cluster upgrades. Fixed in recent releases. Workaround: manually delete the pod before draining. Fix: upgrade addon to latest.
