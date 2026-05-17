# AKS Automatic + MSNP — Solution options for the customer configmap surface

> **Status:** brainstorm / decision-pending. Not committed direction.
> **Companion docs:**
> - [`aks-automatic-msnp-kube-system-findings.md`](./aks-automatic-msnp-kube-system-findings.md) — the verified findings about *why* the existing flow is broken.
> - [`aks-automatic-kube-system-configmap-findings.md`](./aks-automatic-kube-system-configmap-findings.md) — the prior investigation on classic AKS Automatic (no MSNP).
>
> This doc enumerates *possible* solutions only. No implementation has been agreed.

---

## 1. The problem in one paragraph

`prometheus-collector` ships 4 customer-facing configmaps in `otelcollector/configmaps/`:

| File(s) | ConfigMap (always in `kube-system`) | What customers use it for |
|---|---|---|
| `ama-metrics-settings-configmap{,-v1,-v2,-otel}.yaml` | `ama-metrics-settings-configmap` | Toggle default scrape targets, set `cluster_alias`, configure HTTPS, label whitelisting, debug mode, schema-version selection, etc. |
| `ama-metrics-prometheus-config-configmap.yaml` | `ama-metrics-prometheus-config` | Custom Prometheus scrape configs for the **cluster-scope** Deployment replica |
| `ama-metrics-prometheus-config-node-configmap.yaml` | `ama-metrics-prometheus-config-node` | Custom Prometheus scrape configs for the **per-node Linux DaemonSet** |
| `ama-metrics-prometheus-config-node-windows-configmap.yaml` | `ama-metrics-prometheus-config-node-windows` | Same, **Windows DaemonSet** |

On AKS Automatic + MSNP, **`kubectl apply` of any of these is denied at admission** by `aks-managed-protect-system-namespaces` (see findings doc §3 for the full deny pipeline). None of them ship as defaults — they're only materialized when the customer opts to override — so the practical effect is **zero customer-side knobs on MSNP today**.

The customer's identity (Entra ID + `Azure Kubernetes Service RBAC Cluster Admin`) is *not* exempt from the VAP. There is no escape hatch via "create your own SA in kube-system and run as it" because every dependency of that escape (Pod, ServiceAccount, RoleBinding in kube-system) is itself blocked by the same VAP.

So we need a different path. This doc lists 5 candidate paths.

---

## 2. Solution options (ranked by my current preference)

> **Note on the volume mount inside ama-metrics pods**
>
> Today the 4 customer configmaps are surfaced to the agent via a **configmap volume mount** on the `ama-metrics` Deployment, the 7 sized `ama-metrics-node-*` DaemonSets, and `ama-metrics-ksm` / `ama-metrics-operator-targets`. The agent reads them off disk (`/etc/config/settings/*`, `/etc/config/prom-config/prometheus-config.yml`, …).
>
> Most options below have **two viable sub-variants** — one that keeps the volume mount and one that removes it. They behave the same from a customer's perspective but have very different blast radius on the agent's deployment specs and on API-server load:
>
> | Sub-variant | What changes inside the agent | Trade-off |
> |---|---|---|
> | **a — operator round-trips through a CM in `kube-system`** | The operator (SA in `kube-system`, exempt from the VAP) writes/mirrors the rendered configmap into `kube-system`. Existing volume mount + on-disk reader stay unchanged. | Minimal delta to the Deployment + 7 DaemonSets; the `kube-system` CM becomes an **internal implementation detail**, no longer the customer-facing surface. |
> | **b — agent reads via the K8s API directly** | Each agent pod opens a `Watch` on the CRD / CM in the configured ns. Volume mount + on-disk reader removed. | Cleaner conceptually, but adds N watches per cluster (1 per DaemonSet pod) and changes the agent's startup sequence. |
>
> **Default assumption in this doc: sub-variant (a) — keep the volume mount.** It's the safer production landing for both Option 1 and Option 3, because it leaves the per-node DaemonSet hot path untouched. Sub-variant (b) is called out explicitly where it's worth considering.

### Option 1 — New CRD surface in customer namespaces *(preferred — Kubernetes-native)*

