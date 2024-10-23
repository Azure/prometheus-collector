import { PrometheusRules } from '../types/prometheus-rules'
import StepResult from '../types/step-result';
import toAzurePromRuleGroup from './to-azure/to-azure-prom-rule-group'

export default function toArmTemplate(promRules: PrometheusRules, params: any): StepResult {
  const result: StepResult = {
    success: true,
    output: getArmTemplateFormat(params)
  };

  promRules?.groups?.every((group, i) => {
    try {
      const resource = toAzurePromRuleGroup(group, params);
      result.output.resources.push(resource);
      return true;
    } catch (exception) {
      result.success = false;
      result.error = {
        title: `Error converting group ${i}`,
        details: {
          group,
          exception
        }
      };
      return false;
    }
  });

  return result;
}

/**
 * Get Arm Template format
 * taken from https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/syntax
 * @returns
 */
const getArmTemplateFormat = (params: any): any => {
  const result: any = {
    $schema: "https://schema.management.azure.com/schemas/2019-04-01/deploymentTemplate.json#",
    contentVersion: "1.0.0.0",
    parameters: {
      location: {
        "type": "string",
        "defaultValue": "[resourceGroup().location]"
      },
      clusterName: {
        "type": "string",
        "metadata": {
          "description": "Cluster name"
        }
      },
      actionGroupId: {
        "type": "string",
        "metadata": {
          "description": "Action Group ResourceId"
        }
      },
      azureMonitorWorkspace: {
        "type": "string",
        "metadata": {
          "description": "ResourceId of Azure monitor workspace to associate to"
        }
      }
    },
    variables: {},
    resources: []
  };
  ['clusterName', 'actionGroupId', 'azureMonitorWorkspace', 'location'].forEach((paramName) => {
    // console.log(paramName, params[paramName]);
    if (params[paramName]) {
      result.parameters[paramName].defaultValue = params[paramName];
    }
  });
  return result;
}
