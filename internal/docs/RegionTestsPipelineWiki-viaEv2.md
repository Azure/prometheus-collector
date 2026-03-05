
<style>
h1 {
    color: deepskyblue;
}
</style>

# **Testing new regions via Ev2 deployments**

This is version 2 of a solution to automate testing of new regions for Prometheus Collector. This solution uses ADO YAML pipelines and Ev2 rollouts to deploy an AKS cluster, AMW workspace, DCR, DCE, DCRA, and Prometheus rule groups using ARM templates found in the Enterprise GitHub prometheus-collector repository. It then runs a PowerShell script on the cluster that executes a compiled version of the Ginkgo regionTests test suite and archives the test output to a storage account.

## **Prerequisites**

1. Register the service using [New-AzureServiceRolloutServiceRegistration](https://eng.ms/docs/products/ev2/references/cmdlets/new-service):

```powershell
New-AzureServiceRolloutServiceRegistration
    -ServiceSpecificationPath <path_to_ServiceSpec.json>
    -RolloutInfra ...
```

2. Register subscriptionKeys for the following 2 service groups using [Register-AzureServiceSubscription](https://eng.ms/docs/products/ev2/references/cmdlets/register-subscription):

    - Microsoft.Azure.PrometheusCollector.GinkgoE2E.Tests.Infra
    - Microsoft.Azure.PrometheusCollector.GinkgoE2E.Tests

```powershell
Register-AzureServiceSubscription
    -ServiceIdentifier ...
    -SubscriptionId ...
    -SubscriptionKey ...
    -ServiceGroup <each_of_the_listed_service_groups>
    -RolloutInfra ...
```

3. Assign permissions to the **PrometheusCollector-Tests-DevOps** App (public regions only)

The App PrometheusCollector-Tests-DevOps is used for public deployments. AGRM (SRM) is used for sovereign/air-gapped deployments.
PrometheusCollector-Tests-DevOps needs Contributor and UserAccessAdministrator rights on a subscription in public where testing is being performed.

Within your ADO project where the pipelines will run, create an Ev2 service connection for the PrometheusCollector-Tests-DevOps service principal. The App in Entra must have federated credentials to this service connection via the service connection's Issuer and Subject Identifier.

In the release pipeline's YAML file, add "PrometheusCollector-Tests-DevOps" as the ConnectedServiceName in the release job's input. See PromCollector-GinkgoTests-Test-Release.yml for an example.

## User Assigned Managed Identity

Ev2 runs the GinkGo tests in a shell extension as the user-assigned managed identity **prom-test-msi**. For this to work, prom-test-msi needs the following permissions on the target resource group:

- Contributor
- NetworkContributor
- ManagedIdentityOperator
- StorageBlobDataOwner

This MSI and its role assignments are automatically provisioned during the buildout phase.

## Pipelines

There are two kinds of pipelines: "Buildout" and "Release".

A "Buildout" pipeline provisions the user-assigned managed identity prom-test-msi in the resource group **prom-test** and all the AKS-related resources (i.e., AKS cluster, AMW workspace, DCR, DCE, DCRA, and Prometheus rule groups) needed by Prometheus Collector in the resource group **prom-test-`<region`>**.

A "Release" pipeline runs the GinkGo tests and archives the output to a file in the **shellextlogs** container in the storage account **stshellextlogs`<region`>**.

### Build pipelines

- PromCollector-GinkgoTests-BuildArtifacts-1ES-Official
  - Parameters: none
  - Output artifact: **drop-prometheus-collector**
  - Build image (see [1ESHub](https://aka.ms/1eshub))

```YAML
pool:   ## https://aka.ms/1eshub
    name: 1ESPieminiBuildLarge
    image: 1ESPTGen2Large-ipie-prometheuscollector
    os: windows
```

- PromCollector-GinkgoTests-Signing-OneBranch-Official
  - Parameters:
    - buildVersion
    - cvrpPath (default: ".\\cvrp.manifest.json")
    - artifactToDeploy (default: "drop-prometheus-collector")
    - debugMode (bool)
  - Output artifact: **drop-prometheus-collector-signed**

### Release pipelines

- Public/Test
  - PromCollector-GinkgoTests-Test-Buildout
  - PromCollector-GinkgoTests-Test-Release
- USNat
  - PromCollector-GinkgoTests-USNat-Buildout
  - PromCollector-GinkgoTests-USNat-Release

#### Parameters

- buildVersion - build version containing the artifacts to deploy
- cvrpPath (default: ".\\cvrp.manifest.json")
- artifactToDeploy (default: "drop-prometheus-collector-signed")
- deployToRegions
