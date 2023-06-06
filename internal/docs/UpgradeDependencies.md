This document outlines briefly how to upgrade KSM,Node Exporter, Telegraf and other components.

Example PR: https://github.com/Azure/prometheus-collector/pull/418

Updating KSM and NE charts
1. For updating the charts for KSM and NE, replace the respective folders from here respectively under dependentcharts, [KSM](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics) and [NE](https://github.com/prometheus-community/helm-charts/tree/main/charts/prometheus-node-exporter).
2. Update the Version and Appversion in all the yaml files for both 1p and addon charts from the example PR above. (Note : The appversion corresponds to the Version in the kube-state-metrics chart and the version to the appversion)
3. Update both the images in our pipeline build file : azure-pipeline-build.yml

For upgrading all other dependencies, update them in setup.sh (for linux) and setup.ps1(for win).

Below are some links to the packes repo of these dependencies.

Mariner packages repo
1. https://packages.microsoft.com/cbl-mariner/2.0/prod/extras/x86_64/Packages/a/
2. https://packages.microsoft.com/cbl-mariner/2.0/prod/base/x86_64/Packages/a/

Telegraf
1. https://github.com/influxdata/telegraf/releases

ME(windows)
1. https://msblox.visualstudio.com/Azure%20Geneva%20Monitoring%20and%20Diagnostics%20Pipeline/_artifacts/feed/AzureGenevaMonitoring/NuGet/MdmMetricsExtension/versions/2.2023.224.2214
