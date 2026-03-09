# prometheus-collector

Azure Monitor managed service for Prometheus — a Kubernetes-based agent that collects Prometheus metrics from cluster workloads and sends them to Azure Monitor's managed Prometheus backend. Built as a custom OpenTelemetry Collector distribution with Prometheus receiver, target allocator, Fluent Bit log forwarding, and configuration validation.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Core Language | Go 1.24 |
| Collector Framework | OpenTelemetry Collector v0.144.0 |
| Metrics Protocol | Prometheus / OTLP |
| Log Forwarding | Fluent Bit (Go plugin) |
| Container Runtime | Docker (multi-stage, multi-arch) |
| Orchestration | Kubernetes (DaemonSet + Deployment) |
| Package Manager | Helm 3 |
| IaC | Bicep, Terraform, ARM Templates |
| CI/CD | GitHub Actions + Azure Pipelines |
| E2E Testing | Ginkgo v2 (on live AKS clusters) |
| Unit Testing | Go `testing` + testify, Jest (TypeScript) |
| Security Scanning | Trivy, Dependabot |
| CLI Tooling | TypeScript (Commander.js) |
| Mixins | Jsonnet (kubernetes, node, coredns) |

## Architecture Overview

The collector runs as a DaemonSet (per-node scraping) and optionally as a Deployment (centralized). Key components:
- **Main orchestrator** (`otelcollector/main/`) — initializes subsystems, handles config parsing
- **Custom OTel Collector** (`otelcollector/opentelemetry-collector-builder/`) — the collector binary
- **Prometheus Receiver** (`otelcollector/prometheusreceiver/`) — custom fork of the contrib receiver
- **Target Allocator** (`otelcollector/otel-allocator/`) — distributes scrape targets across replicas
- **Config Validator** (`otelcollector/prom-config-validator-builder/`) — validates Prometheus scrape configs
- **Configuration Reader** (`otelcollector/configuration-reader-builder/`) — reads and parses configs
- **Fluent Bit plugin** (`otelcollector/fluent-bit/`) — Go plugin for Application Insights log shipping
- **Shared libraries** (`otelcollector/shared/`) — common code for MP and CCP config modes

## Functional Requirements

### 1) Prometheus metric collection from Kubernetes workloads
Scrape Prometheus endpoints discovered via static config, PodMonitor, and ServiceMonitor CRDs.

### 2) Metric forwarding to Azure Monitor
Export collected metrics via OTLP to Azure Monitor's managed Prometheus backend.

### 3) Multi-platform support
Run on Linux and Windows nodes, amd64 and arm64 architectures, across AKS and Azure Arc clusters.

### 4) Dynamic target allocation
Distribute scrape targets across collector replicas to avoid duplicate scraping.

### 5) Configuration validation
Validate user-provided Prometheus scrape configurations before applying them.

## Non-Functional Requirements

- **Security**: Hardened Go builds with PIE and RELRO, Trivy vulnerability scanning, Dependabot dependency updates
- **Observability**: Self-monitoring via Prometheus metrics, Fluent Bit log forwarding to Application Insights
- **Performance**: Scale/perf testing required for feature PRs (per PR template)
- **Compatibility**: Support Prometheus operator CRDs (PodMonitor, ServiceMonitor)

## Expected Project Files

| Path | Purpose |
|------|---------|
| `otelcollector/opentelemetry-collector-builder/` | Main collector build with Makefile |
| `otelcollector/prometheusreceiver/` | Custom Prometheus receiver |
| `otelcollector/otel-allocator/` | Target allocation service |
| `otelcollector/shared/` | Shared config parsing libraries |
| `otelcollector/build/linux/Dockerfile` | Multi-stage Linux container build |
| `otelcollector/deploy/chart/` | Helm chart templates |
| `otelcollector/test/ginkgo-e2e/` | E2E test suites |
| `tools/az-prom-rules-converter/` | Prometheus rules conversion CLI |
| `mixins/` | Jsonnet-based Prometheus mixins |

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `TELEMETRY_DISABLED` | Set to `true` to disable telemetry collection |
| `CONFIG_VALIDATOR_RUNNING_IN_AGENT` | Indicates validator runs inside the agent container |
| `OS_TYPE` | Target OS type (linux/windows) |
| `CONTROLLER_TYPE` | Kubernetes controller type (DaemonSet/ReplicaSet) |
| `GOLANG_VERSION` | Go version for Docker builds |
| `PROMETHEUS_VERSION` | Prometheus version bundled in collector |

## Acceptance Criteria

- All existing Ginkgo E2E tests pass on a live AKS cluster
- `make all` in `otelcollector/opentelemetry-collector-builder/` succeeds
- Trivy scan passes with no new CRITICAL/HIGH vulnerabilities
- TypeScript tests pass: `cd tools/az-prom-rules-converter && npm test`
- Conventional Commit format used in PR title
- PR template checklist completed
