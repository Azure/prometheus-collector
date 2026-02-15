# Health Metrics Implementation Summary

## Overview
This document summarizes the implementation of health metrics for CCP mode in the prometheus-collector repository.

## Problem Statement
The CCP mode (Control-Plane Prometheus) was not emitting health metrics because:
1. CCP mode skips fluent-bit startup (line 239 in main.go)
2. Health metrics were only exposed through fluent-bit's out_appinsights plugin
3. No alternative mechanism existed for CCP mode to expose these metrics

## Solution Implemented

### 1. Created Shared Health Metrics Module
**File**: `otelcollector/shared/health_metrics.go`

**Key Features**:
- Standalone module that doesn't depend on fluent-bit
- Exposes metrics on port **2234** via HTTP endpoint `/metrics`
- Updates metrics every **60 seconds**
- Thread-safe with mutex protection

**Metrics Exposed**:
1. `timeseries_received_per_minute` (gauge) - Timeseries to be sent to storage
2. `timeseries_sent_per_minute` (gauge) - Timeseries sent to storage
3. `bytes_sent_per_minute` (gauge) - Bytes of timeseries sent to storage
4. `invalid_custom_prometheus_config` (gauge) - Invalid config indicator
5. `exporting_metrics_failed` (counter) - Export failure count

**Labels on Each Metric**:
- `computer` - Node name (from NODE_NAME env var)
- `release` - Helm release name (from HELM_RELEASE_NAME env var)
- `controller_type` - Controller type (from CONTROLLER_TYPE env var)
- `error` - Error message (only on invalid_custom_prometheus_config)

### 2. Updated Main Entry Point
**File**: `otelcollector/main/main.go`

**Changes**:
- Added conditional logic at lines 259-263
- In CCP mode: calls `shared.ExposePrometheusCollectorHealthMetrics()` directly
- In non-CCP mode: continues using fluent-bit (no breaking changes)

```go
if ccpMetricsEnabled != "true" {
    shared.StartFluentBit(fluentBitConfigFile)
    // ... fluent-bit initialization
} else {
    // In CCP mode, expose health metrics directly without fluent-bit
    log.Println("Starting Prometheus Collector Health metrics in CCP mode")
    go shared.ExposePrometheusCollectorHealthMetrics()
}
```

### 3. Updated CCP Deployment Template
**File**: `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-deployment.yaml`

**Changes**:
- Added port configuration to expose containerPort 2234
- Named the port "health-metrics" for clarity
- Makes metrics accessible for scraping by monitoring systems

```yaml
ports:
  - name: health-metrics
    containerPort: 2234
    protocol: TCP
```

### 4. Updated Dependencies
**Files**: 
- `otelcollector/go.mod` and `otelcollector/go.sum`
- `otelcollector/shared/go.mod` and `otelcollector/shared/go.sum`

Added Prometheus client dependencies for metric exposure.

## Testing

### Unit Tests ✅
**File**: `otelcollector/shared/health_metrics_test.go`

**Test Coverage** (7 tests, all passing):
1. **TestHealthMetricsRegistration** - Verifies all metrics can be registered
2. **TestHealthMetricsEndpoint** - Validates HTTP endpoint functionality
3. **TestHealthMetricsLabels** - Tests label configuration with env vars
4. **TestInvalidCustomConfigMetric** - Tests invalid config detection
5. **TestMetricMutexSafety** - Verifies thread-safe concurrent access
6. **TestExportingFailedCounter** - Tests counter increment/reset logic
7. **TestMetricsConstantsAreCorrect** - Validates constants (port, interval)

**Running Unit Tests**:
```bash
cd otelcollector/shared
go test -v -run TestHealth
```

**Expected Output**:
```
=== RUN   TestHealthMetricsRegistration
--- PASS: TestHealthMetricsRegistration (0.00s)
=== RUN   TestHealthMetricsEndpoint
--- PASS: TestHealthMetricsEndpoint (0.00s)
=== RUN   TestHealthMetricsLabels
--- PASS: TestHealthMetricsLabels (0.00s)
=== RUN   TestInvalidCustomConfigMetric
--- PASS: TestInvalidCustomConfigMetric (0.00s)
=== RUN   TestMetricMutexSafety
--- PASS: TestMetricMutexSafety (0.00s)
=== RUN   TestExportingFailedCounter
--- PASS: TestExportingFailedCounter (0.00s)
=== RUN   TestMetricsConstantsAreCorrect
--- PASS: TestMetricsConstantsAreCorrect (0.00s)
PASS
```

### Integration/E2E Test Proposal
**Document**: `otelcollector/test/HEALTH_METRICS_TESTING_PROPOSAL.md`

**Sample Test Suite**: `otelcollector/test/ginkgo-e2e/healthmetrics/`

**Proposed Tests** (11 additional tests):

**Integration Tests** (7 tests):
1. CCP Mode Health Metrics Endpoint Accessibility
2. All Required Metrics Are Present
3. Metrics Have Correct Labels
4. Metrics Update Over Time
5. Invalid Config Detection
6. Health Metrics Not Available via Fluent-bit in CCP Mode
7. Health Metrics Available via Fluent-bit in Non-CCP Mode

**E2E Tests** (4 tests):
1. CCP Deployment Health Check
2. CCP Metrics Ingestion Flow
3. CCP Pod Restart Resilience
4. Health Metrics Scraped by Prometheus

