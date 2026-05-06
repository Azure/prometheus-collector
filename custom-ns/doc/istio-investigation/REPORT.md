# ama-metrics + AKS Istio Add-on: Compatibility Report

**Date:** May 6, 2026
**Cluster under test:** `zane-istio-test` / RG `zane-istio` / Sub `9c17527c-af8f-4148-8019-27bada0845f7`
**Versions:** AKS 1.33.8, Istio add-on revision `asm-1-27`, ama-metrics image `6.27.0-main-04-10-2026-196e83aa`

---

## TL;DR

**Yes, ama-metrics in `kube-system` can scrape user app metrics in an Istio-enabled namespace — including under STRICT mTLS — *as long as the customer relies on the default pod-annotation-based scraping*.**

It breaks in two cases:

| Breaking case | mTLS mode dependence | What happens | Available options |
|---|---|---|---|
| **(a)** Customer disables Istio's Prometheus merging (`ISTIO_META_ENABLE_PROMETHEUS_MERGE=false`) | **Independent** — breaks the same way under PERMISSIVE or STRICT | ama-metrics still scrapes `:15020` successfully, but the response no longer contains app metrics — only Envoy stats | • Re-enable merging (recommended)<br>• Or apply Fix A (port-level PERMISSIVE) / Fix B (`excludeInboundPorts`) and point a custom job back at the app port |
| **(b)** Customer uses a custom scrape config / `PodMonitor` / `ServiceMonitor` that hardcodes the application port (e.g. `:8080`) | **STRICT only** — works on PERMISSIVE | Target is `down`, `read: connection reset by peer` (Envoy rejects plaintext under STRICT mTLS) | • **Fix C (recommended):** rewrite the scrape config to target `:15020/stats/prometheus`<br>• **Fix A:** `PeerAuthentication` with `portLevelMtls: 8080: PERMISSIVE` for the app port<br>• **Fix B:** pod annotation `traffic.sidecar.istio.io/excludeInboundPorts: "8080"` |

All three fixes were verified end-to-end on a STRICT-mTLS namespace. See *Key findings #5* and *Appendix E*.

---

## Question

ama-metrics runs in `kube-system` (no Istio sidecar). Customer app runs in its own namespace (e.g. `app`) with Istio injection enabled. **Can ama-metrics still scrape the app's metrics?**

---

## Key findings

### 1. Default behavior — works automatically
When a namespace is added to the mesh, Istio's mutating webhook **rewrites prometheus annotations** on every injected pod:
- `prometheus.io/port`: `8080` → `15020`
- `prometheus.io/path`: `/metrics` → `/stats/prometheus`

ama-metrics then scrapes Envoy's "merged metrics" endpoint at `:15020/stats/prometheus`, which returns **app metrics + Envoy stats** in one payload. → *See Appendix A.*

### 2. STRICT mTLS does not break the default flow
A `PeerAuthentication mode: STRICT` policy in the namespace blocks plaintext scrapes of the app port (`:8080`), but:
- The merged endpoint `:15020/stats/prometheus` is **explicitly exempt** from mTLS enforcement by Istio's design (the sidecar does not intercept this port).
- Annotation-based scraping continues to work — health = `up`. → *See Appendix B.*

### 3. Failure mode #1 — disabling Istio's prometheus merge
Setting `proxy.istio.io/config: "{ proxyMetadata: { ISTIO_META_ENABLE_PROMETHEUS_MERGE: \"false\" } }"` causes the merged endpoint to return **only Envoy stats** (no app metrics). Annotations are still rewritten to `:15020`, so ama-metrics scrapes the merged endpoint successfully — but the app's metrics are missing. → *See Appendix C.*

### 4. Failure mode #2 — custom scrape config hardcoded to app port
A custom Prometheus scrape config (or `PodMonitor`/`ServiceMonitor`) that targets the app port directly bypasses annotation rewriting and is rejected by Envoy under STRICT mTLS. Result: target is `down` with `read: connection reset by peer`. → *See Appendix D.*

### 5. Three available fixes for failure mode #2
All three were verified end-to-end — *see Appendix E.*

