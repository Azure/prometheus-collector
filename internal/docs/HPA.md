## Managed Prometheus support for Horizontal Pod Autoscaling for Collector Replicaset(Preview)


### Overview
The Managed Prometheus Addon now supports Horizontal Pod Autoscaling(HPA) for the ama-metrics replicaset pod. This feature is currently in preview.
With this, the ama-metrics replicaset pod which handles the scraping of prometheus metrics with custom jobs can scale automatically based on the memory utilization. By default, the HPA is configured to support a minimum of 2 replicas (which is our global default) and a maximum of 12 replicas. The customers will also the have capability to set the shards to any number of minimum and maximum repliacas as long as they are within the range of 2 and 12.
WIth this, customers do not have to wait for the Managed Prometheus team to increase/decreaset the number of shards and the HPA automatically takes care of scaling based on the memory utlization of the ama-metrics pod to avoid OOMKills.
Currently the average utilization is set to 5Gi, with the goal of reducing the limits of the ama-metrics replicaset from 14Gi to 8Gi to have the replicasets be smaller in size instead of monolithic replicaset pods that grow vertically in memory usage.

The link below is the documentation on kubernetes support for HPA.

https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/

### HPA Configuration
Here's a link to the spec of the HPA object that will be deployed as a part of the managed prometheus addon.

[HPA Deployment Spec](../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-collector-hpa.yaml)

### Enablement
Currently this feature is in private preview and the Managed Prometheus team needs to enable it on the clusters. This is temporary as the feature is tested out on the currently sharded clusters and will soon be rolled out globally.

### Update Min and Max shards
In order to update the min and max shards on the HPA, you can edit the HPA object and it will not be reconciled as long as it is within the supported range of 2 and 12.

**Update Min replicas**
```bash
kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"minReplicas": 4}}'
```

**Update Max replicas**
```bash
kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"maxReplicas": 10}}'
```

**Update Min and Max replicas**
```bash
kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"minReplicas": 3, "maxReplicas": 10}}'
```

**or**

You could also edit the min and max replicas by doing a **kubectl edit** and updating the spec in the editor
```bash
kubectl edit hpa ama-metrics-hpa -n kube-system
```

### Update min and max shards to disable HPA scaling
HPA should be able to handle the load automatically for varying customer needs. But, it it doesnt fit the needs you can set min shards = max shards so that HPA doesnt scale the replicas based on the varying loads. 

Ex - If the customer wants to set the shards to 8 and not have the HPA update the shards, update the min and max shards to 8.

**Update Min and Max replicas**
```bash
kubectl patch hpa ama-metrics-hpa -n kube-system --type merge --patch '{"spec": {"minReplicas": 8, "maxReplicas": 8}}'
```


