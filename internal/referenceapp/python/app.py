# Example code snippet from https://github.com/prometheus/client_python

from prometheus_client import start_http_server, Summary
import random
import time
import os
from opentelemetry import metrics
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import (
    ConsoleMetricExporter,
    PeriodicExportingMetricReader,
)
from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter

# Create a metric to track time spent and requests made.
REQUEST_TIME = Summary('request_processing_seconds', 'Time spent processing request')

# Decorate function with metric.
@REQUEST_TIME.time()
def process_request(t):
    """A dummy function that takes some time."""
    work_counter.add(1, {"work.type": "label.value"})
    time.sleep(t)

if __name__ == '__main__':
    # Get the endpoint from environment variable or use a default
    endpoint = os.environ.get("OTEL_ENDPOINT", "http://localhost:4318")
    metric_reader = PeriodicExportingMetricReader(OTLPMetricExporter(endpoint=f"{endpoint}/v1/metrics"))
    provider = MeterProvider(metric_readers=[metric_reader])

    # Sets the global default meter provider
    metrics.set_meter_provider(provider)

    # Creates a meter from the global meter provider
    meter = metrics.get_meter("my.meter.name")

    # Define work_counter as global so it can be used in process_request
    global work_counter
    work_counter = meter.create_counter(
        "work.counter", unit="1", description="Counts the amount of work done"
    )
    # Start up the server to expose the metrics.
    start_http_server(2114)
    # Generate some requests.
    while True:
        process_request(random.random())