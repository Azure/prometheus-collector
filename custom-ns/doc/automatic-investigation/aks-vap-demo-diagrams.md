# Demo diagrams: the AKS `kube-system` lockdown story

> Visual companion to `aks-vap-demo-script.md`. All diagrams are **Mermaid** (render on GitHub / VS Code / most markdown viewers). One line of speaker cue under each — talk over the picture, don't read it.
>
> **Suggested order:** 1 (spine) → 2 (reframe) → 3 (money diagram) → 4 (fix) → 5 (reproduce) → 6 (results).

---

## 1. The story spine — how I approached it

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

> **Say:** "Six steps. The whole thing turned on step 2 — finding the *actual* mechanism — which made steps 3–6 cheap."

---

## 2. The reframe — solution vs problem

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

## 3. The money diagram — WHERE the block happens

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

## 4. The fix — VAP decision tree, before vs after

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

## 5. How I reproduced it safely (the PoC lever)

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

## 6. Validation of what AKS shipped

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
- **Colors** use the classic Mermaid palette (blue = input/context, yellow = investigation/decision, green = success, red = deny) — consistent across all six diagrams so the audience learns the legend once.
