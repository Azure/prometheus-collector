# Repository Instructions

## Summary

This repository is **Azure Monitor managed service for Prometheus** (`prometheus-collector`), a Kubernetes-based agent that collects Prometheus metrics and sends them to Azure Monitor. Primary languages are **Go** (~17%) and **YAML** (~25%), with TypeScript tooling. It builds a custom **OpenTelemetry Collector** distribution (v0.144.0) with Prometheus receiver, target allocator, Fluent Bit integration, and configuration validation. Targets **AKS**, **Azure Arc**, and on-premise clusters across Linux/Windows on amd64/arm64.

## General Guidelines

1. This is a Go monorepo with 23+ `go.mod` files — always identify the correct module before making changes.
2. Follow Conventional Commits format for PR titles: `feat:`, `fix:`, `test:`, `build:`, `docs:`.
3. OpenTelemetry Collector and contrib dependencies are pinned and upgraded together via the automated `otelcollector-upgrade.yml` workflow — do NOT bump them individually via Dependabot.
4. If newer commits make prior changes unnecessary, revert them rather than layering fixes.
5. All PRs must follow the checklist in `.github/pull_request_template.md`, including telemetry documentation and E2E test evidence.

## Build Instructions

### Prerequisites
- Go 1.24+ (toolchain 1.24.7)
- Docker (multi-stage builds)
- Node.js 18+ (for `az-prom-rules-converter` tool only)
- `make`, `gcc` (for CGO-enabled builds)

### Build the collector
```bash
cd otelcollector/opentelemetry-collector-builder
make all
```

### Build individual components
```bash
cd otelcollector/opentelemetry-collector-builder && make otelcollector
cd otelcollector/opentelemetry-collector-builder && make targetallocator
cd otelcollector/opentelemetry-collector-builder && make promconfigvalidator
cd otelcollector/opentelemetry-collector-builder && make fluentbitplugin
```

### Build TypeScript tool
```bash
cd tools/az-prom-rules-converter
npm install && npm run build
```

### Run TypeScript tests
```bash
cd tools/az-prom-rules-converter && npm test
```

### Run Ginkgo E2E tests (requires live AKS cluster)
```bash
cd otelcollector/test/ginkgo-e2e/<suite>
go test -v ./...
```

### Build Prometheus mixins
```bash
cd mixins/kubernetes && jb install && make all
```

## Known Patterns & Gotchas

- The main Dockerfile (`otelcollector/build/linux/Dockerfile`) uses security-hardened build flags: `-buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now'`.
- Local `replace` directives in `otelcollector/go.mod` point to `./shared`, `./shared/configmap/mp`, and `./shared/configmap/ccp` — these must be maintained when modifying shared code.
- Ginkgo E2E tests require a bootstrapped AKS cluster; they cannot run locally without cluster access. See `otelcollector/test/README.md`.
- The `.trivyignore` file tracks temporarily-accepted CVEs — entries need justification comments and follow-up dates.
- Version files `otelcollector/VERSION`, `OPENTELEMETRY_VERSION`, and `TARGETALLOCATOR_VERSION` are updated during releases.
- Dependabot ignores `go.opentelemetry.io/collector*` and `github.com/open-telemetry/opentelemetry-collector-contrib*` — these are upgraded via the dedicated workflow.
