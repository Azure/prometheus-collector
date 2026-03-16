// Data source configurations for the TSG dashboard Kusto clusters

export interface DataSource {
  name: string;
  clusterUri: string;
  database: string;
  description: string;
}

export const DATA_SOURCES: Record<string, DataSource> = {
  PrometheusAppInsights: {
    name: "PrometheusAppInsights",
    clusterUri:
      "https://ade.applicationinsights.io/subscriptions/13d371f9-5a39-46d5-8e1b-60158c49db84/resourceGroups/ContainerInsightsPrometheusCollector-Prod/providers/microsoft.insights/components/ContainerInsightsPrometheusCollector-Prod",
    database: "ContainerInsightsPrometheusCollector-Prod",
    description: "Collector logs, metrics, configs (App Insights)",
  },
  MetricInsights: {
    name: "MetricInsights",
    clusterUri: "https://metricsinsights.westus2.kusto.windows.net",
    database: "metricsinsightsUX",
    description: "Time series counts and ingestion rates",
  },
  AMWInfo: {
    name: "AMWInfo",
    clusterUri: "https://appinsightstlm.kusto.windows.net",
    database: "azuremonitorattach",
    description: "Azure Monitor Workspace, DCR, MDM mapping",
  },
  AKS: {
    name: "AKS",
    clusterUri: "https://akshuba.centralus.kusto.windows.net",
    database: "AKSprod",
    description: "AKS cluster state, pod restarts, settings",
  },
  "AKS CCP": {
    name: "AKS CCP",
    clusterUri: "https://akshuba.centralus.kusto.windows.net",
    database: "AKSccplogs",
    description: "Control plane metrics configuration and logs",
  },
  "AKS Infra": {
    name: "AKS Infra",
    clusterUri: "https://akshuba.centralus.kusto.windows.net",
    database: "AKSinfra",
    description: "Control plane pod CPU and container restarts",
  },
  Vulnerabilities: {
    name: "Vulnerabilities",
    clusterUri: "https://shavulnmgmtprdwus.kusto.windows.net",
    database: "ShaVulnMgmt",
    description: "Image CVE vulnerability scanning",
  },
};

// App Insights REST API config (for PrometheusAppInsights queries)
export const APP_INSIGHTS = {
  appId: "ContainerInsightsPrometheusCollector-Prod",
  resourceId:
    "/subscriptions/13d371f9-5a39-46d5-8e1b-60158c49db84/resourceGroups/ContainerInsightsPrometheusCollector-Prod/providers/microsoft.insights/components/ContainerInsightsPrometheusCollector-Prod",
  apiScope: "https://api.applicationinsights.io/.default",
};

// Kusto auth scope
export const KUSTO_SCOPE = "https://kusto.kusto.windows.net/.default";
