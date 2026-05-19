# ama-metrics + addon-token-adapter: Custom Namespace Validation

**Date:** 2026-05-19
**Author:** Zane Johnson
**Cluster:** `zane-metrics-custom-ns` (RG `zane-custom-ns`, sub `9c17527c-af8f-4148-8019-27bada0845f7`, region westeurope)
**Test namespace:** `ama-metrics-zane-test`

## TL;DR

- Moving `ama-metrics` (and its `addon-token-adapter` sidecar) out of
  `kube-system` is **technically supported** by the AKS team — the image
  runs in any namespace, and the token secret is provisioned in both old
  and new namespaces during migration.
- The **real risk** is that `addon-token-adapter` requires Linux
  capabilities `NET_ADMIN` + `NET_RAW` (used to install an iptables DNAT
  chain that redirects IMDS calls to the sidecar). Any cluster policy
  that restricts capabilities on non-`kube-system` namespaces will block
  the sidecar from starting.
- Reproduced on `zane-metrics-custom-ns` two ways:
  1. **Pod Security Admission** — labeling the namespace with
     `pod-security.kubernetes.io/enforce=restricted` blocks the
     DaemonSet/Deployment with `forbidden ... capabilities ... must drop ... NET_RAW`.
  2. **Azure Policy / Gatekeeper** — assigning the built-in
     "Kubernetes cluster containers should only use allowed capabilities"
     (`c26596ff-4d70-4e6a-9a30-c2506bd2f80c`) at scope `zane-custom-ns`
     in **Deny** mode with `allowedCapabilities: []` and the standard
     Defender exclusion list (no `ama-metrics-zane-test`) produces
     Gatekeeper denials for both `addon-token-adapter` and
     `prometheus-collector` containers.
- **Recommendation for customers running pod-security / capability
  policies:** explicitly exclude the new ama-metrics namespace, or
  whitelist `NET_ADMIN` + `NET_RAW` there. Default Defender posture
  excludes only `kube-system`, `gatekeeper-system`, `azure-arc`,
  `azuredefender`, `mdc`, `azure-extensions-usage-system`, so any
  custom namespace is in scope by default.
- The `privileged: true` variant of the sidecar (`msi-adapter:1.29.3`)
  is **Arc-extension / OpenShift only** — not deployed on AKS standard,
  so `K8sAzureV2NoPrivilege` is not a concern for this migration.

---

## Background

Our team is planning to move the `ama-metrics` agent out of `kube-system` into a
dedicated namespace. The `addon-token-adapter` sidecar that ama-metrics depends
on must come along to the new namespace.

The AKS team confirmed:

1. **Token secret** — during migration the token secret is provisioned in both
   `kube-system` and the new namespace; after migration completes, the
   `kube-system` copy is no longer provisioned.
2. **Image runtime** — `addon-token-adapter` itself can run in any namespace,
   **but** it requires the Linux capabilities `NET_ADMIN` and `NET_RAW`. If a
   customer has cluster policies that block those capabilities, the new
   namespace must be whitelisted.

## Why token-adapter needs NET_ADMIN / NET_RAW

Source: `msi/addon-token-adapter/` in `Azure/aks-rp`.

The sidecar performs two functions (per `msi/addon-token-adapter/readme.md`):

1. Watches the Kubernetes secret containing the addon's AAD/MSI access token
   and caches it.
2. Intercepts the addon's outbound MSI token requests to the IMDS endpoint
   (`169.254.169.254`) and serves the cached token instead.

Interception is implemented by manipulating **iptables rules inside the pod's
network namespace** (`msi/addon-token-adapter/cmd/iptables.go`). On startup it
runs the equivalent of:

```
iptables -t nat -N aad-metadata
iptables -t nat -A aad-metadata -p tcp -d 169.254.169.254 --dport 80 \
         -j DNAT --to-destination 127.0.0.1:<adapterPort>
iptables -t nat -I PREROUTING 1 -j aad-metadata
iptables -t nat -I OUTPUT    1 -j aad-metadata
```

Linux requires:

