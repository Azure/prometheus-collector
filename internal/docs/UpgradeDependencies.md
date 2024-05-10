This document outlines briefly how to upgrade KSM,Node Exporter, Telegraf and other components.

Example PR: https://github.com/Azure/prometheus-collector/pull/418

Updating KSM and NE charts
1. For updating the charts for KSM and NE, replace the respective folders from here respectively under dependentcharts, [KSM](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics) and [NE](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-node-exporter).
2. Update the Version and Appversion in all the yaml files for both 1p and addon charts from the example PR above.
3. Update both the images in our pipeline build file : azure-pipeline-build.yml
4. Before creating a pull request in the main repository for both KSM and NE, make sure to run the "build and push dependent helm chart pipeline". You can find the pipeline [here](https://github.com/Azure/prometheus-collector/actions/workflows/build-and-push-dependent-helm-charts.yml).
5. The current service principal responsible for controlling the Azure Container Registry (ACR) pull and push on the containerinsightsprod registry can be accessed [here](https://ms.portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationMenuBlade/~/Credentials/appId/c58817c2-f216-4308-bb6c-126e0d82b824). If necessary, you can renew the password for the service principal from that page. The current password's expiry date is 3/13/2024.
6. If you renew the password, remember to update the following two secrets in the [GitHub Actions settings](https://github.com/Azure/prometheus-collector/settings/secrets/actions):
    a. MANAGED_PROM_SERVICE_PRINCIPAL_OBJECT_ID (This will likely remain the same unless you create a new service principal).
    b. MANAGED_PROM_SERVICE_PRINCIPAL_PASSWORD
7. Update the following variables in the [release pipeline](https://github-private.visualstudio.com/azure/_releaseDefinition?definitionId=79&_a=definition-variables)
    a. KSMChartTag - with the new version
    b. NEChartTag - with the new version
    c. PushNewKSMChart - to true for the said release (remember to set it back to false after the release is done!)
    d. PushNewNEChart - to true for the said release (remember to set it back to false after the release is done!)

For upgrading all other dependencies, update them in setup.sh (for linux) and setup.ps1(for win).

Below are some links to the packes repo of these dependencies.

Mariner packages repo
1. https://packages.microsoft.com/cbl-mariner/2.0/prod/extras/x86_64/Packages/a/
2. https://packages.microsoft.com/cbl-mariner/2.0/prod/base/x86_64/Packages/a/

Telegraf
1. https://github.com/influxdata/telegraf/releases

ME(windows)
1. https://msblox.visualstudio.com/Azure%20Geneva%20Monitoring%20and%20Diagnostics%20Pipeline/_artifacts/feed/AzureGenevaMonitoring/NuGet/MdmMetricsExtension/versions/2.2023.224.2214
