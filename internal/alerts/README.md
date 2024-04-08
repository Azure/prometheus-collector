Here are the cluster  parameters which need to be updated in example-alert-template.json before deploying the alerts templates for each cluster. Please
update the "scopes" field in the alerts template with the cluster id and AMW id from the list below depending on the cluster.
Update the clusterName field with the cluster name below. Update the location according to the cluster. Update the alert name accordingly.

Cluster name                               Cluster id


ci-dev-aks-mac-eus                         /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-aks-mac-eus-rg/providers/Microsoft.ContainerService/managedClusters/ci-dev-aks-mac-eus
ci-dev-arc-wcus                            /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-dev-arc-wcus
ci-prod-aks-mac-weu                        /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-aks-mac-weu-rg/providers/Microsoft.ContainerService/managedClusters/ci-prod-aks-mac-weu
ci-prod-arc-wcus                           /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/Microsoft.ContainerService/managedClusters/ci-prod-arc-wcus
monitoring-metrics-prod-aks-eus2euap       /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-eus2euap
monitoring-metrics-prod-aks-wcus           /subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-prod-aks/providers/Microsoft.ContainerService/managedClusters/monitoring-metrics-prod-aks-wcus


Azure Monitor Workspace                                                                                                                                              Location
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-aks-mac-eus-rg/providers/microsoft.monitor/accounts/ci-dev-aks-eus-mac                     eastus
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-dev-arc-wcus/providers/microsoft.monitor/accounts/ci-dev-arc-amw                               westcentralus
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-aks-mac-weu-rg/providers/Microsoft.Monitor/accounts/ci-prod-aks-weu-mac                   westeurope
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/ci-prod-arc-wcus/providers/microsoft.monitor/accounts/ci-prod-arc-wcus                            westcentralus
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-amw/providers/microsoft.monitor/accounts/monitoring-metrics-amw-eus2euap       eastus2euap
/subscriptions/9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb/resourceGroups/monitoring-metrics-amw/providers/microsoft.monitor/accounts/monitoring-metrics-amw-wcus           westcentralus


