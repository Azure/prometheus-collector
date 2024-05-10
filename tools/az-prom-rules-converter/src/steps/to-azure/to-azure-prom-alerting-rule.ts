import promDurationToIso8601 from "../../utils/prom-duration-to-iso8601";
import Converter from '../../utils/converter';

const alertingRuleConverters: any = {
  for: promDurationToIso8601
}

const mapPropertyNames = {
  expr: 'expression'
}

function createExtendedPropsFunc(params: any) {
  return {
    severity: 3,
    resolveConfiguration: {
        autoResolved: true,
        timeToResolve: "PT10M"
    },
    actions: [
        {
            "actionGroupId": "[parameters('actionGroupId')]"
        }
    ]
  }
}

const toArmPrometheusRuleConverter = new Converter(alertingRuleConverters, createExtendedPropsFunc, [], mapPropertyNames);

export default toArmPrometheusRuleConverter.convert.bind(toArmPrometheusRuleConverter);
