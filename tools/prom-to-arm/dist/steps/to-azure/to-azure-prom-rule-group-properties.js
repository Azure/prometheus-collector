"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const prom_duration_to_iso8601_1 = __importDefault(require("../../utils/prom-duration-to-iso8601"));
const to_azure_prom_alerting_rule_1 = __importDefault(require("./to-azure-prom-alerting-rule"));
const to_azure_prom_recording_rule_1 = __importDefault(require("./to-azure-prom-recording-rule"));
const converter_1 = __importDefault(require("../../utils/converter"));
const ruleConverter = (rules, options) => rules === null || rules === void 0 ? void 0 : rules.map(rule => rule.record ? (0, to_azure_prom_recording_rule_1.default)(rule, options) : (0, to_azure_prom_alerting_rule_1.default)(rule, options));
const converters = {
    interval: prom_duration_to_iso8601_1.default,
    rules: ruleConverter
};
function createExtendedPropsFunc(params) {
    return {
        interval: "PT1M",
        scopes: [
            "[parameters('azureMonitorWorkspace')]"
        ],
        clusterName: "[parameters('clusterName')]"
    };
}
const converter = new converter_1.default(converters, createExtendedPropsFunc, ['name', 'limit']);
exports.default = converter.convert.bind(converter);
