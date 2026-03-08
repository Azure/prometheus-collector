# SPIKE Proposal for Azure Managed Prometheus Support AKS ISTIO and AKS AUTOMATIC CLUSTER


This is a DRAFT, and should not be shared outside of Azure Managed Prometheus team.

## Goal
Azure Managed Prometheus needs to support AKS Istio and AKS Automatic clusters. However, because ama-metrics agent currently runs in the `kube-system` namespace, it is unable to support these cluster types.


## Assumptions (TO BE VALIDATED)
1. **Istio incompatibility** — Istio cannot inject sidecars into `kube-system`, where ama-metrics currently runs. This blocks Istio mTLS and service mesh integration.
2. **AKS Automatic restrictions** — AKS Automatic clusters do not allow customers to create resources (e.g., ConfigMaps) in `kube-system`, preventing custom scrape configuration for ama-metrics.

## Solution
Based on the assumptions, this doc assumes that the only possible solution is to migrate Azure Managed Prometheus out of the `kube-system` namespace. However, other solutions might be worth investigation, see the section [Open Questions and Discussions](#open-questions-and-discussions).

## Scope
- AKS cluster (with and without Istio)
- AKS Automatic cluster

## Spike Tasks

### 0.1 Investigate and confirm existing issues of AKS cluster with Istio
- Create an AKS cluster with Istio and ama-metrics enabled, investigate and document all issues.

### 0.2 Investigate and confirm existing issues of AKS automatic cluster
- Create an AKS Automatic cluster with ama-metrics enabled, investigate and document all issues.

### 1. ama-metrics agent support customized namespace through hacky/hardcode changes
- Research changes needed for the ama-metrics agent so it can run in customized namespace. Some studies have been done. (see [Appendix A](#appendix-a-azure-managed-prometheus-code-changes) for details)
- Note this is only for ama-metrics agent that is from the prometheus-collector repo, and it does not include any other services, e.g. token-adaptor.
- **Priority:** High. We need this to support many other tasks below.

### 2. Identify issues of ama-metrics agent in customized namespace in AKS cluster
ama-metrics agent will have several issues when running in customized namespace.

One known issue is the secret `aad-msi-auth-token`. It is currently created by AKS RP in `kube-system` only, thus, **AKS RP needs to be modified** so it can create this secret in the configured custom namespace. As a result, we need to file a dependency to aks team for secret creation and refresh in the customized namespace used by ama-metrics agent.

Another potential issue is the token-adaptor itself, which is developed by the AKS team and runs inside `kube-system` namespace. ama-metrics agent and token-adaptor need to run in the same namespace, thus token-adaptor will need to run outside `kube-system` namespace, which may have issues. Based on hacky work that deployed ama-metrics agent in a customized namespace, token-adaptor works fine outside `kube-system` namespace, but this needs to be thoroughly tested and confirmed by the token-adaptor team.

To thoroughly understand all other issues, based on Task 1, we can deploy the ama-metrics agent in a customized namespace to an AKS cluster and identify the issues.

- **Priority:** High


### 3. Investigate issues of ama-metrics agent in customized namespace in AKS Cluster with Istio enabled
Based on the outcome of Task 1, we deploy the hacky ama-metrics agent that can work in a customized namespace in an AKS cluster with Istio enabled, and investigate all the issues.

For example, when ama-metrics agent runs outside `kube-system` namespace and its communication is managed by Istio, the agent may have permission issues when making calls to services in `kube-system` (e.g., Kubernetes API server) because Istio can't inject sidecar inside `kube-system`.

Another example, Istio injects a sidecar that intercepts all traffic. ama-metrics currently relies on token-adaptor for auth — there could be conflicts between token-adaptor and the Istio-injected sidecar.

To thoroughly understand all other issues, based on Task 1, we can deploy the ama-metrics agent in a customized namespace to an AKS cluster and identify the issues.

- **Priority:** High

### 4. (Risk) Resource and scheduling priorities in non-kube-system namespace
- priority class name of ama-metrics is system-node-critical, so theoretically there should be no resource priority changes when running in a non-kube-system namespace.
- **Risk mitigation:**
  - Get clarification from AKS team on priority class behavior outside `kube-system`
  and or
  - Perform capacity and pressure testing in custom namespace
- **Priority:** High

### 5. (Migration) Customer ConfigMaps
- Customers may have existing ConfigMaps (e.g., custom scrape configs) in `kube-system`. How do we migrate existing customers from `kube-system` to the custom namespace without disruption?
- **Priority:** High

### 6. Recording rules and alert rules
- Existing recording rules and alert rules could be impacted by the namespace change. This task will investigate how to handle recording rules and alert rules in customized namespace, which will include both new rules and existing rules that need to be migrated.
- **Priority:** High


### 7. Investigate issues of ama-metrics agent in customized namespace in AKS Automatic Cluster
Based on the outcome of Task 1, we deploy the hacky ama-metrics agent that can work in a customized namespace in an AKS Automatic cluster and investigate the issues.


### 8. (Side Impact study) Investigate issues of ama-metrics agent in customized namespace in AKS Arc Cluster
When ama-metrics agent runs outside `kube-system`, AKS Arc cluster might be impacted. Based on the outcome of Task 1, we deploy the hacky ama-metrics agent that can work in a customized namespace in an AKS Arc cluster and investigate the issues. One issue is related to token-adaptor, similarly to the token-adaptor challenge in AKS cluster, but they could be different because token-adaptor uses different images for AKS Cluster and AKS Arc Cluster.

## Open Questions and Discussions
1. We have assumed that migrating ama-metrics agent outside of `kube-system` is the solution, but we should also be open to other approaches:
   - **Istio whitelisting:** There could be solutions on the Istio side to whitelist ama-metrics agent pods so they can communicate with other pods without going through the Istio sidecar.
   - **AKS Automatic exceptions:** For AKS Automatic clusters, exceptions might be made for certain services so they can create resources in `kube-system`.
   - **Cross-namespace ConfigMap access:** ama-metrics agent could remain in `kube-system` while accessing ConfigMaps in other namespaces.

## Appendix A: Azure Managed Prometheus Code Changes
Note: below findings are from a quick research, and more changes may be required. A more thorough investigation is needed.

### Helm Chart (16 template files)
- Added `namespace` field to `values-template.yaml` (default: `kube-system`)
- Replaced all hardcoded `namespace: kube-system` with `{{ $.Values.namespace }}` in every template
- Updated `lookup` calls and `--secret-namespace` args to use the configured namespace

### Go Code (4 files)
- Added `getTargetAllocatorNamespace()` helper (checks `OTELCOL_NAMESPACE` → `POD_NAMESPACE` → `kube-system` fallback)
- Parameterized target allocator service URLs, TLS cert DNS SANs, secret creation namespaces, and NO_PROXY entries

### OTel Collector + Fluent-Bit Configs (4 files)
- Target allocator endpoint URLs use `${env:POD_NAMESPACE}` instead of hardcoded `kube-system`
