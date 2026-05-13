# Upgrade Health Reporting Architecture

## Overview

This document describes the architecture for integrating upgrade health reporting into the AKS Managed Prometheus collector. The system evaluates cluster, node pool, and node health during AKS upgrades by intercepting scraped metrics in real time, caching them in-memory, and writing HealthSignal custom resources that the AKS RP uses to decide whether to proceed or abort an upgrade.

## Problem Statement

During AKS cluster upgrades, the AKS Resource Provider (RP) needs real-time health signals from the monitoring stack to determine if it is safe to continue. The monitoring addon (ama-metrics) already scrapes Prometheus metrics from the cluster — we need to leverage this data to produce health assessments without adding significant resource overhead.

Key requirements:
- Health reports must be available **immediately** when an upgrade starts, with baseline data from before the upgrade
- Customer-defined rules must be supported alongside built-in health checks
- Memory overhead must scale appropriately with cluster size
- The solution must work across all pod types (DaemonSet, ReplicaSet, CCP)

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     AKS Cluster                                 │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  ama-metrics (ReplicaSet, overlay)                        │  │
│  │  Scrapes: kube-state-metrics, app endpoints               │  │
│  │                                                           │  │
│  │  Collector Process (single binary):                       │  │
│  │    Prometheus Receiver → Batch Processor ──► OTLP Exporter│  │
│  │                                          └─► health_cache │  │
│  │                                               Exporter    │  │
│  │                                                  │        │  │
│  │                                           MetricsCache    │  │
│  │                                           (in-process)    │  │
│  │                                                  │        │  │
│  │    HealthSignal Controller ◄─────────────────────┘        │  │
│  │    Handles: Cluster, NodePool, customer app rules         │  │
│  │    Reads: ama-metrics-upgrade-gate ConfigMap               │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  ama-metrics-node (DaemonSet, one per node, overlay)      │  │
│  │  Scrapes: node_exporter, kubelet, cAdvisor                │  │
│  │                                                           │  │
│  │  Collector Process:                                       │  │
│  │    Prometheus Receiver → Batch Processor ──► OTLP Exporter│  │
│  │                                          └─► health_cache │  │
│  │                                               Exporter    │  │
│  │                                                  │        │  │
│  │                                           MetricsCache    │  │
│  │                                                  │        │  │
│  │    HealthSignal Controller ◄─────────────────────┘        │  │
│  │    Handles: Node scope for THIS node only                 │  │
│  │    Reads: ama-metrics-upgrade-gate ConfigMap               │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  ama-metrics-ccp (Deployment, underlay)                   │  │
│  │  Scrapes: API server, etcd, controller-manager            │  │
│  │                                                           │  │
│  │  Collector Process:                                       │  │
│  │    Same pipeline with health_cache Exporter               │  │
│  │    MetricsCache → HealthSignal Controller                 │  │
│  │    Built-in health checks ONLY (no customer ConfigMap)    │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  ConfigMap: ama-metrics-upgrade-gate (kube-system)         │  │
│  │  Customer configures:                                      │  │
│  │  - rules: PromQL queries + thresholds (scoped per level)  │  │
│  │  - cachedMetrics: additional metric names to cache         │  │
│  │  - retentionPercent: memory budget percentage (1-100)      │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  AKS RP                                                   │  │
│  │  Creates HealthCheckRequest CRs → Reads HealthSignal CRs  │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Components

### 1. health_cache Exporter (Custom OTel Exporter)

A custom OpenTelemetry exporter registered in the collector pipeline. It sits alongside the existing OTLP exporter as a fan-out — every scraped metric batch passes through it.

**Responsibilities:**
- Filters metrics relevant to upgrade health (built-in list + customer-declared metrics)
- Converts OTel pdata format to lightweight `HealthMetricSample` structs
- Records samples into the shared in-process `MetricsCache`
- Periodically reloads the ConfigMap to discover new customer metric names (overlay only)

**Key property:** Runs continuously from pod startup, so the cache always has baseline data before any upgrade begins.

### 2. MetricsCache (In-Memory Sliding Window)

