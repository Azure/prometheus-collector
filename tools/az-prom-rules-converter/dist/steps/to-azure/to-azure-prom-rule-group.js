"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const to_azure_prom_rule_group_properties_1 = __importDefault(require("./to-azure-prom-rule-group-properties"));
function toAzurePromRuleGroup(group, params) {
    const properties = (0, to_azure_prom_rule_group_properties_1.default)(group, params);
    return {
        name: group.name,
        "type": "Microsoft.AlertsManagement/prometheusRuleGroups",
        "apiVersion": "2023-03-01",
        "location": "[parameters('location')]",
        properties
    };
}
exports.default = toAzurePromRuleGroup;
