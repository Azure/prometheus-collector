# CCP Configmap V1/V2 Test Workflow

## Overview

This workflow validates that the `ama-metrics-ccp` deployment correctly parses and applies settings from both **v1** and **v2** schema configmaps across all control plane metric ingestion scenarios. It builds on top of the [CCP Health Metrics Validation Workflow](ccp-health-metrics-workflow.md) — you must have a running CCP environment before starting these tests.

### Ingestion Scenarios

| Scenario | Method | Minimal Profile | Keep List |
|----------|--------|----------------|-----------|
| **A — Minimal only** | Default behavior, no changes needed. Only metrics in the minimal list are ingested. | ON (default) | empty |
| **B — Minimal + additional** | Keep minimal enabled, add specific metrics to the keep list. Both minimal and specified metrics are ingested. | ON | non-empty |
| **C — Specific set only** | Disable minimal ingestion, specify a keep list. Only the listed metrics are ingested. | OFF | non-empty |
| **D — All metrics** | Disable minimal ingestion, leave the keep list empty. All scraped metrics are ingested. | OFF | empty |

### Control Plane Targets & Test Metrics

Each scenario is tested with a specific **non-minimal** metric per target to verify keep-list behavior:

| Target | Non-Minimal Test Metric |
|--------|------------------------|
| `apiserver` | `apiserver_admission_step_admission_duration_seconds_count` |
| `kube-scheduler` | `scheduler_scheduler_cache_size` |
| `etcd` | `etcd_cluster_version` |
| `cluster-autoscaler` | `cluster_autoscaler_cluster_cpu_current_cores` |
| `kube-controller-manager` | `workqueue_adds_total` |

### Test Matrix

All 4 scenarios are tested under both v1 and v2 configmap schemas, plus a default (no configmap) case:

| Test | Schema | Scenario | Description |
|------|--------|----------|-------------|
| 1 | v2 | A | Minimal only — all targets enabled, minimal ON, empty keep lists |
| 2 | v2 | B | Minimal + additional — minimal ON, test metrics added to keep lists |
| 3 | v2 | C | Specific set only — minimal OFF, test metrics in keep lists |
| 4 | v2 | D | All metrics — minimal OFF, empty keep lists |
| 5 | v1 | A | Minimal only — all targets enabled, minimalingestionprofile=true, empty keep lists |
| 6 | v1 | B | Minimal + additional — minimalingestionprofile=true, test metrics in keep lists |
| 7 | v1 | C | Specific set only — minimalingestionprofile=false, test metrics in keep lists |
| 8 | v1 | D | All metrics — minimalingestionprofile=false, empty keep lists |
| 9 | none | — | No configmap — defaults used (apiserver+etcd, minimal ON) |

### When to Use

- After code changes to configmap parsing (`configmapparserforccp.go`, `configmapparser.go`)
- After changes to keep-list logic or minimal-ingestion-profile handling
- After changes to v1 ↔ v2 schema support
- As a regression test for CCP configmap handling

---

## Prerequisites

Complete **Parts 1–4** of the [CCP Health Metrics Workflow](ccp-health-metrics-workflow.md):
- Standalone environment running with cx-1 underlay
- Test cluster created with `AzureMonitorMetricsControlPlanePreview`
- Reconcilers scaled to 0
- CCP namespace annotated with `skip-ccp-reconcile-until-this-time`
- `ama-metrics-ccp` deployment running 3/3 containers
- Port-forward to `hcp-kubernetes` service on port 6443 (for customer cluster access)
- Customer cluster kubeconfig available at `customer-cluster-local.kubeconfig`

### Environment Variables

```powershell
$UNDERLAY_KUBECONFIG = "<path-to>/standalone-<name>-cx-1.kubeconfig"
$CUSTOMER_KUBECONFIG = "<path-to>/customer-cluster-local.kubeconfig"
$CCP_NS = "<ccp-namespace-id>"  # starts with '6'
$CONFIGMAP_DIR = "<prometheus-collector-repo>/otelcollector/configmaps"
```

---

## Common Test Procedure

Each test follows the same pattern:

1. **Prepare** the configmap yaml with the desired settings
2. **Apply** the configmap to the customer cluster's `kube-system`
3. **Wait** for configmap-watcher to detect the change (~10-30s) and pod to reparse (~60s)
4. **Verify** pod logs show correct parsing (schema, minimal profile, keep list regexes)
5. **Wait** 3-5 minutes for ME to process and publish metrics
6. **Check** health metrics endpoint for data flow

