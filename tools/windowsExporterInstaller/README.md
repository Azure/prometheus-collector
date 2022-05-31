<!-- Update the bottom of the `deployPrometheusWindowsExporter.ps1` script with the subscription and cluster resource group (starting with `MC_`) that you want to install the VMSS extension for auto-deploying windows-exporter on. -->

Deploy the `windows-exporter-daemonset.yaml` to your kubernetes cluster (version 1.21+) to get windows exporter running on your clusters nodes.

For versions lower than 1.21, please use the `deployPrometheusWindowsExporter.ps1` script with the subscription and cluster resource group (starting with `MC_`) that you want to install the VMSS extension for auto-deploying windows-exporter on.