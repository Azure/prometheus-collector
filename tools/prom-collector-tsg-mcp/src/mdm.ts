// Geneva MDM QoS throttling checks via the Geneva MDM MCP server (HTTP)
// Queries the same metrics as the Geneva QoS dashboard: mac_91c1e6c2-bcdf-4650-9f80-179b245c2533

const MDM_MCP_URL = process.env.MDM_MCP_URL || "http://localhost:5050/mcp";

interface MdmToolResult {
  Success: boolean;
  Result: string[] | null;
  Error: string | null;
  Tool: string;
}

interface ThrottlingMetric {
  panel: string;
  metric: string;
  status: "ok" | "warning" | "error" | "no_data";
  summary: string;
  values?: { sum: number; nonNanPoints: number; totalPoints: number };
}

/**
 * Call a tool on the Geneva MDM MCP server via JSON-RPC over HTTP.
 */
async function callMdmTool(
  toolName: string,
  args: Record<string, string>
): Promise<MdmToolResult> {
  const payload = {
    jsonrpc: "2.0",
    id: Date.now(),
    method: "tools/call",
    params: { name: toolName, arguments: args },
  };

  const response = await fetch(MDM_MCP_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    throw new Error(
      `MDM MCP server returned ${response.status}: ${await response.text()}`
    );
  }

  const body = await response.json() as {
    result?: { content?: Array<{ text: string }> };
    error?: { message: string };
  };

  if (body.error) {
    throw new Error(`MDM MCP error: ${body.error.message}`);
  }

  const text = body.result?.content?.[0]?.text;
  if (!text) {
    throw new Error("No content in MDM MCP response");
  }

  return JSON.parse(text) as MdmToolResult;
}

/**
 * Parse a Sum series string like "[0,0,NaN,5,10]" into stats.
 */
function parseSumSeries(raw: string): {
  sum: number;
  max: number;
  nonNanPoints: number;
  totalPoints: number;
} {
  const match = raw.match(/Sum:\s*\[([^\]]+)\]/);
  if (!match) return { sum: 0, max: 0, nonNanPoints: 0, totalPoints: 0 };

  const vals = match[1].split(",").map((v) => v.trim());
  const totalPoints = vals.length;
  const numbers = vals
    .filter((v) => v !== "NaN")
    .map(Number)
    .filter((n) => !isNaN(n));

  return {
    sum: numbers.reduce((a, b) => a + b, 0),
    max: numbers.length > 0 ? Math.max(...numbers) : 0,
    nonNanPoints: numbers.length,
    totalPoints,
  };
}

/** The 6 QoS panel metric definitions from the Geneva dashboard. */
const QOS_METRICS = [
  {
    panel: "Incoming Events Throttled",
    metric: "ThrottledClientMetricCount",
    dims: "{}",
    isThrottleMetric: true,
  },
  {
    panel: "Incoming Events Dropped",
    metric: "DroppedClientMetricCount",
    dims: "{}",
    isThrottleMetric: true,
  },
  {
    panel: "MStore Time Series Throttled",
    metric: "ThrottledTimeSeriesCount",
    dims: "{}",
    isThrottleMetric: true,
  },
  {
    panel: "MStore Samples Dropped",
    metric: "MStoreDroppedSamplesCount",
    dims: "{}",
    isThrottleMetric: true,
  },
  {
    panel: "Client Event Volume",
    metric: "ClientAggregatedMetricCount",
    dims: "{}",
    isThrottleMetric: false,
  },
  {
    panel: "MStore Active Time Series",
    metric: "MStoreActiveTimeSeriesCount",
    dims: "{}",
    isThrottleMetric: false,
  },
  {
    panel: "Client Event Limit",
    metric: "ClientAggregatedMetricCountLimit",
    dims: "{}",
    isThrottleMetric: false,
  },
  {
    panel: "MStore Active Time Series Limit",
    metric: "MStoreActiveTimeSeriesLimit",
    dims: "{}",
    isThrottleMetric: false,
  },
  {
    panel: "Queries Throttled",
    metric: "ThrottledQueriesCount",
    dims: "{}",
    isThrottleMetric: true,
  },
];

