# Migrate ama-metrics out of kube-system namespace — the story

> **This is the demo doc — drive the talk straight from here.** Each diagram has a one-line **`Say:`** cue beneath it: that's your talk-track. Walk top to bottom (0 → 7) and talk over each picture; don't read the cue verbatim. All diagrams are **Mermaid** (render on GitHub / VS Code / most markdown viewers).
>
> `aks-vap-demo-script.md` is optional backup only — deeper wording, the verbatim error message, and the Q&A appendix if someone digs in. You don't need it open during the demo.
>
> **Order (built for an audience new to ama-metrics):**
> 0 (what is ama-metrics) → 1 (the project) → 2 (why it's a mountain) → 3 (the story spine) → 4 (RCA — proving it's the VAP) → 5 (fix) → 6 (rollout) → 7 (lessons) → Bonus (ama-logs unblocked for free).
>
> **Color legend (consistent across every diagram):** blue = context/input · yellow = investigation/decision · green = success · red = deny/break · orange = the policy itself.

---

## 0. What is ama-metrics? (set the stage)

```mermaid
flowchart LR
    subgraph Cluster["Customer's AKS cluster"]
        direction TB
        APP["Customer pods<br/>expose <b>/metrics</b>"]
        subgraph KS["namespace: kube-system"]
            AMA["<b>ama-metrics</b><br/>Azure Managed Prometheus agent"]
            CFG["configured by:<br/>ConfigMaps · a Secret ·<br/>PodMonitor / ServiceMonitor CRs"]
            CFG -.-> AMA
        end
        APP -->|scrape| AMA
    end
    AMA ==> AMW["<b>Azure Monitor</b><br/>Workspace"]
    AMW --> USE["Grafana ·<br/>alerts ·<br/>dashboards"]

    style KS fill:#e6f0ff,stroke:#4472c4
    style AMA fill:#fff2cc,stroke:#d6b656
    style AMW fill:#d5e8d4,stroke:#82b366
    style USE fill:#d5e8d4,stroke:#82b366
```

> **Say:** "Before the story makes sense, one thing about ama-metrics: it's Azure's *managed* Prometheus agent — we run it for the customer, inside their AKS cluster, in the `kube-system` namespace. Its job: scrape the `/metrics` endpoints on their pods and send everything to an Azure Monitor Workspace, which feeds Grafana, alerts, and dashboards. The important part for today is *how customers configure it*: they can deploy their ConfigMaps in kube-system, secrets, and a couple of custom resources — PodMonitors and ServiceMonitors. And Configmaps must lives in `kube-system`, right next to the agent. Hold onto that fact — it's one of the most important reasons that this project was initiated."

---

## 1. The project we planned — migrate ama-metrics out of kube-system namespace

```mermaid
flowchart LR
    NEW["New AKS security feature:kube-system namespace is locked down"] --> BROKE["Customers can no longer<br/>deploy configmaps<br/>to <b>kube-system</b>"]
    BROKE ==> ASK["<b>The ask:</b><br/>migrate ama-metrics<br/>OUT of kube-system"]

    style NEW fill:#e6f0ff,stroke:#4472c4
    style BROKE fill:#ffcccc,stroke:#c00
    style ASK fill:#ffe6cc,stroke:#d79b00
```

> **Say:** "AKS shipped a lockdown on system namespaces; customers couldn't apply ama-metrics used configmaps to `kube-system`. The project landed on my desk as a *solution*: **move ama-metrics to a different namespace.**"

---

## 2. Why "just migrate it" is a mountain

