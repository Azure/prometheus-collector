# This script is only for AKS cluster testing. It reads the template files (Chart-template.yaml and values-template.yaml),
# replaces placeholders with actual values, removes specific sections, 
# and then writes the modified content back to new files (Chart.yaml and values.yaml).
# The placeholders replaced include HELM_CHART_NAME, IMAGE_TAG, MCR_REPOSITORY, ARC_EXTENSION, 
# AKS_REGION, and AKS_RESOURCE_ID. It also removes the dependencies section from the 
# Chart-template.yaml file.

# Define variables
$ImageTag = "6.9.0-kaveesh-golang-windows-09-23-2024-f3674c7f"
$AKSRegion = "northeurope"
$AKSResourceId = "/subscriptions/ce4d1293-71c0-4c72-bc55-133553ee9e50/resourceGroups/kaveeshwin/providers/Microsoft.ContainerService/managedClusters/kaveeshwin"

# Read files
$chartTemplatePath = ".\Chart-template.yaml"
$valuesTemplatePath = ".\values-template.yaml"

$chartTemplateContent = Get-Content -Path $chartTemplatePath -Raw
$valuesTemplateContent = Get-Content -Path $valuesTemplatePath -Raw

# Create copies of the files
$chartOutputPath = ".\Chart.yaml"
$valuesOutputPath = ".\values.yaml"
$chartTemplateContent | Out-File -FilePath $chartOutputPath
$valuesTemplateContent | Out-File -FilePath $valuesOutputPath

# Replace placeholders in Chart-template.yaml
$chartTemplateContent = $chartTemplateContent -replace '\$\{HELM_CHART_NAME\}', 'ama-metrics'
$chartTemplateContent = $chartTemplateContent -replace '\$\{IMAGE_TAG\}', $ImageTag
# Remove the dependencies section
$chartTemplateContent = $chartTemplateContent -replace '(?s)dependencies:\s*-\s*name:\s*prometheus-node-exporter\s*version:\s*"4\.26\.0"\s*repository:\s*oci://\$\{MCR_REGISTRY\}\$\{MCR_REPOSITORY_HELM_DEPENDENCIES\}\s*condition:\s*AzureMonitorMetrics\.ArcExtension\s*', ''


# Replace placeholders in values-template.yaml
$valuesTemplateContent = $valuesTemplateContent -replace '\$\{IMAGE_TAG\}', $ImageTag
$valuesTemplateContent = $valuesTemplateContent -replace '\$\{MCR_REPOSITORY\}', '/azuremonitor/containerinsights/cidev/prometheus-collector/images'
$valuesTemplateContent = $valuesTemplateContent -replace '\$\{ARC_EXTENSION\}', 'false'
$valuesTemplateContent = $valuesTemplateContent -replace '\$\{AKS_REGION\}', $AKSRegion
$valuesTemplateContent = $valuesTemplateContent -replace '\$\{AKS_RESOURCE_ID\}', $AKSResourceId

# Write the modified content back to the files
$chartTemplateContent | Out-File -FilePath $chartOutputPath
$valuesTemplateContent | Out-File -FilePath $valuesOutputPath

Write-Host "Files have been processed and saved as Chart.yaml and values.yaml"
