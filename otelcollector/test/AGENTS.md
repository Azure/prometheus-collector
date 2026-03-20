# Test Framework Guide

## Test Decision Tree

When adding tests, use this decision tree:

1. **Testing collector component logic (no cluster needed)?** → Go unit test with `testing.T` + `testify/require`
2. **Testing Prometheus scrape config validation?** → Unit test in `otelcollector/prom-config-validator-builder/`
3. **Testing metric collection end-to-end on a live cluster?** → Ginkgo E2E test under `ginkgo-e2e/`
4. **Testing the TypeScript rules converter?** → Jest test in `tools/az-prom-rules-converter/src/`
5. **Testing Kubernetes operator CRDs (PodMonitor, ServiceMonitor)?** → Ginkgo E2E with `operator` label
6. **Testing multi-architecture support?** → Ginkgo E2E with `arm64` label
7. **Testing Windows node collection?** → Ginkgo E2E with `windows` label
8. **Testing Azure Arc extension?** → Ginkgo E2E with `arc-extension` label

## Test Patterns in This Repo

### Go Unit Tests
- **Framework**: Go `testing` package + `github.com/stretchr/testify/require`
- **Mocking**: Standard Go interfaces and test doubles
- **Location**: Alongside source files as `*_test.go`
- **Naming**: `Test<FunctionName>(t *testing.T)`
- **Style**: Table-driven tests with `[]struct{}` slices
- **Run**: `go test ./...` in the module directory

### Ginkgo E2E Tests
- **Framework**: Ginkgo v2 (`github.com/onsi/ginkgo/v2 v2.21.0`)
- **Location**: `otelcollector/test/ginkgo-e2e/`
- **Suites**: `operator/`, `querymetrics/`, `configprocessing/`, `livenessprobe/`, `prometheusui/`, `containerstatus/`, `regionTests/`
- **Naming**: BDD-style with `Describe`, `Context`, `It`, `BeforeAll`
- **Labels**: `operator`, `windows`, `arm64`, `arc-extension`, `fips`
- **Requires**: Live AKS cluster bootstrapped per `otelcollector/test/README.md`

### TypeScript Tests
- **Framework**: Jest ^29.3.1 with ts-jest
- **Location**: `tools/az-prom-rules-converter/src/*.test.ts`
- **Naming**: `describe('suite')` → `test('case')`
- **Run**: `cd tools/az-prom-rules-converter && npm test`

## Common Test Utilities
- `otelcollector/test/utils/constants.go` — Test label string constants
- `otelcollector/test/utils/` — Shared test helper functions
- `otelcollector/test/test-cluster-yamls/` — Kubernetes manifests for test environments
- `otelcollector/test/testkube/testkube-test-crs.yaml` — Testkube test CRD definitions
- `otelcollector/test/testkube/api-server-permissions.yaml` — RBAC for test execution

## Test Data
- E2E test configurations: `otelcollector/test/test-cluster-yamls/configmaps/`
- TypeScript test fixtures: `tools/az-prom-rules-converter/examples/`
- Custom resources for operator tests: `otelcollector/test/test-cluster-yamls/`

## Adding New Tests Checklist
1. Add test file following the naming convention for the test type
2. For new scrape jobs: add to `otelcollector/test/test-cluster-yamls/`
3. For new test labels: add constant to `utils/constants.go`, document in `README.md`, add to PR template, add to `testkube-test-crs.yaml`
4. For new API permissions: update `testkube/api-server-permissions.yaml`
5. For new test suites: register in `testkube/testkube-test-crs.yaml`
