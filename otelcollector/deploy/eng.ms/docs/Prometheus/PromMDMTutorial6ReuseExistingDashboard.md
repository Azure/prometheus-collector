> [!Note]
> Prometheus metrics in MDM is still in active development. It is only available for a very small set of customers to provide very early feedback - limited private preview. Geneva will open this up for broader preview, after we've had a chance to address feedback received in the current limited preview. If your team has not already been contacted for the limited preview, then you are not yet eligible for this preview. You can also join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly.

# Use built-in dashboards

## Default dashboards list

As part of this private preview you will have the following 19 dashboards available out of the box.  

1. K8s mix-ins
    * API-server
    * Cluster compute resources
    * Namespace compute resources(Pods)
    * Namespace compute resources(Workloads)
    * Node compute resources
    * Pod compute resources
    * Workload compute resources
    * Cluster network
    * Workload network
    * Namespace network(Pods)
    * Namespace network(Workloads)
    * Persistent Volume
2. Core-dns
3. Kubelet
4. kube-proxy
5. Node exporter(if installed)
    * Nodes
    * USE Method(Cluster)
    * USE Method(Node)

To see these

* Go to Manage dashboards  
![Manage](~/metrics/images/prometheus/AMGGrafanaPostSignIn.png)

* You will see a list like this 
![Manage](~/metrics/images/prometheus/AMGDashboards.png)  
  
These dashboards are already pre-installed in your managed Grafana instance. Also they will continue to be updated as we get feedback, so check in often for updates to dashboards at the [github source here](https://github.com/Azure/prometheus-collector/tree/main/otelcollector/deploy/dashboard).

> [!Note]
> To access this github repo you will need
  - [A GitHub account](https://docs.opensource.microsoft.com/tools/github/accounts/index.html)
  - [Link MS account to GitHub account](https://docs.opensource.microsoft.com/tools/github/accounts/linking.html)
  - [Join the 'Azure' organization on GitHub](https://docs.opensource.microsoft.com/tools/github/accounts/linking.html#join-organizations)

## Steps to use these dashboards  

To use the built-in dashboards  

* Click on one of dashboards listed (for e.g. Kubernetes / Compute Resources / Cluster)
![Manage](~/metrics/images/prometheus/AMGDashboards2.png)

* You can see a list of visualizations in this dashboards.  

> By default dashboards are configured to pick up the default data source and that is already set to Azure Monitor-Prometheus. You can also explicitly change it to Azure Monitor-Prometheus if you don't see data loaded.  

![Manage](~/metrics/images/prometheus/AMGDashboards3.png)

* You can now filter, view query and modify dashboard just like any Grafana instance.  

* Read on below to see how we can modify the dashboard.  

## Modify dashboards via Grafana

As part of Azure Grafana Service, you will get in-built dashboards.You can query using [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/). Here are a few few limitations that you need be aware of, in case you modify the grafana dashboard

1. Query durations > 14d are blocked.  
2. Grafana Template functions  
   * `label_values(my_label)` not supported due to cost of the query on MDM storage. Use `label_values(my_metric,my_label)` instead.  
3. Case-sensitivity  
   * Due to limitations on MDM store(being case in-sensitive), query will do the following.  
   * Any specific casing specified in the query for labels & values(non-regex), will be honored by the query service(meaning results returned will have the same casing).  
   * For labels and values not specified in the query(including regex based value matchers), query service will return results all in lower case.  

## Create new dashboards

In addition to reusing existing dashboard, you can also create new [dashboards in Grafana](https://grafana.com/docs/grafana/latest/dashboards/).  

--------------------------------------

**Tutorial Summary**
In this tutorial you learned how to create a metrics account, deploy an agent to a Kubernetes cluster, configure it to scrape [Prometheus](https://prometheus.io/docs/introduction/overview/) metrics into your metrics account, use [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) and [Grafana](https://grafana.com/grafana/) to query / visualize the metrics in dashboards.
