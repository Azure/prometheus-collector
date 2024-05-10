"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.default = (text, options) => {
    if (!!text)
        return {
            success: true,
            output: text
        };
    return {
        success: false,
        error: {
            title: 'Input is empty',
            details: 'Input is empty'
        }
    };
};
