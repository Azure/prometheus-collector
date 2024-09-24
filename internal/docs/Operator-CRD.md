## Managed Prometheus support for CRD

### Use Prometheus Pod and Service Monitor Custom Resources
The Azure Monitor metrics add-on supports scraping Prometheus metrics using Prometheus - Pod Monitors and Service Monitors, similar to the OSS Prometheus operator. Enabling the add-on will deploy the Pod and Service Monitor custom resource definitions to allow you to create your own custom resources.
Creating these custom resources allows for easy configuration of scrape jobs in any namespace, especially useful in the multi tenancy scenario, where partners don’t have access to the kube-system namespace and one error prone scrape job might affect the other partners scrape jobs. This is also true without the multitenancy model. Currently, when there is an erroneous scrape job in the custom config map, all the jobs in custom config map are ignored and the addon uses just the default scrape jobs. For reference, see [create custom scrape config with ConfigMap](https://learn.microsoft.com/azure/azure-monitor/containers/prometheus-metrics-scrape-validate#create-prometheus-configuration-file).

This document illustrates the steps need to setup custom resources (pod monitors and service monitors) with Azure Managed Prometheus to setup scrape jobs for the workloads running in your AKS clusters. 

### Pre-requisites
1.	Azure Monitor Workspace is configured and receiving Azure Managed Prometheus metrics.
2.	The workload that you want to scrape metrics from is deployed and running on the AKS cluster.

### Enable Azure Managed Prometheus with Operator/CRD support
Once your cluster has Managed Prometheus enabled, Azure Monitor metrics add-on and will automatically install the custom resource definition (CRD) for pod and service monitors. The add-on will use the same custom resource definition (CRD) for pod and service monitors as open-source Prometheus, except for a change in the group name and API version. If you have existing Prometheus CRDs and custom resources on your cluster, these will not conflict with the CRDs created by the add-on.
At the same time, the CRDs created for the OSS Prometheus will not be picked up by the managed Prometheus addon. This is intentional for the purposes of isolation of scrape jobs.

### Create a Pod or Service Monitor
Use the [Pod and Service Monitor templates](https://github.com/Azure/prometheus-collector/tree/main/otelcollector/customresources) and follow the API specification to create your custom resources. **Note** that if you are using existing pod/service monitors with Prometheus, you can simply change the API version to **azmonitoring.coreos.com/v1** for Managed Prometheus to start scraping metrics.
Your pod and service monitors should look like the examples below:

#### Example Service Monitor - 
```yaml
# Note the API version is azmonitoring.coreos.com/v1 instead of monitoring.coreos.com/v1
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor

# Can be deployed in any namespace
metadata:
  name: reference-app
  namespace: app-namespace
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023

  # The selector filters endpoints by service labels.
  selector:
    matchLabels:
      app: reference-app

  # Multiple endpoints can be specified. Port requires a named port.
  endpoints:
  - port: metrics
```

#### Example Pod Monitor -

```yaml
# Note the API version is azmonitoring.coreos.com/v1 instead of monitoring.coreos.com/v1
apiVersion: azmonitoring.coreos.com/v1
kind: PodMonitor

# Can be deployed in any namespace
metadata:
  name: reference-app
  namespace: app-namespace
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023

  # The selector specifies which pods to filter for
  selector:

    # Filter by pod labels
    matchLabels:
      environment: test
    matchExpressions:
      - key: app
        operator: In
        values: [app-frontend, app-backend]

    # [Optional] Filter by pod namespace
    namespaceSelector:
      matchNames: [app-frontend, app-backend]

  # [Optional] Labels on the pod with these keys will be added as labels to each metric scraped
  podTargetLabels: [app, region, environment]

  # Multiple pod endpoints can be specified. Port requires a named port.
  podMetricsEndpoints:
    - port: metrics
```

### Deploy a Pod or Service Monitor

You can deploy the pod or service monitor the same way as any other Kubernetes resource. Save your pod or service monitor as a yaml file and run:
```bash
kubectl apply -f <file name>.yaml
```
Any validation errors will be shown after running the command.

#### Verify Deployment
```bash
kubectl get podmonitors --all-namespaces
kubectl get servicemonitors --all-namespaces
kubectl describe podmonitor <pod monitor name> -n <pod monitor namespace>
kubectl describe servicemonitor <service monitor name> -n <service monitor namespace>
```

Verify that the API version is `azmonitoring.coreos.com/v1`.
Verify in the logs that the custom resource was detected:

```bash
kubectl get pods -n kube-system | grep ama-metrics-operator-targets
kubectl logs -n kube-system <ama-metrics- operator-targets name> -c targetallocator
```


The log output should have a similar message indicating the deployment was successful:


```bash
{"level":"info","ts":"2023-08-30T17:51:06Z","logger":"allocator","msg":"ScrapeConfig hash is different, updating new scrapeconfig detected for ","source":"EventSourcePrometheusCR"}
```

#### Delete a Pod or Service Monitor
You can delete a pod or service monitor the same way as any other Kubernetes resource. 

kubectl delete podmonitor <podmonitor name> -n <podmonitor namespace>
kubectl delete servicemonitor <servicemonitor name> -n <servicemonitor namespace>

