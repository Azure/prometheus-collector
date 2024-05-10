## Managed Prometheus support for Sharding (In private preview)


### Overview
The Managed Prometheus Addon takes dependency on the  OSS [Opentelemetry Operator](https://github.com/open-telemetry/opentelemetry-operator) - [TargetAllocator(TA)](https://github.com/open-telemetry/opentelemetry-operator/tree/main/cmd/otel-allocator#target-allocator) component specifically.

The link below is the documentation on how TA handles sharding.

https://github.com/open-telemetry/opentelemetry-operator/tree/main/cmd/otel-allocator#even-distribution-of-prometheus-targets

As a part of the addon, we set the shards(collector instances) to a specific number based on our telemetry and usage or customer specific requests (not customer configurable today), in order to evenly distribute the target scraping to collector instances (ama-metrics replicas)

### Resource utilization
The sharding capability deploys multiple instances of the ama-metrics replicaset, which gets the targets allocated by the TA by querying the service exposing the targets per collector instance.
The default resource requests and limits for the ama-metrics replicaset is -
````
resources:
  limits:
    cpu: 7
    memory: 14Gi
  requests:
    cpu: 150m
    memory: 500Mi
````
When we shard to multiple instances, the limits and requests will be set to the same for all the instances, except we can expect a drop in the resource utilization since the scraping will be split by a collector instance.



