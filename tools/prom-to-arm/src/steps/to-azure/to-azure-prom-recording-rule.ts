import Converter from "../../utils/converter";

const mapPropertyNames = {
  expr: 'expression'
}

const converter = new Converter({}, () => ({}), [], mapPropertyNames);
export default converter.convert.bind(converter);
