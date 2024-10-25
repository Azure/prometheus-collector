import path from 'path';
import fs from 'fs/promises';

import yaml2json from './steps/yaml2json'
import StepResult from './types/step-result'
import validatePromSchema from './steps/validate-prom-schemas'
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

function toArmTemplateFlow(input: string, options: any): StepResult {
  let result: StepResult = {
    success: true
  }

  for (let i = 0; i < steps.length && result.success; i++) {
    result = steps[i](input, options);
    input = result.output;
  }

  return result;
}

export default async function (input: string, options: any) {
  let str: string
  if (options.json) {
    steps.splice(steps.indexOf(yaml2json), 1);
    str = JSON.parse(input)
  } else {
    const inputAbsolutePath = path.resolve(input);
    str = (await fs.readFile(inputAbsolutePath))?.toString();
  }
  const flowResult: StepResult = toArmTemplateFlow(str, options);

  if (flowResult.success == false) {
    console.error(flowResult.error?.title);
    console.error(JSON.stringify(flowResult.error?.details, null, 2));
    return;
  }

  const flowResultString = JSON.stringify(flowResult.output, null, 2);
  if (options.output) {
    const outputAbsolutePath = path.resolve(options.output);
    await fs.writeFile(outputAbsolutePath, flowResultString, 'utf8');
  } else {
    console.log(flowResultString);
  }
}
