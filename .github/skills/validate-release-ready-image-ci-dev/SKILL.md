---
name: validate-release-ready-image-cidev
description: Validate a prometheus-collector release image after version bump PR deploys to CI dev clusters, ensuring it's ready for production. Covers both the automated CI pipeline flow and manual validation steps. Use when "validate release image", "check CI test results", "debug testkube failures", or "is this image ready to release".
allowed-tools:
  - run_in_terminal
  - read_file
  - mcp_azure-devops_get_build
  - mcp_azure-devops_get_build_logs
  - mcp_azure-devops_analyze_build_errors
---

# Validate Release-Ready Image

## Agent Execution Plan

**IMPORTANT**: You MUST execute ALL of the following phases in order. Do NOT skip any phase or step. Do NOT declare the image "ready" until every phase is complete.

### Phase 1: CI Pipeline Check
1. Find the latest build on `main` for pipeline definition 440 (project `azure`, org `github-private.visualstudio.com`).
2. Check the build result. If it failed, analyze build errors and identify which stage/job failed.
3. For TestKube failures, get the "Run TestKube workflow" task log and identify which test workflows passed/failed and why.
4. Record the CI results for all stages: Build, Deploy (all clusters), TestKube AKS, TestKube OTel, TestKube ARC.

### Phase 2: Manual Validation (ALL steps required)
Get credentials for `ci-dev-aks-mac-eus` cluster, then execute **every** step below:

5. **Step 1 — Pod Status**: Check ALL ama-metrics pod types (replicaset, linux daemonset, windows daemonset) are Running with correct image tags.
6. **Step 2 — Pod Restarts**: Check restart counts for ALL pod types. If any restarts > 0, investigate with `--previous` logs and events.
7. **Step 3 — Container Logs**: Check logs for errors in ALL containers across ALL pod types:
   - `prometheus-collector` in replicaset, linux daemonset, AND windows daemonset pods
   - `addon-token-adapter` / `addon-token-adapter-win` in all pod types
   - `config-reader` in all pod types (if present — may be merged into prometheus-collector)
8. **Step 4 — Liveness/Readiness Probes**: Verify probe configuration on all pod types using `kubectl describe`.
9. **Step 5a — Config Sources**: Check `ama-metrics-settings-configmap` (default scrape settings + intervals), custom prometheus config configmaps (`ama-metrics-prometheus-config`, `ama-metrics-prometheus-config-node`, `ama-metrics-prometheus-config-node-windows`), and PodMonitors/ServiceMonitors.
10. **Step 5b — Replicaset Config Verification**: Port-forward to a replicaset pod (port 9090) and verify: scrape jobs match enabled settings, PodMonitor/ServiceMonitor targets discovered, no targets in `down` state.
11. **Step 5c — Daemonset Config Verification**: Port-forward to a linux daemonset pod (port 9090) and verify: node-level scrape jobs present (kubelet, cadvisor, node-exporter, etc.), no targets in `down` state.
12. **Step 6 — Metrics Ingestion**: Query the AMW endpoint to confirm metrics are flowing (count of `up`, `kube_pod_info`, `scrape_samples_scraped`).
13. **Step 7 — Grafana Dashboard Metrics**: Query AMW for ALL key metrics that power Grafana dashboards: `container_cpu_usage_seconds_total`, `container_memory_working_set_bytes`, `kubelet_running_pods`, `kube_pod_info`, `node_cpu_seconds_total`, `apiserver_request_total`, `coredns_dns_requests_total`, `kubeproxy_sync_proxy_rules_duration_seconds_count`, `windows_cs_physical_memory_bytes`. Verify all jobs report fresh data with no gaps.

### Phase 3: Summary and Verdict
14. Generate a **Validation Summary Report** using the template below. Fill in every row with actual results and the evidence that led to your pass/fail determination. Do NOT leave any row blank.
15. Declare verdict: READY or NOT READY, with justification for any failures or warnings.

#### Validation Summary Report Template

