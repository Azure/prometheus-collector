# Feature Development

## Description
Guide for adding new features to the prometheus-collector following project conventions.

USE FOR: add feature, implement, new endpoint, new component, new module, create new scrape config
DO NOT USE FOR: bug fixes, refactoring, documentation only changes

## Instructions

### When to Apply
When implementing a new capability, adding a new scrape target, supporting a new Azure cloud feature, or adding a new deployment option.

### Step-by-Step Procedure
1. **Plan**: Identify which components need changes. This repo has 8+ components:
   - OTel Collector (`otelcollector/opentelemetry-collector-builder/`)
   - Prometheus Receiver (`otelcollector/prometheusreceiver/`)
   - Configuration Reader (`otelcollector/configuration-reader-builder/`)
   - Shared libraries (`otelcollector/shared/`)
   - Fluent Bit plugin (`otelcollector/fluent-bit/src/`)
   - Prometheus UI (`otelcollector/prometheus-ui/`)
   - Target Allocator (`otelcollector/otel-allocator/`)
   - Helm charts (`otelcollector/deploy/`)

2. **Implement**: Write Go code following conventions — PascalCase exports, `fmt.Errorf` error wrapping, env var config via `shared.GetEnv()`.

3. **Configure**: If the feature needs new configuration:
   - Add environment variables and document them
   - Update Helm chart values in `otelcollector/deploy/*/values.yaml`
   - Update ConfigMap parsing in `otelcollector/shared/configmap/`

4. **Test**: Add Ginkgo E2E tests with appropriate labels. If a new scrape job is needed, add it to `otelcollector/test/test-cluster-yamls/`.

5. **Document**: Update the PR with the New Feature Checklist:
   - List telemetry added
   - Link to one-pager
   - List release tasks
   - Attach scale/perf results

6. **Build**: Ensure multi-arch Docker builds succeed for Linux (amd64/arm64) and Windows if applicable.

7. **Commit**: Use `feat:` prefix: `feat: <description> (#PR)`.

### Files Typically Involved
- Go source in `otelcollector/*/`
- Helm chart values and templates in `otelcollector/deploy/`
- Docker build files in `otelcollector/build/`
- Test files in `otelcollector/test/ginkgo-e2e/`
- Test cluster yamls in `otelcollector/test/test-cluster-yamls/`

### Validation
- `go build ./...` in affected modules
- Ginkgo E2E tests pass with relevant labels
- Docker multi-arch build succeeds
- Helm chart renders correctly (`helm template`)
- Trivy scan clean

## Examples from This Repo
- `308d8df` — Added OTel gRPC ports support in extension chart (#1438)
- `b98f324` — feat: Add OperationEnvironment argument to MetricsExtension
- `ce58307` — Enable DCGM exporter by default and optimize label handling
