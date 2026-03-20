# Dependency Update

## Description
Guide for safely updating Go module and npm dependencies in this monorepo.

USE FOR: update dependency, bump package, upgrade library, renovate, dependabot, update go.mod, update package.json
DO NOT USE FOR: adding a brand new dependency, removing a dependency, major OTel collector version migration (use internal/otel-upgrade-scripts/upgrade.sh for that)

## Instructions

### When to Apply
When Dependabot PRs need manual intervention, when upgrading dependencies manually, or when a security advisory requires a version bump.

### Step-by-Step Procedure
1. Identify which `go.mod` file(s) need updating. This repo has 24 `go.mod` files — check if the dependency appears in multiple modules:
   - `otelcollector/opentelemetry-collector-builder/go.mod` (main collector)
   - `otelcollector/prometheusreceiver/go.mod`
   - `otelcollector/fluent-bit/src/go.mod`
   - `otelcollector/shared/go.mod` and `otelcollector/shared/configmap/*/go.mod`
   - `otelcollector/test/ginkgo-e2e/*/go.mod` (8 test modules)
   - `tools/az-prom-rules-converter/package.json` (TypeScript)
2. For Go dependencies: run `go get <package>@<version>` in each affected module directory.
3. Run `go mod tidy` in each affected module directory.
4. For npm dependencies: run `npm install` in `tools/az-prom-rules-converter/`.
5. Build affected components to verify compatibility.
6. Run existing tests to ensure nothing broke.

### Files Typically Involved
- `otelcollector/*/go.mod`, `otelcollector/*/go.sum`
- `tools/az-prom-rules-converter/package.json`, `package-lock.json`
- `.trivyignore` (if adding/removing CVE exceptions)

### Validation
- `go build ./...` succeeds in affected modules
- `go test ./...` passes in affected modules
- `npm test` passes in `tools/az-prom-rules-converter/` (if changed)
- No new Trivy CRITICAL/HIGH findings

## Examples from This Repo
- `49c9c8e` — build(deps): bump k8s.io/client-go from 0.34.2 to 0.35.1
- `c6c5a4d` — build(deps): bump go.opentelemetry.io/otel/exporters

## References
- Dependabot config: `.github/dependabot.yml`
- OTel upgrade scripts: `internal/otel-upgrade-scripts/upgrade.sh`
- Note: `go.opentelemetry.io/collector*` and `github.com/open-telemetry/opentelemetry-collector-contrib*` are ignored by Dependabot and updated manually via the OTel upgrade process.
