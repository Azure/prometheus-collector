# AKS Automatic — Can a user create a ConfigMap in `kube-system`?

> **Investigation date:** 2026-05-05
> **Cluster under test:** `zane-auto` (RG `zane-rg-auto`, sub `9c17527c-af8f-4148-8019-27bada0845f7`)
> **Identity tested:** `zanejohnson@microsoft.com`

---

## TL;DR

**Yes — a user CAN create a ConfigMap in `kube-system` on an AKS Automatic cluster, _provided their Azure role assignment is scoped at the cluster (or higher) and grants `Microsoft.ContainerService/managedClusters/configmaps/write`._**

The popular assumption "AKS Automatic blocks writes to `kube-system`" is **only true when**:
- The user has a **namespace-scoped** Azure RBAC for Kubernetes role assignment (e.g., scoped to `…/namespaces/<some-other-ns>`), **OR**
- The user has only `Reader`-tier permissions, **OR**
- A custom Gatekeeper / admission policy explicitly denies the resource (none exist by default on AKS Automatic for `ConfigMap`).

There is no built-in deployment safeguard on AKS Automatic that blocks ConfigMap mutations in `kube-system`. The default Azure Policy / Gatekeeper constraints all target **pod/workload** concerns (privileged containers, host namespaces, image allow-lists, capabilities, etc.).

---

## 1. Cluster facts

| Property | Value |
|---|---|
| Name | `zane-auto` |
| Resource group | `zane-rg-auto` |
| Subscription | `9c17527c-af8f-4148-8019-27bada0845f7` |
| Region | `eastus2` |
| Kubernetes version | `1.34` |
| SKU | **`Automatic`** (confirmed via `aks_cluster_get`) |
| Network plugin | Azure CNI Overlay + Cilium dataplane |
| `disableLocalAccounts` | `true` (Entra ID required) |
| Azure RBAC for K8s Authorization | Enabled |
| Azure Policy add-on (Gatekeeper) | Enabled |
| Workload Identity | Enabled |

---

## 2. Test performed

A `ConfigMap` named `ama-metrics-settings-configmap` was applied to `kube-system`:

```yaml
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
  default-scrape-settings-enabled: |-
    kubelet = true
    coredns = false
    cadvisor = true
    kubeproxy = false
    apiserver = false
    kubestate = true
    nodeexporter = true
    windowsexporter = false
    windowskubeproxy = false
    kappiebasic = true
    networkobservabilityRetina = true
    networkobservabilityHubble = true
    networkobservabilityCilium = true
    prometheuscollectorhealth = false
    controlplane-apiserver = true
    controlplane-cluster-autoscaler = false
    controlplane-kube-scheduler = false
    controlplane-kube-controller-manager = false
    controlplane-etcd = true
    acstor-capacity-provisioner = true
    acstor-metrics-exporter = true
  pod-annotation-based-scraping: |-
    podannotationnamespaceregex = ""
  send-ds-prom-data-to-cluster-side-collector: "false"
```

### Authentication setup

```powershell
az aks get-credentials -g zane-rg-auto -n zane-auto -f $env:TEMP\zane-auto-kubeconfig --overwrite-existing
$env:KUBECONFIG = "$env:TEMP\zane-auto-kubeconfig"
kubelogin convert-kubeconfig -l azurecli
```

### Apply (server-side dry run, then real)

```powershell
kubectl apply -f $env:TEMP\ama-metrics-test-cm.yaml --dry-run=server
# configmap/ama-metrics-settings-configmap created (server dry run)

kubectl apply -f $env:TEMP\ama-metrics-test-cm.yaml
# configmap/ama-metrics-settings-configmap created
```

### Result

| Field | Value |
|---|---|
| Created | ✅ |
| UID | `9f2327c6-24cd-4d06-8474-8bd4111d499f` |
| `creationTimestamp` | `2026-05-06T05:19:52Z` |
| Namespace | `kube-system` |

A second test using an arbitrary ConfigMap name in `kube-system` also passed server-side dry run, confirming the result was **not** name-specific.

---

## 3. Identity & role assignments under test

`zanejohnson@microsoft.com` had the following assignments on the cluster:

| Role | Scope |
|---|---|
| `Owner` | Subscription / RG (cluster-scoped) |
| `Azure Kubernetes Service RBAC Cluster Admin` | Cluster |

Both are **cluster-scoped** — neither restricts the identity to a specific namespace.

---

## 4. Azure RBAC for Kubernetes — role definitions

AKS Automatic (with `disableLocalAccounts: true`) authorizes kubectl through the **Azure RBAC for Kubernetes Authorization** webhook. The Kubernetes-side `ClusterRole`/`ClusterRoleBinding` objects for these roles do **not** exist in the cluster — the webhook decides allow/deny against the Azure role definition each request.

The Azure `dataActions` for the four built-in AKS roles, as captured via `az role definition list --name "<role>"`:

| Built-in role | ConfigMap dataActions | Other notable scope |
|---|---|---|
| **Azure Kubernetes Service RBAC Reader** | `…/configmaps/read` | read on most resources; **no** secrets read |
| **Azure Kubernetes Service RBAC Writer** | `…/configmaps/*` | full CRUD on most resources, **including secrets** |
| **Azure Kubernetes Service RBAC Admin** | `…/configmaps/*` | adds `roles/*`, `rolebindings/*` (RBAC mgmt within a namespace) |
| **Azure Kubernetes Service RBAC Cluster Admin** | `*/*` (full) | full cluster including ClusterRoles, namespaces, etc. |

> **Critical observation:** None of the role definitions encode a namespace restriction. The blast radius is determined purely by the **scope** of the role assignment.

### Scope behavior