Ship 1–4 CRDs (initial naming TBD; e.g. `monitoring.azure.com/v1`) that customers `kubectl apply` to **their own namespace** (or any namespace not in the VAP's protected list). `ama-metrics-operator-targets` already runs as a Kubernetes operator that watches CRDs cluster-wide — extend it to watch the new ones and translate them into the in-memory equivalent of the configmaps.

**Data flow (default sub-variant 1a — keeps the volume mount):**

```
customer ── kubectl apply ──▶ AmaMetricsSettings CR  (customer's own ns, e.g. my-team-config)
                                       │
                                       │  watch
                                       ▼
                  ama-metrics-operator-targets  (SA in kube-system, exempt from VAP)
                                       │
                                       │  render + write
                                       ▼
                  ConfigMap/ama-metrics-settings-configmap  (kube-system)
                                       │
                                       │  volume mount (unchanged)
                                       ▼
                  ama-metrics pods (Deployment + 7 DaemonSets)
                                       │
                                       │  on-disk reader (unchanged)
                                       ▼
                                  agent runtime
```

In this variant the `kube-system` configmap **still exists and is still mounted**, but it becomes an *internal implementation detail* — the customer never touches it. The VAP doesn't fire because the writer is `ama-metrics-operator-targets` (a kube-system SA, exempt at Gate 4).

Sub-variant **1b** would skip the round-trip through the CM and have the operator push rendered config into each agent pod directly (or have each agent open its own CRD watch). That removes the volume mount but materially changes the cluster-scope Deployment and every DaemonSet pod's startup path. Probably not worth it given the small savings.

Sketch:

```yaml
# Customer applies this in their own namespace, e.g. "my-team-config"
apiVersion: monitoring.azure.com/v1
kind: AmaMetricsSettings
metadata:
  name: cluster-default
  namespace: my-team-config
spec:
  schemaVersion: v2
  clusterAlias: prod-eu1
  defaultTargetsScrapeEnabled:
    kubelet: true
    coredns: true
    cadvisor: true
    apiserver: false
```

| Pros | Cons |
|---|---|
| ✅ No AKS RP work needed — fully owned by prometheus-collector team | ❌ New CRDs to design, version, document, maintain |
| ✅ Sidesteps the VAP entirely (CRDs land in customer namespaces) | ❌ Multi-tenant edge cases ("which CRD wins if a customer has two `AmaMetricsSettings`?") |
| ✅ Customer keeps `kubectl apply` muscle memory | ❌ `ama-metrics-operator-targets` needs a new watch loop + reconcile logic |
| ✅ Works identically on MSNP and non-MSNP clusters — single codepath | ❌ Migration story for existing customers ("rewrite your configmap as a CRD") |
| ✅ Composes naturally with existing `PodMonitor`/`ServiceMonitor` | |

**Open design questions:**
- One omnibus CRD or 4 specialized ones (one per existing configmap)?
- Cluster-scoped or namespaced? Namespaced is friendlier for RBAC, but then "which namespace's CRD wins?" needs an answer (label selector, alphabetical, named on cluster, …).
- How do we preserve schema-version v1 / v2 distinction the existing configmap parser carries?

### Option 2 — Extend ARM `azureMonitorProfile.metrics` → AKS RP renders configmap *(Azure-native)*

Extend the AKS managed-cluster ARM contract to expose every key these 4 configmaps cover. The AKS RP (which runs as `aksService` / `hcpService`, both exempt at Gate 4 of the VAP) renders the configmap on the customer's behalf.

Existing state: `azureMonitorProfile.metrics` already covers some `ama-metrics-settings-configmap` keys today (interval, label-name strategy, KSM defaults). The gap is the *custom Prometheus config* — `ama-metrics-prometheus-config` and the two node variants — which currently has no ARM equivalent.

| Pros | Cons |
|---|---|
| ✅ Pure Azure-native UX — Portal, Bicep, Terraform, ARM all work | ❌ Requires AKS RP changes — coordination with the AKS team |
| ✅ Works identically on MSNP and non-MSNP clusters | ❌ Slow to ship (ARM contract review, GA process, multi-team sign-off) |
| ✅ Auditable via Azure Activity Log | ❌ Loses Kubernetes-native ergonomics (no `kubectl diff`, no GitOps in K8s flow) |
| ✅ Already the documented "recommended" path on classic AKS Automatic | ❌ Hard to express raw Prometheus YAML (multi-line scrape configs) cleanly inside ARM (escaping, validation, length limits) |

**Open design questions:**
- Where exactly do the new properties live? `azureMonitorProfile.metrics.customScrapeConfigs`? A separate `prometheusConfig` block? A reference to a separate ARM resource (e.g. `Microsoft.Monitor/dataCollectionRules`)?
- How do we surface the per-node-vs-cluster-scope distinction in ARM, given customers don't have a strong mental model of the agent's deployment topology?

### Option 3 — Configmap moves to a customer-owned namespace *(smallest agent change)*

Customer creates the same 4 configmaps (same names, same schema) but in a **customer-owned namespace** (e.g. `my-team-config`) instead of `kube-system`. Crucially, a pod can only volume-mount configmaps from **its own namespace** (Kubernetes restriction), so ama-metrics — which runs in `kube-system` — cannot mount the customer's CM directly. That leaves two sub-variants:

**Sub-variant 3a (preferred — keeps the volume mount):**

```
customer ── kubectl apply ──▶ ConfigMap/ama-metrics-settings-configmap  (my-team-config)
                                       │
                                       │  watch
                                       ▼
                  ama-metrics-operator-targets  (SA in kube-system, exempt from VAP)
                                       │
                                       │  mirror (copy bytes 1:1)
                                       ▼
                  ConfigMap/ama-metrics-settings-configmap  (kube-system)
                                       │
                                       │  volume mount (unchanged)
                                       ▼
                  ama-metrics pods (Deployment + 7 DaemonSets)
```

This is functionally close to Option 1a, except the customer's source-of-truth is still a configmap (same shape they're documented to use) instead of a CRD.