```mermaid
flowchart TB
    ASK["<b>Move ama-metrics<br/>out of kube-system</b>"]

    subgraph OURS["1 · Our code — refactor across every deploy mode"]
        direction TB
        O1["helm charts · ARM/Bicep<br/>AKS addon · Arc · CCP config-ref"]
        O2["Go code: make POD_NAMESPACE required<br/>OTel + fluent-bit configs · TA URLs / TLS SANs"]
        O3["mixins · dashboards ·<br/>recording &amp; alert rules"]
    end

    subgraph EXT["2 · Dependencies we DON'T own — AKS-RP &amp; others"]
        direction TB
        E1["aad-msi-auth-token secret<br/>(AKS-RP must dual-provision)"]
        E2["token-adaptor image<br/>(different for AKS vs Arc) · retina"]
        E3["priority class · default-deny NetworkPolicy ·<br/>Pod Security (NET_ADMIN/NET_RAW) ·<br/>Deployment-Safeguards allowlist"]
        E4["CCP configmap-watcher<br/>(Overlay → Underlay name)"]
    end

    subgraph CX["3 · Breaking change for every customer"]
        direction TB
        C1["existing ConfigMaps in kube-system<br/>(settings · prometheus-config · node · node-windows)"]
        C2["PodMonitor / ServiceMonitor CRs ·<br/>recording &amp; alert rules"]
    end

    ASK --> OURS
    ASK --> EXT
    ASK --> CX

    OURS --> COST["<b>Multi-month · cross-team · high blast radius</b><br/>— and still just a <i>hypothesis</i> until<br/>Phase-0 validation says migration is even needed"]
    EXT --> COST
    CX --> COST

    style ASK fill:#ffe6cc,stroke:#d79b00
    style OURS fill:#e6f0ff,stroke:#4472c4
    style EXT fill:#fff2cc,stroke:#d6b656
    style CX fill:#ffe6e6,stroke:#c00
    style COST fill:#ffcccc,stroke:#c00
```

> **Say:** "Migration isn't one change — it's three problems stacked. **One:** we refactor our own code across every deploy mode — helm, ARM, Go, the OTel and fluent-bit configs, plus dashboards and recording/alert rules that filter on the agent's namespace. **Two:** a pile of things we *don't* own — the `aad-msi-auth-token` secret AKS-RP provisions, the token-adaptor image (which differs between AKS and Arc), retina, priority classes, network policy, pod-security capabilities, the CCP config watcher — every one needs another team to move in lockstep. **Three:** it's a breaking change for *every* customer — all their existing ConfigMaps, CRs, and rules point at `kube-system`. Multi-month, cross-team, high blast radius. And the kicker: migration was still only a *hypothesis* — nobody had confirmed we even needed it. So before building any of it, I stopped and asked one question."

---

## 3. The story spine — how I approached it

```mermaid
flowchart LR
    START["ama-metrics writes to<br/>kube-system start failing"]

    START --> Q["<b>Track 1 — ask the owners</b><br/>AKS Automatic team:<br/>why is kube-system<br/>locked down?"]
    START --> R["<b>Track 2 — RCA in parallel</b><br/>tear the mechanism<br/>apart myself"]

    Q --> SUG["their suggestion:<br/>expose a config <b>CRD</b><br/>(app-routing · scheduler pattern)<br/><i>= redesign · didn't fit</i>"]
    R --> B["<b>Root cause:</b> a native VAP<br/>aks-managed-protect-<br/>system-namespaces"]

    SUG -.->|"set aside"| B
    B --> D["Prototype fix<br/><b>one CEL clause</b><br/>no code, no migration"]
    D --> E["AKS buy-in"]
    E --> F["Rollout ✅"]

    style START fill:#e6f0ff,stroke:#4472c4
    style Q fill:#fff2cc,stroke:#d6b656
    style R fill:#fff2cc,stroke:#d6b656
    style SUG fill:#f2f2f2,stroke:#999
    style B fill:#ffe6cc,stroke:#d79b00
    style D fill:#d5e8d4,stroke:#82b366
    style E fill:#d5e8d4,stroke:#82b366
    style F fill:#d5e8d4,stroke:#82b366
```

**What AKS first suggested — one pattern, two examples:** *"configure a managed add-on through a **CRD** the customer applies outside `kube-system`, not through ConfigMaps in `kube-system`."*

