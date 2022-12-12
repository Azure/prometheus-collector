"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const moment_1 = __importDefault(require("moment"));
const millisecond = 1, second = 1000 * millisecond, minute = 60 * second, hour = 60 * minute, day = 24 * hour, week = 7 * day, year = 356 * day;
const unitMap = {
    'ms': millisecond,
    's': second,
    'm': minute,
    'h': hour,
    'd': day,
    'w': week,
    'y': year
};
const durationRegex = /^(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?$/;
function parseDuration(str) {
    const matches = str.match(durationRegex) || [];
    if (matches[0] != str)
        throw new Error(`${str} don't match ${durationRegex}`);
    const result = {};
    ['y', 'w', 'd', 'h', 'm', 's', 'ms'].forEach((s, i) => {
        if (matches[(i + 1) * 2])
            result[s] = Number(matches[(i + 1) * 2]);
    });
    return result;
}
function toMomentDuration(parsedDuration) {
    let duration = moment_1.default.duration(0);
    for (const unit in parsedDuration) {
        duration = duration.add(parsedDuration[unit], unit);
    }
    return duration;
}
function promDurationToIso8601(promDuration) {
    const duration = parseDuration(promDuration);
    return toMomentDuration(duration).toISOString();
}
exports.default = promDurationToIso8601;