**Sub-variant 3b (no operator, but no volume mount):** Change the agent's config-reader to look up the 4 configmaps in a configurable namespace (env var, addon param, etc.), and have each agent pod open a `Watch` on those CMs via the K8s API. The `kube-system` CM and its volume mount disappear, but every DaemonSet pod now holds a long-lived API watch — adds N watches per cluster.

| Pros | Cons |
|---|---|
| ✅ Smallest code delta — reuses existing TOML/configmap parser | ❌ Customer has to remember a non-standard namespace name |
| ✅ Customer keeps the *exact* configmap shape they're documented to use | ❌ Plumbing question: where does "which ns?" live? (env var on agent, addon param, ARM property, label on namespace) |
| ✅ Multi-tenant works naturally — pick the namespace per cluster | ❌ Documentation churn — every existing customer doc says `kube-system` |
| ✅ Backward-compatible: `kube-system` keeps working on non-MSNP clusters | ❌ The `aks-managed-protect-system-namespaces` VAP doesn't help us here, since we're trying to *avoid* it — risk of customer accidentally picking a protected namespace |

**Open design questions:**
- How does the agent discover the namespace at startup? (env var probably the simplest.)
- Race conditions during cluster-create — what happens before the customer creates their CM?

### Option 4 — Workaround only: hybrid CRDs + accept the gap *(today, no code change)*

Tell customers on MSNP:

- For **custom scrape targets** → use existing `PodMonitor` / `ServiceMonitor` CRDs (already work cluster-wide; not blocked by the VAP because they live in customer namespaces). Covers most `ama-metrics-prometheus-config*` use cases.
- For **`ama-metrics-settings-configmap`** → use whatever ARM `azureMonitorProfile.metrics` already exposes. Document the gaps explicitly.
- For everything else (cluster_alias, HTTPS settings, label whitelisting via configmap, debug mode, schema-version pinning, etc.) → **not supported on MSNP**.

| Pros | Cons |
|---|---|
| ✅ Zero code change — ships today | ❌ Genuine functional regression vs. classic AKS Automatic |
| ✅ Aligns with the prometheus-operator ecosystem | ❌ Bad doc story — "this configmap exists but you can't use it on MSNP" |
| | ❌ Customer-visible breaking change for anyone migrating to MSNP |

### Option 5 — Petition AKS to add per-CM exemption to the VAP *(unlikely to land)*

Ask the AKS team to add an exemption to `aks-managed-protect-system-namespaces` for these specific CMs by name — e.g., a `matchConditions` carve-out: *"request is allowed if the resource is one of `ama-metrics-settings-configmap`, `ama-metrics-prometheus-config`, `ama-metrics-prometheus-config-node`, `ama-metrics-prometheus-config-node-windows` in `kube-system`, AND the operation is CREATE or UPDATE."*

