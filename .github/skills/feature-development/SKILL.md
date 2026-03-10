# Feature Development

## Description
Guide for adding new features to the prometheus-collector, including new scrape targets, exporters, configuration options, and deployment modes.

USE FOR: add feature, implement, new endpoint, new component, new module, new scrape target, new exporter, create
DO NOT USE FOR: bug fixes, refactoring, documentation-only changes, dependency updates

## Instructions

### When to Apply
When adding a new capability such as a new metrics source, exporter, configuration option, or deployment mode.

### Step-by-Step Procedure
1. **Identify the component**: Determine which module needs changes — collector builder, receiver, allocator, shared libraries, or deploy charts.
2. **Implement the feature** in the appropriate Go module under `otelcollector/`.
3. **Add configuration support**: Update ConfigMap templates in `otelcollector/configmaps/` or shared config parsers in `otelcollector/shared/`.
4. **Update Helm charts** if the feature requires new deployment options: `otelcollector/deploy/chart/prometheus-collector/templates/`.
5. **Add tests**: Add Ginkgo E2E test cases and/or Go unit tests covering the new feature.
6. **Document telemetry**: List any new telemetry emitted by the feature (per PR template).
7. **Update version files** if this is a release-worthy change.
8. **Commit with `feat:` prefix**: e.g., `feat: Add OperationEnvironment argument to MetricsExtension (#1403)`.

### Files Typically Involved
- `otelcollector/<component>/` — feature implementation
- `otelcollector/shared/configmap/` — configuration support
- `otelcollector/deploy/chart/` — Helm chart changes
- `otelcollector/test/ginkgo-e2e/` — E2E tests
- `otelcollector/test/test-cluster-yamls/` — test configurations
- `otelcollector/build/linux/Dockerfile` — if new build stages needed

### Validation
- `make all` succeeds
- New E2E tests pass on a bootstrapped AKS cluster
- PR template checklist completed (telemetry, one-pager, perf testing)
- Commit uses `feat:` prefix

## Examples from This Repo
- `b98f324` — feat: Add OperationEnvironment argument to MetricsExtension (#1403)
- `85aa399` — Add DCGM exporter support for GPU metrics collection (#1391)
- `f7dc82d` — Add AI resource for bleu cloud (#1408)
