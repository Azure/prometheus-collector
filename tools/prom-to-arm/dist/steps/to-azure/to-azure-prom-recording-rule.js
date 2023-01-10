"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const converter_1 = __importDefault(require("../../utils/converter"));
const mapPropertyNames = {
    expr: 'expression'
};
const converter = new converter_1.default({}, () => ({}), [], mapPropertyNames);
exports.default = converter.convert.bind(converter);
