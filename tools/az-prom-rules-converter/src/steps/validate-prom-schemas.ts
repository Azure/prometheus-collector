import Ajv from 'ajv'
import addFormats from "ajv-formats"
import StepResult from '../types/step-result'

import promSchema from '../schemas/prometheus/prometheus.rules.json'
const promAjv = new Ajv();
addFormats(promAjv);
const ajvValidatePromSchema = promAjv.compile(promSchema);
export default (json: any, options: any) : StepResult => {
  const result : StepResult = {
    success: true,
    output: json
  };
  if (!options.skipValidation && !ajvValidatePromSchema(json)) {
    result.success = false,
    result.error = {
      title: 'Failed to validate Prometheus Rules Schema',
      details: ajvValidatePromSchema.errors,
    }
  }
  return result;
} 




