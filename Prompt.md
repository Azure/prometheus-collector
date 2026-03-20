# Prometheus Collector

Azure Monitor Prometheus Metrics Collector — an enterprise OpenTelemetry-based agent that scrapes Prometheus metrics from Kubernetes workloads and forwards them to Azure Monitor.

## Tech Stack

| Component | Technology |
|-----------|------------|
| Core Language | Go 1.23 (toolchain 1.23.8) |
| Metrics Framework | OpenTelemetry Collector v0.144.0 |
| Prometheus Integration | Custom Prometheus Receiver, client_golang v1.23.2 |
| Log Forwarding | Fluent Bit (Go plugin, C-shared) |
| CLI Tools | TypeScript 4.8 (az-prom-rules-converter) |
| Container Runtime | Docker (multi-stage, multi-arch: amd64/arm64) |
| Orchestration | Kubernetes (Helm 3, DaemonSet + ReplicaSet) |
| IaC | ARM Templates, Bicep, Terraform |
| CI/CD | GitHub Actions + Azure Pipelines |
| Testing | Ginkgo v2 (BDD), Jest (TypeScript) |
| Cloud Platform | Azure (AKS, Azure Arc, Azure Monitor) |

## Architecture Overview

The collector runs as pods in Kubernetes clusters (AKS or Arc-enabled). It consists of:
- **OTel Collector** with a custom Prometheus receiver that scrapes `/metrics` endpoints
- **Configuration Reader** that processes ConfigMaps and custom resources
- **Metrics Extension** that handles remote write to Azure Monitor Workspace
- **Fluent Bit plugin** for log forwarding to Application Insights
- **Prometheus UI** for debugging scrape targets
- **Target Allocator** for distributed target assignment across replicas

Deployment modes: ReplicaSet (cluster-level), DaemonSet (node-level), Operator Targets (CRD-based).

## Functional Requirements

### 1) Prometheus Metrics Collection
Scrape Prometheus metrics from pods, services, and endpoints in Kubernetes clusters using service discovery and static configs.

### 2) Azure Monitor Integration
Forward collected metrics to Azure Monitor Workspace via remote write, supporting all Azure cloud environments (Public, Fairfax, Mooncake, USNat, USSec, Bleu).

### 3) Multi-Platform Support
Run on Linux (amd64/arm64) and Windows (Server 2019/2022) nodes with appropriate container images.

### 4) Configuration Management
Support ConfigMap-based configuration, Prometheus Operator CRDs (PodMonitor, ServiceMonitor), and custom resource definitions.

## Non-Functional Requirements

- **Performance**: Handle high-cardinality metrics at scale; support scale/perf testing before feature merges.
- **Security**: Trivy vulnerability scanning (CRITICAL/HIGH), Dependabot automated updates, base64-encoded telemetry keys.
- **Observability**: Self-monitoring via Application Insights; liveness/readiness probes; Prometheus UI for debugging.
- **Deployment**: Helm-based deployment with ARM/Bicep/Terraform templates for Azure resource provisioning.

## Expected Project Files

| Path | Purpose |
|------|---------|
| `otelcollector/opentelemetry-collector-builder/` | Main OTel collector binary |
| `otelcollector/prometheusreceiver/` | Custom Prometheus scrape receiver |
| `otelcollector/shared/` | Shared Go libraries (config, telemetry, process mgmt) |
| `otelcollector/fluent-bit/src/` | Fluent Bit output plugin (Go, C-shared) |
| `otelcollector/deploy/` | Helm charts for Kubernetes deployment |
| `otelcollector/build/` | Dockerfiles (Linux + Windows) |
| `otelcollector/test/ginkgo-e2e/` | Ginkgo BDD E2E test suites |
| `tools/az-prom-rules-converter/` | TypeScript CLI for Prometheus rule conversion |
| `AddonArmTemplate/` | ARM templates for Azure addon deployment |
| `AddonBicepTemplate/` | Bicep templates for Azure addon deployment |
| `AddonTerraformTemplate/` | Terraform configs for Azure addon deployment |
| `mixins/` | Prometheus recording/alerting rule templates |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CLUSTER` | Kubernetes cluster name |
| `AKSREGION` | Azure region for the AKS cluster |
| `customEnvironment` | Azure cloud environment (AzurePublicCloud, AzureUSGovernmentCloud, etc.) |
| `MODE` | Collector mode (advanced, nodefault) |
| `CONTROLLER_TYPE` | Deployment type (DaemonSet, ReplicaSet) |
| `APPLICATIONINSIGHTS_AUTH` | Base64-encoded Application Insights key |
| `MAC` | Monitoring Account Configuration flag |

## Acceptance Criteria

- All Ginkgo E2E test suites pass on a bootstrapped cluster.
- TypeScript tests pass (`npm test` in `tools/az-prom-rules-converter/`).
- Go builds succeed across all 24 modules (`go build ./...`).
- Trivy scan reports no new CRITICAL/HIGH vulnerabilities.
- PR template checklist is fully completed.
