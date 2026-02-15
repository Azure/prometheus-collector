# Health Metrics Testing Proposal

## Overview
This document proposes integration and end-to-end (e2e) testing strategies for the health metrics functionality in CCP mode.

## Unit Tests (✅ Implemented)

The following unit tests have been implemented in `otelcollector/shared/health_metrics_test.go`:

1. **TestHealthMetricsRegistration** - Verifies all 5 health metrics can be registered with Prometheus
2. **TestHealthMetricsEndpoint** - Validates that metrics are properly exposed via HTTP endpoint
3. **TestHealthMetricsLabels** - Tests metric labels with different environment variable configurations
4. **TestInvalidCustomConfigMetric** - Tests invalid config detection logic
5. **TestMetricMutexSafety** - Verifies thread-safe concurrent access to metric counters
6. **TestExportingFailedCounter** - Tests the exporting failed counter increment/reset logic
7. **TestMetricsConstantsAreCorrect** - Validates health metrics constants (port, interval)

All unit tests pass successfully.

## Integration Test Proposal

### 1. Health Metrics Endpoint Validation (CCP Mode)

**Location**: `otelcollector/test/ginkgo-e2e/healthmetrics/`

**Test Suite Structure**:
```
healthmetrics/
├── suite_test.go           # Ginkgo test suite setup
├── health_metrics_test.go  # Integration tests
├── go.mod
└── go.sum
```

**Test Cases**:

#### Test 1: CCP Mode Health Metrics Endpoint Accessibility
```go
It("should expose health metrics on port 2234 in CCP mode", func() {
    // Prerequisites:
    // - CCP mode deployment running (ama-metrics-ccp pod)
    // - CCP_METRICS_ENABLED=true
    
    // Steps:
    // 1. Port-forward to CCP pod port 2234
    // 2. HTTP GET to http://localhost:2234/metrics
    // 3. Verify 200 status code
    // 4. Verify response contains Prometheus metrics format
    
    // Assertions:
    // - Response status is 200
    // - Response Content-Type is text/plain
    // - Response contains "# HELP" and "# TYPE" comments
})
```

#### Test 2: All Required Metrics Are Present
```go
It("should expose all 5 required health metrics in CCP mode", func() {
    // Steps:
    // 1. Port-forward to CCP pod port 2234
    // 2. HTTP GET to /metrics endpoint
    // 3. Parse response body
    
    // Assertions:
    // - timeseries_received_per_minute metric is present
    // - timeseries_sent_per_minute metric is present
    // - bytes_sent_per_minute metric is present
    // - invalid_custom_prometheus_config metric is present
    // - exporting_metrics_failed metric is present
})
```

#### Test 3: Metrics Have Correct Labels
```go
It("should have correct labels on all health metrics", func() {
    // Steps:
    // 1. Get health metrics from CCP pod
    // 2. Parse Prometheus metrics format
    
    // Assertions:
    // - All gauge metrics have labels: computer, release, controller_type
    // - invalid_custom_prometheus_config has additional "error" label
    // - Label values match environment variables:
    //   - computer = NODE_NAME
    //   - release = HELM_RELEASE_NAME
    //   - controller_type = CONTROLLER_TYPE
})
```

#### Test 4: Metrics Update Over Time
```go
It("should update metrics values over time", func() {
    // Prerequisites:
    // - Scraping is actively happening
    
    // Steps:
    // 1. Get initial metric values at T0
    // 2. Wait 90 seconds (1.5 update cycles)
    // 3. Get metric values at T1
    
    // Assertions:
    // - timeseries_received_per_minute value changed (if scraping is active)
    // - OR all volume metrics are 0 (if no scraping)
    // - Metrics endpoint remains accessible throughout
})
```

#### Test 5: Invalid Config Detection
```go
It("should report invalid config when AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG is true", func() {
    // Prerequisites:
    // - Deploy with invalid Prometheus config
    // - Set AZMON_INVALID_CUSTOM_PROMETHEUS_CONFIG=true
    // - Set INVALID_CONFIG_FATAL_ERROR to error message
    
    // Steps:
    // 1. Get health metrics
    // 2. Find invalid_custom_prometheus_config metric
    
    // Assertions:
    // - Metric value is 1 (indicating invalid config)
    // - Error label contains the expected error message
})
```

### 2. CCP Mode vs Non-CCP Mode Comparison

#### Test 6: Health Metrics Not Available via Fluent-bit in CCP Mode
```go
It("should not expose fluent-bit in CCP mode", func() {
    // Steps:
    // 1. Get CCP pod
    // 2. List running processes in container
    
    // Assertions:
    // - fluent-bit process is NOT running
    // - Health metrics endpoint on :2234 IS available
})
```

#### Test 7: Health Metrics Available via Fluent-bit in Non-CCP Mode
```go
It("should expose health metrics via fluent-bit in non-CCP mode", Label("non-ccp"), func() {
    // Prerequisites:
    // - Non-CCP deployment (ama-metrics replicaset)
    // - AZMON_PROMETHEUS_COLLECTOR_HEALTH_SCRAPING_ENABLED=true
    
    // Steps:
    // 1. Get non-CCP pod
    // 2. Port-forward to port 2234
    // 3. HTTP GET to /metrics
    
    // Assertions:
    // - fluent-bit process IS running
    // - Health metrics endpoint on :2234 IS available
    // - All 5 metrics are present
})
```

## End-to-End Test Proposal

### 3. Full CCP Deployment E2E Test

**Location**: `otelcollector/test/ginkgo-e2e/ccp-deployment/`

**Label**: `ccp`

**Test Cases**:

#### E2E Test 1: CCP Deployment Health Check
```go
It("should have healthy CCP deployment with metrics", Label("ccp"), func() {
    // Steps:
    // 1. Verify ama-metrics-ccp deployment exists
    // 2. Verify pod is Running
    // 3. Verify all containers are ready
    // 4. Check health endpoint (:8080/health)
    // 5. Check health metrics endpoint (:2234/metrics)
    
    // Assertions:
    // - Deployment has 1 replica
    // - Pod status is Running
    // - All containers have status Running
    // - Health endpoint returns 200
    // - Health metrics endpoint returns 200
    // - All 5 health metrics are present
})
```

#### E2E Test 2: CCP Metrics Ingestion Flow
```go
It("should successfully ingest metrics in CCP mode", Label("ccp"), func() {
    // Prerequisites:
    // - Azure Monitor Workspace (AMW) configured
    // - CCP deployment running
    
    // Steps:
    // 1. Wait for metrics to be scraped and sent (2-3 minutes)
    // 2. Query AMW for CCP-specific metrics
    // 3. Get health metrics from CCP pod
    
    // Assertions:
    // - AMW contains metrics from CCP deployment
    // - timeseries_sent_per_minute > 0
    // - bytes_sent_per_minute > 0
    // - exporting_metrics_failed == 0
})
```

#### E2E Test 3: CCP Pod Restart Resilience
```go
It("should maintain health metrics after pod restart", Label("ccp"), func() {
    // Steps:
    // 1. Delete CCP pod
    // 2. Wait for new pod to start
    // 3. Wait for health metrics endpoint to be available
    // 4. Get health metrics
    
    // Assertions:
    // - New pod starts within 60 seconds
    // - Health metrics endpoint is available within 30 seconds after pod ready
    // - All 5 metrics are present
    // - Metrics have correct labels
})
```

### 4. Metrics Scraping Validation

#### E2E Test 4: Health Metrics Scraped by Prometheus
```go
It("should allow health metrics to be scraped by Prometheus", Label("ccp"), func() {
    // Prerequisites:
    // - Prometheus or compatible scraper configured to scrape :2234/metrics
    
    // Steps:
    // 1. Configure a scrape job for CCP health metrics:
    //    ```yaml
    //    - job_name: 'ccp-health-metrics'
    //      static_configs:
    //        - targets: ['ama-metrics-ccp:2234']
    //    ```
    // 2. Wait for scrape interval
    // 3. Query Prometheus for health metrics
    
    // Assertions:
    // - Prometheus has timeseries_received_per_minute
    // - Prometheus has timeseries_sent_per_minute
    // - Prometheus has bytes_sent_per_minute
    // - Prometheus has invalid_custom_prometheus_config
    // - Prometheus has exporting_metrics_failed
})
```

## Test Implementation Guidelines

### Test Labels
Add a new test label for CCP-specific tests:
- `ccp`: Tests that should only run on clusters with CCP metrics enabled

### Test Configuration Requirements

1. **CCP Mode Enabled**:
   ```yaml
   env:
     - name: CCP_METRICS_ENABLED
       value: "true"
   ```

2. **Port Exposure**:
   ```yaml
   ports:
     - name: health-metrics
       containerPort: 2234
       protocol: TCP
   ```

3. **Service Definition** (if needed for scraping):
   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: ama-metrics-ccp-health
   spec:
     ports:
       - name: health-metrics
         port: 2234
         targetPort: 2234
     selector:
       rsName: ama-metrics-ccp
   ```

### Utility Functions Needed

Add to `otelcollector/test/ginkgo-e2e/utils/`:

```go
// health_metrics_utils.go

// GetHealthMetrics retrieves health metrics from a CCP pod
func GetHealthMetrics(podName, namespace string) (map[string]float64, error)

// ParsePrometheusMetrics parses Prometheus text format metrics
func ParsePrometheusMetrics(metricsText string) (map[string]MetricFamily, error)

// VerifyMetricLabels checks if a metric has the expected labels
func VerifyMetricLabels(metric MetricFamily, expectedLabels map[string]string) bool

// PortForwardToPod creates a port forward to a pod and returns cleanup function
func PortForwardToPod(podName, namespace string, localPort, remotePort int) (cleanup func(), error)
```

## Test Execution

### Running Unit Tests
```bash
cd otelcollector/shared
go test -v -run TestHealth
```

### Running Integration Tests
```bash
cd otelcollector/test/ginkgo-e2e
ginkgo -v --label-filter="ccp" ./healthmetrics
```

### Running All CCP E2E Tests
```bash
cd otelcollector/test/ginkgo-e2e
ginkgo -v --label-filter="ccp"
```

## Success Criteria

✅ **Unit Tests**:
- All 7 unit tests pass
- Code coverage > 80% for health_metrics.go

⏳ **Integration Tests**:
- All health metrics endpoint tests pass
- Tests complete in < 5 minutes

⏳ **E2E Tests**:
- CCP deployment tests pass
- Metrics ingestion verified
- Tests complete in < 10 minutes

## Next Steps

1. ✅ Implement unit tests (COMPLETE)
2. Add integration test suite to `otelcollector/test/ginkgo-e2e/healthmetrics/`
3. Add utility functions to `otelcollector/test/ginkgo-e2e/utils/`
4. Update test README with new `ccp` label
5. Add CCP test configuration to `otelcollector/test/test-cluster-yamls/`
6. Update testkube configuration to include CCP tests
7. Run tests on CCP-enabled cluster and validate

## References

- Existing test structure: `/otelcollector/test/README.md`
- Test utilities: `/otelcollector/test/ginkgo-e2e/utils/`
- Ginkgo documentation: https://onsi.github.io/ginkgo/
- Prometheus metrics format: https://prometheus.io/docs/instrumenting/exposition_formats/
