# Performance Testing

## E2E Scale and Perf Test Instructions

### Benchmark Perf Testing

On your own smaller cluster, you can use the reference app running in perf test mode to quickly check if there is any change in the CPU or memory usage between agents.

1. Create a new nodepool and 2 nodes with a large enough SKU so that two agent's CPU and memory limits can be raised to at least 6 cores and 10GiB.

2. Deploy two helm releases with different release names, one with the old agent and one with the changes of the new agent.

3. To make changes to the reference app, see the section below for how to build and deploy. Otherwise you can use the existing yaml and image.

4. Deploy the reference app with these settings in the yaml:

    ```yaml
    env:
      - name: RUN_PERF_TEST
        value: "true"
      - name: SCRAPE_INTERVAL
        value: "15"
      - name: METRIC_COUNT
        value: "62500"
    ```

    With these settings, the app generates `62,500 metrics`, each with `8 timeseries`, every `15 seconds`, for a total of `2,000,000 samples` per minute.

5. To scrape the metrics from the app, deploy the custom Prometheus config as: 

    ```yaml
    scrape_configs:
    - job_name: prometheus_ref_app
      scheme: http
      scrape_interval: 15s
      kubernetes_sd_configs:
      - role: pod
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: "prometheus-reference-app"
    ```

    The `scrape_interval` value should match the `SCRAPE_INTERVAL` environment variable (in seconds) that is set for the reference app.

6. See the `Viewing the Results` section for where to view the performance and metric volume of each.

### Scale Testing

Compared to benchmark testing with just the reference app having a high load of metrics, the scale cluster with the default scrape targets enabled gives a more realistic environment of discovering, scraping, and sending metrics of varrying types but requires some more setup. You can directly compare the previous agent and the one you are testing by running them simultaneously in the same environment of pods and nodes.

#### Comparing Performance

1. Deploy two helm releases with different release names, one with the old agent and one with the changes of the new agent.

2. Setup the number of nodes and pods. See how many are currently deployed by running:
    * Check the total number of nodes: `kubectl get nodes | wc -l`
    * Check the total number of pods: `kubectl get pods --all-namespaces | wc -l`

   Then scale up or down the nodepools to reach the desired number of nodes and change the number of replicas in the nginx deployments to get the desired number of pods.

   You can use the existing perf documented and the scale for that as a starting point.

3. Compare the performance between the old and new agent for that number of nodes and pods. Repeat for any other combination.

#### Finding the Max Volume

The other necessary test is if the maximum volume of metrics that the agent can handle has not regressed. Follow the same steps as above for `Comparing Performance` but increase the number of nodes and pods until either
1. `timeseries_published` metric is consistently less than the `timeseries_received` metric and the agent is restarting due to getting `OOM-killed`.
2. or ME is logging that the number of metrics is above the limit set in the me.config file and is dropping metrics

### Viewing the Results

* The `Scale Testing` dashboard in our CI monitoring grafana instance has all of the information below in one dashboard. It can be filtered down to one cluster, with two agents chosen to compare

* The overall CPU and memory usage for the pod can be viewed in the `Azure Monitor Container Insights /Kubernetes / Compute Resources / Pod` out-of-the-box Grafana dashboard.

* Use the `Prometheus-Collector Health` dashboard to view the number of metrics and bytes that are received and published by ME.

* The process-level CPU and memory usage for the OpenTelemetry Collector and Metrics Extension can be viewed in our `Telemetry` dashboard.

* OOM-kills and exporting failures can also be viewed in our `Telemetry` dashboard.

* For Windows perf, the following queries can be used by replacing `<container_id>` with the ID of the windows agent container:

    ```
    rate(windows_container_cpu_usage_seconds_total{container_id="<container_id>"}[5m])
    ```

    ```
    windows_container_memory_usage_private_working_set_bytes{container_id="<container_id"} / 1000000000
    ```

### Building the Reference App

Note: This step is only necessary if making changes to the app. Otherwise, the yamls below will have the latest image.

To build the reference app, go to the directory `cd otelcollector/referenceapp/<golang or python>` and run `docker build -f ./<linux or windows>/Dockerfile -t <your image tag> .` depending on which OS you want to build.

### Deploy the Reference App with Perf Mode Enabled

Deploy the [linux reference app](../../referenceapp/prometheus-reference-app.yaml) or the [windows reference app](../../referenceapp/win-prometheus-reference-app.yaml) with `RUN_PERF_TEST` set to `true` to generate the specified number of metrics at a specified interval. Specify how many replicas should be scraped in the yaml spec. In the environment variables, set `SCRAPE_INTERVAL` to be an integer in seconds of how often the metrics should be generated. Set `METRIC_COUNT` to be the number of OTLP metrics to generate. Note that OTLP counts metrics by name. Multiply this by the number of timeseries of that metric to get the total number of timeseries that will be generated. For example, the reference app has 8 timeseries for the metric `myapp_temperature`. If we want 2,000,000 of these metrics to be generated every 15 seconds, the environment variables set in the yaml would be:

```yaml
env:
  - name: RUN_PERF_TEST
    value: "true"
  - name: SCRAPE_INTERVAL
    value: "15"
  - name: METRIC_COUNT
    value: "62500"
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