**New Test Label**: `ccp` - For CCP-specific tests

**Running Integration Tests** (when implemented):
```bash
cd otelcollector/test/ginkgo-e2e
ginkgo -v --label-filter="ccp" ./healthmetrics
```

## Architecture

### Before (Non-CCP Mode)
```
┌─────────────────────────────────────┐
│    prometheus-collector pod         │
├─────────────────────────────────────┤
│  ┌──────────────┐                   │
│  │ otelcollector│                   │
│  └──────────────┘                   │
│  ┌──────────────┐                   │
│  │   fluent-bit │─┐                 │
│  │              │ │                 │
│  │ ┌──────────┐ │ │ Port 2234      │
│  │ │out_app   │ │ │ /metrics       │
│  │ │insights  │◄┼─┼──────────────► │
│  │ │          │ │ │                 │
│  │ │ExposePrometheus                │
│  │ │CollectorHealth                 │
│  │ │Metrics() │ │ │                 │
│  │ └──────────┘ │ │                 │
│  └──────────────┘─┘                 │
└─────────────────────────────────────┘
```

### After (CCP Mode)
```
┌─────────────────────────────────────┐
│    ama-metrics-ccp pod              │
├─────────────────────────────────────┤
│  ┌──────────────┐                   │
│  │ otelcollector│                   │
│  └──────────────┘                   │
│  ┌──────────────────────────────┐   │
│  │ main.go                      │   │
│  │                              │   │
│  │ if CCP_METRICS_ENABLED {    │   │
│  │   shared.Expose...()  ───┐  │   │
│  │ }                        │  │   │
│  └──────────────────────────┼───┘   │
│                             │       │
│  ┌──────────────────────────┼───┐   │
│  │ shared/health_metrics.go │   │   │
│  │                          ▼   │   │
│  │ ExposePrometheusCollectorHealth │
│  │ Metrics()                    │   │
│  │   ├─ timeseriesReceivedMetric   │
│  │   ├─ timeseriesSentMetric       │
│  │   ├─ bytesSentMetric            │
│  │   ├─ invalidCustomConfigMetric  │
│  │   └─ exportingFailedMetric      │
│  │                          │   │   │
│  └──────────────────────────┼───┘   │
│                             │       │
│                    Port 2234│       │
│                    /metrics │       │
│                             ▼       │
└─────────────────────────────────────┘
```

## Benefits

1. **No fluent-bit overhead** - CCP mode avoids running unnecessary process
2. **Clean architecture** - Shared module reusable by both modes
3. **Minimal changes** - Only 3 source files changed + deps
4. **Backward compatible** - Non-CCP mode unchanged
5. **Well tested** - Comprehensive unit tests included
6. **Clear path forward** - Integration/E2E test proposal ready

## Files Changed

### Source Code (3 files)
1. `otelcollector/shared/health_metrics.go` - New file (151 lines)
2. `otelcollector/main/main.go` - Modified (4 lines added)
3. `otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/ama-metrics-deployment.yaml` - Modified (4 lines added)

### Tests (5 files)
1. `otelcollector/shared/health_metrics_test.go` - New file (399 lines)
2. `otelcollector/test/HEALTH_METRICS_TESTING_PROPOSAL.md` - New file (proposal doc)
3. `otelcollector/test/ginkgo-e2e/healthmetrics/suite_test.go` - New file (sample)
4. `otelcollector/test/ginkgo-e2e/healthmetrics/health_metrics_test.go` - New file (sample)
5. `otelcollector/test/ginkgo-e2e/healthmetrics/go.mod` - New file

### Documentation (1 file)
1. `otelcollector/test/README.md` - Modified (added `ccp` label and tests)

### Dependencies (6 files)
1. `otelcollector/go.mod` and `otelcollector/go.sum`
2. `otelcollector/shared/go.mod` and `otelcollector/shared/go.sum`
3. `otelcollector/main/go.mod` and `otelcollector/main/go.sum`

## Verification Steps

### 1. Build Verification
```bash
cd otelcollector/main
go build -o /tmp/main .
# Should complete without errors
```

### 2. Unit Test Verification
```bash
cd otelcollector/shared
go test -v -run TestHealth
# All 7 tests should pass
```

### 3. Runtime Verification (Manual)
When deployed in CCP mode:
```bash
# Port-forward to CCP pod
kubectl port-forward -n kube-system pod/ama-metrics-ccp-xxxxx 2234:2234

# Access health metrics
curl http://localhost:2234/metrics

# Expected output should include:
# - # HELP timeseries_received_per_minute
# - # TYPE timeseries_received_per_minute gauge
# - timeseries_received_per_minute{computer="...",release="...",controller_type="..."}
# - (similar for other 4 metrics)
```

## Next Steps

1. **Integration Testing**: Implement the proposed integration test suite
2. **E2E Testing**: Add E2E tests to validate full metrics ingestion flow
3. **Monitoring**: Set up alerts/dashboards for health metrics in production
4. **Documentation**: Update user-facing docs with health metrics information

## References

- Original Issue: CCP mode doesn't emit health metrics
- PR: Enable CCP mode to emit health metrics by default
- Test Proposal: `otelcollector/test/HEALTH_METRICS_TESTING_PROPOSAL.md`
- Code Review: Completed with feedback addressed
- Unit Tests: All passing ✅

## Contributors

- Implementation: GitHub Copilot
- Review: davidkydd