/**
 * Check if the Geneva MDM MCP server is reachable.
 */
export async function checkMdmServerHealth(): Promise<boolean> {
  try {
    const payload = {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2024-11-05",
        capabilities: {},
        clientInfo: { name: "prom-collector-tsg", version: "1.0" },
      },
    };
    const resp = await fetch(MDM_MCP_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
      signal: AbortSignal.timeout(5000),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

/**
 * Query all QoS throttling metrics for a given MDM monitoring account.
 * Returns formatted results for each panel.
 */
export async function queryMdmThrottling(
  monitoringAccount: string,
  timeRangeHours: number = 6
): Promise<string> {
  const serverUp = await checkMdmServerHealth();
  if (!serverUp) {
    return [
      "## MDM QoS Throttling Check",
      "",
      "❌ **Geneva MDM MCP server is not running.**",
      "",
      "Start it with:",
      "```",
      "DOTNET_EnableDiagnostics=0 ASPNETCORE_URLS=\"http://localhost:5050\" ASPNETCORE_ENVIRONMENT=Local \\",
      "  nohup dotnet run --project /tmp/mdm-mcp/src/MdmMcp/GenevaMDM-MCP.csproj -- --urls \"http://localhost:5050\" &",
      "```",
      "",
      "Or set `MDM_MCP_URL` env var if running on a different port.",
    ].join("\n");
  }

  const now = new Date();
  const start = new Date(now.getTime() - timeRangeHours * 60 * 60 * 1000);
  const startTime = start.toISOString().replace(/\.\d{3}Z$/, "Z");
  const endTime = now.toISOString().replace(/\.\d{3}Z$/, "Z");

  const results: ThrottlingMetric[] = [];

  // Run all metric queries in parallel
  const promises = QOS_METRICS.map(async (def) => {
    try {
      const result = await callMdmTool("QueryDimensionMDM", {
        monitoringAccount,
        nameSpace: "MdmQos",
        metrics: def.metric,
        startTime,
        endTime,
        dimensionMapJson: def.dims,
      });

      if (!result.Success || !result.Result || result.Result.length === 0) {
        const errMsg =
          result.Result?.[0] || result.Error || "No data returned";
        return {
          panel: def.panel,
          metric: def.metric,
          status: "no_data" as const,
          summary: errMsg.includes("Error")
            ? `Query error: ${errMsg.slice(0, 100)}`
            : "No data",
        };
      }

      const raw = result.Result[0];
      const stats = parseSumSeries(raw);

      if (stats.nonNanPoints === 0) {
        return {
          panel: def.panel,
          metric: def.metric,
          status: (def.isThrottleMetric ? "ok" : "no_data") as
            | "ok"
            | "no_data",
          summary: def.isThrottleMetric
            ? "No throttling/drops detected ✅"
            : "No data (NaN)",
          values: stats,
        };
      }

      if (def.isThrottleMetric) {
        if (stats.sum === 0) {
          return {
            panel: def.panel,
            metric: def.metric,
            status: "ok" as const,
            summary: `No throttling/drops (${stats.nonNanPoints} data points, all zero) ✅`,
            values: stats,
          };
        }
        return {
          panel: def.panel,
          metric: def.metric,
          status: "warning" as const,
          summary: `⚠️ THROTTLING DETECTED: total=${stats.sum.toLocaleString()}, max=${stats.max.toLocaleString()} (${stats.nonNanPoints} points)`,
          values: stats,
        };
      }

      // Volume/limit metrics
      const avg = stats.sum / stats.nonNanPoints;
      return {
        panel: def.panel,
        metric: def.metric,
        status: "ok" as const,
        summary: `avg=${avg.toLocaleString(undefined, {
          maximumFractionDigits: 0,
        })}/min, total=${stats.sum.toLocaleString(undefined, {
          maximumFractionDigits: 0,
        })} (${stats.nonNanPoints} points)`,
        values: stats,
      };
    } catch (err) {
      return {
        panel: def.panel,
        metric: def.metric,
        status: "error" as const,
        summary: `Error: ${err instanceof Error ? err.message : String(err)}`,
      };
    }
  });

  const settled = await Promise.all(promises);
  results.push(...settled);

  // Calculate utilization percentages
  const eventUsage = results.find(
    (r) => r.metric === "ClientAggregatedMetricCount"
  );
  const eventLimit = results.find(
    (r) => r.metric === "ClientAggregatedMetricCountLimit"
  );
  const tsUsage = results.find(
    (r) => r.metric === "MStoreActiveTimeSeriesCount"
  );
  const tsLimit = results.find(
    (r) => r.metric === "MStoreActiveTimeSeriesLimit"
  );

  let utilizationLines: string[] = [];
  if (
    eventUsage?.values &&
    eventLimit?.values &&
    eventUsage.values.nonNanPoints > 0 &&
    eventLimit.values.nonNanPoints > 0
  ) {
    const usageAvg = eventUsage.values.sum / eventUsage.values.nonNanPoints;
    const limitAvg = eventLimit.values.sum / eventLimit.values.nonNanPoints;
    if (limitAvg > 0) {
      const pct = ((usageAvg / limitAvg) * 100).toFixed(1);
      utilizationLines.push(`- **Event Volume Utilization**: ${pct}%`);
    }
  }
  if (
    tsUsage?.values &&
    tsLimit?.values &&
    tsUsage.values.nonNanPoints > 0 &&
    tsLimit.values.nonNanPoints > 0
  ) {
    const usageAvg = tsUsage.values.sum / tsUsage.values.nonNanPoints;
    const limitAvg = tsLimit.values.sum / tsLimit.values.nonNanPoints;
    if (limitAvg > 0) {
      const pct = ((usageAvg / limitAvg) * 100).toFixed(1);
      utilizationLines.push(`- **MStore Time Series Utilization**: ${pct}%`);
    }
  }

  // Format output
  const lines: string[] = [
    `## MDM QoS Throttling Check`,
    `Account: **${monitoringAccount}** | Namespace: **MdmQos** | Window: last ${timeRangeHours}h`,
    "",
  ];

  // Throttle/Drop metrics first
  const throttleResults = results.filter((r) =>
    QOS_METRICS.find((m) => m.metric === r.metric)?.isThrottleMetric
  );
  const volumeResults = results.filter(
    (r) => !QOS_METRICS.find((m) => m.metric === r.metric)?.isThrottleMetric
  );

  lines.push("### Throttling & Drops");
  lines.push(
    "| Panel | Metric | Status |",
    "| --- | --- | --- |"
  );
  for (const r of throttleResults) {
    const icon =
      r.status === "ok" ? "✅" : r.status === "warning" ? "⚠️" : "ℹ️";
    lines.push(`| ${r.panel} | \`${r.metric}\` | ${icon} ${r.summary} |`);
  }

  lines.push("", "### Volume & Limits");
  lines.push(
    "| Panel | Metric | Status |",
    "| --- | --- | --- |"
  );
  for (const r of volumeResults) {
    lines.push(`| ${r.panel} | \`${r.metric}\` | ${r.summary} |`);
  }

  if (utilizationLines.length > 0) {
    lines.push("", "### Utilization");
    lines.push(...utilizationLines);
  }

  const anyThrottled = throttleResults.some((r) => r.status === "warning");
  lines.push("");
  if (anyThrottled) {
    lines.push(
      "### ⚠️ ACTION REQUIRED",
      "Throttling or drops detected. Check the [Geneva QoS Dashboard](https://portal.microsoftgeneva.com/dashboard/mac_91c1e6c2-bcdf-4650-9f80-179b245c2533/GenevaQos/%E2%86%90%20MdmQos) for details.",
      "Common causes: high cardinality metrics, metric explosion, exceeded account limits."
    );
  } else {
    lines.push("### ✅ No throttling or drops detected.");
  }

  return lines.join("\n");
}
