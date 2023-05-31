import  StepResult  from '../types/step-result';
import validateAzurePromSchema from './validations/validate-azure-prom-schema'

export default function validateArmTemplate(armTemplate: any, options: any) : StepResult {
  const result : StepResult = {
    success: true,
    output: armTemplate
  }
  if (options.skipValidation) {
    return result;
  }
  
  armTemplate.resources.every((resource: any, i: number) => {
    const success = validateAzurePromSchema(resource);
    if (!success) {
      result.success = false;
      result.error = {
        title: `Failed to validate Azure Prometheus schema for group ${i}`,
        details: validateAzurePromSchema.errors
      } 
    }
  });

  return result;
}