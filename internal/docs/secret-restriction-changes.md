# Scoped Secrets Access for Pod/ServiceMonitors (Branch: `rashmi/secret-restriction`)

## Summary

This change removes **cluster-wide secrets access** from the `ama-metrics` ClusterRole and replaces it with **namespace-scoped** RBAC, giving users explicit control over which namespaces the target allocator can read secrets from. This is a security improvement: previously, the target allocator had unrestricted `get`/`list`/`watch` on secrets across **all** namespaces in the cluster.

### What Changed

| Area | Before | After |
|------|--------|-------|
| **ClusterRole** | Included `get`, `list`, `watch` on `secrets` cluster-wide | On Kubernetes >= 1.36, cluster-wide secrets verbs **removed**. On < 1.36, kept for backward compatibility. |
| **Namespace RBAC** | N/A (cluster-wide) | User creates namespaced `Role` + `RoleBinding` in each namespace where secrets are needed |
| **Target allocator config** | Metadata informer watched secrets in all namespaces | Metadata informer scoped to configured namespaces only |

### Default Behavior

- **Kubernetes < 1.36:** Cluster-wide secrets access is retained in the ClusterRole for backward compatibility. No additional RBAC is needed.
- **Kubernetes >= 1.36:** Cluster-wide secrets access is removed from the ClusterRole. By default (no configuration), the target allocator watches **no** namespaces for secrets. Users must configure `secrets_access_namespaces` and create the appropriate Role+RoleBinding in each namespace (see [User Instructions](#user-instructions-basic-auth-with-podservicemonitors) below).

---

## Files Changed (11 files, 73 additions, 10 deletions)

| File | Change |
|------|--------|
| `ama-metrics-clusterRole.yaml` | Cluster-wide `secrets` `get`/`list`/`watch` conditionally included only on Kubernetes < 1.36 via `semverCompare` |
| `ama-metrics-settings-configmap*.yaml` (3 files) | Added `secrets_access_namespaces = ""` setting under `prometheus-collector-settings` |
| `definitions.go` | Added `SecretsAccessNamespaces []string` to `ConfigProcessor` struct |
| `tomlparser-prometheus-collector-settings.go` | Parses `secrets_access_namespaces` from configmap (comma-separated), writes `AZMON_SECRETS_ACCESS_NAMESPACES` env var |
| `configuration-reader-builder/main.go` | Reads `AZMON_SECRETS_ACCESS_NAMESPACES` env var, populates target allocator config |
| `otel_config.go` | Added `SecretsAccessNamespaces` to shared `Config` struct |
| `config.go` (otel-allocator) | Added `SecretsAccessNamespaces` to allocator `Config` struct |
| `promOperator.go` | Builds `secretsAllowList` from `cfg.SecretsAccessNamespaces`, passes to `NewMetadataInformerFactory` instead of using the monitoring allowList |

### Config Flow

```
ama-metrics-settings-configmap (TOML)
  secrets_access_namespaces = "kube-system,my-app"
        │
        ▼
tomlparser-prometheus-collector-settings.go
  → comma-split → cp.SecretsAccessNamespaces
  → writes env var AZMON_SECRETS_ACCESS_NAMESPACES
        │
        ▼
configuration-reader-builder/main.go
  → reads env var → populates Config.SecretsAccessNamespaces
  → writes to target allocator YAML config
        │
        ▼
promOperator.go (target allocator)
  → builds secretsAllowList map from SecretsAccessNamespaces
  → NewMetadataInformerFactory(secretsAllowList, ...)
  → secrets list/watch scoped to those namespaces only
```

---

## User Instructions: Basic Auth with Pod/ServiceMonitors

When a `ServiceMonitor` or `PodMonitor` uses `basicAuth`, it references a Kubernetes `Secret` containing the username and/or password. The target allocator needs permission to read that secret. With this change, you must explicitly grant access for each namespace where such secrets exist.

### Step 1: Create the Basic Auth Secret

Create a secret in the namespace where your application runs:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-basic-auth
  namespace: my-app          # <-- your application namespace
type: Opaque
data:
  username: <base64-encoded-username>
  password: <base64-encoded-password>
```

### Step 2: Reference the Secret in Your ServiceMonitor/PodMonitor

**ServiceMonitor example:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-service-monitor
  namespace: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  endpoints:
    - port: metrics
      basicAuth:
        username:
          name: my-basic-auth    # Secret name from Step 1
          key: username
        password:
          name: my-basic-auth
          key: password
```

**PodMonitor example:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: my-pod-monitor
  namespace: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  podMetricsEndpoints:
    - port: metrics
      basicAuth:
        username:
          name: my-basic-auth
          key: username
        password:
          name: my-basic-auth
          key: password
```

### Step 3: Configure `secrets_access_namespaces`

Edit (or create) the `ama-metrics-settings-configmap` to include the namespace(s) where your secrets live:

```yaml
kind: ConfigMap
apiVersion: v1
metadata:
  name: ama-metrics-settings-configmap
  namespace: kube-system
data:
  prometheus-collector-settings: |-
    cluster_alias = ""
    secrets_access_namespaces = "kube-system,my-app"
```

> **Note:** Use a comma-separated list for multiple namespaces. Include `kube-system` if you also have secrets there. The setting takes effect after the next pod restart (the configmap is re-read on startup).

### Step 4: Create RBAC in Each Namespace (Kubernetes >= 1.36)

On Kubernetes >= 1.36, the ClusterRole no longer grants cluster-wide secrets access. You must create a `Role` and `RoleBinding` in **every** namespace listed in `secrets_access_namespaces` (including `kube-system` if needed).

> **Kubernetes < 1.36:** This step is **not required** — the ClusterRole still includes cluster-wide secrets access for backward compatibility.

Apply the following in each namespace. Replace `my-app` with the target namespace:

**Create a Role:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ama-metrics-secrets-reader
  namespace: my-app              # <-- repeat for each namespace
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
```

**Create a RoleBinding:**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ama-metrics-secrets-rolebinding
  namespace: my-app              # <-- repeat for each namespace
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: kube-system       # <-- SA lives in kube-system
roleRef:
  kind: Role
  name: ama-metrics-secrets-reader
  apiGroup: rbac.authorization.k8s.io
```

> **Cross-namespace note:** The RoleBinding in `my-app` references the ServiceAccount in `kube-system`. This is valid Kubernetes RBAC — a RoleBinding can reference a subject from any namespace.

### Step 5: Verify

After the `ama-metrics` pod restarts:

1. Check the target allocator logs for:
   ```
   SecretsAccessNamespaces from configmap: [kube-system my-app]
   ```
2. Confirm your ServiceMonitor/PodMonitor targets appear in the target allocator's discovered targets.
3. Verify that scrape results include metrics from the basic-auth-protected endpoints.

---

## Quick Reference: Multiple Namespaces

If you have secrets in `my-app`, `backend`, and `monitoring`:

1. **ConfigMap setting:**
   ```toml
   secrets_access_namespaces = "kube-system,my-app,backend,monitoring"
   ```

2. **RBAC (>= 1.36 only):** Create the Role + RoleBinding (from Step 4) in each of `my-app`, `backend`, and `monitoring`.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| ServiceMonitor targets not discovered | Secret not readable by target allocator | Ensure namespace is in `secrets_access_namespaces` AND Role+RoleBinding exists |
| `forbidden: User "system:serviceaccount:kube-system:ama-metrics-serviceaccount" cannot list resource "secrets"` in TA logs | Missing RBAC in the namespace | Create Role+RoleBinding in that namespace (Step 4) |
| Targets discovered but scrape fails with 401 | Secret exists but credentials wrong | Verify the Secret's `data` fields have correct base64-encoded values |
| Setting ignored after configmap update | Pod hasn't restarted | Restart the `ama-metrics` pod to pick up new configmap values |