| Their pointer | The CRD it uses | What they meant for ama-metrics |
|---|---|---|
| [App-routing NGINX config](https://learn.microsoft.com/en-us/azure/aks/app-routing-nginx-configuration?tabs=azure-cli&pivots=nginx-ingress-controller) | `NginxIngressController` — customer applies the CR, the add-on operator reads it and configures NGINX | Do the same: replace ama-metrics' ConfigMaps with a config **CRD** the customer applies in their own namespace |
| [AKS scheduler profiles](https://learn.microsoft.com/en-us/azure/aks/configure-aks-scheduler?tabs=new-cluster#limitations) | `SchedulerConfiguration` — customer applies the CR, AKS's controller configures the scheduler (system `aks-system` scheduler stays off-limits) | Same pattern again — CRD in, no writes to `kube-system` |

**Why I set it aside:** both amount to *re-architecting ama-metrics' config surface into a new CRD* — a big build, a breaking change for every customer already using the 4 ConfigMaps, and it doesn't unblock existing customers now. It's the same migration mountain from §2, just dressed as a CRD. The VAP exception does the same job with **zero code and no customer migration**.

> **Say:** "I didn't just wait on AKS — I asked the owning team **and** ran my own root-cause in parallel. Their first answer was a pattern their other add-ons use: expose a config **CRD** so customers apply a custom resource outside `kube-system` instead of editing ConfigMaps in it — that's what the app-routing and scheduler examples both show. Honest take: I didn't chase it, because it's the same redesign-and-migrate mountain from the last slide, just wearing a CRD costume — big build, breaks every existing customer, unblocks no one today. The parallel RCA is what actually cracked it: the block is a native Validating Admission Policy. Once I had that, the fix and the buy-in were quick — a quarter of migration became a month."

---

## 4. RCA — how I proved it was the VAP

**Part 1 — the error names the culprit.** On an MSNP AKS Automatic cluster, applying an ama-metrics ConfigMap to `kube-system` fails — and the error body itself points the finger:

```mermaid
flowchart LR
    U["kubectl apply<br/>ama-metrics ConfigMap<br/>-n kube-system<br/><i>(MSNP AKS Automatic)</i>"] --> Z{"Azure RBAC<br/>authorization"}
    Z ==>|"✅ YES<br/>(Cluster Admin)"| ADM{"<b>Admission</b>"}
    ADM ==>|"❌ DENY"| ERR["<b>Forbidden</b> — body NAMES the policy:<br/>ValidatingAdmissionPolicy<br/>'aks-managed-protect-system-namespaces'<br/>denied request: 'Modification of resources in<br/>managed system namespaces is not allowed'"]

    style Z fill:#d5e8d4,stroke:#82b366
    style ADM fill:#ffe6cc,stroke:#d79b00
    style ERR fill:#ffcccc,stroke:#c00
```

> **Say (Part 1):** "Read the failure. A `403 Forbidden` can come from two very different places, and telling them apart *is* the root-cause analysis. If **authorization** denied you — a role/RBAC problem — you get a plain 403 that names no policy. But look at this body: it literally names a `ValidatingAdmissionPolicy`, `aks-managed-protect-system-namespaces`. That's the fingerprint of the *other* gate — **admission**. So authorization actually **passed**; the customer is Cluster Admin. The block happens one step later, at admission. That tells me two things immediately: it's a VAP, and because it's not an authorization decision, **no Azure role can ever bypass it.**"

**Part 2 — confirm it by flipping the switch.** The same VAP ships on *classic* AKS Automatic, but in `[Audit]` mode (writes still succeed). On a cluster I could modify, I flipped one field and the failure reproduced exactly:

```mermaid
flowchart LR
    subgraph Before["Classic AKS Automatic — VAP present, [Audit]"]
        V1["binding<br/>validationActions: <b>[Audit]</b>"] --> OK["apply ConfigMap<br/>→ ✅ succeeds<br/>(deny only logged)"]
    end

    OK == "kubectl patch<br/>[Audit] → [Deny]" ==> After

    subgraph After["after the flip — [Deny]"]
        FLIP["binding<br/>validationActions: <b>[Deny]</b>"] --> SAME["apply the SAME ConfigMap<br/>→ ❌ identical Forbidden error"]
    end

    SAME --> CONF["✅ root cause<br/><b>CONFIRMED = the VAP</b>"]

    style Before fill:#e6f0ff,stroke:#4472c4
    style After fill:#fff2cc,stroke:#d6b656
    style SAME fill:#ffcccc,stroke:#c00
    style CONF fill:#d5e8d4,stroke:#82b366
```

> **Say (Part 2):** "Then I confirmed it. That policy is present on *every* AKS Automatic cluster — on classic ones it just runs in Audit mode, so writes succeed and the deny is only logged. On a cluster I controlled, I changed one field on the binding, `Audit` to `Deny`, and re-applied the *exact same* ConfigMap. The identical error came back. Nothing else changed — so the VAP, and only the VAP, is the root cause. That's the proof I walked into AKS with."

---

## 5. The fix — one exempt clause, proven on the same cluster

Still on that classic AKS Automatic cluster (binding already flipped to `[Deny]` from the RCA step), I appended **one** `matchCondition` to the VAP — a negated CEL clause that exempts the ama-metrics ConfigMaps — then re-applied the exact ConfigMap that had just failed:

```yaml
# appended to spec.matchConditions[] of aks-managed-protect-system-namespaces
- name: exempt-ama-metrics-configmaps
  expression: |
    !(request.namespace == "kube-system" &&
      request.resource.resource == "configmaps" &&
      request.name in ["ama-metrics-prometheus-config",
                       "ama-metrics-settings-configmap",
                       "ama-metrics-prometheus-config-node",
                       "ama-metrics-prometheus-config-node-windows"])
```

```mermaid
flowchart LR
    D["classic AKS Automatic<br/>binding still <b>[Deny]</b><br/><i>(from the RCA step)</i>"] --> P["kubectl patch VAP:<br/>append<br/><b>exempt-ama-metrics-configmaps</b>"]
    P --> R["re-apply the SAME<br/>ama-metrics ConfigMap"]
    R --> OK["✅ ADMITTED —<br/>ConfigMap created"]
    R -.->|"unrelated ConfigMap"| NO["❌ still denied<br/>(scope holds)"]

    style D fill:#ffe6cc,stroke:#d79b00
    style P fill:#fff2cc,stroke:#d6b656
    style OK fill:#d5e8d4,stroke:#82b366
    style NO fill:#ffcccc,stroke:#c00
```

> **Say:** "Same cluster, still in Deny mode from a minute ago. I appended one `matchCondition` — this negated clause that says *if it's one of these named ama-metrics ConfigMaps in `kube-system`, skip the policy and admit.* Then I re-applied the **exact same** ConfigMap that had just been rejected — and it went straight through. Everything else in `kube-system` stayed blocked. That's the entire fix: no ama-metrics code changed, nothing moves namespaces, one CEL clause."

**How the clause works** — it just adds one branch to the policy's decision:

```mermaid
flowchart TD
    subgraph Before["BEFORE — everything in kube-system blocked"]
        direction TB
        S1["Write → kube-system"] --> C1{"protected<br/>namespace?"}
        C1 -->|no| A1["✅ admit"]
        C1 -->|"yes"| D1["❌ deny"]
    end

    subgraph After["AFTER — one added matchCondition"]
        direction TB
        S2["Write → kube-system"] --> C2{"protected<br/>namespace?"}
        C2 -->|no| A2["✅ admit"]
        C2 -->|"yes"| C3{"ama-metrics<br/>exception?"}
        C3 -->|"no"| D2["❌ deny<br/>(everything else<br/>still blocked)"]
        C3 -->|"yes"| A2
    end

    style D1 fill:#ffcccc,stroke:#c00
    style D2 fill:#ffcccc,stroke:#c00
    style A1 fill:#d5e8d4,stroke:#82b366
    style A2 fill:#d5e8d4,stroke:#82b366
    style C3 fill:#fff2cc,stroke:#d6b656
```

**From PoC to what AKS shipped** — AKS generalized my 4-name PoC into the final exception, covering all three object types ama-metrics is forced to put in `kube-system`:

```mermaid
flowchart LR
    EX["ama-metrics<br/>exception (shipped)"] --> CM["ConfigMaps<br/>prefix <b>ama-metrics-*</b><br/>or <b>container-azm-ms-*</b>"]
    EX --> SEC["Secret<br/>exact <b>ama-metrics-mtls-secret</b>"]
    EX --> CR["CRs<br/><b>podmonitors / servicemonitors</b><br/>azmonitoring.coreos.com/v1"]

    style EX fill:#fff2cc,stroke:#d6b656
    style CM fill:#e6f0ff,stroke:#4472c4
    style SEC fill:#e6f0ff,stroke:#4472c4
    style CR fill:#e6f0ff,stroke:#4472c4
```

> **Say:** "My PoC exempted four named ConfigMaps. AKS took that same idiom and generalized it — a prefix match for the ConfigMaps so it covers ama-logs and future ones too, plus the mTLS Secret by exact name, plus the PodMonitor and ServiceMonitor CRs. Same one-clause shape, just the complete list of what's structurally pinned to `kube-system`."

---

## 6. Rollout — validated in canary, zero code on our side

```mermaid
flowchart LR
    A["AKS implements<br/>the VAP allowlist<br/>(the exempt clause)"] --> B["rolls out to<br/><b>canary region</b>"]
    B --> C["<b>we validate</b> on canary:<br/>ConfigMaps · the mTLS Secret ·<br/>PodMonitor / ServiceMonitor CRs<br/>all create successfully in kube-system"]
    C --> D["AKS <b>continues the rollout</b><br/>→ broader regions"]
    C -.->|"unrelated CM / Secret<br/>still denied"| SCOPE["scope held —<br/>exception, not a hole"]

    style A fill:#ffe6cc,stroke:#d79b00
    style B fill:#fff2cc,stroke:#d6b656
    style C fill:#e6f0ff,stroke:#4472c4
    style D fill:#d5e8d4,stroke:#82b366
    style SCOPE fill:#f2f2f2,stroke:#999
```

**✅ Zero code changes on ama-metrics** — the entire fix lived in the AKS-managed policy.

> **Say:** "Here's how it shipped. AKS implemented the allowlist and rolled it to a **canary region** first. On a canary cluster, *we* did the validation — applied the ama-metrics ConfigMaps, the mTLS Secret, and the PodMonitor/ServiceMonitor CRs, and confirmed they all create successfully in `kube-system`, while unrelated objects stay blocked so the scope held. With that green signal, AKS continued the rollout to broader regions. And the headline: **zero code changes on ama-metrics.** We shipped a cross-cutting customer-facing fix without touching our agent at all."

---

## 7. Lessons — what I'd want you to take away

1. **Work backwards from the problem, not forward from a solution.** "Migrate the addon" was a solution in disguise. Keep asking *what problem does this solve?* until you hit an actual problem — then you often find a cheaper answer.
2. **AI is a superpower outside your own domain.** I'd never heard of a Validating Admission Policy — it's really the AKS team's area, not a monitoring engineer's. With AI I learned enough to prototype the exact fix in hours, not the weeks it would've taken alone.
3. **Less code — or no code — wins.** Zero ama-metrics lines changed. Code you don't write can't break, can't rot, and can't page you at 2am.

> **Say:** "If you forget everything else: keep asking *why* until you find the real problem, lean on AI to move fast outside your own domain, and remember the cheapest fix we ever ship is the one we talk ourselves out of building."

---

## Bonus — ama-logs got AKS Automatic support for free

Because the shipped ConfigMap exception is a **prefix** match (`ama-metrics-*` **or** `container-azm-ms-*`), the same one clause that unblocked ama-metrics **also** covers ama-logs (Container Insights) — its customer ConfigMaps are `container-azm-ms-*`.

```mermaid
flowchart LR
    EX["one prefix clause<br/><b>container-azm-ms-*</b>"] --> M["ama-metrics<br/>ConfigMaps"]
    EX --> L["ama-logs<br/>container-azm-ms-agentconfig ·<br/>container-azm-ms-vpaconfig<br/>✅ admitted in kube-system"]

    style EX fill:#fff2cc,stroke:#d6b656
    style M fill:#e6f0ff,stroke:#4472c4
    style L fill:#d5e8d4,stroke:#82b366
```

| | ama-logs on AKS Automatic |
|---|---|
| **The alternative** | migrate ama-logs out of `kube-system` — its own multi-month, cross-team effort |
| **What we actually spent** | **0 extra effort** — the ama-metrics prefix clause already admits its ConfigMaps (validated on the same cluster) |

> **Say:** "And a bonus we didn't even plan for. Because AKS made the ConfigMap rule a prefix match, the exact same clause that unblocked ama-metrics *also* unblocks **ama-logs** — Container Insights uses `container-azm-ms-` ConfigMaps, which fall under the same prefix. So a second addon that would have needed its own months-long migration to support AKS Automatic got it for **zero additional effort**. One narrowly-scoped fix, two products unblocked."

---

## Appendix — quick render tips

- **GitHub / VS Code**: renders inline automatically. In VS Code use the built-in Markdown preview (`Ctrl+Shift+V`).
- **Export to image** (for a slide, if ever needed): paste a block into <https://mermaid.live> → export SVG/PNG.
- **Colors** use the classic Mermaid palette (blue = context/input, yellow = investigation/decision, green = success, red = deny/break, orange = the policy) — consistent across all diagrams so the audience learns the legend once.
