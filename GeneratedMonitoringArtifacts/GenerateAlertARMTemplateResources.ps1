# Helper script to generate the ARM template resources for the default Alerts
# This script outputs just the resources, you can copy / paste into a full ARM template
#
#  Important: Use Powershell7 otherwise special characters are espcaped
#
#    See TODO's below, manual changes are needed for now

# text file with the names of alerts to include
$includeFile = ".\DefaultAlertsList.txt"
# json with the alerts
$alertsFile = "D:\GitHub\prometheus-collector\mixins\kubernetes\prometheus_alerts.json"

# Load the data files
$include = Get-Content $includeFile
$rules = Get-Content $alertsFile -Raw | ConvertFrom-Json

# variable for output
$output = @()

ForEach ($group in $rules.groups) {
    ForEach ($alert in $group.rules) {
        if ($include -contains $alert.alert) {

            $myObject = [PSCustomObject]@{
                alert                = $alert.alert
                expression           = $alert.expr  
                for                  = $alert.for # TODO: Convert to ISO 8601 duration format
                # TODO: for is optional, only add if set
                labels               = $alert.labels
                severity             = 3 # TODO - map from the label?
                resolveConfiguration = [PSCustomObject]@{
                    autoResolved  = $true
                    timeToResolve = "PT10M"
                }
                actions = @([PSCustomObject]@{ actionGroupId = "[parameters('actionGroupResourceId')]" })
            }

            $output += $myObject
        }
    }
}

ConvertTo-Json  -InputObject $output -depth 5