```
## Validation Summary Report
**Image:** <full image tag, e.g. 6.27.0-main-04-10-2026-a2c43cc1>
**Build:** <ADO build ID>
**Date:** <validation date>
**Cluster:** ci-dev-aks-mac-eus

### Phase 1: CI Pipeline Results
| Stage | Result | Details |
|-------|--------|---------|
| Build | ✅/❌ | <all images built? any build errors?> |
| Deploy_AKS_Chart | ✅/❌ | <helm upgrade succeeded?> |
| Deploy_AKS_Chart_Test_Cluster | ✅/❌ | |
| Deploy_AKS_Chart_OTel_Cluster | ✅/❌ | |
| Deploy_Chart_ARC | ✅/❌ | |
| Testkube (AKS) | ✅/❌/⚠️ | <list each workflow: containerstatus, livenessprobe, prometheusui, operator, querymetrics — passed/failed/skipped. If failed, include root cause.> |
| Testkube_OTel | ✅/❌ | <list each workflow result> |
| Testkube_ARC | ✅/❌ | <list each workflow result> |
| TestKube_Summary | ✅/❌ | |

### Phase 2: Manual Validation Results
| Step | Result | Evidence |
|------|--------|----------|
| 1. Pod Status | ✅/❌ | <# of RS pods, DS pods, Win DS pods running. Image tag confirmed.> |
| 2. Pod Restarts | ✅/❌ | <restart counts for each pod type. If >0, root cause.> |
| 3. Container Logs | ✅/❌/⚠️ | <errors found? In which container/pod type? Transient or ongoing? Timestamp of errors vs deployment time.> |
| 4. Liveness/Readiness Probes | ✅/❌ | <probes configured on all pod types? Any probe failures in events?> |
| 5a. Config Sources | ✅/❌ | <default scrape settings enabled, scrape intervals, custom configmaps present, PodMonitors/ServiceMonitors listed.> |
| 5b. Replicaset Config | ✅/❌ | <# scrape jobs in running config, # active targets, # down targets. Do jobs match enabled settings?> |
| 5c. Daemonset Config | ✅/❌ | <# scrape jobs in running config, # active targets, # down targets. Node-level jobs present?> |
| 6. Metrics Ingestion | ✅/❌ | <count(up), count(kube_pod_info), count(scrape_samples_scraped) from AMW query.> |
| 7. Grafana Dashboard Metrics | ✅/❌ | <series counts for: container_cpu, container_memory, kubelet, kube_pod_info, node_cpu, apiserver_request, coredns, kubeproxy, windows. # of jobs reporting. Latest data timestamp — is it fresh?> |

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
- **Daemonset Config**: PASS if node-level jobs (kubelet, cadvisor, node-exporter, kappie-basic, etc.) are present and zero targets are `down`.
- **Metrics Ingestion**: PASS if `count(up)` returns a reasonable number (>0), `kube_pod_info` and `scrape_samples_scraped` are present. FAIL if any query returns 0 or errors.
- **Grafana Dashboard Metrics**: PASS if all key metrics have non-zero series counts AND the latest data timestamp is within the last 5 minutes (no data gaps). FAIL if any key metric returns 0 series or data is stale.

---

## Overview

After a version bump PR merges to `main`, the [Azure.prometheus-collector pipeline](https://github-private.visualstudio.com/azure/_build?definitionId=440) automatically builds images, deploys them to CI dev clusters, and runs TestKube validation tests. This skill covers:

1. Understanding the automated CI validation pipeline.
2. Debugging test failures from the pipeline.
3. Manually validating the image on the `ci-dev-aks-mac-eus` cluster when needed.

### When to Use

- A version bump PR just merged and you want to confirm the image is release-ready.
- The CI pipeline failed on the TestKube validation stage and you need to diagnose why.
- You want to manually re-run or validate tests on the CI dev cluster.

---

## Pipeline Architecture

The pipeline (definition ID `440`, project `azure`, org `github-private.visualstudio.com`) runs these stages on `main` branch merges:

### Build Stage
- Builds Linux, Windows, CCP, target allocator, and config reader images.
- Pushes to `cidev` MCR: `mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:<TAG>`
- Produces Helm chart artifacts.

### Deploy Stages (parallel)
| Stage | Cluster | Region | Purpose |
|-------|---------|--------|---------|
| `Deploy_AKS_Chart` | `ci-dev-aks-mac-eus` | eastus | Primary AKS validation |
| `Deploy_AKS_Chart_Test_Cluster` | `ci-dev-aks-tests` | centralus | Config processing tests |
| `Deploy_AKS_Chart_OTel_Cluster` | `ciprom-dev-aks-otlp` | westus3 | OTel/OTLP validation |
| `Deploy_Chart_ARC` | `ci-dev-arc-wcus` | westcentralus | Arc extension validation |
| `Deploy_AKS_Chart_OTel_Upgrade_Cluster` | `ciprom-upgrade-bot` | westus3 | OTel upgrade (only on `bot/*` branches) |

Each deploy stage:
1. Waits for images to appear in MCR (polls up to 10 minutes).
2. Substitutes image tags into Chart.yaml and values.yaml.
3. Runs `helm upgrade` with the new chart on the target cluster.
4. Also deploys Retina (network observability) on AKS clusters.

### TestKube Stages (after deploy)
| Stage | Depends On | Test CRs File | Target Env |
|-------|-----------|---------------|------------|
| `Testkube` | `Deploy_AKS_Chart` | `testkube-test-crs.yaml` | AKS |
| `Testkube_OTel` | `Deploy_AKS_Chart_OTel_Cluster` | `testkube-test-crs-otel.yaml` | OTel |
| `Testkube_ARC` | `Deploy_Chart_ARC` | `testkube-test-crs-arc.yaml` | ARC |
| `Testkube_OTel_Upgrade` | `Deploy_AKS_Chart_OTel_Upgrade_Cluster` | `testkube-test-crs-otelcollector-upgrade.yaml` | OTelCollector-Upgrade |

Each TestKube stage runs `run-testkube-workflow.sh` which:
1. Installs TestKube CLI.
2. Applies test workflow CRs and configmaps to the cluster.
3. Deploys the prometheus reference app.
4. Waits 360s (default) for cluster readiness.
5. Discovers and runs all test workflows sequentially.
6. Collects results into `testkube-results-<ENV>.json`.

### Summary Stage
`TestKube_Summary` aggregates results from AKS, OTel, and ARC TestKube stages and sends a notification.

---

## Test Workflows

### AKS Tests (`testkube-test-crs.yaml`)

All tests use Ginkgo framework with label filter `!(arc-extension,linux-daemonset-custom-config,otlp)`:

| Workflow | Test Suite | What It Validates |
|----------|-----------|-------------------|
| `containerstatus` | `./containerstatus` | All ama-metrics containers are running and healthy |
| `livenessprobe` | `./livenessprobe` | Liveness probes pass (90m timeout) |
| `prometheusui` | `./prometheusui` | Prometheus UI is accessible and functional |
| `operator` | `./operator` | CRD operator (azmonitoring.coreos.com) works correctly |
| `querymetrics` | `./querymetrics` | Metrics are ingested and queryable from Azure Monitor Workspace |

### OTel Tests (`testkube-test-crs-otel.yaml`)

Same test suites with label filter `!(arc-extension,linux-daemonset-custom-config,fips,mdsd)` — validates OTLP pipeline-specific behavior.

### ARC Tests (`testkube-test-crs-arc.yaml`)

Uses `!(linux-daemonset-custom-config,otlp)` filter — includes Arc extension tests.

### OTel Upgrade Tests (`testkube-test-crs-otelcollector-upgrade.yaml`)

Same 5 workflows. Only runs on `bot/*` branches (IS_OTEL_UPGRADE_BRANCH).

---

## Debugging CI Pipeline Failures

### Step 1: Get Build Status

Use the ADO MCP tools (requires `github-private.visualstudio.com` org):

```
Get build info: buildId=<ID>, project=azure
Analyze build errors: buildId=<ID>, project=azure
```

Key things to check:
- `result`: `failed` vs `partiallySucceeded`
- `finishTime - startTime`: if >4h, likely a timeout
- Error summary for `TestKube tests failed` messages

### Step 2: Identify Which Stage Failed

Look at the error messages from `analyze_build_errors`:
- `"TestKube tests failed"` — a TestKube workflow failed
- `"TestKube results file not found"` — the run script crashed before producing results
- `"Images are not published to mcr within the timeout"` — MCR push delay
- `"Helm lint failed"` — chart issue (build stage, not test)

### Step 3: Get TestKube Logs

The TestKube run log is in the "Run TestKube workflow" task. Find it by searching build logs for content containing `run-testkube-workflow.sh` or `Running workflow:`.

Look for:
- Which workflow failed: `<workflow> TestWorkflow failed. Execution ID: <id>`
- The `TestWorkflow Summary` section at the end listing failed/successful workflows
- Individual workflow execution output from `kubectl testkube watch`

### Step 4: Common Failure Patterns

| Pattern | Cause | Resolution |
|---------|-------|------------|
| Timeout after ~5h | `livenessprobe` test has 90m Ginkgo timeout + 6m sleep per workflow | Check if pods are crashlooping |
| `querymetrics` fails | Metrics not reaching AMW | Check ME/MDSD logs, AMW endpoint connectivity |
| `containerstatus` fails | Pod not running | Check image pull errors, node scheduling |
| `operator` fails | CRD not installed or operator crash | Check operator pod logs |
| `No testworkflows found` | TestKube CRs not applied | Check kubectl apply step |
| `Could not find execution ID` | TestKube API server issue | Check testkube namespace pods |

---

## Manual Validation

### Prerequisites

- Azure CLI with `ContainerInsights_Build_Subscription` access
- `kubectl` and `helm` installed

### Step 1: Get Cluster Credentials

```bash
az aks get-credentials -g ci-dev-aks-mac-eus-rg -n ci-dev-aks-mac-eus
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

Query the Azure Monitor Workspace endpoint to confirm metrics are flowing:
- AKS AMW endpoint: `https://ci-dev-aks-eus-mac-mih6.eastus.prometheus.monitor.azure.com`
- Client ID: `c7f895bb-c4f6-45af-be82-2273a424e237`

### Step 7: Verify Grafana Dashboards

Open the Grafana instance linked to the CI dev Azure Monitor Workspace and confirm all dashboards under **Azure Managed Prometheus** show data for the time window after the new agent version was deployed.

1. Navigate to the Grafana portal and select the data source connected to the CI dev AMW.
2. Set the time range to start from when the new image was deployed (check the Helm upgrade timestamp from the pipeline or `helm history ama-metrics -n default`).
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
   - The `cluster` label matches the expected CI dev cluster name

---

## Release Readiness Checklist

Before declaring an image ready for production:

**CI Pipeline (automated)**
- [ ] CI pipeline build completed (build stage succeeded)
- [ ] All images published to cidev MCR
- [ ] `Deploy_AKS_Chart` succeeded — Helm upgrade on ci-dev-aks-mac-eus
- [ ] `Testkube` (AKS) — all 5 workflows passed
- [ ] `Testkube_OTel` — all workflows passed
- [ ] `Testkube_ARC` — all workflows passed
- [ ] `TestKube_Summary` notification sent

**Manual Validation**
- [ ] All ama-metrics pods running (replicaset, linux daemonset, windows daemonset) with correct image tags
- [ ] No pod restarts after new version deployed; if restarts occurred, root cause identified and resolved
- [ ] Container logs show no errors across all pod types and containers (prometheus-collector, addon-token-adapter, config-reader)
- [ ] Prometheus config matches configmap sources — enabled default targets in `ama-metrics-settings-configmap` appear as active scrape jobs
- [ ] Custom scrape config from `ama-metrics-prometheus-config` (replicaset) and `ama-metrics-prometheus-config-node` (daemonset) reflected in running config
- [ ] All PodMonitors and ServiceMonitors discovered and showing as active targets
- [ ] No scrape targets in `down` health state
- [ ] Metrics flowing to Azure Monitor Workspace (queryable via AMW endpoint)
- [ ] All Grafana dashboards under Azure Managed Prometheus show data with no gaps since deployment time

---

## Cluster Reference

| Cluster | Resource Group | Subscription | AMW Endpoint |
|---------|---------------|--------------|--------------|
| ci-dev-aks-mac-eus | ci-dev-aks-mac-eus-rg | 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb | ci-dev-aks-eus-mac-mih6.eastus.prometheus.monitor.azure.com |
| ciprom-dev-aks-otlp | ciprom-dev-aks-otlp | 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb | ci-prom-dev-aks-otlp-geaqdgeuapfeh8b2.westus3.prometheus.monitor.azure.com |
| ci-dev-arc-wcus | ci-dev-arc-wcus | 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb | ci-dev-arc-amw-p3eu.eastus.prometheus.monitor.azure.com |
| ciprom-upgrade-bot | ciprom-upgrade-bot | 9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb | ciprom-upgrade-bot-e4c4gvcgcqd7awhw.westus3.prometheus.monitor.azure.com |

---

## Key Files

| File | Purpose |
|------|---------|
| `.pipelines/azure-pipeline-build.yml` | Main CI/CD pipeline definition |
| `otelcollector/test/testkube/run-testkube-workflow.sh` | TestKube orchestration script |
| `otelcollector/test/testkube/testkube-test-crs.yaml` | AKS test workflow definitions |
| `otelcollector/test/testkube/testkube-test-crs-otel.yaml` | OTel test workflow definitions |
| `otelcollector/test/testkube/testkube-test-crs-arc.yaml` | ARC test workflow definitions |
| `otelcollector/test/testkube/testkube-test-crs-otelcollector-upgrade.yaml` | OTel upgrade test definitions |
| `otelcollector/test/ginkgo-e2e/` | Ginkgo test source code |
