# Demo diagrams: the AKS `kube-system` lockdown story

> Visual companion to `aks-vap-demo-script.md`. All diagrams are **Mermaid** (render on GitHub / VS Code / most markdown viewers). One line of speaker cue under each — talk over the picture, don't read it.
>
> **Suggested order (built for an audience new to ama-metrics):**
> 0 (what is ama-metrics) → 1 (the project) → 2 (why it's a mountain) → 3 (the story spine) → 4 (reframe) → 5 (money diagram) → 6 (fix) → 7 (reproduce) → 8 (validation).
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

> **Say:** "ama-metrics is Azure's managed Prometheus agent. It runs *inside* the customer's cluster — in `kube-system` — scrapes their pods, and ships metrics to an Azure Monitor Workspace. Customers steer it with ConfigMaps, a Secret, and a couple of custom resources. Remember that last part: **all of that config lives in `kube-system`.**"

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

> **Say:** "This is the whole insight. **RBAC says YES.** The deny happens *later*, at admission. So no Azure role — not even a custom one — can bypass it. The fix has to live in the policy."

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

## Appendix — quick render tips

- **GitHub / VS Code**: renders inline automatically. In VS Code use the built-in Markdown preview (`Ctrl+Shift+V`).
- **Export to image** (for a slide, if ever needed): paste a block into <https://mermaid.live> → export SVG/PNG.
- **Colors** use the classic Mermaid palette (blue = context/input, yellow = investigation/decision, green = success, red = deny/break, orange = the policy) — consistent across all nine diagrams so the audience learns the legend once.
