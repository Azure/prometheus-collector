"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const ajv_1 = __importDefault(require("ajv"));
const ajv_formats_1 = __importDefault(require("ajv-formats"));
const azure_prometheus_rule_group_json_1 = __importDefault(require("../../schemas/azure/azure-prometheus-rule-group.json"));
const azure_common_types_json_1 = __importDefault(require("../../schemas/azure/azure-common-types.json"));
const azurePromAjv = new ajv_1.default({ strictSchema: false });
(0, ajv_formats_1.default)(azurePromAjv);
azurePromAjv.addSchema(azure_common_types_json_1.default, 'azure-common-types.json');
exports.default = azurePromAjv.compile(azure_prometheus_rule_group_json_1.default);
