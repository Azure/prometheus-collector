# What VAP modification should we ask AKS for? — Variants A / B / C tested on `zane-auto-2`

> **Date:** 2026-05-19
> **Tested on:** `zane-auto-2` (RG `zane-auto-2`, sub `9c17527c-af8f-4148-8019-27bada0845f7`) — classic AKS Automatic, no MSNP, VAPB normally in `[Audit]`, `(automatic-authz)` lock not active. This is the only cluster where the experiment is possible: the VAP exists, the binding can be flipped to `[Deny]` to simulate MSNP, and the customer is still allowed to patch the VAP itself (which they can't on real MSNP).
> **Identity used in tests:** `zanejohnson@microsoft.com` (subscription `Owner` + `Azure Kubernetes Service RBAC Cluster Admin` on the cluster)
> **Related docs:**
> - [`aks-automatic-msnp-kube-system-findings.md`](./aks-automatic-msnp-kube-system-findings.md) — primary findings on the VAP + `(automatic-authz)` lock
> - [`aks-automatic-msnp-configmap-solution-options.md`](./aks-automatic-msnp-configmap-solution-options.md) — broader solution space (CRDs, ARM property, customer-ns CM, etc.) — Option 5 in that doc is the AKS-side ask this doc nails down

---

## TL;DR

**The cleanest ask to bring to AKS is to add one new `matchCondition` to the `ValidatingAdmissionPolicy` named `aks-managed-protect-system-namespaces`.** That single CEL clause exempts exactly the four `ama-metrics-*` ConfigMaps in `kube-system` and **nothing else** — same user, same role, same protected namespace; only the four specifically named ConfigMaps become writable.

The drop-in YAML:

```yaml
# In spec.matchConditions[], appended as the third entry (evaluated AND with the
# existing two: apply-to-non-exempt-users, apply-to-non-exempt-groups).
- name: exempt-ama-metrics-configmaps
  expression: |
    !(request.namespace == "kube-system" &&
      request.resource.resource == "configmaps" &&
      request.name in ["ama-metrics-prometheus-config",
                       "ama-metrics-settings-configmap",
                       "ama-metrics-prometheus-config-node",
                       "ama-metrics-prometheus-config-windowsdaemonset"])
```

Two alternative shapes were tested and rejected:

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
                       "ama-metrics-prometheus-config-windowsdaemonset"])
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
- The list of 4 names becomes a contract between us and AKS. Each new CM we add to ama-metrics is a new round of "petition AKS" and a new release.
- This only carves out **CMs we already know about**. If a customer wants to *delete* one of these (e.g. roll back), they'd be exempt too — that's almost certainly fine but worth flagging to the AKS reviewer.
- Doesn't help with ama-metrics needs in any other namespace (we're currently only writing to `kube-system`, so n/a today).

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
> - name: exempt-ama-metrics-configmaps
>   expression: |
>     !(request.namespace == "kube-system" &&
>       request.resource.resource == "configmaps" &&
>       request.name in ["ama-metrics-prometheus-config",
>                        "ama-metrics-settings-configmap",
>                        "ama-metrics-prometheus-config-node",
>                        "ama-metrics-prometheus-config-windowsdaemonset"])
> ```
>
> This carves out only the four `ama-metrics-*` ConfigMaps in `kube-system` from the protect-system-namespaces deny. All other writes to `kube-system` (other ConfigMaps, Secrets, RBAC, workloads, etc.) and all writes to the other 19 protected namespaces remain blocked. We verified this on a fresh `zane-auto-2` cluster (classic AKS Automatic, VAPB flipped to `[Deny]` to simulate MSNP behavior) with a 4-case test matrix; see [Appendix A](#appendix-a-full-reproduction).
>
> No change is required to the `(automatic-authz)` authorization webhook, the binding, or any other AKS-managed resource.

### Pre-emptive answers to likely AKS questions

- **"Why can't ama-metrics use `ama-metrics-serviceaccount` instead?"** That SA *is* already exempt (it's in `matchConditions[0]`'s `userInfo.username` allowlist). But our customer-facing UX has customers `kubectl apply` these CMs themselves — that's the whole problem. We could change that UX (Options 1/2/3 in the solution-options doc), but Variant A is the cheapest fix.
- **"Why hardcode the names instead of a `kubernetes.azure.com/created-by=ama-metrics` label?"** Because the CMs are customer-created — the customer doesn't always set such a label, and asking them to is a worse migration than just renaming on our side. We could ship Variant A *and* a label-based fallback if AKS prefers.
- **"Will you add more names later?"** Each new ama-metrics CM means another AKS ask. We currently have 4. We'll commit to renames before adding to the list.
- **"What about a new namespace?"** We'd come back with a new ask. The current `kube-system`-only carve-out is intentional.

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
        '"ama-metrics-prometheus-config-windowsdaemonset"])'
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
