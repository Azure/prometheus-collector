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
- **Status: Goal I-B path validated end-to-end** against `zane-istio-test` (AKS 1.33.8, ASM `asm-1-27`, ama-metrics `6.27.0`). See [`./istio-investigation/REPORT.md`](./istio-investigation/REPORT.md) for the full investigation, evidence, and reproducible YAMLs.
  - **Headline finding:** ama-metrics running in `kube-system` (no sidecar, no mesh identity) successfully scrapes user-app metrics in an Istio-meshed namespace **even under STRICT mTLS**, by default. Istio's mutating webhook automatically rewrites the pod's `prometheus.io/port` and `prometheus.io/path` annotations to point at the merged-metrics endpoint `:15020/stats/prometheus`, which is `Captured: No` in Istio's port table — i.e. the sidecar does not intercept it and `PeerAuthentication` mTLS rules do not apply.
  - **Two customer-side failure modes** were identified, both fixable without product code changes:
    - **Failure (a)** — customer disables `ISTIO_META_ENABLE_PROMETHEUS_MERGE`: the merged endpoint returns Envoy stats only, app metrics are missing. Fix: re-enable merge, or use one of the Failure (b) port-level fixes to scrape the app port directly.
    - **Failure (b)** — customer's custom scrape config / `PodMonitor` / `ServiceMonitor` hardcodes the app port (e.g. `:8080`): under STRICT mTLS the scrape is rejected. Three verified fixes:
      - **Fix C (recommended):** rewrite the scrape config to target `:15020/stats/prometheus`.
      - **Fix A:** port-level PERMISSIVE `PeerAuthentication` for the app port.
      - **Fix B:** pod annotation `traffic.sidecar.istio.io/excludeInboundPorts: "<port>"`.
