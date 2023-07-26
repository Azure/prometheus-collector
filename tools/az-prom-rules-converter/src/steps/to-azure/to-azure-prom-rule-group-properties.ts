import promDurationToIso8601 from "../../utils/prom-duration-to-iso8601";
import toAzurePromAlertingRule from './to-azure-prom-alerting-rule';
import toAzurePromRecordingRule from "./to-azure-prom-recording-rule";
import Converter from '../../utils/converter';

const ruleConverter = (rules: any[], options: any) => 
  rules?.map(rule => rule.record ? toAzurePromRecordingRule(rule, options) : toAzurePromAlertingRule(rule, options)); 

const converters: any = {
  interval: promDurationToIso8601,
  rules: ruleConverter
}

function createExtendedPropsFunc(params: any) {
  return {
    interval: "PT1M",
    scopes: [
      "[parameters('azureMonitorWorkspace')]"
    ],
    clusterName: "[parameters('clusterName')]"
  };
}

const converter = new Converter(converters, createExtendedPropsFunc, ['name', 'limit']); 

export default converter.convert.bind(converter);