### Why Apply to Customer Cluster?

The `configmap-watcher` sidecar watches the **customer cluster's** `kube-system` namespace (via the internal kubeconfig from the `kubeconfig-file` secret), NOT the underlay CCP namespace. Configmaps must be applied using the customer cluster kubeconfig.

### Configmap-Watcher Verification

After every apply, check that the watcher detected the change:

```powershell
$env:KUBECONFIG = $UNDERLAY_KUBECONFIG
kubectl logs deploy/ama-metrics-ccp -c configmap-watcher -n $CCP_NS --tail=20
```

**Expected log patterns:**
```
Configmap ama-metrics-settings-configmap in namespace kube-system has been updated
Updating file: /etc/config/settings/schema-version
Updating file: /etc/config/settings/controlplane-metrics
```

### Health Metrics Validation

After every test (wait 3-5 min for ME), port-forward and check:

```powershell
# Start port-forward (background)
$env:KUBECONFIG = $UNDERLAY_KUBECONFIG
kubectl port-forward deploy/ama-metrics-ccp 2234:2234 -n $CCP_NS

# Fetch metrics (separate terminal)
$response = Invoke-WebRequest -Uri http://localhost:2234/metrics -UseBasicParsing
$response.Content
```

**Validation criteria (all tests):**
| Metric | Condition |
|--------|-----------|
| `me_metrics_sent_per_minute` | > 0 |
| `me_metrics_received_per_minute` | > 0 |
| `me_bytes_sent_per_minute` | > 0 |
| `overall_metrics_dropped_total` | = 0 |
| `invalid_metrics_settings_config` | = 0 |

---

## V2 Schema Tests (Tests 1–4)

### Test 1: V2 Scenario A — Minimal Only (Default Behavior)

All targets enabled, minimal ingestion ON, empty keep lists — only minimal metrics ingested.

#### Configmap Settings

```yaml
controlplane-metrics: |-
  default-targets-scrape-enabled: |-
    apiserver = true
    cluster-autoscaler = true
    kube-scheduler = true
    kube-controller-manager = true
    etcd = true
  default-targets-metrics-keep-list: |-
    apiserver = ""
    cluster-autoscaler = ""
    kube-scheduler = ""
    kube-controller-manager = ""
    etcd = ""
  minimal-ingestion-profile: |-
    enabled = true
```

#### Apply & Verify

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl apply -f "$CONFIGMAP_DIR/ama-metrics-settings-configmap-v2.yaml"

# Wait ~60s, then check logs
$env:KUBECONFIG = $UNDERLAY_KUBECONFIG
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --tail=200
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v2` |
| `minimal-ingestion-profile enabled` | `true` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | Minimal regexes only (`apiserver_request_total\|apiserver_request_duration_seconds\|...`) |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | Minimal etcd regexes only |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | Minimal scheduler regexes only |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | Minimal kcm regexes only |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | Minimal cluster-autoscaler regexes only |

#### Validation

- Health metrics: `me_metrics_sent_per_minute > 0`
- Non-minimal test metrics **should NOT** appear in ingested data (they're filtered out by minimal profile)
- Metric rate should be moderate (only minimal metrics)

---

### Test 2: V2 Scenario B — Minimal + Additional Metrics

Minimal ingestion ON, with non-minimal test metrics added to keep lists — both minimal and specified metrics ingested.

#### Configmap Settings

```yaml
controlplane-metrics: |-
  default-targets-scrape-enabled: |-
    apiserver = true
    cluster-autoscaler = true
    kube-scheduler = true
    kube-controller-manager = true
    etcd = true
  default-targets-metrics-keep-list: |-
    apiserver = "apiserver_admission_step_admission_duration_seconds_count"
    cluster-autoscaler = "cluster_autoscaler_cluster_cpu_current_cores"
    kube-scheduler = "scheduler_scheduler_cache_size"
    kube-controller-manager = "workqueue_adds_total"
    etcd = "etcd_cluster_version"
  minimal-ingestion-profile: |-
    enabled = true
```

#### Apply & Verify

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl apply -f "$CONFIGMAP_DIR/ama-metrics-settings-configmap-v2.yaml"
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v2` |
| `minimal-ingestion-profile enabled` | `true` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | `apiserver_admission_step_admission_duration_seconds_count` + minimal regexes appended |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | `etcd_cluster_version` + minimal etcd regexes appended |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | `scheduler_scheduler_cache_size` + minimal scheduler regexes appended |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | `workqueue_adds_total` + minimal kcm regexes appended |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | `cluster_autoscaler_cluster_cpu_current_cores` + minimal ca regexes appended |