| # | Fix | Where | Result |
|---|---|---|---|
| A | `PeerAuthentication` with `portLevelMtls: 8080: PERMISSIVE` | `app` namespace | ✅ |
| B | Pod annotation `traffic.sidecar.istio.io/excludeInboundPorts: "8080"` | App workload | ✅ |
| C | Rewrite scrape config to use `:15020/stats/prometheus` | ama-metrics custom config | ✅ |

### 6. Recommendation

| Customer scenario | Recommended action |
|---|---|
| Default ama-metrics + standard Istio install | **No action needed** — works out of the box |
| Custom scrape config / `PodMonitor` / `ServiceMonitor` | **Fix C** — point the job at `:15020/stats/prometheus` instead of the app port |
| Customer disabled prometheus-merge | Re-enable merge, or apply Fix A/B for the app port |
| Cannot edit scrape config (locked by other tooling) | **Fix A** (port-level PERMISSIVE) — minimal blast radius |

---

# Appendices (evidence)

All commands run against cluster `zane-istio-test`. ama-metrics has 2 replicas and the target allocator shards targets between them, so we query both replicas.

## Appendix A — Default behavior, no STRICT mTLS

**Files:** `01-app-namespace.yaml`, `02-ama-metrics-settings.yaml`

**Setup:**
- ama-metrics: 2 replicas in `kube-system`, default config
- ama-metrics scrape config: pod-annotation-based scraping enabled for namespace `app` via `02-ama-metrics-settings.yaml` (`podannotationnamespaceregex = "app"`)
- App workload: namespace `app` from `01-app-namespace.yaml` — deployment `prom-example` (image `quay.io/brancz/prometheus-example-app:v0.5.0`, port 8080) with annotations `prometheus.io/scrape=true`, `prometheus.io/port=8080`, `prometheus.io/path=/metrics`
- Istio: namespace `app` labeled `istio.io/rev=asm-1-27` (sidecar injection enabled)
- mTLS mode: **PERMISSIVE** (Istio add-on default — no `PeerAuthentication` applied)
- Pod restarted after labeling so sidecar is injected

**Annotations after sidecar injection** (rewritten by Istio):
```json
{
  "prometheus.io/scrape": "true",
  "prometheus.io/port": "15020",
  "prometheus.io/path": "/stats/prometheus",
  "sidecar.istio.io/status": "...initContainers: [istio-init, istio-proxy]..."
}
```

**Active scrape target:**
```
scrapePool : kubernetes-pods
health     : up
scrapeUrl  : http://10.244.0.64:15020/stats/prometheus
lastError  :
```

**Merged endpoint contents** (both app and Envoy metrics):
```
$ wget -qO- http://10.244.0.64:15020/stats/prometheus | grep -E '^version|^envoy_server_uptime'
envoy_server_uptime{} 265
version{version="v0.5.0"} 1     # ← app metric
```

---

## Appendix B — STRICT mTLS applied, default behavior still works

**Files:** `01-app-namespace.yaml`, `02-ama-metrics-settings.yaml`, `03-peerauth-strict.yaml`

**Setup:**
- All of Appendix A's setup (ama-metrics default config, pod-annotation scraping for `app`, sidecar-injected `prom-example` pod)
- **Plus:** `PeerAuthentication strict-mtls` applied to namespace `app` from `03-peerauth-strict.yaml`:

```yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata: { name: strict-mtls, namespace: app }
spec: { mtls: { mode: STRICT } }
```

**Annotation-based scrape (via `:15020`):**
```
scrapePool : kubernetes-pods
health     : up
scrapeUrl  : http://10.244.0.99:15020/stats/prometheus
lastError  :
```

**Direct plaintext scrape of `:8080` from outside mesh — fails as expected:**
```
$ wget -T 8 -qO- http://10.244.0.99:8080/metrics
wget: error getting response: Connection reset by peer
```

→ Confirms `:15020` is exempt from mTLS, app port `:8080` is not.

