"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const path_1 = __importDefault(require("path"));
const promises_1 = __importDefault(require("fs/promises"));
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
function toArmTemplateFlow(input, options) {
    let result = {
        success: true
    };
    for (let i = 0; i < steps.length && result.success; i++) {
        result = steps[i](input, options);
        input = result.output;
    }
    return result;
}
function convert(input, options) {
    var _a, _b, _c;
    return __awaiter(this, void 0, void 0, function* () {
        let str;
        if (options.json) {
            steps.splice(steps.indexOf(yaml2json_1.default), 1);
            str = JSON.parse(input);
        }
        else {
            const inputAbsolutePath = path_1.default.resolve(input);
            str = (_a = (yield promises_1.default.readFile(inputAbsolutePath))) === null || _a === void 0 ? void 0 : _a.toString();
        }
        const flowResult = toArmTemplateFlow(str, options);
        if (flowResult.success == false) {
            console.error((_b = flowResult.error) === null || _b === void 0 ? void 0 : _b.title);
            console.error(JSON.stringify((_c = flowResult.error) === null || _c === void 0 ? void 0 : _c.details, null, 2));
            return;
        }
        const flowResultString = JSON.stringify(flowResult.output, null, 2);
        if (options.output) {
            const outputAbsolutePath = path_1.default.resolve(options.output);
            yield promises_1.default.writeFile(outputAbsolutePath, flowResultString, 'utf8');
        }
        else {
            console.log(flowResultString);
        }
    });
}
exports.default = convert;
