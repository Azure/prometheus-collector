#!/usr/bin/env node

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { ProgressNotificationSchema } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { DefaultAzureCredential } from "@azure/identity";
import { LogsQueryClient } from "@azure/monitor-query";
import { QUERIES, parameterizeQuery, type QueryCategory } from "./queries.js";
import { DATA_SOURCES, APP_INSIGHTS } from "./datasources.js";
import { queryMdmThrottling, queryScrapeTargetHealth } from "./mdm.js";
import { scrapeICMIncident } from "./icm-browser.js";
import type { ServerNotification, ServerRequest } from "@modelcontextprotocol/sdk/types.js";
import type { RequestHandlerExtra } from "@modelcontextprotocol/sdk/shared/protocol.js";

const credential = new DefaultAzureCredential();
const logsClient = new LogsQueryClient(credential);

// Common input schemas
const clusterParam = z.string().describe(
  "AKS cluster ARM resource ID, e.g. /subscriptions/.../managedClusters/name"
);
const timeRangeParam = z
  .string()
  .default("24h")
  .describe("Time range to query, e.g. 1h, 6h, 24h, 2d, 7d");
const intervalParam = z
  .string()
  .default("6h")
  .describe("Aggregation interval, e.g. 1h, 6h");
const startTimeParam = z
  .string()
  .optional()
  .describe("Absolute start time in ISO 8601 format, e.g. '2026-03-10T00:00:00Z'. When provided with endTime, overrides timeRange for precise historical queries.");
const endTimeParam = z
  .string()
  .optional()
  .describe("Absolute end time in ISO 8601 format, e.g. '2026-03-11T00:00:00Z'. When provided with startTime, overrides timeRange for precise historical queries.");

interface QueryResult {
  name: string;
  datasource: string;
  status: "success" | "error";
  data?: Record<string, unknown>[];
  error?: string;
  rowCount?: number;
}

// Query timeout in ms — generous to avoid MCP SDK timeout (-32001) on large/slow clusters
const QUERY_TIMEOUT_MS = parseInt(process.env.KQL_TIMEOUT_MS || "180000", 10); // 3 minutes

type ToolExtra = RequestHandlerExtra<ServerRequest, ServerNotification>;

/**
 * Send a progress notification to reset the client's timeout counter.
 * Only sends if the client provided a progressToken in the request.
 */
async function sendProgress(
  extra: ToolExtra,
  progress: number,
  total: number,
  message: string
): Promise<void> {
  const progressToken = extra._meta?.progressToken;
  if (progressToken === undefined) return;
  try {
    await extra.sendNotification({
      method: "notifications/progress",
      params: { progressToken, progress, total, message },
    });
  } catch {
    // Best-effort — don't let notification failures break the tool
  }
}

/**
 * Execute a KQL query against App Insights via the LogsQueryClient.
 * Supports either relative timeRange (e.g. "24h") or absolute startTime/endTime.
 */
async function runAppInsightsQuery(
  kql: string,
  timeRange: string,
  startTime?: string,
  endTime?: string
): Promise<Record<string, unknown>[]> {
  let timespan: { duration: string } | { startTime: Date; endTime: Date };

  if (startTime && endTime) {
    timespan = {
      startTime: new Date(startTime),
      endTime: new Date(endTime),
    };
  } else {
    const durationMap: Record<string, string> = {
      "1h": "PT1H",
      "2h": "PT2H",
      "6h": "PT6H",
      "12h": "PT12H",
      "24h": "PT24H",
      "1d": "PT24H",
      "2d": "P2D",
      "3d": "P3D",
      "7d": "P7D",
    };
    timespan = { duration: durationMap[timeRange] || "PT24H" };
  }

  const result = await logsClient.queryResource(
    APP_INSIGHTS.resourceId,
    kql,
    timespan,
    { serverTimeoutInSeconds: Math.floor(QUERY_TIMEOUT_MS / 1000) }
  );

  const rows: Record<string, unknown>[] = [];

  if (result.status === "Success") {
    for (const table of result.tables) {
      for (const row of table.rows) {
        const obj: Record<string, unknown> = {};
        table.columnDescriptors.forEach((col, i) => {
          obj[col.name ?? `col${i}`] = row[i];
        });
        rows.push(obj);
      }
    }
  } else if (result.status === "PartialFailure") {
    for (const table of result.partialTables) {
      for (const row of table.rows) {
        const obj: Record<string, unknown> = {};
        table.columnDescriptors.forEach((col, i) => {
          obj[col.name ?? `col${i}`] = row[i];
        });
        rows.push(obj);
      }
    }
  }

  return rows;
}

