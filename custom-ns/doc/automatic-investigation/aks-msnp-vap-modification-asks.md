# What VAP modification should we ask AKS for? — Variants A / B / C tested on `zane-auto-2`

> **Date:** 2026-05-19
> **Tested on:** `zane-auto-2` (RG `zane-auto-2`, sub `9c17527c-af8f-4148-8019-27bada0845f7`) — classic AKS Automatic, no MSNP, VAPB normally in `[Audit]`, `(automatic-authz)` lock not active. This is the only cluster where the experiment is possible: the VAP exists, the binding can be flipped to `[Deny]` to simulate MSNP, and the customer is still allowed to patch the VAP itself (which they can't on real MSNP).
> **Identity used in tests:** `zanejohnson@microsoft.com` (subscription `Owner` + `Azure Kubernetes Service RBAC Cluster Admin` on the cluster)
> **Related docs:**
> - [`aks-automatic-msnp-kube-system-findings.md`](./aks-automatic-msnp-kube-system-findings.md) — primary findings on the VAP + `(automatic-authz)` lock
> - [`aks-automatic-msnp-configmap-solution-options.md`](./aks-automatic-msnp-configmap-solution-options.md) — broader solution space (CRDs, ARM property, customer-ns CM, etc.) — Option 5 in that doc is the AKS-side ask this doc nails down

---

## TL;DR

**The cleanest ask to bring to AKS is to add one new `matchCondition` to the `ValidatingAdmissionPolicy` named `aks-managed-protect-system-namespaces`.** That single CEL clause exempts exactly the customer-facing `ama-metrics-*` objects the public docs tell customers to apply in `kube-system` — **4 ConfigMaps + 1 mTLS Secret + 1 Role + 1 RoleBinding**, all by fixed name — and **nothing else**. Same user, same role, same protected namespace; only the seven specifically named objects become writable.

> The pattern was experimentally validated for the **ConfigMap branch** on `zane-auto-2` with a 4-test matrix (see §3 Variant A and Appendix A). The other three branches (Secret, Role, RoleBinding) are **additional `||` clauses of the same CEL idiom** — same `matchCondition` semantics, same `(group, resource, name)` shape, no new mechanism. See [§7.5](#75-beyond-configmaps-deriving-the-full-kube-system-inventory) for the per-resource derivation from the public docs.

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
       request.name == "ama-metrics-mtls-secret") ||
      (request.resource.group == "rbac.authorization.k8s.io" &&
       request.resource.resource == "roles" &&
       request.name == "ama-metrics-secrets-reader") ||
      (request.resource.group == "rbac.authorization.k8s.io" &&
       request.resource.resource == "rolebindings" &&
       request.name == "ama-metrics-secrets-rolebinding")
    ))
