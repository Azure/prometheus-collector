#!/usr/bin/env node
import path from 'path';
import { Command } from 'commander';
import fs from 'fs/promises';
import toArmTemplateFlow from '.';
import StepResult from './types/step-result';

const program = new Command();

program.name('az-prom-rules-converter');
program.description('Azure Prometheus rule groups tool');
// program.version(pack.version);

async function yaml2arm(inputPath: string, options: any, command: Command) {
  const inputAbsolutePath = path.resolve(inputPath);
  const str = (await fs.readFile(inputAbsolutePath))?.toString();
  const flowResult: StepResult = toArmTemplateFlow(str, options);
  
  if (flowResult.success == false){
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

program//.command('yaml2arm')
  .description('Convert Prometheus rules Yaml file to ARM template')
  .argument('<input>', 'Input Prometheus rule groups Yaml file path.')
  .option('-amw, --azure-monitor-workspace <string>', 'Azure monitor workspace id\'s that this rule group is scoped to.')
  .option('-c, --cluster-name <string>', 'The cluster name of the rule group evaluation.')
  .option('-a, --action-group-id <string>', 'The resource id of the action group to use for alerting rules.')
  .option('-o, --output <string>', 'Output path. If not set, output would be printed to std out.')
  .option('-s, --skip-validation', 'Skip validation.')
  .option('-l, --location <string>', 'Rule group location.')
  .action(yaml2arm);

program.parse();