import yamlToArmTemplateFlow from './index';
import StepResult from './types/step-result';
import fs from "fs/promises";

describe('toArmTemplateFlow exmpale 1', () => {
  let expectedResult: any;

  beforeAll(async () => {
    expectedResult = JSON.parse(
      await fs.readFile("./examples/result1.json", { encoding: "utf-8" })
    );
  });

  test('Successful flow with valid YAML input', async () => {
    await readFileAndRunFlow("./examples/yaml-example1.yml");

  });

  test("Successful flow with valid JSON input", async () => {
    await readFileAndRunFlow("./examples/json-example1.json");
  });

  async function readFileAndRunFlow(examplePath : string) {
    const options = {};
    const str = (await fs.readFile(examplePath))?.toString();
    const result: StepResult = yamlToArmTemplateFlow(str, options);

    expect(result.success).toBe(true);
    expect(result.output).toEqual(expectedResult);
  }

});