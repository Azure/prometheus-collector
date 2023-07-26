"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const js_yaml_1 = __importDefault(require("js-yaml"));
function yamlToJson(yamlStr) {
    let res = {
        success: true,
        output: {},
    };
    try {
        const doc = js_yaml_1.default.load(yamlStr);
        res.output = doc;
    }
    catch (e) {
        res.success = false;
        res.error = {
            title: 'Failed to convert YAML to JSON',
            details: e
        };
    }
    return res;
}
exports.default = yamlToJson;
