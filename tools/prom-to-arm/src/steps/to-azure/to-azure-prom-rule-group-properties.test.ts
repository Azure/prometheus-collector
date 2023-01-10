import toAzurePromAlertingRuleProperties from './to-azure-prom-rule-group-properties'
import{test, expect} from '@jest/globals';

const params = {
  actionGroupId: 'actionGroupId',
  clusterName: 'clusterName',
  macResourceId: 'macResourceId'
};
test('Empty source should add the default properties', () => {
  const converted = toAzurePromAlertingRuleProperties({}, params);
  expect(converted.name).not.toBeDefined();
  expect(converted.limit).not.toBeDefined();
  expect(converted.rules).not.toBeDefined();
  expect(converted.scopes[0]).toBe("[parameters('azureMonitorWorkspace')]");
  expect(converted.clusterName).toBe("[parameters('clusterName')]");
});


test('Convert interval to ISO 8601', () => {
  const group = {
    interval: '15m'
  }
  const converted = toAzurePromAlertingRuleProperties(group, params);
  expect(converted.interval).toBe("PT15M");
});

test('Distinguish between alerting and recording rule', () => {
  const group = {
    rules: [
      {
        record: 'recordName'
      },
      {
        alert: 'alertName'
      }
    ]
  }
  const converted = toAzurePromAlertingRuleProperties(group, params);
  //recording rule
  expect(converted.rules[0].record).toBe("recordName");
  expect(Object.keys(converted.rules[0]).length).toBe(1);
  //alerting rule
  expect(converted.rules[1].alert).toBe("alertName");
  expect(converted.rules[1].severity).toBe(3);
  expect(converted.rules[1].resolveConfiguration).toEqual({
      autoResolve: true,
      timeToResolve: "PT10M"
  });
  expect(converted.rules[1].actions[0].actionGroupId).toBe("[parameters('actionGroupId')]");
});

