# SPIKE Proposal for Azure Managed Prometheus Support AKS ISTIO and AKS AUTOMATIC CLUSTER


This is a DRAFT, and should not be shared outside of Azure Managed Prometheus team.

## Goal
Azure Managed Prometheus needs to support AKS Istio and AKS Automatic clusters. However, because ama-metrics agent currently runs in the `kube-system` namespace, it is unable to support these cluster types. See the [Assumptions](#assumptions) for reasons. 


## Assumptions
1. **AKS Istio incompatibility**
- (TO BE VALIDATED) Istio cannot inject sidecars into `kube-system`, where ama-metrics currently runs. This blocks Istio mTLS and service mesh integration.

2. **AKS Automatic restrictions**
- AKS Automatic clusters do not allow customers to create resources (e.g., ConfigMaps) in `kube-system`, preventing custom scrape configuration for ama-metrics.

## Solution
Based on the assumptions, this doc assumes that the only possible solution is to migrate Azure Managed Prometheus out of the `kube-system` namespace. However, other solutions might be worth investigation, see the section [Open Questions and Discussions](#open-questions-and-discussions).

## Scope
- AKS automatic cluster and AKS cluster (with Istio disabled)
- AKS cluser (with Istio enabled)

Priorties are in order.

## Out of Scope
- solutions other than custom namespace. The goal of this project is to support AKS Automatic cluster and AKS Istio feature, instead of migrating ama-metrics out of kube-system. However, we assume that namespace migration is the soluiton for these goals due the the restrictions of running ama-metrics in kube-system for AKS Automatic cluster and AKS cluter with istio enabled.


## Depndencies
### Upstream
- secret `aad-msi-auth-token` in aks-rp
  
- token-adaptor in aks-rp
- retina
  it is a dependency use for network observabiltiy. 

### Downstream
- ccp/aks-rp
  ccp runs Underlay, and it copies configmap from kube-system namespace from Overlay. CCP dependes on some resources in AKS-RP that copies the configmap over.


## Tasks
Tasks are categroized in following categories:
aks automatic - related to AKS Automatic cluster support
aks istio - related to AKS Istio
common - related to both above

### 1 (aks automatic) Investigate existing issues of ama-metrics in AKS automatic cluster
- Create an AKS Automatic cluster with ama-metrics enabled, investigate and document all issues.
- **Priority:** Medium

### 2 (aks istio) Investigate existing issues of AKS cluster with Istio
- Create an AKS cluster with Istio and ama-metrics enabled, investigate and document all issues.
- **Priority:** TBD

### 3 (common) Refactor ama-metrics codebase (prometheus-collector repo) to support custom namespace
- Identify and implemente changes of ama-metrics so it can run in custom namespace. (see [Appendix A](#appendix-a-azure-managed-prometheus-code-changes) for details)
- Note this is only for ama-metrics agent that is from the prometheus-collector repo, and it does not include any other services, e.g. token-adaptor.
- **Priority:** High. We need this to support many other tasks below.

### 4 (common) Identify issues of ama-metrics in custom namespace in AKS cluster
We need identify the issues of ama-metrics agent when it runs in custom namspace. Based on task 3, this can be done by deploying ama-metrics agent through helm chart into an AKS cluster. we will need evaluate regualr dataflow, and performance testings. 

When ama-metrics runs outside kube-system, it may has lower priority for resoulrce allocation from AKS. Theoretically, because ama-metrics will still keep the `system-node-critical` priority class, there should be no resource priority changes. But we may want to perform our tests to validate this. Additionally, we can get clarification from AKS team on priority class behavior outside `kube-system`

Note that there is a known based on previous tests.
1. secret `aad-msi-auth-token`: it is currently created by AKS RP in `kube-system`. Instead of waiting for AKS RP changes to support `aad-msi-auth-token` in a customized namespace, in this task, ama-logs will be enabled in the test cluster, and the secrect will be copied over to the customized namespace duirng testings.

- **Priority:** High

### 5. (aks automatic) Identify issue of ama-metrics in custom namespace in AKS Automatic Cluster
Similar as 4, we deploy ama-metrics in custom namespace to an AKS Automatic cluster through helm chart approach and investiate issues. On top of task 4, we should not expect ay new issue from this task.

- **Priority:** High

### 6 (aks istio) Investigate issues of ama-metrics agent in custom namespace in AKS Cluster with Istio enabled
Similar as Task 4, we deploy ama-metrics in custom namespace to an AKS cluster with Istio enabled through helm chart approach and investiate issues. We will need to research different configurations of Istio for AKS and investigate Istio scenarios both ama-metrics can support and can't support when it runs in custom namespace.

For example, when ama-metrics agent runs outside `kube-system` namespace and its communication is managed by Istio, the agent may have permission issues when making calls to services in `kube-system` (e.g., Kubernetes API server) because Istio can't inject sidecar inside `kube-system`.

Another example, Istio injects a sidecar that intercepts all traffic. ama-metrics currently relies on token-adaptor for auth — there could be conflicts between token-adaptor and the Istio-injected sidecar.

- **Priority:** TBD. Depending on the proriot8y of aks istio support. If it has high priority, this task should be priotirized once Task 3 is done because we could encounter permission issues due to pod permissions are managed by Istio.


### 7 (common) AKS-RP Helm Chart Changes 
ama-metrics has helm chart in aks-rp repo. The helm chart needs to be modified to use custom namespace.
- **Priority:** Low. This is needed when ama-metrics need to rollout the changes.


### 8 (common) Customer ConfigMaps Migration
- Some customers' clusters have configmaps (e.g., custom scrape configs) in `kube-system` namespace. Deploying ama-metrics in a custom namespace will break these clusters. We need develop a plan how to handle the migration of the configmaps to new namespace.
- **Priority:** High

### 9 Recording rules and alert rules
- Existing recording rules and alert rules could be impacted by the namespace change. This task will investigate how to handle recording rules and alert rules in customized namespace, which will include both new rules and existing rules that need to be migrated.
- **Priority:** High


## Open Questions and Discussions
1. We have assumed that migrating ama-metrics agent outside of `kube-system` is the solution, but we should also be open to other approaches:
   - **Istio whitelisting:** There could be solutions on the Istio side to whitelist ama-metrics agent pods so they can communicate with other pods without going through the Istio sidecar.
   - **AKS Automatic exceptions:** For AKS Automatic clusters, exceptions might be made for certain services so they can create resources in `kube-system`.
   - **Cross-namespace ConfigMap access:** ama-metrics agent could remain in `kube-system` while accessing ConfigMaps in other namespaces.

2. - AKS Arc cluster 
Scope or not?


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

## Appendix B: AKS Arc cluster
When ama-metrics agent runs outside `kube-system`, AKS Arc cluster might be impacted. Based on the outcome of Task 1, we deploy the hacky ama-metrics agent that can work in a customized namespace in an AKS Arc cluster and investigate the issues. One issue is related to token-adaptor, similarly to the token-adaptor challenge in AKS cluster, but they could be different because token-adaptor uses different images for AKS Cluster and AKS Arc Cluster.
