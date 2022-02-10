> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

[Prometheus metrics in MDM - FAQ](~/metrics/Prometheus/PromMDMFAQ.md)  
[Prometheus metrics in MDM - Getting help](https://teams.microsoft.com/l/channel/19%3a0ee871c52d1744b0883e2d07f2066df0%40thread.skype/Prometheus%2520metrics%2520in%2520MDM%2520(Limited%2520Preview)?groupId=5658f840-c680-4882-93be-7cc69578f94e&tenantId=72f988bf-86f1-41af-91ab-2d7cd011db47)

# Working with Prometheus metrics in MDM

This tutorial is an introduction to working with Prometheus based metrics in MDM.

Upon completion of this tutorial, you will have created a metrics account, deployed an agent to a Kubernetes cluster, configured it to scrape [Prometheus](https://prometheus.io/docs/introduction/overview/) metrics into your metrics account, and used [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) and [Grafana](https://grafana.com/grafana/) to query / visualize the metrics in dashboards.

![PromMDMgrafana](~/metrics/images/prometheus/PromMetricsMDMgrafana.png)  
  
Here are steps we will walk through.  

1. [Create metrics account and set up KeyVault authentication](~/metrics/Prometheus/PromMDMTutorial1Account.md)  
2. [Deploy agent to Kubernetes cluster for metrics collection](~/metrics/Prometheus/PromMDMTutorial2DeployAgentHELM.md)  
3. [Configure metrics collection](~/metrics/Prometheus/PromMDMTutorial3ConfigureCollection.md)  
4. [Set up Azure Grafana Service](~/metrics/Prometheus/PromMDMTutorial4SetUpGrafana.md)  
5. [Configure Prometheus data source](~/metrics/Prometheus/PromMDMTutorial5AddPromDataSource.md)  
6. [Use built-in dashboards](~/metrics/Prometheus/PromMDMTutorial6ReuseExistingDashboard.md)

## Prerequisites

Please ensure you have the following set up before continuing with this tutorial.

* Kubernetes: `>=1.16.0-0`  
* [Kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/)  
* [HELM client tool(v3.7.0 or later - see below note)](https://helm.sh/docs/intro/install/)  

    ```Note: Our charts will not work on HELM clients < 3.7.0```  
 
* Join the Private Preview of Azure Managed Workspaces for Grafana and deploy an instance. If you are not already part of the Private Preview, please send your Azure subscription ID to [this email](mailto:ad4g@microsoft.com) with a subject line Request to join private preview and see the full instructions below.

  [Set up Azure Grafana Service](~/metrics/Prometheus/PromMDMTutorial4SetUpGrafana.md)  


--------------------------------------

Lets start with [creating a metrics account](~/metrics/Prometheus/PromMDMTutorial1Account.md)  
