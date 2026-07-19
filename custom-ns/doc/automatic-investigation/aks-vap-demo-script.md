# Demo: How I approached the AKS `kube-system` lockdown problem

> **Audience:** the team · **Duration:** ~10 min · **Format:** talk-through (no slides)
>
> **One-liner:** We were about to spend a quarter migrating ama-metrics out of `kube-system`. Turned out we didn't have to move anything — we had to change one admission-policy clause. This is the story of how a *solution* got reframed into a *problem*, and how the real root cause got found, reproduced, fixed, and shipped.

**Three acts:**
1. The Problem (~2 min) — the reframe
2. My Approach (~5–6 min) — RCA → reproduce → fix → buy-in
3. Lessons (~2–3 min)

---

## Act 1 — The Problem (~2 min)

**Say this:**

- **Quick context first — what is ama-metrics?** It's Azure's **managed Prometheus agent**. It runs *inside* the customer's AKS cluster — in the `kube-system` namespace — scrapes their pods' `/metrics` endpoints, and remote-writes to an Azure Monitor Workspace for Grafana, alerts, and dashboards. Customers configure it with **ConfigMaps, a Secret, and PodMonitor/ServiceMonitor custom resources** — and today *all of that lives in `kube-system`*. Hold onto that last fact; it's why this whole thing is hard.
- The project landed on my plate as: *"migrate ama-metrics to another namespace."*
- That's a **big** lift — the addon is deeply wired into `kube-system`: DaemonSet + ReplicaSet + target allocator, hardcoded Secret volume mounts, a ClusterRole, an addon-managed service account. Moving it is a multi-month, high-risk migration touching every deployment mode (AKS addon / Arc / CCP).
- So the first question I asked wasn't *"how do I move it?"* It was **"why do we think we need to move it at all?"**
- The trigger was: on **MSNP / AKS Automatic** clusters, customers can no longer apply the ama-metrics ConfigMaps to `kube-system`. Their Prometheus scrape customization silently stops working.
- **The reframe:** "migrate ama-metrics" is a *proposed solution*. The actual **problem** is narrower and concrete:

  > *A customer can't write the ama-metrics ConfigMaps / Secret / CRs into `kube-system` on MSNP clusters.*

- Once it's framed that way, migrating the whole addon is obviously not the only option — it's just the most expensive one.

**Show this** (the customer-visible symptom — verbatim from the cluster):

```text
❯ kubectl apply -f ama-metrics-settings-configmap.yaml
Error from server (Forbidden): error when creating "ama-metrics-settings-configmap.yaml":
configmaps "ama-metrics-settings-configmap" is forbidden:
ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces'
with binding 'aks-managed-protect-system-namespaces-binding'
denied request: Modification of resources in managed system namespaces is not allowed
```

---

## Act 2 — My Approach (~5–6 min)

### Step 1 — Ask the domain owners, but keep researching in parallel

**Say this:**

- I asked the AKS team directly: *why is `kube-system` locked down, and what's the mechanism?*
- Their answer pointed at **Deployment Safeguards** and they proposed some reference solutions (workarounds on our side).
- I **didn't just take the suggested path.** In parallel I did my own investigation on a live cluster — because I wanted to confirm the *actual* mechanism before committing engineering to any fix. (Foreshadow: "Deployment Safeguards" turned out to be the wrong culprit.)

### Step 2 — Root-cause analysis

**Say this:**

- On a real MSNP cluster I confirmed the block is **not** Deployment Safeguards / Gatekeeper / Azure Policy.
- It's a native Kubernetes (1.30+) **`ValidatingAdmissionPolicy`** named **`aks-managed-protect-system-namespaces`**, enforced by its binding in `[Deny]` mode. It denies writes to a 20-namespace protected set — `kube-system` is one of them.
- Key insight that unlocked everything: **this is an *admission* deny, not an *authorization* deny.**

  ```bash
  kubectl auth can-i create configmap -n kube-system
  # yes        ← RBAC says YES
  ```

  RBAC authorizes the action; the VAP rejects it *after* authorization succeeds. **This means no Azure role — not even a custom one — can bypass it.** I proved that separately: an elevated role still gets denied, because the webhook keys off the request, not the caller.
