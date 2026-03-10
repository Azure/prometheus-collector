# Telemetry Authoring

## Description
Guide for adding telemetry instrumentation following existing patterns in the prometheus-collector.

USE FOR: add telemetry, add metrics, add tracing, add observability, instrument code, track event, emit metric, add logging, add Application Insights
DO NOT USE FOR: fixing broken telemetry pipelines, configuring telemetry infrastructure, dashboard creation, alert rule authoring

## Instructions

### When to Apply
When adding new features, error paths, or entry points that need observability coverage.

### Step-by-Step Procedure

#### 1. Telemetry Pattern Discovery
Before adding ANY telemetry, identify the existing pattern:
- **Go standard logging**: `log.Println()`, `log.Fatalf()` — used throughout the codebase
- **CCP JSON logging**: `shared.SetupCCPLogging()` for structured JSON output in control-plane mode
- **Application Insights**: Used for cloud telemetry (keys in env vars, endpoint routing per cloud environment)
- **Prometheus self-monitoring**: `:8888/metrics` endpoint for collector health

Sample 3-5 existing files WITH telemetry in the same module to match the exact pattern.

#### 2. What to Instrument (by priority)

a. **Error paths** (highest priority)
   - Every `if err != nil` block representing an unexpected failure
   - Use `log.Printf("error context: %v", err)` following existing patterns
   - Include: error type, message, operation context

b. **Entry points and API boundaries**
   - HTTP handlers, gRPC endpoints, K8s informer callbacks
   - Track: operation name, duration, success/failure
   - See existing patterns in `otelcollector/main/main.go` for startup logging

c. **External calls** (outbound HTTP, K8s API, Azure SDK)
   - Track: target service, operation, duration, response status/error

d. **Critical business logic**
   - Config parsing results, scrape target discovery, metric processing milestones
   - Use `log.Printf` with structured context

e. **Startup and shutdown**
   - Component initialization success/failure
   - Signal handling and graceful shutdown (SIGTERM)

#### 3. Telemetry Conventions
- **Log format**: `log.Printf("component: operation: detail: %v", value)`
- **Error logging**: Always include the error: `log.Printf("failed to X: %v", err)`
- **Environment context**: Include relevant env vars (`CLUSTER`, `CONTROLLER_TYPE`, `MODE`) in startup logs
- **No sensitive data**: Never log Application Insights keys, Azure credentials, or scrape target credentials

#### 4. Anti-Patterns to Avoid
- Do NOT log sensitive data (Application Insights keys, tokens, credentials)
- Do NOT add telemetry inside tight scrape loops (generates excessive volume)
- Do NOT use `fmt.Println` for production telemetry — use `log` package
- Do NOT hardcode instrumentation keys — use env vars (`APPLICATIONINSIGHTS_AUTH`)
- Do NOT create new telemetry clients — use the existing shared utilities

### Files Typically Involved
- Go source files in `otelcollector/*/`
- Shared telemetry utilities in `otelcollector/shared/`
- Application Insights configuration in `otelcollector/scripts/`

### Validation
- Verify log statements match the existing format in neighboring code
- Run `go build` to ensure no compilation errors
- Run `go test ./...` to check test isolation
- Verify no secrets appear in log output
