# AMA-Metrics Custom Namespace Support — Branch Review Summary

**Branch:** `zane/istio-play` vs `main`  
**Reviewed:** February 27, 2026  
**Total:** 42 files changed, ~3,687 insertions, ~110 deletions

---

## Overview

This branch parameterizes the ama-metrics Helm chart and supporting code so that ama-metrics can be deployed into **any Kubernetes namespace** (not just the default `kube-system`). A new `namespace` value in `values.yaml` controls the target namespace, defaulting to `kube-system` for backward compatibility.

---

## 1. Helm Chart Changes (16 template files + values)

### values-template.yaml
Added a top-level `namespace` field:
```yaml
# Custom namespace for deployment (defaults to kube-system for compatibility)
namespace: "kube-system"
```

### Template Files — `namespace: kube-system` → `{{ $.Values.namespace }}`

Every hardcoded `namespace: kube-system` in metadata was replaced with `{{ $.Values.namespace }}`:

| File | Changes |
|------|---------|
| `ama-metrics-daemonset.yaml` | Linux + Windows DaemonSet metadata, `--secret-namespace` args (×2) |
| `ama-metrics-deployment.yaml` | Deployment metadata, `lookup` call, `--secret-namespace` arg |
| `ama-metrics-collector-hpa.yaml` | HPA metadata |
| `ama-metrics-serviceAccount.yaml` | ServiceAccount metadata |
| `ama-metrics-clusterRoleBinding.yaml` | Subject namespace reference |
| `ama-metrics-ksm-deployment.yaml` | KSM Deployment metadata |
| `ama-metrics-ksm-service.yaml` | KSM Service metadata |
| `ama-metrics-ksm-serviceaccount.yaml` | KSM ServiceAccount metadata |
| `ama-metrics-ksm-clusterrolebinding.yaml` | Subject namespace reference |
| `ama-metrics-pod-disruption-budget.yaml` | PDB metadata |
| `ama-metrics-secret.yaml` | Proxy config + proxy cert secrets (×2) |
| `ama-metrics-scc.yaml` | SCC metadata |
| `ama-metrics-targetallocator.yaml` | Deployment metadata, `OTELCOL_NAMESPACE` env, `POD_NAMESPACE` env |
| `ama-metrics-targetallocator-service.yaml` | Service metadata |
| `ama-metrics-extensionIdentity.yaml` | ServiceAccount namespace in Arc identity spec |
| `_ama-metrics-helpers.tpl` | `lookup` call for HPA uses `.Values.namespace` |

### New Files
- `Chart.yaml` — Generated chart file (appVersion 1.0.0)
- `values.yaml` — Generated values file with `namespace: "ama-metrics-zane-test"` for testing

---

## 2. Go Code Changes (4 files)

All hardcoded `kube-system` references in target allocator service URLs were parameterized using environment variables.

### New Helper Function (added to 3 files)
```go
func getTargetAllocatorNamespace() string {
    if ns := os.Getenv("OTELCOL_NAMESPACE"); ns != "" { return ns }
    if ns := os.Getenv("POD_NAMESPACE"); ns != "" { return ns }
    return "kube-system"
}
```

| File | What Changed |
|------|-------------|
| `configuration-reader-builder/main.go` | Added `getNamespace()`. TLS cert DNS SAN uses dynamic namespace. Secret creation uses dynamic namespace (server + client TLS secrets). |
| `shared/collector_replicaset_config_helper.go` | Added `getTargetAllocatorNamespace()`. HTTP/HTTPS target allocator URLs use dynamic namespace. |
| `fluent-bit/src/telemetry.go` | Added `getTargetAllocatorNamespace()`. Scrape config URLs (HTTP + HTTPS) use dynamic namespace. |
| `shared/proxy_settings.go` | NO_PROXY entry uses dynamic namespace instead of hardcoded `kube-system`. |

**Environment Variable Priority:** `OTELCOL_NAMESPACE` → `POD_NAMESPACE` → `"kube-system"` (fallback)

---

## 3. OTel Collector Config Changes (3 YAML files)

Target allocator endpoint URLs changed from hardcoded `kube-system` to `${env:POD_NAMESPACE}`:

| File | Change |
|------|--------|
| `collector-config-replicaset.yml` | `https://ama-metrics-operator-targets.${env:POD_NAMESPACE}.svc.cluster.local:443` |
| `ccp-collector-config-replicaset.yml` | `http://ama-metrics-operator-targets.${env:POD_NAMESPACE}.svc.cluster.local` |
| `shared/configmap/mp/testdata/collector-config-replicaset.yml` | Test data updated to match |

**Note:** These files also had indentation changes (extra indent level under `receivers:`).

---

## 4. Fluent-Bit Config Change (1 file)

| File | Change |
|------|--------|
| `fluent-bit/fluent-bit.yaml` | Host changed to `ama-metrics-operator-targets.${POD_NAMESPACE:-kube-system}.svc.cluster.local` (bash-style default) |

---

## 5. ARM Template Changes (2 files)

| File | Change |
|------|--------|
| `FullAzureMonitorMetricsProfile.json` | Commented out the addon-enable deployment section (41 lines removed) |
| `FullAzureMonitorMetricsProfileParameters.json` | Updated parameter values for `zane-custom-ns` cluster |

---

## 6. New `istio-fun/` Directory

Relevant documentation and deployment scripts for custom namespace support:

| File | Purpose |
|------|---------|
| `NAMESPACE_DEPLOYMENT_FINDINGS.md` | Detailed findings from custom namespace testing |
| `CUSTOM_NAMESPACE_GUIDE.md` | Step-by-step guide for custom namespace deployment |
| `deploy-simple.sh` | Simplified deployment script with configurable `NAMESPACE` |
| `uninstall-ama-metrics.sh` | Clean uninstall script |
| `parameterize-helm-templates.sh` | Script that automates the `kube-system` → `{{ $.Values.namespace }}` replacement |

**Ignored:** Files under `istio-fun/ignore-now/` (Istio mTLS policies, destination rules, etc.) are unrelated to custom namespace support.

---

## Key Finding: `aad-msi-auth-token` Secret

The most important discovery: deploying to a custom namespace requires the `aad-msi-auth-token` secret, which is **only created by the AKS control plane** when the managed addon is enabled.

### Working Workflow
1. **Enable** managed addon → creates secret in `kube-system`
2. **Disable** managed addon → secret persists
3. **Deploy** Helm chart to custom namespace
4. **Copy** secret from `kube-system` to custom namespace (must be done AFTER deploy, not before, because the deploy script recreates the namespace)
5. Pods auto-recover via liveness probes

### Verified Result
✅ **All 8 pods Running/Ready** in custom namespace `ama-metrics-zane-test` (Dec 1, 2025)

---

## Files That Can Be Ignored

- `istio-fun/ignore-now/*` — Istio policies, destination rules, mTLS guides (moved to ignore-now, separate concern)
- ARM template parameter changes (`FullAzureMonitorMetricsProfileParameters.json`) — Cluster-specific values
- `values.yaml` (generated) — Test-specific generated file with hardcoded cluster details