#### Validation

- Health metrics: `me_metrics_sent_per_minute > 0`
- Rate should be slightly **higher** than Test 1 (minimal + extra metrics)
- The non-minimal test metrics **should** now appear in ingested data

---

### Test 3: V2 Scenario C — Specific Set Only

Minimal ingestion OFF, with non-minimal test metrics in keep lists — only those specific metrics ingested.

#### Configmap Settings

```yaml
controlplane-metrics: |-
  default-targets-scrape-enabled: |-
    apiserver = true
    cluster-autoscaler = true
    kube-scheduler = true
    kube-controller-manager = true
    etcd = true
  default-targets-metrics-keep-list: |-
    apiserver = "apiserver_admission_step_admission_duration_seconds_count"
    cluster-autoscaler = "cluster_autoscaler_cluster_cpu_current_cores"
    kube-scheduler = "scheduler_scheduler_cache_size"
    kube-controller-manager = "workqueue_adds_total"
    etcd = "etcd_cluster_version"
  minimal-ingestion-profile: |-
    enabled = false
```

#### Apply & Verify

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl apply -f "$CONFIGMAP_DIR/ama-metrics-settings-configmap-v2.yaml"
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v2` |
| `minimal-ingestion-profile enabled` | `false` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | `apiserver_admission_step_admission_duration_seconds_count` (no minimal appended) |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | `etcd_cluster_version` (no minimal appended) |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | `scheduler_scheduler_cache_size` (no minimal appended) |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | `workqueue_adds_total` (no minimal appended) |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | `cluster_autoscaler_cluster_cpu_current_cores` (no minimal appended) |

#### Validation

- Health metrics: `me_metrics_sent_per_minute > 0`
- Rate should be **lowest** of all tests (only 1 metric per target)
- Only the specified test metrics should appear in ingested data — no minimal metrics

---

### Test 4: V2 Scenario D — All Metrics

Minimal ingestion OFF, empty keep lists — all scraped metrics ingested (no filtering).

#### Configmap Settings

```yaml
controlplane-metrics: |-
  default-targets-scrape-enabled: |-
    apiserver = true
    cluster-autoscaler = true
    kube-scheduler = true
    kube-controller-manager = true
    etcd = true
  default-targets-metrics-keep-list: |-
    apiserver = ""
    cluster-autoscaler = ""
    kube-scheduler = ""
    kube-controller-manager = ""
    etcd = ""
  minimal-ingestion-profile: |-
    enabled = false
```

#### Apply & Verify

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl apply -f "$CONFIGMAP_DIR/ama-metrics-settings-configmap-v2.yaml"
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v2` |
| `minimal-ingestion-profile enabled` | `false` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |

#### Validation

- Health metrics: `me_metrics_sent_per_minute > 0`
- Rate should be **highest** of all tests (no filtering at all)
- All metrics from all targets should appear in ingested data

---

## V1 Schema Tests (Tests 5–8)

V1 uses a flat structure: targets are prefixed with `controlplane-` in `default-scrape-settings-enabled` and `default-targets-metrics-keep-list`, and minimal ingestion is set via `minimalingestionprofile = true|false` inside the keep-list section.

### Test 5: V1 Scenario A — Minimal Only

All targets enabled, minimalingestionprofile=true, empty keep lists.

#### Configmap Settings (v1 format)

```yaml
schema-version: v1
default-scrape-settings-enabled: |-
  controlplane-apiserver = true
  controlplane-cluster-autoscaler = true
  controlplane-kube-scheduler = true
  controlplane-kube-controller-manager = true
  controlplane-etcd = true
default-targets-metrics-keep-list: |-
  controlplane-apiserver = ""
  controlplane-cluster-autoscaler = ""
  controlplane-kube-scheduler = ""
  controlplane-kube-controller-manager = ""
  controlplane-etcd = ""
  minimalingestionprofile = true
```