- **`NET_ADMIN`** — to add/modify netfilter rules and chains. Without it the
  iptables syscalls fail with `EPERM`.
- **`NET_RAW`** — to use raw/packet sockets, which iptables/netfilter
  manipulation and the underlying libraries depend on.

(The Windows variant `msi/addon-token-adapter-win/netsh/netsh.go` does the
equivalent with `netsh portproxy` and does not need Linux capabilities.)

## Validation: reproducing the customer-policy failure

We used **Kubernetes built-in Pod Security Admission (PSA)** at the `baseline`
enforcement level to simulate a strict customer policy. PSA is built into the
API server (since v1.25); no add-on, no Gatekeeper, no Azure Policy required.

### Steps run

```powershell
# 1. Get cluster credentials
az aks get-credentials -g zane-custom-ns -n zane-metrics-custom-ns `
  --subscription 9c17527c-af8f-4148-8019-27bada0845f7 --overwrite-existing

# 2. Enforce baseline on the target namespace
kubectl label namespace ama-metrics-zane-test `
  pod-security.kubernetes.io/enforce=baseline `
  pod-security.kubernetes.io/enforce-version=latest --overwrite

# 3. Roll the workloads so new pods hit admission
kubectl -n ama-metrics-zane-test rollout restart `
  daemonset/ama-metrics-node deployment/ama-metrics

# 4. Inspect events
kubectl -n ama-metrics-zane-test get events --sort-by=.lastTimestamp
```

### Pre-existing pods

PSA only evaluates pods at **admission time** (creation). Pods already running
before the label was applied continued to run unaffected. The breakage only
materializes when the DaemonSet/Deployment controllers try to create
replacement pods — e.g., on a chart upgrade, node reboot, or rollout.

### Error returned by the API server

For every new pod the controllers tried to create:

```
Error creating: pods "ama-metrics-node-9l4m6" is forbidden:
violates PodSecurity "baseline:latest":
  non-default capabilities
    (container "addon-token-adapter" must not include "NET_ADMIN", "NET_RAW"
     in securityContext.capabilities.add),
  hostPath volumes
    (volumes "host-log-containers", "host-log-pods",
     "anchors-mariner", "anchors-ubuntu")
```

This was surfaced two ways:

- `FailedCreate` events on `daemonset/ama-metrics-node` and the
  `replicaset` owned by `deployment/ama-metrics`.
- An inline `kubectl` warning during `rollout restart`:
  > Warning: would violate PodSecurity "baseline:latest": non-default
  > capabilities (container "addon-token-adapter" must not include "NET_ADMIN",
  > "NET_RAW" ...), hostPath volumes (...).

## Findings

1. **Confirmed:** strict customer pod-security policies will reject the
   `addon-token-adapter` sidecar in any namespace other than ones they have
   explicitly exempted (which today is typically only `kube-system`).
2. **New concern (not mentioned by AKS team):** the same `baseline` profile
   also rejects the **hostPath volumes** that the `ama-metrics-node` DaemonSet
   uses (`host-log-containers`, `host-log-pods`, `anchors-mariner`,
   `anchors-ubuntu`). Customers strict enough to block `NET_ADMIN`/`NET_RAW`
   are almost certainly also blocking `hostPath`.
3. **Silent failure mode:** because PSA only blocks new admissions, existing
   pods keep running. Migration could appear successful until the next
   rollout, upgrade, node reboot, or autoscaling event — at which point new
   pods will stop scheduling.

## Recommendations / open items

- Pick a **stable, well-known namespace name** for ama-metrics and publish it
  ahead of migration so customers can pre-update their security policies.
- When filing the dependency request to the AKS team, ask them to confirm
  customer guidance covers **both**:
  - `securityContext.capabilities.add: [NET_ADMIN, NET_RAW]` on the sidecar
  - `hostPath` volume mounts on the DaemonSet pod
- Consider documenting recommended `PodSecurity` namespace label for the new
  namespace (e.g., `enforce=privileged`) so customers don't accidentally
  apply `baseline`/`restricted` and break the addon.
