
<style>
h1 {
    color: deepskyblue;
}
</style>

# **azure-pipeline-aksdeploy.yml**

This "AksDeploy" pipeline deploys the AKS cluster, AMW workspace, DCR, DCE, DCRA, and Prometheus rule groups using the ci-cd-cluster ARM template found in the GitHub prometheus-collector repository.

## "AksDeploy" Pipeline Variables

+ **AZURESUBSCRIPTION**
+ **RESOURCE-GROUP**
+ **PARAMETERS**

A typical value for the "PARAMETERS" variable is as follows:

>{\"clusterName\":{\"value\":\"demoAksCluster\"},\"actionGroupId\":{\"value\":\"/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourceGroups/wtd-test/providers/Microsoft.Insights/actiongroups/wtdTestAg\"}}

If the '*azureMonitorWorkspaceLocation*' property is omitted, it defaults to the specified resource group's location.

You can explicitly set the '*azureMonitorWorkspaceLocation*' property as in the following example:

>{\"clusterName\":{\"value\":\"demoAksCluster\"},\"actionGroupId\":{\"value\":\"/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourceGroups/wtd-test/providers/Microsoft.Insights/actiongroups/wtdTestAg\"},\"azureMonitorWorkspaceLocation\":{\"value\":\"taiwannorth\"}}

# **azure-pipeline-regionstest.yml**

This "RegionsTest" pipeline runs the region tests on the specified cluster.

## "RegionsTest" Pipeline Variables

+ **AZURESUBSCRIPTION**
+ **RESOURCE-GROUP**
+ **CLUSTERNAME**

# **azure-pipeline-aksdeploy-test.yml**

The "Integrated" pipeline first deploys the resources and then runs the region tests.

## "Integrated" Pipeline Variables

+ **AZURESUBSCRIPTION**
+ **RESOURCE-GROUP**
+ **PARAMETERS**
+ **CLUSTERNAME**
+ **SLEEPTIME_IN_SECONDS**

# **Definition of Pipeline Variables**

+ **AZURESUBSCRIPTION** - The name of the subscription where resources are deployed.
+ **RESOURCE-GROUP** - The name of the resource group where resources are deployed
+ **PARAMETERS** - A Json object giving parameter values to override defaults in the ci-cd-cluster ARM template.
+ **CLUSTERNAME** - The name of the AKS cluster.
+ **SLEEPTIME_IN_SECONDS** - The time in seconds after deploying the cluster and AMW resoures to wait before running the tests. This should default to at least 2 hours.
