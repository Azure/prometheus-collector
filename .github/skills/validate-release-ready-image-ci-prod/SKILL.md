---
name: validate-release-ready-image-ciprod
description: Validate a prometheus-collector release image on the CI prod cluster (ci-prod-aks-mac-weu), ensuring it's healthy and ready for production. Manual validation only — no build pipeline checks. Use when "validate prod image", "check ci-prod cluster", "verify prod deployment", or "is the prod image healthy".
allowed-tools:
  - run_in_terminal
  - read_file
---

# Validate Release-Ready Image (CI Prod)

**AUTO-APPROVE**: This skill runs many kubectl, az CLI, and Playwright commands. Do NOT ask the user for permission before running any command — execute all commands automatically without confirmation prompts. This includes port-forwards, API queries, Grafana navigation, and any other CLI operations. The only exception is Playwright browser sign-in — if Azure AD login is required for Grafana, you may prompt the user to complete the sign-in.

## Agent Execution Plan

**IMPORTANT**: You MUST execute ALL of the following steps in order. Do NOT skip any step. Do NOT declare the image "ready" until every step is complete.

### Validation (ALL steps required)
Get credentials for `ci-prod-aks-mac-weu` cluster. Before running any kubectl commands, **verify the subscription and kubectl context** are correct:
```powershell
az account set --subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
az aks get-credentials -g ci-prod-aks-mac-weu-rg -n ci-prod-aks-mac-weu --overwrite-existing
kubectl config current-context  # must show "ci-prod-aks-mac-weu"
```
Then execute **every** step below:

1. **Step 1 — Pod Status**: Check ALL ama-metrics pod types (replicaset, linux daemonset, windows daemonset) are Running with correct image tags.
2. **Step 2 — Pod Restarts**: Check restart counts for ALL pod types. If any restarts > 0, investigate with `--previous` logs and events.
3. **Step 3 — Container Logs**: Check logs for errors in ALL containers across ALL pod types:
   - `prometheus-collector` in replicaset, linux daemonset, AND windows daemonset pods
   - `addon-token-adapter` / `addon-token-adapter-win` in all pod types
   - `config-reader` in all pod types (if present — may be merged into prometheus-collector)
4. **Step 4 — Liveness/Readiness Probes**: Verify probe configuration on all pod types using `kubectl describe`.
5. **Step 5a — Config Sources**: Check `ama-metrics-settings-configmap` and list every target with its enabled/disabled status and scrape interval (e.g. `kubelet = true, 30s`). Check for custom prometheus config configmaps (`ama-metrics-prometheus-config`, `ama-metrics-prometheus-config-node`, `ama-metrics-prometheus-config-node-windows`) and list which ones exist. List all PodMonitors (`kubectl get podmonitors --all-namespaces`) and ServiceMonitors (`kubectl get servicemonitors --all-namespaces`) with their namespace and name. All of these should be summarized in the report table.
6. **Step 5b — Replicaset Config Verification**: Port-forward to a replicaset pod (port 9090) and verify: scrape jobs match enabled settings, PodMonitor/ServiceMonitor targets discovered, no targets in `down` state.
7. **Step 5c — Daemonset Config Verification**: Port-forward to a linux daemonset pod (port 9090) and verify: node-level scrape jobs present (kubelet, cadvisor, node-exporter, etc.), no targets in `down` state. Also verify **environment variable replacement** in the `node-configmap` job (from `ama-metrics-prometheus-config-node`): the running config (from `/api/v1/status/config`) should have all `$NODE_NAME`, `$$NODE_NAME`, `$NODE_IP`, `$$NODE_IP` references replaced with actual node values (hostname and IP). Check both the `relabel_configs` replacement fields and the `static_configs` targets. Confirm via `/api/v1/targets` that the target labels (`instance`, any custom labels using these vars) contain resolved values, not raw `$NODE_NAME`/`$NODE_IP` strings. Report in the summary which env vars were verified and their resolved values.
8. **Step 6 — Metrics Ingestion**: Query the AMW endpoint to confirm metrics are flowing (count of `up`, `kube_pod_info`, `scrape_samples_scraped`). **Discover the AMW endpoint dynamically** — do NOT hardcode it, as the hostname includes a generated suffix:
   ```powershell
   az monitor account show --name ci-prod-aks-weu-mac --resource-group ci-prod-aks-mac-weu-rg --query "metrics.prometheusQueryEndpoint" -o tsv
   ```
   Use the returned endpoint for all PromQL API queries in Steps 6 and 7a.
