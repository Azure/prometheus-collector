# Copilot Instructions — prometheus-collector

## Repository Context

This is the **Azure Managed Prometheus** (prometheus-collector) repository. It contains the AMA metrics addon for AKS clusters that collects Prometheus metrics and sends them to Azure Monitor Workspaces.

## Troubleshooting Tools

This repo includes an MCP server for ICM investigation and troubleshooting:

- **`tools/prom-collector-tsg-mcp/`** — MCP server with 10 diagnostic tools
  - Build: `cd tools/prom-collector-tsg-mcp && npm install && npx tsc`
  - All tools require a cluster ARM resource ID as input
  - Start with `tsg_triage`, then drill into `tsg_errors`, `tsg_workload`, `tsg_pods`
  - Use `tsg_mdm_throttling` to check Geneva MDM QoS account throttling (requires Geneva MDM MCP server on port 5050)

## Key Data Sources

The TSG tools query these Kusto clusters:
- **PrometheusAppInsights** — Collector telemetry (logs, metrics, configs)
- **MetricInsights** — Time series counts and ingestion rates
- **AMWInfo** — Azure Monitor Workspace and DCR mapping
- **AKS / AKS CCP / AKS Infra** — Cluster state, control plane, pod health
- **Vulnerabilities** — Image CVE scanning

## Skills

Available skills are in `.github/skills/`:
- **troubleshooting-setup** — Complete onboarding guide for setting up the troubleshooting environment

## Architecture

- `otelcollector/` — OpenTelemetry collector configurations
- `internal/` — Internal scripts including troubleshooting PowerShell scripts
- `mixins/` — Kubernetes monitoring mixins (recording rules, alerts, runbooks)
- `tools/` — Development and diagnostic tools
