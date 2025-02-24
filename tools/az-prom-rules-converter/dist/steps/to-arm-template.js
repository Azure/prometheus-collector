"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const to_azure_prom_rule_group_1 = __importDefault(require("./to-azure/to-azure-prom-rule-group"));
function toArmTemplate(promRules, params) {
    var _a;
    const result = {
        success: true,
        output: getArmTemplateFormat(params)
    };
    (_a = promRules === null || promRules === void 0 ? void 0 : promRules.groups) === null || _a === void 0 ? void 0 : _a.every((group, i) => {
        try {
            const resource = (0, to_azure_prom_rule_group_1.default)(group, params);
            result.output.resources.push(resource);
            return true;
        }
        catch (exception) {
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
exports.default = toArmTemplate;
/**
 * Get Arm Template format
 * taken from https://learn.microsoft.com/en-us/azure/azure-resource-manager/templates/syntax
 * @returns
 */
const getArmTemplateFormat = (params) => {
    const result = {
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
        if (params[paramName]) {
            result.parameters[paramName].defaultValue = params[paramName];
        }
    });
    return result;
};