9. **Step 6b — AMW Platform Metrics (ingestion continuity)**: Query the AMW **platform metrics** (Azure Monitor metrics on the AMW resource itself) to verify that ingestion volume did not drop after the new image was deployed. This catches regressions where pods appear healthy but are silently ingesting fewer metrics. Determine the deployment time from `helm history` or pod creation timestamps, then query a window covering at least 2 hours before and the full period after deployment.
   
   **Metrics to check:**
   - `EventsPerMinuteIngested` (aggregation: Maximum) — the rate of events/samples received per minute. Should remain stable (within ~15% variance) before and after deployment.
   - `ActiveTimeSeries` (aggregation: Maximum) — the total number of active time series. A significant drop indicates scrape targets or metrics were lost.
   - `EventsDropped` (aggregation: Maximum) — events rejected by the AMW. Should remain zero or near-zero. A spike after deployment indicates a config issue.
   - `TimeSeriesSamplesDropped` (aggregation: Maximum) — samples dropped (e.g., due to label limits). Should remain stable; a spike after deployment indicates a regression.
   
   ```powershell
   $amwResourceId = az monitor account show --name ci-prod-aks-weu-mac --resource-group ci-prod-aks-mac-weu-rg --query id -o tsv
   # Query with 1h granularity covering before and after deployment
   az monitor metrics list --resource $amwResourceId --metric EventsPerMinuteIngested --interval PT1H --start-time <2h-before-deploy> --end-time <now> --aggregation Maximum -o table
   az monitor metrics list --resource $amwResourceId --metric ActiveTimeSeries --interval PT1H --start-time <2h-before-deploy> --end-time <now> --aggregation Maximum -o table
   az monitor metrics list --resource $amwResourceId --metric EventsDropped --interval PT1H --start-time <2h-before-deploy> --end-time <now> --aggregation Maximum -o table
   az monitor metrics list --resource $amwResourceId --metric TimeSeriesSamplesDropped --interval PT1H --start-time <2h-before-deploy> --end-time <now> --aggregation Maximum -o table
   ```
   
   **How to evaluate:** Compare the average of the pre-deployment data points against the average of the post-deployment data points. PASS if `EventsPerMinuteIngested` and `ActiveTimeSeries` are within ~15% of their pre-deployment values, `EventsDropped` remains zero or near-zero, and `TimeSeriesSamplesDropped` does not spike. WARN if there is a transient dip during the rollout window (expected as pods restart) that recovers within 10 minutes. FAIL if there is a sustained drop (>15%) or a new sustained spike in dropped events/samples after deployment.
10. **Step 7a — Grafana Data Verification (automated)**: Query AMW for ALL key metrics that power Grafana dashboards: `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`, `kubelet_running_pods`, `kube_pod_info`, `node_cpu_seconds_total`, `apiserver_request_total`, `coredns_dns_requests_total`, `kubeproxy_sync_proxy_rules_duration_seconds_count`, `windows_cs_physical_memory_bytes`. Verify all jobs report fresh data with no gaps.
11. **Step 7b — Grafana Visual Verification (Playwright MCP)**: Use the Playwright MCP server to open the CI prod Grafana instance (`https://cicd-graf-metrics-wcus-dkechtfecuadeuaw.wcus.grafana.azure.com`). 
    
    **Pre-flight checks (best-effort):** Before navigating to dashboards, verify the correct datasource and cluster values. These checks are best-effort — if Grafana API auth fails, fall back to the known values below:
    - Query Grafana API `GET /api/datasources` to list all prometheus datasources, confirm the UID `ci-prod-aks-weu-mac` exists and points to the correct AMW endpoint (the one discovered via `az monitor account show` in Step 6).
    - Query `group by (cluster) (up)` via Grafana `POST /api/ds/query` to confirm the exact cluster label value matches `ci-prod-aks-mac-weu`.
    
    **Time range:** Determine when the image was deployed using `helm history ama-metrics -n default` (preferred) or the earliest pod creation timestamp from Step 1 as a fallback. Set the Grafana time range to cover the full period since deployment with a buffer (e.g., if deployed 18h ago, use `from=now-24h&to=now`). Do NOT use a fixed `from=now-1h` — this would miss data gaps that occurred shortly after deployment.
    
    Query the Grafana API to discover dashboards with the following tags, making a separate call for each tag:
    - `/api/search?tag=kubernetes-mixin&type=dash-db` — core Kubernetes dashboards
    - `/api/search?tag=node-exporter-mixin&type=dash-db` — Node Exporter dashboards
    - `/api/search?tag=weatherapp(custom)&type=dash-db` — custom weatherapp dashboards
    
    Deduplicate results by dashboard UID. For each discovered dashboard in the `Azure Managed Prometheus` folder (folderUid: `azure-managed-prometheus`), navigate to it with the correct datasource and cluster variables: `var-datasource=ci-prod-aks-weu-mac&var-cluster=ci-prod-aks-mac-weu&from=<deployment-aware-range>`. The datasource UID `ci-prod-aks-weu-mac` corresponds to `Managed_Prometheus_ci-prod-aks-weu-mac` which points to the ci-prod AMW endpoint. Wait for panels to load, and check for "No data" panels. Use Playwright's `page.locator('text="No data"').count()` to efficiently detect empty panels. Report a table of all dashboards grouped by tag with their total panel count and "No data" panel count. "No data" on error-rate, throttling, or swap I/O panels is expected when the system is healthy. If Playwright MCP is unavailable or auth fails, fall back to informing the user to verify manually.