/**
 * Execute a KQL query against a Kusto cluster via REST API.
 */
async function runKustoQuery(
  clusterUri: string,
  database: string,
  kql: string
): Promise<Record<string, unknown>[]> {
  // Determine the correct scope based on the cluster URI
  let scope: string;
  if (clusterUri.includes("applicationinsights.io")) {
    scope = "https://api.applicationinsights.io/.default";
  } else {
    // Extract the cluster host for the scope
    const url = new URL(clusterUri);
    scope = `${url.protocol}//${url.host}/.default`;
  }

  const token = await credential.getToken(scope);

  const response = await fetch(`${clusterUri}/v1/rest/query`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${token.token}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      db: database,
      csl: kql,
    }),
    signal: AbortSignal.timeout(QUERY_TIMEOUT_MS),
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(`Kusto query failed (${response.status}): ${text.slice(0, 500)}`);
  }

  const body = await response.json();
  const rows: Record<string, unknown>[] = [];

  if (body.Tables && body.Tables.length > 0) {
    const table = body.Tables[0];
    const columns = table.Columns.map((c: { ColumnName: string }) => c.ColumnName);
    for (const row of table.Rows) {
      const obj: Record<string, unknown> = {};
      columns.forEach((col: string, i: number) => {
        obj[col] = row[i];
      });
      rows.push(obj);
    }
  }

  return rows;
}

/**
 * Run a single named query from the dashboard.
 */
