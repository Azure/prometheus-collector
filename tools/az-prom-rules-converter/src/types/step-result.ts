export default interface StepResult {
  success: boolean,
  output?: any,
  error?: {
    title?: string,
    details?: any
  }
}