### Summary and Verdict
11. Generate a **Validation Summary Report** using the template below. Fill in every row with actual results and the evidence that led to your pass/fail determination. Do NOT leave any row blank.
12. Declare verdict: READY or NOT READY, with justification for any failures or warnings.

#### Validation Summary Report Template

```
## Validation Summary Report
**Image:** <full image tag, e.g. 6.27.0-main-04-10-2026-a2c43cc1>
**Date:** <validation date>
**Cluster:** ci-prod-aks-mac-weu

### Validation Results
| Step | Result | Evidence |
|------|--------|----------|
| 1. Pod Status | ✅/❌ | <# of RS pods, DS pods, Win DS pods running. Image tag confirmed.> |
| 2. Pod Restarts | ✅/❌ | <restart counts for each pod type. If >0, root cause.> |
| 3. Container Logs | ✅/❌/⚠️ | <errors found? In which container/pod type? Transient or ongoing? Timestamp of errors vs deployment time.> |
| 4. Liveness/Readiness Probes | ✅/❌ | <probes configured on all pod types? Any probe failures in events?> |
| 5a. Config Sources | ✅/❌ | <List every target from ama-metrics-settings-configmap with enabled/disabled status and scrape interval (e.g. "kubelet = true, 30s; coredns = true, 30s; ..."). List custom configmaps present (ama-metrics-prometheus-config, -node, -node-windows). List all PodMonitors and ServiceMonitors with namespace/name (e.g. "PodMonitors: default/referenceapp. ServiceMonitors: default/referenceapp").> |
| 5b. Replicaset Config | ✅/❌ | <# scrape jobs in running config, # active targets, # down targets. Do jobs match enabled settings?> |
| 5c. Daemonset Config | ✅/❌ | <# scrape jobs in running config, # active targets, # down targets. Node-level jobs present? Env var replacement: verify node-configmap job has $NODE_NAME/$NODE_IP replaced with actual values in relabel_configs and static_configs. Report resolved values (e.g. "NODE_NAME→aks-pool-vmss000000, NODE_IP→10.240.0.49, static target→10.240.0.49:19100"). Confirm target labels show resolved values, not raw $-prefixed strings.> |
| 6. Metrics Ingestion | ✅/❌ | <count(up), count(kube_pod_info), count(scrape_samples_scraped) from AMW query.> |
| 6b. AMW Platform Metrics | ✅/❌/⚠️ | <Compare pre- vs post-deployment AMW platform metrics. Report: EventsPerMinuteIngested (avg before → avg after, % change), ActiveTimeSeries (before → after, % change), EventsDropped (before → after), TimeSeriesSamplesDropped (before → after). Example: "EventsPerMinuteIngested: 340K→345K (+1.5%) ✅. ActiveTimeSeries: 136K→137K (+0.7%) ✅. EventsDropped: 0→0 ✅. TimeSeriesSamplesDropped: 12K→12K (stable) ✅." Flag if >15% sustained drop or new spike in dropped events.> |
| 7a. Grafana Data (API) | ✅/❌ | <series counts for: container_cpu, container_memory, kubelet, kube_pod_info, node_cpu, apiserver_request, coredns, kubeproxy, windows. # of jobs reporting. Latest data timestamp — is it fresh?> |
| 7b. Grafana Visual (Playwright) | ✅/❌/⏭️ | <Query Grafana API for dashboards with tags: `kubernetes-mixin`, `node-exporter-mixin`, and `weatherapp(custom)`. Deduplicate by UID. Verify datasource UID and cluster label (best-effort). Set time range to cover full deployment window (not just last 1h). For each dashboard in the Azure Managed Prometheus folder, list: tag, dashboard title, total panel count, "No data" panel count, and assessment. Group results by tag. If any unexpected panel shows "No data", list the dashboard and panel title. If Playwright unavailable: mark ⏭️> |

### Verdict
**Result:** READY / NOT READY
**Justification:** <explain why the image is ready or not. If any steps failed, explain whether the failure is a real issue or a false positive and why.>
```