- **Why this matters:** it kills the entire class of "just give the customer a more powerful role" workarounds in one shot, and it tells us the fix has to live *in the policy itself*.

**Show this** (the actual denying policy identity):

```text
ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces'
  binding: 'aks-managed-protect-system-namespaces-binding'
  validationActions: [Deny]
  mechanism: native K8s VAP (CEL), NOT Gatekeeper / Deployment Safeguards
```

### Step 3 — Reproduce it safely (proof of concept)

**Say this:**

- I didn't want to experiment on a real MSNP cluster. So I reproduced the exact failure on a **classic AKS Automatic cluster that had no MSNP.**
- On classic AKS Automatic, the *same* VAP is already present — its binding just runs in **`[Audit]`** mode, so writes still succeed.
- I flipped the binding from **`[Audit]` → `[Deny]`**, which reproduced the **identical** error message a real MSNP customer sees. That gave me a safe, disposable lab that behaves exactly like MSNP.

**Show this** (the reproduce lever):

```bash
# Same policy exists on classic AKS Automatic in Audit mode.
# Flip the binding to Deny → reproduces the MSNP customer failure exactly.
kubectl patch validatingadmissionpolicybinding \
  aks-managed-protect-system-namespaces-binding \
  --type merge -p '{"spec":{"validationActions":["Deny"]}}'
```

### Step 4 — Prototype the fix and validate it

**Say this:**

- A VAP runs its deny only when **all** its `matchConditions` are true. So an exception is just one more `matchCondition` — a negated CEL clause: *"if the request is this specific allowed case, short-circuit and admit."*
- I prototyped exactly that: a single clause exempting **only the objects that are structurally forced into `kube-system`** — the ama-metrics ConfigMaps and the one mTLS Secret — and validated with a test matrix that (a) the allowed objects now go through, and (b) everything else in `kube-system` is **still blocked**.
- This is the whole fix. **Zero code change in ama-metrics. Nothing moves namespaces.** One CEL clause.

**Show this** (the concrete ask I brought to AKS):

```yaml
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

### Step 5 — Get AKS buy-in, then validate what they shipped

**Say this:**

- I took the reproduction + the prototyped CEL + a written justification (which 5 objects, and *why each is structurally pinned* to `kube-system`) to the AKS Automatic team. Because it was already reproduced and scoped, they bought in.
- **They implemented the exceptions** on a canary cluster, and I validated end-to-end on 2026-06-18 (`trang-hosted-eastus2euap`, MSNP, K8s 1.35.5). What shipped was slightly **broader and better** than what I asked for:

  | Exception | What AKS implemented | Note |
  |---|---|---|
  | ConfigMaps | name **prefix** `ama-metrics-*` or `container-azm-ms-*` | Broader than my 4 exact names — also covers **ama-logs** and future CMs |
  | Secret | **exact** name `ama-metrics-mtls-secret` | Narrow, as asked |
  | CRs | `podmonitors` / `servicemonitors` in `azmonitoring.coreos.com/v1` | Lets customers put Monitors in `kube-system` too |

- **Validation results:** every allowed case (P1–P7) admitted; every negative control (N1, N2) still denied. I even proved the policy keys on `metadata.name`, not the filename (E1 denied, E2 allowed).
- **AKS is rolling this out now.**

**Show this** (positive + negative controls, verbatim):

```text
# ALLOW — ama-metrics ConfigMap now admitted
$ kubectl apply -f ama-metrics-settings-configmap.yaml
configmap/ama-metrics-settings-configmap created                       ✅

# ALLOW — ServiceMonitor CR in kube-system
$ kubectl apply -f vap-validation-servicemonitor.yaml
servicemonitor.azmonitoring.coreos.com/vap-validation-servicemonitor created   ✅

