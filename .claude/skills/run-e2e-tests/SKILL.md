---
name: run-e2e-tests
description: Bootstrap a dev cluster and run Ginkgo E2E tests for the prometheus-collector (ama-metrics) agent. Use when "run e2e tests", "run ginkgo tests", "bootstrap test cluster", "test prometheus-collector", or "validate ama-metrics on cluster".
allowed-tools: Read, Grep, Glob, Bash, LS
---

# Run Prometheus-Collector Ginkgo E2E Tests

Bootstraps a dev AKS cluster for E2E testing and runs the Ginkgo test suites in `otelcollector/test/ginkgo-e2e/`.

## Prerequisites

Verify **all** of these before starting:

1. **Kubectl** access pointed to a test cluster with ama-metrics deployed via [backdoor deployment](../../otelcollector/deploy/addon-chart/Readme.md)
2. **Azure CLI** logged in (`az login`)
3. **Corpnet VPN** connected
4. **Go** installed (1.23+)
5. **Ginkgo CLI** installed (see Stage 2)

Ask the user for:
- `$KUBECONFIG` path or confirm current kubectl context
- `$AMW_QUERY_ENDPOINT` — the Azure Monitor Workspace query endpoint (required for querymetrics tests)

---

## Execution Stages

### Stage 1: Verify Cluster Access

```bash
# Confirm kubectl can reach the cluster
kubectl get nodes
kubectl get pods -n kube-system -l rsName=ama-metrics
kubectl get pods -n kube-system -l dsName=ama-metrics-node
```

If ama-metrics pods are not running, the cluster needs the agent deployed first. Refer to `otelcollector/deploy/addon-chart/Readme.md` for backdoor deployment.

**Success Criteria**: `ama-metrics` replicaset and `ama-metrics-node` daemonset pods are running.

---

### Stage 2: Install Ginkgo CLI

```bash
export GOPROXY=https://proxy.golang.org
go install -v github.com/onsi/ginkgo/v2/ginkgo@latest

# Verify
ginkgo version
```

**Success Criteria**: `ginkgo version` prints a version string.

---

### Stage 3: Deploy Test Workloads

Deploy reference apps and scraping configuration needed by the tests:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Reference apps
kubectl apply -f "$REPO_ROOT/internal/referenceapp/prometheus-reference-app.yaml"
# Only if Windows nodes exist:
# kubectl apply -f "$REPO_ROOT/internal/referenceapp/win-prometheus-reference-app.yaml"

# Scraping configmaps
kubectl apply -f "$REPO_ROOT/otelcollector/test/test-cluster-yamls/configmaps/"

# Pod and Service Monitor CRs
kubectl apply -f "$REPO_ROOT/otelcollector/test/test-cluster-yamls/customresources/"
```

**Success Criteria**: All resources applied without error. Reference app pods are running.

---

### Stage 4: Run Ginkgo Tests

```bash
cd "$REPO_ROOT/otelcollector/test/ginkgo-e2e"

AMW_QUERY_ENDPOINT="$AMW_QUERY_ENDPOINT" \
ginkgo -p -r --keep-going \
  --label-filter='!/./' \
  -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"
```

#### Label Filter Options

Adjust `--label-filter` based on the cluster configuration:

| Cluster Has | Filter |
|---|---|
| Basic (no special nodes) | `--label-filter='!/./'` (unlabeled tests only) |
| Operator enabled | `--label-filter='!/./ \|\| operator'` |
| Windows nodes | `--label-filter='!/./ \|\| windows'` |
| All features | `--label-filter='!(arc-extension)'` |
| Specific suite only | Add path, e.g., `./containerstatus` |

Available labels: `operator`, `arc-extension`, `windows`, `arm64`, `fips`, `linux-daemonset-custom-config`, `retina`, `mdsd`, `otlp`

#### Running a Single Test Suite

```bash
# Example: only container status tests
ginkgo -p -r --keep-going ./containerstatus

# Example: only liveness probe tests
ginkgo -p -r --keep-going ./livenessprobe

# Example: only query metrics tests
AMW_QUERY_ENDPOINT="$AMW_QUERY_ENDPOINT" \
ginkgo -p -r --keep-going ./querymetrics \
  -ldflags="-s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com"
```

**Success Criteria**: All test suites pass (green output). Failed tests are reported with file/line references.

---

### Stage 5: Interpret Results

Ginkgo outputs a summary like:
```
Ran X of Y Specs in Z seconds
PASS! -- X Passed | 0 Failed | 0 Pending | 0 Skipped
```

If tests fail:
1. Check the failure output for the specific `Describe/Context/It` path and assertion
2. Known transient errors are listed in `otelcollector/test/ginkgo-e2e/utils/constants.go` under `LogLineErrorsToExclude`
3. For querymetrics failures, verify `AMW_QUERY_ENDPOINT` is correct and the AMW has had time to ingest (15+ min after deployment)
4. For containerstatus failures, check `kubectl get pods -n kube-system` for crashing pods

---

## Test Suites Reference

| Suite | What It Tests |
|---|---|
| `containerstatus` | All pods running, no log errors, all processes alive, daemonsets scheduled on all nodes |
| `livenessprobe` | Pods restart when processes are killed, configmap changes trigger restarts |
| `prometheusui` | Prometheus UI API returns scrape pools, config, targets, metadata |
| `querymetrics` | `up` metric and expected labels queryable from the AMW |
| `operator` | Target allocator functionality |
| `configprocessing` | Configmap parsing produces correct Prometheus config |

---

## Troubleshooting

- **`ginkgo: command not found`**: Ensure `$(go env GOPATH)/bin` is in `$PATH`
- **Cluster unreachable**: Check VPN and `kubectl config current-context`
- **querymetrics fails with empty results**: Wait 15+ minutes after deploying for metrics to ingest. Verify the AMW query endpoint is correct and the kubelet managed identity has `Monitoring Data Reader` role on the AMW.
- **TLS/HTTPS errors in containerstatus**: These are expected transient errors during startup. They are excluded automatically via `LogLineErrorsToExclude` in constants.go.
