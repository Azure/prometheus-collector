# Test Authoring

## Description
Guide for creating Ginkgo BDD E2E tests following existing patterns in this repository.

USE FOR: add test, write test, test coverage, add unit test, add integration test, add E2E test, TDD, test-driven development
DO NOT USE FOR: fixing a flaky test, refactoring test infrastructure, test cluster bootstrapping

## Instructions

### When to Apply
When adding new features that need test coverage, fixing bugs that need regression tests, or improving test coverage for existing code.

### Step-by-Step Procedure
1. Determine the test type:
   - **Ginkgo E2E tests** (most common): For testing deployed pod behavior, metrics collection, config processing
   - **Go unit tests**: For testing pure logic in shared modules
   - **Jest tests**: For TypeScript code in `tools/az-prom-rules-converter/`

2. For Ginkgo E2E tests:
   a. Choose the appropriate test suite directory under `otelcollector/test/ginkgo-e2e/` (configprocessing, prometheusui, containerstatus, operator, livenessprobe, querymetrics, regionTests).
   b. Create a new `*_test.go` file or add to an existing one.
   c. Use the existing `suite_test.go` pattern with `BeforeSuite` for K8s client setup.
   d. Use Ginkgo constructs: `Describe`, `Context`, `It`, `DescribeTable`, `Entry`.
   e. Use Gomega matchers: `Expect(err).NotTo(HaveOccurred())`, `Expect(value).To(Equal(...))`.
   f. Add appropriate labels: `Label(utils.ConfigProcessingCommon)`, `Label("operator")`, etc.
   g. Use test utilities from `otelcollector/test/utils/` for K8s client setup, Prometheus queries, API responses.

3. If a new test label is needed:
   a. Add a string constant to `otelcollector/test/utils/constants.go`.
   b. Document the label in `otelcollector/test/README.md`.
   c. Add to `.github/pull_request_template.md` checklist.
   d. Add to `otelcollector/test/testkube/testkube-test-crs.yaml`.

4. If a new test suite (new folder) is needed:
   a. Create `suite_test.go` with `BeforeSuite` following existing suites.
   b. Create a `go.mod` with appropriate dependencies.
   c. Add to `otelcollector/test/testkube/testkube-test-crs.yaml`.

### Files Typically Involved
- `otelcollector/test/ginkgo-e2e/<suite>/*_test.go`
- `otelcollector/test/ginkgo-e2e/<suite>/suite_test.go`
- `otelcollector/test/utils/constants.go` (for new labels)
- `otelcollector/test/utils/*.go` (shared helpers)
- `tools/az-prom-rules-converter/__tests__/*.test.ts` (TypeScript tests)

### Validation
- `go test -v ./...` passes in the test suite directory
- `npm test` passes for TypeScript tests
- New labels documented in README and PR template

## Examples from This Repo
- `otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go` — DescribeTable pattern for checking container status
- `otelcollector/test/ginkgo-e2e/prometheusui/prometheus_ui_test.go` — API endpoint testing with response parsing

## References
- Test README: `otelcollector/test/README.md`
- Test utilities: `otelcollector/test/utils/`
- Ginkgo docs: https://onsi.github.io/ginkgo/
- Gomega docs: https://onsi.github.io/gomega/
