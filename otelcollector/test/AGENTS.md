# Test Framework Guide

## Test Decision Tree

When adding tests, use this decision tree:

1. **Testing deployed pod behavior, metrics collection, or config processing?** → Ginkgo E2E test (`otelcollector/test/ginkgo-e2e/`)
2. **Testing pure Go logic in shared modules?** → Go unit test (`*_test.go` alongside source)
3. **Testing TypeScript rules converter?** → Jest test (`tools/az-prom-rules-converter/__tests__/`)
4. **Testing Helm chart rendering?** → `helm template` validation
5. **Testing multi-platform builds?** → Docker build on target arch

## Test Patterns in This Repo

### Ginkgo E2E Tests (Primary)
- **Framework**: Ginkgo v2.21.0 + Gomega v1.35.1
- **Location**: `otelcollector/test/ginkgo-e2e/`
- **Naming**: `*_test.go` with `suite_test.go` per package
- **Structure**: `Describe` → `Context` → `It` or `DescribeTable` → `Entry`
- **Labels**: Used for selective execution (`Label(utils.ConfigProcessingCommon)`, `Label("operator")`, `Label("windows")`, `Label("arm64")`)
- **Setup**: `BeforeSuite` initializes K8s client via `utils.SetupKubernetesClient()`

### Test Suites

| Suite | Directory | Focus |
|-------|-----------|-------|
| configprocessing | `otelcollector/test/ginkgo-e2e/configprocessing/` | Pod status, config validation |
| prometheusui | `otelcollector/test/ginkgo-e2e/prometheusui/` | Prometheus UI API endpoints |
| containerstatus | `otelcollector/test/ginkgo-e2e/containerstatus/` | Container health checks |
| operator | `otelcollector/test/ginkgo-e2e/operator/` | Prometheus Operator CRDs |
| livenessprobe | `otelcollector/test/ginkgo-e2e/livenessprobe/` | Liveness probe validation |
| querymetrics | `otelcollector/test/ginkgo-e2e/querymetrics/` | Metrics API querying |
| regionTests | `otelcollector/test/ginkgo-e2e/regionTests/` | Regional deployment validation |

### Jest Tests (TypeScript)
- **Framework**: Jest 29.3.1 + ts-jest
- **Location**: `tools/az-prom-rules-converter/`
- **Run**: `npm test`

## Common Test Utilities

Located in `otelcollector/test/utils/`:
- `setup.go` — K8s client initialization (`SetupKubernetesClient()`)
- `constants.go` — Label constants for test selection
- `prometheus_helpers.go` — Prometheus API query utilities
- `azure_monitor_query.go` — Azure Monitor query helpers
- `api_response.go` — API response parsing structures

## Test Data

- **Scrape configs**: `otelcollector/test/test-cluster-yamls/` — ConfigMaps and CRs for test scrape targets
- **TestKube configs**: `otelcollector/test/testkube/` — Test execution CRs and API server permissions
- **Arc conformance**: `otelcollector/test/arc-conformance/` — Arc extension conformance tests