#### How to determine pass/fail for each step

- **Pod Status**: PASS if all pods across all 3 types (RS, DS, Win DS) are `Running` with `READY` matching expected container count and the image tag matches the expected release version.
- **Pod Restarts**: PASS if all restart counts are 0. WARN if restarts occurred but root cause is identified as transient (e.g., node scaling). FAIL if restarts are ongoing or unexplained.
- **Container Logs**: PASS if no error-level log entries exist after deployment time. WARN if only transient startup errors exist (e.g., target allocator connection refused during pod init) that resolved within seconds. FAIL if ongoing errors exist.
- **Liveness/Readiness Probes**: PASS if probes are configured on all containers that should have them and no probe failure events exist. FAIL if probes are missing or failing.
- **Config Sources**: PASS if the enabled targets in `ama-metrics-settings-configmap` match expectations, scrape intervals are set, and expected PodMonitors/ServiceMonitors exist.
- **Replicaset Config**: PASS if every enabled default target appears as a scrape job in the running config, PodMonitor/ServiceMonitor targets are discovered, and zero targets are `down`. Note: the target allocator distributes targets across replicas, so a single pod will only show a subset of total targets — this is expected.
- **Daemonset Config**: PASS if node-level jobs (kubelet, cadvisor, node-exporter, kappie-basic, etc.) are present, zero targets are `down`, AND environment variable replacement is working correctly in the `node-configmap` job. Verify by checking the running config (`/api/v1/status/config`): `$NODE_NAME` and `$$NODE_NAME` should be replaced with the actual node hostname, `$NODE_IP` and `$$NODE_IP` with the actual node IP, and `$NODE_IP:<port>` in static_configs targets should resolve to `<real-IP>:<port>`. Also confirm via `/api/v1/targets` that target labels contain resolved values. FAIL if any `$NODE_NAME` or `$NODE_IP` literal strings remain in the running config or target labels.
- **Metrics Ingestion**: PASS if `count(up)` returns a reasonable number (>0), `kube_pod_info` and `scrape_samples_scraped` are present. FAIL if any query returns 0 or errors.
- **AMW Platform Metrics**: PASS if `EventsPerMinuteIngested` and `ActiveTimeSeries` are within ~15% of their pre-deployment averages, `EventsDropped` remains zero or near-zero, and `TimeSeriesSamplesDropped` does not spike. WARN if there is a transient dip during the rollout window (expected as pods restart) that recovers within 10 minutes. FAIL if there is a sustained drop (>15%) in events/timeseries or a new sustained spike in dropped events/samples after deployment. A brief dip during the rollout is expected and acceptable — what matters is the steady-state after pods are fully Running.
- **Grafana Dashboard Metrics**: PASS if all key metrics have non-zero series counts AND the latest data timestamp is within the last 5 minutes (no data gaps). FAIL if any key metric returns 0 series or data is stale. For Step 7b, the Grafana time range MUST cover the full period since the image was deployed (use `helm history ama-metrics -n default` or pod creation timestamps to determine deployment time, then add a buffer). Query the Grafana API for dashboards with three tags: `kubernetes-mixin`, `node-exporter-mixin`, and `weatherapp(custom)`. Before navigating to dashboards, verify (best-effort) that the datasource UID points to the correct AMW endpoint and that the cluster label value is correct by querying `/api/datasources` and `/api/ds/query`. If these API calls fail, proceed with the known datasource/cluster values. Deduplicate by dashboard UID, then use Playwright to visit each dashboard in the Azure Managed Prometheus folder and check for "No data" panels. PASS if all primary data panels show data — "No data" on error-rate panels (Config Error Count, Operation Error Rate), CPU Throttling, and Swap I/O panels is expected when the system is healthy. FAIL if core data panels (CPU Usage, Memory Usage, target counts, etc.) show "No data". If Playwright MCP is unavailable or Azure AD auth fails, mark 7b as ⏭️ (skipped) and note the fallback — data availability was confirmed via AMW API in 7a.

