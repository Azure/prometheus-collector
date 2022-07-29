
# Deleting any output files if they exist

$rules_group_1 = 'k8s-resource-windows-cluster-rules-group-1.json'
$rules_group_2 = 'k8s-resource-windows-cluster-rules-group-2.json'
$rules_group_3 = 'k8s-resource-windows-cluster-rules-group-3.json'
$rules_group_4 = 'k8s-resource-windows-cluster-rules-group-4.json'
$rules_group_5 = 'default-prometheus-alerts.json'

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

if (Test-Path $rules_group_4) {
    Write-Output "Deleting $rules_group_4"
    Remove-Item $rules_group_4
}

if (Test-Path $rules_group_5) {
    Write-Output "Deleting $rules_group_5"
    Remove-Item $rules_group_5
}

Write-Output "-----------------------------------------------------------------------------------"
# Reading values file

$values_ps_object = Get-Content .\templates\values.json -Raw | ConvertFrom-Json 

Write-Output "The following values were read from the supplied values.json file..."

Write-Output $values_ps_object.MACLocation
Write-Output $values_ps_object.mac
Write-Output $values_ps_object.cluster

$mac_name = $values_ps_object.mac.split('/')[8]
$rulegroupname1 = $mac_name + '_' + $values_ps_object.cluster + 'ci_rulegroup_1'
$rulegroupname2 = $mac_name + '_' + $values_ps_object.cluster + 'ci_rulegroup_2'
$rulegroupname3 = $mac_name + '_' + $values_ps_object.cluster + 'ci_rulegroup_3'
$rulegroupname4 = $mac_name + '_' + $values_ps_object.cluster + 'ci_rulegroup_4'
$rulegroupname5 = $mac_name + '_' + $values_ps_object.cluster + 'ci_rulegroup_5'

$rulegroupname1 = $rulegroupname1.replace('-', '_')
$rulegroupname2 = $rulegroupname2.replace('-', '_')
$rulegroupname3 = $rulegroupname3.replace('-', '_')
$rulegroupname4 = $rulegroupname4.replace('-', '_')
$rulegroupname5 = $rulegroupname5.replace('-', '_')

Write-output $rulegroupname1
Write-output $rulegroupname2
Write-output $rulegroupname3
Write-output $rulegroupname4
Write-output $rulegroupname5

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Replacing location, mac and cluster in the template files..."

(Get-Content -path .\templates\$rules_group_1).replace('$MACLocation', $values_ps_object.MACLocation) | Set-Content -Path .\$rules_group_1
(Get-Content -path .\$rules_group_1).replace('$name', $rulegroupname1) | Set-Content -Path .\$rules_group_1
(Get-Content -path .\$rules_group_1).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_1
(Get-Content -path .\$rules_group_1).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_1


(Get-Content -path .\templates\$rules_group_2).replace('$MACLocation', $values_ps_object.MACLocation) | Set-Content -Path .\$rules_group_2
(Get-Content -path .\$rules_group_2).replace('$name', $rulegroupname2) | Set-Content -Path .\$rules_group_2
(Get-Content -path .\$rules_group_2).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_2
(Get-Content -path .\$rules_group_2).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_2


(Get-Content -path .\templates\$rules_group_3).replace('$MACLocation', $values_ps_object.MACLocation) | Set-Content -Path .\$rules_group_3
(Get-Content -path .\$rules_group_3).replace('$name', $rulegroupname3) | Set-Content -Path .\$rules_group_3
(Get-Content -path .\$rules_group_3).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_3
(Get-Content -path .\$rules_group_3).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_3


(Get-Content -path .\templates\$rules_group_4).replace('$MACLocation', $values_ps_object.MACLocation) | Set-Content -Path .\$rules_group_4
(Get-Content -path .\$rules_group_4).replace('$name', $rulegroupname4) | Set-Content -Path .\$rules_group_4
(Get-Content -path .\$rules_group_4).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_4
(Get-Content -path .\$rules_group_4).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_4

(Get-Content -path .\templates\$rules_group_5).replace('$MACLocation', $values_ps_object.MACLocation) | Set-Content -Path .\$rules_group_5
(Get-Content -path .\$rules_group_5).replace('$name', $rulegroupname5) | Set-Content -Path .\$rules_group_5
(Get-Content -path .\$rules_group_5).replace('$mac', $values_ps_object.mac) | Set-Content -Path .\$rules_group_5
(Get-Content -path .\$rules_group_5).replace('$cluster', $values_ps_object.cluster) | Set-Content -Path .\$rules_group_5

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Logging into az to deploy the recording rules"

az login
# az login --use-device-code

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Setting subscription extracted from the MAC scope"

$sub_id_from_mac = $values_ps_object.mac.split('/')[2]

az account set --subscription $sub_id_from_mac

Write-Output "-----------------------------------------------------------------------------------"

$resource_group_from_mac = $values_ps_object.mac.split('/')[4]

Write-Output "Deploying $rules_group_1"

az deployment group create --resource-group $resource_group_from_mac --template-file .\$rules_group_1

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_2"

az deployment group create --resource-group $resource_group_from_mac --template-file .\$rules_group_2


Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_3"

az deployment group create --resource-group $resource_group_from_mac --template-file .\$rules_group_3


Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_4"

az deployment group create --resource-group $resource_group_from_mac --template-file .\$rules_group_4

Write-Output "-----------------------------------------------------------------------------------"

Write-Output "Deploying $rules_group_5"

az deployment group create --resource-group $resource_group_from_mac --template-file .\$rules_group_5