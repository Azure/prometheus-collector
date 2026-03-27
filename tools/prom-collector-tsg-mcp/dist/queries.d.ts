export interface Query {
    name: string;
    datasource: string;
    kql: string;
}
export type QueryCategory = "triage" | "errors" | "config" | "workload" | "pods" | "logs" | "controlPlane" | "metricInsights" | "armInvestigation";
export declare const QUERIES: Record<QueryCategory, Query[]>;
/**
 * Replace dashboard parameters in a KQL query with actual values.
 */
export declare function parameterizeQuery(kql: string, params: {
    cluster: string;
    timeRange?: string;
    interval?: string;
    mdmAccountId?: string;
    aksClusterId?: string;
    startTime?: string;
    endTime?: string;
}): string;
//# sourceMappingURL=queries.d.ts.map