- **Sub-deliverables:**
  - **Confirm with customer/PM which Istio mode is required:**
    - **Goal I-A:** ama-metrics IS in the mesh (sidecar injected on ama-metrics pods, mTLS to other meshed services, subject to Istio AuthorizationPolicy).
    - **Goal I-B:** ama-metrics is EXCLUDED from the mesh (`sidecar.istio.io/inject: false` annotation; behaves the same as in non-Istio clusters). **This is the current state in `kube-system` and the investigation confirms it works under STRICT mTLS.**
  - **Goal I-B is acceptable → the Istio half of this spike collapses.** Namespace migration is **not** required for Istio support. The remaining work is documentation (publish the `:15020/stats/prometheus` rewrite guidance for custom scrape configs).
  - **If Goal I-A is later required**, enumerate the sidecar compatibility issues — open, not yet investigated:
    - Token-adaptor + IMDS at `169.254.169.254` likely needs `traffic.sidecar.istio.io/excludeOutboundIPRanges: 169.254.169.254/32` annotation.
    - Mesh-to-non-mesh egress: ama-metrics scraping `kube-system` targets (CoreDNS, kube-proxy, kube-state-metrics, node-exporter, kubelet, cAdvisor, kube-apiserver, kappie, retina, hubble, cilium, ACStor, local CSI driver) requires a per-target carve-out strategy because `kube-system` pods have no sidecar to terminate mTLS. **Mitigation strategy is fully characterized in [Cross-namespace concerns / Layer 3](#cross-namespace-concerns-after-migration)** — DestinationRule `tls.mode: DISABLE` for service-FQDN targets, `traffic.sidecar.istio.io/excludeOutboundPorts` for pod-IP / node-IP / hostNetwork targets.
- **Priority:** High (Phase 0 — gates Task 3). **I-B sub-deliverable complete; I-A sub-deliverable open pending customer/PM confirmation that I-A is required.**

### 2.5 (common) Evaluate alternatives to namespace migration
Take the catalog of failure modes produced by Tasks 1 and 2 and, for each failure mode, evaluate whether it can be solved **without** moving ama-metrics out of `kube-system`. Specifically evaluate:
- **Istio whitelisting:** Can sidecar injection be enabled for `kube-system` via namespace label, OR can ama-metrics pods be excluded from mesh enforcement via annotation while still allowing customer pods in mesh? Does the customer goal require ama-metrics to be *in* the mesh, or just *not blocked by* the mesh?
- **AKS Automatic exceptions:** Can ama-metrics be added to AKS Automatic's allowlist of first-party services that *can* create resources in `kube-system`? (ama-logs, retina, kube-proxy, coredns, csi-* etc. all live in `kube-system` today — there is precedent.)
- **Cross-namespace ConfigMap watch:** Can ama-metrics remain in `kube-system` while watching ConfigMaps in customer namespaces, satisfying the AKS Automatic ConfigMap restriction without moving the pod?

**Decision criterion:** Return "go for migration" unless alternatives can fully cover **both** AKS Automatic *and* AKS Istio support scenarios. Partial coverage = migrate (for whatever the alternatives don't cover).

**Status (Istio half — resolved):** Task 2 confirmed that the I-B alternative (ama-metrics excluded from mesh, current `kube-system` posture) fully covers the AKS Istio scrape-meshed-app scenario under STRICT mTLS via Istio's `:15020/stats/prometheus` exemption. **Istio is no longer a forcing function for namespace migration.** AKS Automatic (Task 1) remains the open driver. If Task 1 also clears with non-migration alternatives, the migration may be reduced or deferred.

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

**Scope reduced after Task 2.** The I-B variant (ama-metrics in custom namespace, *excluded* from mesh via `sidecar.istio.io/inject: false`) is mechanically identical to today's `kube-system` behavior — the Istio data path doesn't see ama-metrics, the `:15020` merged endpoint covers meshed-app scraping, and the only delta vs `kube-system` is the agent's pod namespace. Task 2's findings (see [`./istio-investigation/REPORT.md`](./istio-investigation/REPORT.md)) carry over. This sub-task collapses to a regression check on the new namespace.

The I-A variant (ama-metrics injected with a sidecar in the custom namespace) is the new investigation surface — only feasible with namespace migration since `kube-system` cannot be meshed:

- When ama-metrics agent runs outside `kube-system` namespace and its communication is managed by Istio, the agent may have permission issues when making calls to services in `kube-system` (e.g., Kubernetes API server) because Istio can't inject sidecar inside `kube-system`.
- Istio injects a sidecar that intercepts all traffic. ama-metrics currently relies on token-adaptor for auth — there could be conflicts between token-adaptor and the Istio-injected sidecar.
- IMDS at `169.254.169.254` likely needs `traffic.sidecar.istio.io/excludeOutboundIPRanges: 169.254.169.254/32`.
- Mesh-to-non-mesh egress to `kube-system` scrape targets (see Cross-namespace concerns / Layer 3) needs a mitigation strategy.

- **Priority:** TBD. Only triggered if customer/PM confirms Goal I-A is required. The I-B regression check is low-effort and folds into Task 4. The I-A investigation should be sequenced after Task 3 is done because we could encounter permission issues due to pod permissions being managed by Istio.


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
- **If Goal I-B** (ama-metrics excluded from mesh): plain HTTP scraping, no Envoy interception, works exactly as today. **Validated** — see [`./istio-investigation/REPORT.md`](./istio-investigation/REPORT.md). Scraping a meshed app under STRICT mTLS works via Istio's `:15020/stats/prometheus` merged endpoint (annotation rewriting handles this automatically). For customers with custom scrape configs, the only gotcha is that the config must target `:15020/stats/prometheus`, not the app port — documentation fix only.
- **If Goal I-A** (ama-metrics in mesh) with STRICT mTLS: ama-metrics's Envoy sidecar will attempt mTLS for outbound traffic, but `kube-system` targets have no sidecar to terminate it. The detailed mitigation strategy depends on **how each target is discovered** and whether it's reachable through Istio's service registry. See **Layer 3a** below.

**Layer 3a: I-A egress to `kube-system` scrape targets — detailed analysis.**

*Default-install behavior: auto-mTLS often handles this without any explicit mitigation.* Istio's auto-mTLS (on by default) asks istiod about each destination; if the destination has no sidecar (no `security.istio.io/tlsMode: istio` label), Envoy automatically downgrades to plaintext. Explicit mitigations become necessary only when the customer (or platform operator) has done one of: (a) defined a wildcard `DestinationRule` with `tls.mode: ISTIO_MUTUAL` that overrides auto-mTLS, (b) enabled `outboundTrafficPolicy: REGISTRY_ONLY`, or (c) configured a `meshConfig.defaultDestinationRule` that forces mTLS origination.

*Per-target verdict*, derived from the actual scrape configs in `otelcollector/configmapparser/default-prom-configs/`:

| Target | Discovery | `__address__` after relabel | Recommended carve-out |
|---|---|---|---|
| CoreDNS (`kube-dns` job) | `role: pod` ns=kube-system | `<podIP>:9153` (HTTP) | DestinationRule on `kube-dns.kube-system.svc.cluster.local` (service-resolved path) **plus** `excludeOutboundPorts: "9153"` belt-and-braces (pod-IP path may bypass DR) |
| kube-proxy | `role: pod` ns=kube-system | `<podIP>:10249` (HTTP, hostNetwork → pod IP == node IP) | `excludeOutboundPorts: "10249"` (no backing Service for DR to bind to) |
| kube-state-metrics (`ama-metrics-ksm`) | `static_configs` FQDN | `$$KUBE_STATE_NAME$$.$$POD_NAMESPACE$$.svc.cluster.local:8080` | **None** — KSM moves with ama-metrics into `azure-monitoring-metrics` (deployed by same addon chart), both pods sidecar-injected, mTLS works natively |
| node-exporter | `static_configs` node-IP | `$$NODE_IP$$:$$NODE_EXPORTER_TARGETPORT$$` (HTTP, hostNetwork) | Works under default `outboundTrafficPolicy: ALLOW_ANY` (passthrough). Add `excludeOutboundPorts: "9100"` for `REGISTRY_ONLY` resilience |
| kubelet / cAdvisor | `static_configs` node-IP | `$$NODE_IP$$:10250` (HTTPS, Prometheus does its own TLS with SA bearer token) | **`excludeOutboundPorts: "10250"` required** — Envoy attempting mTLS over Prometheus's existing TLS will break the scrape |
| kube-apiserver | `role: endpoints` ns=default | `<apiserverEP>:443` (HTTPS, Prometheus does its own TLS) | DestinationRule `tls.mode: DISABLE` on `kubernetes.default.svc.cluster.local` (registered service), OR `excludeOutboundPorts: "443"` |

*Recommended package* (ship both with the addon chart, scoped to ama-metrics' namespace so the customer's mesh posture is untouched):

1. **DestinationRules** for service-FQDN targets:
   ```yaml
   apiVersion: networking.istio.io/v1beta1
   kind: DestinationRule
   metadata: { name: ama-metrics-apiserver-no-mtls, namespace: azure-monitoring-metrics }
   spec:
     host: kubernetes.default.svc.cluster.local
     trafficPolicy: { tls: { mode: DISABLE } }
   ---
   apiVersion: networking.istio.io/v1beta1
   kind: DestinationRule
   metadata: { name: ama-metrics-coredns-no-mtls, namespace: azure-monitoring-metrics }
   spec:
     host: kube-dns.kube-system.svc.cluster.local
     trafficPolicy: { tls: { mode: DISABLE } }
   ```

2. **Sidecar exclusions** (pod template annotations on ama-metrics) for everything that bypasses the service registry — pod-IP scrapes, node-IP scrapes, hostNetwork pods, and TLS-on-TLS scrapes:
   ```yaml
   traffic.sidecar.istio.io/excludeOutboundPorts: "9153,10249,10250,9100"
   ```
   Rationale: CoreDNS `9153`, kube-proxy `10249`, kubelet/cAdvisor `10250`, node-exporter `9100`. Port `443` is intentionally **not** excluded — leaving the API server flowing through DestinationRule preserves mesh telemetry on that hop.

*Mitigations explicitly ruled out (called out so future readers don't reintroduce them):*
- **`PeerAuthentication mode: PERMISSIVE` on `kube-system`** — no-op. PeerAuthentication is server-side and `kube-system` pods have no Envoy to enforce it.
- **`AuthorizationPolicy` carve-outs** — runs after TLS termination. The failure here is at the TLS handshake, so AuthZ is never reached.

*Long-term maintenance debt:* `excludeOutboundPorts` is a port-list that must be kept in sync as AKS adds new `kube-system` add-ons with new metrics ports. Two fallback options if this proves brittle: (a) `excludeOutboundIPRanges` over the cluster pod CIDR (coarser, self-maintaining; loses telemetry on all kube-system flows), or (b) a `Sidecar` resource declaring ama-metrics' egress surface explicitly (most YAML, best documentation of intent).

**Layer 4 (related): K8s ≥1.36 secrets access.** The existing ClusterRole already shifts to namespace-scoped Roles for K8s ≥1.36. After migration, ama-metrics still needs to read (a) `aad-msi-auth-token` in `kube-system` (gated on the AKS-RP coordination workstream), (b) `ama-metrics-mtls-secret`, and (c) customer-defined PodMonitor/ServiceMonitor `basicAuth` secrets in arbitrary namespaces. Verification is a sub-deliverable of Task 4.


## Open Questions and Discussions
1. We have assumed that migrating ama-metrics agent outside of `kube-system` is the solution, but we should also be open to other approaches. **These alternatives are now formally evaluated in Task 2.5** (see Tasks section); the items below remain documented for context:
   - **Istio whitelisting:** There could be solutions on the Istio side to whitelist ama-metrics agent pods so they can communicate with other pods without going through the Istio sidecar.
   - **AKS Automatic exceptions:** For AKS Automatic clusters, exceptions might be made for certain services so they can create resources in `kube-system`.
   - **Cross-namespace ConfigMap access:** ama-metrics agent could remain in `kube-system` while accessing ConfigMaps in other namespaces.

2. **AKS Arc cluster** — Scope or not? Deferred. Working assumption: Arc remains in `kube-system` and the standalone helm chart at `otelcollector/deploy/chart/prometheus-collector/` is **not** parameterized in this spike unless this question flips to "in scope."

3. **Istio mode (Goal I-A vs I-B)** — **Partially resolved.** I-B has been investigated and validated end-to-end (see [`./istio-investigation/REPORT.md`](./istio-investigation/REPORT.md)): the current `kube-system` deployment scrapes meshed apps under STRICT mTLS via Istio's `:15020/stats/prometheus` merged endpoint, no product changes required. I-A remains open pending customer/PM confirmation that ama-metrics-in-mesh is actually required by any customer scenario. Cost difference:
   - **I-B (excluded from mesh):** zero code change; one optional annotation if migrating to a meshable namespace; documentation fix for customers with custom scrape configs.
   - **I-A (in mesh):** requires namespace migration (kube-system can't be meshed), per-target mTLS exemptions for `kube-system` scrape targets, token-adaptor IMDS exemption, and AuthorizationPolicy compatibility work.

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