```

Two alternative shapes were tested and rejected (rejections apply equally to the broader form):

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
- The 4-test matrix above was run against the **ConfigMap branch only**. The broader ask (see [§7](#7-the-exact-ask-to-take-to-aks) and [§7.5](#75-beyond-configmaps-deriving-the-full-kube-system-inventory)) extends the same `(group, resource, name)` shape to Secret + Role + RoleBinding — structurally identical, additional `||` branches of the same CEL idiom — but those branches are not empirically re-validated here. We can re-run the matrix per kind on `zane-auto-2` if AKS asks.

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
>        request.name == "ama-metrics-mtls-secret") ||
>       (request.resource.group == "rbac.authorization.k8s.io" &&
>        request.resource.resource == "roles" &&
>        request.name == "ama-metrics-secrets-reader") ||
>       (request.resource.group == "rbac.authorization.k8s.io" &&
>        request.resource.resource == "rolebindings" &&
>        request.name == "ama-metrics-secrets-rolebinding")
>     ))
> ```
>
> This carves out the **seven** specific objects the public docs tell customers to apply in `kube-system`:
>
> - **4 ConfigMaps** — `ama-metrics-settings-configmap` (settings) + the three custom-scrape-config variants (`-prometheus-config` for replica, `-prometheus-config-node` for Linux daemonset, `-prometheus-config-node-windows` for Windows daemonset).
> - **1 Secret** — `ama-metrics-mtls-secret` ([TLS/mTLS scraping certs](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#tls-based-scraping); docs explicitly require this Secret in `kube-system` with this exact name).
> - **1 Role + 1 RoleBinding** — `ama-metrics-secrets-reader` / `ama-metrics-secrets-rolebinding`, required on K8s 1.37+ in every namespace listed in `secrets_access_namespaces` ([Step 4](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#step-4-create-rbac-in-each-namespace-kubernetes--137) — "including `kube-system` if needed").
>
> All other writes to `kube-system` (other ConfigMaps, other Secrets, other RBAC, workloads, etc.) and all writes to the other 19 protected namespaces remain blocked. We verified this on a fresh `zane-auto-2` cluster (classic AKS Automatic, VAPB flipped to `[Deny]` to simulate MSNP behavior) with a 4-case test matrix — see [Appendix A](#appendix-a-full-reproduction). The matrix was run against the ConfigMap branch; the other three branches are the same CEL idiom extended with additional `(group, resource, name)` triples (see [§7.5](#75-beyond-configmaps-deriving-the-full-kube-system-inventory) for the derivation).
>
> No change is required to the `(automatic-authz)` authorization webhook, the binding, or any other AKS-managed resource.

### Pre-emptive answers to likely AKS questions

- **"Why these seven names specifically?"** They are the complete set of `kube-system`-scoped customer-applied objects across the two public docs pages for managed Prometheus customization ([ConfigMap-based](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration) and [CRD-based with basic-auth / TLS](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd)). See §7.5 for the full inventory walk.
- **"Why can't ama-metrics use `ama-metrics-serviceaccount` instead?"** That SA *is* already exempt (it's in `matchConditions[0]`'s `userInfo.username` allowlist). But our customer-facing UX has customers `kubectl apply` these objects themselves — that's the whole problem. We could change that UX (Options 1/2/3 in the solution-options doc), but the name carve-out is the cheapest fix.
- **"Why hardcode names instead of a `kubernetes.azure.com/created-by=ama-metrics` label?"** Because the objects are customer-created — the customer doesn't always set such a label, and asking them to is a worse migration than enumerating the seven names. We could ship the name carve-out *and* a label-based fallback if AKS prefers.
- **"Will you add more names later?"** Each new ama-metrics object the public docs add to `kube-system` (or any rename) is a new round of "petition AKS". We currently have 7. We'll commit to flagging docs changes that introduce new `kube-system` objects.
- **"What about a new namespace?"** We'd come back with a new ask. The current `kube-system`-only carve-out is intentional.
- **"Did you test the Secret / Role / RoleBinding branches too?"** No — the 4-test matrix in §3 / Appendix A only ran against the ConfigMap branch. The other three branches are structurally identical (`||` of the same `(group, resource, name)` shape evaluated under the same `matchCondition` semantics), so re-testing is a regression check, not a correctness check. We're happy to re-run the matrix per resource kind on `zane-auto-2` if AKS would like the additional reassurance before merging.
- **"What about the customer's basic-auth Secret (e.g. `my-basic-auth`)?"** Its **name is customer-chosen**, so we can't allowlist it by name. The public docs already steer customers to put basic-auth Secrets in their app namespace (where the VAP doesn't fire) — that's the right default and we're not asking AKS to support `kube-system` placement for that case. See §7.5 footnote.

---

## 7.5 Beyond ConfigMaps: deriving the full kube-system inventory

Variant A's 4-test matrix on `zane-auto-2` validated the **pattern** — a `(resource, name)`-keyed `matchCondition` skip applied to one resource kind in one namespace. The same pattern extends naturally to additional `(group, resource, name)` triples without re-testing the underlying mechanism. The remaining question is: **which triples**?

We answered that by walking the two public docs pages for customer-configurable Prometheus collection on AKS — [`prometheus-metrics-scrape-configuration`](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration) (ConfigMap-based) and [`prometheus-metrics-scrape-crd`](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd) (CRD-based, basic-auth, TLS) — and noting every K8s object the customer is told to apply, plus its namespace:

| Doc step | Kind | apiGroup | Namespace (per docs) | VAP-blocked? | In ask? |
|---|---|---|---|---|---|
| [Settings ConfigMap](https://aka.ms/azureprometheus-addon-settings-configmap) | ConfigMap (`ama-metrics-settings-configmap`) | `""` (core) | `kube-system` | ✅ blocked | ✅ in ask |
| [Custom scrape config — replica](../../../otelcollector/configmaps/ama-metrics-prometheus-config-configmap.yaml) | ConfigMap (`ama-metrics-prometheus-config`) | `""` | `kube-system` | ✅ blocked | ✅ in ask |
| [Custom scrape config — Linux daemonset](../../../otelcollector/configmaps/ama-metrics-prometheus-config-node-configmap.yaml) | ConfigMap (`ama-metrics-prometheus-config-node`) | `""` | `kube-system` | ✅ blocked | ✅ in ask |
| [Custom scrape config — Windows daemonset](../../../otelcollector/configmaps/ama-metrics-prometheus-config-node-windows-configmap.yaml) | ConfigMap (`ama-metrics-prometheus-config-node-windows`) | `""` | `kube-system` | ✅ blocked | ✅ in ask |
| [TLS/mTLS scraping cert bundle](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#tls-based-scraping) | Secret (`ama-metrics-mtls-secret`) | `""` | `kube-system` (docs specify exactly this) | ✅ blocked | ✅ in ask |
| [Basic-auth Secret access — Role](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#step-4-create-rbac-in-each-namespace-kubernetes--137) | Role (`ama-metrics-secrets-reader`) | `rbac.authorization.k8s.io` | each ns in `secrets_access_namespaces` — **including `kube-system` if used** | ✅ blocked (when placed in `kube-system`) | ✅ in ask |
| [Basic-auth Secret access — RoleBinding](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#step-4-create-rbac-in-each-namespace-kubernetes--137) | RoleBinding (`ama-metrics-secrets-rolebinding`) | `rbac.authorization.k8s.io` | same as Role above | ✅ blocked (when placed in `kube-system`) | ✅ in ask |
| [PodMonitor CR](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#example-pod-monitor) | PodMonitor | `azmonitoring.coreos.com` | **customer app namespace** (docs example: `app-namespace` / `my-app`) | ❌ not blocked (outside the 20 protected ns) | ❌ no ask needed |
| [ServiceMonitor CR](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#example-service-monitor) | ServiceMonitor | `azmonitoring.coreos.com` | **customer app namespace** | ❌ not blocked | ❌ no ask needed |
| [PodMonitor CRD](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-podmonitor-crd.yaml) | CustomResourceDefinition | `apiextensions.k8s.io` | cluster-scoped, **addon-installed** (not customer-applied) | ❌ not blocked (cluster-scoped, and ama-metrics SA is already exempt) | ❌ no ask needed |
| [ServiceMonitor CRD](https://github.com/Azure/prometheus-collector/blob/main/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-servicemonitor-crd.yaml) | CustomResourceDefinition | `apiextensions.k8s.io` | cluster-scoped, addon-installed | ❌ not blocked | ❌ no ask needed |
| [Basic-auth Secret (customer-named)](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#step-1-create-the-basic-auth-secret) | Secret (customer-chosen name, e.g. `my-basic-auth`) | `""` | **customer app namespace** (docs example: `my-app`) | ❌ not blocked when in customer ns | ❌ no ask needed — see footnote |
| [Basic-auth Role/RoleBinding in customer ns](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#step-4-create-rbac-in-each-namespace-kubernetes--137) | Role + RoleBinding | `rbac.authorization.k8s.io` | customer app namespace | ❌ not blocked | ❌ no ask needed |

> **Footnote on the customer-named basic-auth Secret.** Step 4 of the basic-auth docs notes "(including `kube-system` if needed)" — so a customer *could* place the basic-auth Secret in `kube-system`. We deliberately exclude that from the ask because the Secret's name is **customer-chosen**: there is no fixed name we could allowlist. Our recommendation to customers is the docs' default — put basic-auth Secrets in your *app* namespace, where the VAP does not fire. If a customer insists on `kube-system`, that's an edge case AKS would need to handle through a different mechanism (e.g. a per-customer allowlist or an opt-in label) — out of scope for this ask.

### Scope summary

- **In the ask (7 named objects, all in `kube-system`, all with fixed names):** 4 ConfigMaps + 1 Secret + 1 Role + 1 RoleBinding.
- **Out of scope, deliberately:** customer-named basic-auth Secret in `kube-system` (no fixed name to allowlist).
- **Out of scope, naturally:** everything customers apply in their own app namespace (PodMonitor / ServiceMonitor CRs, basic-auth Secret, Role, RoleBinding) — the VAP doesn't cover those namespaces. Everything cluster-scoped (the two CRDs) — the VAP's `resourceRules` are `scope: Namespaced`.

### Empirical coverage of the ask

| Branch of the CEL | Tested on `zane-auto-2`? | Confidence |
|---|---|---|
| ConfigMaps (4 names) | ✅ Variant A 4-test matrix (§3, Appendix A) | High |
| Secret (`ama-metrics-mtls-secret`) | ❌ not yet | High by analogy — same VAP, same `matchCondition` semantics, additional `(group, resource, name)` triple in the same `\|\|` chain. We recommend AKS apply the consolidated CEL in one shot; happy to re-run the matrix per kind if AKS asks. |
| Role (`ama-metrics-secrets-reader`) | ❌ not yet | Same as above |
| RoleBinding (`ama-metrics-secrets-rolebinding`) | ❌ not yet | Same as above |

### Why we're confident the additional branches don't widen the blast radius

The added branches each lock down to one fixed `name` in `kube-system`. The CEL operator semantics are:

- `||` short-circuits — if any branch matches, the whole inner expression is `true`, and `!true == false` makes the `matchCondition` skip the policy (request allowed). Branches are independent; they don't cross-contaminate.
- A request that doesn't match `request.namespace == "kube-system"` short-circuits the outer `&&` to `false`, so the entire negation evaluates to `true` and the policy still applies — i.e., the carve-out is strictly scoped to `kube-system`.
- The existing `matchConditions[0]` and `matchConditions[1]` (user/group exempt lists) are unchanged; they still run first and are ANDed with our new condition, so the carve-out does nothing for callers who would already have been exempt.

The net effect, formally: the set of admit-without-validation requests grows from `{kube-system, configmaps, ∈ 4-name list}` to `{kube-system, configmaps, ∈ 4-name list} ∪ {kube-system, secrets, ama-metrics-mtls-secret} ∪ {kube-system, roles, ama-metrics-secrets-reader} ∪ {kube-system, rolebindings, ama-metrics-secrets-rolebinding}`. No other set membership changes.

---

## 8. Open questions for the AKS meeting

1. **Is AKS willing to ship per-customer-app exemptions to the protect-system-namespaces VAP at all?** If the answer is "no, file Options 1/2/3", that's the end of this conversation.
2. **What's AKS's process for adding to the exemption list?** Quarterly release? Per-ask? Bound to ama-metrics's release cadence or AKS's?
3. **If we keep adding entries over time, at what count do they prefer we move to a more scalable mechanism** (e.g. a label-based exemption, or an annotation on the CM that ama-metrics could co-sign)?
4. **Does the `(automatic-authz)` webhook have its own exemption mechanism we should know about?** (For future asks where we'd need customer modifications to *other* aks-managed resources — out of scope today, but worth knowing.)
5. **Audit→Deny timeline for non-MSNP AKS Automatic.** This is the deadline question from the findings doc; restated here as the time-sensitive constraint on the conversation.

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