---

## Manual Validation

### Prerequisites

- Azure CLI with `ContainerInsights_Build_Subscription` access
- `kubectl` and `helm` installed

### Step 1: Get Cluster Credentials

```bash
az aks get-credentials -g ci-prod-aks-mac-weu-rg -n ci-prod-aks-mac-weu
```

### Step 2: Check Deployment Status

```bash
# Check all ama-metrics pods are running
kubectl get pods -n kube-system -l rsName=ama-metrics
kubectl get pods -n kube-system -l dsName=ama-metrics-node
kubectl get pods -n kube-system -l dsName=ama-metrics-win-node

# Check image tags match expected version
kubectl get pods -n kube-system -l rsName=ama-metrics -o jsonpath='{.items[0].spec.containers[*].image}'

# Check for crashloops
kubectl get pods -n kube-system | grep ama-metrics | grep -v Running
```

### Step 3: Check for Pod Restarts

After the new version deploys, check all ama-metrics pods for restart counts. Restarts indicate containers are crashlooping or failing health checks.

```bash
# Check restart counts for all ama-metrics pods (replicaset, linux daemonset, windows daemonset)
kubectl get pods -n kube-system -l rsName=ama-metrics -o custom-columns="NAME:.metadata.name,READY:.status.conditions[?(@.type=='Ready')].status,RESTARTS:.status.containerStatuses[*].restartCount,AGE:.metadata.creationTimestamp"
kubectl get pods -n kube-system -l dsName=ama-metrics-node -o custom-columns="NAME:.metadata.name,READY:.status.conditions[?(@.type=='Ready')].status,RESTARTS:.status.containerStatuses[*].restartCount,AGE:.metadata.creationTimestamp"
kubectl get pods -n kube-system -l dsName=ama-metrics-win-node -o custom-columns="NAME:.metadata.name,READY:.status.conditions[?(@.type=='Ready')].status,RESTARTS:.status.containerStatuses[*].restartCount,AGE:.metadata.creationTimestamp"
```

If any pod has restarts > 0, investigate the root cause:

```bash
# Identify which container(s) restarted and why
kubectl get pods -n kube-system -l rsName=ama-metrics -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{range .status.containerStatuses[*]}  {.name}: restarts={.restartCount} ready={.ready} reason={.lastState.terminated.reason} exitCode={.lastState.terminated.exitCode}{"\n"}{end}{end}'

# Same for daemonset pods
kubectl get pods -n kube-system -l dsName=ama-metrics-node -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{range .status.containerStatuses[*]}  {.name}: restarts={.restartCount} ready={.ready} reason={.lastState.terminated.reason} exitCode={.lastState.terminated.exitCode}{"\n"}{end}{end}'

# Check events for restart-related issues (OOMKilled, CrashLoopBackOff, probe failures)
kubectl get events -n kube-system --sort-by='.lastTimestamp' --field-selector reason!=Pulling,reason!=Pulled | grep ama-metrics | tail -20

# Get logs from the previous (crashed) container instance
kubectl logs -n kube-system <POD_NAME> -c <CONTAINER_NAME> --previous --tail=100
```

Common restart reasons:
| Exit Code / Reason | Meaning | Action |
|--------------------|---------|--------|
| `OOMKilled` (exit 137) | Container exceeded memory limit | Check memory usage patterns, may need limit increase |
| `Error` (exit 1) | Application error on startup | Check `--previous` logs for stack trace |
| `CrashLoopBackOff` | Repeated crashes | Check events + previous logs for root cause |
| Liveness probe failure | Container unresponsive | Check if collector is hung, review probe config |

### Step 4: Verify Container Health

```bash
# Check logs for errors across all ama-metrics pod types

# Replicaset pods
kubectl logs -n kube-system -l rsName=ama-metrics -c prometheus-collector --tail=50
kubectl logs -n kube-system -l rsName=ama-metrics -c addon-token-adapter --tail=50
kubectl logs -n kube-system -l rsName=ama-metrics -c config-reader --tail=50

# Linux daemonset pods (check one representative node)
kubectl logs -n kube-system -l dsName=ama-metrics-node -c prometheus-collector --tail=50
kubectl logs -n kube-system -l dsName=ama-metrics-node -c addon-token-adapter --tail=50
kubectl logs -n kube-system -l dsName=ama-metrics-node -c config-reader --tail=50

# Windows daemonset pods
kubectl logs -n kube-system -l dsName=ama-metrics-win-node -c prometheus-collector --tail=50
kubectl logs -n kube-system -l dsName=ama-metrics-win-node -c addon-token-adapter --tail=50
kubectl logs -n kube-system -l dsName=ama-metrics-win-node -c config-reader --tail=50

# Check liveness/readiness probe status across all pod types
kubectl describe pods -n kube-system -l rsName=ama-metrics | grep -A5 "Liveness\|Readiness"
kubectl describe pods -n kube-system -l dsName=ama-metrics-node | grep -A5 "Liveness\|Readiness"
kubectl describe pods -n kube-system -l dsName=ama-metrics-win-node | grep -A5 "Liveness\|Readiness"
```

