# How `ama-metrics-mtls-secret` is used at scrape time

> **Scope.** This doc explains how TLS material flows into the ama-metrics scraper at runtime, and why the same TLS information can be supplied through **two completely different routes** depending on how the customer authored their scrape configuration.
>
> **Why this matters.** The two routes have different namespace constraints, different RBAC implications, and different rotation semantics. Conflating them leads to incorrect documentation (e.g., "PodMonitor must be in `kube-system`") and to incorrect MSNP / VAP allowlist asks. This doc grounds those discussions in the actual runtime mechanics.

---

## TL;DR

| | Route A — ConfigMap path | Route B — CRD path |
|---|---|---|
| Customer authoring surface | Raw scrape config in `ama-metrics-prometheus-config` ConfigMap (`kube-system`) | `PodMonitor` / `ServiceMonitor` CR (any namespace) |
| Secret name | **Must** be `ama-metrics-mtls-secret` | Any name |
| Secret namespace | **Must** be `kube-system` | Same namespace as the Monitor (CRD schema enforces this) |
| How addon discovers the Secret | Hardcoded `secretName` in pod volume spec (5 places in helm chart) | TA finds the Monitor → Monitor's `tlsConfig` references the Secret by name |
| How addon reads the Secret | kubelet projects the Secret as files at pod boot | Target allocator does live K8s `GET secret` API call |
| Where the bytes live inside the ama-metrics pod | Files under `/etc/prometheus/certs/<key>` | Inline `ca:` / `cert:` / `key:` strings in the in-memory scrape config |
| Customer references it from scrape config via | `tls_config.{ca_file, cert_file, key_file}` file paths | `tlsConfig.{ca.secret, cert.secret, keySecret}` SecretKeySelector |
| RBAC needed (K8s ≥ 1.36) | None — pod already has the volume mount baked in | Role + RoleBinding in the Monitor's ns, granting `kube-system:ama-metrics-serviceaccount` `get/list/watch` on Secrets; plus the ns listed in `secrets_access_namespaces` |
| Rotation semantics | kubelet refreshes the mounted files when the Secret updates | TA's `handleSecretUpdate` callback regenerates the scrape config and pushes it to scrapers |

**The two routes never interact at runtime.** Each produces its own scrape job in the final Prometheus config, with its own self-contained TLS settings. Customers can use one, the other, both, or neither — and the routes can coexist without ambiguity.

---

## Setup we're walking through

We'll trace what happens when a customer has **both** of these in their cluster:

| | Secret A | Secret B |
|---|---|---|
| Name | `ama-metrics-mtls-secret` | `my-app-mtls` (could be any name) |
| Namespace | `kube-system` | `app-ns` |
| Used by | Raw scrape config in `ama-metrics-prometheus-config` | A PodMonitor in `app-ns` |

