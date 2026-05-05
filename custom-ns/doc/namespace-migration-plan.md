# ama-metrics Namespace Migration Plan

> **Goal**: Migrate all ama-metrics workloads from `kube-system` to a dedicated namespace (e.g., `ama-metrics-ns`).
>
> **Status**: Planning
>
> **Date**: March 2026

---

## Table of Contents

1. [Why Migrate?](#1-why-migrate)
2. [Scope](#2-scope)
3. [Impact Summary](#3-impact-summary)
4. [Work Breakdown — P0 (Blockers)](#4-work-breakdown--p0-blockers)
5. [Work Breakdown — P1 (High)](#5-work-breakdown--p1-high)
6. [Work Breakdown — P2 (Medium)](#6-work-breakdown--p2-medium)
7. [Work Breakdown — P3 (Low)](#7-work-breakdown--p3-low)
8. [Architecture Notes](#8-architecture-notes)
9. [Migration Strategy — Rollout Plan](#9-migration-strategy--rollout-plan)
10. [Testing Plan](#10-testing-plan)
11. [Risk Assessment](#11-risk-assessment)

---

## 1. Why Migrate?

`kube-system` is a privileged namespace shared with core Kubernetes components. Deploying monitoring addons here creates:
- **Blast radius** — a bad ama-metrics deployment could disrupt kube-system scheduling/quotas
- **RBAC confusion** — service accounts in kube-system have elevated access patterns
- **Resource quota conflicts** — kube-system may have different LimitRanges
- **Operational clarity** — dedicated namespace makes it obvious which resources belong to monitoring

---

## 2. Scope

### In Scope

All ama-metrics components currently in `kube-system`:
- `ama-metrics` Deployment (ReplicaSet)
- `ama-metrics-node` DaemonSet (Linux)
- `ama-metrics-win-node` DaemonSet (Windows)
- `ama-metrics-ksm` Deployment (Kube State Metrics)
- `ama-metrics-operator-targets` Deployment (Target Allocator + config-reader)
- All associated Services, ServiceAccounts, ConfigMaps, Secrets, HPA, PDB
- ClusterRoleBindings (subject namespace)
- CRDs (PodMonitor, ServiceMonitor) — these are cluster-scoped, no change needed
- ClusterRoles — cluster-scoped, no change needed

### Out of Scope

- Other kube-system workloads (coredns, kube-proxy, etc.) — they stay in kube-system
- Default scrape targets for coredns/kube-proxy/retina/cilium — the namespace filters in their scrape configs refer to **their** namespace, not ama-metrics'

---

## 3. Impact Summary

| Category | Files Affected | Severity |
|----------|---------------|----------|
| **Go source code** (compiled into binary) | 4 files, ~12 refs | **P0 — blocks image build** |
| **Addon Helm chart** (deployed by Flux) | ~20 templates, ~40 refs | **P0 — blocks deployment** |
| **OTel Collector config YAMLs** | 2 files | **P1** |
| **Fluent-Bit configs** | 3 files, ~5 refs | **P1** |
| **CCP plugin chart** | 4 files | **P1** |
| **prometheus-collector chart** | 3 files (has validation gate) | **P1** |
| **ConfigMap reference manifests** | ~7 files | **P2** |
| **ARM/Bicep templates** | 2 files | **P2** |
| **CI pipeline** | 1 file, 3 refs | **P2** |
| **Test files** | ~15 files, ~200 refs | **P3** |
| **Docs, dashboards, scripts** | ~10 files | **P3** |

The `$$POD_NAMESPACE$$` / `POD_NAMESPACE` mechanism (already used in ~20 places) is the correct pattern. The migration extends this to the remaining hardcoded references.

---

## 4. Work Breakdown — P0 (Blockers)

### 4.1 Go Source Code — Replace Hardcoded `kube-system` with `POD_NAMESPACE`

These are compiled into the container image. Must be changed and image rebuilt.

#### 4.1.1 `otelcollector/configuration-reader-builder/main.go`

| Line | Current | Fix |
|------|---------|-----|
| 320 | `"ama-metrics-operator-targets.kube-system.svc.cluster.local"` | Build DNS from `os.Getenv("POD_NAMESPACE")`: `fmt.Sprintf("ama-metrics-operator-targets.%s.svc.cluster.local", namespace)` |
| 372 | `namespace := "kube-system"` (server TLS secret) | `namespace := os.Getenv("POD_NAMESPACE")` with fallback |
| 432 | `namespace := "kube-system"` (client TLS secret) | Same |

**Impact**: TLS certificates will be generated with the new namespace in the SAN (Subject Alternative Name). The Target Allocator's server cert must have the correct DNS name for mTLS to work. This is **the most sensitive change** — if the cert SAN doesn't match the new service DNS, all collector↔TA mTLS connections fail.

#### 4.1.2 `otelcollector/shared/collector_replicaset_config_helper.go`

| Line | Current | Fix |
|------|---------|-----|
| 34 | `"http://ama-metrics-operator-targets.kube-system.svc.cluster.local"` | Build from `POD_NAMESPACE` env var |
| 112 | `"https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs"` | Same |

**Impact**: This is where the OTel Collector connects to the Target Allocator. Wrong namespace = collector can't get target assignments = no scraping for ReplicaSet pods.

#### 4.1.3 `otelcollector/shared/proxy_settings.go`

| Line | Current | Fix |
|------|---------|-----|
| 73 | `addNoProxy("ama-metrics-operator-targets.kube-system.svc.cluster.local")` | Build from `POD_NAMESPACE` |

**Impact**: If using HTTP proxy, TA traffic would be routed through the proxy instead of direct, causing latency or failures.

#### 4.1.4 `otelcollector/fluent-bit/src/telemetry.go`

| Line | Current | Fix |
|------|---------|-----|
| 600 | `"http://ama-metrics-operator-targets.kube-system.svc.cluster.local/scrape_configs"` | Build from `POD_NAMESPACE` |
| 604 | `"https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443/scrape_configs"` | Same |

**Impact**: Fluent-bit telemetry won't be able to query TA for target count metrics. Non-critical (diagnostics only), but causes telemetry gaps.

#### Recommended Pattern

All Go files should use a shared helper:

```go
func getNamespace() string {
    if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
        return ns
    }
    return "kube-system" // backward-compatible default
}

func getTAEndpoint(scheme string) string {
    ns := getNamespace()
    if scheme == "https" {
        return fmt.Sprintf("https://ama-metrics-operator-targets.%s.svc.cluster.local:443", ns)
    }
    return fmt.Sprintf("http://ama-metrics-operator-targets.%s.svc.cluster.local", ns)
}
```

---

### 4.2 Addon Helm Chart — Replace Hardcoded `namespace: kube-system`

Directory: `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/`

Every template currently has `namespace: kube-system`. Replace with `{{ .Release.Namespace }}`.

#### 4.2.1 Workloads

| Template File | Lines with `kube-system` | What to change |
|---------------|--------------------------|----------------|
| `ama-metrics-deployment.yaml` | L6 (metadata.namespace), L14 (`lookup` call), L320 (`--secret-namespace`) | Replace all with `{{ .Release.Namespace }}` |
| `ama-metrics-daemonset.yaml` | L21, L299 (Linux), L448, L604 (Windows) | Same |
| `ama-metrics-ksm-deployment.yaml` | L6 | Same |
| `ama-metrics-targetallocator.yaml` | L10, L49 (`OTELCOL_NAMESPACE` value), L136 (`POD_NAMESPACE` value) | Same — use `{{ .Release.Namespace }}` for env values |

#### 4.2.2 Supporting Resources

| Template File | What to change |
|---------------|----------------|
| `ama-metrics-serviceAccount.yaml` | `namespace: kube-system` → `{{ .Release.Namespace }}` |
| `ama-metrics-ksm-serviceaccount.yaml` | Same |
| `ama-metrics-ksm-service.yaml` | Same |
| `ama-metrics-ksm-clusterrolebinding.yaml` | `subjects[].namespace` → `{{ .Release.Namespace }}` |
| `ama-metrics-clusterRoleBinding.yaml` | `subjects[].namespace` → `{{ .Release.Namespace }}` |
| `ama-metrics-secret.yaml` | `namespace` × 2 |
| `ama-metrics-targetallocator-service.yaml` | `namespace` |
| `ama-metrics-collector-hpa.yaml` | `namespace` |
| `ama-metrics-pod-disruption-budget.yaml` | `namespace` |
| `ama-metrics-extensionIdentity.yaml` | `namespace` |
| `ama-metrics-scc.yaml` | `system:serviceaccount:kube-system:ama-metrics-serviceaccount` → use `{{ .Release.Namespace }}` |

#### 4.2.3 Helm Helpers

| File | Line | Current | Fix |
|------|------|---------|-----|
| `_ama-metrics-helpers.tpl` | L21 | `lookup "autoscaling/v2" "HorizontalPodAutoscaler" "kube-system"` | Use `{{ .Release.Namespace }}` |

#### 4.2.4 addon-token-adapter `--secret-namespace`

All workload templates pass `--secret-namespace=kube-system` to the addon-token-adapter sidecar. This tells it where to find the `aad-msi-auth-token` K8s secret.

**Critical question**: Does the `aad-msi-auth-token` secret move to the new namespace, or stay in `kube-system`?

- **If it moves** → change `--secret-namespace={{ .Release.Namespace }}`
- **If it stays in kube-system** → keep `--secret-namespace=kube-system`, but the addon-token-adapter needs RBAC to read secrets in `kube-system` from the new namespace

This depends on the AKS RP behavior — the RP creates this secret. Check with the AKS team.

---

### 4.3 CCP Plugin Chart

Directory: `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/`

| File | What to change |
|------|----------------|
| `ama-metrics-deployment.yaml` | `--configmap-namespace=kube-system`, `--secret-namespace=kube-system` |
| `ama-metrics-role.yaml` | `namespace: kube-system` |
| `ama-metrics-roleBinding.yaml` | `namespace: kube-system` |

---

## 5. Work Breakdown — P1 (High)

### 5.1 OTel Collector Config YAMLs

These are static files loaded at container startup. The Target Allocator endpoint is hardcoded.

| File | Line | Current |
|------|------|---------|
| `opentelemetry-collector-builder/collector-config-replicaset.yml` | 43 | `endpoint: https://ama-metrics-operator-targets.kube-system.svc.cluster.local:443` |
| `opentelemetry-collector-builder/ccp-collector-config-replicaset.yml` | 33 | `endpoint: http://ama-metrics-operator-targets.kube-system.svc.cluster.local` |

**Fix options**:
- **Option A**: Add a `$$POD_NAMESPACE$$` placeholder and have the configmap parser replace it at startup (consistent with existing pattern)
- **Option B**: Have `collector_replicaset_config_helper.go` dynamically patch the endpoint after loading the config (it already modifies the config at runtime)

Option B is cleaner since the helper already exists and does URL construction. Just ensure the Go code (§4.1.2) builds the URL dynamically.

### 5.2 Fluent-Bit Configs

| File | Line | Current | Fix |
|------|------|---------|-----|
| `fluent-bit/fluent-bit.yaml` | 107 | `host: ama-metrics-operator-targets.kube-system.svc.cluster.local` | Use `${POD_NAMESPACE}` env var in config (Fluent-Bit supports env expansion) |
| `fluent-bit/fluent-bit-daemonset.yaml` | 23 | `path: /var/log/containers/ama-metrics-ksm*kube-system*.log` | Change to `*ama-metrics-ns*` or use a wildcard `*ama-metrics*` |
| `fluent-bit/fluent-bit-daemonset.yaml` | 31 | `path: ...operator-targets*kube-system*targetallocator*.log` | Same |
| `fluent-bit/fluent-bit-daemonset.yaml` | 38 | `path: ...operator-targets*kube-system*config-reader*.log` | Same |

**Note on log paths**: Kubernetes container log filenames follow the pattern `/var/log/containers/{pod}_{namespace}_{container}-{id}.log`. After migration, the namespace segment changes from `kube-system` to `ama-metrics-ns`.

### 5.3 prometheus-collector Chart — Namespace Validation Gate

`otelcollector/deploy/chart/prometheus-collector/templates/_helpers.tpl` lines 70–79:

```
{{- define "mac-namespace-validate" -}}
  {{ $namespace := .Release.Namespace }}
  {{- if eq $namespace "kube-system" -}}
  namespace: {{ $namespace }}
  {{- end -}}
{{- end -}}
```

This **explicitly blocks** deploying to any namespace other than `kube-system`. Must be updated to allow the new namespace:

```
{{- if or (eq $namespace "kube-system") (eq $namespace "ama-metrics-ns") -}}
```

Or remove the gate entirely and just use `{{ .Release.Namespace }}`.

Also in the deployment template:
```
required "namespace value is incorrect. The valid value is 'kube-system'"
```

### 5.4 ConfigMap Reference Manifests

~7 files in `otelcollector/configmaps/` have `namespace: kube-system`:

- `ama-metrics-prometheus-config-configmap.yaml`
- `ama-metrics-prometheus-config-node-configmap.yaml`
- `ama-metrics-prometheus-config-node-windows-configmap.yaml`
- `ama-metrics-settings-configmap.yaml`
- `ama-metrics-settings-configmap-v1.yaml`
- `ama-metrics-settings-configmap-v2.yaml`
- `ama-metrics-settings-configmap-otel.yaml`

These are reference manifests that customers may apply. Update namespace and document in release notes.

---

## 6. Work Breakdown — P2 (Medium)

### 6.1 ARM/Bicep Deployment Templates

| File | What to change |
|------|----------------|
| `ArcArmTemplate/FullAzureMonitorMetricsProfile.json` | `"releaseNamespace": "kube-system"` |
| `ArcBicepTemplate/nested_...bicep` | `releaseNamespace: 'kube-system'` |

### 6.2 CI Pipeline

`.pipelines/azure-pipeline-build.yml` — 3 references deploying to `namespace: 'kube-system'`. Update deployment targets.

### 6.3 Retina Network Observability

`otelcollector/deploy/retina/custom-files/network-observability-service.yaml` — `namespace: kube-system`. This may need to stay in `kube-system` if retina lives there.

---

## 7. Work Breakdown — P3 (Low)

### 7.1 Test Files (~15 files, ~200 refs)

| Category | Files |
|----------|-------|
| Unit tests | `configmapparser_test.go`, `tomlparser-*_test.go` — `"POD_NAMESPACE": "kube-system"` in test env |
| E2E tests | `config_processing_test.go`, `container_status_test.go`, `liveness_test.go`, `prometheus_ui_test.go`, `regionTests_suite_test.go` |
| Test fixtures | `test/ci-cd/ci-cd-cluster.json`, test configmap YAMLs |
| Prom receiver tests | `prom_to_otlp_test.go` — test data |

### 7.2 Monitoring Dashboards

Production dashboards (`prod-near-ring-db.json`, `cicd-db.json`) have 100+ PromQL expressions with `namespace="kube-system"`.

### 7.3 Troubleshooting Scripts

`internal/scripts/troubleshoot/TroubleshootError.ps1` — ~15 kubectl commands targeting `-n kube-system`.

### 7.4 Documentation

- `internal/docs/1PChartToAddonMigration.md`
- `internal/referenceapp/otel-prometheus-reference-app.yaml`

---

## 8. Architecture Notes

### 8.1 What's Already Namespace-Aware (No Changes Needed)

| Component | Why it's safe |
|-----------|--------------|
| **Target Allocator Go code** (`otel-allocator/internal/collector/collector.go`) | Reads namespace from `OTELCOL_NAMESPACE` env var — fully parameterized |
| **Default scrape configs using `$$POD_NAMESPACE$$`** | Replaced at runtime by configmap parser using `os.Getenv("POD_NAMESPACE")` |
| **ClusterRoles / CRDs** | Cluster-scoped — namespace doesn't matter |
| **Downward API `POD_NAMESPACE` injection** | Already set via `fieldRef: metadata.namespace` in all Helm templates |

### 8.2 Service DNS Name Change

All inter-component communication uses K8s service DNS names. These change:

| Current | After migration |
|---------|----------------|
| `ama-metrics-operator-targets.kube-system.svc.cluster.local` | `ama-metrics-operator-targets.ama-metrics-ns.svc.cluster.local` |
| `ama-metrics-ksm.kube-system.svc.cluster.local` | `ama-metrics-ksm.ama-metrics-ns.svc.cluster.local` |

Affects: TA endpoint URLs, mTLS cert SANs, no_proxy entries, OTel collector config, Fluent-Bit config.

### 8.3 mTLS Certificate SAN Impact

The configuration-reader generates x509 certificates with SANs including the service DNS name. After migration:
- Server cert SAN must include `ama-metrics-operator-targets.ama-metrics-ns.svc.cluster.local`
- Client certs validate against this new SAN
- Old certs with `kube-system` SAN will fail verification after migration

**Important**: During rolling migration, both DNS names must be in the SAN to support both old and new pods.

### 8.4 `aad-msi-auth-token` Secret

The addon-token-adapter reads from a K8s secret `aad-msi-auth-token`. Currently in `kube-system`, created by the AKS RP.

Options:
1. **AKS RP creates secret in new namespace** — cleanest, but requires RP change
2. **Copy/mirror secret to new namespace** — operational complexity
3. **Keep secret in kube-system, grant cross-namespace RBAC** — addon-token-adapter needs `--secret-namespace=kube-system` + a Role/RoleBinding in kube-system

**Recommendation**: Coordinate with AKS RP team on option 1.

### 8.5 Default Scrape Configs — Which `kube-system` References Stay?

These scrape configs filter to `kube-system` because the **target** lives there (not because ama-metrics lives there):

| Config | Target | Action |
|--------|--------|--------|
| `corednsDefault.yml` | coredns pods in kube-system | **Keep** — coredns stays in kube-system |
| `kubeproxyDefault.yml` | kube-proxy in kube-system | **Keep** |
| `kappieBasicDefaultDs.yml` | kappie in kube-system | **Keep** |
| `networkobservabilityCiliumDefaultDs.yml` | cilium in kube-system | **Keep** |
| `networkobservabilityHubbleDefaultDs.yml` | hubble in kube-system | **Keep** |
| `networkobservabilityRetinaDefaultDs.yml` | retina in kube-system | **Keep** |
| `kubestateDefault.yml` | Uses `$$POD_NAMESPACE$$` | **Already dynamic** — works in any namespace |

**No changes needed to default scrape configs.** They scrape remote targets in their actual namespaces.

---

## 9. Migration Strategy — Rollout Plan

### Phase 1: Make Code Namespace-Agnostic

1. Update all Go code to use `POD_NAMESPACE` env var instead of hardcoded `kube-system`
2. Update collector-config YAML templates
3. Update Fluent-Bit configs
4. Build new container image
5. All changes must be **backward-compatible** — default to `kube-system` when env var is missing

### Phase 2: Update Helm Charts

1. Replace all `kube-system` literals with `{{ .Release.Namespace }}`
2. Remove the namespace validation gate in `_helpers.tpl`
3. Update `--secret-namespace` to be configurable (with documented default)
4. Test deploying to `kube-system` (regression) AND `ama-metrics-ns` (new)

### Phase 3: Update ARM/Bicep/CI

1. ARM templates: `releaseNamespace` parameter default changes
2. Bicep templates: same
3. CI pipeline: deploy to new namespace

### Phase 4: AKS RP Coordination

1. AKS RP must create the new namespace
2. AKS RP must create `aad-msi-auth-token` in new namespace (or provide cross-namespace access)
3. AKS RP Flux configuration must target the new namespace
4. The addon registration must specify the new `releaseNamespace`

### Phase 5: Rolling Migration for Existing Clusters

For existing clusters already running in `kube-system`:

1. New addon version deploys to new namespace
2. Old resources in `kube-system` must be cleaned up (Flux should handle this if `releaseNamespace` changes)
3. mTLS certs must include both DNS names during transition
4. Monitor for data gaps during the switchover

### Phase 6: Update Tests, Docs, Dashboards

1. Update all test files
2. Update monitoring dashboards (add `namespace=~"kube-system|ama-metrics-ns"` during transition, then switch)
3. Update troubleshoot scripts
4. Update customer-facing docs

---

## 10. Testing Plan

### Unit Tests

- Run all `configmapparser_test.go` tests with `POD_NAMESPACE=ama-metrics-ns`
- Verify generated scrape configs have correct namespace references
- Verify TA endpoint URL construction

### Integration Tests

- Deploy to `ama-metrics-ns` on a test cluster
- Verify:
  - [ ] OTel Collector connects to Target Allocator via new DNS name
  - [ ] mTLS works (certs have correct SAN)
  - [ ] MetricsExtension authenticates successfully (addon-token-adapter works)
  - [ ] MDSD writes TokenConfig.json to correct path
  - [ ] Health check passes (TokenConfig exists, all processes running)
  - [ ] Metrics appear in Azure Monitor Workspace
  - [ ] Fluent-Bit collects logs from new namespace paths
  - [ ] HPA scales correctly
  - [ ] PDB works correctly
  - [ ] ConfigMap changes trigger pod restart (inotify still works)
  - [ ] Prometheus UI shows targets

### Regression Tests

- Deploy to `kube-system` with same code changes — ensure nothing breaks
- Verify `POD_NAMESPACE` defaults to `kube-system` gracefully

### E2E Tests

- Update all e2e test suites to parameterize namespace
- Run full suite against both namespaces

---

## 11. Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| mTLS cert DNS mismatch during rollout | **Critical** | Include both old and new DNS names in SAN during transition |
| `aad-msi-auth-token` secret not in new namespace | **Critical** | Coordinate with AKS RP team early — this is a dependency |
| Flux doesn't clean up old kube-system resources | **High** | Manual cleanup script or Flux prune configuration |
| Monitoring dashboards show no data during migration | **High** | Update PromQL to `namespace=~"kube-system\|ama-metrics-ns"` first |
| Customer custom scrape configs reference `kube-system` | **Medium** | Document in release notes; `$$POD_NAMESPACE$$` is already the recommended pattern |
| Windows DaemonSet has separate template with its own namespace refs | **Medium** | Ensure Windows template is updated in lockstep |
| CCP mode (control plane) has different namespace assumptions | **Medium** | CCP may stay in kube-system — clarify scope early |
| Existing PodMonitor/ServiceMonitor CRDs may reference kube-system | **Low** | CRDs are cluster-scoped, but `namespaceSelector` in specs may need updating |

---

## Appendix A: FAQ — Cross-Namespace Communication

### Q: Will ama-metrics have problems calling services in `kube-system` (e.g., K8s API server) when running outside it?

**No.** Here's why for each communication path:

| Communication | Mechanism | Why it works outside kube-system |
|---|---|---|
| **K8s API server** | `rest.InClusterConfig()` → `kubernetes.default.svc` | API server lives in `default` namespace, not `kube-system`. In-cluster config reads the SA token from the pod's mounted volume — the calling pod's namespace is irrelevant |
| **kubelet/cAdvisor scraping** | Direct node IP (`:10250`) or `role: node` SD | Nodes are not namespaced. SA token is valid regardless of pod namespace |
| **coredns/kube-proxy scraping** | `role: pod` SD with `namespaces: [kube-system]` | The namespace filter refers to the **target's** namespace, not the scraper's. ClusterRole grants cluster-wide read access |
| **KSM scraping** | `ama-metrics-ksm.$$POD_NAMESPACE$$.svc.cluster.local:8080` | KSM co-deploys with ama-metrics — both move to the new namespace together |
| **Kappie/Retina/Cilium scraping** | `role: service` SD with label + namespace filters | Same as coredns — filters target namespace, not scraper |
| **MDSD/MetricsExtension → Azure** | Outbound HTTPS | Local processes, no K8s API calls, no namespace dependency |
| **addon-token-adapter** | Sidecar in same pod | Namespace doesn't matter — `localhost` communication |

**The key insight**: Prometheus service discovery with a **ClusterRole** can scrape any namespace regardless of where the scraper pod runs. The hardcoded `kube-system` references in scrape configs (coredns, kube-proxy, kappie, retina) are correct — they target workloads that genuinely reside in `kube-system`.

**What does matter**: The **ClusterRoleBinding subject** must reference the correct namespace for the ServiceAccount. This is already handled — the addon chart uses `subjects[].namespace: {{ $.Values.namespace }}`.

**No NetworkPolicy issues**: The addon chart deploys no NetworkPolicies. KSM and node-exporter sub-charts have them disabled by default (`networkPolicy.enabled: false`).

### Q2: Will customer custom scrape configs be impacted by the namespace migration?

**Yes.** Custom ConfigMaps must be recreated in the new namespace.

#### How custom config works today

Customers create ConfigMaps **manually** in `kube-system`:

| ConfigMap | Used by | Contains |
|-----------|---------|----------|
| `ama-metrics-prometheus-config` | Deployment (replicas) + Target Allocator | Custom `scrape_configs` for cluster-wide targets |
| `ama-metrics-prometheus-config-node` | DaemonSet (Linux) | Custom `scrape_configs` for node-local targets |
| `ama-metrics-prometheus-config-node-windows` | DaemonSet (Windows) | Custom `scrape_configs` for Windows nodes |
| `ama-metrics-settings-configmap` | All pods | Settings: which defaults to enable, keep-lists, intervals |

These are mounted as **optional** volumes (pods start fine without them):

```yaml
volumes:
  - name: prometheus-config-vol
    configMap:
      name: ama-metrics-prometheus-config
      optional: true          # pod runs even if CM doesn't exist
```

Mounted at `/etc/config/settings/prometheus/prometheus-config` inside the container. The Go code reads from this file path — it has **zero concept of namespaces**. It's pure file I/O.

#### Why namespace migration impacts custom config

**Kubernetes rule: a pod can only mount ConfigMaps from its own namespace.** The volume definition uses just the ConfigMap `name` (no namespace field) — Kubernetes implicitly looks in the pod's namespace.

If pods move from `kube-system` to the new namespace:
- ConfigMaps in `kube-system` become **invisible** to the new pods
- Customers must recreate all 4 ConfigMaps in the new namespace
- The Go code needs zero changes — it reads from `/etc/config/settings/...` regardless

#### Impact summary

| Aspect | Impact |
|--------|--------|
| **Go code** | No change — reads from file paths, namespace-unaware |
| **Helm templates** | No change — volume mount uses `name:` only, already correct |
| **Customer ConfigMaps** | **Must be recreated in the new namespace** |
| **Reference YAML examples** | Update `namespace: kube-system` → new namespace in all 7 files in `otelcollector/configmaps/` |
| **Documentation** | Must tell customers to create ConfigMaps in the new namespace |
| **Existing clusters during migration** | Custom configs will be **lost** until recreated — data gap risk |

#### Mitigation options

1. **Copy ConfigMaps during migration** — a migration job/script that copies all 4 ConfigMaps from `kube-system` to the new namespace before or during deployment
2. **Cross-namespace ConfigMap reference** — not natively supported in Kubernetes (pods can only mount from own namespace)
3. **Document in release notes** — tell customers to recreate their custom configs in the new namespace

### Q3: Will recording rules and alerts be impacted by the namespace migration?

**The rules themselves don't break, but metric label values shift — and customer-created rules can silently fail.**

#### How recording rules/alerts work

Recording rules and alerts are **Azure ARM resources** (`Microsoft.AlertsManagement/prometheusRuleGroups`), created via ARM/Bicep templates. They run PromQL against the Azure Monitor Workspace — they are **not** Kubernetes resources and don't live in any K8s namespace. No code changes are needed for them.

> **Key finding**: All 62 recording rules and 30 alerts contain **zero references to `kube-system`** in their PromQL expressions. The rules are not impacted by the namespace migration. However, rules that aggregate `by (namespace)` will produce **shifted output values** — the `kube-system` bucket decreases and a new namespace bucket appears at migration time. This is a cosmetic time-series discontinuity, not a functional breakage.

#### Q3a: Do built-in rules reference ama-metrics?

**Not directly, but they're subtly affected.** No rule mentions `ama-metrics` or `kube-system` by name. However, many rules use cadvisor and KSM metrics that **include ama-metrics pods** and aggregate by `namespace`:

| Rule type | Example | What happens after migration |
|---|---|---|
| **cadvisor CPU/memory** | `node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate` (`by namespace, pod`) | ama-metrics CPU/memory moves from `namespace="kube-system"` to `namespace="<new-ns>"` |
| **KSM resource requests/limits** | `namespace_cpu:kube_pod_container_resource_requests:sum` (`by namespace, cluster`) | `kube-system` totals **decrease**, new namespace totals **appear** — visible step-change on dashboards |
| **Workload owner rules** | `namespace_workload_pod:kube_pod_owner:relabel` | ama-metrics deployments/daemonsets shift namespace label |
| **~12 alert rules** | `KubePodCrashLooping`, `KubePodReadyStateLow`, `KubeDeploymentReplicasMismatch`, etc. | If ama-metrics triggers these, the alert's `namespace` label is the new namespace — not broken, just different |

**Nothing breaks** — but dashboards querying `namespace="kube-system"` totals will show a step-down at migration time as ama-metrics' resource usage leaves that namespace bucket.

#### Q3b: Will customer-created recording rules/alerts be impacted?

**Yes — potentially with silent failures.** Customers create `Microsoft.AlertsManagement/prometheusRuleGroups` via ARM/Bicep/Terraform containing arbitrary PromQL. Three risk patterns:

| Risk | Customer PromQL pattern | What happens |
|---|---|---|
| **HIGH — silent breakage** | `container_cpu_usage_seconds_total{namespace="kube-system", pod=~"ama-metrics.*"}` | Returns **0** after migration — alert stops firing, dashboard goes blank, **no error** |
| **HIGH — false positives** | `container_memory_working_set_bytes{namespace!="kube-system"}` (exclude system pods) | ama-metrics pods **start matching** after migration — unexpected alerts fire |
| **MEDIUM — value shift** | `sum(container_cpu_usage_seconds_total{namespace="kube-system"})` (broad kube-system monitoring) | Value silently decreases by the ama-metrics portion |
| **LOW — no impact** | `sum(rate(container_cpu_usage_seconds_total[5m]))` (no namespace filter) | Fully transparent — ama-metrics still counted, just with different label |

**The worst failure mode**: queries that filter `namespace="kube-system", pod=~"ama-metrics.*"` will **silently return empty results** — no error, no notification that the query broke.

#### Recommended mitigations

1. **Announce the namespace change** in release notes well before migration
2. **Call out explicitly** that any PromQL referencing `namespace="kube-system"` for ama-metrics pods must be updated
3. **Warn about exclusion patterns** — `namespace!="kube-system"` will start including ama-metrics pods
4. **Dashboard audit** — any Grafana dashboards querying `namespace_cpu:...:sum{namespace="kube-system"}` will show a step-down

---

## Appendix B: Full File Inventory

### Files Requiring Changes (by priority)

**P0 — Go code (4 files)**:
- `otelcollector/configuration-reader-builder/main.go`
- `otelcollector/shared/collector_replicaset_config_helper.go`
- `otelcollector/shared/proxy_settings.go`
- `otelcollector/fluent-bit/src/telemetry.go`

**P0 — Addon Helm chart (~20 files)**:
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-deployment.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-targetallocator.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-targetallocator-service.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-deployment.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-service.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-serviceaccount.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-clusterrolebinding.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-serviceAccount.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-clusterRoleBinding.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-secret.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-collector-hpa.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-pod-disruption-budget.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-extensionIdentity.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-scc.yaml`
- `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/_ama-metrics-helpers.tpl`

**P0 — CCP chart (4 files)**:
- `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-deployment.yaml`
- `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-role.yaml`
- `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-roleBinding.yaml`

**P1 — Runtime configs (5 files)**:
- `otelcollector/opentelemetry-collector-builder/collector-config-replicaset.yml`
- `otelcollector/opentelemetry-collector-builder/ccp-collector-config-replicaset.yml`
- `otelcollector/fluent-bit/fluent-bit.yaml`
- `otelcollector/fluent-bit/fluent-bit-daemonset.yaml`
- `otelcollector/deploy/chart/prometheus-collector/templates/_helpers.tpl`

**P1 — ConfigMaps (7 files)**:
- `otelcollector/configmaps/ama-metrics-prometheus-config-configmap.yaml`
- `otelcollector/configmaps/ama-metrics-prometheus-config-node-configmap.yaml`
- `otelcollector/configmaps/ama-metrics-prometheus-config-node-windows-configmap.yaml`
- `otelcollector/configmaps/ama-metrics-settings-configmap.yaml`
- `otelcollector/configmaps/ama-metrics-settings-configmap-v1.yaml`
- `otelcollector/configmaps/ama-metrics-settings-configmap-v2.yaml`
- `otelcollector/configmaps/ama-metrics-settings-configmap-otel.yaml`

**P2 — Deployment templates (3 files)**:
- `ArcArmTemplate/FullAzureMonitorMetricsProfile.json`
- `ArcBicepTemplate/nested_azuremonitormetrics_arc_k8s_extension_clusterResourceId.bicep`
- `.pipelines/azure-pipeline-build.yml`

**P3 — Tests (~15 files)**, docs (~5 files), dashboards (~4 files), scripts (~1 file)

### Files That Need NO Changes

- `otelcollector/otel-allocator/internal/collector/collector.go` — already parameterized via `OTELCOL_NAMESPACE`
- All scrape configs using `$$POD_NAMESPACE$$` — already dynamic
- ClusterRoles, CRDs — cluster-scoped
- Default scrape configs filtering to `kube-system` for coredns/kube-proxy/retina — those targets stay in kube-system
