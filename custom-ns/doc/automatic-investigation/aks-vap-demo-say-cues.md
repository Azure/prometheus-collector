# Say cues — talk-track for `aks-vap-demo-diagrams.md`

> Section-aligned speaker cues for the demo. Show the matching section in `aks-vap-demo-diagrams.md` and talk over the picture — don't read these verbatim. One cue per section (§4 has two parts).

---

## 0. What is ama-metrics?

"Before the story makes sense, one thing about ama-metrics: it's Azure's *managed* Prometheus agent — we run it for the customer, inside their AKS cluster, in the `kube-system` namespace. Its job: scrape the `/metrics` endpoints on their pods and send everything to an Azure Monitor Workspace, which feeds Grafana, alerts, and dashboards. The important part for today is *how customers configure it*: they can deploy their ConfigMaps in kube-system, secrets, and a couple of custom resources — PodMonitors and ServiceMonitors. And Configmaps must lives in `kube-system`, right next to the agent. Hold onto that fact — it's one of the most important reasons that this project was initiated."

---

## 1. The project we planned — migrate ama-metrics out of kube-system

"AKS shipped a lockdown on system namespaces; customers couldn't apply ama-metrics used configmaps to `kube-system`. The project landed on my desk as a *solution*: **move ama-metrics to a different namespace.**"

---

## 2. Why "just migrate it" is a mountain

"Migration isn't one change — it's three problems stacked. **One:** we refactor our own code across every deploy mode — helm, ARM, Go, the OTel and fluent-bit configs, plus dashboards and recording/alert rules that filter on the agent's namespace. **Two:** a pile of things we *don't* own — the `aad-msi-auth-token` secret AKS-RP provisions, the token-adaptor image (which differs between AKS and Arc), retina, priority classes, network policy, pod-security capabilities, the CCP config watcher — every one needs another team to move in lockstep. **Three:** it's a breaking change for *every* customer — all their existing ConfigMaps, CRs, and rules point at `kube-system`. Multi-month, cross-team, high blast radius. And the kicker: migration was still only a *hypothesis* — nobody had confirmed we even needed it. So before building any of it, I stopped and asked one question."

---

## 3. The story spine — how I approached it

"I didn't just wait on AKS — I asked the owning team **and** ran my own root-cause in parallel. Their first answer was a pattern their other add-ons use: expose a config **CRD** so customers apply a custom resource outside `kube-system` instead of editing ConfigMaps in it — that's what the app-routing and scheduler examples both show. Honest take: I didn't chase it, because it's the same redesign-and-migrate mountain from the last slide, just wearing a CRD costume — big build, breaks every existing customer, unblocks no one today. The parallel RCA is what actually cracked it: the block is a native Validating Admission Policy. Once I had that, the fix and the buy-in were quick — a quarter of migration became a month."

---

## 4. RCA — how I proved it was the VAP

**Part 1 — the error names the culprit.**

"Read the failure. A `403 Forbidden` can come from two very different places, and telling them apart *is* the root-cause analysis. If **authorization** denied you — a role/RBAC problem — you get a plain 403 that names no policy. But look at this body: it literally names a `ValidatingAdmissionPolicy`, `aks-managed-protect-system-namespaces`. That's the fingerprint of the *other* gate — **admission**. So authorization actually **passed**; the customer is Cluster Admin. The block happens one step later, at admission. That tells me two things immediately: it's a VAP, and because it's not an authorization decision, **no Azure role can ever bypass it.**"

**Part 2 — confirm it by flipping the switch.**

"Then I confirmed it. That policy is present on *every* AKS Automatic cluster — on classic ones it just runs in Audit mode, so writes succeed and the deny is only logged. On a cluster I controlled, I changed one field on the binding, `Audit` to `Deny`, and re-applied the *exact same* ConfigMap. The identical error came back. Nothing else changed — so the VAP, and only the VAP, is the root cause. That's the proof I walked into AKS with."

---

## 5. The fix — one exempt clause, proven on the same cluster

"Same cluster, still in Deny mode from a minute ago. I appended one `matchCondition` — this negated clause that says *if it's one of these named ama-metrics ConfigMaps in `kube-system`, skip the policy and admit.* Then I re-applied the **exact same** ConfigMap that had just been rejected — and it went straight through. Everything else in `kube-system` stayed blocked. That's the entire fix: no ama-metrics code changed, nothing moves namespaces, one CEL clause."

*(on the "from PoC to what AKS shipped" diagram)* "My PoC exempted four named ConfigMaps. AKS took that same idiom and generalized it — a prefix match for the ConfigMaps so it covers ama-logs and future ones too, plus the mTLS Secret by exact name, plus the PodMonitor and ServiceMonitor CRs. Same one-clause shape, just the complete list of what's structurally pinned to `kube-system`."

---

## 6. Rollout — validated in canary, zero code on our side

"Here's how it shipped. AKS implemented the allowlist and rolled it to a **canary region** first. On a canary cluster, *we* did the validation — applied the ama-metrics ConfigMaps, the mTLS Secret, and the PodMonitor/ServiceMonitor CRs, and confirmed they all create successfully in `kube-system`, while unrelated objects stay blocked so the scope held. With that green signal, AKS continued the rollout to broader regions. And the headline: **zero code changes on ama-metrics.** We shipped a cross-cutting customer-facing fix without touching our agent at all."

---

## 7. Lessons

"If you forget everything else: keep asking *why* until you find the real problem, lean on AI to move fast outside your own domain, and remember the cheapest fix we ever ship is the one we talk ourselves out of building."

---

## Bonus — ama-logs got AKS Automatic support for free

"And a bonus we didn't even plan for. Because AKS made the ConfigMap rule a prefix match, the exact same clause that unblocked ama-metrics *also* unblocks **ama-logs** — Container Insights uses `container-azm-ms-` ConfigMaps, which fall under the same prefix. So a second addon that would have needed its own months-long migration to support AKS Automatic got it for **zero additional effort**. One narrowly-scoped fix, two products unblocked."