Both happen to hold the same kind of data: keys `ca.crt`, `client.crt`, `client.key` with cert/key bytes. The interesting question — *do they collide?* — is answered at the end ([§4](#4-do-the-two-routes-collide-no---heres-why)).

---

## 1. Route A: `kube-system/ama-metrics-mtls-secret` — used via **file mount**

### 1.1 At ama-metrics pod start

The addon's helm chart pre-declares a volume in the pod spec, hardcoding the Secret name:

```yaml
volumes:
  - name: certs
    secret:
      secretName: ama-metrics-mtls-secret   # hardcoded, kube-system
volumeMounts:
  - name: certs
    mountPath: /etc/prometheus/certs
```

(See: `ama-metrics-daemonset.yaml:416`, `ama-metrics-daemonset.yaml:678`, `ama-metrics-deployment.yaml:466`, `ama-metrics-targetallocator.yaml:286`, plus the matching ClusterRole `resourceNames` entry in `ama-metrics-clusterRole.yaml:27`.)

kubelet reads the Secret from kube-system at pod boot and projects each key in `data` as a separate file under `/etc/prometheus/certs/`:

```
/etc/prometheus/certs/
├── ca.crt        # ← Secret data["ca.crt"], base64-decoded
├── client.crt    # ← Secret data["client.crt"], base64-decoded
└── client.key    # ← Secret data["client.key"], base64-decoded
```

### 1.2 Customer's scrape config references the file paths

Customer puts the following into the `ama-metrics-prometheus-config` ConfigMap (which must live in `kube-system`):

```yaml
scrape_configs:
  - job_name: my-https-target
    scheme: https
    tls_config:
      ca_file:   /etc/prometheus/certs/ca.crt        # file path on disk
      cert_file: /etc/prometheus/certs/client.crt
      key_file:  /etc/prometheus/certs/client.key
    static_configs:
      - targets: ['my-endpoint.example:8443']
```

The config processor merges this into the final Prometheus config handed to the scraper.

### 1.3 At scrape time

When Prometheus inside the ama-metrics pod scrapes `my-endpoint.example:8443`:

1. It calls `os.Open("/etc/prometheus/certs/ca.crt")` → reads the bytes → builds the TLS trust pool.
2. Similarly opens `client.crt` and `client.key` → loads the client cert.
3. Opens a TLS connection to the target with mutual auth.
4. Scrapes metrics over the secured channel.

**Key property.** The Kubernetes API is *not involved at scrape time.* Cert bytes flow:

```
K8s Secret object
   ↓ (at pod boot or Secret update)
kubelet projects to tmpfs
   ↓
/etc/prometheus/certs/<key>
   ↓ (at scrape time)
Prometheus reads the file
```

Rotation: kubelet automatically refreshes the mounted files when the Secret is updated (within ~60s); Prometheus picks up the new bytes on the next scrape.

---

## 2. Route B: `app-ns/my-app-mtls` — used via **K8s API fetch + inline config injection**

### 2.1 Customer creates the Secret in `app-ns`

```yaml
apiVersion: v1
kind: Secret
metadata: { name: my-app-mtls, namespace: app-ns }
type: Opaque
data:
  ca.crt:     <base64-encoded ca bytes>
  client.crt: <base64-encoded cert bytes>
  client.key: <base64-encoded key bytes>
```

The name `my-app-mtls` is arbitrary — it could be anything. The name `ama-metrics-mtls-secret` is **only** meaningful for the kube-system instance (where it's hardcoded into pod volume mounts). In other namespaces, there is no special name.

### 2.2 Customer creates a PodMonitor in `app-ns` referencing it

```yaml
apiVersion: azmonitoring.coreos.com/v1
kind: PodMonitor
metadata: { name: my-pod-monitor, namespace: app-ns }
spec:
  selector: { matchLabels: { app: my-app } }
  podMetricsEndpoints:
    - port: metrics
      scheme: https
      tlsConfig:
        ca:        { secret: { name: my-app-mtls, key: ca.crt } }
        cert:      { secret: { name: my-app-mtls, key: client.crt } }
        keySecret: { name: my-app-mtls, key: client.key }
```

Note that the `SecretKeySelector` fields (`name`, `key`, `optional`) **have no `namespace:` field** — this is enforced by the CRD schema. The reference always resolves in the Monitor's own namespace.

### 2.3 Customer also sets up RBAC + secrets_access_namespaces (K8s ≥ 1.36)

On K8s 1.36+, the addon's ClusterRole no longer grants cluster-wide secrets access. The customer must:

```yaml
# In app-ns: grant the ama-metrics SA permission to read Secrets
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata: { name: ama-metrics-secrets-reader, namespace: app-ns }
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata: { name: ama-metrics-secrets-rolebinding, namespace: app-ns }
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: kube-system
roleRef:
  kind: Role
  name: ama-metrics-secrets-reader
  apiGroup: rbac.authorization.k8s.io
```

And edit the settings ConfigMap in kube-system to include the namespace:

```yaml
# kube-system/ama-metrics-settings-configmap
data:
  prometheus-collector-settings: |-
    secrets_access_namespaces = "kube-system,app-ns"
```

### 2.4 At Monitor reconciliation time

The **target allocator** (TA) — a separate process running in its own pod in `kube-system` — does this:

1. Sees the new PodMonitor via its CRD informer.
2. For each `SecretKeySelector` in `tlsConfig`, performs a live K8s API call:
   `GET /api/v1/namespaces/app-ns/secrets/my-app-mtls`
   (requires the RBAC from §2.3 + `app-ns` listed in `secrets_access_namespaces`).
3. Pulls out `data["ca.crt"]`, `data["client.crt"]`, `data["client.key"]` (base64-decoded into raw bytes).
4. Generates a Prometheus scrape job. Because TA uses **`prometheus.WithInlineTLSConfig()`** ([`otel-allocator/internal/watcher/promOperator.go:107`](../../../otelcollector/otel-allocator/internal/watcher/promOperator.go)), the cert bytes are **embedded directly into the generated YAML** rather than written to files:
   ```yaml
   scrape_configs:
     - job_name: podMonitor/app-ns/my-pod-monitor/0
       scheme: https
       tls_config:
         ca: |
           -----BEGIN CERTIFICATE-----
           MIID...                                  # ca bytes inline
           -----END CERTIFICATE-----
         cert: |
           -----BEGIN CERTIFICATE-----
           MIID...                                  # client cert bytes inline
           -----END CERTIFICATE-----
         key: |
           -----BEGIN PRIVATE KEY-----
           MIIE...                                  # key bytes inline
           -----END PRIVATE KEY-----
       kubernetes_sd_configs:
         - role: pod
           namespaces: { names: [app-ns] }
   ```
5. Hands this generated config to the scraper (via TA's HTTP API, which scrapers poll periodically).

### 2.5 At scrape time

Prometheus inside the ama-metrics pod:

1. Reads the inline `ca:` / `cert:` / `key:` bytes from its in-memory parsed scrape config.
2. Builds the TLS trust pool and client cert from those bytes (no file read).
3. Opens a TLS connection to the discovered pod's `metrics` port.
4. Scrapes.

**Key property.** The Secret bytes **never touch the ama-metrics pod's filesystem.** Flow:

```
K8s Secret object in app-ns
   ↓ (at TA reconciliation)
TA does GET secret via K8s API
   ↓ (cert bytes in TA process memory)
TA generates scrape config with bytes inlined
   ↓ (HTTP poll from scraper)
ama-metrics scraper holds bytes in its parsed config
   ↓ (at scrape time)
Prometheus uses bytes directly to build TLS context
```

Rotation: TA's `handleSecretUpdate` callback ([`promOperator.go:470`](../../../otelcollector/otel-allocator/internal/watcher/promOperator.go)) fires on Secret update events from its informer, refreshes the asset store, regenerates the config, and notifies scrapers via the `notifyEvents` channel. Scrapers pull the new config on their next poll.

---

## 3. Side-by-side summary

| Aspect | Route A — kube-system Secret (file mount) | Route B — app-ns Secret (CRD path) |
|---|---|---|
| How addon discovers the Secret | Hardcoded in pod spec (`secretName: ama-metrics-mtls-secret`) | Indirect — TA finds Monitor → Monitor references Secret by name |
| How addon reads the Secret | kubelet projects as files at mount time | TA does live K8s API `GET secret` |
| Where the bytes live in the pod | Files under `/etc/prometheus/certs/` | In-memory scrape config (string fields) |
| How the customer's scrape job references it | `*_file:` paths in raw config | `tlsConfig.{ca,cert,keySecret}` SecretKeySelector in CRD spec |
| What Prometheus actually does at scrape time | `os.Open("/etc/prometheus/certs/ca.crt")` | Reads bytes from its parsed scrape config |
| Rotation mechanism | kubelet refreshes mount on Secret update | TA's `handleSecretUpdate` regenerates config and pushes to scraper |
| RBAC needed | None — pod already has the volume mount baked in | Role + RoleBinding in Monitor's ns (1.36+); ns listed in `secrets_access_namespaces` |
| Constraint on Secret namespace | Must be `kube-system` (kubelet volume mount: same-ns rule) | Must equal Monitor's namespace (CRD schema: no `namespace:` field in SecretKeySelector) |
| Constraint on Secret name | Must be `ama-metrics-mtls-secret` (hardcoded in helm chart) | Any name |
| Failure mode if Secret missing | Pod fails to start (volume mount fails) OR scrape fails with "no such file" | Monitor's targets silently fail to be discovered; TA log shows "secret not found" or RBAC `forbidden` |

---

## 4. Do the two routes collide? **No** — here's why

Suppose a customer has both Secrets present **and** has both routes configured (some raw scrape config in the ConfigMap referencing `/etc/prometheus/certs/...`, plus a PodMonitor in `app-ns` referencing `my-app-mtls`). Could the ama-metrics pod get "confused"?

No. Walking through every potential collision point:

1. **Could TA accidentally pull the kube-system Secret for the app-ns Monitor?**
   No — the SecretKeySelector has no `namespace:` field. It strictly resolves to the Monitor's own ns (`app-ns`). The kube-system instance is invisible to it.

2. **Could the raw-config job accidentally pull from `app-ns`?**
   No — `ca_file:` is a literal file path inside the pod's filesystem. It can only read what's mounted at that path, which is the kube-system Secret.

3. **What if both Secrets are named `ama-metrics-mtls-secret`?**
   Doesn't matter — they're separate K8s objects in separate namespaces. Each route looks up its own object. Same-name is fine; cross-namespace fetches don't happen.

4. **What if the keys inside collide (e.g., both have `ca.crt`)?**
   Still fine — Route A reads `/etc/prometheus/certs/ca.crt` (kube-system bytes), Route B reads `app-ns/<secret>.ca.crt` (app-ns bytes) and inlines them into the generated YAML. The two sets of bytes end up in different scrape job blocks. No merge step exists.

5. **Does TA inject anything into `/etc/prometheus/certs/`?**
   No. The CRD path doesn't touch the filesystem at all (because of `WithInlineTLSConfig()`). Zero risk of a file-path collision.

6. **Could one scrape target be scraped twice (once per route)?**
   Yes, if the customer points both a raw scrape config AND a PodMonitor at the same endpoint. That would produce **duplicate metrics with different `job` labels** — but that's customer misconfiguration, not ama-metrics confusion. Prometheus handles it correctly; the customer just sees double-counted data. Not a TLS issue.

**Root cause of the non-collision:** the two routes are architecturally separate code paths that produce **independent scrape job blocks** in the final Prometheus config. Each job carries its own `tls_config`. There is no global trust pool, no merge logic, no "which Secret wins" question to answer.

---

## 5. Why this matters for MSNP / VAP discussions

The two routes have very different implications for the MSNP allowlist ask:

- **Route A** is the *only* reason the **kube-system instance of `ama-metrics-mtls-secret`** must be allowlisted under MSNP. The ConfigMap path has no alternative — the file mount is hardcoded. If a customer needs TLS-secured scraping via raw scrape config, they need to be able to create/update this Secret in `kube-system`.

- **Route B** does **not** require any kube-system allowlist entry for its credentials. The Secret lives in the customer's own namespace (which the VAP doesn't protect). The Monitor itself can live anywhere except the 20 protected namespaces — which is fine if customers place it in their own namespace.

- **The naming overlap is a coincidence with consequences.** Customers see examples that use the name `ama-metrics-mtls-secret` for both routes and assume it must be the same Secret object. The public docs reinforce this by saying things like "the PodMonitor should be created in `kube-system`" (which technically applies only to the combined-credentials shortcut where you want to reuse the kube-system file mount). On MSNP, this guidance is broken: customers cannot put their Monitor in `kube-system`, so they must use Route B with a Secret in their own namespace — but the public docs do not yet describe this path clearly.

- **Public docs need an MSNP-specific clarification.** Something like: "If you are running on a cluster where `kube-system` writes are restricted (e.g., MSNP / AKS Automatic), create a Secret in your own namespace (any name) and reference it from a PodMonitor in the same namespace. The kube-system `ama-metrics-mtls-secret` is only required for the raw scrape-config path."

---

## Appendix: Code references

| Component | Path | Purpose |
|---|---|---|
| Volume mount, daemonset Linux | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml:416` | Hardcoded `secretName: ama-metrics-mtls-secret` |
| Volume mount, daemonset Windows | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-daemonset.yaml:678` | Hardcoded `secretName: ama-metrics-mtls-secret` |
| Volume mount, deployment | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-deployment.yaml:466` | Hardcoded `secretName: ama-metrics-mtls-secret` |
| Volume mount, target allocator | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-targetallocator.yaml:286` | Hardcoded `secretName: ama-metrics-mtls-secret` |
| ClusterRole resourceName | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-clusterRole.yaml:27` | `resourceNames: ["aad-msi-auth-token", "ama-metrics-mtls-secret"]` |
| Inline TLS config generation | `otelcollector/otel-allocator/internal/watcher/promOperator.go:107` | `prometheus.WithInlineTLSConfig()` option passed to the config generator |
| Secret asset store | `otelcollector/otel-allocator/internal/watcher/promOperator.go:112` | `assets.NewStoreBuilder(client.CoreV1(), client.CoreV1())` — TA's in-memory cache of Secret bytes |
| Secret update handler | `otelcollector/otel-allocator/internal/watcher/promOperator.go:470` | `handleSecretUpdate` — triggers config regeneration on Secret change |
| Namespace-scoped secrets watcher | `otelcollector/otel-allocator/internal/watcher/promOperator.go:69-73` | "If SecretsAccessNamespaces is not configured, no namespaces are watched for secrets" |
| K8s 1.36 RBAC gate | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-clusterRole.yaml:28-33` | `{{- if semverCompare "<1.36.0" .Values.global.commonGlobals.Versions.Kubernetes }}` — cluster-wide secrets rule removed at 1.36+ |
| Configuration reader 1.36 switch | `otelcollector/configuration-reader-builder/main.go:143` | `!parsedKubeVersion.LessThan(utilversion.MustParseSemantic("v1.36.0"))` — switches between cluster-wide and namespace-scoped watching |
| PodMonitor CRD tlsConfig schema | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-podmonitor-crd.yaml:285` | All Secret refs are `SecretKeySelector` with `name`/`key`/`optional` — no `namespace:` field |
| ServiceMonitor CRD tlsConfig schema | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-servicemonitor-crd.yaml:247` | Same shape as PodMonitor |
| Public docs (TLS-based scraping) | https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-crd#tls-based-scraping | Customer-facing instructions for both routes (currently inconsistent re: Monitor placement) |