- For the token-secret provisioning (item #1 of the AKS team's reply), follow
  up separately to understand the RP-side changes needed to provision the
  secret into a non-`kube-system` namespace and to track the deprecation of
  the `kube-system` copy.

## Revert (for the test cluster)

```powershell
kubectl label namespace ama-metrics-zane-test `
  pod-security.kubernetes.io/enforce- `
  pod-security.kubernetes.io/enforce-version- --overwrite

kubectl -n ama-metrics-zane-test rollout restart `
  daemonset/ama-metrics-node deployment/ama-metrics
```

---

## Validation #2 — Azure Policy / Gatekeeper (Defender for Cloud parity)

PSA is a useful proxy, but real customers usually enforce capability rules via
Azure Policy (Gatekeeper). This validation reproduces the failure with the
exact mechanism Microsoft Defender for Cloud uses, which is what most strict
customers have enabled.

### Setup observation

Defender for Cloud's `SecurityCenterBuiltIn` initiative already targets this
cluster and **already audits** the violation today. The constraint
`azurepolicy-k8sazurev3allowedcapabilities-a2d2d13756bc1747ca08` (from policy
set `1f3afdf9-d0c9-4c3d-847f-89da613e70a8`, reference
`AllowedCapabilitiesInKubernetesCluster`, version 6.2.0) is live with
`enforcementAction: dryrun`, `parameters.allowedCapabilities: []`, and
`excludedNamespaces`:

```
kube-system, gatekeeper-system, azure-arc,
azuredefender, mdc, azure-extensions-usage-system
```

`ama-metrics-zane-test` is **not** excluded. Its status already records
22 audit violations, including:

```
container <addon-token-adapter>     has a disallowed capability
container <addon-token-adapter-win> has a disallowed capability
container <prometheus-collector>    has a disallowed capability
```

So Defender is already telling customers this is non-compliant — it just isn't
blocking pod admission today.

### Reproduction (deny mode)

A second policy assignment was layered on at the RG scope using the matching
built-in (Defender's assignment was left untouched):

- Built-in: `c26596ff-4d70-4e6a-9a30-c2506bd2f80c`
  ("Kubernetes cluster containers should only use allowed capabilities")
- Assignment name: `block-caps-token-adapter-test`
- Scope: `/subscriptions/9c17527c-.../resourceGroups/zane-custom-ns`
- `effect: deny`, `allowedCapabilities: []`, same `excludedNamespaces` as Defender

Force-synced via `kubectl -n kube-system rollout restart deploy/azure-policy`.
A second constraint appeared after ~2 minutes:

```
NAME                                                             ENFORCEMENT-ACTION   TOTAL-VIOLATIONS
azurepolicy-k8sazurev3allowedcapabilities-24598bbe30bf0b2f58dd   deny
azurepolicy-k8sazurev3allowedcapabilities-a2d2d13756bc1747ca08   dryrun               22
```

Rolled `daemonset/ama-metrics-node` and `deployment/ama-metrics`. New pod
creation was rejected by the Gatekeeper validating webhook:

```
DaemonSet/ama-metrics-node  FailedCreate
Error creating: admission webhook "validation.gatekeeper.sh" denied the request:
  [azurepolicy-k8sazurev3allowedcapabilities-24598bbe30bf0b2f58dd]
    container <addon-token-adapter> has a disallowed capability.
    Allowed capabilities are []. For more information, visit
    https://aka.ms/aks/deployment-safeguards
  [azurepolicy-k8sazurev3allowedcapabilities-24598bbe30bf0b2f58dd]
    container <prometheus-collector> has a disallowed capability.
    Allowed capabilities are []. For more information, visit
    https://aka.ms/aks/deployment-safeguards
```

DaemonSet became `1/2 ready` (the pre-existing pod survived; new pods could
not be created). Deployment retained its old replicas for the same reason.

### Additional finding (Azure Policy only)

`prometheus-collector` was **also** rejected for declaring a disallowed
capability. PSA `baseline` didn't catch this because PSA only blocks the
`NET_ADMIN`/`NET_RAW` privileged caps; this Azure Policy is parameterised
with `allowedCapabilities: []`, which rejects **any** capability beyond
the default set, so `prometheus-collector`'s own non-default cap also trips
the rule. This is something customers running tight Defender configs may
hit even before they touch token-adapter.

## Updated findings

3. **Confirmed under real Azure Policy / Gatekeeper:** customer clusters with
   Defender for Cloud's capability constraint moved from `dryrun` to `deny`
   (a common hardening step) will block `ama-metrics` pods in any namespace
   they have not explicitly excluded — `kube-system` is excluded by Defender's
   default, the new customer namespace is not.
4. **Defender already reports this as a finding today** (audit/dryrun), so
   customers who scan their compliance dashboard will already see token-adapter
   listed as non-compliant in the new namespace.
5. **`prometheus-collector` shares the same risk** when capabilities are
   restricted via Azure Policy (vs. PSA baseline).

## Teardown

```powershell
az policy assignment delete `
  --name block-caps-token-adapter-test `
  --scope /subscriptions/9c17527c-af8f-4148-8019-27bada0845f7/resourceGroups/zane-custom-ns

kubectl -n kube-system rollout restart deploy/azure-policy
kubectl -n ama-metrics-zane-test rollout restart daemonset/ama-metrics-node deployment/ama-metrics
```

### Why the denial fires — confirmed cap list

The deny rule is `allowedCapabilities: []`, so any container that adds **any**
Linux capability is rejected. Live spec on the cluster:

```
$ kubectl -n ama-metrics-zane-test get ds/ama-metrics-node \
    -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{" "}{.securityContext.capabilities.add}{"\n"}{end}'
prometheus-collector  ["DAC_OVERRIDE"]
addon-token-adapter   ["NET_ADMIN","NET_RAW"]
```

- `addon-token-adapter` is denied for `NET_ADMIN` + `NET_RAW`
  (needed for the `iptables -t nat` DNAT rule that redirects
  `169.254.169.254` → `127.0.0.1`).
- `prometheus-collector` is denied for `DAC_OVERRIDE`
  (separate issue, same mechanism).

The pod spec that adds these caps is owned by the ama-metrics /
prometheus-collector Helm chart (Container Insights team), not by aks-rp.

The current `addon-token-adapter:master.*` image used by ama-metrics is
**not** privileged. Its securityContext (from the live DaemonSet) is:

```yaml
securityContext:
  capabilities:
    drop:
      - ALL
    add:
      - NET_ADMIN
      - NET_RAW
```

`drop: ALL` strips the kernel-default capability set; `add` re-grants only
`NET_ADMIN` and `NET_RAW` (least-privilege pattern). Azure Policy /
Gatekeeper still denies it because the constraint inspects
`capabilities.add[]` and `allowedCapabilities: []` rejects anything in that
list.

Note: the ama-metrics Helm chart (in `Azure/prometheus-collector`) has a
second sidecar variant — `arc-msi-adapter` using
`mcr.microsoft.com/azurearck8s/msi-adapter:1.29.3` with
`privileged: true` — but it is gated on
`isArcExtension` + `distribution == "openshift"`, so it only renders on
Arc-connected OpenShift clusters. On an AKS standard cluster (what we
tested) it is not deployed, and `K8sAzureV2NoPrivilege` (built-in
`95edb821-ddaf-4404-9732-666045e056b4`) is not relevant here.

---

## How to reproduce on a new cluster

End-to-end recipe to recreate both validations from scratch. Replace the
placeholders at the top with your own values.

### 0. Prerequisites

- An AKS cluster you can `az aks get-credentials` against
- `kubectl`, `az`, PowerShell (commands below use PowerShell line-continuation
  backticks — swap for `\` if running in bash)
- Cluster Kubernetes version **v1.25+** (for PSA)
- For Validation #2 only: the **`azure-policy`** add-on enabled on the cluster
  (`az aks enable-addons -a azure-policy ...`)

```powershell
$SUB    = "<your-subscription-id>"
$RG     = "<your-resource-group>"
$AKS    = "<your-cluster-name>"
$NS     = "ama-metrics-zane-test"   # any non-kube-system namespace
```

### 1. Deploy ama-metrics into a custom namespace

Install/enable the Azure Monitor metrics (managed Prometheus) addon however
your environment normally does it, then move the workloads into a non-default
namespace. The simplest path is to enable the addon (which lands in
`kube-system`) and then redeploy the chart into `$NS`, or — for a pure test —
copy the live manifests and re-apply them.

Quick copy approach (works for reproducing the policy denial; not a
production migration path):

```powershell
az aks get-credentials -g $RG -n $AKS --subscription $SUB --overwrite-existing

kubectl create namespace $NS

# Copy the live DaemonSet + Deployment into the new namespace, stripping
# server-side metadata so they re-create cleanly.
kubectl -n kube-system get ds ama-metrics-node -o yaml `
  | Select-String -NotMatch -Pattern '^\s+(uid:|resourceVersion:|creationTimestamp:|generation:|selfLink:)' `
  | ForEach-Object { $_ -replace 'namespace: kube-system', "namespace: $NS" } `
  | Set-Content ama-metrics-node.yaml
kubectl apply -f ama-metrics-node.yaml

kubectl -n kube-system get deploy ama-metrics -o yaml `
  | Select-String -NotMatch -Pattern '^\s+(uid:|resourceVersion:|creationTimestamp:|generation:|selfLink:)' `
  | ForEach-Object { $_ -replace 'namespace: kube-system', "namespace: $NS" } `
  | Set-Content ama-metrics.yaml
kubectl apply -f ama-metrics.yaml
```

You should now have:

```
kubectl -n $NS get ds,deploy
NAME                              DESIRED   CURRENT   READY
daemonset.apps/ama-metrics-node   2         2         2
deployment.apps/ama-metrics       2/2       2         2
```

### 2. Validation #1 — Pod Security Admission (PSA)

```powershell
# Enforce baseline on the test namespace
kubectl label namespace $NS `
  pod-security.kubernetes.io/enforce=baseline `
  pod-security.kubernetes.io/enforce-version=latest --overwrite

# Force new pod creation
kubectl -n $NS rollout restart daemonset/ama-metrics-node deployment/ama-metrics

# Watch the failures
kubectl -n $NS get events --sort-by=.lastTimestamp | Select-String FailedCreate
```

**Expected:** `FailedCreate` events naming `addon-token-adapter` for
`NET_ADMIN`/`NET_RAW`, plus `hostPath volumes` violations. Pre-existing pods
keep running; new pods are blocked.

**Revert PSA:**

```powershell
kubectl label namespace $NS `
  pod-security.kubernetes.io/enforce- `
  pod-security.kubernetes.io/enforce-version- --overwrite
kubectl -n $NS rollout restart daemonset/ama-metrics-node deployment/ama-metrics
```

### 3. Validation #2 — Azure Policy / Gatekeeper (Defender parity)

```powershell
$scope = "/subscriptions/$SUB/resourceGroups/$RG"

# Layer a deny-mode assignment of the same built-in Defender uses in dryrun
az policy assignment create `
  --name "block-caps-token-adapter-test" `
  --display-name "Block disallowed caps (token-adapter test)" `
  --scope $scope `
  --policy "c26596ff-4d70-4e6a-9a30-c2506bd2f80c" `
  --params '{
    "effect":              { "value": "deny" },
    "allowedCapabilities": { "value": [] },
    "excludedNamespaces":  { "value": ["kube-system","gatekeeper-system","azure-arc","azuredefender","mdc","azure-extensions-usage-system"] }
  }'

# Force the azure-policy addon to pull the new assignment immediately
kubectl -n kube-system rollout restart deploy/azure-policy
kubectl -n kube-system rollout status  deploy/azure-policy --timeout=120s

# Poll until a deny constraint shows up (typically ~2 min, may take 5–15)
for ($i = 0; $i -lt 30; $i++) {
  $hits = kubectl get k8sazurev3allowedcapabilities --no-headers 2>$null `
          | Select-String -Pattern "deny"
  if ($hits) { $hits; break }
  Start-Sleep -Seconds 15
}

