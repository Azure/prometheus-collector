# Proposal: AMA-Metrics Custom Namespace Support

## Problem

1. **Istio incompatibility** — Istio cannot inject sidecars into `kube-system`, where ama-metrics currently runs. This blocks Istio mTLS and service mesh integration.
2. **AKS Automatic restrictions** — AKS Automatic clusters do not allow customers to create resources (e.g., ConfigMaps) in `kube-system`, preventing custom scrape configuration for ama-metrics.

## Approach

<!-- TODO -->

## Changes

### Helm Chart (16 template files)
- Added `namespace` field to `values-template.yaml` (default: `kube-system`)
- Replaced all hardcoded `namespace: kube-system` with `{{ $.Values.namespace }}` in every template
- Updated `lookup` calls and `--secret-namespace` args to use the configured namespace

### Go Code (4 files)
- Added `getTargetAllocatorNamespace()` helper (checks `OTELCOL_NAMESPACE` → `POD_NAMESPACE` → `kube-system` fallback)
- Parameterized target allocator service URLs, TLS cert DNS SANs, secret creation namespaces, and NO_PROXY entries

### OTel Collector + Fluent-Bit Configs (4 files)
- Target allocator endpoint URLs use `${env:POD_NAMESPACE}` instead of hardcoded `kube-system`

 thi
- **aks-rp must be modified** to create this secret in the configured custom namespace instead
- For demo/prototyping, the secret was manually copied from `kube-system` to the target namespace

## Risks

### 1. Namespace Migration Implementation

<!-- TODO -->

### 2. Istio Support

<!-- TODO -->

### 3. Azure AKS Automatic Cluster Support

<!-- TODO -->

### 4. Azure Arc Cluster Support

<!-- TODO -->
