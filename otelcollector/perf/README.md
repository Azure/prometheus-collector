# OpenTelemetry Collector Performance Test Instructions

The testbed and test code are in the [OpenTelemetry Collector repo](https://github.com/open-telemetry/opentelemetry-collector/tree/main/testbed). Prometheus perf tests are in the file [testbed/tests/metric_test.go](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/tests/metric_test.go#L125) (currently a forked branch).

The default settings for the Prometheus perf test is a 15s scrape interval and 150,000 active time series to scrape from a single static instance.

The test can be configured with custom settings through [environment variables](https://github.com/gracewehner/opentelemetry-collector/blob/gracewehner-otel/prometheus-receiver-perf/testbed/README.md#environment-variables). The default test duration is 15s, so be sure to set `TESTCASE_DURATION` to be at least `1m` for the default settings above. The scrape interval and the number of timeseries in that interval can be configured by setting `SCRAPE_INTERVAL` and `ITEMS_PER_INTERVAL`.

The test infrastructure sets up a [load generator](https://github.com/gracewehner/opentelemetry-collector/blob/gracewehner-otel/prometheus-receiver-perf/testbed/testbed/load_generator.go) which generates the specified amount of [OTLP metrics](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/data_providers.go#L118). These metrics are [converted to Prometheus format and exported to an endpoint through the Prometheus exporter](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/senders.go#L587). The collector is run as a child process with the Prometheus receiver and OTLP exporter and the performance is recorded with the pprof golang package. The test infrastructure also has a [mock backend](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/mock_backend.go#L155) that receives the metrics the OTLP exporter is sending. The [validator](https://github.com/gracewehner/opentelemetry-collector/blob/e955cbe9677337d9292c0b6894d00e08a1150438/testbed/testbed/validator.go#L44) makes sure the number of metrics received is correct and the timestamps of the metrics are the scrape interval.


## Running the Tests
The tests can be run for any collector build. The executable must be called `otelcol_{{.GOOS}}_{{.GOARCH}}` (ex: `otelcol_linux_amd`) and in the path `../../bin/otelcol_{{.GOOS}}_{{.GOARCH}}` relative to /testbed/tests. Set your environment variables, `cd` to `testbed/tests` and run `./runtests.sh` for all tests.

Run the specific test with `go test -v -run ^(TestMain|TestMetrics10kDPSScraped)`. `TestMain` is needed to generate the perf results document.

To get the test executable to run the test on a container, add the arguments `-c -o <executable name>` to the test. Kubectl cp to the container and run. The collector executable must also be at the location `../../bin/otelcol_{{.GOOS}}_{{.GOARCH}}`. 

## Results
The stdout of each test will print the CPU and RAM and items sent and received every 3 seconds while running. At the end, the results of the validator are printed.

`TestMain` will give a chart with an entry for each test with info for if it passed, the duration, number of items, and average and max CPU % and RAM in `testbed/tests/results/TESTRESULTS.md`. Collector logs can be found in `testbed/tests/results/<TestName>/agent.log`