A thread-safe, typed cache keyed by `HealthMetricType` + target (node name, pool name, rule name).

**Features:**
- Sliding window retention (auto-computed from cluster size and customer-set `retentionPercent`)
- Dedup TTL (15s) prevents redundant processing within the same scrape cycle
- Memory budget scales inversely with node count: ~30min at 1000 nodes, 1hr at ≤500 nodes
- `EvictExpired()` prunes old entries; `pruneUnsafe()` runs on every `Record()` call

### 3. HealthSignal Controller

A controller-runtime reconciler that watches `HealthCheckRequest` CRs and writes `HealthSignal` CRs.

**Evaluation flow:**
1. HealthCheckRequest CR created by AKS RP
2. Controller loads customer rules from ConfigMap (overlay only)
3. Evaluates rules against cached metric data — **no Prometheus API queries needed**
4. Evaluates built-in health checks (node ready, network, pods, PDBs) from cache
5. Writes HealthSignal CR with `True` (healthy), `False` (unhealthy), or `Unknown` (pending)
6. Re-evaluates every 30 seconds

### 4. UpgradeGate (ConfigMap Reader)

Reads the `ama-metrics-upgrade-gate` ConfigMap for customer-defined rules, cached metric names, and retention settings. Only active on overlay pods.

**Customer-facing configuration:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ama-metrics-upgrade-gate
  namespace: kube-system
data:
  retentionPercent: "80"
  cachedMetrics: |
    ["http_requests_total", "app_error_count"]
  rules: |
    [
      {
        "name": "error-rate-low",
        "scope": "NodePool",
        "query": "sum(rate(http_requests_total{code=~\"5..\",pool=\"{{.PoolName}}\"}[5m]))",
        "operator": "<",
        "threshold": 0.05
      }
    ]
```

## Pod Responsibilities

| Pod | Scrape Targets | Health Scopes | Customer Rules | ConfigMap |
|-----|---------------|---------------|---------------|-----------|
| ama-metrics (ReplicaSet) | kube-state-metrics, app endpoints | Cluster, NodePool, app-level | Yes | Yes |
| ama-metrics-node (DaemonSet) | node_exporter, kubelet, cAdvisor | Node (own node only) | Yes (node-scoped) | Yes |
| ama-metrics-ccp (underlay) | API server, etcd, scheduler | Built-in only | No | No |

Each pod has its own independent MetricsCache. No data sharing between pods.

## Design Decisions

### Why move the controller into the collector process?

**Chosen:** Controller runs inside the collector binary (same process).

**Rejected alternatives:**

| Alternative | Why Rejected |
|-------------|-------------|
| **Controller in separate process (original design)** | The collector runs in agent mode with no queryable storage (TSDB, WAL are nil). The controller cannot query historical metrics via the Prometheus API — only instant point-in-time values are available. This means no baseline data when an upgrade starts, and no support for trend-based customer rules like "error rate stable for 15 minutes." |
| **gRPC/HTTP between processes** | Adds ~5-6MB overhead for the gRPC runtime. Requires protobuf definitions, server/client implementations, and serialization for every scrape cycle. Introduces a failure mode (connection drops between processes in the same pod). The data being transferred is small, but the infrastructure complexity is not justified when in-process sharing eliminates all IPC. |
| **Shared file on emptyDir volume** | Requires serialization/deserialization on every scrape cycle. File I/O adds latency and disk wear. Polling-based reads introduce lag. Race conditions between writer (collector) and reader (controller) require careful locking or atomic rename patterns. Overall more complex and slower than in-process. |
| **Custom OTel Processor (instead of Exporter)** | A processor sits inline in the pipeline — a bug in it blocks all metric export to Azure Monitor. An exporter runs as a parallel fan-out, so a bug only affects health caching while the main OTLP export path continues unaffected. The exporter provides better failure isolation. |

**Benefits of the chosen approach:**
- Zero IPC overhead — cache is a shared pointer, no serialization
- Cache is continuously warm from pod startup — immediate baseline data for upgrades
- One fewer OS process per pod — saves ~30-50MB of controller process overhead
- Exporter failure doesn't affect main metric export path (fan-out isolation)
- Single binary simplifies deployment, logging, and lifecycle management

### Why a custom OTel exporter instead of a processor?

In the OTel collector pipeline, metrics flow through three stages:

```
Receiver  →  Processor  →  Exporter
(input)      (transform)    (output)
```

A **processor** sits inline — every metric passes through it sequentially. If it's slow or crashes, the entire pipeline backs up and no metrics reach any exporter. A **exporter** sits at the end as a fan-out destination — multiple exporters receive the same data in parallel, and a failure in one does not affect the others.

```
                              ┌→ OTLP Exporter (to Azure Monitor)  ✅ unaffected
