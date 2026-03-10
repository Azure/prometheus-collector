# Repository Instructions

## Summary

This is the **Azure Monitor Prometheus Metrics Collector** (`prometheus-collector`), an enterprise-grade OpenTelemetry-based metrics collection agent for Kubernetes. It collects Prometheus metrics from pods and services running on AKS (Azure Kubernetes Service) and Azure Arc-enabled clusters, then forwards them to Azure Monitor. Primary languages: Go (~20%), YAML (~27%), TypeScript (~5%). Built on OpenTelemetry Collector v0.144.0 with a custom Prometheus receiver, Fluent Bit log forwarder, and Prometheus UI.

## General Guidelines

1. Follow the existing code conventions documented in `.github/instructions/go.instructions.md` for Go code and `.github/instructions/typescript.instructions.md` for TypeScript code.
2. Use the PR template at `.github/pull_request_template.md` — it requires test checklists, telemetry notes, and scale/perf results for new features.
3. If newer commits make prior changes unnecessary, revert them rather than layering workarounds.
4. Run Ginkgo E2E tests on a bootstrapped cluster before submitting PRs (see `otelcollector/test/README.md`).
5. Never hardcode secrets — use environment variables (`APPLICATIONINSIGHTS_AUTH`, `CLUSTER`, `AKSREGION`, `customEnvironment`).
6. This is a monorepo with 24 Go modules — changes to shared modules (`otelcollector/shared/`) may affect multiple components.

## Prompting Best Practices

1. Break complex tasks into smaller prompts — one module or one Go file at a time. This repo has 24 Go modules with `replace` directives, so context matters.
2. Be specific: reference actual file paths like `otelcollector/opentelemetry-collector-builder/main.go` or `otelcollector/shared/configmap/mp/`.
3. Open relevant files before prompting — the `otelcollector/` directory contains 80% of the codebase.
4. Start new chat sessions when switching between otelcollector core code and tools/infrastructure.
5. Use the explore → plan → code → commit workflow for multi-file changes (see `AGENTS.md`).
6. Always validate AI-generated code: run `go build`, `go test ./...`, and check Trivy scans.

## Custom Agents

| Agent | Triggers | Description |
|-------|----------|-------------|
| @CodeReviewer | review PR, review code | Structured code review following repo conventions |
| @SecurityReviewer | security review, threat model | Deep STRIDE-based security analysis |
| @ThreatModelAnalyst | threat model analysis | Full threat model with Mermaid diagrams and persistent artifacts |
| @DocumentWriter | write docs, update README | Documentation following repo conventions |
| @prd | create PRD, write requirements | Product Requirements Document generation |

## Task-Specific Skills

| Skill | Triggers | Description |
|-------|----------|-------------|
| dependency-update | update dependency, bump package | Safe Go module and npm dependency updates |
| test-authoring | add test, write test | Create Ginkgo E2E tests following repo patterns |
| bug-fix | fix bug, resolve issue, hotfix | Structured bug fix with regression tests |
| feature-development | add feature, implement, new module | New feature scaffolding with proper placement |
| ci-cd-pipeline | update pipeline, CI change | Modify GitHub Actions or Azure Pipelines |
| infrastructure | update helm, change deployment | Helm chart and IaC changes |
| security-review | security review, STRIDE analysis | STRIDE-based security review |
| telemetry-authoring | add telemetry, add metrics | Add telemetry following existing patterns |
| fix-critical-vulnerabilities | fix CVE, trivy fix | Remediate critical/high vulnerabilities |

## Build Instructions

```bash
# Prerequisites: Go 1.23+, Docker, kubectl, Helm 3

# Build the OTel collector (from otelcollector/opentelemetry-collector-builder/)
cd otelcollector/opentelemetry-collector-builder
make all

# Build the TypeScript rules converter
cd tools/az-prom-rules-converter
npm install && npm run build

# Run TypeScript tests
cd tools/az-prom-rules-converter
npm test

# Run Ginkgo E2E tests (requires bootstrapped K8s cluster)
cd otelcollector/test/ginkgo-e2e/<suite>
go test -v ./...
```

## Known Patterns & Gotchas

- Multiple `go.mod` files use `replace` directives for local modules — run `go mod tidy` in each affected module after dependency changes.
- Dependabot ignores `go.opentelemetry.io/collector*` and `github.com/open-telemetry/opentelemetry-collector-contrib*` — these are updated manually via the OTel upgrade process (`internal/otel-upgrade-scripts/upgrade.sh`).
- Docker builds are multi-arch (amd64/arm64) and multi-stage — changes to one stage can break downstream stages.
- The `.trivyignore` file tracks temporarily suppressed CVEs — check it when updating dependencies.
- Application Insights keys are base64-encoded and environment-specific (Public, Fairfax, Mooncake, USNat, USSec).
