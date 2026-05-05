# Copilot Instructions for prometheus-collector

## Project Overview

Azure Monitor managed service for Prometheus — an agent that collects Prometheus metrics from AKS clusters and sends them to Azure Monitor. Runs as a DaemonSet (per-node) and/or ReplicaSet, with optional Target Allocator for sharding scrape targets across replicas.

## Architecture

The system has three deployment modes: **AKS addon** (managed by Azure), **Arc extension** (hybrid/multicloud), and **CCP** (Container Control Plane, internal Azure infrastructure).

### Key Components (all under `otelcollector/`)

- **opentelemetry-collector-builder/** — Custom OTel Collector binary with prometheus receiver, OTLP exporter, batch/resource/filter processors
- **prometheusreceiver/** — Forked/customized Prometheus receiver (upstream OTel contrib with local patches)
- **otel-allocator/** — Target Allocator that distributes scrape targets to collector replicas via consistent hashing. Forked from opentelemetry-operator
- **main/** — Entrypoint orchestrator that parses ConfigMaps, starts ME (MetricsExtension), otelcollector, and fluent-bit
- **shared/** — Shared Go utilities, config parsing, health metrics
- **shared/configmap/mp/** — ConfigMap parser for managed Prometheus (AKS addon/Arc)
- **shared/configmap/ccp/** — ConfigMap parser for CCP mode
- **configuration-reader-builder/** — Sidecar that watches ConfigMaps and regenerates collector config at runtime
- **fluent-bit/** — Custom Go output plugin for App Insights telemetry
- **prom-config-validator-builder/** — Validates customer-provided prometheus scrape configs
- **deploy/** — Helm charts for addon and standalone deployment

### Supporting Directories

- **toggles/** — Feature flag JSON files controlling rollout (image tags, resource limits, etc.)
- **internal/** — Internal tooling, reference apps, monitoring dashboards, upgrade scripts
- **mixins/** — Prometheus recording rules (coredns, node, kubernetes) using jsonnet
- **AddonArmTemplate/, AddonBicepTemplate/, ArcArmTemplate/** — ARM/Bicep templates for onboarding

## Build & Test Commands

### Go Modules

The repo has multiple Go modules (not a single workspace). Key modules:

| Module | Directory |
|--------|-----------|
| `prometheus-collector` | `otelcollector/` (root module) |
| `otel-allocator` | `otelcollector/otel-allocator/` |
| `prometheusreceiver` | `otelcollector/prometheusreceiver/` |
| `shared` | `otelcollector/shared/` |

### Building

```bash
# Build all components (from otelcollector/opentelemetry-collector-builder/)
cd otelcollector/opentelemetry-collector-builder && make all

# Build individual components
cd otelcollector/opentelemetry-collector-builder && make otelcollector
cd otelcollector/otel-allocator && make targetallocator
cd otelcollector/configuration-reader-builder && make configurationreader
```

### Running Tests

```bash
# Target allocator tests
cd otelcollector/otel-allocator && go test ./...

# Run a single test
cd otelcollector/otel-allocator && go test ./internal/prehook/ -run TestRelabel

# Shared module tests (uses Ginkgo)
cd otelcollector/shared/configmap/mp && go test ./...

# Prometheus receiver tests
cd otelcollector/prometheusreceiver && go test ./...

# E2E tests (require cluster connection)
cd otelcollector/test/ginkgo-e2e && go test ./querymetrics/ -v
```

### Docker Images

PR builds produce test images with tags: `0.0.0-{branch}-{date}-{commit}` (plus `-win`, `-cfg`, `-targetallocator` suffixes).

## Conventions

### Go Code

- Use `shared.GetEnv(key, default)` instead of raw `os.Getenv()` for env vars with defaults
- Use `shared.EchoError()` / `shared.EchoWarning()` for structured logging in config parsers
- CCP mode uses JSON structured logging via `shared.SetupCCPLogging()` — configure early in main
- Platform-specific code uses Go build tags: `_linux.go` / `_windows.go` suffixes
- The `shared` package is referenced via replace directives in go.mod, not published externally

### Target Allocator

- Forked from `opentelemetry-operator/cmd/otel-allocator` — keep module path as `github.com/open-telemetry/opentelemetry-operator/cmd/otel-allocator`
- Allocation strategies: `consistent-hashing` (default), `least-weighted`, `per-node`
- When writing tests with `relabel.Config`, set `NameValidationScheme: model.UTF8Validation` to avoid panics
- The relabel prehook stores ORIGINAL labels (not relabeled) — Prometheus receiver applies relabeling from scratch

### ConfigMap Parsing

- Config parsers read TOML ConfigMaps and set environment variables consumed by downstream components
- Two parallel config paths: `mp` (managed prometheus) and `ccp` (container control plane)
- Environment variables are set via `shared.SetEnvAndSourceBashrcOrPowershell()`

### Feature Flags / Toggles

- JSON files in `toggles/` control image tags, resource limits, and feature enablement
- Changes to toggles affect production rollout — treat with care

### CI/CD

- Primary pipelines are Azure DevOps (`.pipelines/`), not GitHub Actions
- GitHub Actions (`.github/workflows/`) handle auxiliary tasks: scanning, mixin builds, dependent charts
- The `OneBranch.Official.yml` pipeline is the main production release pipeline
