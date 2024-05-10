"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const validate_azure_prom_schema_1 = __importDefault(require("./validations/validate-azure-prom-schema"));
function validateArmTemplate(armTemplate, options) {
    const result = {
        success: true,
        output: armTemplate
    };
    if (options.skipValidation) {
        return result;
    }
    armTemplate.resources.every((resource, i) => {
        const success = (0, validate_azure_prom_schema_1.default)(resource);
        if (!success) {
            result.success = false;
            result.error = {
                title: `Failed to validate Azure Prometheus schema for group ${i}`,
                details: validate_azure_prom_schema_1.default.errors
            };
        }
    });
    return result;
}
exports.default = validateArmTemplate;
