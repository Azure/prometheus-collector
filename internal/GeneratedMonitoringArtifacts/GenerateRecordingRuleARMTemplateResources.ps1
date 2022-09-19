# Helper script to generate the ARM template resources for the default Recording Rules
# This script outputs just the resources, you can copy / paste into a full ARM template
#
#   Attempted to use ConvertFrom-YAML to load the .yaml directly but hit formatting issues
#   For now manually converting to json using: https://jsonformatter.org/yaml-to-json
#   And removing \n with edot
#   TODO: figure out better way, perhaps python better for yaml files
#  
#   Note - this does not retain the original rule group was in, just outputs flat list of rules
#          also if uses a non-default interval such as api-server does not retain

# text file with the names of rules to include
$includeRulesFile = ".\DefaultRecordingRulesList.txt"

# json with the recording rules
#$recordingRuleFile = "D:\GitHub\prometheus-collector\mixins\kubernetes\prometheus_rules.json"
$recordingRuleFile = "D:\GitHub\prometheus-collector\mixins\node\node_rules.json"

# Laoad the files
$includeRules = Get-Content $includeRulesFile
$rules = Get-Content $recordingRuleFile -Raw | ConvertFrom-Json

# variable for output
$outputRules = @()

ForEach ($group in $rules.groups) {
    ForEach ($rule in $group.rules) {
        # $rule.record
        if ($includeRules -contains $rule.record) {
            #$rule.record

            $myObject = [PSCustomObject]@{
                record = $rule.record
                expression = $rule.expr
            }

            if ( $rule.labels ) {
                $myObject | Add-Member -MemberType NoteProperty -Name 'labels' -Value $rule.labels 
            }

            $outputRules += $myObject
        }
    }
}

ConvertTo-Json -InputObject $outputRules