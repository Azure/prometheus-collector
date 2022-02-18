# Performance Testing

## E2E Performance Test Instructions

### Deploy the Prometheus Collectors

Run two deployments of the prometheus-collector. One with all the default scrape configs enabled and the other with all default scrape configs disabled and just the reference app as the target. The first will observe the second, which will be scraping the heavy load.

You can find the linux scrape config [here]((../../referenceapp/linux-scrape-config.yaml)) and the windows scrape config [here](../../referenceapp/windows-scrape-config.yaml)

### Deploy the Reference App
Deploy the [linux reference app](../../referenceapp/prometheus-reference-app.yaml) or the [windows reference app](../../referenceapp/win-prometheus-reference-app.yaml) with `RUN_PERF_TEST` set to `true` to generate the specified number of metrics at a specified interval. Specify how many replicas should be scraped in the yaml spec. In the environment variables, set `SCRAPE_INTERVAL` to be an integer in seconds of how often the metrics should be generated. Set `METRIC_COUNT` to be the number of OTLP metrics to generate. Note that OTLP counts metrics by name. Multiply this by the number of timeseries of that metric to get the total number of timeseries that will be generated. For example, the reference app has 8 timeseries for the metric `myapp_temperature`. If we want 1,000,000 of these metrics to be generated every 15 seconds, the environment variables set in the yaml would be:

```
env:
  - name: RUN_PERF_TEST
    value: "true"
  - name: SCRAPE_INTERVAL
    value: "15"
  - name: METRIC_COUNT
    value: "125000"
```

### View the Performance Results

* Use the out-of-the-box dashboards in Grafana to view the cpu, memory, and disk metrics (sent from the other deployment) for the deployment scraping the large load.

* For Windows perf, the following queries can be used by replacing `<container_id>` with the ID of the windows agent container:

    ```
    rate(windows_container_cpu_usage_seconds_total{container_id="<container_id>"}[5m])
    ```

    ```
    windows_container_memory_usage_private_working_set_bytes{container_id="<container_id"} / 1000000000
    ```

* Use the `Prometheus-Collector Health` dashboard to view the number of metrics being received, processed, and dropped by ME.

* Use our telemetry to view the cpu and memory usage of the OpenTelemetry Collector and ME individually:

    ```
    customMetrics
    | where customDimensions.cluster == "<cluster-name>"
    | where name contains 'cpu'
    | render timechart
    ```

    ```
    customMetrics
    | where customDimensions.cluster == "<cluster-name>"
    | where name contains 'memory'
    | render timechart
    ```

## OpenTelemetry Collector Performance Test Instructions

The testbed and test code are in the [OpenTelemetry Collector repo](https://github.com/open-telemetry/opentelemetry-collector/tree/main/testbed). Prometheus perf tests are in the file [testbed/tests/metric_test.go](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/tests/metric_test.go#L125) (currently a forked branch).

The default settings for the Prometheus perf test is a 15s scrape interval and 150,000 active time series to scrape from a single static instance.

The test can be configured with custom settings through [environment variables](https://github.com/gracewehner/opentelemetry-collector/blob/gracewehner-otel/prometheus-receiver-perf/testbed/README.md#environment-variables). The default test duration is 15s, so be sure to set `TESTCASE_DURATION` to be at least `1m` for the default settings above. The scrape interval and the number of timeseries in that interval can be configured by setting `SCRAPE_INTERVAL` and `ITEMS_PER_INTERVAL`.

The test infrastructure sets up a [load generator](https://github.com/gracewehner/opentelemetry-collector/blob/gracewehner-otel/prometheus-receiver-perf/testbed/testbed/load_generator.go) which generates the specified amount of [OTLP metrics](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/data_providers.go#L118). These metrics are [converted to Prometheus format and exported to an endpoint through the Prometheus exporter](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/senders.go#L587). The collector is run as a child process with the Prometheus receiver and OTLP exporter and the performance is recorded with the pprof golang package. The test infrastructure also has a [mock backend](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/mock_backend.go#L155) that receives the metrics the OTLP exporter is sending. The [validator](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/validator.go#L44) makes sure the number of metrics received is correct and the timestamps of the metrics are the scrape interval.


### Running the Tests
The tests can be run for any collector build. The executable must be called `otelcol_{{.GOOS}}_{{.GOARCH}}` (ex: `otelcol_linux_amd`) and in the path `../../bin/otelcol_{{.GOOS}}_{{.GOARCH}}` relative to /testbed/tests. Set your environment variables, `cd` to `testbed/tests` and run `./runtests.sh` for all tests.

Run the specific test with `go test -v -run ^(TestMain|TestMetrics10kDPSScraped)`. `TestMain` is needed to generate the perf results document.

To get the test executable to run the test on a container, add the arguments `-c -o <executable name>` to the test. Kubectl cp to the container and run. The collector executable must also be at the location `../../bin/otelcol_{{.GOOS}}_{{.GOARCH}}`. 

### Results
The stdout of each test will print the CPU and RAM and items sent and received every 3 seconds while running. At the end, the results of the validator are printed.

`TestMain` will give a chart with an entry for each test with info for if it passed, the duration, number of items, and average and max CPU % and RAM in `testbed/tests/results/TESTRESULTS.md`. Collector logs can be found in `testbed/tests/results/<TestName>/agent.log`