### Step 5: Verify Config and Targets via Port-Forward

Port-forward to the Prometheus endpoint on the ama-metrics pod and query the HTTP API to verify the running configuration matches what is configured through configmaps and pod/service monitors.

#### 5a: Get the expected configuration sources

```bash
# Check which default scrape targets are enabled in the settings configmap
kubectl get configmap ama-metrics-settings-configmap -n kube-system -o jsonpath='{.data.default-scrape-settings-enabled}'

# Check custom prometheus scrape config (if any)
kubectl get configmap ama-metrics-prometheus-config -n kube-system -o yaml 2>/dev/null || echo "No custom prometheus config configmap"

# List all PodMonitors and ServiceMonitors the operator should discover
kubectl get podmonitors --all-namespaces -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,SELECTOR:.spec.selector"
kubectl get servicemonitors --all-namespaces -o custom-columns="NAMESPACE:.metadata.namespace,NAME:.metadata.name,SELECTOR:.spec.selector"
```

#### 5b: Query the running Prometheus config and compare

```bash
# Get the replicaset pod name
RS_POD=$(kubectl get pods -n kube-system -l rsName=ama-metrics -o jsonpath='{.items[0].metadata.name}')

# Port-forward to Prometheus (port 9090) in the background
kubectl port-forward -n kube-system $RS_POD 9090:9090 &
PF_PID=$!

# Get the active scrape job names from the running config
curl -s http://localhost:9090/api/v1/status/config | jq -r '.data.yaml' | grep 'job_name:' | sort

# Compare against expected defaults — each enabled target in ama-metrics-settings-configmap
# should appear as a scrape job (e.g., kubelet=true means a job_name containing "kubelet")

# Check all targets and their health status
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, instance: .labels.instance, health, lastError}'

# List only DOWN targets
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health == "down") | {job: .labels.job, instance: .labels.instance, lastError}'

# Verify PodMonitor/ServiceMonitor targets are discovered by the target allocator
# These show up as jobs with names matching the monitor name
curl -s http://localhost:9090/api/v1/targets | jq '[.data.activeTargets[].labels.job] | unique | sort'

kill $PF_PID
```

#### 5c: Verify daemonset config

The linux daemonset uses a separate custom config configmap (`ama-metrics-prometheus-config-node`) and the windows daemonset uses `ama-metrics-prometheus-config-node-windows`. Verify these match what the running daemonset collector has loaded.

```bash
# Check the daemonset-specific custom prometheus config (if any)
kubectl get configmap ama-metrics-prometheus-config-node -n kube-system -o yaml 2>/dev/null || echo "No custom node prometheus config"
kubectl get configmap ama-metrics-prometheus-config-node-windows -n kube-system -o yaml 2>/dev/null || echo "No custom windows node prometheus config"

# Port-forward to a linux daemonset pod and compare
DS_POD=$(kubectl get pods -n kube-system -l dsName=ama-metrics-node -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward -n kube-system $DS_POD 9090:9090 &
PF_PID=$!

# Daemonset should have node-level jobs (kubelet, cadvisor, nodeexporter, etc.)
curl -s http://localhost:9090/api/v1/status/config | jq -r '.data.yaml' | grep 'job_name:' | sort

# Verify custom scrape jobs from ama-metrics-prometheus-config-node are present
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health}'

# Check for DOWN targets
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.health == "down") | {job: .labels.job, instance: .labels.instance, lastError}'

kill $PF_PID
```

#### What to verify

- Each target enabled in `ama-metrics-settings-configmap` (`kubelet=true`, `cadvisor=true`, etc.) has a corresponding scrape job in the running config
- Custom scrape jobs from `ama-metrics-prometheus-config` configmap appear in the running config
- All PodMonitors and ServiceMonitors are discovered and show up as active targets
- No targets are in `down` health state
- Scrape intervals match what is set in `default-targets-scrape-interval-settings`

