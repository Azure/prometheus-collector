"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const yaml2json_1 = __importDefault(require("./steps/yaml2json"));
const validate_prom_schemas_1 = __importDefault(require("./steps/validate-prom-schemas"));
const validate_arm_template_1 = __importDefault(require("./steps/validate-arm-template"));
const to_arm_template_1 = __importDefault(require("./steps/to-arm-template"));
const validate_input_not_empty_1 = __importDefault(require("./steps/validate-input-not-empty"));
const steps = [
    validate_input_not_empty_1.default,
    yaml2json_1.default,
    validate_prom_schemas_1.default,
    to_arm_template_1.default,
    validate_arm_template_1.default
];
function yamlToArmTemplateFlow(yamlStr, options) {
    let input = yamlStr;
    let result = {
        success: true
    };
    for (let i = 0; i < steps.length && result.success; i++) {
        result = steps[i](input, options);
        input = result.output;
    }
    return result;
}
exports.default = yamlToArmTemplateFlow;
