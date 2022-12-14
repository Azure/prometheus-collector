import StepResult from "../types/step-result";

export default (text: string, options: any) : StepResult => {
  if (!!text) return {
    success: true,
    output: text
  }; 
  return {
    success: false,
    error: {
      title: 'Input is empty',
      details: 'Input is empty'
    }
  }
} 