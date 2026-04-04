# Proposal: AMA-Metrics Custom Namespace Support

## Problem

1. **Istio incompatibility** — Istio cannot inject sidecars into `kube-system`, where ama-metrics currently runs. This blocks Istio mTLS and service mesh integration.
2. **AKS Automatic restrictions** — AKS Automatic clusters do not allow customers to create resources (e.g., ConfigMaps) in `kube-system`, preventing custom scrape configuration for ama-metrics.

## Scope

- AKS cluster (with and without Istio)
- AKS Arc cluster
- AKS Automatic cluster

## Approach

<!-- TODO -->

## High-Level Tasks

### 1. ama-metrics agent in customized namespace
- Parameterize Helm chart and Go code to support deploying to any namespace (see [Appendix A](#appendix-a-ama-metrics-repo-code-changes) for details)

### 2. Token adapter secret in customized namespace
- `aad-msi-auth-token` secret is currently created by AKS RP in `kube-system` only
- **aks-rp must be modified** to create this secret in the configured custom namespace instead
- For demo/prototyping, the secret was manually copied from `kube-system` to the target namespace

## Dependencies

- **AKS RP**: Must support creating the `aad-msi-auth-token` secret in a customized namespace (instead of only `kube-system`)
- **Arc**: Must support token adapter running in a customized namespace

## Rollout Plan and Migration Strategy

### Stage 1: AKS clusters
- <!-- TODO -->

### Stage 2: AKS clusters with Istio
- <!-- TODO -->

### Stage 3: AKS Arc clusters
- <!-- TODO -->

### Stage 4: AKS Automatic clusters
- <!-- TODO -->

## Risks and Open Questions

### 1. Migration strategy
- How do we migrate existing customers from `kube-system` to the custom namespace without disruption?
- **Risk mitigation:** <!-- TODO -->
- **Priority:** High

### 2. Customer ConfigMaps and alerts
- Customers may have existing ConfigMaps (e.g., custom scrape configs) and alert configurations in `kube-system` that reference the current namespace
- Need to investigate how the namespace change affects these customer-facing resources
- **Risk mitigation:** Deploy ama-metrics agent in custom namespace with secrets and investigate the impact on customer configs
- **Priority:** High

### 3. Resource and scheduling priorities in non-kube-system namespace
- ama-metrics is annotated as a system-critical pod, so theoretically there should be no resource priority changes when running in a non-kube-system namespace
- **Risk mitigation:**
  - Get clarification from AKS team on priority class behavior outside `kube-system`
  - Perform capacity and pressure testing in custom namespace
- **Priority:** Medium

### 4. Recording rules and alert rules
- Recording rules and alert rules could be impacted by the namespace change
- **Risk mitigation:** <!-- TODO -->
- **Priority:** Medium

### 5. AKS Automatic clusters
- <!-- TODO -->
- **Priority:** High

## Appendix A: ama-metrics Repo Code Changes

### Helm Chart (16 template files)
- Added `namespace` field to `values-template.yaml` (default: `kube-system`)
- Replaced all hardcoded `namespace: kube-system` with `{{ $.Values.namespace }}` in every template
- Updated `lookup` calls and `--secret-namespace` args to use the configured namespace

### Go Code (4 files)
- Added `getTargetAllocatorNamespace()` helper (checks `OTELCOL_NAMESPACE` → `POD_NAMESPACE` → `kube-system` fallback)
- Parameterized target allocator service URLs, TLS cert DNS SANs, secret creation namespaces, and NO_PROXY entries

### OTel Collector + Fluent-Bit Configs (4 files)
- Target allocator endpoint URLs use `${env:POD_NAMESPACE}` instead of hardcoded `kube-system`
