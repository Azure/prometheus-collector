### **This wiki contains links of all the resources related to alerts and dashboards of the CI CD and prod monitoring near ring clusters**

Below is the linking of the AKS cluster to Azure Monitor Workspace to Grafana for cicd and prod monitoring clusters:

ci/cd clusters (cluster --> amw --> grafana)
============================================

[AKS]
dev=[ci-dev-aks-mac-eus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-aks-mac-eus-rg/providers/Microsoft.ContainerService/managedClusters/ci-dev-aks-mac-eus/overview) --> [ci-dev-aks-eus-mac](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-aks-mac-eus-rg/providers/microsoft.monitor/accounts/ci-dev-aks-eus-mac/resourceOverviewId) --> [cicd-graf-metrics-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.Dashboard/grafana/cicd-graf-metrics-wcus/overview)

prod=[ci-prod-aks-mac-weu](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-aks-mac-weu-rg/providers/Microsoft.ContainerService/managedClusters/ci-prod-aks-mac-weu/overview) --> [ci-prod-aks-weu-mac](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-aks-mac-weu-rg/providers/microsoft.monitor/accounts/ci-prod-aks-weu-mac/resourceOverviewId) --> [cicd-graf-metrics-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.Dashboard/grafana/cicd-graf-metrics-wcus/overview)

[ARC]
dev=[ci-dev-arc-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-dev-arc-wcus/overview) --> [ci-dev-arc-amw](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/microsoft.monitor/accounts/ci-dev-arc-amw/resourceOverviewId) --> [cicd-graf-metrics-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.Dashboard/grafana/cicd-graf-metrics-wcus/overview)

prod=[ci-prod-arc-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-prod-arc-wcus/overview)--> [ci-prod-arc-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/microsoft.monitor/accounts/ci-prod-arc-wcus/resourceOverviewId) --> [cicd-graf-metrics-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.Dashboard/grafana/cicd-graf-metrics-wcus/overview)

canary/prod monitoring clusters (cluster --> amw -->grafana)
===========================================================

[monitoring-metrics-prod-aks-eus2euap](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-eus2euap/overview) --> [monitoring-metrics-amw-eus2euap](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-amw/providers/microsoft.monitor/accounts/monitoring-metrics-amw-eus2euap/resourceOverviewId) --> [monitoring-grafana-metrics-westus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/microsoft.dashboard/grafana/mon-graf-metric-westus/overview)
[monitoring-metrics-prod-aks-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-wcus/overview) --> [monitoring-metrics-amw-wcus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-amw/providers/microsoft.monitor/accounts/monitoring-metrics-amw-wcus/resourceOverviewId) --> [monitoring-grafana-metrics-westus](https://ms.portal.azure.com/#@microsoft.onmicrosoft.com/resource/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/microsoft.dashboard/grafana/mon-graf-metric-westus/overview)


**Dashboard for CI CD and prod monitoring clusters**

* CICD - [link](https://cicd-graf-metrics-wcus-dkechtfecuadeuaw.wcus.grafana.azure.com/d/gp9556IVy/cpu-and-memory-utilization-k-s-m-replicaset-and-daemonset?orgId=1)

* Prod near ring - [link](https://mon-graf-metric-westus-f5hvdcaxc3hjdcdm.wus.grafana.azure.com/d/gp9556IVy/cpu-and-memory-utilization-k-s-m-replicaset-and-daemonset?orgId=1)
