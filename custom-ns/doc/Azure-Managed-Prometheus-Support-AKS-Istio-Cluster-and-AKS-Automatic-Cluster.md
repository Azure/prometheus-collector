# MIGRATION OF AMA-METRIC NAMESPACE TO SUPPORT AKS ISTIO and AKS AUTOMATIC CLUSTER

This is a DRAFT, and should not be shared outside of Azure Managed Prometheus team.

## Goal
Azure Managed Prometheus needs to support AKS Istio and AKS Automatic clusters. However, because ama-metrics agent currently runs in the `kube-system` namespace, it is unable to support these cluster types. See the [Assumptions](#assumptions) for reasons. 


## Assumptions
1. **AKS Istio incompatibility**
- ama-metrics has sidecar injected after moving outside kube-system.
- Istio cannot inject sidecars into `kube-system`, where ama-metrics currently runs. This blocks ama-metrics to scrap customers' pods that have istio sidecar injected with mTLS.

2. **AKS Automatic restrictions**
- AKS Automatic clusters do not allow customers to create resources (e.g., ConfigMaps) in `kube-system`, preventing custom scrape configuration for ama-metrics.

## Solution
The working hypothesis is to migrate Azure Managed Prometheus out of the `kube-system` namespace. This is **contingent on the outcome of the Phase 0 validation tasks** (Tasks 1, 2); namespace migration is not a foregone conclusion until those tasks confirm no lower-blast-radius alternative exists. See [Open Questions and Discussions](#open-questions-and-discussions) for the alternatives still on the table.

## Scope
- AKS automatic cluster and AKS cluster (with Istio disabled)
- AKS cluser (with Istio enabled)
- AKS Arc cluster (**TO BE DISCUSSED**)

Priorties are in order.

## Out of Scope
- **Namespace migration as a foregone conclusion.** Namespace migration is the working hypothesis but is *gated* on Tasks 1, 2, and 2.5 confirming no alternative covers both AKS Automatic and AKS Istio support goals. If alternatives can fully cover both goals, migration may not happen at all.
- **CCP runtime namespace.** CCP runs in the Azure-managed control plane (Underlay), not in the customer cluster — neither AKS Istio nor AKS Automatic restrictions apply to CCP pods. CCP's runtime namespace is unchanged. The only CCP-side impact is updating the configmap-name reference once the customer-side namespace changes.

## Depndencies
- **`aad-msi-auth-token` secret namespace.** Per AKS team (token-adaptor owner Tongyao, email 2026-05-12): AKS-RP will need **dual-provision** the token secret in BOTH `kube-system` AND new namespace of ama-metrics (e.g. `azure-monitoring-metrics`) during the migration window, and stop provisioning to `kube-system` only after migration completes.
- **token-adaptor (addon-token-adapter) image** for AKS Cluster — confirmed by AKS team to run in any namespace; no binary changes required. The image requires `NET_ADMIN` and `NET_RAW` Linux capabilities. **AKS standard verified by empirical test (ama-metrics + token-adaptor deployed to non-kube-system namespace, works as-is).** Two narrow cases still need verification: see [Appendix C: Capability/Pod-Security concern](#appendix-c-apabilitypod-security-concern) below.
- **token-adaptor (addon-token-adapter) image** for AKS Arc Cluster — token-adaptor has different image for AKS Cluster. Whehter it will be impacted by namespace migration needs to be investigated and tested.
- **retina** - it is a dependency used by ama-metrics for network observabiltiy. Now it runs inside kube-system, and whehter it will be impacted by namespace migration needs to be investigated and tested.
- **CCP-side configmap-name reference.** ccp runs Underlay, and it copies configmap from kube-system namespace from Overlay using [configmap-watcher](#https://msazure.visualstudio.com/CloudNativeCompute/_git/prometheus-extensions?path=%2Fconfigmap-watcher&_a=contents). When the customer-side configmap moves to `azure-monitoring-metrics`, AKS-RP's configmap-copy mechanism (Overlay → Underlay) and CCP's watcher must track the new name.



## Tasks
Tasks are categroized in following categories:
aks automatic - related to AKS Automatic cluster support
aks istio - related to AKS Istio
common - related to both above

**Phase 0 (Tasks 1, 2) is gating for Task 3 and downstream work.** No Phase 0 task may be skipped before starting the migration refactor.

### 1 (aks automatic) Investigate existing issues of ama-metrics in AKS automatic cluster and possible solutions except namespace migration
- Create an AKS Automatic cluster with ama-metrics enabled, investigate and document issues. For identified issues, explore solutions except namespace migration.
- **what has been done**
  - Verified user can create configmaps in kube-system namespace. This is one of major potential conerns that drives migration of ama-metrics outside kube-system namespace.
- If configmap creation in kube-system is a blocker. Explore and brainstorm alternative solutons, for examples:
  **AKS Automatic exceptions:** Can ama-metrics be added to AKS Automatic's allowlist of first-party services that *can* create resources in `kube-system`? (ama-logs, retina, kube-proxy, coredns, csi-* etc. all live in `kube-system` today — there is precedent.)
  **Cross-namespace ConfigMap watch:** Can ama-metrics remain in `kube-system` while watching ConfigMaps in customer namespaces, satisfying the AKS Automatic ConfigMap restriction without moving the pod?

- **Priority:** High (Phase 0 — gates Task 3)

### 2 (aks istio) Investigate existing issues of AKS cluster with Istio and possible solutions except namespace migration
- Create an AKS cluster with Istio and ama-metrics enabled, investigate and document all issues. For identified issues, explore solutions except namespace migration.
**what has been done** details are in [`./istio-investigation/REPORT.md`](./istio-investigation/REPORT.md)
  - **Headline finding:** ama-metrics running in `kube-system` (no sidecar, no mesh identity) successfully scrapes user-app metrics in an Istio-meshed namespace **even under STRICT mTLS**, by default. Istio's mutating webhook automatically rewrites the pod's `prometheus.io/port` and `prometheus.io/path` annotations to point at the merged-metrics endpoint `:15020/stats/prometheus`, which is `Captured: No` in Istio's port table — i.e. the sidecar does not intercept it and `PeerAuthentication` mTLS rules do not apply.
  - **Two customer-side failure modes** were identified, both fixable without product code changes:
    - **Failure (a)** — customer disables `ISTIO_META_ENABLE_PROMETHEUS_MERGE`: the merged endpoint returns Envoy stats only, app metrics are missing. Fix: re-enable merge, or use one of the Failure (b) port-level fixes to scrape the app port directly.
    - **Failure (b)** — customer's custom scrape config / `PodMonitor` / `ServiceMonitor` hardcodes the app port (e.g. `:8080`): under STRICT mTLS the scrape is rejected. Three verified fixes:
      - **Fix C (recommended):** rewrite the scrape config to target `:15020/stats/prometheus`.
      - **Fix A:** port-level PERMISSIVE `PeerAuthentication` for the app port.
      - **Fix B:** pod annotation `traffic.sidecar.istio.io/excludeInboundPorts: "<port>"`

- If metrics scraping under mTLS is still a blocker. Explore and brainstorm alternative solutons, for examples:
- **Istio whitelisting:** Can sidecar injection be enabled for `kube-system` via namespace label, OR can ama-metrics pods be excluded from mesh enforcement via annotation while still allowing customer pods in mesh? Does the customer goal require ama-metrics to be *in* the mesh, or just *not blocked by* the mesh?

- **Priority:** High (Phase 0 — gates Task 3).


### 3 (common) Refactor ama-metrics codebase (prometheus-collector repo) to support custom namespace
- Identify and implemente changes of ama-metrics so it can run in custom namespace. (see [Appendix A](#appendix-a-azure-managed-prometheus-code-changes) for details)
- Note this is only for ama-metrics agent that is from the prometheus-collector repo, and it does not include any other services, e.g. token-adaptor.
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
Similar as 4, we deploy ama-metrics in custom namespace to an AKS Automatic cluster through helm chart approach and investiate issues.
  - ama-metrics needs to scrap metrics of resources in kube-system. Since ama-metrics runs under non kube-system, we need to investigate whether metrics scrapping of resources in kube-system is impacted. This issue is commont to both AKS Automatic cluster and AKS Istio.
  - Verify that `system-node-critical` and `system-cluster-critical` priority classes are admitted in non-`kube-system` namespaces under AKS Automatic Deployment Safeguards. Vanilla Kubernetes and AKS standard do not restrict these, but AKS Automatic's mandatory Azure Policy / Gatekeeper add-on is the area of concern. If blocked, fallback options to evaluate: (a) custom PriorityClass owned by AKS-RP, or (b) request a policy carve-out for first-party agents.
  - Verify AKS Automatic's default NetworkPolicy posture. If a default-deny baseline is enforced, an explicit egress NetworkPolicy from `azure-monitoring-metrics` to `kube-system` will be required for ama-metrics to scrape kube-system targets (CoreDNS, kube-proxy, retina, etc.).
  - **AKS Automatic allowlist for `azure-monitoring-metrics`.** AKS Automatic enforces Deployment Safeguards and may restrict resource creation outside system namespaces; the new namespvace must be added to the first-party agent allowlist.

- **Priority:** High

### 6 (aks istio) Investigate issues of ama-metrics agent in custom namespace in AKS Cluster with Istio enabled
Similar as Task 4, we deploy ama-metrics in custom namespace to an AKS cluster with Istio enabled through helm chart approach and investiate issues. We will need to research different configurations of Istio for AKS and investigate Istio scenarios both ama-metrics can support and can't support when it runs in custom namespace.
Potentail issues:
- token-adaptor: Istio injects a sidecar that intercepts all traffic. ama-metrics currently relies on token-adaptor for auth — there could be conflicts between token-adaptor and the Istio-injected sidecar. IMDS at `169.254.169.254` likely needs `traffic.sidecar.istio.io/excludeOutboundIPRanges: 169.254.169.254/32`.
- ablity to scrap metrics of resources in kube-system: 
  Mesh-to-non-mesh egress to `kube-system` scrape targets. Mesh-to-non-mesh egress: ama-metrics scraping `kube-system` targets (CoreDNS, kube-proxy, kube-state-metrics, node-exporter, kubelet, cAdvisor, kube-apiserver, kappie, retina, hubble, cilium, ACStor, local CSI driver) requires a per-target carve-out strategy because `kube-system` pods have no sidecar to terminate mTLS. **Mitigation strategy is fully characterized in [Cross-namespace concerns / Layer 3](#cross-namespace-concerns-after-migration)** — DestinationRule `tls.mode: DISABLE` for service-FQDN targets, `traffic.sidecar.istio.io/excludeOutboundPorts` for pod-IP / node-IP / hostNetwork targets. See [](TOD:)
- ablity to call api server
  When ama-metrics agent runs outside `kube-system` namespace and its communication is managed by Istio, the agent may have permission issues when making calls to services in `kube-system` (e.g., Kubernetes API server) because Istio can't inject sidecar inside `kube-system`.

- **Priority:** TBD.


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



## Open Questions and Discussions

2. **Customer ConfigMap migration semantics (Task 8)** — Design deferred. Rollout blocker for Task 3. Options to reconsider: hard cutover (unacceptable), dual-watch with deprecation, auto-migrate at startup, cross-namespace watch.

3. **Hybrid migration model** — Parked as fallback. If uniform migration proves too risky during rollout planning, hybrid (per-cluster-type namespace selection) can be revisited.

4. **Rollout / phased deployment / toggle strategy** — Not yet designed.

5. **Validation / success criteria for Tasks 4–6** — Not yet specified.


### Migration mode
If Task 1 & 2 returns a "go for migration" verdict, **all customers** (every AKS cluster type in scope) get migrated to the custom namespace — uniform migration, not hybrid. Hybrid (per-cluster-type namespace selection) is parked as a fallback option to revisit if uniform migration proves too risky during rollout planning.


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


### Appendix C: Capability/Pod-Security concern
addon-token-adapter requires `NET_ADMIN` and `NET_RAW` Linux capabilities (explicitly added via `securityContext.capabilities.add` in its pod spec). Pod Security Standards (PSS) `baseline` and `restricted` profiles forbid adding `NET_ADMIN`. However, **PSS enforcement is opt-in per namespace via the `pod-security.kubernetes.io/enforce` label** — it is not on by default in standard AKS clusters. Concern narrows to two specific cases (Task 1 / Task 2 sub-deliverables):

1. **AKS Automatic** — Automatic clusters have Deployment Safeguards enabled by default. Verify whether `azure-monitoring-metrics` (when created by AKS-RP) inherits a `baseline`-or-stricter PSS label, and whether AKS-RP has an exemption mechanism for first-party namespaces (e.g., labeling with `pod-security.kubernetes.io/enforce: privileged`). If the new namespace inherits baseline enforcement without an exemption, token-adaptor pods fail admission.
2. **Customers running policy engines (Gatekeeper, Kyverno, or explicit PSA labels)** — Per AKS team: "if cx has some additional in-cluster policy configured to block the permission, they'll need to whitelist the new namespace." Customer-facing migration documentation must call this out.

**AKS standard is NOT a concern** — confirmed by empirical test on a deployed cluster.


### Appendix D: Production namespace name
The production namespace needs to be determined. For example, `azure-monitoring-metrics` (single fixed name, not customer-configurable). The single source of truth is the helm `values.yaml` `namespace:` field in the prometheus-collector repo. AKS-RP must match the value set here. Runtime centralization already exists via the `POD_NAMESPACE` downward-API env var (`shared/namespace.go`) and `$$POD_NAMESPACE$$` substitution in default scrape configs — minimal Go changes are needed beyond the helm parameterization. Parameterization in helm exists for dev/test only; production deployments use the fixed name.

AKS team must be asked to allowlist this specific name.


## Appendix E: Cross-namespace concerns when ama-metrics runs outside kube-system

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