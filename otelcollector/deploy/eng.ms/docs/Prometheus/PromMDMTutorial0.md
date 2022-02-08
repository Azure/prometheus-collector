> [!Note]
> Prometheus metrics in MDM is still in active development and is offered as a Private Preview. You can join the [K8s Observability Updates](https://idwebelements/GroupManagement.aspx?Group=K8sObsUpdates&Operation=join) alias for updates on this feature, including when this will roll out more broadly in Public Preview.

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
7. [Windows Kubernetes metrics](~/metrics/Prometheus/windows.md)

## Prerequisites

Please ensure you have the following set up before continuing with this tutorial.

* Kubernetes: `>=1.16.0-0`  
* [Kubectl client tool](https://kubernetes.io/docs/tasks/tools/install-kubectl-windows/)  
* [HELM client tool(v3.7.0 or later - see below note)](https://helm.sh/docs/intro/install/)  

    ```Note: Our charts will not work on HELM clients < 3.7.0```  
 
* Access to a dogfood subscription of Azure Grafana Service. If you do not yet have one, [please request one via](mailto:ad4g@microsoft.com)  
* Permission to set up data sources and dashboards in Azure Grafana. To do this [join the Azure Dashboard for Grafana Dogfood (Admin) group](https://idweb/identitymanagement/aspx/groups/MyGroups.aspx?popupFromClipboard=%2Fidentitymanagement%2Faspx%2FGroups%2FEditGroup.aspx%3Fid%3Daa23b20a-f5ef-485d-94bd-468bbf2346fb) via IDWeb.

--------------------------------------

Lets start with [creating a metrics account](~/metrics/Prometheus/PromMDMTutorial1Account.md)  