**Sources** (Istio's own docs confirm the `:15020` exemption is by design):
- [Istio — Ports used by Istio](https://istio.io/latest/docs/ops/deployment/application-requirements/#ports-used-by-istio): table lists port `15020` ("Merged Prometheus telemetry from Istio agent, Envoy, and application") with `Captured: No` — i.e. the sidecar does not intercept this port, so `PeerAuthentication` mTLS rules don't apply.
- [Istio — Prometheus integration, Option 1: Metrics merging](https://istio.io/latest/docs/ops/integrations/prometheus/#option-1-metrics-merging): "merged metrics will be scraped from `:15020/stats/prometheus`. This option exposes all the metrics in plain text."

---

## Appendix C — Disabling Istio prometheus-merge breaks app metrics

> **Note:** This failure is **independent of mTLS mode** — it would reproduce identically under PERMISSIVE. STRICT was active during this test only because it carried over from Appendix B.

**Files:** `01-app-namespace.yaml`, `02-ama-metrics-settings.yaml`, `03-peerauth-strict.yaml`, `04-no-annotation-rewrite.yaml`

**Setup:**
- All of Appendix B's setup (sidecar-injected `prom-example`, STRICT `PeerAuthentication` in `app`, pod-annotation scraping enabled)
- **Plus:** deployment patched from `04-no-annotation-rewrite.yaml` to add the pod-level proxy override that disables prometheus-merge:

```yaml
# applied via pod annotation on the prom-example deployment
proxy.istio.io/config: '{ "proxyMetadata": { "ISTIO_META_ENABLE_PROMETHEUS_MERGE": "false" } }'
```

- Pod recreated so the new proxy config takes effect
- ama-metrics scrape config: still default annotation-based

**Pod annotations** (annotations still rewritten):
```
prometheus.io/port: 15020
prometheus.io/path: /stats/prometheus
```

**Merged endpoint now contains only Istio/Envoy metrics:**
```
$ wget -qO- http://10.244.0.207:15020/stats/prometheus | head -20
# HELP istio_agent_cert_expiry_seconds ...
# HELP istio_agent_go_gc_duration_seconds ...
# (no `version{version="v0.5.0"}` — app metrics are absent)
```

**Direct `:8080` still blocked by STRICT mTLS:**
```
$ wget -T 8 -qO- http://10.244.0.207:8080/metrics
wget: error getting response: Connection reset by peer
```

→ ama-metrics scrape succeeds but customer loses their app metrics silently.

---

## Appendix D — Custom scrape config hardcoded to `:8080` fails

> **Note:** This failure **is mTLS-mode dependent** — under PERMISSIVE the same scrape would succeed. The `connection reset by peer` is Envoy enforcing STRICT mTLS on the app port.

**Files:** `01-app-namespace.yaml`, `02-ama-metrics-settings.yaml`, `03-peerauth-strict.yaml`, `06-custom-prometheus-config.yaml`

**Setup:**
- App workload: namespace `app` from `01-app-namespace.yaml`, sidecar injected (Istio label `asm-1-27`)
- App pod: **default** Istio config — prometheus-merge is back ON (the `proxy.istio.io/config` override from Appendix C reverted)
- mTLS: STRICT `PeerAuthentication` in `app` from `03-peerauth-strict.yaml`
- ama-metrics custom scrape config: `ama-metrics-prometheus-config` ConfigMap in `kube-system` from `06-custom-prometheus-config.yaml` adding job `app-direct-8080` that hardcodes `:8080/metrics`:

```yaml
scrape_configs:
  - job_name: app-direct-8080
    kubernetes_sd_configs: [{ role: pod, namespaces: { names: [app] } }]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: prom-example
      - source_labels: [__meta_kubernetes_pod_ip]
        target_label: __address__
        replacement: "$1:8080"
      - target_label: __metrics_path__
        replacement: /metrics
```

- ama-metrics-operator-targets and ama-metrics pods restarted to pick up the new config

**Active target state in ama-metrics:**
```
scrapePool : app-direct-8080
health     : down
scrapeUrl  : http://10.244.0.188:8080/metrics
lastError  : Get "http://10.244.0.188:8080/metrics":
             read tcp 10.244.0.50:36372->10.244.0.188:8080:
             read: connection reset by peer
```

For comparison, the default annotation-based job on the same pod is healthy:
```
scrapePool : kubernetes-pods
health     : up
scrapeUrl  : http://10.244.0.188:15020/stats/prometheus
```

---

## Appendix E — Three fixes verified

**Common files (Appendix D baseline):** `01-app-namespace.yaml`, `02-ama-metrics-settings.yaml`, `03-peerauth-strict.yaml`, `06-custom-prometheus-config.yaml`

**Common setup for all three fixes:** starts from Appendix D's broken state — STRICT mTLS in `app`, sidecar-injected `prom-example`, ama-metrics custom job `app-direct-8080` targeting `:8080/metrics`. Each fix is applied independently (previous fix reverted before applying the next) so the scrape pool was the same in every case.

### Fix A — Port-level PERMISSIVE PeerAuthentication

**Files:** common + `07-fixA-portlevel-permissive.yaml`

**Setup delta:** apply `07-fixA-portlevel-permissive.yaml` to namespace `app`:

```yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata: { name: allow-metrics-8080, namespace: app }
spec:
  selector: { matchLabels: { app: prom-example } }
  portLevelMtls:
    "8080": { mode: PERMISSIVE }
```

The namespace-wide `strict-mtls` PeerAuthentication remains in place; this port-scoped policy carves out `:8080` only.

**Result:**
```
scrapePool : app-direct-8080
health     : up
scrapeUrl  : http://10.244.0.188:8080/metrics
lastError  :
```

### Fix B — Exclude inbound port from sidecar

**Files:** common + `08-fixB-excludeInboundPorts.yaml`

**Setup delta:** Fix A reverted (deleted `allow-metrics-8080` PeerAuthentication). Apply `08-fixB-excludeInboundPorts.yaml` — patches the `prom-example` deployment to add a pod template annotation, then pod is recreated:

```yaml
metadata:
  annotations:
    traffic.sidecar.istio.io/excludeInboundPorts: "8080"
```

**Result:**
```
scrapePool : app-direct-8080
health     : up
scrapeUrl  : http://10.244.0.125:8080/metrics
lastError  :
```

### Fix C — Rewrite scrape config to merged endpoint

**Files:** common + `09-fixC-rewrite-config-to-15020.yaml`

**Setup delta:** Fix B reverted (removed the `excludeInboundPorts` annotation, pod recreated). Apply `09-fixC-rewrite-config-to-15020.yaml` which replaces the `app-direct-8080` job's relabel rules to point at Envoy's merged endpoint:

```yaml
relabel_configs:
  - source_labels: [__meta_kubernetes_pod_ip]
    target_label: __address__
    replacement: "$1:15020"
  - target_label: __metrics_path__
    replacement: /stats/prometheus
```

ama-metrics-operator-targets and ama-metrics restarted to pick up the new config.

**Result:**
```
scrapePool : app-direct-8080
health     : up
scrapeUrl  : http://10.244.0.108:15020/stats/prometheus
lastError  :
```

---

## Appendix F — Reusable diagnostic command

Targets are sharded across the 2 ama-metrics replicas; this iterates both:

```powershell
foreach ($p in (kubectl -n kube-system get pod -l rsName=ama-metrics -o jsonpath='{.items[*].metadata.name}').Split(' ')) {
  Write-Host "=== $p ==="
  kubectl -n kube-system exec $p -c prometheus-collector -- sh -c "wget -qO- 'http://localhost:9090/api/v1/targets?state=active' 2>/dev/null" `
    | ConvertFrom-Json | Select-Object -ExpandProperty data | Select-Object -ExpandProperty activeTargets `
    | Select-Object scrapePool,health,scrapeUrl,lastError | Format-Table -AutoSize -Wrap
}
```

---

## Appendix G — Test artifacts in this worktree

All YAMLs live under `test/istio-investigation/`:

| File | Purpose |
|---|---|
| `01-app-namespace.yaml` | Baseline app namespace, deployment, service |
| `02-ama-metrics-settings.yaml` | Enable pod-annotation scraping for `app` namespace |
| `03-peerauth-strict.yaml` | STRICT mTLS in `app` |
| `04-no-annotation-rewrite.yaml` | Disable Istio prometheus-merge (Failure #1) |
| `06-custom-prometheus-config.yaml` | Custom scrape config hardcoded to `:8080` (Failure #2) |
| `07-fixA-portlevel-permissive.yaml` | Fix A |
| `08-fixB-excludeInboundPorts.yaml` | Fix B |
| `09-fixC-rewrite-config-to-15020.yaml` | Fix C |
