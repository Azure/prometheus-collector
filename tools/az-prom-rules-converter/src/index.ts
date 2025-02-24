import yaml2json from './steps/yaml2json'
import StepResult from './types/step-result'
import validatePromSchema  from './steps/validate-prom-schemas'
import validateArmTemplate from './steps/validate-arm-template'
import toArmTemplate from './steps/to-arm-template'
import validateInputNotEmpty from './steps/validate-input-not-empty'


const steps: ((input: any, options: any) => StepResult)[] = [
  validateInputNotEmpty,
  yaml2json, 
  validatePromSchema,
  toArmTemplate,
  validateArmTemplate
];

export default function yamlToArmTemplateFlow(yamlStr: string, options: any) : StepResult {
  let input = yamlStr;
  let result: StepResult = {
    success: true
  }

  for (let i = 0; i < steps.length && result.success; i++) {
    result = steps[i](input, options);
    input = result.output;
  }

  return result;
}
