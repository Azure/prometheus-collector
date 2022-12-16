import yaml from 'js-yaml';
import StepResult from '../types/step-result';


export default function yamlToJson(yamlStr: string): StepResult {
  let res: StepResult = {
    success: true,
    output: {},
  };
  try {
    const doc = yaml.load(yamlStr);
    res.output = doc;
  } catch (e) {
    res.success = false;
    res.error = {
      title: 'Failed to convert YAML to JSON',
      details: e
    };
  }
  return res;
}


