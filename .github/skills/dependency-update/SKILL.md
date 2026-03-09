# Dependency Update

## Description
Guide for safely updating Go module dependencies, npm packages, and GitHub Actions versions in this multi-module repository.

USE FOR: update dependency, bump package, upgrade library, renovate, dependabot, update go.mod, update package.json
DO NOT USE FOR: adding a brand new dependency, removing a dependency, upgrading OpenTelemetry Collector versions (use the automated otelcollector-upgrade.yml workflow instead)

## Instructions

### When to Apply
When Dependabot PRs need manual intervention, or when manually bumping a dependency version.

### Step-by-Step Procedure
1. Identify the correct `go.mod` file — this repo has 23+ Go modules. Check `otelcollector/opentelemetry-collector-builder/go.mod`, `otelcollector/fluent-bit/src/go.mod`, `otelcollector/prom-config-validator-builder/go.mod`, `internal/referenceapp/golang/go.mod`, etc.
2. Update the dependency version in the appropriate `go.mod` file.
3. Run `go mod tidy` in the module directory to update `go.sum`.
4. If the dependency is shared across modules, check other `go.mod` files for the same dependency and update them consistently.
5. For npm packages: update `tools/az-prom-rules-converter/package.json` and run `npm install` to regenerate `package-lock.json`.
6. Build the affected component to verify compatibility: `cd otelcollector/opentelemetry-collector-builder && make all`.
7. Run tests for affected modules.

### Files Typically Involved
- `otelcollector/opentelemetry-collector-builder/go.mod` and `go.sum`
- `otelcollector/fluent-bit/src/go.mod` and `go.sum`
- `otelcollector/prom-config-validator-builder/go.mod` and `go.sum`
- `internal/referenceapp/golang/go.mod` and `go.sum`
- `tools/az-prom-rules-converter/package.json` and `package-lock.json`
- `.trivyignore` (if CVE-related)

### Validation
- `make all` succeeds in `otelcollector/opentelemetry-collector-builder/`
- `npm test` passes in `tools/az-prom-rules-converter/`
- No new CRITICAL/HIGH Trivy findings

## Examples from This Repo
- `49c9c8e` — build(deps): bump k8s.io/client-go from 0.34.2 to 0.35.1 (#1413)
- `c6c5a4d` — build(deps): bump go.opentelemetry.io/otel/exporters/stdout (#1421)
- `be1a206` — build(deps): bump ajv from 8.11.2 to 8.18.0 (#1416)
