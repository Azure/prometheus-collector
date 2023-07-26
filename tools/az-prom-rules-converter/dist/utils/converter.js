"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
class Converter {
    constructor(converters, createExtendedPropsFunc, exclude = [], propertyNamesMap = {}) {
        this.converters = converters;
        this.createExtendedPropsFunc = createExtendedPropsFunc;
        this.exclude = exclude;
        this.propertyNamesMap = propertyNamesMap;
    }
    convert(sourceObj, params) {
        const extraProps = this.createExtendedPropsFunc(params);
        const filtered = this._removeExcluded(sourceObj);
        const converted = this._convertObject(filtered, params);
        return Object.assign(extraProps, this._changePropertyNames(converted));
    }
    _changePropertyNames(obj) {
        Object.keys(this.propertyNamesMap).forEach(key => {
            if (obj[key] != undefined) {
                obj[this.propertyNamesMap[key]] = obj[key];
                delete obj[key];
            }
        });
        return obj;
    }
    _removeExcluded(obj) {
        if (!this.exclude)
            return Object.assign(obj);
        const filtered = Object.keys(obj)
            .filter(key => !this.exclude.includes(key))
            .reduce((newObj, key) => {
            newObj[key] = obj[key];
            return newObj;
        }, {});
        return filtered;
    }
    _convertObject(obj, params) {
        const res = Object.assign({}, obj);
        if (!this.converters)
            return res;
        Object.keys(this.converters).forEach(key => {
            if (obj[key] != undefined) {
                res[key] = this.converters[key](obj[key], params);
            }
        });
        return res;
    }
}
exports.default = Converter;