| Pros | Cons |
|---|---|
| ✅ Customer's existing `kubectl apply` workflow works unchanged | ❌ Almost certainly rejected — the entire point of MSNP is to lock down `kube-system` |
| ✅ Zero agent code change | ❌ Sets a precedent — every other AKS-managed addon will ask for the same |
| | ❌ Brittle: hard-coded by name; breaks if we ever rename a CM |
| | ❌ Doesn't generalize — every new customer-facing CM requires an AKS-side change |

---

## 3. Comparison at a glance

| Option | Code lives in | Customer UX | MSNP-compatible | Estimated effort | Customer migration cost |
|---|---|---|---|---|---|
| 1 — CRDs in customer ns | prometheus-collector (operator-targets) | `kubectl apply` of new CRDs | ✅ | Medium (CRD design + watcher) | Medium (rewrite CMs as CRDs) |
| 2 — ARM `azureMonitorProfile.metrics` | AKS RP | Azure Portal / Bicep / Terraform | ✅ | High (multi-team, ARM contract) | Low (declarative IaC users), High (kubectl users) |
| 3 — CM in customer-owned ns | prometheus-collector (config reader) | `kubectl apply` of CMs to a non-system ns | ✅ | Small | Small (rename namespace in CM) |
| 4 — Document the gap | none | Use PodMonitor/ServiceMonitor + ARM, accept missing knobs | ✅ partial | Zero | n/a — features just don't work |
| 5 — VAP exemption | AKS team | Unchanged | ✅ if accepted | Zero (us), unknown (AKS) | Zero |

---

## 4. Recommendation (current thinking)

**Pursue Options 1 and 2 in parallel, ship Option 4 as the interim story.**

- **Option 1 (CRD)** is fully in our team's control. Could land in 1–2 sprints. Becomes the official MSNP path. Works on every cluster (MSNP or not), so no version-skew gating.
- **Option 2 (ARM)** is what Azure customers will eventually expect. Drive it as a longer-term ask with the AKS team and the Azure Monitor PG. Position it as the "Azure-native" path; the CRD remains the "Kubernetes-native" path.
- **Option 4 (document the gap)** is the holding pattern — ship the docs immediately so customers piloting MSNP know what they're getting into.
- **Option 3 (CM in customer ns)** is on the table as a small-delta alternative to Option 1, but loses to Option 1 on long-term cleanliness (CRDs > magic-namespace configmaps for an Azure-managed product).
- **Option 5 (VAP exemption)** — non-starter; do not pursue.

---

## 5. Decisions still needed

Before any of this can move, the team needs to agree on:

1. **Which option(s) we pursue.** Single-track or parallel?
2. **CRD scope (if Option 1).** One omnibus `AmaMetricsConfig`, or 4 specialized CRDs matching the existing CMs?
3. **CRD ownership/conflict model (if Option 1).** Cluster-scoped, or namespaced with a "named on cluster" winner-selection mechanism?
4. **ARM property shape (if Option 2).** Embed YAML strings, reference a DCR resource, or model fields explicitly?
5. **Back-compat policy.** How long do we keep `kube-system` configmaps as a supported input on non-MSNP clusters?
6. **Doc plan.** Who owns the customer-facing migration guide? (ours, AKS docs, Azure Monitor docs?)

---

## 6. Out of scope for this doc

- Diagnosis of *why* `kubectl apply` is denied — see findings doc §3.
- The doc-discrepancy about ama-metrics Deployments running on `system-surge` instead of `hostedpool` — see findings doc §6.1.
- Workarounds for `kubectl exec`-based debugging — that's a separate problem (different VAP, `aks-managed-protect-interactive-access`).

---

## 7. Open follow-ups

- [ ] Audit `shared/configmap/mp/` for the complete key surface each of the 4 configmaps exposes — needed for a precise gap analysis between Options 1, 2, and 3.
- [ ] Survey existing customer support tickets / GitHub issues for which configmap keys are most-used — informs MVP scope for Option 1.
- [ ] Reach out to the AKS team (MSNP owners) to (a) socialize the problem, (b) gauge appetite for Option 2.
- [ ] Open a tracking issue on the prometheus-collector repo once a direction is picked.
