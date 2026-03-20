export interface DataSource {
    name: string;
    clusterUri: string;
    database: string;
    description: string;
}
export declare const DATA_SOURCES: Record<string, DataSource>;
export declare const APP_INSIGHTS: {
    appId: string;
    resourceId: string;
    apiScope: string;
};
export declare const KUSTO_SCOPE = "https://kusto.kusto.windows.net/.default";
//# sourceMappingURL=datasources.d.ts.map