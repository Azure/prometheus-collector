# What VAP modification should we ask AKS for? — Variants A / B / C tested on `zane-auto-2`

> **Date:** 2026-05-19
> **Tested on:** `zane-auto-2` (RG `zane-auto-2`, sub `9c17527c-af8f-4148-8019-27bada0845f7`) — classic AKS Automatic, no MSNP, VAPB normally in `[Audit]`, `(automatic-authz)` lock not active. This is the only cluster where the experiment is possible: the VAP exists, the binding can be flipped to `[Deny]` to simulate MSNP, and the customer is still allowed to patch the VAP itself (which they can't on real MSNP).
> **Identity used in tests:** `zanejohnson@microsoft.com` (subscription `Owner` + `Azure Kubernetes Service RBAC Cluster Admin` on the cluster)
> **Related docs:**
> - [`aks-automatic-msnp-kube-system-findings.md`](./aks-automatic-msnp-kube-system-findings.md) — primary findings on the VAP + `(automatic-authz)` lock
> - [`aks-automatic-msnp-configmap-solution-options.md`](./aks-automatic-msnp-configmap-solution-options.md) — broader solution space (CRDs, ARM property, customer-ns CM, etc.) — Option 5 in that doc is the AKS-side ask this doc nails down

---

## TL;DR

**The ask to bring to AKS is to add one new `matchCondition` to the `ValidatingAdmissionPolicy` named `aks-managed-protect-system-namespaces`.** That single CEL clause exempts **only the 5 objects that are *structurally forced* into `kube-system`** — 4 ConfigMaps (the addon's customization surface) + 1 mTLS Secret (the shared cert bundle for the **ConfigMap scrape-config path**, mounted into the ama-metrics pods themselves). Everything else customers apply for managed Prometheus (PodMonitor / ServiceMonitor CRs and **their own** per-target credential Secrets — basic-auth, TLS, bearer-token, OAuth2 — plus supporting RBAC) is steered by our public docs into **the customer's own namespace**, where the VAP doesn't fire and no allowlist is needed.

> **Key finding driving this scoping:** The PodMonitor/ServiceMonitor CR's namespace is the **anchor** that determines where its per-target credential Secrets (basic-auth, TLS, bearer, OAuth2), the Role, and the RoleBinding must live — they're all chained to it by the CRD schema (every credential field is a `SecretKeySelector` with no `namespace:`) and by RBAC scoping. If the customer puts the Monitor outside `kube-system` (the documented default), the whole chain stays outside `kube-system`. See [§7.5](#75-the-monitors-namespace-anchors-everything) for the cascade map and [§7.6](#76-two-strategic-options-considered) for the Option 1 vs Option 2 trade-off behind the scoping.

The drop-in YAML:

```yaml
# In spec.matchConditions[], appended as the third entry (evaluated AND with the
# existing two: apply-to-non-exempt-users, apply-to-non-exempt-groups).
- name: exempt-ama-metrics-customer-resources
  expression: |
    !(request.namespace == "kube-system" && (
      (request.resource.group == "" && request.resource.resource == "configmaps" &&
       request.name in ["ama-metrics-prometheus-config",
                        "ama-metrics-settings-configmap",
                        "ama-metrics-prometheus-config-node",
                        "ama-metrics-prometheus-config-node-windows"]) ||
      (request.resource.group == "" && request.resource.resource == "secrets" &&
       request.name == "ama-metrics-mtls-secret")
    ))
```

Two alternative VAP-shape strategies were tested experimentally and rejected:

| Variant | Shape | Why rejected |
|---|---|---|
| **A — name carve-out** | New `matchCondition` exempts only 4 named CMs of resource `configmaps` in `kube-system` | **Surgical. Recommended.** |
| **B — identity exemption** | Add `zanejohnson@microsoft.com` to the existing `userInfo.username` exempt list in `matchConditions[0]` | Too broad — exempts the *user* across **all** resources in **all** 20 protected namespaces; also wrong axis (customer identity isn't constant) |
| **C — namespace carve-out** | Drop `kube-system` from `namespaceSelector.matchExpressions[0].values` | Too coarse — opens *all* resources in `kube-system` (Secrets, RBAC, …), not just the 4 ConfigMaps |

The webhook layer (`(automatic-authz)`) does **not** need to change. Its job is to keep customers from tampering with the VAP — Variant A doesn't ask it for anything.

---

## 1. Why this doc exists

The companion findings doc proves the customer cannot write to `kube-system` on MSNP (VAP enforcement) and cannot escape by editing the VAP (`(automatic-authz)` webhook). The solution-options doc enumerates broad paths forward (CRDs, ARM property, customer-namespace CM, agent-side workarounds, "raise with AKS"). The "raise with AKS" path was the only customer-visible path that requires AKS to do anything — but until now we had **no concrete ask** to bring to that conversation. This doc fixes that gap by:

1. Defining three plausible exemption shapes (Variants A, B, C),
2. Reproducing MSNP-style `[Deny]` mode on `zane-auto-2` and applying each variant,
3. Verifying with positive and negative test cases what each variant actually allows / blocks,
4. Picking the variant with the narrowest blast radius (Variant A),
5. Recording the exact CEL clause and the test matrix so anyone (us or AKS) can re-run the experiment and confirm.

It is **not** a doc about whether AKS will accept the ask — that's a separate conversation about MSNP policy. It only nails down *what to ask for*.

---

## 2. Experimental setup

### Why `zane-auto-2` is the right cluster for this test

| Property | `zane-auto` | `zane-auto-2` | `zane-auto-msnp` |
|---|---|---|---|
| VAP `aks-managed-protect-system-namespaces` present | ✅ | ✅ | ✅ |
| VAPB normally in… | `[Audit]` | `[Audit]` | `[Deny]` |
| Can the customer flip VAPB to `[Deny]`? | ✅ yes | ✅ yes | ❌ no (`automatic-authz` lock) |
| Can the customer patch the VAP itself? | ✅ yes | ✅ yes | ❌ no (`automatic-authz` lock) |

→ `zane-auto-2` (or `zane-auto`) is the only cluster where we can both simulate MSNP enforcement (by flipping the binding to `[Deny]`) **and** apply different exemption shapes to the VAP. On `zane-auto-msnp` itself we can do neither.

### The four test cases used for every variant

For each variant we ran the same four `kubectl apply -f`s to characterize what the modified policy actually allows:

| # | Test | What it tells us |
|---|---|---|
| 1 | Create CM `ama-metrics-settings-configmap` in `kube-system` | **Positive:** did our intended exemption land? |
| 2 | Create CM `zane-not-ama-metrics` in `kube-system` | **Negative:** are non-ama-metrics CMs in the same namespace still protected? |
| 3 | Create CM `ama-metrics-settings-configmap` in `gatekeeper-system` | **Negative:** is the exemption scoped to `kube-system` only? |
| 4 | Create Secret `zane-test-secret` in `kube-system` | **Negative:** is the exemption scoped to ConfigMaps only? |

A "good" variant: Test 1 succeeds, Tests 2/3/4 all fail. Anything else means the variant is broader than intended.

### Reproduction baseline (Stage 1 of every test): flip VAPB to `[Deny]`

Before each variant, `validationActions` is patched from `[Audit]` to `[Deny]`. As an independent sanity check, with the VAP unmodified and the binding in `[Deny]`, a normal customer `kubectl create cm zane-experiment-cm -n kube-system` produces:

```
Error from server (Forbidden): … configmaps "zane-experiment-cm" is forbidden:
ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding
'aks-managed-protect-system-namespaces-binding' denied request:
Modification of resources in managed system namespaces is not allowed
```

That error message is **byte-identical** to what `zane-auto-msnp` (real MSNP) produces. Confirms the VAP+VAPB combination is the entire customer-visible enforcement mechanism — there's no MSNP-specific magic beyond `[Deny]` (and the `(automatic-authz)` lock on top, which only affects whether the customer can disarm the VAP).

---

## 3. Variant A — name-based resource carve-out (the recommended ask)

### Modification

Append a third `matchCondition` to `spec.matchConditions[]`:

```yaml
- name: exempt-ama-metrics-configmaps
  expression: |
    !(request.namespace == "kube-system" &&
      request.resource.resource == "configmaps" &&
      request.name in ["ama-metrics-prometheus-config",
                       "ama-metrics-settings-configmap",
                       "ama-metrics-prometheus-config-node",
                       "ama-metrics-prometheus-config-node-windows"])
```

The expression is a negation of the "this is one of the protected ama-metrics CMs" check. Returning `false` makes the policy **skip** this request (per Kubernetes' `matchConditions` semantics: if any returns `false`, the policy does not fire). Returning `true` lets evaluation continue into the `validations` block, which then denies.

### Why this shape (and not a friendlier-looking CEL):

- `request.namespace`, `request.resource.resource`, `request.name` are all stable fields of the K8s `AdmissionRequest` exposed in CEL. Using a hardcoded `in [...]` list is the most boring, least-clever expression possible — easier for AKS reviewers to accept.
- The list of four names is the complete set documented in [`otelcollector/configmaps/`](../../../otelcollector/configmaps/). If we ever add a fifth CM, the AKS ask needs to be re-issued.

### Results

| Test | Expected | Actual |
|---|---|---|
| 1. Create `ama-metrics-settings-configmap` in `kube-system` | succeed | ✅ `configmap/ama-metrics-settings-configmap created` |
| 2. Create `zane-not-ama-metrics` in `kube-system` | **fail** | ✅ `Forbidden … denied request: Modification of resources in managed system namespaces is not allowed` |
| 3. Create CM `ama-metrics-settings-configmap` in `gatekeeper-system` | **fail** | ✅ `Forbidden …` |
| 4. Create Secret `ama-metrics-settings-configmap` in `kube-system` | **fail** | ✅ `Forbidden …` |

All four match expectations. The exemption is precisely shaped.

### Pros
- **Minimum possible blast radius.** Exactly 4 CMs, 1 namespace, 1 resource type. Nothing else changes.
- **No customer-identity assumption.** Works for any caller — Azure AAD users, customer service accounts, anyone with the right RBAC role.
- **Survives a future "ama-metrics agent identity changes" event.** Customer-shipped tooling (e.g. `kubectl apply` from CI, Helm install of an upstream chart, ArgoCD sync, etc.) all benefit equally.
- **Reviewable.** A reviewer can read the 4 names, look up what each is for, and decide. No "trust us" axis.

### Cons / open questions
- The list of names becomes a contract between us and AKS. Each new ama-metrics object (or rename) the public docs add to `kube-system` is a new round of "petition AKS" and a new release.
- This only carves out **objects we already know about**. If a customer wants to *delete* one of these (e.g. roll back), they'd be exempt too — that's almost certainly fine but worth flagging to the AKS reviewer.
- Doesn't help with ama-metrics needs in any other protected namespace (we're currently only writing to `kube-system`, so n/a today).
- The 4-test matrix above was run against the **ConfigMap branch only**. The final ask in [§7](#7-the-exact-ask-to-take-to-aks) extends the same `(group, resource, name)` shape to one additional triple (the mTLS Secret) — structurally identical, another `||` branch of the same CEL idiom — but that branch is not empirically re-validated here. We can re-run the matrix on `zane-auto-2` if AKS asks.

---

## 4. Variant B — identity-based exemption (rejected: too broad, wrong axis)

### Modification

Append `'zanejohnson@microsoft.com'` to the existing `userInfo.username` exempt list in `spec.matchConditions[0]` (the `apply-to-non-exempt-users` clause).

This is a strawman: in production the username would be the customer's, not a hardcoded `zanejohnson@`. We tested it only to characterize the blast radius of an identity-based exemption.

### Results (after a clean reset to canonical first)

| Test | Expected | Actual | Verdict |
|---|---|---|---|
| 1. Create `ama-metrics-settings-configmap` in `kube-system` | succeed | ✅ created | ok |
| 2. Create `zane-not-ama-metrics` in `kube-system` | **fail** | ⚠️ ALSO created | too broad |
| 3. Create CM in `azuresecuritylinuxagent` (other protected ns) | **fail** | ⚠️ ALSO created | too broad |
| 4. Create Secret in `kube-system` | **fail** | ⚠️ ALSO created | too broad |

### Verdict
**Rejected.** This exempts the user across **all 20 protected namespaces** and **every resource type**, not just the 4 ama-metrics CMs. Worse, it's the wrong axis entirely — the customer's identity isn't a fixed string we can put in the policy. Even if AKS were willing to accept identity exemptions, what would the customer's identity be? Their AAD UPN? Their cluster admin role assignment? A new service principal? None of these are durable, named-constants we can hardcode.

---

## 5. Variant C — namespace-level carve-out (rejected: too coarse)

### Modification

Remove `"kube-system"` from `spec.matchConstraints.namespaceSelector.matchExpressions[0].values`. The list shrinks from 20 to 19 protected namespaces.

### Results (after a clean reset to canonical first)

| Test | Expected | Actual | Verdict |
|---|---|---|---|
| 1. Create `ama-metrics-settings-configmap` in `kube-system` | succeed | ✅ created | ok |
| 2. Create `zane-not-ama-metrics` in `kube-system` | **fail** | ⚠️ ALSO created | too broad |
| 3. Create CM in `gatekeeper-system` | **fail** | ✅ correctly denied | ok |
| 4. Create Secret in `kube-system` | **fail** | ⚠️ ALSO created | too broad |

### Verdict
**Rejected.** This correctly preserves protection for the other 19 namespaces (test 3) but opens up **everything** in `kube-system` — Secrets (including service account tokens, TLS bundles), RBAC bindings, DaemonSets, you name it. Most of the value of MSNP comes from `kube-system` being read-only-ish for customers, so this is a regression AKS would never accept.

---

## 6. Comparison matrix

|                                  | Variant A | Variant B | Variant C |
|----------------------------------|:---:|:---:|:---:|
| Exempts the 4 ama-metrics CMs   | ✅ | ✅ | ✅ |
| Blocks unrelated CMs in `kube-system` | ✅ | ❌ | ❌ |
| Blocks unrelated resource types in `kube-system` (Secrets, RBAC, etc.) | ✅ | ❌ | ❌ |
| Blocks the same name in other protected namespaces | ✅ | ❌ | ✅ |
| Doesn't depend on a hardcoded user identity | ✅ | ❌ | ✅ |
| Reviewable in isolation (a single CEL clause)   | ✅ | ✅ | ✅ |
| **Recommendation** | **Ask** | Reject | Reject |

---

## 7. The exact ask to take to AKS

> Could you please add the following entry to the `spec.matchConditions[]` array of the `ValidatingAdmissionPolicy` named `aks-managed-protect-system-namespaces`?
>
> ```yaml
> - name: exempt-ama-metrics-customer-resources
>   expression: |
>     !(request.namespace == "kube-system" && (
>       (request.resource.group == "" && request.resource.resource == "configmaps" &&
>        request.name in ["ama-metrics-prometheus-config",
>                         "ama-metrics-settings-configmap",
>                         "ama-metrics-prometheus-config-node",
>                         "ama-metrics-prometheus-config-node-windows"]) ||
>       (request.resource.group == "" && request.resource.resource == "secrets" &&
>        request.name == "ama-metrics-mtls-secret")
>     ))
> ```
>
> This carves out the **5** specific objects that are *structurally forced* into `kube-system`:
>
> - **4 ConfigMaps** — `ama-metrics-settings-configmap` (settings) + the three custom-scrape-config variants (`-prometheus-config` for replica, `-prometheus-config-node` for Linux daemonset, `-prometheus-config-node-windows` for Windows daemonset).
> - **1 Secret** — `ama-metrics-mtls-secret`. This is the shared TLS cert bundle used by the **ConfigMap scrape-config path** (where customers write raw Prometheus scrape config and reference cert file paths under `/etc/prometheus/certs/...`). It is *mounted as a volume* on every ama-metrics scraper pod (daemonset, deployment, target allocator) via `secretName: ama-metrics-mtls-secret` baked into the addon's pod spec, and a Secret-volume can only mount Secrets from the same namespace as the pod. Since the pods live in `kube-system`, the Secret must too. Docs: [TLS-based scraping](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#tls-based-scraping).
>
> Note: the **PodMonitor / ServiceMonitor path does *not* use `ama-metrics-mtls-secret`.** Its CRD `tlsConfig` schema has no file-path fields (`caFile` / `certFile` / `keyFile` were removed upstream years ago); it only accepts `ca.{configMap,secret}` / `cert.{configMap,secret}` / `keySecret`, which are all `SecretKeySelector`s with no `namespace:` field — so they resolve to the Monitor's own namespace. That's why CRD-based TLS doesn't need a `kube-system` allowlist entry either; it follows the same anchor as basic-auth (see [§7.5](#75-the-monitors-namespace-anchors-everything)).
>
> All other writes to `kube-system` (other ConfigMaps, other Secrets, RBAC, workloads, etc.) and all writes to the other 19 protected namespaces remain blocked. We verified this on a fresh `zane-auto-2` cluster (classic AKS Automatic, VAPB flipped to `[Deny]` to simulate MSNP behavior) with a 4-case test matrix on the ConfigMap branch — see [Appendix A](#appendix-a-full-reproduction). The mTLS Secret branch is the same CEL idiom extended with one additional `(group, resource, name)` triple (see [§7.5](#75-the-monitors-namespace-anchors-everything) for why this is the complete list).
>
> No change is required to the `(automatic-authz)` authorization webhook, the binding, or any other AKS-managed resource.

### Pre-emptive answers to likely AKS questions

- **"Why only 5 names and not more?"** Customer-facing resources like PodMonitor / ServiceMonitor CRs, per-target credential Secrets (basic-auth, TLS, bearer, OAuth2), and supporting RBAC are steered by our public docs ([`prometheus-metrics-scrape-crd`](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd)) into the **customer's own app namespace** (every example uses `namespace: app-namespace` / `my-app`). When placed there — the documented default — the VAP doesn't fire at all because the customer's namespace isn't one of the 20 protected ones. See [§7.5](#75-the-monitors-namespace-anchors-everything) and [§7.6](#76-two-strategic-options-considered) for the full reasoning.
- **"Why can't ama-metrics use `ama-metrics-serviceaccount` instead?"** That SA *is* already exempt (it's in `matchConditions[0]`'s `userInfo.username` allowlist). But our customer-facing UX has customers `kubectl apply` these objects themselves — that's the whole problem. We could change that UX (Options 1/2/3 in the solution-options doc), but the name carve-out is the cheapest fix.
- **"Why hardcode names instead of a `kubernetes.azure.com/created-by=ama-metrics` label?"** Because the objects are customer-created — the customer doesn't always set such a label, and asking them to is a worse migration than enumerating the 5 names. We could ship the name carve-out *and* a label-based fallback if AKS prefers.
- **"Will you add more names later?"** Each new ama-metrics object the public docs add to `kube-system` (or any rename) is a new round of "petition AKS". We currently have 5. We'll commit to flagging docs changes that introduce new `kube-system` objects.
- **"What about a new namespace?"** We'd come back with a new ask. The current `kube-system`-only carve-out is intentional.
- **"Did you test the Secret branch too?"** No — the 4-test matrix in §3 / Appendix A only ran against the ConfigMap branch. The Secret branch is structurally identical (`||` of the same `(group, resource, name)` shape evaluated under the same `matchCondition` semantics), so re-testing is a regression check, not a correctness check. We're happy to re-run the matrix on `zane-auto-2` if AKS would like the additional reassurance before merging.
- **"What about customers who put their PodMonitor / Secret in `kube-system`?"** Our recommendation (which we'll reinforce in the public docs as part of this change) is **do not** — the documented default is the app namespace, which works on MSNP today without any allowlist. Customers who insist on `kube-system` placement are an edge case we're deliberately not optimizing for; see [§7.6 Option 2](#76-two-strategic-options-considered) for why an allowlist for that case is structurally hard (CR and Secret names are customer-chosen, can't be enumerated).

---

## 7.5 The Monitor's namespace anchors everything

The investigation that drove the scoping of [§7](#7-the-exact-ask-to-take-to-aks) hinges on one structural fact about prometheus-operator CRDs (which ama-metrics inherits): **the PodMonitor / ServiceMonitor CR's namespace is the anchor that determines where every other customer-applied resource for the basic-auth scrape path must live.** The customer makes one choice — *"where do I put my Monitor?"* — and the cascade is fully determined from there.

### The cascade

```
Customer picks namespace N for the Monitor CR
              │
              ▼
    ┌─────────────────────────────────────────────────────┐
    │   N (the Monitor's namespace)                       │
    │                                                     │
    │   • PodMonitor / ServiceMonitor CR                  │
    │   • per-target credential Secret(s)                 │ ← CRD schema forces same ns
    │     — basic-auth, TLS, bearer, OAuth2               │   (no `namespace:` field in
    │       (customer-chosen names)                       │   any SecretKeySelector)
    │   • Role (ama-metrics-secrets-reader)               │ ← K8s 1.36+: must be where
    │   • RoleBinding (ama-metrics-secrets-rolebinding)   │   the Secret is readable
    └─────────────────────────────────────────────────────┘
              │
              ▼
    Edit ama-metrics-settings-configmap in kube-system to
    add N to secrets_access_namespaces (this one is always
    in kube-system, already in the ask)
```

The pinning is enforced at three levels:

1. **CRD schema** — *every* Secret-reference field in the PodMonitor/ServiceMonitor schema is a `corev1.SecretKeySelector` (or `corev1.ConfigMapKeySelector` for the CA / cert variants) with only `name`, `key`, `optional` properties — **no `namespace` field exists**. This covers `basicAuth.{username,password}`, `bearerTokenSecret`, `authorization.credentials`, `oauth2.{clientId,clientSecret}`, and `tlsConfig.{ca.secret, cert.secret, keySecret}` — all five credential families share the same shape. Verified in the repo: [`ama-metrics-servicemonitor-crd.yaml`](../../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-servicemonitor-crd.yaml) lines 64–92 and [`ama-metrics-podmonitor-crd.yaml`](../../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-podmonitor-crd.yaml) lines 104–132 (`basicAuth` shape) plus the corresponding `tlsConfig` blocks (PodMonitor lines 285–376). A customer literally cannot type a namespace into any of these references; the API server rejects `namespace:` as an unknown field. The reference is always resolved against the Monitor's own namespace. **Important corollary:** the CRD `tlsConfig` deliberately omits the file-path fields (`caFile` / `certFile` / `keyFile`) that exist in raw Prometheus config — so the PodMonitor/ServiceMonitor path *cannot* reach into the shared `ama-metrics-mtls-secret` mounted at `/etc/prometheus/certs/`. CRD-based TLS is fully self-contained in the Monitor's ns.
2. **RBAC scoping** — on K8s 1.36+ the addon's ClusterRole no longer grants cluster-wide Secret access (verified in [`ama-metrics-clusterRole.yaml`](../../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-clusterRole.yaml) lines 28-33 — the cluster-wide `secrets get/list/watch` rule is gated by `{{- if semverCompare "<1.36.0" ... }}`). The target allocator (running as `system:serviceaccount:kube-system:ama-metrics-serviceaccount`) needs an explicit `Role` granting `get/list/watch` on `secrets` *in the Secret's namespace*, plus a `RoleBinding` in that namespace pointing back to the kube-system SA. The Role + RoleBinding therefore must co-locate with the Secret, which must co-locate with the Monitor.
3. **Settings ConfigMap** — the customer must list every Secret's namespace in `secrets_access_namespaces` of `ama-metrics-settings-configmap`. This is the *only* link back into `kube-system`, and that ConfigMap is already in the ask.

### Why this matters for the VAP

Because of the cascade, **the customer's single choice of `N` decides whether the VAP fires at all** for the basic-auth scrape path:

| Customer picks `N` = | VAP fires? | Resources blocked |
|---|---|---|
| App namespace (docs default, e.g. `my-app`) | ❌ No | — |
| Centralized monitoring namespace (e.g. `monitoring-config`) | ❌ No | — |
| Any other non-protected ns | ❌ No | — |
| `kube-system` | ✅ Yes | PodMonitor/ServiceMonitor CR (customer-named), per-target credential Secret (customer-named), Role + RoleBinding (fixed-named) |
| Any other VAP-protected ns (e.g. `gatekeeper-system`) | ✅ Yes | same |

The docs' examples and defaults uniformly use the app namespace. The docs *do* acknowledge `kube-system` as a valid alternative in two places (the `secrets_access_namespaces` note and Step 4's "(including `kube-system` if needed)" parenthetical), but never as the recommended path.

### Mapping every public-docs resource to its VAP status

The full inventory walk of [`prometheus-metrics-scrape-configuration`](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration) and [`prometheus-metrics-scrape-crd`](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd):

| Resource | Kind | Namespace per docs (default) | Structurally forced into `kube-system`? | In ask? |
|---|---|---|---|---|
| Settings ConfigMap | ConfigMap | `kube-system` | ✅ Yes — agent reads it from `kube-system` only | ✅ |
| Custom scrape-config CMs (3 variants) | ConfigMap | `kube-system` | ✅ Yes — same | ✅ |
| mTLS cert bundle for **ConfigMap path** (`ama-metrics-mtls-secret`) | Secret | `kube-system` | ✅ Yes — mounted on the ama-metrics pods, which live in `kube-system` | ✅ |
| PodMonitor CR | PodMonitor | Customer app ns (`namespace: app-namespace`) | ❌ No — CR can live anywhere; `namespaceSelector` decouples target ns from CR ns | ❌ |
| ServiceMonitor CR | ServiceMonitor | Customer app ns | ❌ No — same | ❌ |
| Per-target credential Secret for **CRD path** — basic-auth, TLS material, bearer token, OAuth2 (customer-named, e.g. `my-basic-auth`) | Secret | Customer app ns (`namespace: my-app`) | ❌ No — chained to the Monitor's ns by the CRD schema (every credential field is a `SecretKeySelector` with no `namespace:`); customer chooses both | ❌ |
| `ama-metrics-secrets-reader` Role (K8s 1.36+) | Role | "each ns in `secrets_access_namespaces`" — default = customer app ns | ❌ No — chained to the Secret's ns | ❌ |
| `ama-metrics-secrets-rolebinding` RoleBinding | RoleBinding | same as Role | ❌ No — chained to the Role's ns | ❌ |
| PodMonitor / ServiceMonitor **CRDs** (definitions) | CustomResourceDefinition | Cluster-scoped, addon-installed | ❌ No — cluster-scoped; VAP's `resourceRules` are `scope: Namespaced`, and the install is done by the ama-metrics SA which is already exempt | ❌ |

### Scope summary

- **In the ask (5 named objects, all in `kube-system`, all with fixed names, all structurally forced):** 4 ConfigMaps + 1 mTLS Secret (the latter for the ConfigMap scrape-config path only).
- **Out of scope, by design:** PodMonitor / ServiceMonitor CRs, their per-target credential Secrets (basic-auth, TLS, bearer, OAuth2), and supporting RBAC — they all chain to the Monitor, and the documented default places the Monitor outside `kube-system`. See [§7.6](#76-two-strategic-options-considered) for why we made this design choice.
- **Out of scope, naturally:** customer-applied resources in any non-protected namespace (the VAP doesn't fire there), and cluster-scoped resources like the two CRDs (the VAP's `resourceRules` are `scope: Namespaced`).

---

## 7.6 Two strategic options considered

There were two clean strategies for handling the customer-applied side of the scrape path (PodMonitor / ServiceMonitor CRs, their per-target credential Secrets — basic-auth, TLS, bearer, OAuth2 — and the supporting RBAC). We picked **Option 1** as the basis for [§7](#7-the-exact-ask-to-take-to-aks); Option 2 was evaluated and rejected as structurally infeasible.

### Option 1 — Force customer CRs outside `kube-system` (recommended, the basis for §7)

**Mechanism:** Strengthen our public docs to make "deploy in your app namespace (or any non-protected namespace)" the *only supported* placement for PodMonitor / ServiceMonitor / their credential Secrets / RBAC. A customer who tries `kube-system` hits the VAP block and is steered by docs + support to pick a different namespace.

**Allowlist ask:** Only the 5 structurally-forced objects (4 ConfigMaps + mTLS Secret for the ConfigMap path). The Role + RoleBinding are not needed because they only end up in `kube-system` if the Monitor does.

**Pros:**
- Minimal ask. Easy AKS conversation.
- VAP stays meaningful — no wildcard hole, no name-pattern exemption.
- Already aligned with what every example in our public docs shows.
- Mental model is simple: "customer stuff lives in customer namespaces; addon stuff lives in `kube-system`."

**Cons:**
- The workload-in-`kube-system` edge case is stranded. A customer whose application pods themselves run in `kube-system` and use basic-auth scraping cannot put the Monitor + Secret next to those pods. Vanishingly small slice (customers extending `kube-system` with their own workloads is already an antipattern).
- Requires a docs update: remove the parenthetical "(including `kube-system` if needed)" from Step 4 of the basic-auth section and add an explicit "do not place these resources in `kube-system`" line.
- Customers who *currently* have Monitors in `kube-system` on non-MSNP clusters would need to migrate before they enable MSNP. (Unknown how many, but the docs have never recommended this layout.)

### Option 2 — Allow customer CRs inside `kube-system` (rejected as infeasible)

**Mechanism:** Extend the VAP allowlist to also cover customer-applied PodMonitor / ServiceMonitor CRs, basic-auth Secrets, and the Role + RoleBinding pair, all in `kube-system`.

**Why this fails on the structural level:** the names of two of those four kinds are **customer-chosen**, so they cannot be enumerated up-front:

| Resource | Name | Allowlistable by fixed name? |
|---|---|---|
| PodMonitor CR | customer-chosen (e.g. `my-app-monitor`, `frontend-scraper`, …) | ❌ unknowable in advance |
| ServiceMonitor CR | customer-chosen | ❌ unknowable |
| Per-target credential Secret (basic-auth / TLS / bearer / OAuth2) | customer-chosen (e.g. `my-basic-auth`, `nginx-tls`, …) | ❌ unknowable |
| Role | Fixed: `ama-metrics-secrets-reader` (per docs) | ✅ |
| RoleBinding | Fixed: `ama-metrics-secrets-rolebinding` (per docs) | ✅ |

To handle the unbounded cases, the VAP would need a *non-name* allowlist mechanism. Three sub-options, all bad:

| Sub-option | Mechanism | Why it fails |
|---|---|---|
| **2a. Wildcard / drop name check** | `request.resource.resource == "podmonitors"` with no name filter | Allows *any* PodMonitor in `kube-system` from *any* caller — VAP loses meaning for that resource kind; malicious actor could create monitors that scrape sensitive endpoints and exfil via relabel rules |
| **2b. Label-based exemption** | Require customer to label CR/Secret with `kubernetes.azure.com/managed-by=ama-metrics` | Requires customer cooperation (often forgotten); `kubectl apply` can overwrite labels; AKS would need a separate webhook to enforce label immutability; doesn't compose with `(automatic-authz)` which is name-keyed |
| **2c. Sub-namespace area** | Create a label-selected subset of `kube-system` that's exempted | Sub-namespace concepts don't exist in K8s; would require new K8s primitive or custom Gatekeeper overlay |

**Verdict:** Option 2 is structurally hard. The only way to make it work cleanly is to teach the VAP to recognize labels/annotations, and that's a heavier ask of AKS than "exempt these 5 names" — with a worse security posture and a UX cost on customers (who'd have to remember the label).

### Why we picked Option 1

- **Aligned with current docs** — every example in `prometheus-metrics-scrape-crd` already uses the app-namespace pattern. We're tightening from "recommended default" to "only supported placement," not introducing a new convention.
- **Minimal AKS ask** — 5 fixed names vs. an unbounded resource-kind exemption. Easier to review, easier to approve, easier to maintain.
- **Preserves VAP integrity** — no wildcard holes, no name-pattern matching, no label dependency.
- **The stranded edge case is genuinely rare** — customer apps in `kube-system` are an antipattern that's already discouraged by AKS broadly.

### What Option 1 requires from us (besides the ask)

1. **Docs change in `prometheus-metrics-scrape-crd`:** remove the "(including `kube-system` if needed)" parenthetical from Step 4 of the basic-auth section; add an explicit "do not place PodMonitor, ServiceMonitor, per-target credential Secrets (basic-auth, TLS, bearer, OAuth2), or their RBAC in `kube-system` — use your app namespace or a dedicated monitoring namespace" guidance line.
2. **Migration guidance for existing customers:** a short note in the MSNP onboarding docs telling customers who currently use `kube-system` placement to move before enabling MSNP, plus a `kubectl get podmonitors,servicemonitors -n kube-system -A` audit command.
3. **Support runbook entry:** "Customer hit VAP deny when applying PodMonitor in `kube-system`" → "Please move to your app namespace per docs."

---

## 8. Open questions for the AKS meeting

1. **Is AKS willing to ship per-customer-app exemptions to the protect-system-namespaces VAP at all?** If the answer is "no, file Options 1/2/3", that's the end of this conversation.
2. **Does AKS agree with the Option 1 framing in [§7.6](#76-two-strategic-options-considered)?** Specifically: do they accept that we'll tighten the public docs to forbid `kube-system` placement of PodMonitor / ServiceMonitor / per-target credential Secrets (basic-auth, TLS, bearer, OAuth2) / RBAC (currently the recommended-but-not-only path), in exchange for keeping the allowlist at 5 fixed names? If they want to support `kube-system` placement for those resources, we'd need Option 2's label-based or wildcard mechanism — both of which weaken the VAP.
3. **What's AKS's process for adding to the exemption list?** Quarterly release? Per-ask? Bound to ama-metrics's release cadence or AKS's?
4. **If we keep adding entries over time, at what count do they prefer we move to a more scalable mechanism** (e.g. a label-based exemption, or an annotation on the CM that ama-metrics could co-sign)?
5. **Does the `(automatic-authz)` webhook have its own exemption mechanism we should know about?** (For future asks where we'd need customer modifications to *other* aks-managed resources — out of scope today, but worth knowing.)
6. **Audit→Deny timeline for non-MSNP AKS Automatic.** This is the deadline question from the findings doc; restated here as the time-sensitive constraint on the conversation.

---

## Appendix A — Full reproduction

This appendix is a step-by-step recipe to re-run the entire experiment on another cluster. Anyone with `Azure Kubernetes Service RBAC Cluster Admin` on a classic AKS Automatic cluster (i.e., `hostedSystemProfile.enabled == false`, VAPB in `[Audit]`, no `(automatic-authz)` lock) should be able to reproduce these results.

### A.1 Prerequisites

- `kubectl` connected to a classic AKS Automatic cluster (not MSNP — see findings doc §0 "What changed since the original investigation" for how to tell the two apart).
- `python3` (used for safe JSON patch construction).
- An AAD identity in the cluster's `Azure Kubernetes Service RBAC Cluster Admin` group. Make sure `az aks get-credentials` has wired kubelogin in: `kubectl get ns` should work without prompting.
- About 5 minutes.

### A.2 Capture canonical state

**Do this first.** Every restoration step below depends on these snapshots.

```bash
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces \
  -o yaml > /tmp/zane-canonical-vap.yaml
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces \
  -o json > /tmp/zane-canonical-vap.json
kubectl get validatingadmissionpolicybinding aks-managed-protect-system-namespaces-binding \
  -o yaml > /tmp/zane-canonical-vapb.yaml

# Sanity check: binding should be [Audit] before we start
kubectl get validatingadmissionpolicybinding aks-managed-protect-system-namespaces-binding \
  -o jsonpath='{.spec.validationActions}'
# expected output: ["Audit"]
```

### A.3 Test resource YAMLs

```bash
cat > /tmp/zane-test-cm.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: { name: zane-experiment-cm, namespace: kube-system }
data: { test: "stage-1-experiment" }
EOF

cat > /tmp/zane-exempt-cm.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: { name: ama-metrics-settings-configmap, namespace: kube-system }
data: { test: "variant-positive" }
EOF

cat > /tmp/zane-nonexempt-cm.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: { name: zane-not-ama-metrics, namespace: kube-system }
data: { test: "variant-negative-same-ns" }
EOF

cat > /tmp/zane-exempt-cm-otherns.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata: { name: ama-metrics-settings-configmap, namespace: gatekeeper-system }
data: { test: "variant-negative-other-ns" }
EOF

cat > /tmp/zane-secret.yaml <<'EOF'
apiVersion: v1
kind: Secret
metadata: { name: zane-test-secret, namespace: kube-system }
type: Opaque
data: { test: dGVzdA== }
EOF
```

### A.4 Stage 1: flip VAPB to `[Deny]` (simulate MSNP)

```bash
# Baseline: kube-system CM creation should currently succeed (VAPB is in [Audit]).
kubectl apply -f /tmp/zane-test-cm.yaml
kubectl delete configmap zane-experiment-cm -n kube-system

# Stage 1: flip the binding.
kubectl patch validatingadmissionpolicybinding aks-managed-protect-system-namespaces-binding \
  --type=json -p='[{"op":"replace","path":"/spec/validationActions","value":["Deny"]}]'

# Sanity: kube-system write should now fail with the protect-system-namespaces message.
kubectl apply -f /tmp/zane-test-cm.yaml
# Expected: Error from server (Forbidden): … 'aks-managed-protect-system-namespaces' …
#           Modification of resources in managed system namespaces is not allowed

# Control: writes to non-protected namespaces are unaffected.
kubectl create configmap zane-control -n default --from-literal=test=x
kubectl delete configmap zane-control -n default
```

### A.5 Build a reusable "reset to canonical" patch

The two-line approach used in the experiment. `kubectl apply -f canonical.yaml` runs into `last-applied-configuration` annotation issues for resources we didn't create with `--save-config`; the merge-patch approach below is reliable:

```bash
python3 <<'PYEOF'
import json
with open("/tmp/zane-canonical-vap.json") as f:
    vap = json.load(f)
reset = {"spec": {
    "matchConditions": vap["spec"]["matchConditions"],
    "matchConstraints": vap["spec"]["matchConstraints"]
}}
json.dump(reset, open("/tmp/zane-reset-patch.json", "w"), indent=2)
PYEOF
```

After each variant, run:
```bash
kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-reset-patch.json
```

### A.6 Variant A — name carve-out

```bash
# Build the patch
python3 <<'PYEOF'
import json
vap = json.load(open("/tmp/zane-canonical-vap.json"))
mcs = vap["spec"]["matchConditions"] + [{
    "name": "exempt-ama-metrics-configmaps",
    "expression": (
        '!(request.namespace == "kube-system" && '
        'request.resource.resource == "configmaps" && '
        'request.name in ["ama-metrics-prometheus-config", '
        '"ama-metrics-settings-configmap", '
        '"ama-metrics-prometheus-config-node", '
        '"ama-metrics-prometheus-config-node-windows"])'
    )
}]
json.dump({"spec": {"matchConditions": mcs}}, open("/tmp/zane-variantA-patch.json", "w"), indent=2)
PYEOF

# Apply, wait, test
kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-variantA-patch.json
sleep 3

kubectl apply -f /tmp/zane-exempt-cm.yaml         # T1: expected SUCCEED
kubectl apply -f /tmp/zane-nonexempt-cm.yaml      # T2: expected FAIL
kubectl apply -f /tmp/zane-exempt-cm-otherns.yaml # T3: expected FAIL
kubectl apply -f /tmp/zane-secret.yaml            # T4: expected FAIL

# Cleanup whichever succeeded
kubectl delete configmap ama-metrics-settings-configmap -n kube-system --ignore-not-found
kubectl delete configmap zane-not-ama-metrics -n kube-system --ignore-not-found
kubectl delete configmap ama-metrics-settings-configmap -n gatekeeper-system --ignore-not-found
kubectl delete secret zane-test-secret -n kube-system --ignore-not-found

# Reset before next variant!
kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-reset-patch.json
```

### A.7 Variant B — identity exemption

```bash
# Replace the user identity below with your own (whatever your kubelogin returns).
MY_USER="zanejohnson@microsoft.com"

python3 <<PYEOF
import json
vap = json.load(open("/tmp/zane-canonical-vap.json"))
mcs = vap["spec"]["matchConditions"]
needle = "'system:serviceaccount:kube-system:konnectivity-agent-autoscaler'"
mcs[0]["expression"] = mcs[0]["expression"].replace(
    needle, needle + ", '${MY_USER}'"
)
json.dump({"spec": {"matchConditions": mcs}}, open("/tmp/zane-variantB-patch.json", "w"), indent=2)
PYEOF

kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-variantB-patch.json
sleep 3

kubectl apply -f /tmp/zane-exempt-cm.yaml         # T1: SUCCEED
kubectl apply -f /tmp/zane-nonexempt-cm.yaml      # T2: ALSO SUCCEEDS (too broad)
kubectl apply -f /tmp/zane-exempt-cm-otherns.yaml # T3: ALSO SUCCEEDS (too broad)
kubectl apply -f /tmp/zane-secret.yaml            # T4: ALSO SUCCEEDS (too broad)

# Cleanup
kubectl delete configmap ama-metrics-settings-configmap -n kube-system --ignore-not-found
kubectl delete configmap zane-not-ama-metrics -n kube-system --ignore-not-found
kubectl delete configmap ama-metrics-settings-configmap -n gatekeeper-system --ignore-not-found
kubectl delete secret zane-test-secret -n kube-system --ignore-not-found
kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-reset-patch.json
```

### A.8 Variant C — namespace carve-out

```bash
python3 <<'PYEOF'
import json
vap = json.load(open("/tmp/zane-canonical-vap.json"))
me = vap["spec"]["matchConstraints"]["namespaceSelector"]["matchExpressions"]
me[0]["values"] = [v for v in me[0]["values"] if v != "kube-system"]
json.dump({"spec": {"matchConstraints": vap["spec"]["matchConstraints"]}},
          open("/tmp/zane-variantC-patch.json", "w"), indent=2)
PYEOF

kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-variantC-patch.json
sleep 3

kubectl apply -f /tmp/zane-exempt-cm.yaml         # T1: SUCCEED
kubectl apply -f /tmp/zane-nonexempt-cm.yaml      # T2: ALSO SUCCEEDS (too broad)
kubectl apply -f /tmp/zane-exempt-cm-otherns.yaml # T3: correctly FAILS (other ns)
kubectl apply -f /tmp/zane-secret.yaml            # T4: ALSO SUCCEEDS (too broad)

# Cleanup
kubectl delete configmap ama-metrics-settings-configmap -n kube-system --ignore-not-found
kubectl delete configmap zane-not-ama-metrics -n kube-system --ignore-not-found
kubectl delete configmap ama-metrics-settings-configmap -n gatekeeper-system --ignore-not-found
kubectl delete secret zane-test-secret -n kube-system --ignore-not-found
```

### A.9 Stage 3: full cleanup

**Critical.** Leaving the binding in `[Deny]` will break the cluster for the next user. Always run:

```bash
# 1. Reset VAP to canonical
kubectl patch validatingadmissionpolicy aks-managed-protect-system-namespaces \
  --type=merge --patch-file=/tmp/zane-reset-patch.json

# 2. Flip VAPB back to [Audit]
kubectl patch validatingadmissionpolicybinding aks-managed-protect-system-namespaces-binding \
  --type=json -p='[{"op":"replace","path":"/spec/validationActions","value":["Audit"]}]'

# 3. Verify byte-identical spec
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces -o json | python3 -c "
import json, sys
live = json.load(sys.stdin)['spec']
canonical = json.load(open('/tmp/zane-canonical-vap.json'))['spec']
print('SAME' if live == canonical else 'DIFFERENT')
"

# 4. Verify binding is back to [Audit]
kubectl get validatingadmissionpolicybinding aks-managed-protect-system-namespaces-binding \
  -o jsonpath='{.spec.validationActions}'
# expected: ["Audit"]

# 5. Verify no leftover test resources
kubectl get configmap -A 2>/dev/null | grep -i zane || echo "no zane-* CMs left"
kubectl get secret -A 2>/dev/null | grep -i zane || echo "no zane-* secrets left"

# 6. Verify behavior: as customer, kube-system CM creation succeeds again
kubectl apply -f /tmp/zane-test-cm.yaml
kubectl delete configmap zane-experiment-cm -n kube-system

# 7. Wipe /tmp files
rm -f /tmp/zane-*.yaml /tmp/zane-*.json
```

### A.10 Gotchas encountered during the original run

- **`kubectl apply -f /tmp/canonical.yaml` fails** with `Operation cannot be fulfilled … the object has been modified; please apply your changes to the latest version` and a `last-applied-configuration` annotation warning. Reason: the VAP wasn't originally created with `kubectl apply --save-config`, so it has no last-applied annotation; kubectl then tries to compute a strategic merge using only the current state and races against any in-flight reconcile. **Solution:** use `kubectl patch --type=merge --patch-file=…` with a hand-built reset patch that only mentions `spec.matchConditions` and `spec.matchConstraints` (the two fields we ever change). This is what `/tmp/zane-reset-patch.json` is for.
- **`kubectl patch --type=merge` on `spec.matchConstraints` does not touch `spec.matchConditions`** (and vice versa). This is what bit us between Variant B and Variant C: applying Variant C's `matchConstraints` patch left Variant B's `matchConditions` change in place, and the test results were misleadingly contaminated until we noticed `zanejohnson@microsoft.com` was still in the list. **Solution:** always run the full reset patch between variants, and re-verify `matchConditions[0]` doesn't contain your test user before running a non-B variant.
- **`kubectl auth can-i` can mislead** on these resources because it sends a `SelfSubjectAccessReview` without a resource name; the name-based authorization checks (incl. the `(automatic-authz)` webhook on MSNP) can't apply without a name. Use `kubectl auth can-i <verb> <resource>/<name>` to get a name-aware answer. (Not strictly needed for this experiment since we're on a cluster without the lock, but worth knowing if anyone tries to translate this recipe to MSNP.)
- **Don't forget step A.9.2 (revert VAPB to `[Audit]`).** Leaving it in `[Deny]` means the next person on the cluster can't write to `kube-system`. Set yourself a reminder before starting.
