import Ajv from 'ajv'
import addFormats from "ajv-formats"

import azurePrometheusRuleGroupSchema from '../../schemas/azure/azure-prometheus-rule-group.json'
import commonTypeSchema from '../../schemas/azure/azure-common-types.json'
const azurePromAjv = new Ajv({strictSchema: false});
addFormats(azurePromAjv);
azurePromAjv.addSchema(commonTypeSchema, 'azure-common-types.json');
export default azurePromAjv.compile(azurePrometheusRuleGroupSchema);