| Role assignment scope | Can write to `kube-system`? |
|---|---|
| Subscription / Resource group / Cluster | ✅ Yes |
| `…/managedClusters/<cluster>/namespaces/kube-system` | ✅ Yes |
| `…/managedClusters/<cluster>/namespaces/<other-ns>` | ❌ No (the webhook checks the requested namespace against the assignment scope) |

---

## 5. AKS Automatic deployment safeguards (Gatekeeper) — what they actually block

All Gatekeeper `Constraint` objects on `zane-auto` were enumerated. Every one of them targets pod/workload concerns. **None** target `ConfigMap` (or `Secret`) resources in `kube-system`:

| Constraint family | Targets |
|---|---|
| `K8sAzureV*ContainerAllowedImages` | Container images |
| `K8sAzureV*HostNamespace` | `hostNetwork`, `hostPID`, `hostIPC` |
| `K8sAzureV*Privilege` | `privileged: true`, `allowPrivilegeEscalation` |
| `K8sAzureV*Capabilities` | Linux capabilities |
| `K8sAzureV*ReadOnlyRootFilesystem` | RO root fs |
| `K8sAzureV*HostFilesystem` | HostPath volumes |
| `K8sAzureV*ContainerNoPrivilege` | Pod security |
| `K8sAzureV*BlockEndpointEditDefault` | Limits `endpoints` edits in the default namespace (not `kube-system`, not configmaps) |

> So the AKS Automatic "deployment safeguards" advertising does **not** include "block configmap writes to kube-system". They are pod/workload guardrails plus a narrow `endpoints/default-namespace` rule.

---

## 6. Why the original assumption is partially right

Microsoft documentation often phrases AKS Automatic as "users can't write to `kube-system`". That phrasing reflects the **intended customer experience** rather than a hard cluster-side block:

1. AKS Automatic disables local accounts → no `cluster-admin` shortcut.
2. The default RBAC tier handed to most teams is `Azure Kubernetes Service RBAC Reader` or namespace-scoped `Writer` — neither of which can mutate `kube-system`.
3. AKS-managed addons (monitor agent, policy, etc.) own their own ConfigMaps in `kube-system`; Microsoft's reconciliation may overwrite user edits.

So the practical statement is:

> "By default, customer identities on AKS Automatic don't have permissions that let them write to `kube-system`."

…which is **not** the same as "the cluster blocks it". A subscription Owner / cluster RBAC Admin can absolutely apply a ConfigMap there — as demonstrated.

---

## 7. Caveats for `ama-metrics-settings-configmap` specifically

The `ama-metrics-settings-configmap` is consumed by the **Azure Monitor managed Prometheus** (ama-metrics) addon, which is **AKS-managed** on Automatic clusters.

- Edits made directly via kubectl **may be reconciled / overwritten** by the AKS RP or addon controller.
- The supported way to configure managed Prometheus on AKS Automatic is via the **AzureMonitorWorkspace / DCR / data collection settings on the resource** (ARM-level), not by hand-editing the ConfigMap.
- A successful `kubectl apply` does **not** mean the change will persist or take effect.

---

## 8. Reproduction commands

```powershell
# 1. Confirm cluster is Automatic
az aks show -g zane-rg-auto -n zane-auto --query "sku" -o json

# 2. Get + convert kubeconfig
az aks get-credentials -g zane-rg-auto -n zane-auto -f $env:TEMP\zane-auto-kubeconfig --overwrite-existing
$env:KUBECONFIG = "$env:TEMP\zane-auto-kubeconfig"
kubelogin convert-kubeconfig -l azurecli

# 3. Apply the configmap
kubectl apply -f $env:TEMP\ama-metrics-test-cm.yaml

# 4. Inspect what was created
kubectl -n kube-system get configmap ama-metrics-settings-configmap -o yaml

# 5. List role assignments for current identity
$me = az ad signed-in-user show --query id -o tsv
az role assignment list --assignee $me `
  --scope /subscriptions/9c17527c-af8f-4148-8019-27bada0845f7/resourceGroups/zane-rg-auto/providers/Microsoft.ContainerService/managedClusters/zane-auto `
  -o table

# 6. Inspect the four built-in AKS RBAC role definitions
$roles = @(
  'Azure Kubernetes Service RBAC Reader',
  'Azure Kubernetes Service RBAC Writer',
  'Azure Kubernetes Service RBAC Admin',
  'Azure Kubernetes Service RBAC Cluster Admin'
)
foreach ($r in $roles) {
  az role definition list --name $r `
    --query "[0].{Name:roleName, DataActions:permissions[0].dataActions, NotDataActions:permissions[0].notDataActions}" -o json
}

# 7. Enumerate Gatekeeper constraints
kubectl get constraints -A
kubectl get constrainttemplates
```

---

## 9. Conclusions

1. **AKS Automatic does not categorically prevent ConfigMap writes to `kube-system`.** The block — when present — is an Azure RBAC scoping decision, not a cluster admission rule.
2. **A cluster-scoped `Azure Kubernetes Service RBAC Cluster Admin` (or higher) can write any resource in `kube-system`.** Verified end-to-end on `zane-auto`.
3. **Default deployment safeguards (Gatekeeper) on AKS Automatic don't cover ConfigMaps in `kube-system`.** They cover pod/workload security.
4. **For ama-metrics specifically**, configuration should be done at the ARM/DCR layer, not by editing the ConfigMap, because the addon controller may reconcile the resource.
5. **To actually restrict a user from writing to `kube-system` on Automatic**, either:
   - Don't grant them cluster-scoped Writer/Admin/Cluster-Admin (use namespace-scoped role assignments), and/or
   - Author a custom Gatekeeper / Azure Policy constraint that targets `ConfigMap` in `kube-system`.