# Trigger new pod creation
kubectl -n $NS rollout restart daemonset/ama-metrics-node deployment/ama-metrics

# Observe the denial
kubectl -n $NS get events --sort-by=.lastTimestamp `
  | Select-String "validation.gatekeeper.sh"
```

**Expected denial message:**

```
admission webhook "validation.gatekeeper.sh" denied the request:
  [azurepolicy-k8sazurev3allowedcapabilities-<hash>]
  container <addon-token-adapter> has a disallowed capability.
  Allowed capabilities are []. For more information, visit
  https://aka.ms/aks/deployment-safeguards
  [azurepolicy-k8sazurev3allowedcapabilities-<hash>]
  container <prometheus-collector> has a disallowed capability.
  ...
```

`daemonset/ama-metrics-node` will sit at `1/2 ready` until teardown.

**Optional — also reproduce `privileged: true` denial** (separate built-in):

```powershell
az policy assignment create `
  --name "block-privileged-token-adapter-test" `
  --display-name "Block privileged containers (token-adapter test)" `
  --scope $scope `
  --policy "95edb821-ddaf-4404-9732-666045e056b4" `
  --params '{
    "effect":             { "value": "deny" },
    "excludedNamespaces": { "value": ["kube-system","gatekeeper-system","azure-arc","azuredefender","mdc","azure-extensions-usage-system"] }
  }'
