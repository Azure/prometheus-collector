# Define variables
$ImageTag = "{IMAGE_TAG}"
$AKSRegion = "{AKS_REGION}"
$AKSResourceId = "{CLUSTER_RESOURCE_ID}"

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