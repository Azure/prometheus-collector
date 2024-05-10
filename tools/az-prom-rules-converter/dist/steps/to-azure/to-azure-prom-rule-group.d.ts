import { PrometheusRulesGroup } from "../../types/prometheus-rules";
export default function toAzurePromRuleGroup(group: PrometheusRulesGroup, params: any): {
    name: string;
    type: string;
    apiVersion: string;
    location: string;
    properties: any;
};
