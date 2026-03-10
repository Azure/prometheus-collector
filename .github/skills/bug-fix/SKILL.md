# Bug Fix

## Description
Structured workflow for fixing bugs in the prometheus-collector, including diagnosis, fix implementation, and regression test addition.

USE FOR: fix bug, resolve issue, patch, hotfix, debug, error fix, crash fix
DO NOT USE FOR: feature development, refactoring, performance optimization, dependency updates

## Instructions

### When to Apply
When addressing a reported bug, crash, or incorrect behavior in the collector, configuration parsing, target allocation, or metric collection.

### Step-by-Step Procedure
1. **Identify the affected component**: Determine which module is affected — main orchestrator (`otelcollector/main/`), Prometheus receiver (`otelcollector/prometheusreceiver/`), target allocator (`otelcollector/otel-allocator/`), config validator (`otelcollector/prom-config-validator-builder/`), shared libraries (`otelcollector/shared/`), or Fluent Bit plugin (`otelcollector/fluent-bit/`).
2. **Reproduce the issue**: Check logs, configuration, and runtime environment. For collector issues, check the Prometheus scrape config and OTel pipeline config.
3. **Implement the fix** in the correct module, following Go conventions.
4. **Add a regression test**: Every bug fix should include a test that would have caught the issue.
5. **Build and test**: Run `make all` in `otelcollector/opentelemetry-collector-builder/` and run unit tests for the affected module.
6. **Commit with `fix:` prefix**: e.g., `fix: correct node affinity syntax in ama-metrics DS (#1234)`.

### Files Typically Involved
- `otelcollector/main/main.go` — orchestrator bugs
- `otelcollector/prometheusreceiver/` — scraping bugs
- `otelcollector/shared/` — config parsing bugs
- `otelcollector/otel-allocator/` — target allocation bugs
- `otelcollector/deploy/chart/` — Helm chart/manifest bugs

### Validation
- `make all` succeeds
- Unit tests pass for the affected module
- Regression test covers the bug scenario
- PR follows Conventional Commits: `fix: <description>`

## Examples from This Repo
- `3b36c58` — BUG: Missing log columns: Add pod and containerID columns (#1398)
- `e8867d0` — fix: proxy basic auth for mdsd (#1383)
- `a68f1a8` — fix: Correct node affinity syntax in ama-metrics DS