async function executeQuery(
  queryDef: { name: string; datasource: string; kql: string },
  params: { cluster: string; timeRange: string; interval: string; mdmAccountId?: string; aksClusterId?: string; startTime?: string; endTime?: string }
): Promise<QueryResult> {
  try {
    const kql = parameterizeQuery(queryDef.kql, {
      cluster: params.cluster,
      timeRange: params.timeRange,
      interval: params.interval,
      mdmAccountId: params.mdmAccountId,
      aksClusterId: params.aksClusterId,
      startTime: params.startTime,
      endTime: params.endTime,
    });

    const ds = DATA_SOURCES[queryDef.datasource];
    if (!ds) {
      return {
        name: queryDef.name,
        datasource: queryDef.datasource,
        status: "error",
        error: `Unknown data source: ${queryDef.datasource}`,
      };
    }

    let data: Record<string, unknown>[];

    if (queryDef.datasource === "PrometheusAppInsights") {
      data = await runAppInsightsQuery(kql, params.timeRange, params.startTime, params.endTime);
    } else {
      data = await runKustoQuery(ds.clusterUri, ds.database, kql);
    }

    return {
      name: queryDef.name,
      datasource: queryDef.datasource,
      status: "success",
      data: data.slice(0, 100), // Limit rows to avoid huge responses
      rowCount: data.length,
    };
  } catch (err) {
    return {
      name: queryDef.name,
      datasource: queryDef.datasource,
      status: "error",
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

/**
 * Run all queries in a category and return combined results.
 * Sends progress notifications after each batch to keep the client timeout alive.
 */
async function runCategory(
  category: QueryCategory,
  params: { cluster: string; timeRange: string; interval: string; mdmAccountId?: string; aksClusterId?: string; startTime?: string; endTime?: string },
  extra?: ToolExtra
): Promise<QueryResult[]> {
  const queries = QUERIES[category];
  if (!queries || queries.length === 0) {
    return [];
  }

  // Run queries in parallel with concurrency limit
  const CONCURRENCY = 5;
  const results: QueryResult[] = [];
  const totalQueries = queries.length;

  for (let i = 0; i < queries.length; i += CONCURRENCY) {
    const batch = queries.slice(i, i + CONCURRENCY);
    const batchResults = await Promise.all(
      batch.map((q) => executeQuery(q, params))
    );
    results.push(...batchResults);

    // Send progress notification to reset client timeout
    if (extra) {
      const completed = Math.min(i + CONCURRENCY, totalQueries);
      await sendProgress(extra, completed, totalQueries, `${category}: ${completed}/${totalQueries} queries complete`);
    }
  }

  return results;
}

/**
 * Resolve the CCP cluster ID from the ARM resource ID.
 * Many AKS/CCP queries filter on cluster_id which is the CCP control plane ID
 * (e.g. "6604ae19e8805300010dae5e"), not the ARM resource ID.
 * This must be resolved first before running those queries.
 */
async function resolveCcpClusterId(
  cluster: string,
  timeRange: string,
  startTime?: string,
  endTime?: string
): Promise<string | undefined> {
  const ccpQuery = QUERIES.triage.find((q) => q.name === "CCP Cluster ID");
  if (!ccpQuery) return undefined;

  try {
    const result = await executeQuery(ccpQuery, {
      cluster,
      timeRange,
      interval: "6h",
      startTime,
      endTime,
    });
    if (result.status === "success" && result.data && result.data.length > 0) {
      const id = String(result.data[0].cluster_id);
      if (id && id !== "undefined" && id !== "null") return id;
    }
  } catch {
    // Silently fail — queries that need CCP ID will get errors individually
  }
  return undefined;
}

function formatResults(results: QueryResult[]): string {
  const parts: string[] = [];

  for (const r of results) {
    parts.push(`### ${r.name}`);
    parts.push(`Data Source: ${r.datasource}`);

    if (r.status === "error") {
      parts.push(`❌ Error: ${r.error}`);
    } else if (r.data && r.data.length > 0) {
      parts.push(`✅ ${r.rowCount} row(s) returned`);
      // Format as a simple table for readability
      const columns = Object.keys(r.data[0]);
      parts.push(`| ${columns.join(" | ")} |`);
      parts.push(`| ${columns.map(() => "---").join(" | ")} |`);
      for (const row of r.data.slice(0, 20)) {
        const values = columns.map((c) => {
          const v = row[c];
          if (v === null || v === undefined) return "";
          const s = String(v);
          return s.length > 100 ? s.slice(0, 97) + "..." : s;
        });
        parts.push(`| ${values.join(" | ")} |`);
      }
      if (r.data.length > 20) {
        parts.push(`... and ${r.data.length - 20} more rows`);
      }
    } else {
      parts.push("ℹ️ No data returned");
    }
    parts.push("");
  }

  return parts.join("\n");
}

// Create the MCP server
const server = new McpServer({
  name: "prom-collector-tsg",
  version: "1.0.0",
});

// Tool: tsg_triage
server.tool(
  "tsg_triage",
  "Run initial triage queries to identify cluster version, region, AMW config, token adapter health, and DCR/DCE setup. Start here for any investigation.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const aksClusterId = await resolveCcpClusterId(cluster, timeRange, startTime, endTime);
    const results = await runCategory("triage", { cluster, timeRange, interval, aksClusterId, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_errors
server.tool(
  "tsg_errors",
  "Scan all error categories: container errors, OtelCollector, MetricsExtension, MDSD, token adapter, target allocator, DNS, private link, liveness probes, and DCR/DCE config errors.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const results = await runCategory("errors", { cluster, timeRange, interval, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_config
server.tool(
  "tsg_config",
  "Check all prometheus-collector configuration: scrape configs enabled, default targets, keep list regex, scrape intervals, custom config validity, HPA, debug mode, pod monitors, service monitors.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const aksClusterId = await resolveCcpClusterId(cluster, timeRange, startTime, endTime);
    const results = await runCategory("config", { cluster, timeRange, interval, aksClusterId, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_workload
server.tool(
  "tsg_workload",
  "Check workload health: replica/pod counts, samples per minute, samples dropped, CPU/memory usage, queue sizes, export failures, target allocator errors, collector discovery.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const aksClusterId = await resolveCcpClusterId(cluster, timeRange, startTime, endTime);
    const results = await runCategory("workload", { cluster, timeRange, interval, aksClusterId, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_pods
server.tool(
  "tsg_pods",
  "Check pod health: latest pod restarts, restart counts during interval, and restart reasons for the AMA metrics addon pods.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const aksClusterId = await resolveCcpClusterId(cluster, timeRange, startTime, endTime);
    const results = await runCategory("pods", { cluster, timeRange, interval, aksClusterId, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_logs
server.tool(
  "tsg_logs",
  "Get raw logs from a specific component. Use 'component' to select: replicaset, linux-daemonset, windows-daemonset, or configreader.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
    component: z
      .enum(["replicaset", "linux-daemonset", "windows-daemonset", "configreader"])
      .default("replicaset")
      .describe("Component to get logs for"),
  },
  async ({ cluster, timeRange, interval, startTime, endTime, component }) => {
    const componentMap: Record<string, string> = {
      replicaset: "All ReplicaSet Logs",
      "linux-daemonset": "All Linux DaemonSet Logs",
      "windows-daemonset": "All Windows DaemonSet Logs",
      configreader: "All ConfigReader Logs",
    };

    const queryName = componentMap[component];
    const queries = QUERIES.logs.filter((q) => q.name === queryName);

    if (queries.length === 0) {
      return {
        content: [{ type: "text", text: `No query found for component: ${component}` }],
      };
    }

    const results = await Promise.all(
      queries.map((q) => executeQuery(q, { cluster, timeRange, interval, startTime, endTime }))
    );

    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_control_plane
server.tool(
  "tsg_control_plane",
  "Check control plane metrics: whether enabled, which jobs are running, metrics keep list, minimal ingestion profile, configmap watcher logs, and container restarts.",
  {
    cluster: clusterParam,
    timeRange: timeRangeParam,
    interval: intervalParam,
    startTime: startTimeParam,
    endTime: endTimeParam,
  },
  async ({ cluster, timeRange, interval, startTime, endTime }, extra) => {
    const aksClusterId = await resolveCcpClusterId(cluster, timeRange, startTime, endTime);
    const results = await runCategory("controlPlane", { cluster, timeRange, interval, aksClusterId, startTime, endTime }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_query
server.tool(
  "tsg_query",
  "Run an arbitrary KQL query against any of the configured data sources: PrometheusAppInsights, MetricInsights, AMWInfo, AKS, AKS CCP, AKS Infra, Vulnerabilities.",
  {
    datasource: z
      .enum([
        "PrometheusAppInsights",
        "MetricInsights",
        "AMWInfo",
        "AKS",
        "AKS CCP",
        "AKS Infra",
        "Vulnerabilities",
      ])
      .describe("Data source to query against"),
    kql: z.string().describe("KQL query to execute"),
  },
  async ({ datasource, kql }) => {
    const ds = DATA_SOURCES[datasource];
    if (!ds) {
      return {
        content: [{ type: "text", text: `Unknown data source: ${datasource}` }],
      };
    }

    try {
      let data: Record<string, unknown>[];
      if (datasource === "PrometheusAppInsights") {
        data = await runAppInsightsQuery(kql, "24h");
      } else {
        data = await runKustoQuery(ds.clusterUri, ds.database, kql);
      }

      const result: QueryResult = {
        name: "Custom Query",
        datasource,
        status: "success",
        data: data.slice(0, 100),
        rowCount: data.length,
      };

      return {
        content: [{ type: "text", text: formatResults([result]) }],
      };
    } catch (err) {
      return {
        content: [
          {
            type: "text",
            text: `❌ Query failed: ${err instanceof Error ? err.message : String(err)}`,
          },
        ],
      };
    }
  }
);

// Tool: tsg_dashboard_link
server.tool(
  "tsg_dashboard_link",
  "Generate a direct link to the TSG ADX dashboard pre-filtered for a specific cluster.",
  {
    cluster: clusterParam,
  },
  async ({ cluster }) => {
    const encoded = encodeURIComponent(cluster);
    const url = `https://dataexplorer.azure.com/dashboards/94da59c1-df12-4134-96bb-82c6b32e6199?p-_cluster=v-${encoded}`;
    return {
      content: [
        {
          type: "text",
          text: `## TSG Dashboard Link\n\n[Open Dashboard](${url})\n\n\`${url}\``,
        },
      ],
    };
  }
);

// Tool: tsg_metric_insights
server.tool(
  "tsg_metric_insights",
  "Analyze metric volume and cardinality using MDM account data. Shows top metrics by time series count, sample rate, and high-dimension cardinality. Requires the MDM account name (get it from tsg_triage → 'MDM Account ID' query). Use this to identify which metrics or jobs are causing high volume, cardinality spikes, or throttling.",
  {
    mdmAccountId: z
      .string()
      .describe(
        "MDM monitoring account name from tsg_triage, e.g. 'cirruspl_promws_at52044_neu1'"
      ),
  },
  async ({ mdmAccountId }, extra) => {
    const results = await runCategory("metricInsights", {
      cluster: "",
      timeRange: "24h",
      interval: "6h",
      mdmAccountId,
    }, extra);
    return {
      content: [{ type: "text", text: formatResults(results) }],
    };
  }
);

// Tool: tsg_mdm_throttling
server.tool(
  "tsg_mdm_throttling",
  "Check Geneva MDM QoS metrics for account throttling, dropped events, and time series limits. Queries the MdmQos namespace for: ThrottledClientMetricCount, DroppedClientMetricCount, ThrottledTimeSeriesCount, MStoreDroppedSamplesCount, ClientAggregatedMetricCount vs Limit, MStoreActiveTimeSeriesCount vs Limit, and ThrottledQueriesCount. Requires the Geneva MDM MCP server running on localhost:5050.",
  {
    monitoringAccount: z
      .string()
      .describe(
        "Geneva MDM monitoring account name, e.g. 'GenevaQos'. This is the account whose QoS metrics will be checked."
      ),
    timeRangeHours: z
      .number()
      .default(6)
      .describe("Number of hours to look back (default: 6)"),
  },
  async ({ monitoringAccount, timeRangeHours }) => {
    const text = await queryMdmThrottling(monitoringAccount, timeRangeHours);
    return {
      content: [{ type: "text", text }],
    };
  }
);

// Tool: tsg_scrape_health
server.tool(
  "tsg_scrape_health",
  "Check scrape target health by querying the `up`, `scrape_samples_scraped`, and `scrape_samples_post_metric_relabeling` metrics from Geneva MDM. When called with a specific job, shows detailed per-bucket success/failure analysis and relabeling drop rate. When called without a job, probes all common scrape targets and returns a per-job summary table. Always requires a cluster name to filter to a specific cluster. Requires the Geneva MDM MCP server running on localhost:5050. Use the MDMAccountName from tsg_triage results.",
  {
    monitoringAccount: z
      .string()
      .describe(
        "Geneva MDM monitoring account name (from tsg_triage 'MDM Account ID' result), e.g. 'mac_0d8947c8-888e-497d-b762-3296a8cf265a'"
      ),
    job: z
      .string()
      .default("")
      .describe(
        "Scrape job name to check, e.g. 'kube-state-metrics', 'kubelet', 'node', 'cadvisor'. If omitted, probes all common jobs and shows a per-job summary."
      ),
    cluster: z
      .string()
      .describe(
        "Cluster name (the 'cluster' label in MDM, typically the AKS cluster short name). Required because MDM accounts serve multiple clusters."
      ),
    timeRangeHours: z
      .number()
      .default(24)
      .describe("Number of hours to look back (default: 24)"),
  },
  async ({ monitoringAccount, job, cluster, timeRangeHours }) => {
    const text = await queryScrapeTargetHealth(
      monitoringAccount,
      job,
      cluster,
      timeRangeHours
    );
    return {
      content: [{ type: "text", text }],
    };
  }
);

server.tool(
  "tsg_icm_page",
  "Scrape an ICM incident page via Edge browser CDP connection. Works on both Windows (native) and WSL2. Opens the incident in Edge (or finds an already-open tab) and extracts the authored summary, discussion entries, and ARM resource IDs. On Windows, connects to localhost:9222. On WSL2, connects via port proxy on 9223. Requires Edge running with --remote-debugging-port=9222. Use this to get ICM details not available via the ICM API (authored summary text, discussion content, ARM resource IDs mentioned in descriptions).",
  {
    incidentId: z
      .number()
      .describe("The ICM incident ID to scrape, e.g. 749876123"),
  },
  async ({ incidentId }) => {
    const text = await scrapeICMIncident(incidentId);
    return {
      content: [{ type: "text", text }],
    };
  }
);

// Start the server
async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("prom-collector-tsg-mcp server started");
}

main().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
