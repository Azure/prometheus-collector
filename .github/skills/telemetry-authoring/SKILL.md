# Telemetry Authoring

## Description
Guide for adding telemetry instrumentation (metrics, traces, logging) to prometheus-collector components following existing patterns.

USE FOR: add telemetry, add metrics, add tracing, add observability, instrument code, track event, emit metric, add logging, add OpenTelemetry
DO NOT USE FOR: fixing broken telemetry pipelines, configuring telemetry infrastructure, dashboard creation, alert rule authoring

## Instructions

### When to Apply
When adding new functionality that needs observability coverage, or when filling telemetry gaps in existing code.

### Step-by-Step Procedure

#### 1. Identify the Telemetry Pattern
Before adding any telemetry, examine existing files in the same module:

**Go components** use:
- `log.Println()` / `log.Fatalf()` — standard library logging
- `github.com/prometheus/client_golang/prometheus` — Prometheus client metrics
- OpenTelemetry SDK for OTLP export

**Reference apps** demonstrate both approaches:
- Prometheus client: `prometheus.NewCounterVec`, `promauto.NewHistogram`
- OpenTelemetry: `otel.Meter("reference-app")`, `meter.Int64Counter`

#### 2. What to Instrument

**Error paths** (highest priority):
- Every `if err != nil` block representing an unexpected failure
- Include error type, message, operation context
- Use `log.Printf("error: %s: %v", operation, err)`

**Entry points and boundaries**:
- HTTP handlers, gRPC endpoints, collector pipeline stages
- Track operation name, duration, success/failure

**External calls**:
- Azure SDK calls, IMDS token requests, Kubernetes API calls
- Track target, duration, response status

**Configuration processing**:
- Config validation results, config reload events
- Track config source, validation errors, reload success/failure

#### 3. Telemetry Conventions
- **Metric naming**: `<component>_<operation>_<measurement>` (e.g., `config_validation_errors_total`)
- **Standard labels/dimensions**: `controller_type`, `os_type`, `component`
- **Environment awareness**: Check `TELEMETRY_DISABLED` env var before emitting
- **Error telemetry**: Include error type, message, source location

#### 4. Anti-Patterns to Avoid
- Do NOT log sensitive data (tokens, connection strings, PII)
- Do NOT add telemetry inside tight loops
- Do NOT use `fmt.Println` for production telemetry — use `log.Println`
- Do NOT introduce new telemetry libraries — use the existing patterns
- Do NOT hardcode instrumentation keys — use environment variables

### Validation
- Verify import statements match existing files
- Verify metric/event names follow the naming convention
- Run unit tests to ensure telemetry additions don't break test isolation
- Check that telemetry respects `TELEMETRY_DISABLED` flag

## References
- `internal/referenceapp/golang/main.go` — Prometheus + OpenTelemetry reference
- `internal/referenceapp/python/main.py` — Python telemetry reference
- `otelcollector/fluent-bit/src/` — Fluent Bit plugin with Application Insights
