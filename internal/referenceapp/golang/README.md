# Reference App

This app generates both Prometheus and OTLP metrics.

## Configurable Environment Variables

### Prometheus

| Variable          | Type  | Description                                           | Default |
|-------------------|-------|-------------------------------------------------------|---------|
| `SCRAPE_INTERVAL` | int   | How often in seconds the sample values should change  | `15`    |
| `RUN_PERF_TEST`   | bool  | If metrics set for perf testing should be generated   | `false` |
| `METRIC_COUNT`    | int   | How many metrics should be generated if `RUN_PERF_TEST` is `true` | `1000`  |

### OTLP

| Variable               | Type   | Description                           | Default       |
|------------------------|--------|---------------------------------------|---------------|
| `OTEL_TEMPORALITY`     | string | `cumulative` or `delta`               | `cumulative`  |
| `OTEL_CONSOLE_METRICS` | bool   | Print metrics to stdout instead of exporting   | `false`       |
| `OTEL_EXPORT_ENDPOINT` | string |  GRPC endpoint to export metrics to | `localhost:4317` |
| `OTEL_INTERVAL`        | int    | How often to export metrics in seconds        | `15`          |