kubectl -n kube-system rollout restart deploy/azure-policy
```

**Teardown Validation #2:**

```powershell
az policy assignment delete --name "block-caps-token-adapter-test"        --scope $scope --subscription $SUB
az policy assignment delete --name "block-privileged-token-adapter-test"  --scope $scope --subscription $SUB 2>$null

kubectl -n kube-system rollout restart deploy/azure-policy
kubectl -n kube-system rollout status  deploy/azure-policy --timeout=120s

# Wait for the deny constraint(s) to disappear
for ($i = 0; $i -lt 24; $i++) {
  $deny = (kubectl get k8sazurev3allowedcapabilities --no-headers 2>$null `
           | Select-String -Pattern "deny" | Measure-Object -Line).Lines
  if ($deny -eq 0) { break }
  Start-Sleep -Seconds 15
}

kubectl -n $NS rollout restart daemonset/ama-metrics-node deployment/ama-metrics
kubectl -n $NS get pods,ds,deploy
```

### 4. Sanity checks while reproducing

```powershell
# Confirm the live container caps (proves WHY the policy fires)
kubectl -n $NS get ds/ama-metrics-node `
  -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"  "}{.securityContext.capabilities.add}{"\n"}{end}'

# Confirm token-adapter is also privileged
kubectl -n $NS get ds/ama-metrics-node `
  -o jsonpath='{range .spec.template.spec.containers[*]}{.name}{"  privileged="}{.securityContext.privileged}{"\n"}{end}'

# See which Azure Policy constraints are currently synced
kubectl get constraints

# See Defender's pre-existing audit findings
kubectl get k8sazurev3allowedcapabilities `
  -o jsonpath='{.items[?(@.spec.enforcementAction=="dryrun")].status.totalViolations}'
```

### Notes / gotchas

- Pre-existing pods always survive — both PSA and Gatekeeper only block at
  pod **creation** time. To see the failure you must roll the workload.
- Azure Policy add-on syncs roughly every 15 minutes. Restarting the
  `azure-policy` Deployment in `kube-system` forces an immediate sync.
- If `prometheus-collector` shows up as denied alongside `addon-token-adapter`
  in Validation #2, that's expected — it adds `DAC_OVERRIDE`, which is also
  outside the `allowedCapabilities: []` allow-list.
- Defender for Cloud's `SecurityCenterBuiltIn` initiative may already have
  the same constraint deployed in `dryrun`; leave it alone and just layer
  the deny-mode assignment on top.
