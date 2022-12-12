import { PrometheusRulesGroup } from "../../types/prometheus-rules"
import toArmPrometheusRulesGroupProperties from './to-azure-prom-rule-group-properties'


export default function toAzurePromRuleGroup(group: PrometheusRulesGroup, params: any) {
  const properties = toArmPrometheusRulesGroupProperties(group, params);  
  
  return {
      name: group.name,
      "type": "Microsoft.AlertsManagement/prometheusRuleGroups",
      "apiVersion": "2021-07-22-preview",
      "location": "[parameters('location')]",
      properties 
  };
}