#### Apply

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl apply -f "$CONFIGMAP_DIR/ama-metrics-settings-configmap-v1.yaml"
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v1` |
| `minimalingestionprofile` | `true` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | Minimal regexes only |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | Minimal etcd regexes only |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | Minimal scheduler regexes only |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | Minimal kcm regexes only |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | Minimal ca regexes only |

#### Validation

- Rate should be moderate (only minimal metrics)

---

### Test 6: V1 Scenario B — Minimal + Additional Metrics

minimalingestionprofile=true, with test metrics in keep lists.

#### Configmap Settings (v1 format)

```yaml
default-targets-metrics-keep-list: |-
  controlplane-apiserver = "apiserver_admission_step_admission_duration_seconds_count"
  controlplane-cluster-autoscaler = "cluster_autoscaler_cluster_cpu_current_cores"
  controlplane-kube-scheduler = "scheduler_scheduler_cache_size"
  controlplane-kube-controller-manager = "workqueue_adds_total"
  controlplane-etcd = "etcd_cluster_version"
  minimalingestionprofile = true
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v1` |
| `minimalingestionprofile` | `true` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | `apiserver_admission_step_admission_duration_seconds_count` + minimal regexes appended |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | `etcd_cluster_version` + minimal etcd regexes appended |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | `scheduler_scheduler_cache_size` + minimal scheduler regexes appended |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | `workqueue_adds_total` + minimal kcm regexes appended |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | `cluster_autoscaler_cluster_cpu_current_cores` + minimal ca regexes appended |

#### Validation

- Rate slightly higher than Test 5 (minimal + extra metrics)

---

### Test 7: V1 Scenario C — Specific Set Only

minimalingestionprofile=false, with test metrics in keep lists.

#### Configmap Settings (v1 format)

```yaml
default-targets-metrics-keep-list: |-
  controlplane-apiserver = "apiserver_admission_step_admission_duration_seconds_count"
  controlplane-cluster-autoscaler = "cluster_autoscaler_cluster_cpu_current_cores"
  controlplane-kube-scheduler = "scheduler_scheduler_cache_size"
  controlplane-kube-controller-manager = "workqueue_adds_total"
  controlplane-etcd = "etcd_cluster_version"
  minimalingestionprofile = false
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v1` |
| `minimalingestionprofile` | `false` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | `apiserver_admission_step_admission_duration_seconds_count` (no minimal appended) |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | `etcd_cluster_version` (no minimal appended) |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | `scheduler_scheduler_cache_size` (no minimal appended) |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | `workqueue_adds_total` (no minimal appended) |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | `cluster_autoscaler_cluster_cpu_current_cores` (no minimal appended) |

#### Validation

- Rate should be lowest (only 1 metric per target)

---

### Test 8: V1 Scenario D — All Metrics

minimalingestionprofile=false, empty keep lists — all metrics pass through.

#### Configmap Settings (v1 format)

```yaml
default-targets-metrics-keep-list: |-
  controlplane-apiserver = ""
  controlplane-cluster-autoscaler = ""
  controlplane-kube-scheduler = ""
  controlplane-kube-controller-manager = ""
  controlplane-etcd = ""
  minimalingestionprofile = false
```

#### Expected Log Patterns

| Pattern | Expected Value |
|---------|---------------|
| `AZMON_AGENT_CFG_SCHEMA_VERSION` | `v1` |
| `minimalingestionprofile` | `false` |
| `CONTROLPLANE_APISERVER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_ETCD_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_KUBE_SCHEDULER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_KUBE_CONTROLLER_MANAGER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |
| `CONTROLPLANE_CLUSTER_AUTOSCALER_KEEP_LIST_REGEX` | (empty — all metrics pass through) |

#### Validation

- Rate should be highest (no filtering)

---

## Test 9: No Configmap (Defaults)

This test verifies the pod uses correct defaults when no configmap exists.

### 9.1 Delete the Configmap

```powershell
$env:KUBECONFIG = $CUSTOMER_KUBECONFIG
kubectl delete configmap ama-metrics-settings-configmap -n kube-system
```

### 9.2 Restart the Pod

```powershell
$env:KUBECONFIG = $UNDERLAY_KUBECONFIG
kubectl delete pod -n $CCP_NS -l rsName=ama-metrics-ccp
```

### 9.3 Verify Default Behavior

Wait ~90 seconds for the pod to restart:

```powershell
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --tail=200
```

**Expected:**
- `Invalid schema version or no configmap present. Using defaults.`
- Default scrape targets enabled: apiserver=true, etcd=true, all others false
- Minimal ingestion profile defaults to `true`
- Only 2 default prometheus configs merged (apiserver + etcd)

### 9.4 Validate Data Still Flows

After 3-5 minutes, health metrics should show `me_metrics_sent_per_minute > 0` with default settings.

---

## Quick Validation Script

After applying any configmap, run this sequence to validate:

```powershell
# 1. Check configmap-watcher picked it up
$env:KUBECONFIG = $UNDERLAY_KUBECONFIG
kubectl logs deploy/ama-metrics-ccp -c configmap-watcher -n $CCP_NS --tail=10

# 2. Check prometheus-collector parsed it (look for all 5 control plane targets)
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --tail=200 2>&1 |
  Select-String -Pattern "SCHEMA_VERSION|minimal.*ingestion|CONTROLPLANE.*KEEP_LIST|controlplane-etcd|controlplane-apiserver|controlplane-kube-scheduler|controlplane-kube-controller|controlplane-cluster-autoscaler|Invalid schema"

# 3. Fetch health metrics (requires port-forward on 2234)
$metrics = (Invoke-WebRequest -Uri http://localhost:2234/metrics -UseBasicParsing).Content
$metrics -split "`n" | Select-String -Pattern "me_metrics_sent|me_metrics_received|me_bytes_sent|overall_metrics_dropped|invalid_metrics_settings"
```

---

## Expected Results Summary

### V2 Tests

| Test | Scenario | Minimal | Keep Lists | Expected Keep List Regex | Rate Expectation |
|------|----------|---------|------------|--------------------------|------------------|
| 1 | A — Minimal only | ON | empty | minimal regexes only | moderate |
| 2 | B — Minimal + additional | ON | test metrics | test metrics + minimal regexes | slightly > Test 1 |
| 3 | C — Specific only | OFF | test metrics | test metrics only (no minimal) | lowest |
| 4 | D — All metrics | OFF | empty | empty (all pass through) | highest |

### V1 Tests

| Test | Scenario | Minimal | Keep Lists | Expected Keep List Regex | Rate Expectation |
|------|----------|---------|------------|--------------------------|------------------|
| 5 | A — Minimal only | ON | empty | minimal regexes only | moderate |
| 6 | B — Minimal + additional | ON | test metrics | test metrics + minimal regexes | slightly > Test 5 |
| 7 | C — Specific only | OFF | test metrics | test metrics only (no minimal) | lowest |
| 8 | D — All metrics | OFF | empty | empty (all pass through) | highest |

### No Configmap

| Test | Schema | Description | Rate Expectation |
|------|--------|-------------|------------------|
| 9 | none | Defaults: apiserver+etcd, minimal ON | moderate (2 targets only) |

### Rate Ordering (expected)

```
Test 4/8 (all metrics) > Test 2/6 (minimal+extra) >= Test 1/5 (minimal) > Test 9 (defaults, 2 targets) > Test 3/7 (specific only)
```

---

## Troubleshooting

1. **Configmap-watcher doesn't detect change** — Verify port-forward to `hcp-kubernetes` is still active. Re-establish if needed: `kubectl port-forward svc/hcp-kubernetes 6443:443 -n $CCP_NS`
2. **Pod doesn't restart after configmap change** — The configmap-watcher writes to an emptyDir volume. The prometheus-collector detects changes via inotify and restarts its config parsing loop. If it doesn't restart, manually delete the pod.
3. **`me_metrics_sent_per_minute = 0` after 5 minutes** — Check `addon-token-adapter` logs for MSI token issues. Verify the `CLUSTER` env var points to the cx-1 underlay resource ID.
4. **Schema version not changing** — Configmap-watcher may cache. Delete the pod to force a clean restart.
5. **V1 controlplane settings not applying** — In v1, controlplane targets use `controlplane-` prefix in the flat `default-scrape-settings-enabled` section (e.g., `controlplane-etcd = true`).
6. **Port-forward stale after pod restart** — When pod restarts, the existing port-forward on 2234 becomes stale. Kill via `netstat -ano | Select-String ":2234"` + `Stop-Process`, then re-establish.
7. **Cluster-autoscaler/kube-scheduler/kube-controller-manager not scraping** — These targets may not have active pods in all environments. Check that the target pods exist before expecting metrics. Verify: `kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --tail=200 | Select-String "Done merging"` to confirm how many configs were merged.

---

## Related Documents

| Document | Location |
|----------|----------|
| CCP Health Metrics Workflow | [ccp-health-metrics-workflow.md](ccp-health-metrics-workflow.md) |
| V1 Configmap Template | [ama-metrics-settings-configmap-v1.yaml](../../configmaps/ama-metrics-settings-configmap-v1.yaml) |
| V2 Configmap Template | [ama-metrics-settings-configmap-v2.yaml](../../configmaps/ama-metrics-settings-configmap-v2.yaml) |
| Configmap Parser (CCP) | [configmapparserforccp.go](../../shared/configmap/ccp/configmapparserforccp.go) |