### Step 6: Verify Metrics Ingestion

Query the Azure Monitor Workspace endpoint to confirm metrics are flowing. **Discover the endpoint dynamically** using `az monitor account show --name ci-prod-aks-weu-mac --resource-group ci-prod-aks-mac-weu-rg --query "metrics.prometheusQueryEndpoint" -o tsv` — do NOT hardcode the hostname as it includes a generated suffix (e.g., `-3d8z`).

### Step 7: Verify Grafana Dashboards

This step has two parts: automated data verification (what the CLI agent can do) and visual dashboard verification (requires a browser or Grafana API access).

#### 7a: Automated — Verify dashboard data exists via AMW API (CLI agent does this)

Query the AMW PromQL API directly to confirm the underlying metrics that power Grafana dashboards exist and are fresh. This proves data is flowing into the AMW but does NOT verify that Grafana dashboards render correctly.

```bash
# Get an access token for the AMW
TOKEN=$(az account get-access-token --resource "https://prometheus.monitor.azure.com" --query accessToken -o tsv)
AMW="$(az monitor account show --name ci-prod-aks-weu-mac --resource-group ci-prod-aks-mac-weu-rg --query 'metrics.prometheusQueryEndpoint' -o tsv)"

# Query key metrics that power Grafana dashboards
for metric in container_cpu_usage_seconds_total container_memory_working_set_bytes kubelet_running_pods kube_pod_info node_cpu_seconds_total apiserver_request_total coredns_dns_requests_total kubeproxy_sync_proxy_rules_duration_seconds_count windows_cs_physical_memory_bytes; do
  count=$(curl -s -H "Authorization: Bearer $TOKEN" "$AMW/api/v1/query?query=count($metric)" | jq -r '.data.result[0].value[1] // "0"')
  echo "$metric: $count series"
done

# Verify data freshness — check that all jobs have recent data (within last 5 minutes)
curl -s -H "Authorization: Bearer $TOKEN" "$AMW/api/v1/query?query=max(up) by (job)" | jq -r '.data.result[] | "\(.metric.job): \(.value[0] | todate)"'
```

**What this proves:** The metrics exist in the AMW with non-zero series counts and fresh timestamps. If data is present and recent here, the Grafana dashboards will show it (assuming Grafana is correctly configured).

**What this does NOT prove:** That Grafana is accessible, dashboards are configured, panels render without errors, or there are no visual anomalies.

#### 7b: Automated — Visual Grafana dashboard verification via Playwright MCP

Use the Playwright MCP server (`@playwright/mcp`) to open the Grafana instance in a browser and verify dashboards visually. This requires the Playwright MCP server to be configured in `~/.copilot/mcp-config.json`.

**Prerequisites:**
- Playwright MCP server added to MCP config (see below)
- User must be logged into Azure in their browser (Grafana uses Azure AD SSO)

**MCP config entry:**
```json
{
  "playwright": {
    "type": "local",
    "command": "npx",
    "tools": ["*"],
    "args": ["@playwright/mcp@latest"]
  }
}
```

**Procedure:**

1. Use Playwright MCP `browser_navigate` to open the CI prod Grafana instance:
   - URL: `https://cicd-graf-metrics-wcus-dkechtfecuadeuaw.wcus.grafana.azure.com/`
   - If an Azure AD login page appears, inform the user they need to complete authentication in the browser window that Playwright opened, then retry.

2. Once in Grafana, navigate to the **Azure Managed Prometheus** dashboard folder:
   - Use `browser_navigate` to go to: `https://cicd-graf-metrics-wcus-dkechtfecuadeuaw.wcus.grafana.azure.com/dashboards/f/azure-managed-prometheus/`
   - Or use `browser_click` to navigate via the sidebar: Dashboards → Browse → Azure Managed Prometheus

3. Navigate to the dashboard folder listing page and use `browser_snapshot` to get the list of ALL dashboards in the folder. Record the full list.

4. For **every** dashboard in the folder, open it and verify ALL panels have data:
   - Use `browser_navigate` or `browser_click` to open the dashboard
   - Use `browser_snapshot` to get the full accessibility tree of the dashboard page
   - Search the snapshot for ALL panel titles — each panel should have associated data values
   - Search for "No data", "No values", or empty panel indicators — flag any panel that shows these
   - Record: dashboard name, total panel count, panels with data, panels with "No data"
   - Do NOT skip any dashboard. Do NOT stop after a few dashboards.

