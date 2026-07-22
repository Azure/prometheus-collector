# Demo narrative — read-through for `aks-vap-demo-diagrams.md`

> ~1000 words · ~10 minutes at 120 wpm. Advance the matching diagram as you hit each numbered beat. This is a continuous talk-track — read it naturally, don't rush.

---

**(0 · What is ama-metrics)**

Quick context first, because the rest only makes sense with it. ama-metrics is Azure's *managed* Prometheus agent. We run it for the customer, inside their AKS cluster, in the `kube-system` namespace. Its job: scrape the `/metrics` endpoints on their pods and ship everything to an Azure Monitor Workspace, which feeds Grafana, alerts, and dashboards. The detail that matters today is *how customers configure it*: they hand us ConfigMaps, a Secret, and two custom resources — PodMonitors and ServiceMonitors. And those ConfigMaps have to live in `kube-system`, right next to the agent. Hold onto that — it's the reason this project started.

**(1 · The project we planned)**

Here's what kicked it off. AKS shipped a security feature that locks down system namespaces — `kube-system` included. Overnight, customers could no longer apply their ama-metrics ConfigMaps there. So the project landed on my desk already framed as a solution: *move ama-metrics out of `kube-system` into its own namespace.* That was the ask.

**(2 · Why "just migrate it" is a mountain)**

But "just migrate it" isn't one change — it's three problems stacked. **One:** we'd refactor our own code across every deployment mode — helm, ARM/Bicep, the Go code, the OTel and fluent-bit configs, plus dashboards and recording rules that filter on the agent's namespace. **Two:** a pile of things we don't own — the auth-token secret AKS-RP provisions, the token-adaptor image (different for AKS versus Arc), retina, priority classes, network policy, pod-security capabilities, the CCP config watcher — every one needs another team moving in lockstep. **Three:** it's a breaking change for *every* existing customer — all their ConfigMaps, custom resources, and rules point at `kube-system`. Multi-month, cross-team, high blast radius. And the kicker: migration was still just a *hypothesis*. So before building any of it, I asked one question — *why* is `kube-system` locked down, and can we get around it cheaply?

**(3 · The story spine)**

I ran two tracks in parallel. Track one: ask the owners — the AKS Automatic team — why the lockdown exists and what they'd recommend. Track two: don't wait, tear the mechanism apart myself. Their answer on track one was a pattern their other add-ons use: expose a config **CRD**, so customers apply a custom resource *outside* `kube-system` instead of editing ConfigMaps inside it — what their app-routing and scheduler examples do. I didn't chase it: it's the same redesign-and-migrate mountain in a CRD costume — big build, breaks every existing customer, unblocks nobody today. Track two is what actually cracked it.

**(4 · RCA — proving it's the VAP)**

Two parts here. **Part one — read the error.** Applying an ama-metrics ConfigMap to `kube-system` gives you a `403 Forbidden` — and a 403 can come from two very different places, so telling them apart *is* the root-cause analysis. If **authorization** denied you — an RBAC problem — you get a plain 403 that names no policy. But this error body literally names a `ValidatingAdmissionPolicy`, `aks-managed-protect-system-namespaces`. That's the fingerprint of the *other* gate — admission. So authorization actually *passed*; the customer is Cluster Admin. Two things follow immediately: it's a VAP, and because it isn't an authorization decision, *no Azure role will ever bypass it.*

**Part two — confirm it.** That same policy ships on classic AKS Automatic too, but in Audit mode — writes succeed, the deny is only logged. On a cluster I controlled, I flipped one field on the binding — Audit to Deny — and re-applied the same ConfigMap. The identical error came back. Nothing else changed, so the VAP, and only the VAP, is the root cause. That's the proof I walked into AKS with.

**Part three — the finding that killed the migration idea.** While in there, I applied a ConfigMap to a *different* AKS-managed namespace — `azuresecuritylinuxagent`. Failed too, same policy. So the lockdown isn't about `kube-system` specifically; it protects *every* namespace AKS manages. That reframes the whole ask: moving out was never a solution — only a prerequisite. Think it through. Move to a namespace AKS *manages*, and we're still blocked — we'd need an exception anyway, which is exactly what we did without moving. Move to an *unmanaged* one, and we lose everything that made a managed namespace worth it — security, priority class, autoscaling. Either way, migration doesn't solve the problem. The exception does.

**(5 · The fix)**

And the fix is almost anticlimactic. Same cluster, still in Deny mode. I appended one `matchCondition` — a negated CEL clause that says: *if it's one of these named ama-metrics ConfigMaps in `kube-system`, skip the policy and admit.* Re-applied the exact ConfigMap that had just been rejected, and it went straight through — while everything else in `kube-system` stayed blocked. That's the whole fix: no ama-metrics code changed, nothing moves namespaces, one clause. AKS then generalized my four-name PoC — a prefix match on the ConfigMaps, the mTLS Secret by exact name, and the two custom resources.

**(6 · Rollout)**

Here's how it shipped. AKS implemented the allowlist and rolled it to a canary region first. On a canary cluster, *we* validated — applied the ConfigMaps, the Secret, and the custom resources, confirmed they all create in `kube-system` while unrelated objects stayed blocked, so the scope held. Green signal, and AKS continued the rollout to broader regions. The headline: *zero code changes on ama-metrics* — a cross-cutting, customer-facing fix without touching our agent at all.

**(7 · Lessons)**

Three takeaways. First — work backwards from the problem, not forward from a solution. "Migrate the addon" was a solution in disguise; keep asking *what problem does this actually solve* until you hit a real one, and you often find a cheaper answer. Second — AI is a superpower outside your own domain. I'd never touched a Validating Admission Policy — it's the AKS team's turf — but with AI I prototyped the exact fix in hours instead of weeks. Third — less code, or no code, wins. Code you don't write can't break, can't rot, and can't page you at 2am.

**(Bonus · ama-logs for free)**

And one bonus. While we were at it, I asked AKS to also allowlist the `container-azm-ms-` ConfigMap prefix that ama-logs — Container Insights — uses, and they added it in the same clause. So for basically zero extra effort, a second product that would've needed its *own* months-long migration to support AKS Automatic got unblocked in the same stroke. One narrowly-scoped fix, two addons supported. Thank you.