Receiver → Processor → ──────┤
                              └→ health_cache Exporter (our cache) ← if this crashes
```

| | Processor | Exporter |
|---|---|---|
| Pipeline position | Inline (serial) | End (parallel fan-out) |
| Failure impact | **Blocks entire pipeline** — no metrics reach Azure Monitor | **Only affects health caching** — main export continues |
| Can modify/drop data | Yes | No (receives a read-only copy) |
| Do we need to modify data? | No — we only read and cache | Correct — read-only is all we need |
| Risk to customer metrics | **High** — a bug stops all metric ingestion | **Zero** — main OTLP path is independent |
| Testing complexity | Must validate no side effects on the data stream | Isolated — cannot corrupt or delay the main path |

**Decision:** Since we only need to **read** metrics and copy health-relevant ones into the cache, an exporter gives us identical data access with zero risk to the customer's production metric pipeline. The health_cache exporter can crash, hang, or be disabled entirely without any impact on metric ingestion to Azure Monitor.

### Why not cache all metrics?

Memory. A 1000-node cluster may scrape thousands of unique metric names with millions of series. Caching everything would consume gigabytes. Instead:
- Built-in health metrics (6 metric names) are always cached
- Customer explicitly declares additional metric names via `cachedMetrics` in the ConfigMap
- Retention window auto-scales based on cluster size and customer-set `retentionPercent`

### Why per-pod caches instead of a shared cache?

Each pod scrapes different targets:
- DaemonSet pods scrape node-local exporters — they only have metrics for their own node
- ReplicaSet pods scrape cluster-wide targets — they have kube-state-metrics and app endpoints
- CCP pods scrape control-plane components — entirely different metric set

A shared cache would require cross-pod communication (network, shared storage) for data that is naturally partitioned. Per-pod caches match the scrape topology exactly.

### Why customer-explicit metric names instead of auto-detection from PromQL queries?

Parsing metric names from arbitrary PromQL queries is unreliable:
- `sum(rate(http_requests_total{code=~"5.."}[5m]))` — extractable
- `http_requests_total / ignoring(code) http_requests_total` — two metrics
- `{__name__=~"app_.*"}` — regex, not a single name
- `label_replace(metric, ...)` — nested functions

Rather than building a fragile PromQL parser, we let customers declare exactly which metrics to cache. This is explicit, predictable, and documented.

## Data Flow

### Continuous (from pod startup):

```
Prometheus scrape → OTel pipeline → health_cache exporter
                                         │
                                    filter by name
                                    (builtin + customer-declared)
                                         │
                                    MetricsCache.Record()
                                    (sliding window, auto-pruned)
```

### During upgrade:

```
AKS RP creates HealthCheckRequest CR
         │
         ▼
HealthSignal Controller reconciles
         │
    ┌────┴────┐
    │         │
    ▼         ▼
Read cache   Load ConfigMap rules
(builtin     (overlay only)
 metrics)         │
    │         Evaluate rules
    │         against cache
    │              │
    └──────┬───────┘
           │
     Write HealthSignal CR
     (True / False / Unknown)
           │
     RequeueAfter: 30s
```

### After upgrade:

```
AKS RP deletes HealthCheckRequest CR
         │
         ▼
HealthSignal CR garbage collected (ownerReference)
Cache continues filling (ready for next upgrade)
```
