export default class Converter {
    converters: Record<string, (value: any, params: any) => any>;
    createExtendedPropsFunc: (params: any) => any;
    exclude: string[];
    propertyNamesMap: Record<string, string>;
    constructor(converters: Record<string, (value: any, params: any) => any>, createExtendedPropsFunc: (params: any) => any, exclude?: string[], propertyNamesMap?: Record<string, string>);
    convert(sourceObj: any, params: any): any;
    _changePropertyNames(obj: any): any;
    _removeExcluded(obj: any): any;
    _convertObject(obj: any, params: any): any;
}
