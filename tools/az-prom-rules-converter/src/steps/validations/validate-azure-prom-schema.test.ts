import validateAzurePrometheusRuleGroup from './validate-azure-prom-schema'

test('validateAzurePromSchema', () => {
  const result = validateAzurePrometheusRuleGroup(azurePromRuleGroup);

  // console.log(result, validateAzurePrometheusRuleGroup.errors);
});

const azurePromRuleGroup = {
  location: "location",
  properties: {
    interval: "PT15M",
    scopes: [
      "/subscriptions/asd/resourcegroups/dsfsdf/providers/microsoft.monitor/accounts/asdf"
    ],
    rules: [
      {
        "record": "asd",
        "expression": "asdas"
      }
    ]
  }
};