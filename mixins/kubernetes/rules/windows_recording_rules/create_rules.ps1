
# Deleting any output files if they exist

$rules_group_1 = 'k8s-resource-windows-cluster-rules-group-1.json'
$rules_group_2 = 'k8s-resource-windows-cluster-rules-group-2.json'
$rules_group_3 = 'k8s-resource-windows-cluster-rules-group-3.json'

Write-Output "Deleting output files from previous runs if they exist"

if (Test-Path $rules_group_1) {
    Write-Output "Deleting $rules_group_1"
    Remove-Item $rules_group_1
}

if (Test-Path $rules_group_2) {
    Write-Output "Deleting $rules_group_2"
    Remove-Item $rules_group_2
}

if (Test-Path $rules_group_3) {
    Write-Output "Deleting $rules_group_3"
    Remove-Item $rules_group_3
}

Write-Output "-----------------------------------------------------------------------------------"
# Reading values file

$values_ps_object = Get-Content .\templates\values.json -Raw | ConvertFrom-Json 

Write-Output "The following values were read from the supplied values.json file..."

Write-Output $values_ps_object.location
Write-Output $values_ps_object.mac
Write-Output $values_ps_object.cluster
Write-Output $values_ps_object.resource_group

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Replacing location, mac and cluster in the template files..."

(Get-Content -path .\templates\$rules_group_1).replace('$location', $values_ps_object.location) | Set-Content -Path .\$rules_group_1
(Get-Content -path .\$rules_group_1).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_1
(Get-Content -path .\$rules_group_1).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_1


(Get-Content -path .\templates\$rules_group_2).replace('$location', $values_ps_object.location) | Set-Content -Path .\$rules_group_2
(Get-Content -path .\$rules_group_2).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_2
(Get-Content -path .\$rules_group_2).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_2


(Get-Content -path .\templates\$rules_group_3).replace('$location', $values_ps_object.location) | Set-Content -Path .\$rules_group_3
(Get-Content -path .\$rules_group_3).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_3
(Get-Content -path .\$rules_group_3).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_3

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Logging into az to deploy the recording rules"

az login
# az login --use-device-code

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Setting subscription extracted from the MAC scope"

$sub_id_from_mac = $values_ps_object.mac.split('/')[2]

az account set --subscription $sub_id_from_mac

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_1"

az deployment group create --resource-group $values_ps_object.resource_group --template-file .\$rules_group_1

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_2"

az deployment group create --resource-group $values_ps_object.resource_group --template-file .\$rules_group_2


Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_3"

az deployment group create --resource-group $values_ps_object.resource_group --template-file .\$rules_group_3