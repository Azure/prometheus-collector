# ARMClient doc: https://github.com/projectkudu/ARMClient
# ARMClient login

$grafanaResourceId="/subscriptions/{sub_id}/resourceGroups/{rg_name}/providers/Microsoft.Dashboard/grafana/{name}"
$grafanaApiVersion="2023-10-01-preview"

armclient get "$($grafanaResourceId)?api-version=$($grafanaApiVersion)"

Write-Output "Add user-assigned managed identity to Grafana"
armclient patch "$($grafanaResourceId)?api-version=$($grafanaApiVersion)" patch-add-umi.json -verbose
