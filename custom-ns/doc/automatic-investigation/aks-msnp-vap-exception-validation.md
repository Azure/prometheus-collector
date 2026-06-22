# AKS MSNP VAP exception — on-cluster validation

> **Purpose.** The AKS VAP team implemented exceptions in the `aks-managed-protect-system-namespaces` Validating Admission Policy (VAP) so that Azure Monitor (ama-metrics / ama-logs) customer-facing resources can be written to `kube-system` on MSNP-enabled clusters (including AKS Automatic). This document records how we validated those exceptions on a live test cluster: the access setup, the baseline state, the exact policy that was implemented, the test plan, and the results.

| | |
|---|---|
| **Date** | 2026-06-18 |
| **Cluster** | `trang-hosted-eastus2euap` |
| **Resource group** | `trang` |
| **Subscription** | `8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8` (Azure Container Service - Test (AKS Standalone)) |
| **Region** | East US 2 EUAP (canary) |
| **Kubernetes version** | v1.35.5 |
| **Auth model** | Azure RBAC for Kubernetes enabled (`enableAzureRbac: true`); local accounts disabled (`disableLocalAccounts: true`) |
| **Addons present** | ama-metrics (Managed Prometheus) + ama-logs (Container Insights) |

---

## 1. Objective

Verify, end-to-end on a real MSNP cluster, that a customer (non-system identity) **can**:

1. Create/update the ama-metrics customer ConfigMaps in `kube-system` (7 source files → 4 distinct names; see §5.0)
2. Create/update the `ama-metrics-mtls-secret` Secret in `kube-system`
3. Create PodMonitor / ServiceMonitor custom resources in `kube-system`

…and that the exception is **scoped** — i.e., a non-Azure-Monitor ConfigMap or Secret in `kube-system` is **still blocked**.

---

## 2. Access setup (and lessons learned)

Getting authenticated + authorized to this cluster was non-trivial. Recording it because anyone repeating this validation will hit the same walls.

### 2.1 Get credentials

```powershell
az aks get-credentials `
  --subscription 8ecadfc9-d1a3-4ea4-b844-0d9f87e4d7c8 `
  --resource-group trang `
  --name trang-hosted-eastus2euap `
  --overwrite-existing
```

Because `disableLocalAccounts: true`, the `--admin` path is unavailable — you must authenticate as your Azure AD identity and be authorized via Azure RBAC.

### 2.2 Required Azure role

You need **`Azure Kubernetes Service RBAC Cluster Admin`** (role def `b1ff04bb-8a4e-4dc4-8eb5-8693973ce19b`) on the cluster (or an inherited scope).

- This is the **data-plane** role — it grants kubectl access *through* Azure RBAC.
- Do **not** confuse it with `Azure Kubernetes Service Cluster Admin Role` (`0ab0b1a8-…`), which only grants the `--admin` local-credential pull and is useless here (local accounts disabled).

### 2.3 Gotchas encountered

| Symptom | Root cause | Fix |
|---|---|---|
| `Forbidden … User does not have access to the resource in Azure` on every `kubectl` call | Role assignment had not yet been created for the identity (admin "forgot"), then needed propagation time after creation | Have the cluster/RG owner assign **RBAC Cluster Admin** to your token's object ID; wait for propagation |
| Worked on one cluster name, not another | There are two similarly named clusters (`trang-hosted-eastus2euap` vs `trang-hosted-westus2`) in the same RG — make sure you target the right one | Confirm `kubectl config current-context` |
| `az role assignment list` / `az ad user show` fail with `AADSTS530084 … Conditional Access token protection policy` | Microsoft tenant conditional-access blocks Microsoft Graph calls from the CLI app | Use ARM REST (`az rest … /providers/Microsoft.Authorization/roleAssignments`) which does not hit Graph; reference principals by **object ID**, not UPN |
| Portal shows your assignment but `kubectl` still denied | ARM role-assignment eventual consistency (worse in EUAP/canary regions) + AKS authorization webhook caches decisions ~5 min | Wait and retry; re-creating the assignment can force fresh replication |

### 2.4 Confirm access

```powershell
kubectl get namespaces
```

Expected: the 6 namespaces list (below) returns without `Forbidden`.

---

## 3. The policy that was implemented

`kubectl get validatingadmissionpolicy aks-managed-protect-system-namespaces -o yaml`

The policy denies writes to a set of protected namespaces (`matchConstraints.namespaceSelector`, which includes `kube-system`) for all namespaced resources. It is **enforced** — the binding `aks-managed-protect-system-namespaces-binding` has `validationActions: [Deny]`.

Exceptions are encoded as `matchConditions`. A VAP only runs its `validations` when **all** `matchConditions` are true; the single validation here is `expression: "false"` (always deny). Each exception is therefore written as a negated clause `!( <the allowed case> )` — when the request matches an allowed case, that condition becomes false, the policy short-circuits, and the request is **admitted**.

The three Azure Monitor exceptions present on the cluster:

