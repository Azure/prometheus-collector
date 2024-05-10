#!/usr/bin/env node
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
const commander_1 = require("commander");
const promises_1 = __importDefault(require("fs/promises"));
const _1 = __importDefault(require("."));
const program = new commander_1.Command();
program.name('az-prom-rules-converter');
program.description('Azure Prometheus rule groups tool');
// program.version(pack.version);
function yaml2arm(inputPath, options, command) {
    var _a, _b, _c;
    return __awaiter(this, void 0, void 0, function* () {
        const inputAbsolutePath = path_1.default.resolve(inputPath);
        const str = (_a = (yield promises_1.default.readFile(inputAbsolutePath))) === null || _a === void 0 ? void 0 : _a.toString();
        const flowResult = (0, _1.default)(str, options);
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
program //.command('yaml2arm')
    .description('Convert Prometheus rules Yaml file to ARM template')
    .argument('<input>', 'Input Prometheus rule groups Yaml file path.')
    .option('-amw, --azure-monitor-workspace <string>', 'Azure monitor workspace id\'s that this rule group is scoped to.')
    .option('-c, --cluster-name <string>', 'The cluster name of the rule group evaluation.')
    .option('-a, --action-group-id <string>', 'The resource id of the action group to use for alerting rules.')
    .option('-o, --output <string>', 'Output path. If not set, output would be printed to std out.')
    .option('-s, --skip-validation', 'Skip validation.')
    .option('-l, --location <string>', 'Rule group location.')
    .action(yaml2arm);
program.parse();
