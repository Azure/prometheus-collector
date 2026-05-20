# AKS Automatic with Managed System Node Pools (MSNP) — `kube-system` is locked down

> **Investigation date:** 2026-05-15 (original) / **amended 2026-05-19** (see "What changed since the original investigation" below)
> **Cluster under test:** `zane-auto-msnp` (RG `zane-rg-auto-msnp`, sub `9c17527c-af8f-4148-8019-27bada0845f7`, region `westus2`)
> **Identity tested:** `zanejohnson@microsoft.com` (Owner @ subscription + `Azure Kubernetes Service RBAC Cluster Admin` @ cluster)
> **Companion doc:** [`aks-automatic-kube-system-configmap-findings.md`](./aks-automatic-kube-system-configmap-findings.md) — original investigation on a *non-MSNP* AKS Automatic cluster.

---

## What changed since the original investigation (2026-05-19)

The original 2026-05-15 framing of this doc said the VAP `aks-managed-protect-system-namespaces` is **MSNP-specific** (didn't exist on classic AKS Automatic). **That's wrong** — re-testing on 2026-05-19 shows:

| Cluster | Created | MSNP? | VAP present? | Binding `validationActions` | Customer write to `kube-system` |
|---|---|---|---|---|---|
| `zane-auto` (original "no MSNP") | < 2026-05-05 | ❌ no | ✅ **yes**, since 2026-05-05 22:57 UTC | **`[Audit]`** | ✅ Succeeds (silently logged in audit) |
| `zane-auto-2` (re-confirmation) | 2026-05-19 | ❌ no | ✅ yes, since 2026-05-19 21:58 UTC | **`[Audit]`** | ✅ Succeeds (silently logged) |
| `zane-auto-msnp` (this doc) | 2026-05-15 | ✅ yes | ✅ yes, since 2026-05-15 21:07 UTC | **`[Deny]`** | ❌ Forbidden |

**Corrected model:**

- **The VAP itself is present on every AKS Automatic cluster** (not just MSNP). AKS has been rolling it out since at least 2026-05-05.
- **The only MSNP-specific bit is `validationActions: [Deny]`** vs. `[Audit]` on classic AKS Automatic.
- On classic AKS Automatic the policy still evaluates `expression: "false"` on every write to one of the 20 protected namespaces — the API server just records it instead of rejecting it. Customer writes still succeed today.
- **This is fragile.** AKS could change non-MSNP from `[Audit]` to `[Deny]` at any time via the same managed channel that ships the VAP; the customer-visible effect would be instant and identical to what MSNP already does.

The rest of this doc — the 5-gate evaluation pipeline, the 20-namespace list, the exempt callers, the doc-discrepancy about ama-metrics Deployments on system-surge, the inventory of pods, etc. — is **still correct**. The only thing changed is "VAP exists vs. doesn't" → "VAP exists everywhere; binding is `[Audit]` vs. `[Deny]`."

---

## TL;DR

**On AKS Automatic with the new managed system node pools (MSNP) preview, customer identities — including subscription `Owner` + cluster-scoped `Azure Kubernetes Service RBAC Cluster Admin` — can no longer create, update, or delete resources in `kube-system` *or in any of 19 other AKS-managed namespaces*.**

The protection is **not `kube-system`-only**. The same VAP fires identically across all 20 namespaces in its `namespaceSelector` **protected-namespace list** (incl. `gatekeeper-system`, `app-routing-system`, `azuresecuritylinuxagent`, `aks-istio-system`, `flux-system`, `dapr-system`, `azureml`, …) — see §5.1 for the verified per-namespace test.

The block is enforced by an in-tree Kubernetes **`ValidatingAdmissionPolicy`** named `aks-managed-protect-system-namespaces`, not Gatekeeper. RBAC still says `yes` for the same operation; the deny happens at admission, *after* authorization succeeds.

**The VAP is present on classic AKS Automatic clusters too**, but its binding runs in `[Audit]` mode there — writes succeed but are silently logged. MSNP flips the binding to `[Deny]`. See "What changed since the original investigation" above and the [previous investigation doc](./aks-automatic-kube-system-configmap-findings.md) for the cross-cluster comparison.

| Cluster mode | VAP present? | Binding `validationActions` | Cluster Admin → write `kube-system` configmap? |
|---|---|---|---|
| AKS Standard | ❌ no | n/a | ✅ Yes |
| AKS Automatic (no MSNP — `zane-auto`, `zane-auto-2`) | ✅ yes | **`[Audit]`** | ✅ Yes (silently logged) |
| **AKS Automatic + MSNP (`zane-auto-msnp`)** | ✅ yes | **`[Deny]`** | ❌ **No — blocked at admission** |

---

## 1. Cluster facts

| Property | Value |
|---|---|
| Name | `zane-auto-msnp` |
| Resource group | `zane-rg-auto-msnp` |
| Subscription | `9c17527c-af8f-4148-8019-27bada0845f7` |
| Region | `westus2` |
| SKU | `Automatic` (tier `Standard`) |
| `hostedSystemProfile.enabled` | **`true`** ← MSNP signal |
| FQDN | `zane-auto--zane-rg-auto-msn-9c1752-nkzio8dw.hcp.westus2.azmk8s.io` |

How to detect MSNP from outside the cluster:

```bash
az aks show -g zane-rg-auto-msnp -n zane-auto-msnp \
  --query "hostedSystemProfile" -o json
# { "enabled": true, "nodeSubnetId": null, "systemNodeSubnetId": null }
```

How to detect from inside the cluster:

```bash
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces
# present  → MSNP cluster
# NotFound → classic AKS Automatic or AKS Standard
```

`az aks nodepool list` returns an **empty table** on MSNP — managed system pools are intentionally hidden from the agent-pool API ([documented behavior](https://learn.microsoft.com/azure/aks/automatic/aks-automatic-managed-system-node-pools-about)).

### Node pool topology (verified)

The same cluster shows two completely different views depending on whether you ask the ARM control plane or the Kubernetes API:

**ARM view — 0 visible pools**

```bash
$ az aks nodepool list -g zane-rg-auto-msnp --cluster-name zane-auto-msnp -o table
(empty — only the extension banner prints)

$ az aks show -n zane-auto-msnp ... --query agentPoolProfiles
null
```

`agentPoolProfiles` is literally `null`. From the customer subscription's perspective the cluster has zero managed pools.

**Cluster view — 5 nodes, 2 logical pools**

```text
$ kubectl get nodes -L kubernetes.azure.com/agentpool,kubernetes.azure.com/mode,karpenter.sh/nodepool
NAME                           AGE   AGENTPOOL    MODE     KARPENTER-NODEPOOL
aks-hostedpool-24979463-vms1   61m   hostedpool   system
aks-hostedpool-24979463-vms2   61m   hostedpool   system
aks-hostedpool-24979463-vms3   61m   hostedpool   system
aks-system-surge-m8hcb         59m                system   system-surge
aks-system-surge-rgfkb         59m                system   system-surge
```

| Pool | Nodes | What it is |
|---|---:|---|
| **`hostedpool`** | 3 | AKS-managed system node pool. Runs `coredns`, `keda-operator`, `vpa-*`, `metrics-server`, `konnectivity-agent`, `azure-wi-webhook`, `eraser-controller-manager`, etc. Owned by AKS; **not billed to the customer subscription**. |
| **`system-surge`** | 2 | NAP-provisioned pool (`karpenter.sh/nodepool=system-surge`). Per the [overview doc](https://learn.microsoft.com/azure/aks/automatic/aks-automatic-managed-system-node-pools-about): *"Other add-ons and extensions run on an `aks-system-surge` node, with scaling handled by node auto-provisioning."* This is where customer-side **Deployments** of addons land (e.g. `ama-metrics`, `ama-metrics-ksm`, `ama-metrics-operator-targets`). |

Both pools have `MODE=system`, so customer workloads still need to be scheduled to a future user-created pool. Modifying either pool — labeling, draining, cordoning — is blocked by the sibling `aks-managed-protect-hobo-vm-nodes` VAP listed in §4.

#### Where ama-metrics components land (verified on `zane-auto-msnp`)

`ama-metrics-node-*` is **not a single DaemonSet** — it's a family of **7 sized DaemonSet variants**, mutually exclusive via node-affinity on instance type. Each node matches exactly one bucket, so `sum(desired across all variants) ≡ nodeCount`. Coverage is 100% but spread across multiple DaemonSets.

**Per-node inventory after the live customer-app test (6 nodes total):**

| Node | Pool / NAP | SKU | vCPU / RAM | `ama-metrics-*` pods on this node |
|---|---|---|---:|---|
| `aks-hostedpool-24979463-vms1` | `hostedpool` (AKS-managed, system) | `Standard_D4d_v4` | 4 / 16 GiB | `ama-metrics-node-wdzxk` |
| `aks-hostedpool-24979463-vms2` | `hostedpool` (AKS-managed, system) | `Standard_D4d_v5` | 4 / 16 GiB | `ama-metrics-node-p2djd` |
| `aks-hostedpool-24979463-vms3` | `hostedpool` (AKS-managed, system) | `Standard_D4lds_v5` | 4 / 8 GiB  | `ama-metrics-node-dc78s` |
| `aks-system-surge-m8hcb` | NAP `system-surge` (system) | `Standard_D2als_v6` | 2 / 4 GiB  | `ama-metrics-node-xs-bw6n6` **+** `ama-metrics-7577479d6d-497pc` (Deployment replica 1) |
| `aks-system-surge-rgfkb` | NAP `system-surge` (system) | `Standard_D2als_v6` | 2 / 4 GiB  | `ama-metrics-node-xs-46dg8` **+** `ama-metrics-7577479d6d-tbxrm` (Deployment replica 2) **+** `ama-metrics-ksm-…` **+** `ama-metrics-operator-targets-…` |
| `aks-default-kw7ds` | NAP `default` (**user**) | `Standard_D4as_v6` | 4 / 16 GiB | `ama-metrics-node-s-wwfnk` |

**Sized-DaemonSet bookkeeping** (`kubectl -n kube-system get ds | grep ama-metrics`):

```text
NAME                   DESIRED  SCHEDULES ON
ama-metrics-node          3     4-vCPU "large" hostedpool nodes (D4d_v4/v5, D4lds_v5)
ama-metrics-node-xs       2     tiny 2-vCPU/4 GiB (D2als_v6 → system-surge)
ama-metrics-node-s        1     4-vCPU/16 GiB user nodes (D4as_v6 → default NAP pool)
ama-metrics-node-m        0     no matching node SKU
ama-metrics-node-l        0     no matching node SKU
ama-metrics-node-xl       0     no matching node SKU
ama-metrics-win-node      0     no Windows nodes (MSNP forbids them)
                          ──
                     total 6  ≡ 6 cluster nodes ✓
```

**Deployment-style components (1–2 replicas, not DaemonSets):**

| Pod | Lands on | Why only on `system-surge` |
|---|---|---|
| `ama-metrics-7577479d6d-{497pc,tbxrm}` (HA pair) | `aks-system-surge-{m8hcb,rgfkb}` | Tolerates `CriticalAddonsOnly=:NoSchedule` only — does NOT tolerate `kubernetes.azure.com/hostedvm`, so cannot land on `hostedpool`. Has no DaemonSet semantics, so it does not chase user nodes either. |
| `ama-metrics-ksm-…` | `aks-system-surge-rgfkb` | Same toleration story. |
| `ama-metrics-operator-targets-…` | `aks-system-surge-rgfkb` | Same toleration story. |

**Bottom line:**

- **DaemonSet coverage is complete:** every node (system pools *and* the NAP-grown user node) has exactly one `ama-metrics-node-*` pod.
- **Deployment coverage is `system-surge`-only:** the 4 cluster-scope pods cluster on the 2 NAP `system-surge` nodes.
- **NAP elasticity is automatic:** when NAP grows another user node, the matching sized variant gets `desired += 1` and a new pod lands on it within ~30 s of `Ready`. No operator action.

Implication for support / debugging: `kubectl exec` against any `ama-metrics-node-*` pod will be denied by `aks-managed-protect-interactive-access` *because the namespace is `kube-system`*, regardless of which pool the underlying node lives in. The VAP keys off namespace, not node pool.

Summary:

| Layer | Pool count | Notes |
|---|---:|---|
| `az aks nodepool list` | **0** | MSNP hides everything |
| `kubectl get nodes` | **5 nodes / 2 logical pools** | `hostedpool` (3) + `system-surge` (2) |
| Pools the customer can create/modify | **0 system pools**, any future user pools | User pools would show up in `az aks nodepool list` |

#### Can customer apps run on these nodes? — No, NAP grows a separate user pool

**Direct answer:** No. Both the `hostedpool` and `aks-system-surge` nodes are gated against customer workloads at two independent layers:

**Layer 1 — node-side taints**

| Node | Pool | Taints |
|---|---|---|
| `aks-hostedpool-…vms{1,2,3}` | `hostedpool` (AKS-managed) | `CriticalAddonsOnly=true:NoSchedule` + `CriticalAddonsOnly=true:NoExecute` + `kubernetes.azure.com/hostedvm=true:NoSchedule` |
| `aks-system-surge-{m8hcb,rgfkb}` | `system-surge` (NAP) | `CriticalAddonsOnly=true:NoSchedule` |

`CriticalAddonsOnly` is the standard Kubernetes convention for system-only nodes — no normal pod tolerates it. `hostedpool` adds `NoExecute` (evicts already-running non-tolerating pods) plus `kubernetes.azure.com/hostedvm` (an AKS-private toleration key).

**Layer 2 — admission still denies "I'll just add a toleration"**

The MSNP overview doc lists this as a hard restriction:

> Workload placement on managed system nodes — Scheduling or running customer workloads on AKS-managed system nodes, including workloads with **reserved tolerations, broad wildcard tolerations, or custom schedulers**.

Enforced by sibling VAPs already enumerated in §4: `aks-managed-critical-addons-only`, `aks-managed-custom-scheduler`, `aks-managed-protect-hobo-vm-nodes`. So "tolerate around it" is closed off at admission, before the scheduler ever sees the pod.

**What customers actually get: NAP provisions a brand-new user pool on demand.** The cluster ships with two NAP `NodePool` CRDs:

```text
$ kubectl get nodepools.karpenter.sh
NAME           NODES   READY   AGE
default        0       True    70m   ← provisions USER nodes when needed
system-surge   2       True    70m   ← already runs the addon Deployments
```

**Verified empirically on `zane-auto-msnp` (2026-05-15):**

```bash
$ kubectl create ns hello-test
$ kubectl apply -f hello.yaml   # nginx-style Deployment, 1 replica, no nodeSelector/toleration
deployment.apps/hello created

# t=0s
NAME                    READY   STATUS    NODE
hello-765498655-9xr6q   0/1     Pending   <none>

NAME            TYPE   CAPACITY   ZONE   NODE   READY     AGE
default-kw7ds                                   Unknown   3s     ← NAP fired immediately

# t=118s
NAME                    READY   STATUS    NODE
hello-765498655-9xr6q   1/1     Running   aks-default-kw7ds

NAME                           STATUS   AGE   MODE     NODEPOOL
aks-default-kw7ds              Ready    82s   user     default        ← NEW user node
aks-hostedpool-24979463-vms1   Ready    77m   system
aks-hostedpool-24979463-vms2   Ready    77m   system
aks-hostedpool-24979463-vms3   Ready    77m   system
aks-system-surge-m8hcb         Ready    74m   system   system-surge
aks-system-surge-rgfkb         Ready    74m   system   system-surge

NAME            TYPE               CAPACITY    ZONE        NODE                READY   NODEPOOL
default-kw7ds   Standard_D4as_v6   on-demand   westus2-2   aks-default-kw7ds   True    default
```

End-to-end, NAP grew a brand-new `mode=user` node (`Standard_D4as_v6`) under the `default` NodePool in ~2 minutes, the customer pod landed on it, and **none of the 5 system-pool nodes were touched**. The new node has no `CriticalAddonsOnly` taint.

**ama-metrics coverage on the new user node — verified live:**

```text
$ kubectl get pods -A --field-selector spec.nodeName=aks-default-kw7ds -o wide
NAMESPACE                 NAME                                     READY   STATUS    AGE
azuresecuritylinuxagent   azuresecuritylinuxagent-m9xhz            7/7     Running   2m51s
hello-test                hello-765498655-9xr6q                    1/1     Running   4m6s
kube-system               aks-secrets-store-csi-driver-s9gsj       3/3     Running   3m26s
kube-system               aks-secrets-store-provider-azure-n6q55   1/1     Running   3m26s
kube-system               ama-logs-s-5bgxr                         3/3     Running   3m26s
kube-system               ama-metrics-node-s-wwfnk                 2/2     Running   3m26s   ← prometheus-collector
kube-system               azure-cns-5hdk8                          1/1     Running   3m26s
kube-system               azure-ip-masq-agent-6x8br                1/1     Running   3m26s
kube-system               cilium-g8dqc                             3/3     Running   3m26s
kube-system               cloud-node-manager-mtkt8                 1/1     Running   3m26s
kube-system               csi-azuredisk-node-2pf2k                 3/3     Running   3m26s
kube-system               csi-azurefile-node-k9bg9                 4/4     Running   3m26s
```

The right-sized `ama-metrics-node-s` DaemonSet pod (along with `ama-logs-s`, the secrets-store CSI, cilium, azure-cns, etc.) landed on the new user node within ~30 s of the node going `Ready`. **Customer apps scheduled on NAP-grown user nodes are scraped by ama-metrics with no operator action required.**

**Mental model:**

| Pool | Scheduled by | VM owned by | What runs there |
|---|---|---|---|
| `hostedpool` (3 nodes) | AKS | **AKS subscription** (free) | AKS-managed system pods (coredns, keda, vpa, …) and select AKS-allowlisted DaemonSets |
| `system-surge` (2 nodes) | NAP | **Customer subscription** | AKS-managed addon Deployments (`ama-metrics`, `ama-metrics-ksm`, `ama-metrics-operator-targets`) + select DaemonSets |
| NAP `default` → on-demand `aks-default-*` nodes | NAP | **Customer subscription** | **Customer apps** |

Customers don't pre-create user node pools on AKS Automatic — NAP grows them on demand from the `default` NodePool spec. If specific SKUs/zones are needed, additional `karpenter.sh/NodePool` CRDs can be added.

#### Who pays for which node? (verified from `providerID`)

Each node's `spec.providerID` resolves to an Azure subscription + resource group, which definitively identifies the bill payer:

```text
$ kubectl get nodes -o custom-columns=NAME:.metadata.name,PROVIDER:.spec.providerID
NAME                           PROVIDER
aks-hostedpool-24979463-vms1   azure:///subscriptions/3a9b3158-b2f4-4121-af63-2705ea639e5a/resourceGroups/hobo-6a078a5a8483030001ff7771-rg/providers/Microsoft.Compute/virtualMachines/aks-hostedpool-24979463-vms1
aks-hostedpool-24979463-vms2   azure:///subscriptions/3a9b3158-…/resourceGroups/hobo-…-rg/providers/Microsoft.Compute/virtualMachines/aks-hostedpool-24979463-vms2
aks-hostedpool-24979463-vms3   azure:///subscriptions/3a9b3158-…/resourceGroups/hobo-…-rg/providers/Microsoft.Compute/virtualMachines/aks-hostedpool-24979463-vms3
aks-system-surge-m8hcb         azure:///subscriptions/9c17527c-af8f-4148-8019-27bada0845f7/resourceGroups/MC_zane-rg-auto-msnp_zane-auto-msnp_westus2/providers/Microsoft.Compute/virtualMachines/aks-system-surge-m8hcb
aks-system-surge-rgfkb         azure:///subscriptions/9c17527c-…/resourceGroups/MC_…/providers/Microsoft.Compute/virtualMachines/aks-system-surge-rgfkb
aks-default-kw7ds              azure:///subscriptions/9c17527c-…/resourceGroups/MC_…/providers/Microsoft.Compute/virtualMachines/aks-default-kw7ds
```

| Node | Subscription | Resource group | Who pays |
|---|---|---|---|
| `aks-hostedpool-24979463-vms{1,2,3}` | `3a9b3158-b2f4-4121-af63-2705ea639e5a` (Microsoft-internal) | `hobo-6a078a5a8483030001ff7771-rg` | **Microsoft (free to customer)** |
| `aks-system-surge-{m8hcb,rgfkb}` | `9c17527c-…-845f7` (customer) | `MC_zane-rg-auto-msnp_zane-auto-msnp_westus2` | **Customer** (Standard_D2als_v6 meter) |
| `aks-default-kw7ds` (NAP user node) | `9c17527c-…` (customer) | same `MC_…` RG | **Customer** (Standard_D4as_v6 meter) |

Two strong signals confirming the split:

1. **`providerID` resource paths point to different subscriptions.** Customer-billed VMs live under `subscriptions/9c17527c-…/resourceGroups/MC_…`. `hostedpool` VMs live under `subscriptions/3a9b3158-…/resourceGroups/hobo-…-rg`. The `hobo-` prefix is AKS's internal naming for "hosted-on-behalf-of" infrastructure.
2. **`az account show --subscription 3a9b3158-…` returns `Subscription not found`** for the cluster owner (who has Subscription Owner on the customer sub and a wide tenant view). That subscription is in Microsoft's tenant, not the customer's.

**Listing the customer-side `MC_…` RG** confirms it: it contains `aks-system-surge-*` and `aks-default-*` VMs and their `computeAksLinuxBilling` extensions (the meter that drives AKS Linux VM billing) — but **no `aks-hostedpool-*` VMs**. Those literally don't exist in the customer subscription.

**The contract**

| Pool | Customer pays | Customer can use |
|---|---|---|
| `hostedpool` (3 nodes, Microsoft sub) | Nothing — VM cost, OS, control-plane attach all on Microsoft. | No (and can't even see the VMs in `az`). |
| `system-surge` (2 nodes, customer sub) | Full VM meter (Standard_D2als_v6 × 2) + AKS Linux VM meter via `computeAksLinuxBilling`. | No — taints + admission VAPs block customer pods, but you still pay the bill. |
| NAP `default` → `aks-default-*` (user nodes, customer sub) | Full VM meter for whichever SKU NAP picks. | Yes — this is the only place customer apps land. |

**Slightly unintuitive part of MSNP economics:** the 2 `aks-system-surge` nodes are **customer-billed but customer-unusable**. They exist in the customer subscription so AKS-managed addon Deployments (`ama-metrics`, `ama-metrics-ksm`, `ama-metrics-operator-targets`, plus `azurepolicy-…`, `azurekeyvaultsecretsprovider-…`, `webapprouting-…`, etc.) have somewhere to live without spending Microsoft's compute budget on customer-elected addons. NAP scales `system-surge` on demand based on installed addon resource requests.

**Net win vs. classic AKS Automatic (no MSNP):**

- **3 nodes' worth of system overhead** (coredns, keda, vpa, metrics-server, konnectivity-agent, eraser-controller, azure-wi-webhook, …) moves from the customer's bill to Microsoft's.
- Customer keeps paying for the smaller `aks-system-surge` pool (2 × `D2als_v6`) plus their actual user pools.

**Implication for the prometheus-collector team:** complaints/questions of the form "why am I paying for nodes I can't deploy to?" will be common with MSNP. The answer is: those nodes host the AKS-managed addons you opted into (including `ama-metrics`); the *truly* free nodes are `hostedpool` and they're invisible in your subscription view.

---

## 2. Reproduction

### Cluster create

```bash
# one-time prereqs
az extension add --name aks-preview                                  # ≥ 19.0.0b15
az feature register --name AKS-AutomaticHostedSystemProfilePreview \
                    --namespace Microsoft.ContainerService
az provider register --namespace Microsoft.ContainerService

# requires azure-cli ≥ 2.86 (2.83 hits a `too many values to unpack` bug
# inside aks-preview ≥ 21.0.0b1 set_up_network_profile)

az group create -n zane-rg-auto-msnp -l westus2

az aks create \
  -g zane-rg-auto-msnp -n zane-auto-msnp \
  --sku automatic --enable-hosted-system --location westus2
```

### Test payload

```yaml
# /tmp/ama-metrics-test-cm.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  schema-version: "v1"
  config-version: "ver1"
  prometheus-collector-settings: |-
    cluster_alias = ""
```

### Apply (server dry-run + real)

```bash
az aks get-credentials -g zane-rg-auto-msnp -n zane-auto-msnp --overwrite-existing
kubectl apply -f /tmp/ama-metrics-test-cm.yaml --dry-run=server
kubectl apply -f /tmp/ama-metrics-test-cm.yaml
```

Both attempts return:

```
Error from server (Forbidden): error when creating "/tmp/ama-metrics-test-cm.yaml":
configmaps "ama-metrics-settings-configmap" is forbidden:
ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces'
with binding 'aks-managed-protect-system-namespaces-binding'
denied request: Modification of resources in managed system namespaces is not allowed
```

### Authorization vs admission

```bash
kubectl auth can-i create configmap -n kube-system
# yes
```

RBAC allows the action; admission then rejects it. The `Cluster Admin` role definition is unchanged — only the cluster-side admission layer changed.

### Verbatim terminal capture

Reproduced from a regular dev terminal (WSL, Linux `kubectl 1.35.4`) on `2026-05-15`:

```text
❯ kubectl apply -f /tmp/ama-metrics-test-cm.yaml
Error from server (Forbidden): error when creating "/tmp/ama-metrics-test-cm.yaml": configmaps "ama-metrics-settings-configmap" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed
```

Identity: `zanejohnson@microsoft.com` (subscription `Owner` + cluster-scoped `Azure Kubernetes Service RBAC Cluster Admin`). Context: `zane-auto-msnp`. Authentication via `kubelogin -l azurecli` against an Entra-ID-only kubeconfig.

The configmap payload was:

```yaml
# /tmp/ama-metrics-test-cm.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  config-version: "ver1"
```

---

## 3. The denying policy: `aks-managed-protect-system-namespaces`

A pre-installed in-tree Kubernetes `ValidatingAdmissionPolicy` (CEL-based, K8s 1.30+). Captured verbatim from the cluster.

### Match constraints — what it intercepts

```yaml
matchConstraints:
  matchPolicy: Equivalent
  resourceRules:
    - apiGroups:    ['*']
      apiVersions:  ['*']
      operations:   [CREATE, UPDATE, DELETE]
      resources:    ['*', '*/*']        # all resources AND their subresources
      scope: Namespaced
  namespaceSelector:
    matchExpressions:
      - key: kubernetes.io/metadata.name
        operator: In
        values:
          - aks-command
          - kube-system                  # ← us
          - calico-system
          - tigera-system
          - gatekeeper-system
          - azappconfig-system
          - azureml
          - dapr-system
          - dataprotection-microsoft
          - flux-system
          - acstor
          - sc-system
          - azure-extensions-usage-system
          - app-routing-system
          - aks-periscope
          - aks-istio-system
          - aks-istio-ingress
          - aks-istio-egress
          - aks-static-egress-gateway
          - azuresecuritylinuxagent
```

#### How to dump this list yourself

The 20-namespace match list comes straight from the live VAP — there's no static AKS doc that lists it, so dump from the cluster:

```bash
# 1. Just the namespace names, one per line
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces \
  -o jsonpath='{.spec.matchConstraints.namespaceSelector.matchExpressions[0].values}' \
  | tr ',' '\n' | tr -d '[]" '

# 2. Pretty JSON of the full namespaceSelector (shows operator + key + values)
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces \
  -o jsonpath='{.spec.matchConstraints.namespaceSelector}' | python3 -m json.tool

# 3. With jq if available
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces -o json \
  | jq '.spec.matchConstraints.namespaceSelector'
```

> **Terminology note.** The Kubernetes field is a `namespaceSelector` — a *match list*, not formally an allowlist or denylist. Mental model:
>
> ```text
> Is the target namespace in matchExpressions.values?
>        │
>        ├── YES → VAP runs → expression evaluates "false" → DENIED
>        │                    (unless caller is in matchConditions exemption,
>        │                     e.g. ama-metrics-serviceaccount)
>        │
>        └── NO  → VAP doesn't fire → request passes (subject to other admission/RBAC)
> ```
>
> So functionally **a namespace IN the list = your write is BLOCKED**; **a namespace NOT in the list = the policy doesn't apply**. We refer to it as the "protected-namespace list" throughout this doc.

### Validation expression

```yaml
validations:
  - expression: "false"      # always deny
    message: Modification of resources in managed system namespaces is not allowed
    reason: Forbidden
failurePolicy: Fail          # if VAP can't evaluate, deny
```

### Exemptions (`matchConditions`) — who can still mutate

The VAP wraps `validations` in two `matchConditions` that *skip* evaluation for trusted callers, so the policy effectively short-circuits to "allowed" for them:

**Exempt usernames** (`apply-to-non-exempt-users`) — verbatim list:

- AKS control-plane principals: `acsService`, `aksService`, `hcpService`, `aks-support`, `system:apiserver`
- Built-in Kubernetes controllers: `attachdetach-controller`, `certificate-controller`, `clusterrole-aggregation-controller`, `cronjob-controller`, `daemon-set-controller`, `deployment-controller`, `disruption-controller`, `endpoint-controller`, `endpointslice-controller`, `endpointslicemirroring-controller`, `ephemeral-volume-controller`, `expand-controller`, `job-controller`, `namespace-controller`, `node-controller`, `pv-protection-controller`, `pvc-protection-controller`, `replicaset-controller`, `replication-controller`, `resourcequota-controller`, `service-account-controller`, `statefulset-controller`, `ttl-after-finished-controller`, `ttl-controller`, `validatingadmissionpolicy-status-controller`, `horizontal-pod-autoscaler`, `generic-garbage-collector`, `root-ca-cert-publisher`, `cloud-node-manager`
- AKS addon SAs: `cilium`, `cilium-operator`, `keda-operator`, `keda-metrics-server`, `vpa-admission-controller`, `eraser-controller-manager`, `metrics-server`, `coredns-autoscaler`, `konnectivity-agent-autoscaler`, `overlay-vpa-webhook-generation`, `azure-policy`, **`ama-metrics-serviceaccount`**, `gatekeeper-admin`, `app-routing-system:nginx`

**Exempt groups** (`apply-to-non-exempt-groups`):

- `system:masters`, `system:nodes`, `system:bootstrappers`
- `system:serviceaccounts:kube-system`              ← any SA in kube-system
- `system:serviceaccounts:aks-istio-system`
- `system:serviceaccounts:dapr-system`
- `system:serviceaccounts:flux-system`
- `system:serviceaccounts:dataprotection-microsoft`
- `system:serviceaccounts:azure-extensions-usage-system`

**Cluster Admin (Entra-ID-mapped) is not exempt.** Neither is any custom SA in `default` or any other non-listed namespace.

### Binding

```yaml
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: aks-managed-protect-system-namespaces-binding
spec:
  policyName: aks-managed-protect-system-namespaces
  validationActions: [Deny]    # no warn-only mode
```

### Can a customer edit or delete the VAP/binding to escape it?

**No** — and **not because of RBAC**. Tested live on `zane-auto-msnp` as subscription `Owner` + `Azure Kubernetes Service RBAC Cluster Admin`:

```
$ kubectl auth can-i patch validatingadmissionpolicies.admissionregistration.k8s.io
yes
$ kubectl auth can-i patch validatingadmissionpolicybindings.admissionregistration.k8s.io
yes
$ kubectl auth can-i delete validatingadmissionpolicies.admissionregistration.k8s.io
yes
```

Kubernetes RBAC says yes. But the actual write is rejected by a **second authorization layer** that AKS layers on top — an internal authorization webhook the API server identifies as `(automatic-authz)`:

```
Error from server (Forbidden): validatingadmissionpolicybindings.admissionregistration.k8s.io
"aks-managed-protect-system-namespaces-binding" is forbidden: User "zanejohnson@microsoft.com"
cannot patch resource "validatingadmissionpolicybindings" in API group "admissionregistration.k8s.io"
at the cluster scope: (automatic-authz) Modifications to protected
ValidatingAdmissionPolicyBinding 'aks-managed-protect-system-namespaces-binding'
are not allowed for user 'zanejohnson@microsoft.com'
```

| Verified test on `zane-auto-msnp` | Result | Notes |
|---|---|---|
| `patch validationActions: [Warn, Audit]` on the binding | ❌ `(automatic-authz)` deny | The mode flip is locked |
| `delete` the VAP | ❌ `(automatic-authz)` deny | Server-side dry-run still denied |
| `delete` the binding | ❌ `(automatic-authz)` deny | Same |
| `patch validationActions` on an unrelated `aks-managed-*` VAPB (`aks-managed-baseline-apparmor-binding`) | ❌ `(automatic-authz)` deny | **The lock covers *all* `aks-managed-*` VAPs / VAPBs, not just protect-system-namespaces** |

The protection is purely **name-based** — the VAP has no `kubernetes.azure.com/managedby` label or annotation; the protected-resource list is hardcoded inside the `automatic-authz` webhook on AKS's side.

**The `(automatic-authz)` lock is MSNP-specific** (verified on `zane-auto-2`, classic AKS Automatic — patching the VAP succeeds there). When AKS flips classic AKS Automatic from `validationActions: [Audit]` → `[Deny]`, they will almost certainly ship the `automatic-authz` lock at the same time — the two are architecturally paired (no point flipping to Deny if customers could just flip it back).

**Implications:**

- **There is no self-service workaround** to allow customer writes to `kube-system` on MSNP. Modifying the VAP, modifying the binding, deleting either, or modifying any `aks-managed-*` VAP/VAPB — all blocked by the same authz lock, regardless of Azure RBAC tier.
- **"Petition AKS for a per-CM exemption" is not an API call.** Only AKS can edit the VAP, and the change ships through their internal release process (not through any tenant-facing surface).
- This rules out any "we'll just ask cluster admins to disable it for ama-metrics CMs" workaround pattern. See solution-options doc for the architecturally viable paths.

### Evaluation pipeline — the 5 gates a request walks

A single API request has to clear **3 (sometimes 4) gates** before this VAP will deny it. If *any* gate fails, the VAP doesn't fire and the request continues through other policies / RBAC. Use this as a debugging walkthrough whenever a request gets unexpectedly denied (or unexpectedly allowed):

```text
                ┌─────────────────────────────────────┐
                │ Incoming kube-apiserver request     │
                │ (verb + apiGroup + resource + ns +  │
                │  caller identity + payload)         │
                └─────────────────────────────────────┘
                              │
   ┌─────────────────────────────────────────────────────────┐
   │ Gate 1: matchConstraints.resourceRules                  │
   │   - apiGroups   match  ['*']                  ?         │
   │   - apiVersions match  ['*']                  ?         │
   │   - operations  ∈     [CREATE, UPDATE, DELETE]?         │
   │   - resources   match ['*', '*/*']            ?         │
   │   - scope       =     Namespaced              ?         │
   └─────────────────────────────────────────────────────────┘
                              │
                       NO ────┼──── YES
                       │      │
                       ▼      ▼
               policy doesn't fire — request continues
                              │
   ┌─────────────────────────────────────────────────────────┐
   │ Gate 2: matchConstraints.namespaceSelector              │
   │   - request namespace ∈ {kube-system, …, 20 total}  ?   │
   └─────────────────────────────────────────────────────────┘
                              │
                       NO ────┼──── YES
                       │      │
                       ▼      ▼
               policy doesn't fire — request continues
                              │
   ┌─────────────────────────────────────────────────────────┐
   │ Gate 3: matchConstraints.objectSelector                 │
   │   - empty here → matches everything → always YES        │
   └─────────────────────────────────────────────────────────┘
                              │
                              ▼
   ┌─────────────────────────────────────────────────────────┐
   │ Gate 4: matchConditions (CEL "skip if exempt") checks   │
   │   - apply-to-non-exempt-users : caller NOT in           │
   │     exempt-user list (acsService, hcpService,           │
   │     ama-metrics-serviceaccount, …)             ?        │
   │   - apply-to-non-exempt-groups: caller NOT in           │
   │     exempt-group list (system:masters, system:nodes,    │
   │     system:serviceaccounts:kube-system, …)     ?        │
   └─────────────────────────────────────────────────────────┘
                              │
                       NO ────┼──── YES (caller IS exempt)
                       │      │
                       ▼      ▼
               validations skip → policy passes (allows)
                              │
   ┌─────────────────────────────────────────────────────────┐
   │ Gate 5: validations[].expression runs                   │
   │   expression: "false"   ←  always evaluates to deny     │
   └─────────────────────────────────────────────────────────┘
                              │
                              ▼
                     ❌ DENIED  with message
                       "Modification of resources in
                        managed system namespaces …"
```

**Per-gate debugging table:**

| Gate | Field | What to check | If it fails |
|---|---|---|---|
| 1 | `matchConstraints.resourceRules` | Verb in `[CREATE,UPDATE,DELETE]`? Resource namespaced? | VAP doesn't fire — your read (`GET/LIST/WATCH`) or your cluster-scoped resource (`Node`, `ClusterRole`, …) was never in scope. |
| 2 | `matchConstraints.namespaceSelector` | Target namespace one of the 20? | Your namespace was never in scope — the VAP can't be the cause of the deny. |
| 3 | `matchConstraints.objectSelector` | (empty here = match all) | n/a on this VAP. |
| 4 | `matchConditions` | Caller's username on exempt-users list, OR caller's group on exempt-groups list? | If exempt → VAP allows. (This is how `ama-metrics-serviceaccount` writes to `kube-system` even though it's "protected.") |
| 5 | `validations[].expression` | Always `"false"` → deny | This is *the* deny step. Reaching it means the request matched all gates and isn't exempt. |

**Worked examples on `zane-auto-msnp`:**

| Operation | Where it stops | Outcome |
|---|---|---|
| `kubectl get cm -n kube-system` | Gate 1 — `GET` not in `[CREATE,UPDATE,DELETE]` | ✅ allowed |
| `kubectl create cm probe -n default` | Gate 2 — `default` not in the 20-namespace list | ✅ allowed |
| `kubectl get nodes` | Gate 1 — `Node` is cluster-scoped, fails `scope: Namespaced` filter | ✅ allowed |
| `kubectl create cm probe -n kube-system` (as Entra cluster admin) | Reaches Gate 5 | ❌ denied |
| `kubectl create cm probe -n azuresecuritylinuxagent` (as Entra cluster admin) | Reaches Gate 5 | ❌ denied |
| `kubectl create cm foo -n kube-system` (as `ama-metrics-serviceaccount`) | Stops at Gate 4 (exempt user) | ✅ allowed |
| `kubectl create cm foo -n flux-system` (as a SA in `flux-system`) | Stops at Gate 4 (exempt group `system:serviceaccounts:flux-system`) | ✅ allowed |

So the answer to "is this operation allowed?" is **always**: walk gates 1 → 2 → 3 → 4 → 5 in order, and the first one that says NO ends the evaluation. Only if you reach Gate 5 do you get the deny verdict.

---

## 4. Full inventory of `aks-managed-*` admission policies on MSNP

22 policies in total (covering pod security baseline + system-namespace protection):

| Policy | Purpose |
|---|---|
| `aks-managed-protect-system-namespaces` | Block customer CRUD on AKS-managed namespaces (this doc) |
| `aks-managed-protect-system-namespace-objects` | Likely targets specific cluster-scoped objects tied to system ns |
| `aks-managed-protect-interactive-access` | Block `exec`/`attach`/`port-forward` on pods in system namespaces |
| `aks-managed-protect-hobo-vm-nodes` | Block modifying / labeling AKS-managed (hidden) nodes |
| `aks-managed-protect-kubernetes-endpoints` | Protect the `kubernetes` Endpoints object |
| `aks-managed-protect-kubernetes-endpointslice` | Same for EndpointSlice |
| `aks-managed-block-nodes-proxy-rbac` | Block escalation via `nodes/proxy` |
| `aks-managed-critical-addons-only` | Reserved tolerations on system nodes |
| `aks-managed-custom-scheduler` | Disallow custom schedulers on managed nodes |
| `aks-managed-baseline-apparmor` | Pod security: AppArmor |
| `aks-managed-baseline-capabilities` | Pod security: Linux capabilities |
| `aks-managed-baseline-host-namespaces` | Pod security: hostNetwork/hostPID/hostIPC |
| `aks-managed-baseline-host-ports` | Pod security: hostPorts |
| `aks-managed-baseline-host-probes-lifecycle-hooks` | Pod security |
| `aks-managed-baseline-host-process` | Pod security |
| `aks-managed-baseline-hostpath-volumes` | Pod security: hostPath |
| `aks-managed-baseline-privileged-containers` | Pod security: privileged |
| `aks-managed-baseline-proc-mount-type` | Pod security |
| `aks-managed-baseline-seccomp` | Pod security |
| `aks-managed-baseline-selinux` | Pod security |
| `aks-managed-baseline-sysctls` | Pod security |

> Per the [overview doc](https://learn.microsoft.com/azure/aks/automatic/aks-automatic-managed-system-node-pools-about), these implement: managed-system-resource changes, interactive access to system pods, managed-system-node changes, workload placement on system nodes, privileged cluster access paths, protected identity impersonation, and AKS-managed security control changes.

---

## 5. Operations matrix (verified end-to-end)

| Operation | Result | Policy that fired |
|---|---|---|
| `kubectl auth can-i create configmap -n kube-system` | `yes` | (RBAC layer only, no admission) |
| **CREATE** `ama-metrics-settings-configmap` in `kube-system` | ❌ Forbidden | `aks-managed-protect-system-namespaces` |
| **DELETE** `coredns-custom` configmap in `kube-system` | ❌ Forbidden | `aks-managed-protect-system-namespaces` |
| **PATCH** `coredns` configmap in `kube-system` | ❌ Forbidden | `aks-managed-protect-system-namespaces` |
| **EXEC** into `ama-metrics-7577479d6d-497pc` | ❌ Forbidden | `aks-managed-protect-interactive-access` |
| `kubectl get/list/watch` resources in `kube-system` | ✅ Allowed | (read verbs not in `resourceRules.operations`) |
| `kubectl logs` on `kube-system` pods | ✅ Allowed | (read on `pods/log` subresource) |
| Same payload applied in a fresh `mytest` namespace | ✅ Created | (namespace not in `namespaceSelector`) |

The `EXEC` denial returns a different message:

```
Error from server (Forbidden): pods "ama-metrics-..." is forbidden:
ValidatingAdmissionPolicy 'aks-managed-protect-interactive-access'
with binding 'aks-managed-protect-interactive-access-binding'
denied request: Interactive access to pods in system namespaces is not allowed for security reasons
```

### 5.1 Deny extends to ALL 20 protected namespaces — not just `kube-system`

Initial intuition (and the prior investigation) treated this as a `kube-system`-specific block. Empirical re-test on `zane-auto-msnp` (2026-05-15) confirms **the same VAP fires identically across the entire `namespaceSelector` protected-namespace list** — including namespaces that exist out of the box on every MSNP cluster (`gatekeeper-system`, `app-routing-system`, `azuresecuritylinuxagent`) and ones that only appear when their addon is enabled (`aks-istio-system`, `dapr-system`, `flux-system`, `azureml`, …).

> **Terminology note.** Calling this an "allowlist" or "denylist" is misleading; the field is a Kubernetes `namespaceSelector` (a *match list*). Net effect: **a namespace IN the list = the policy fires and the request is DENIED**; **a namespace NOT in the list = the policy doesn't apply and the request proceeds normally**. So functionally it acts like a "denylist of namespaces you can't write to," but Kubernetes itself doesn't use that word.

| Namespace tested | In protected-namespace list? | CREATE configmap result |
|---|---|---|
| `kube-system` | yes | ❌ Forbidden — `aks-managed-protect-system-namespaces` |
| `azuresecuritylinuxagent` | yes | ❌ Forbidden — same VAP, identical message |
| `app-routing-system` | yes | ❌ Forbidden — same VAP, identical message |
| `gatekeeper-system` | yes | ❌ Forbidden — same VAP, identical message |
| `kube-public` (control) | no | ✅ Created |
| `default` (control) | no | ✅ Created |

All three system-namespace denials returned **byte-identical** messages, differing only in the namespace embedded in the resource name:

```
Error from server (Forbidden): configmaps "test-discrepancy-probe" is forbidden:
  ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces'
  with binding 'aks-managed-protect-system-namespaces-binding'
  denied request: Modification of resources in managed system namespaces is not allowed
```

**Takeaway:** the protection surface is **the entire 20-namespace protected list**, not `kube-system` alone. The exempt-group list also widens accordingly — e.g., any SA in `aks-istio-system` is exempt for `aks-istio-system` writes, any SA in `flux-system` is exempt for `flux-system`, etc. (see §3 for the full exempt-group list).

Reproduction (one-liner per namespace):

```bash
for ns in azuresecuritylinuxagent app-routing-system gatekeeper-system; do
  kubectl create configmap probe -n $ns --from-literal=k=v 2>&1
done
```

---

## 6. Implications for managed Prometheus / `ama-metrics`

> **Solution options for the broken customer configmap workflow are tracked in the companion doc:**
> [`aks-automatic-msnp-configmap-solution-options.md`](./aks-automatic-msnp-configmap-solution-options.md). This section documents the *implications* (what's broken and why); the companion doc enumerates *candidate fixes* (CRDs, ARM, etc.) and the trade-offs.

1. **Customer-side `kubectl edit/apply` of `ama-metrics-settings-configmap` is dead.** The configmap can only be mutated by:
   - The exempt user `system:serviceaccount:kube-system:ama-metrics-serviceaccount` (the agent itself), or
   - Anything running as a SA in `kube-system` (group exemption).

   Customers must use the **AzureMonitorWorkspace / DCR / data collection settings** ARM surface — already the recommended path on classic AKS Automatic, now the *only* path on MSNP. **All 4 documented customer-facing CMs in `otelcollector/configmaps/` are affected**, not just `ama-metrics-settings-configmap`. See the [solution-options doc](./aks-automatic-msnp-configmap-solution-options.md) for proposed paths forward.

2. **`kubectl exec deploy/ama-metrics -- ...` for support / debugging is blocked** by `aks-managed-protect-interactive-access`. Live troubleshooting needs to switch to:
   - `kubectl logs` (still allowed), or
   - `kubectl debug` workarounds (likely also constrained by `aks-managed-protect-hobo-vm-nodes` for node-level debug).

3. **Agent code paths that mutate `kube-system` at runtime must run as `ama-metrics-serviceaccount`** (or another exempt SA in `kube-system`). Examples to audit:
   - Leader-election leases.
   - Config reconciliation by `configuration-reader-builder`.
   - Cert/secret material managed by `ama-metrics-mutating-admission-webhook` (if any).
   - Anything in CCP mode or sidecars running under a non-`kube-system` SA.

4. **No 1-click migration path.** Per Microsoft's preview limitations: *"Migrations between AKS Automatic clusters and AKS Automatic clusters with managed system node pools aren't supported."* Existing customers cannot opt-in via `az aks update`; they must create a new MSNP cluster.

5. **The public MSNP doc misclassifies ama-metrics Deployments — they actually run on the customer-billed `aks-system-surge` pool, not on Microsoft-billed `hostedpool`.** See §6.1 for the verified discrepancy and likely root cause.

6. **The protection is not `kube-system`-only — it covers all 20 namespaces in the VAP's protected-namespace list.** This matters for any ama-metrics codepath (or future feature) that touches sibling system namespaces, e.g.:
   - `aks-istio-system` / `aks-istio-ingress` / `aks-istio-egress` — if scraping config wants to push ServiceMonitors or PodMonitors there for service-mesh metrics.
   - `gatekeeper-system` — if drop-in config needs to be added for OPA constraints or audit policies.
   - `app-routing-system` — if NGINX ingress metrics scraping requires per-namespace customization.
   - `flux-system` / `dapr-system` — same story for Flux Helm releases or Dapr sidecar metrics.

   In every case, the same exempt-SA carve-out applies — write from a SA whose namespace matches one of the exempt groups (`system:serviceaccounts:<that-ns>`) or one of the per-name exemptions in §3. **A customer-supplied SA in `default`, or any unlisted namespace, will be denied identically across all 20 namespaces.**

### 6.1 Doc discrepancy — ama-metrics Deployments are on the customer's bill, not Microsoft's

**The public doc claims** ([Components of managed system node pools](https://learn.microsoft.com/azure/aks/automatic/aks-automatic-managed-system-node-pools-about#components-of-managed-system-node-pools)):

> | Component | Namespace | Deployment(s) |
> |---|---|---|
> | **Azure Monitor** | `kube-system` | **`ama-logs, ama-metrics, ama-metrics-ksm, ama-metrics-operator-targets`** |
> | Workload identity | `kube-system` | `azure-wi-webhook-controller-manager` |
> | CoreDNS | `kube-system` | `coredns, coredns-autoscaler` |
> | Eraser | `kube-system` | `eraser-controller-manager` |
> | KEDA | `kube-system` | `keda-admission-webhooks, keda-operator, keda-operator-metrics-apiserver` |
> | Konnectivity | `kube-system` | `konnectivity-agent, konnectivity-agent-autoscaler` |
> | Metrics Server | `kube-system` | `metrics-server` |
> | VPA | `kube-system` | `vpa-admission-controller, vpa-recommender, vpa-updater` |
>
> *"AKS handles the creation, upgrading, and scaling of the system nodes where these components run."*

In the doc's own terminology, "the system nodes where these components run" = the AKS-managed `hostedpool` (Microsoft's subscription, free to the customer).

**What actually runs on `zane-auto-msnp` (verified 2026-05-15):**

| Deployment | Pod | Lands on | Bill payer | Matches doc? |
|---|---|---|---|---|
| `ama-metrics` | `ama-metrics-7577479d6d-497pc` | `aks-system-surge-m8hcb` | **Customer** | ❌ |
| `ama-metrics` | `ama-metrics-7577479d6d-tbxrm` | `aks-system-surge-rgfkb` | **Customer** | ❌ |
| `ama-metrics-ksm` | `ama-metrics-ksm-7c6789756d-f8z27` | `aks-system-surge-rgfkb` | **Customer** | ❌ |
| `ama-metrics-operator-targets` | `ama-metrics-operator-targets-7649885c58-96vzv` | `aks-system-surge-rgfkb` | **Customer** | ❌ |

**The discrepancy is specific to `ama-metrics`-related Deployments.** Every other component in the doc's table actually does land on `hostedpool` as documented:

| Doc-listed component | Actually on (verified) |
|---|---|
| `coredns` / `coredns-autoscaler` | `aks-hostedpool-…vms2` ✓ |
| `azure-wi-webhook-controller-manager` (×2) | `aks-hostedpool-…vms2` ✓ |
| `eraser-controller-manager` | `aks-hostedpool-…vms2` ✓ |
| `keda-admission-webhooks` (×2), `keda-operator-metrics-apiserver` (×2) | `aks-hostedpool-…vms2` ✓ |
| `keda-operator` (×2) | `aks-hostedpool-…vms{1,2}` ✓ |
| `konnectivity-agent` (×2), `konnectivity-agent-autoscaler` | `aks-hostedpool-…vms{1,2,3}` ✓ |
| `metrics-server` (×2) | `aks-hostedpool-…vms{1,2}` ✓ |
| `vpa-admission-controller`, `vpa-recommender`, `vpa-updater` (×2 each) | `aks-hostedpool-…vms{1,2,3}` ✓ |

So 7 of the 8 Azure-managed addons in the doc's table follow the doc; only ama-metrics breaks the pattern. (`ama-logs` is mixed — `ama-logs-rs` Deployment lands on `system-surge`, but the DaemonSet variants land on every pool. The doc-listed Deployment portion is, like ama-metrics, mismatched.)

**Likely root cause:** the prometheus-collector helm chart's `Deployment` specs lack the toleration AKS requires for `hostedpool` placement (`kubernetes.azure.com/hostedvm:NoSchedule`) and/or the matching nodeAffinity. Other addons (coredns, keda, vpa, konnectivity-agent, eraser, etc.) are shipped/managed by AKS itself and presumably have that toleration baked in. The ama-metrics-related sized DaemonSet `ama-metrics-node` *does* tolerate `hostedvm` (it lands on `hostedpool` as expected), so the gap is only on the Deployment manifests + the `ama-metrics-node-xs/-s/...` sized variants that explicitly target non-hostedpool SKUs.

**Customer-visible impact:** customers reading the MSNP doc will assume all 4 Azure Monitor Deployments are on Microsoft's tab. In reality the customer is paying for the `system-surge` capacity that hosts them — currently `2 × Standard_D2als_v6` (≈ $120/mo in `westus2`), shared with `azure-policy`, `azure-policy-webhook`, `ama-logs-rs`, and the `system-surge`-targeted DaemonSets.

**Two ways to resolve (only one is in the prometheus-collector team's hands):**

1. **AKS doc fix** (out of our control): move `ama-metrics, ama-metrics-ksm, ama-metrics-operator-targets` (and `ama-logs`) out of the "AKS-managed" table into the existing "Other add-ons and extensions run on an `aks-system-surge` node, with scaling handled by NAP" sentence — which already correctly describes their actual placement.
2. **Prometheus-collector chart fix** (would need AKS-side coordination): add `kubernetes.azure.com/hostedvm:NoSchedule` toleration + matching nodeAffinity to the `ama-metrics`, `ama-metrics-ksm`, `ama-metrics-operator-targets` Deployments so they qualify for `hostedpool`. This requires AKS to agree to host them — `hostedpool` is in their subscription, so they ultimately gate which workloads they're willing to run for free.

**Recommendation:** file a doc bug against `MicrosoftDocs/azure-aks-docs` with this evidence (provider IDs, pod-to-node mapping, side-by-side comparison with all 7 other addons that *do* land where the doc claims). If the AKS team prefers option 2, escalate to the prometheus-collector engineering team to coordinate the chart change.

---

## 7. Detection patterns for the agent / docs

When ama-metrics (or any tool) gets a 403 on a `kube-system` mutation, distinguish RBAC denial vs MSNP admission denial:

| Signal | Means |
|---|---|
| `403 Forbidden` + body contains `aks-managed-protect-system-namespaces` | MSNP cluster, admission deny |
| `403 Forbidden` + body contains `Modification of resources in managed system namespaces` | Same (resilient to policy rename) |
| `403 Forbidden` + body lacks any `ValidatingAdmissionPolicy` reference | RBAC deny — caller lacks K8s RBAC |

Cluster-side feature detect (cheap one-shot probe):

```go
_, err := dc.AdmissionregistrationV1().
  ValidatingAdmissionPolicies().
  Get(ctx, "aks-managed-protect-system-namespaces", metav1.GetOptions{})
if err == nil { /* MSNP cluster */ }
```

Suggested customer-facing rewrite when the agent surfaces this error:

> *Cannot modify `ama-metrics-settings-configmap` in `kube-system`: this AKS Automatic cluster has managed system node pools enabled (preview), which blocks all customer modifications to AKS-managed namespaces. Configure managed Prometheus through the AzureMonitorWorkspace / DCR on the cluster resource instead. (Admission policy: `aks-managed-protect-system-namespaces`)*

---

## 8. Reproduction commands (consolidated)

```bash
# 1. Confirm MSNP at the ARM layer
az aks show -g zane-rg-auto-msnp -n zane-auto-msnp \
  --query "{name:name, sku:sku, hosted:hostedSystemProfile, state:provisioningState}" -o json

# 2. Confirm MSNP from inside the cluster
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces
kubectl get vap,vapb | grep aks-managed

# 3. Reproduce the deny — write
cat > /tmp/cm.yaml <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  config-version: "ver1"
EOF
kubectl apply -f /tmp/cm.yaml --dry-run=server     # Forbidden
kubectl apply -f /tmp/cm.yaml                       # Forbidden

# 4. Reproduce the deny — interactive
POD=$(kubectl -n kube-system get pods --no-headers | awk '/^ama-metrics-/{print $1; exit}')
kubectl -n kube-system exec "$POD" -- /bin/sh      # Forbidden

# 5. Show RBAC says yes (so the deny is admission, not authz)
kubectl auth can-i create configmap -n kube-system  # yes
kubectl auth can-i '*' '*' -n kube-system           # yes (mostly)

# 6. Confirm a non-system namespace is unaffected
kubectl create ns mytest
kubectl -n mytest apply -f <(sed 's/kube-system/mytest/' /tmp/cm.yaml)
kubectl -n mytest get cm
kubectl delete ns mytest

# 7. Dump the full VAP definition
kubectl get vap aks-managed-protect-system-namespaces -o yaml
kubectl get vapb aks-managed-protect-system-namespaces-binding -o yaml

# 8. Just the 20-namespace protected list (handy one-liner)
kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces \
  -o jsonpath='{.spec.matchConstraints.namespaceSelector.matchExpressions[0].values}' \
  | tr ',' '\n' | tr -d '[]" '
```

---

## 9. Conclusions

1. **AKS Automatic + MSNP introduces hard, cluster-side denial of `kube-system` mutations** for non-exempt callers. Cluster Admin doesn't override it; only AKS-managed identities (and a fixed list of system SAs) can mutate the namespace.
2. **The denial is implemented as a Kubernetes-native `ValidatingAdmissionPolicy`** (`aks-managed-protect-system-namespaces`) — a different mechanism from the Gatekeeper constraints used elsewhere in AKS Automatic. Detection tooling that only checks Gatekeeper constraints will miss it.
3. **`ama-metrics-serviceaccount` is on the exempt-users list**, so the prometheus-collector agent itself continues to function. **Customer-side configuration via the ConfigMap is permanently blocked** — they must use the ARM/DCR surface.
4. **`exec`/`attach`/`port-forward` into `kube-system` pods is also blocked** by the sibling policy `aks-managed-protect-interactive-access`. Support workflows that rely on `kubectl exec deploy/ama-metrics` need to be rebuilt around `kubectl logs` and DCR-side telemetry.
5. **The original investigation's TL;DR remains correct for non-MSNP clusters.** This doc supersedes it specifically for AKS Automatic clusters created with `--enable-hosted-system`.
