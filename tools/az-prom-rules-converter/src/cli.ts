#!/usr/bin/env node
import { Command } from 'commander';
import toArmTemplateFlow from '.';

const program = new Command();

program.name('az-prom-rules-converter');
program.description('Azure Prometheus rule groups tool');
// program.version(pack.version);

program
  .description('Convert Prometheus rules Yaml file to ARM template')
  .argument('<input>', 'Input Prometheus rule groups Yaml file path or the json string representation if -j option is passed.')
  .option('-j, --json', 'Input Prometheus rule groups as a JSON string')
  .option('-amw, --azure-monitor-workspace <string>', 'Azure monitor workspace id\'s that this rule group is scoped to.')
  .option('-c, --cluster-name <string>', 'The cluster name of the rule group evaluation.')
  .option('-a, --action-group-id <string>', 'The resource id of the action group to use for alerting rules.')
  .option('-o, --output <string>', 'Output path. If not set, output would be printed to std out.')
  .option('-s, --skip-validation', 'Skip validation.')
  .option('-l, --location <string>', 'Rule group location.')
  .action(toArmTemplateFlow);

program.parse();
