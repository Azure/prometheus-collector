import { AlertingRule } from '../../types/prometheus-rules';
import toAzurePromAlertingRule from './to-azure-prom-alerting-rule'
import{test, expect} from '@jest/globals';


test('Empty source should add the default properties', () => {
  const converted = toAzurePromAlertingRule(({} as AlertingRule), {actionGroupId: 'actionGroupId'});
  expect(converted.severity).toBe(3);
  expect(converted.resolveConfiguration).toEqual({
      autoResolve: true,
      timeToResolve: "PT10M"
  });
  expect(converted.actions[0].actionGroupId).toBe("[parameters('actionGroupId')]");
});


test('Convert for to ISO 8601', () => {
  const rule = {
    for: '1h2m'
  }
  const converted = toAzurePromAlertingRule((rule as AlertingRule), {actionGroupId: 'actionGroupId'});
  expect(converted.for).toBe("PT1H2M");
});