
export default class Converter {
  converters: Record<string, (value: any, params: any) => any>
  createExtendedPropsFunc: (params: any) => any
  exclude: string[]
  propertyNamesMap: Record<string, string>

  constructor(converters: Record<string, (value: any, params: any) => any>, createExtendedPropsFunc: (params: any) => any, exclude: string[] = [], propertyNamesMap: Record<string, string> = {}) {
    this.converters = converters;
    this.createExtendedPropsFunc = createExtendedPropsFunc;
    this.exclude = exclude;
    this.propertyNamesMap = propertyNamesMap;
  }


  convert(sourceObj: any, params: any) {
    const extraProps = this.createExtendedPropsFunc(params);
    const filtered = this._removeExcluded(sourceObj);
    const converted = this._convertObject(filtered, params);
    return Object.assign(extraProps, this._changePropertyNames(converted));
  }

  _changePropertyNames(obj: any) {
    Object.keys(this.propertyNamesMap).forEach(key => {
      if (obj[key] != undefined) {
        obj[this.propertyNamesMap[key]] = obj[key];
        delete obj[key];
      }
    });
    return obj;
  }

  _removeExcluded(obj: any) {
    if (!this.exclude) 
      return Object.assign(obj);

    const filtered = Object.keys(obj)
    .filter(key => !this.exclude.includes(key))
    .reduce((newObj: any, key: string) => {
      newObj[key] = obj[key];
      return newObj;
    }, {});
    return filtered;
  }

  _convertObject(obj: any, params: any) {
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
