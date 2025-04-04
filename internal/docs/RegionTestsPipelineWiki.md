
<style>
h1 {
    color: deepskyblue;
}
</style>

# **azure-pipeline-aksdeploy.yml**

This "AksDeploy" pipeline deploys the AKS cluster, AMW workspace, DCR, DCE, DCRA, and Prometheus rule groups using the ci-cd-cluster ARM template found in the GitHub prometheus-collector repository.

## "AksDeploy" Pipeline Variables

+ **AZURESUBSCRIPTION**
+ **CLUSTERNAME**
+ **RESOURCE-GROUP**
+ **ACTION-GROUP-ID**
+ **DEFAULT-PARAMETERS**

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
+ **CLUSTERNAME**
+ **RESOURCE-GROUP**
+ **ACTION-GROUP-ID**
+ **DEFAULT-PARAMETERS**
+ **SLEEPTIME_IN_SECONDS**

# **Definition of Pipeline Variables**

+ **AZURESUBSCRIPTION** - The name of the subscription where resources are deployed.
+ **RESOURCE-GROUP** - The name of the resource group where resources are deployed.
+ **CLUSTERNAME** - The base name of the AKS cluster and AMW-related resources created.
+ **ACTION-GROUP-ID** - The resource id of the action group referenced in the prometheus rules created.
+ **DEFAULT-PARAMETERS** - A Json object giving parameter values to override defaults in the ci-cd-cluster ARM template. These values will be replaced with specific YAML pipeline variables if they are supplied.
+ **SLEEPTIME_IN_SECONDS** - The time in seconds after deploying the cluster and AMW resoures to wait before running the tests. This should default to at least 2 hours.

A typical value for **DEFAULT-PARAMETERS** is as follows:

  >{"clusterName":{"value":"aksCluster"},"actionGroupId":{"value":"/subscriptions/b9842c7c-1a38-4385-8f39-a51314758bcf/resourceGroups/wtd-test/providers/Microsoft.Insights/actiongroups/wtdTestAg"}}

  Values in this Json object will be replaced with the values for the following Pipeline variables if they are supplied:

+ **CLUSTERNAME** (clusterName)
+ **ACTION-GROUP-ID** (actionGroupId)