# DENY — unrelated ConfigMap still blocked (scope held)
$ kubectl apply -f vap-validation-negative-cm.yaml
Error from server (Forbidden): ... denied request:
Modification of resources in managed system namespaces is not allowed  ✅ (correctly blocked)
```

---

## Act 3 — Lessons to share (~2 min)

**Lesson 1 — Work backwards from the problem, not forward from a solution.**

- "Migrate ama-metrics to another namespace" *felt* like the problem, but it was a proposed solution in disguise. The real problem was one sentence: *"can't write our ConfigMaps to `kube-system`."*
- If I'd started building the migration, we'd have burned a quarter and still shipped a worse outcome. **RCA before committing to a fix** turned a multi-month migration into a one-clause policy change.
- Rule of thumb: when a task is phrased as *"do X,"* ask *"what problem does X solve?"* until you hit something that's actually a problem, not a solution.

**Lesson 2 — AI gives engineers superpowers *outside* their own domain.**

- I'm a monitoring/agent engineer. Before this, I had **no idea what a Validating Admission Policy was** — CEL expressions, admission vs authorization, VAP bindings, none of it.
- With AI as a research partner, I didn't just *learn* the domain fast enough to hold my own with the AKS admission-control experts — I **solved** a problem squarely in their domain (prototyped the exact CEL clause, reproduced the deny, built the validation matrix).
- The leverage isn't "AI writes my code." It's **"AI collapses the time to become dangerous in an unfamiliar domain,"** which is exactly what cross-team problems like this one demand.

**Lesson 3 — The best fix is often *less* code, or no code at all.**

- We shipped this with **zero lines of ama-metrics code changed** — one negated CEL clause in someone else's policy. No migration, no new deploy path, nothing new to maintain.
- Almost every incident and regression we fire-fight traces back to code someone wrote earlier. Code you *don't* write can't break, can't rot, and can't page you at 2am. Deleting the need for code is a feature.
- When the instinct is "let's build X," it's worth asking whether a config, a policy, or a one-line exception gets the same outcome with a fraction of the surface area.

**Lesson 4 — Reproduce before you fix.**

- The safe, disposable repro (flipping the classic-Automatic VAP binding from `Audit` to `Deny`) turned *"I think this is the mechanism"* into *proof*. That's what let me walk into the AKS conversation with a working demo instead of a theory.
- A repro you control also de-risks the fix: I could validate the exact CEL clause against a real deny before ever asking AKS to change production.

**Lesson 5 — A narrowly-scoped ask gets a "yes" fast.**

- I didn't ask AKS to "open up `kube-system`." I asked for one negated clause exempting ~5 named objects, inside the existing protected-namespace guardrail. Small, auditable asks clear security review in a week; broad ones stall for quarters.
- Cross-team engineering is as much about *framing the ask* as writing the fix. Make the reviewer's yes cheap.

---

## Appendix — Backup material (if asked / for deeper dive)

**Supporting docs (this branch, `zane/custom-ns`):**

| Doc | What's in it |
|---|---|
| `aks-automatic-msnp-kube-system-findings.md` | Full RCA: the VAP teardown, admission-vs-authz proof, why no role bypasses it |
| `aks-automatic-msnp-configmap-solution-options.md` | The options considered (move-out vs allowlist) and why allowlist won |
| `aks-msnp-vap-modification-asks.md` | The exact ask to AKS, options 1 / 1.5 / 2, the 5-object justification |
| `aks-msnp-vap-exception-validation.md` | End-to-end validation of what AKS shipped (test matrix P1–P7, N1–N2, E1–E2) |
| `ama-metrics-mtls-secret-usage.md` | Why the mTLS Secret is *structurally* pinned to `kube-system` (the deepest "why") |

**Likely Q&A:**

- *"Why not just move ama-metrics anyway, long-term?"* — Still an option, but it's no longer *forced*. The policy change unblocks customers today with zero migration risk; a namespace move can be a separate, deliberate decision, not a fire drill.
- *"Does allowlisting CRs in `kube-system` open a credential hole?"* — No. PodMonitor/ServiceMonitor credential Secrets resolve in the Monitor's *own* namespace (CRD schema has no cross-namespace field). We deliberately did **not** ask to allowlist arbitrary Secrets in `kube-system`.
- *"Is the exception too broad (prefix match on ConfigMaps)?"* — It's broader than my 4 exact names, but bounded to `ama-metrics-*` / `container-azm-ms-*` prefixes and still inside the protected-namespace constraint. Net: it also covers ama-logs and future CMs for free, and negative controls confirm everything else stays blocked.
- *"Deployment Safeguards was the first answer — how did you know it was wrong?"* — I enumerated the Gatekeeper policies; none targets ConfigMaps in `kube-system`. The deny came from a native VAP instead, confirmed by capturing the policy off the cluster.
