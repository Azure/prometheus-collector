"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const prom_duration_to_iso8601_1 = __importDefault(require("../../utils/prom-duration-to-iso8601"));
const converter_1 = __importDefault(require("../../utils/converter"));
const alertingRuleConverters = {
    for: prom_duration_to_iso8601_1.default
};
const mapPropertyNames = {
    expr: 'expression'
};
function createExtendedPropsFunc(params) {
    return {
        severity: 3,
        resolveConfiguration: {
            autoResolved: true,
            timeToResolve: "PT10M"
        },
        actions: [
            {
                "actionGroupId": "[parameters('actionGroupId')]"
            }
        ]
    };
}
const toArmPrometheusRuleConverter = new converter_1.default(alertingRuleConverters, createExtendedPropsFunc, [], mapPropertyNames);
exports.default = toArmPrometheusRuleConverter.convert.bind(toArmPrometheusRuleConverter);
