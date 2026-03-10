# Bug Fix

## Description
Structured workflow for fixing bugs in the prometheus-collector with proper regression testing.

USE FOR: fix bug, resolve issue, patch, hotfix, debug, error fix
DO NOT USE FOR: feature development, refactoring, performance optimization

## Instructions

### When to Apply
When a bug is reported via GitHub issue, detected in E2E tests, or identified during code review.

### Step-by-Step Procedure
1. **Reproduce**: Identify the affected component and deployment mode (DaemonSet, ReplicaSet, Operator). Check if the bug is platform-specific (Linux/Windows, amd64/arm64).
2. **Locate**: Find the relevant source files. Key directories by component:
   - Collector core: `otelcollector/opentelemetry-collector-builder/`
   - Config processing: `otelcollector/shared/configmap/`
   - Prometheus receiver: `otelcollector/prometheusreceiver/`
   - Helm/deployment: `otelcollector/deploy/`
   - Fluent Bit: `otelcollector/fluent-bit/src/`
3. **Fix**: Apply the minimal change. Follow Go error handling conventions (`fmt.Errorf("context: %w", err)`).
4. **Test**: Add a regression test in the appropriate Ginkgo E2E suite, or document why a test is not feasible.
5. **Verify**: Build the affected module, run tests, check for multi-module impact if shared code is changed.
6. **Commit**: Use `fix:` prefix: `fix: <description of what was fixed> (#issue)`.

### Files Typically Involved
- Go source files in `otelcollector/*/`
- Helm chart templates in `otelcollector/deploy/*/templates/`
- Test files in `otelcollector/test/ginkgo-e2e/`
- Shell scripts in `otelcollector/scripts/`

### Validation
- `go build ./...` in affected modules
- `go test ./...` in affected modules
- Ginkgo E2E tests pass with relevant labels
- No regressions in other deployment modes

## Examples from This Repo
- `81b03f2` — fix: update acstor node-agent pod selector for label changes
- `a68f1a8` — fix: Correct node affinity syntax in ama-metrics DS
- `e8867d0` — fix: proxy basic auth for mdsd