5. On each dashboard, also verify:
   - The time range picker shows a recent range (e.g., "Last 1 hour")
   - Scroll down to check panels below the fold — use `browser_snapshot` after scrolling to capture all panels
   - If the dashboard has multiple rows/sections that are collapsed, expand them before taking the snapshot

6. Record results in the summary report: list every dashboard checked, total panels per dashboard, and any panels showing "No data" with the panel title.

**Fallback:** If Playwright MCP is not available or authentication fails, fall back to the AMW API approach in Step 7a and inform the user that visual verification must be done manually.

#### What was previously here: manual visual verification checklist

If Playwright MCP is unavailable, the user should manually verify in a browser:

1. Navigate to the Grafana portal and select the data source connected to the CI prod AMW.
2. Set the time range to start from when the new image was deployed (check the Helm upgrade timestamp from the pipeline or `helm history ama-metrics -n default`). Do NOT use a fixed short window like "Last 1 hour" — ensure coverage from deployment time to now.
3. Go through each dashboard under the **Azure Managed Prometheus** folder:
   - **Kubernetes / Compute Resources / Cluster** — verify CPU, memory, and network panels have data
   - **Kubernetes / Compute Resources / Namespace (Pods)** — verify per-namespace metrics
   - **Kubernetes / Compute Resources / Node (Pods)** — verify node-level panels
   - **Kubernetes / Compute Resources / Pod** — verify pod-level panels
   - **Kubernetes / Compute Resources / Workload** — verify workload-level panels
   - **Kubernetes / Kubelet** — verify kubelet metrics
   - **Kubernetes / API server** — verify apiserver metrics (if enabled)
   - **Node Exporter / Nodes** — verify node exporter panels
   - Any other custom dashboards present
4. For each dashboard, confirm:
   - No gaps in data starting from the deployment time
   - No panels showing "No data" that were previously populated
   - Metric values look reasonable (no unexpected zeroes or spikes)
   - The `cluster` label matches the expected CI prod cluster name

#### Automation via Playwright MCP

With the Playwright MCP server configured, the agent can automate visual Grafana verification by:
1. Opening the Grafana URL in a Playwright-controlled browser
2. Taking accessibility snapshots of each dashboard page
3. Searching snapshots for "No data" indicators
4. Recording which dashboards have full data coverage

**Limitation:** Azure Managed Grafana uses Azure AD SSO. If the user is not already authenticated in the browser, Playwright will encounter a login page. In this case, the agent should ask the user to complete the Azure AD login in the Playwright browser window, then retry navigation. Alternatively, if a Grafana service account token is available, the agent can use the Grafana HTTP API directly (no browser needed).

---

## Release Readiness Checklist

Before declaring an image ready for production:

- [ ] All ama-metrics pods running (replicaset, linux daemonset, windows daemonset) with correct image tags
- [ ] No pod restarts after new version deployed; if restarts occurred, root cause identified and resolved
- [ ] Container logs show no errors across all pod types and containers (prometheus-collector, addon-token-adapter, config-reader)
- [ ] Prometheus config matches configmap sources — enabled default targets in `ama-metrics-settings-configmap` appear as active scrape jobs
- [ ] Custom scrape config from `ama-metrics-prometheus-config` (replicaset) and `ama-metrics-prometheus-config-node` (daemonset) reflected in running config
- [ ] All PodMonitors and ServiceMonitors discovered and showing as active targets
- [ ] No scrape targets in `down` health state
- [ ] Metrics flowing to Azure Monitor Workspace (queryable via AMW endpoint)
- [ ] AMW platform metrics (EventsPerMinuteIngested, ActiveTimeSeries) stable before vs after deployment — no sustained >15% drop. EventsDropped and TimeSeriesSamplesDropped not spiking.
- [ ] All Grafana dashboards under Azure Managed Prometheus show data with no gaps since deployment time

---

## Cluster Reference

| Cluster | Resource Group | Subscription | AMW Endpoint |
|---------|---------------|--------------|--------------|
| ci-prod-aks-mac-weu | ci-prod-aks-mac-weu-rg | 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb | Discover dynamically: `az monitor account show --name ci-prod-aks-weu-mac --resource-group ci-prod-aks-mac-weu-rg --query "metrics.prometheusQueryEndpoint" -o tsv` (includes generated suffix, e.g. ci-prod-aks-weu-mac-3d8z.westeurope.prometheus.monitor.azure.com) |