| matchCondition name | Allowed case (CREATE/UPDATE/DELETE in `kube-system`) | Match semantics |
|---|---|---|
| `apply-to-non-azure-monitor-mtls-secret` | `secrets` with name **exactly** `ama-metrics-mtls-secret` | **Exact name** |
| `apply-to-non-azure-monitor-configmap` | `configmaps` whose name **starts with** `ama-metrics-` **or** `container-azm-ms-` | **Prefix** |
| `apply-to-non-azure-monitor-custom-resource` | `podmonitors` / `servicemonitors` in group `azmonitoring.coreos.com`, version `v1` | **Any name; not namespace-restricted** within the protected set |

Plus the pre-existing exemptions for system users/groups (`apply-to-non-exempt-users`, `apply-to-non-exempt-groups`) — notably `system:serviceaccount:kube-system:ama-metrics-serviceaccount` is exempt, so the addon's own service account can always write.

### Observations on the implemented scope

- **ConfigMaps are matched by prefix, not by an exact 4-name list.** Any name under `ama-metrics-*` or `container-azm-ms-*` is allowed. This is broader than the "4 exact names" we asked for, but it conveniently also covers ama-logs (`container-azm-ms-*`) and any future ama-metrics ConfigMap.
- **The Secret exception is a single exact name** (`ama-metrics-mtls-secret`) — narrow, as requested. No other Secret name is exempted.
- **The CR exception is not namespace-restricted in its expression** — it exempts podmonitors/servicemonitors regardless of which protected namespace they target, and regardless of name. (It is still bounded by the policy's `matchConstraints` namespace set.)

---

## 4. Baseline inventory (before testing)

Captured 2026-06-18, before any test resource was created.

**Namespaces (6):** `app-routing-system`, `default`, `gatekeeper-system`, `kube-node-lease`, `kube-public`, `kube-system`. No customer namespaces.

**Addon ConfigMaps in `kube-system`:**

| Name | Age | Addon |
|---|---|---|
| `ama-logs-rs-config` | 17h | ama-logs |
| `container-azm-ms-aks-k8scluster` | 17h | ama-logs / Container Insights |

None of the 4 distinct ama-metrics customer ConfigMap names (from the 7 source files; see §5.0) exist yet — they are optional / customer-created.

**Addon Secrets in `kube-system`:**

| Name | Age | Belongs to |
|---|---|---|
| `aad-msi-auth-token` | 19h | shared (hardcoded MSI token) |
| `ama-logs-secret` | 17h | ama-logs |
| `ama-metrics-operator-targets-client-tls-secret` | 17h | ama-metrics (TA↔collector internal mTLS) |
| `ama-metrics-operator-targets-server-tls-secret` | 17h | ama-metrics (TA↔collector internal mTLS) |
| `extensions-aad-msi-token` | 19h | extensions |
| `omsagent-aad-msi-token` | 19h | ama-logs |

`ama-metrics-mtls-secret` does **not** exist yet — clean to test.

**PodMonitors / ServiceMonitors:** none in any namespace. CRDs `podmonitors.azmonitoring.coreos.com` and `servicemonitors.azmonitoring.coreos.com` are installed (created `2026-06-18T00:08:27Z`).

Because every resource the test creates is currently absent, the create→delete cycle leaves the cluster in its original state.

---

## 5. Test plan

Identity under test: `zanejohnson@microsoft.com` (a normal Azure AD user with RBAC Cluster Admin — **not** a system service account, so the VAP's user/group exemptions do not apply; this faithfully simulates a customer).

### 5.0 Source files → distinct ConfigMap names

The repo's [`otelcollector/configmaps/`](../../../otelcollector/configmaps) directory contains **7 files**, but they resolve to **4 distinct `metadata.name` values** — the four `ama-metrics-settings-configmap*` files are schema/content variants that all carry the **same** ConfigMap name. The apiserver (and therefore the VAP) only sees the `metadata.name`, so testing the 4 distinct names validates all 7 files. And because the VAP matches ConfigMaps by **prefix `ama-metrics-`** (§3), every one of the 7 files is covered regardless of which variant a customer applies.

| Source file | `metadata.name` | Covered by `ama-metrics-` prefix? |
|---|---|---|
| `ama-metrics-settings-configmap.yaml` | `ama-metrics-settings-configmap` | ✅ |
| `ama-metrics-settings-configmap-v1.yaml` | `ama-metrics-settings-configmap` | ✅ |
| `ama-metrics-settings-configmap-v2.yaml` | `ama-metrics-settings-configmap` | ✅ |
| `ama-metrics-settings-configmap-otel.yaml` | `ama-metrics-settings-configmap` | ✅ |
| `ama-metrics-prometheus-config-configmap.yaml` | `ama-metrics-prometheus-config` | ✅ |
| `ama-metrics-prometheus-config-node-configmap.yaml` | `ama-metrics-prometheus-config-node` | ✅ |
| `ama-metrics-prometheus-config-node-windows-configmap.yaml` | `ama-metrics-prometheus-config-node-windows` | ✅ |

→ 7 files, 4 distinct names, 1 prefix rule covering all.

Method: apply a minimal manifest for each case in `kube-system`, record whether admission **allowed** or was **denied** by `aks-managed-protect-system-namespaces`, verify with `kubectl get`, then delete to restore baseline.

| # | Category | Resource (kind / name) | Expected | Rationale (from §3) |
|---|---|---|---|---|
| P1 | ConfigMap | `ama-metrics-settings-configmap` | ALLOW | prefix `ama-metrics-` |
| P2 | ConfigMap | `ama-metrics-prometheus-config` | ALLOW | prefix `ama-metrics-` |
| P3 | ConfigMap | `ama-metrics-prometheus-config-node` | ALLOW | prefix `ama-metrics-` |
| P4 | ConfigMap | `ama-metrics-prometheus-config-node-windows` | ALLOW | prefix `ama-metrics-` |
| P5 | Secret | `ama-metrics-mtls-secret` | ALLOW | exact name |
| P6 | PodMonitor | `vap-validation-podmonitor` | ALLOW | CR kind exempt |
| P7 | ServiceMonitor | `vap-validation-servicemonitor` | ALLOW | CR kind exempt |
| N1 | ConfigMap (negative) | `vap-validation-negative-cm` | DENY | no matching prefix |
| N2 | Secret (negative) | `vap-validation-negative-secret` | DENY | not the exact name |

Optional extra checks (not required, but strengthen the picture):
- **U-prefix:** a ConfigMap named `ama-metrics-foobar` (a name NOT in the repo's 4) → should ALLOW, confirming prefix (not exact-list) semantics.
- **CR in a customer namespace:** a PodMonitor in `default` → should ALLOW (VAP doesn't apply outside protected namespaces at all).

### How to run

Minimal manifests are in [Appendix B](#appendix-b-test-manifests). For each case:

```powershell
kubectl apply -f <manifest>.yaml          # record ALLOW (exit 0) or DENY (admission error)
kubectl get <kind> <name> -n kube-system  # confirm persisted (positive cases)
kubectl delete -f <manifest>.yaml         # clean up
```

A DENY surfaces as:

```
Error from server (Forbidden): error when creating "...": admission webhook / ValidatingAdmissionPolicy
'aks-managed-protect-system-namespaces' denied request: Modification of resources in managed system namespaces is not allowed
```

---

## 6. Test results

> Status: **COMPLETE.** All 9 cases executed on `trang-hosted-eastus2euap` on 2026-06-18; every result matched the prediction (7× ALLOW, 2× DENY). Raw terminal output for each case is in [Appendix C](#appendix-c-raw-terminal-output-execution-trace). Cluster was restored to its baseline (every created resource deleted; denied resources never persisted).

| # | Resource | Expected | Actual | Notes |
|---|---|---|---|---|
| P1 | ConfigMap `ama-metrics-settings-configmap` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P2 | ConfigMap `ama-metrics-prometheus-config` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P3 | ConfigMap `ama-metrics-prometheus-config-node` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P4 | ConfigMap `ama-metrics-prometheus-config-node-windows` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P5 | Secret `ama-metrics-mtls-secret` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P6 | PodMonitor `vap-validation-podmonitor` | ALLOW | ✅ ALLOW | created, verified, deleted |
| P7 | ServiceMonitor `vap-validation-servicemonitor` | ALLOW | ✅ ALLOW | created, verified, deleted |
| N1 | ConfigMap `vap-validation-negative-cm` | DENY | ✅ DENY | blocked by VAP; confirmed NotFound |
| N2 | Secret `vap-validation-negative-secret` | DENY | ✅ DENY | blocked by VAP; confirmed NotFound |
| E1 | ConfigMap — file `ama-metrics-e1-filename.yaml`, `metadata.name: vap-extra-not-prefixed` | DENY | ✅ DENY | file name has prefix, `metadata.name` does not → blocked |
| E2 | ConfigMap — file `zzz-e2-filename.yaml`, `metadata.name: ama-metrics-vap-extra` | ALLOW | ✅ ALLOW | file name lacks prefix, `metadata.name` has it → admitted |

**Result (confirmed by live run):** all 7 positive cases ALLOW, both negative controls DENY — the enforced policy behaves exactly as its text implies. The exception is correctly **scoped**: only `ama-metrics-*` / `container-azm-ms-*` ConfigMaps, the exact-named `ama-metrics-mtls-secret`, and the podmonitor/servicemonitor CR kinds are admitted to `kube-system`; any other ConfigMap or Secret name is still blocked.

---

## 6A. Follow-up: ama-logs ConfigMaps

The same `container-azm-ms-` prefix exception (`apply-to-non-azure-monitor-configmap`, §3) also covers the **ama-logs (Container Insights)** customer-facing ConfigMaps. Validated on 2026-06-19 using the **real** Docker-Provider manifests (`Docker-Provider/kubernetes/container-azm-ms-*config.yaml`), applied as a customer (identity `zanejohnson@microsoft.com`) and deleted afterward to restore baseline.

> Note: there is also a third file, `container-azm-ms-osmconfig.yaml` (`metadata.name: container-azm-ms-osmconfig`), not tested here but covered by the same prefix.

| # | Resource (real file) | `metadata.name` | Expected | Actual | Notes |
|---|---|---|---|---|---|
| AL1 | `container-azm-ms-agentconfig.yaml` | `container-azm-ms-agentconfig` | ALLOW | ✅ ALLOW | created (8 data keys), verified, deleted |
| AL2 | `container-azm-ms-vpaconfig.yaml` | `container-azm-ms-vpaconfig` | ALLOW | ✅ ALLOW | created (1 data key), verified, deleted |

**Result:** both ama-logs customer ConfigMaps are admitted to `kube-system` via the shared `container-azm-ms-` prefix exception, confirming a single rule covers both the ama-metrics and ama-logs ConfigMap surfaces. Raw traces in [Appendix C](#appendix-c-raw-terminal-output-execution-trace).

---

## 6B. Follow-up: RBAC (Role/RoleBinding) placement for credentialed Monitors

On Kubernetes ≥1.36, a Pod/ServiceMonitor that references a **custom-named** credential Secret (basicAuth/bearer/oauth/custom-TLS) requires a namespaced `Role`+`RoleBinding` granting `kube-system:ama-metrics-serviceaccount` read access to that Secret (see PRs [#1493](https://github.com/Azure/prometheus-collector/pull/1493) / [#1536](https://github.com/Azure/prometheus-collector/pull/1536) and `internal/docs/secret-restriction-changes.md`). This sub-test checks **where** that RBAC can be created under the MSNP VAP.

> Note: `Role`/`RoleBinding` are **not** in the VAP exception list (only `ama-metrics-*`/`container-azm-ms-` ConfigMaps, `ama-metrics-mtls-secret`, and the Monitor CRs are). Validated 2026-06-22.

| # | Resource | Namespace | Expected | Actual | Notes |
|---|---|---|---|---|---|
| R1 | Secret + Role + RoleBinding + ServiceMonitor (full credentialed stack) | customer ns `vap-ext-validation` | ALLOW | ✅ ALLOW | all 4 + namespace created; VAP never fires outside protected namespaces |
| R2 | Role `ama-metrics-secrets-reader` | `kube-system` | DENY | ✅ DENY | RBAC not in exception list → blocked |
| R3 | RoleBinding `ama-metrics-secrets-rolebinding` | `kube-system` | DENY | ✅ DENY | RBAC not in exception list → blocked |

**Result — the asymmetry that drives the guidance:** Role/RoleBinding creation is **allowed in a customer namespace** but **denied in `kube-system`**. Therefore a credentialed Monitor that needs RBAC (custom-named secret, K8s ≥1.36) **cannot be fully configured inside `kube-system`** on an MSNP cluster — the Secret-reader Role+RoleBinding step is blocked. Such Monitors must live in a customer namespace, where the Secret, Role, RoleBinding, and Monitor all sit outside the VAP's scope; only the `secrets_access_namespaces` edit to `ama-metrics-settings-configmap` touches `kube-system`, and that is already allowlisted.

> Caveat (unverified): a kube-system Monitor referencing the pre-named `ama-metrics-mtls-secret` *may* avoid the Role/RoleBinding requirement because the ClusterRole `ama-metrics-reader` already grants `get,watch` on that exact name cluster-wide (`ama-metrics-clusterRole.yaml:25-28`, bound via ClusterRoleBinding `ama-metrics-clusterrolebinding`). However that rule grants `get,watch` but **not `list`**, and the target allocator's secret informer typically needs `list`; this case was not tested here.

---

## 6C. Functional test: no-credential Monitor in kube-system works without RBAC

This is an **end-to-end functional** test (beyond admission): does a no-credential ServiceMonitor placed **in `kube-system`** actually get discovered and scraped **without any Role/RoleBinding or `secrets_access_namespaces`**? Validated 2026-06-22.

**Setup:**
- `prometheus-reference-app` (Deployment + Service) in `default` (ports 2112/2113/2114).
- ServiceMonitor `noauth-smon-ks` **in `kube-system`** with `namespaceSelector: [default]`, selecting `app: prometheus-reference-app`, endpoint port `weather-app` (2112), **no `basicAuth`/`tlsConfig`**.
- **No** Role, **no** RoleBinding, **no** `secrets_access_namespaces` entry.

**Verification** (the MSNP VAP `aks-managed-protect-interactive-access` blocks `port-forward`/`exec` to kube-system pods, so the target allocator API was queried from a `busybox` probe pod in `default` hitting the in-cluster service `ama-metrics-operator-targets.kube-system.svc:80`):

| Check | Result |
|---|---|
| TA `/jobs` contains `serviceMonitor/kube-system/noauth-smon-ks/0` | ✅ |
| TA `/scrape_configs` job generated: `role: endpointslice`, `namespaces: [default]`, `scheme: http`, **no `basic_auth`/`tls_config`** | ✅ |
| TA `/jobs/.../targets` discovered real target `10.244.3.248:2112` (= ref-app pod IP) with `endpoint_conditions_ready: true`, allocated to scraper `ama-metrics-6bf79fcd6b-5fkqs` | ✅ |

**Result:** a **no-credential Monitor in `kube-system` is fully functional with zero RBAC** — the target allocator discovers its targets and assigns them to a scraper. The Role/RoleBinding requirement only arises when a Monitor references a credential Secret (and only on K8s ≥1.36). This confirms the clean case for kube-system Monitors: plain/unauthenticated scrape targets need nothing beyond the CR itself. Raw trace in [Appendix C](#appendix-c-raw-terminal-output-execution-trace).

> Note: validated on K8s 1.35.5, but the no-credential path is version-independent (no Secret is ever read), so the conclusion holds for ≥1.36 as well.

---

## 7. Cleanup

```powershell
kubectl delete configmap ama-metrics-settings-configmap ama-metrics-prometheus-config `
  ama-metrics-prometheus-config-node ama-metrics-prometheus-config-node-windows -n kube-system --ignore-not-found
kubectl delete secret ama-metrics-mtls-secret -n kube-system --ignore-not-found
kubectl delete podmonitor vap-validation-podmonitor -n kube-system --ignore-not-found
kubectl delete servicemonitor vap-validation-servicemonitor -n kube-system --ignore-not-found
# Negative-control resources should never have been created; delete only if they somehow exist:
kubectl delete configmap vap-validation-negative-cm -n kube-system --ignore-not-found
kubectl delete secret vap-validation-negative-secret -n kube-system --ignore-not-found
```

After cleanup, re-run the §4 inventory commands to confirm the cluster matches the original baseline.

---

## Appendix A: full implemented VAP (excerpt)

`aks-managed-protect-system-namespaces`, the three Azure Monitor `matchConditions`:

```yaml
matchConditions:
- name: apply-to-non-azure-monitor-mtls-secret
  expression: '!(request.operation in ["CREATE","UPDATE","DELETE"] && request.resource.group == ""
    && request.resource.resource == "secrets" && has(request.namespace) && request.namespace == "kube-system"
    && (!has(request.subResource) || request.subResource == "") && has(request.name)
    && request.name == "ama-metrics-mtls-secret")'
- name: apply-to-non-azure-monitor-configmap
  expression: '!(request.operation in ["CREATE","UPDATE","DELETE"] && request.resource.group == ""
    && request.resource.resource == "configmaps" && has(request.namespace) && request.namespace == "kube-system"
    && (!has(request.subResource) || request.subResource == "") && has(request.name)
    && (request.name.startsWith("container-azm-ms-") || request.name.startsWith("ama-metrics-")))'
- name: apply-to-non-azure-monitor-custom-resource
  expression: '!(request.operation in ["CREATE","UPDATE","DELETE"] && request.resource.group == "azmonitoring.coreos.com"
    && request.resource.version == "v1" && request.resource.resource in ["podmonitors","servicemonitors"])'
validations:
- expression: "false"
  message: Modification of resources in managed system namespaces is not allowed
  reason: Forbidden
```

Binding: `aks-managed-protect-system-namespaces-binding` → `validationActions: [Deny]`.

---

## Appendix B: test manifests

**ConfigMaps** (one per allowlisted name; `data` is placeholder):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-settings-configmap      # repeat for the other 3 names
  namespace: kube-system
data:
  vap-validation: "placeholder"
```

**Secret:**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ama-metrics-mtls-secret
  namespace: kube-system
type: Opaque
stringData:
  ca.crt: "placeholder"
```

**PodMonitor:**

```yaml
apiVersion: azmonitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: vap-validation-podmonitor
  namespace: kube-system
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  selector:
    matchLabels:
      app: vap-validation
  podMetricsEndpoints:
    - port: metrics
```

**ServiceMonitor:**

```yaml
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vap-validation-servicemonitor
  namespace: kube-system
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  selector:
    matchLabels:
      app: vap-validation
  endpoints:
    - port: metrics
```

**Negative controls** (expected DENY):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vap-validation-negative-cm
  namespace: kube-system
data:
  vap-validation: "should-be-denied"
---
apiVersion: v1
kind: Secret
metadata:
  name: vap-validation-negative-secret
  namespace: kube-system
type: Opaque
stringData:
  vap-validation: "should-be-denied"
```

---

## Appendix C: raw terminal output (execution trace)

Each test below shows the exact commands and verbatim output, captured during execution on `trang-hosted-eastus2euap`. Context: `kubectl config current-context` = `trang-hosted-eastus2euap`; identity = `zanejohnson@microsoft.com`.

### P1 — ConfigMap `ama-metrics-settings-configmap` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f ama-metrics-settings-configmap.yaml
configmap/ama-metrics-settings-configmap created

$ kubectl get configmap ama-metrics-settings-configmap -n kube-system -o yaml
apiVersion: v1
data:
  vap-validation: placeholder
kind: ConfigMap
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","data":{"vap-validation":"placeholder"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"ama-metrics-settings-configmap","namespace":"kube-system"}}
  creationTimestamp: "2026-06-18T21:48:37Z"
  name: ama-metrics-settings-configmap
  namespace: kube-system
  resourceVersion: "743630"
  uid: 86781ad3-5239-433a-a40a-2a6c1f8fcb2e

$ kubectl delete configmap ama-metrics-settings-configmap -n kube-system
configmap "ama-metrics-settings-configmap" deleted from kube-system namespace
```

### P2 — ConfigMap `ama-metrics-prometheus-config` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f ama-metrics-prometheus-config.yaml
configmap/ama-metrics-prometheus-config created

$ kubectl get configmap ama-metrics-prometheus-config -n kube-system -o yaml
apiVersion: v1
data:
  vap-validation: placeholder
kind: ConfigMap
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","data":{"vap-validation":"placeholder"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"ama-metrics-prometheus-config","namespace":"kube-system"}}
  creationTimestamp: "2026-06-18T21:51:27Z"
  name: ama-metrics-prometheus-config
  namespace: kube-system
  resourceVersion: "745137"
  uid: 6247445a-4cd7-460a-9587-31eadcb6bc81

$ kubectl delete configmap ama-metrics-prometheus-config -n kube-system
configmap "ama-metrics-prometheus-config" deleted from kube-system namespace
```

### P3 — ConfigMap `ama-metrics-prometheus-config-node` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f ama-metrics-prometheus-config-node.yaml
configmap/ama-metrics-prometheus-config-node created

$ kubectl get configmap ama-metrics-prometheus-config-node -n kube-system -o yaml
apiVersion: v1
data:
  vap-validation: placeholder
kind: ConfigMap
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","data":{"vap-validation":"placeholder"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"ama-metrics-prometheus-config-node","namespace":"kube-system"}}
  creationTimestamp: "2026-06-18T21:54:55Z"
  name: ama-metrics-prometheus-config-node
  namespace: kube-system
  resourceVersion: "747030"
  uid: 2270abce-d8b5-4e05-a05e-b26e40837cbc

$ kubectl delete configmap ama-metrics-prometheus-config-node -n kube-system
configmap "ama-metrics-prometheus-config-node" deleted from kube-system namespace
```

### P4 — ConfigMap `ama-metrics-prometheus-config-node-windows` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f ama-metrics-prometheus-config-node-windows.yaml
configmap/ama-metrics-prometheus-config-node-windows created

$ kubectl get configmap ama-metrics-prometheus-config-node-windows -n kube-system -o yaml
apiVersion: v1
data:
  vap-validation: placeholder
kind: ConfigMap
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","data":{"vap-validation":"placeholder"},"kind":"ConfigMap","metadata":{"annotations":{},"name":"ama-metrics-prometheus-config-node-windows","namespace":"kube-system"}}
  creationTimestamp: "2026-06-18T21:56:04Z"
  name: ama-metrics-prometheus-config-node-windows
  namespace: kube-system
  resourceVersion: "747637"
  uid: 9f40c560-1aaf-4ae7-bbfe-1de31606c2df

$ kubectl delete configmap ama-metrics-prometheus-config-node-windows -n kube-system
configmap "ama-metrics-prometheus-config-node-windows" deleted from kube-system namespace
```

### P5 — Secret `ama-metrics-mtls-secret` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f ama-metrics-mtls-secret.yaml
secret/ama-metrics-mtls-secret created

$ kubectl get secret ama-metrics-mtls-secret -n kube-system -o yaml
apiVersion: v1
data:
  ca.crt: cGxhY2Vob2xkZXI=        # base64 of "placeholder"
kind: Secret
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"Secret","metadata":{"annotations":{},"name":"ama-metrics-mtls-secret","namespace":"kube-system"},"stringData":{"ca.crt":"placeholder"},"type":"Opaque"}
  creationTimestamp: "2026-06-18T21:58:06Z"
  name: ama-metrics-mtls-secret
  namespace: kube-system
  resourceVersion: "748743"
  uid: 576986dc-c71e-40d4-ba70-92f1dfaf71fc
type: Opaque

$ kubectl delete secret ama-metrics-mtls-secret -n kube-system
secret "ama-metrics-mtls-secret" deleted from kube-system namespace
```

### P6 — PodMonitor `vap-validation-podmonitor` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f vap-validation-podmonitor.yaml
podmonitor.azmonitoring.coreos.com/vap-validation-podmonitor created

$ kubectl get podmonitor vap-validation-podmonitor -n kube-system -o yaml
apiVersion: azmonitoring.coreos.com/v1
kind: PodMonitor
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"azmonitoring.coreos.com/v1","kind":"PodMonitor","metadata":{"annotations":{},"name":"vap-validation-podmonitor","namespace":"kube-system"},"spec":{"labelLimit":63,"labelNameLengthLimit":511,"labelValueLengthLimit":1023,"podMetricsEndpoints":[{"port":"metrics"}],"selector":{"matchLabels":{"app":"vap-validation"}}}}
  creationTimestamp: "2026-06-18T22:01:01Z"
  generation: 1
  name: vap-validation-podmonitor
  namespace: kube-system
  resourceVersion: "750341"
  uid: c9011f9a-bbe2-4d76-b9e8-57adbd0d4368
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  podMetricsEndpoints:
  - port: metrics
  selector:
    matchLabels:
      app: vap-validation

$ kubectl delete podmonitor vap-validation-podmonitor -n kube-system
podmonitor.azmonitoring.coreos.com "vap-validation-podmonitor" deleted from kube-system namespace
```

### P7 — ServiceMonitor `vap-validation-servicemonitor` (expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f vap-validation-servicemonitor.yaml
servicemonitor.azmonitoring.coreos.com/vap-validation-servicemonitor created

$ kubectl get servicemonitor vap-validation-servicemonitor -n kube-system -o yaml
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"azmonitoring.coreos.com/v1","kind":"ServiceMonitor","metadata":{"annotations":{},"name":"vap-validation-servicemonitor","namespace":"kube-system"},"spec":{"endpoints":[{"port":"metrics"}],"labelLimit":63,"labelNameLengthLimit":511,"labelValueLengthLimit":1023,"selector":{"matchLabels":{"app":"vap-validation"}}}}
  creationTimestamp: "2026-06-18T22:02:49Z"
  generation: 1
  name: vap-validation-servicemonitor
  namespace: kube-system
  resourceVersion: "751299"
  uid: f164e3f8-bad1-4320-b566-94bff4617459
spec:
  endpoints:
  - port: metrics
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  selector:
    matchLabels:
      app: vap-validation

$ kubectl delete servicemonitor vap-validation-servicemonitor -n kube-system
servicemonitor.azmonitoring.coreos.com "vap-validation-servicemonitor" deleted from kube-system namespace
```

### N1 — ConfigMap `vap-validation-negative-cm` (negative control, expect DENY) → **DENY ✅**

```text
$ kubectl apply -f vap-validation-negative-cm.yaml
Error from server (Forbidden): error when creating "vap-validation-negative-cm.yaml": configmaps "vap-validation-negative-cm" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed

$ kubectl get configmap vap-validation-negative-cm -n kube-system
Error from server (NotFound): configmaps "vap-validation-negative-cm" not found
```

### N2 — Secret `vap-validation-negative-secret` (negative control, expect DENY) → **DENY ✅**

```text
$ kubectl apply -f vap-validation-negative-secret.yaml
Error from server (Forbidden): error when creating "vap-validation-negative-secret.yaml": secrets "vap-validation-negative-secret" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed

$ kubectl get secret vap-validation-negative-secret -n kube-system
Error from server (NotFound): secrets "vap-validation-negative-secret" not found
```

### Bonus — file-name vs `metadata.name` proof (see §6 discussion)

Demonstrates the VAP keys on `metadata.name`, not the file name.

```text
# File 'totally-random-filename.yaml' with metadata.name: ama-metrics-vap-fileproof
$ kubectl apply -f totally-random-filename.yaml
configmap/ama-metrics-vap-fileproof created                # ALLOW (prefix on metadata.name)

# File 'ama-metrics-looks-legit.yaml' with metadata.name: random-not-allowed-cm
$ kubectl apply -f ama-metrics-looks-legit.yaml
Error from server (Forbidden): configmaps "random-not-allowed-cm" is forbidden:
ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' ... denied request:
Modification of resources in managed system namespaces is not allowed   # DENY

$ kubectl delete configmap ama-metrics-vap-fileproof -n kube-system
configmap "ama-metrics-vap-fileproof" deleted from kube-system namespace
```

### E1 — file name has `ama-metrics` prefix, `metadata.name` does not (expect DENY) → **DENY ✅**

Proves the file name is irrelevant: a file *named* `ama-metrics-e1-filename.yaml` whose object is `metadata.name: vap-extra-not-prefixed` is blocked.

```text
$ kubectl apply -f ama-metrics-e1-filename.yaml
Error from server (Forbidden): error when creating "ama-metrics-e1-filename.yaml": configmaps "vap-extra-not-prefixed" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed

$ kubectl get configmap vap-extra-not-prefixed -n kube-system
Error from server (NotFound): configmaps "vap-extra-not-prefixed" not found
```

### E2 — file name lacks `ama-metrics` prefix, `metadata.name` has it (expect ALLOW) → **ALLOW ✅**

Proves the file name is irrelevant in the other direction: a file *named* `zzz-e2-filename.yaml` whose object is `metadata.name: ama-metrics-vap-extra` is admitted.

```text
$ kubectl apply -f zzz-e2-filename.yaml
configmap/ama-metrics-vap-extra created

$ kubectl get configmap ama-metrics-vap-extra -n kube-system -o yaml
apiVersion: v1
data:
  vap-validation: E2 file=non-prefixed meta=ama-metrics* -> expect ALLOW
kind: ConfigMap
metadata:
  creationTimestamp: "2026-06-18T22:11:45Z"
  name: ama-metrics-vap-extra
  namespace: kube-system
  resourceVersion: "756080"
  uid: ec781b35-ac42-467b-a275-0265fdb7c888

$ kubectl delete configmap ama-metrics-vap-extra -n kube-system
configmap "ama-metrics-vap-extra" deleted from kube-system namespace
```

**E1 + E2 conclusion:** the VAP exception matches strictly on the object's `metadata.name` (surfaced as `request.name`). The local file name has no bearing on admission in either direction.

### AL1 — ama-logs `container-azm-ms-agentconfig` (real file, expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f Docker-Provider/kubernetes/container-azm-ms-agentconfig.yaml
configmap/container-azm-ms-agentconfig created

$ kubectl get configmap container-azm-ms-agentconfig -n kube-system
NAME                           DATA   AGE
container-azm-ms-agentconfig   8      3s

$ kubectl get configmap container-azm-ms-agentconfig -n kube-system -o jsonpath="{.data}"   # keys:
agent-settings
alertable-metrics-configuration-settings
config-version
integrations
log-data-collection-settings
metric_collection_settings
prometheus-data-collection-settings
schema-version

$ kubectl delete configmap container-azm-ms-agentconfig -n kube-system
configmap "container-azm-ms-agentconfig" deleted from kube-system namespace
```

### AL2 — ama-logs `container-azm-ms-vpaconfig` (real file, expect ALLOW) → **ALLOW ✅**

```text
$ kubectl apply -f Docker-Provider/kubernetes/container-azm-ms-vpaconfig.yaml
configmap/container-azm-ms-vpaconfig created

$ kubectl get configmap container-azm-ms-vpaconfig -n kube-system
NAME                         DATA   AGE
container-azm-ms-vpaconfig   1      3s

$ kubectl get configmap container-azm-ms-vpaconfig -n kube-system -o jsonpath="{.data}"   # keys:
NannyConfiguration

$ kubectl delete configmap container-azm-ms-vpaconfig -n kube-system
configmap "container-azm-ms-vpaconfig" deleted from kube-system namespace
```

### R1 — full credentialed stack in a customer namespace (expect ALLOW) → **ALLOW ✅**

Secret + Role + RoleBinding + ServiceMonitor (with `basicAuth`) in non-protected namespace `vap-ext-validation`. The VAP does not apply outside protected namespaces.

```text
$ kubectl apply -f 00-namespace.yaml
namespace/vap-ext-validation created
$ kubectl apply -f 01-secret.yaml
secret/basic-auth-creds created
$ kubectl apply -f 02-role.yaml
role.rbac.authorization.k8s.io/ama-metrics-secrets-reader created
$ kubectl apply -f 03-rolebinding.yaml
rolebinding.rbac.authorization.k8s.io/ama-metrics-secrets-rolebinding created
$ kubectl apply -f 04-servicemonitor.yaml
servicemonitor.azmonitoring.coreos.com/basic-auth-smon created

$ kubectl describe rolebinding ama-metrics-secrets-rolebinding -n vap-ext-validation
Role:
  Kind:  Role
  Name:  ama-metrics-secrets-reader
Subjects:
  Kind            Name                        Namespace
  ----            ----                        ---------
  ServiceAccount  ama-metrics-serviceaccount  kube-system
```

### R2 — Role in `kube-system` (expect DENY) → **DENY ✅**

```text
$ kubectl apply -f role-ks.yaml
Error from server (Forbidden): error when creating "role-ks.yaml": roles.rbac.authorization.k8s.io "ama-metrics-secrets-reader" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed

$ kubectl get role ama-metrics-secrets-reader -n kube-system
Error from server (NotFound): roles.rbac.authorization.k8s.io "ama-metrics-secrets-reader" not found
```

### R3 — RoleBinding in `kube-system` (expect DENY) → **DENY ✅**

```text
$ kubectl apply -f rolebinding-ks.yaml
Error from server (Forbidden): error when creating "rolebinding-ks.yaml": rolebindings.rbac.authorization.k8s.io "ama-metrics-secrets-rolebinding" is forbidden: ValidatingAdmissionPolicy 'aks-managed-protect-system-namespaces' with binding 'aks-managed-protect-system-namespaces-binding' denied request: Modification of resources in managed system namespaces is not allowed

$ kubectl get rolebinding ama-metrics-secrets-rolebinding -n kube-system
Error from server (NotFound): rolebindings.rbac.authorization.k8s.io "ama-metrics-secrets-rolebinding" not found
```

### E2E — no-credential ServiceMonitor in kube-system, functional via target allocator → **WORKS, no RBAC ✅**

```text
# ServiceMonitor in kube-system (no auth), ref app in default, NO Role/RoleBinding, NO secrets_access_namespaces
$ kubectl apply -f noauth-smon-ks.yaml
servicemonitor.azmonitoring.coreos.com/noauth-smon-ks created

# port-forward/exec to kube-system pods is blocked by VAP 'aks-managed-protect-interactive-access',
# so the TA API was queried from a busybox probe pod in 'default':
$ kubectl exec -n default ta-probe -- wget -q -O- http://ama-metrics-operator-targets.kube-system.svc.cluster.local:80/jobs
... "serviceMonitor/kube-system/noauth-smon-ks/0":{"_link":"/jobs/serviceMonitor%2Fkube-system%2Fnoauth-smon-ks%2F0/targets"} ...

$ kubectl exec -n default ta-probe -- wget -q -O- http://.../scrape_configs   # job for our Monitor:
"serviceMonitor/kube-system/noauth-smon-ks/0": {
  "job_name":"serviceMonitor/kube-system/noauth-smon-ks/0",
  "kubernetes_sd_configs":[{"role":"endpointslice","namespaces":{"names":["default"]}}],
  "scheme":"http"   # <-- no basic_auth / tls_config block
  ...
}

$ kubectl exec -n default ta-probe -- wget -q -O- http://.../jobs/serviceMonitor%2Fkube-system%2Fnoauth-smon-ks%2F0/targets
"ama-metrics-6bf79fcd6b-5fkqs":{ "targets":[{"targets":["10.244.3.248:2112"],   # <-- real ref-app pod IP discovered
   "labels":{ ... "__meta_kubernetes_endpointslice_endpoint_conditions_ready":"true",
              "__meta_kubernetes_pod_ip":"10.244.3.248", ... }}]}

# cross-check: ref app pod IP
$ kubectl get pods -n default -l app=prometheus-reference-app -o jsonpath="{.items[0].status.podIP}"
10.244.3.248
```












