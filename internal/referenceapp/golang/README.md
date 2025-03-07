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

### OTLP Data Quality Testing
The logic for each metric recorded is [here](main.go?plain=1#L309) with function `recordTestMetrics()`.

Each metric is emitted with cumulative and delta settings. Each metric will have a one time series with label `temporality`=`cumulative` and one with label `temporality`=`delta`.

| Metric Name               | Type     | Int/Float | Labels          | Values Emitted |
|---------------------------|----------|-----------|-----------------|----------------|
| `otlpapp.intcounter.total`| counter  | int       | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>` | Increment by 1 every 60s  |
| `otlpapp.floatcounter.total`| counter| float     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`| Increment by 1.5 every 60s   |
| `otlpapp.intgauge`        | gauge    | int       | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`       | Alternate between recording 1 and 2 every 60s     |
| `otlpapp.floatgauge`      | gauge    | float     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`       | Alternate between recording 1.5 and 2.5 every 60s   |
| `otlpapp.intupdowncounter`| updown counter | int     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`| Alternate between incrementing by 2 every 60s and decrementing by 1 every 60s  |
| `otlpapp.floatupdowncounter`| updown counter | float     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`| Alternate between incrementing by 2.5 every 60s and decrementing by 1.5 every 60s    |
| `otlpapp.intexponentialhistogram`      | exponential histogram  | int       | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`        | Alternate between recording 1 and 2 every 60s    |
| `otlpapp.floatexponentialhistogram`    | exponential histogram  | float     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`        | Alternate between recording 0.5 and 1.5 every 60s   |
| `otlpapp.intexplicithistogram`    | explicit histogram | int       | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`        | Alternate between recording 1 and 2 every 60s   |
| `otlpapp.floatexplicithistogram`  | explicit histogram | float     | `label.1`=`label.1-value`, `label.2`=`label.2-value`, `temporality`=`<cumulative/delta>`        | Alternate between recording 0.5 and 1.5 every 60s   |