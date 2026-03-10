# Test Authoring

## Description
Guide for adding new E2E Ginkgo tests, Go unit tests, or TypeScript tests following this repo's conventions.

USE FOR: add test, write test, test coverage, test for feature, add unit test, add integration test, add e2e test, ginkgo test
DO NOT USE FOR: fixing a flaky test, refactoring tests, test infrastructure changes, testkube configuration

## Instructions

### When to Apply
When adding new functionality that needs test coverage, or when the PR checklist requires new tests.

### Step-by-Step Procedure

#### For Go Unit Tests
1. Create test file alongside the source: `<module>/<package>/<name>_test.go`.
2. Use `testing.T` and `github.com/stretchr/testify/require` for assertions.
3. Use table-driven tests with `[]struct{}` test tables.
4. Run with `go test ./...` in the module directory.

#### For Ginkgo E2E Tests
1. Determine the appropriate suite under `otelcollector/test/ginkgo-e2e/` (operator, querymetrics, configprocessing, etc.).
2. Add test cases using Ginkgo BDD syntax: `Describe`/`It`/`BeforeAll`.
3. If a new scrape job is needed, add it to `otelcollector/test/test-cluster-yamls/` in the correct configmap or as a CR.
4. If a new test label is needed:
   - Add a string constant to `otelcollector/test/utils/constants.go`
   - Document the label in `otelcollector/test/README.md`
   - Add the label to `otelcollector/test/testkube/testkube-test-crs.yaml`
5. If additional API server permissions are needed, update `otelcollector/test/testkube/api-server-permissions.yaml`.
6. New test suites (new folders under `/test/ginkgo-e2e/`) must be registered in `testkube-test-crs.yaml`.

#### For TypeScript Tests
1. Create test file alongside source: `<name>.test.ts` in `tools/az-prom-rules-converter/src/`.
2. Use Jest patterns: `describe`/`test`/`expect`.
3. Run with `npm test`.

### Files Typically Involved
- `otelcollector/test/ginkgo-e2e/<suite>/*_test.go`
- `otelcollector/test/utils/constants.go`
- `otelcollector/test/test-cluster-yamls/`
- `otelcollector/test/testkube/testkube-test-crs.yaml`
- `otelcollector/test/README.md`
- `tools/az-prom-rules-converter/src/*.test.ts`

### Validation
- E2E tests pass on a bootstrapped AKS cluster with appropriate labels
- Unit tests pass with `go test ./...`
- TypeScript tests pass with `npm test`

## Examples from This Repo
- `b03f03f` — test: Testkube workflow migration (#1392)
- `afc1fc7` — test: minimal ingestion profile test cases (#1305)
- `600f94d` — test: support sequential cluster deployment (#1349)
