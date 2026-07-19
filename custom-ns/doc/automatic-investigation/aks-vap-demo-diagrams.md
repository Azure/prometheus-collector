# Demo diagrams: the AKS `kube-system` lockdown story

> **This is the demo doc — drive the talk straight from here.** Each diagram has a one-line **`Say:`** cue beneath it: that's your talk-track. Walk top to bottom (0 → 9) and talk over each picture; don't read the cue verbatim. All diagrams are **Mermaid** (render on GitHub / VS Code / most markdown viewers).
>
> `aks-vap-demo-script.md` is optional backup only — deeper wording, the verbatim error message, and the Q&A appendix if someone digs in. You don't need it open during the demo.
>
> **Order (built for an audience new to ama-metrics):**
> 0 (what is ama-metrics) → 1 (the project) → 2 (why it's a mountain) → 3 (the story spine) → 4 (reframe) → 5 (money diagram) → 6 (fix) → 7 (reproduce) → 8 (validation) → 9 (lessons).
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
    AMA ==>|remote-write| AMW["<b>Azure Monitor</b><br/>Workspace"]
    AMW --> USE["Grafana ·<br/>alerts ·<br/>dashboards"]

    style KS fill:#e6f0ff,stroke:#4472c4
    style AMA fill:#fff2cc,stroke:#d6b656
    style AMW fill:#d5e8d4,stroke:#82b366
    style USE fill:#d5e8d4,stroke:#82b366
```

> **Say:** "Before the story makes sense, one thing about ama-metrics: it's Azure's *managed* Prometheus agent — we run it for the customer, inside their AKS cluster, in the `kube-system` namespace. Its job is simple: scrape the `/metrics` endpoints on their pods and remote-write everything to an Azure Monitor Workspace, which feeds Grafana, alerts, and dashboards. The important part for today is *how customers configure it*: they hand us ConfigMaps, one Secret, and a couple of custom resources — PodMonitors and ServiceMonitors. And every bit of that config lives in `kube-system`, right next to the agent. Hold onto that fact — it's the whole reason this problem is hard."

---

## 1. The project — as it was handed to me

```mermaid
flowchart LR
    NEW["New AKS security feature:<br/><b>MSNP</b> — managed system<br/>namespaces are locked down"] --> BROKE["Customers can no longer<br/>write ama-metrics config<br/>to <b>kube-system</b>"]
    BROKE ==> ASK["<b>The ask:</b><br/>migrate ama-metrics<br/>OUT of kube-system"]

    style NEW fill:#e6f0ff,stroke:#4472c4
    style BROKE fill:#ffcccc,stroke:#c00
    style ASK fill:#ffe6cc,stroke:#d79b00
```

> **Say:** "AKS shipped a lockdown on system namespaces. Overnight, customers couldn't apply ama-metrics config to `kube-system`. The project landed on my desk as a *solution*: **move ama-metrics to a different namespace.** Sounds reasonable — until you look at what that actually costs."

---

## 2. Why "just migrate it" is a mountain

```mermaid
flowchart TB
    ASK["<b>Move ama-metrics<br/>out of kube-system</b>"]

    ASK --> UP["<b>Upstream</b> we own<br/>helm charts · ARM/Bicep ·<br/>3 deploy modes:<br/>AKS addon · Arc · CCP"]
    ASK --> DOWN["<b>Downstream</b> customers own<br/>every ConfigMap, the Secret &<br/>every PodMonitor/ServiceMonitor<br/>references <b>kube-system</b>"]

    DOWN --> BREAK["<b>Breaking change</b><br/>all existing customer config<br/>stops working"]

    UP --> COST["Multi-month · high-risk ·<br/>coordinated migration<br/><b>for every customer</b>"]
    BREAK --> COST

    style ASK fill:#ffe6cc,stroke:#d79b00
    style UP fill:#e6f0ff,stroke:#4472c4
    style DOWN fill:#e6f0ff,stroke:#4472c4
    style BREAK fill:#ffcccc,stroke:#c00
    style COST fill:#ffcccc,stroke:#c00
```

> **Say:** "It touches everything we ship — three deploy modes, helm, ARM. Worse, it's a **breaking change for customers**: every ConfigMap, Secret, and CR they've ever written points at `kube-system`. Migrating the agent means migrating *all of them*. That's a multi-month, high-risk fire drill. So before building any of it, I stopped and asked one question."

---

## 3. The story spine — how I approached it

```mermaid
flowchart LR
    A["Ask AKS<br/><b>why is kube-system<br/>locked down?</b>"] --> B["RCA<br/><b>it's a native VAP</b><br/>not Deployment Safeguards"]
    B --> C["Reproduce<br/><b>flip binding</b><br/>Audit → Deny"]
    C --> D["Prototype fix<br/><b>one CEL clause</b><br/>no code, no migration"]
    D --> E["AKS buy-in"]
    E --> F["Rollout ✅"]

    style A fill:#e6f0ff,stroke:#4472c4
    style B fill:#fff2cc,stroke:#d6b656
    style C fill:#fff2cc,stroke:#d6b656
    style D fill:#d5e8d4,stroke:#82b366
    style E fill:#d5e8d4,stroke:#82b366
    style F fill:#d5e8d4,stroke:#82b366
```

> **Say:** "Six steps. The whole thing turned on step 2 — finding the *actual* mechanism — which made steps 3–6 cheap. Instead of a quarter of migration, it became a month."

---

## 4. The reframe — solution vs problem

```mermaid
flowchart TB
    subgraph Proposed["❌ How it was handed to me (a solution)"]
        direction TB
        P1["<b>Migrate ama-metrics<br/>out of kube-system</b>"]
        P2["Multi-month effort · high risk"]
        P3["Touches every deploy mode:<br/>AKS addon · Arc · CCP"]
        P1 --> P2 --> P3
    end

    subgraph Real["✅ The actual problem (one sentence)"]
        direction TB
        R1["<b>Customer can't write</b><br/>ama-metrics ConfigMaps / Secret / CRs<br/>to kube-system on MSNP clusters"]
    end

    Proposed == "ask: what problem<br/>does this solve?" ==> Real

    style Proposed fill:#ffe6e6,stroke:#c00
    style Real fill:#e6ffe6,stroke:#080
```

> **Say:** "'Migrate the addon' *felt* like the problem. It was a solution in disguise. The real problem is one sentence — and it has cheaper answers."

---

## 5. The money diagram — WHERE the block happens

```mermaid
flowchart LR
    U["Customer<br/>kubectl apply configmap<br/>-n kube-system"] --> N["AuthN<br/>Entra ID"]
    N --> Z{"AuthZ<br/>Azure RBAC"}
    Z -->|"❌ no"| RJ["403 — role problem"]
    Z ==>|"✅ YES<br/>(Cluster Admin)"| ADM{"<b>Admission</b><br/>VAP<br/>aks-managed-protect-<br/>system-namespaces"}
    ADM ==>|"❌ DENY"| MSG["<b>Forbidden</b><br/>'Modification of resources in<br/>managed system namespaces<br/>is not allowed'"]
    ADM -->|allow| OK["Written to etcd"]

    style Z fill:#d5e8d4,stroke:#82b366
    style ADM fill:#ffe6cc,stroke:#d79b00
    style MSG fill:#ffcccc,stroke:#c00
```

> **Say:** "This one diagram is the whole insight, so let me slow down. When a customer runs `kubectl apply`, the request goes through two gates. First **authentication** — who are you — then **authorization**, Azure RBAC — are you allowed. And here's the twist: **RBAC says YES.** The customer is Cluster Admin; by every permission check, they're allowed to write this ConfigMap. But there's a *third* gate they don't see — **admission** — and that's where a Validating Admission Policy steps in and says *no, not in a protected namespace*. Because the deny lives at admission, **not** authorization, there is no Azure role — not even a hand-crafted custom one — that can grant your way past it. That's the key realization: the fix cannot be a permissions change. It has to live in the policy itself. And that completely changes what the right solution is."

---

## 6. The fix — VAP decision tree, before vs after

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

**The exception (one CEL clause) exempts only these:**

```mermaid
flowchart LR
    EX["ama-metrics<br/>exception"] --> CM["ConfigMaps<br/>prefix <b>ama-metrics-*</b><br/>or <b>container-azm-ms-*</b>"]
    EX --> SEC["Secret<br/>exact <b>ama-metrics-mtls-secret</b>"]
    EX --> CR["CRs<br/><b>podmonitors / servicemonitors</b><br/>azmonitoring.coreos.com/v1"]

    style EX fill:#fff2cc,stroke:#d6b656
    style CM fill:#e6f0ff,stroke:#4472c4
    style SEC fill:#e6f0ff,stroke:#4472c4
    style CR fill:#e6f0ff,stroke:#4472c4
```

> **Say:** "The fix is one negated clause: *if it's one of these specific objects, short-circuit and admit.* Zero code change in ama-metrics. Nothing moves namespaces."

---

## 7. How I reproduced it safely (the PoC lever)

```mermaid
flowchart LR
    subgraph Classic["Classic AKS Automatic — no MSNP"]
        V1["Same VAP<br/>already present"] --> B1["Binding: <b>[Audit]</b><br/>writes still succeed"]
    end

    subgraph Lab["My disposable PoC lab"]
        V2["Same VAP"] --> B2["Binding: <b>[Deny]</b><br/>reproduces MSNP<br/>failure exactly"]
    end

    B1 == "kubectl patch<br/>validationActions: [Deny]" ==> B2

    style Classic fill:#e6f0ff,stroke:#4472c4
    style Lab fill:#fff2cc,stroke:#d6b656
    style B2 fill:#ffe6cc,stroke:#d79b00
```

> **Say:** "The same policy ships on classic AKS Automatic in *Audit* mode. Flip one field to *Deny* and I've got a safe lab that behaves exactly like a real MSNP customer — no need to touch production."

---

## 8. Validation of what AKS shipped

```mermaid
flowchart TD
    T["On-cluster test matrix<br/>(trang-hosted-eastus2euap, MSNP, K8s 1.35.5)"]

    T --> POS["<b>Allowed cases</b> P1–P7"]
    T --> NEG["<b>Negative controls</b> N1–N2"]
    T --> EDGE["<b>Edge proofs</b> E1–E2"]

    POS --> POSR["✅ ama-metrics CMs · mtls-secret ·<br/>PodMonitor · ServiceMonitor<br/>→ ADMITTED"]
    NEG --> NEGR["✅ unrelated CM / Secret<br/>→ STILL DENIED<br/>(scope held)"]
    EDGE --> EDGER["✅ keys on metadata.name,<br/>not filename"]

    style POSR fill:#d5e8d4,stroke:#82b366
    style NEGR fill:#d5e8d4,stroke:#82b366
    style EDGER fill:#d5e8d4,stroke:#82b366
    style T fill:#e6f0ff,stroke:#4472c4
```

> **Say:** "Every allowed object goes through; every unrelated object is still blocked. The exception is *scoped*, not a hole. AKS is rolling this out now."

---

## 9. Lessons — what I'd want you to take away

```mermaid
flowchart TB
    L1["<b>1 · Work backwards from the problem</b><br/>'Migrate the addon' was a solution<br/>in disguise — ask <i>what problem<br/>does this solve?</i> until you hit one"]
    L2["<b>2 · AI is a superpower outside your domain</b><br/>I'd never heard of a VAP — AI collapsed<br/>the time to become dangerous in<br/>admission-control, someone else's turf"]
    L3["<b>3 · Less code — or no code — wins</b><br/>Zero ama-metrics lines changed.<br/>Code you don't write can't break,<br/>rot, or page you at 2am"]
    L4["<b>4 · Reproduce before you fix</b><br/>The Audit→Deny lab turned a theory<br/>into proof — walked into AKS with a<br/>working demo, not an opinion"]
    L5["<b>5 · A narrow ask gets a fast yes</b><br/>One CEL clause over ~5 named objects,<br/>not 'open kube-system' — small,<br/>auditable asks clear review in a week"]

    L1 --- L2 --- L3 --- L4 --- L5

    style L1 fill:#e6f0ff,stroke:#4472c4
    style L2 fill:#e6f0ff,stroke:#4472c4
    style L3 fill:#d5e8d4,stroke:#82b366
    style L4 fill:#fff2cc,stroke:#d6b656
    style L5 fill:#fff2cc,stroke:#d6b656
```

> **Say:** "If you forget everything else: the cheapest fix we ever ship is the one we talk ourselves out of building. RCA first, reproduce to prove it, then make the ask small enough that the answer is yes."

---

## Appendix — quick render tips

- **GitHub / VS Code**: renders inline automatically. In VS Code use the built-in Markdown preview (`Ctrl+Shift+V`).
- **Export to image** (for a slide, if ever needed): paste a block into <https://mermaid.live> → export SVG/PNG.
- **Colors** use the classic Mermaid palette (blue = context/input, yellow = investigation/decision, green = success, red = deny/break, orange = the policy) — consistent across all ten diagrams so the audience learns the legend once.
