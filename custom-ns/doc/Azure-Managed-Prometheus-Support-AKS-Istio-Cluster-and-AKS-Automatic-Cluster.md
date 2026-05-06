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
The working hypothesis is to migrate Azure Managed Prometheus out of the `kube-system` namespace. This is **contingent on the outcome of the Phase 0 validation tasks** (Tasks 1, 2, and 2.5 below); namespace migration is not a foregone conclusion until those tasks confirm no lower-blast-radius alternative exists. See [Open Questions and Discussions](#open-questions-and-discussions) for the alternatives still on the table.

## Scope
- AKS automatic cluster and AKS cluster (with Istio disabled)
- AKS cluser (with Istio enabled)

Priorties are in order.

### Migration mode
If Task 2.5 returns a "go for migration" verdict, **all customers** (every AKS cluster type in scope) get migrated to the custom namespace — uniform migration, not hybrid. Hybrid (per-cluster-type namespace selection) is parked as a fallback option to revisit if uniform migration proves too risky during rollout planning.

### Production namespace name
The production namespace is `azure-monitoring-metrics` (single fixed name, not customer-configurable). The single source of truth is the helm `values.yaml` `namespace:` field in the prometheus-collector repo. AKS-RP must match the value set here. Runtime centralization already exists via the `POD_NAMESPACE` downward-API env var (`shared/namespace.go`) and `$$POD_NAMESPACE$$` substitution in default scrape configs — minimal Go changes are needed beyond the helm parameterization. Parameterization in helm exists for dev/test only; production deployments use the fixed name.

The namespace is for ama-metrics only — sharing with `ama-logs` or other Azure Monitor agents is explicitly out of scope for this design. AKS Automatic team must be asked to allowlist this specific name (no auto-allowlist via prefix patterns).

## Out of Scope
- **Namespace migration as a foregone conclusion.** Namespace migration is the working hypothesis but is *gated* on Tasks 1, 2, and 2.5 confirming no alternative covers both AKS Automatic and AKS Istio support goals. If alternatives can fully cover both goals, migration may not happen at all.
- **CCP runtime namespace.** CCP runs in the Azure-managed control plane (Underlay), not in the customer cluster — neither AKS Istio nor AKS Automatic restrictions apply to CCP pods. CCP's runtime namespace is unchanged. The only CCP-side impact is updating the configmap-name reference once the customer-side namespace changes (this is part of the deferred AKS-RP coordination workstream).
- **AKS Arc.** Deferred — see Open Question #2. Working assumption: Arc remains in `kube-system`, the standalone helm chart at `otelcollector/deploy/chart/prometheus-collector/` is **not** parameterized in this spike unless Arc is later brought in.
- **`ama-logs` or other Azure Monitor agents** — this design is scoped to ama-metrics only.


## Depndencies
### Upstream
- secret `aad-msi-auth-token` in aks-rp
  
- token-adaptor in aks-rp
- retina
  it is a dependency use for network observabiltiy. 

### Downstream
- ccp/aks-rp
  ccp runs Underlay, and it copies configmap from kube-system namespace from Overlay. CCP dependes on some resources in AKS-RP that copies the configmap over.

### External coordination workstream (deferred)
The aks-rp repo is owned by the Azure AKS resource provider team, not the ama-metrics team. The following dependencies require coordination with that team and are tracked as a separate workstream — they are not designed in this spike but are hard prerequisites for production rollout:

- **`aad-msi-auth-token` secret namespace.** Today created by AKS-RP in `kube-system`; ama-metrics in the new namespace cannot mount it directly. Solution path (e.g., AKS-RP creates the secret in the new namespace, ama-metrics replicates it via controller, or use Secret CSI driver) is deferred to that workstream. Sequencing: any AKS-RP change must ship and be active in the field **before** ama-metrics flips to the new namespace, otherwise customers hit an MDM-auth outage on upgrade.
- **CCP-side configmap-name reference.** When the customer-side configmap moves to `azure-monitoring-metrics`, AKS-RP's configmap-copy mechanism (Overlay → Underlay) and CCP's watcher must track the new name.
- **AKS Automatic allowlist for `azure-monitoring-metrics`.** AKS Automatic enforces Deployment Safeguards and may restrict resource creation outside system namespaces; the new namespace must be added to the first-party agent allowlist.


## Tasks
Tasks are categroized in following categories:
aks automatic - related to AKS Automatic cluster support
aks istio - related to AKS Istio
common - related to both above

**Phase 0 (Tasks 1, 2, 2.5) is gating for Task 3 and downstream work.** No Phase 0 task may be skipped before starting the migration refactor.

### 1 (aks automatic) Investigate existing issues of ama-metrics in AKS automatic cluster
- Create an AKS Automatic cluster with ama-metrics enabled, investigate and document all issues.
- **Sub-deliverables:**
  - Verify that `system-node-critical` and `system-cluster-critical` priority classes are admitted in non-`kube-system` namespaces under AKS Automatic Deployment Safeguards. Vanilla Kubernetes and AKS standard do not restrict these, but AKS Automatic's mandatory Azure Policy / Gatekeeper add-on is the area of concern. If blocked, fallback options to evaluate: (a) custom PriorityClass owned by AKS-RP, or (b) request a policy carve-out for first-party agents.
  - Verify AKS Automatic's default NetworkPolicy posture. If a default-deny baseline is enforced, an explicit egress NetworkPolicy from `azure-monitoring-metrics` to `kube-system` will be required for ama-metrics to scrape kube-system targets (CoreDNS, kube-proxy, retina, etc.).
- **Priority:** High (Phase 0 — gates Task 3)

### 2 (aks istio) Investigate existing issues of AKS cluster with Istio
- Create an AKS cluster with Istio and ama-metrics enabled, investigate and document all issues.
- **Sub-deliverables:**
  - **Confirm with customer/PM which Istio mode is required:**
    - **Goal I-A:** ama-metrics IS in the mesh (sidecar injected on ama-metrics pods, mTLS to other meshed services, subject to Istio AuthorizationPolicy).
    - **Goal I-B:** ama-metrics is EXCLUDED from the mesh (`sidecar.istio.io/inject: false` annotation; behaves the same as in non-Istio clusters).
  - **If Goal I-B is acceptable, the Istio half of this spike collapses to a single annotation** — namespace migration is not needed for Istio reasons. Document this as the simpler path.
  - **If Goal I-A is required**, enumerate the sidecar compatibility issues:
    - Token-adaptor + IMDS at `169.254.169.254` likely needs `traffic.sidecar.istio.io/excludeOutboundIPRanges: 169.254.169.254/32` annotation.
    - Mesh-to-non-mesh egress: ama-metrics scraping `kube-system` targets (CoreDNS, kube-proxy, kappie, retina, hubble, cilium, ACStor, local CSI driver) under STRICT mTLS will fail because `kube-system` pods have no sidecar. Mitigations to evaluate (any one): per-target `DestinationRule` with `tls.mode: DISABLE`, namespace-level `PeerAuthentication` of `PERMISSIVE` for `kube-system`, or AuthorizationPolicy carve-outs.
- **Priority:** High (Phase 0 — gates Task 3)

### 2.5 (common) Evaluate alternatives to namespace migration
Take the catalog of failure modes produced by Tasks 1 and 2 and, for each failure mode, evaluate whether it can be solved **without** moving ama-metrics out of `kube-system`. Specifically evaluate:
- **Istio whitelisting:** Can sidecar injection be enabled for `kube-system` via namespace label, OR can ama-metrics pods be excluded from mesh enforcement via annotation while still allowing customer pods in mesh? Does the customer goal require ama-metrics to be *in* the mesh, or just *not blocked by* the mesh?
- **AKS Automatic exceptions:** Can ama-metrics be added to AKS Automatic's allowlist of first-party services that *can* create resources in `kube-system`? (ama-logs, retina, kube-proxy, coredns, csi-* etc. all live in `kube-system` today — there is precedent.)
- **Cross-namespace ConfigMap watch:** Can ama-metrics remain in `kube-system` while watching ConfigMaps in customer namespaces, satisfying the AKS Automatic ConfigMap restriction without moving the pod?

**Decision criterion:** Return "go for migration" unless alternatives can fully cover **both** AKS Automatic *and* AKS Istio support scenarios. Partial coverage = migrate (for whatever the alternatives don't cover).

- **Priority:** High (Phase 0 — gates Task 3). Depends on outputs of Tasks 1 and 2.

### 3 (common) Refactor ama-metrics codebase (prometheus-collector repo) to support custom namespace
- Identify and implemente changes of ama-metrics so it can run in custom namespace. (see [Appendix A](#appendix-a-azure-managed-prometheus-code-changes) for details)
- Note this is only for ama-metrics agent that is from the prometheus-collector repo, and it does not include any other services, e.g. token-adaptor.
- **Gated on Tasks 1, 2, and 2.5 returning a "go for migration" verdict.** If alternatives cover all goal-blocking failure modes, this task may be reduced or skipped.
- **Priority:** High (post Phase 0). We need this to support many other tasks below.

### 4 (common) Identify issues of ama-metrics in custom namespace in AKS cluster
We need identify the issues of ama-metrics agent when it runs in custom namspace. Based on task 3, this can be done by deploying ama-metrics agent through helm chart into an AKS cluster. we will need evaluate regualr dataflow, and performance testings. 

When ama-metrics runs outside kube-system, it may has lower priority for resoulrce allocation from AKS. Theoretically, because ama-metrics will still keep the `system-node-critical` priority class, there should be no resource priority changes. But we may want to perform our tests to validate this. Additionally, we can get clarification from AKS team on priority class behavior outside `kube-system` (vanilla Kubernetes and AKS standard do not restrict use of `system-node-critical` outside `kube-system` — see Task 1's verification of AKS Automatic Deployment Safeguards).

Note that there is a known based on previous tests.
1. secret `aad-msi-auth-token`: it is currently created by AKS RP in `kube-system`. Instead of waiting for AKS RP changes to support `aad-msi-auth-token` in a customized namespace, in this task, ama-logs will be enabled in the test cluster, and the secrect will be copied over to the customized namespace duirng testings. (This is a **test-environment hack**; the production solution is part of the deferred external AKS-RP coordination workstream.)

**Sub-deliverables:**
- Verify on K8s ≥1.36 that ama-metrics retains secret read access for (a) `aad-msi-auth-token` in `kube-system`, (b) `ama-metrics-mtls-secret`, and (c) customer-defined PodMonitor/ServiceMonitor `basicAuth` secrets in arbitrary namespaces. The existing ClusterRole already shifts to namespace-scoped Roles for K8s ≥1.36, so the migration interacts with that pattern in a non-obvious way.

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
- **DESIGN DEFERRED.** This is the highest production-impact item in the plan given the uniform-migration commitment. Because no design has been chosen, **Task 3 can be built but not safely shipped to production until this is resolved.** Whatever we decide may push code changes back into Task 3 (e.g., dual-namespace ConfigMap watcher logic). Customer-facing ConfigMaps to migrate are: `ama-metrics-settings-configmap`, `ama-metrics-prometheus-config`, `ama-metrics-prometheus-config-node`, `ama-metrics-prometheus-config-node-windows`.
- **Priority:** High (rollout blocker)

### 9 Recording rules and alert rules
- Existing recording rules and alert rules could be impacted by the namespace change. This task will investigate how to handle recording rules and alert rules in customized namespace, which will include both new rules and existing rules that need to be migrated.
- **Priority:** High


## Cross-namespace concerns after migration

When ama-metrics moves from `kube-system` to `azure-monitoring-metrics`, it still needs to scrape targets that live in `kube-system` (CoreDNS, kube-proxy, kappie, retina, hubble, cilium, ACStor, local CSI driver, etc.). Three layers were analyzed:

**Layer 1: Kubernetes API permission (RBAC) — NOT a concern.** ama-metrics uses a `ClusterRole` + `ClusterRoleBinding` (`ama-metrics-reader`) granting cluster-wide `list/get/watch` on pods, services, endpoints, namespaces, etc. The ClusterRoleBinding subject is the SA in `{{ $.Values.namespace }}` — it works the same regardless of which namespace ama-metrics is deployed to. Cross-namespace target *discovery* via the K8s API is unaffected.

**Layer 2: Network reachability — verify in Task 1.** AKS standard does not enforce default-deny `NetworkPolicy`. AKS Automatic *may* enforce a default-deny baseline; if so, an explicit egress NetworkPolicy from `azure-monitoring-metrics` to `kube-system` (and possibly other namespaces) is required.

**Layer 3: AKS Istio mTLS — gated on the Goal I-A vs I-B decision (Task 2).**
- **If Goal I-B** (ama-metrics excluded from mesh): plain HTTP scraping, no Envoy interception, works exactly as today. No problem.
- **If Goal I-A** (ama-metrics in mesh) with STRICT mTLS: ama-metrics's Envoy sidecar requires mTLS for outbound, but `kube-system` targets have no sidecar (Istio cannot inject into `kube-system`). Result: scrapes to those targets fail. Mitigations (any one): per-target `DestinationRule` with `tls.mode: DISABLE`; namespace-level `PeerAuthentication` of `PERMISSIVE` for `kube-system`; AuthorizationPolicy carve-outs.

**Layer 4 (related): K8s ≥1.36 secrets access.** The existing ClusterRole already shifts to namespace-scoped Roles for K8s ≥1.36. After migration, ama-metrics still needs to read (a) `aad-msi-auth-token` in `kube-system` (gated on the AKS-RP coordination workstream), (b) `ama-metrics-mtls-secret`, and (c) customer-defined PodMonitor/ServiceMonitor `basicAuth` secrets in arbitrary namespaces. Verification is a sub-deliverable of Task 4.


## Open Questions and Discussions
1. We have assumed that migrating ama-metrics agent outside of `kube-system` is the solution, but we should also be open to other approaches. **These alternatives are now formally evaluated in Task 2.5** (see Tasks section); the items below remain documented for context:
   - **Istio whitelisting:** There could be solutions on the Istio side to whitelist ama-metrics agent pods so they can communicate with other pods without going through the Istio sidecar.
   - **AKS Automatic exceptions:** For AKS Automatic clusters, exceptions might be made for certain services so they can create resources in `kube-system`.
   - **Cross-namespace ConfigMap access:** ama-metrics agent could remain in `kube-system` while accessing ConfigMaps in other namespaces.

2. **AKS Arc cluster** — Scope or not? Deferred. Working assumption: Arc remains in `kube-system` and the standalone helm chart at `otelcollector/deploy/chart/prometheus-collector/` is **not** parameterized in this spike unless this question flips to "in scope."

3. **Istio mode (Goal I-A vs I-B)** — Open. To be answered in Task 2 by confirmation with customer/PM. Doc enumerates both paths so the cost difference is visible:
   - **I-B (excluded from mesh):** single annotation, no other changes; namespace migration not needed for Istio reasons.
   - **I-A (in mesh):** requires per-target mTLS exemptions for `kube-system` scrape targets, plus token-adaptor IMDS exemption.

4. **Customer ConfigMap migration semantics (Task 8)** — Design deferred. Rollout blocker for Task 3. Options to reconsider: hard cutover (unacceptable), dual-watch with deprecation, auto-migrate at startup, cross-namespace watch.

5. **Hybrid migration model** — Parked as fallback. If uniform migration proves too risky during rollout planning, hybrid (per-cluster-type namespace selection) can be revisited.

6. **Rollout / phased deployment / toggle strategy** — Not yet designed.

7. **Validation / success criteria for Tasks 4–6** — Not yet specified.


## Appendix A: Azure Managed Prometheus Code Changes
This appendix enumerates the code changes required for Task 3 (the ama-metrics refactor). Categories below are based on a code audit of files referencing `kube-system` in the prometheus-collector repo.

### A.1 In scope — addon helm chart (already mostly parameterized)
The addon helm chart at `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/` has been parameterized with `{{ $.Values.namespace }}`. Remaining:
- Set `values-template.yaml` and `values.yaml` `namespace:` field default to `azure-monitoring-metrics` (currently `ama-metrics-zane-test`, a personal test value — must be changed before any ship).
- Update `local_testing_aks.ps1` references to `kube-system`.
- Verify `lookup` calls and `--secret-namespace` args use the configured namespace.

### A.2 In scope — Go code
- `otelcollector/shared/namespace.go` and `otelcollector/fluent-bit/src/telemetry.go`: **remove the silent `"kube-system"` fallback** when `POD_NAMESPACE` is unset. Make `POD_NAMESPACE` required; fatal-error/panic if missing. (Decision: fail loud — fallbacks mask deployment misconfigurations and would silently split-brain a pod after migration.)
- `otelcollector/configuration-reader-builder/main.go`: remove/parameterize `kube-system` reference.
- Target allocator: `getTargetAllocatorNamespace()` helper (checks `OTELCOL_NAMESPACE` → `POD_NAMESPACE`, no `kube-system` fallback).
- Parameterize target allocator service URLs, TLS cert DNS SANs, secret creation namespaces, and NO_PROXY entries.

### A.3 In scope — OTel Collector + Fluent-Bit configs
- Target allocator endpoint URLs use `${env:POD_NAMESPACE}` instead of hardcoded `kube-system`. Already mostly done; verify across `collector-config-replicaset.yml`, `ccp-collector-config-replicaset.yml`, `fluent-bit.yaml`, `fluent-bit-daemonset.yaml`.
- `otelcollector/prom-config-validator-builder/prometheus-config.yaml`: validator config has `kube-system` reference — needs review.

### A.4 In scope — Mixins / dashboards / recording-rule tests (per-file review)
6 files in `mixins/` reference `kube-system`. Per-file review needed because some references are about *target* services that genuinely live in `kube-system` (CoreDNS) and don't change, while dashboards filtering on `namespace="kube-system"` for the *agent's* metrics will break:
- `mixins/kubernetes/dashboards/network-usage/namespace-by-workload.libsonnet`
- `mixins/kubernetes/dashboards/network-usage/namespace-by-pod.libsonnet`
- `mixins/kubernetes/dashboards/network-usage/workload-total.libsonnet`
- `mixins/kubernetes/dashboards/network-usage/pod-total.libsonnet`
- `mixins/kubernetes/tests.yaml`
- `mixins/coredns/tests.yaml`

This category overlaps with Task 9 (Recording rules and alert rules).

### A.5 In scope only for the configmap-name reference — CCP plugin helm chart
CCP runtime is OUT of scope (CCP runs in the Underlay, not in the customer cluster — neither Istio nor Automatic restrictions apply). However, 3 files in `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/` reference `kube-system` for the customer-side configmap name. These need updating in concert with the deferred AKS-RP coordination workstream:
- `ama-metrics-deployment.yaml`
- `ama-metrics-role.yaml`
- `ama-metrics-roleBinding.yaml`

### A.6 Conditional on AKS Arc decision — standalone helm chart
4 files in `otelcollector/deploy/chart/prometheus-collector/` reference `kube-system`. **Out of scope** under the current working assumption (Arc remains in `kube-system`). Brought back into scope if Open Question #2 flips to "Arc is in scope":
- `templates/_helpers.tpl`
- `templates/prometheus-collector-daemonset.yaml`
- `templates/prometheus-collector-deployment.yaml`
- `templates/prometheus-collector-secretProviderClass.yaml`

### A.7 Out of scope — references to `kube-system` as a SCRAPE TARGET location
The following files reference `kube-system` because they describe the namespace where target services (CoreDNS, kube-proxy, kappie, retina, hubble, cilium, ACStor, local CSI driver) live. These targets stay in `kube-system` regardless of where ama-metrics runs, so **no change needed**:
- 8 files in `otelcollector/configmapparser/default-prom-configs/`
- 7 files in `otelcollector/configmaps/` (example/template configmaps)
- 2 files in `otelcollector/deploy/example-default-scrape-configs/`

## Appendix B: AKS Arc cluster
When ama-metrics agent runs outside `kube-system`, AKS Arc cluster might be impacted. Based on the outcome of Task 1, we deploy the hacky ama-metrics agent that can work in a customized namespace in an AKS Arc cluster and investigate the issues. One issue is related to token-adaptor, similarly to the token-adaptor challenge in AKS cluster, but they could be different because token-adaptor uses different images for AKS Cluster and AKS Arc Cluster.
