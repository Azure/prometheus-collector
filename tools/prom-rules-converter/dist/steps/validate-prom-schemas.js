"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const ajv_1 = __importDefault(require("ajv"));
const ajv_formats_1 = __importDefault(require("ajv-formats"));
const prometheus_rules_json_1 = __importDefault(require("../schemas/prometheus/prometheus.rules.json"));
const promAjv = new ajv_1.default();
(0, ajv_formats_1.default)(promAjv);
const ajvValidatePromSchema = promAjv.compile(prometheus_rules_json_1.default);
exports.default = (json, options) => {
    const result = {
        success: true,
        output: json
    };
    if (!options.skipValidation && !ajvValidatePromSchema(json)) {
        result.success = false,
            result.error = {
                title: 'Failed to validate Prometheus Rules Schema',
                details: ajvValidatePromSchema.errors,
            };
    }
    return result;
};
