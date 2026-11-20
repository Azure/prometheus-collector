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

/**
 * Call a KQLM query on the Geneva MDM MCP server.
 * Returns parsed results with dimensions and sampling type values.
 */
interface KqlmResult {
  Status: string;
  Query: string;
  Results: Array<{
    Dimensions: Record<string, string>;
    SamplingTypes: Record<string, number[]>;
  }>;
  Messages?: string[];
}

async function callKqlm(
  monitoringAccount: string,
  metricNamespace: string,
  query: string,
  startTime: string,
  endTime: string
): Promise<KqlmResult | null> {
  const result = await callMdmTool("KqlmQuery", {
    monitoringAccount,
    metricNamespace,
    startTime,
    endTime,
    finalKqlmQuery: query,
  });

  if (!result.Success || !result.Result || result.Result.length === 0) {
    return null;
  }

  // KqlmQuery returns the result as a JSON string inside Result[0] (via tool response)
  // But callMdmTool already parsed the outer wrapper, so Result is string or string[]
  const raw = Array.isArray(result.Result) ? result.Result[0] : (result.Result as unknown as string);
  try {
    return JSON.parse(raw) as KqlmResult;
  } catch {
    return null;
  }
}

/**
 * Extract the last value from a KQLM Sum result series.
 */
function kqlmLastValue(kqlm: KqlmResult, seriesIndex: number = 0): number | null {
  if (!kqlm.Results || kqlm.Results.length <= seriesIndex) return null;
  const vals = kqlm.Results[seriesIndex].SamplingTypes?.Sum;
  if (!vals || vals.length === 0) return null;
  return vals[vals.length - 1];
}

/**
 * Sum all values in a KQLM Sum result series (excluding NaN).
 */
function kqlmTotalValue(kqlm: KqlmResult, seriesIndex: number = 0): number {
  if (!kqlm.Results || kqlm.Results.length <= seriesIndex) return 0;
  const vals = kqlm.Results[seriesIndex].SamplingTypes?.Sum;
  if (!vals || vals.length === 0) return 0;
  return vals.filter((v) => !isNaN(v)).reduce((a, b) => a + b, 0);
}

/** The QoS panel metric definitions from the Geneva dashboard. */
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
 * Uses KQLM queries which correctly handle MdmQos sampling types.
 * Also includes per-namespace time series breakdown.
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

  // Query all metrics via KQLM in parallel
  const promises = QOS_METRICS.map(async (def) => {
    try {
      const kqlmQuery = `metric("${def.metric}").samplingTypes("Sum")`;
      const kqlmResult = await callKqlm(monitoringAccount, "MdmQos", kqlmQuery, startTime, endTime);

      if (!kqlmResult || kqlmResult.Status !== "Success" || !kqlmResult.Results || kqlmResult.Results.length === 0) {
        return {
          panel: def.panel,
          metric: def.metric,
          status: "no_data" as const,
          summary: "No data",
        };
      }

      const vals = kqlmResult.Results[0].SamplingTypes?.Sum;
      if (!vals || vals.length === 0) {
        return {
          panel: def.panel,
          metric: def.metric,
          status: "no_data" as const,
          summary: "No data",
        };
      }

      const nonNanVals = vals.filter((v) => !isNaN(v));
      if (nonNanVals.length === 0) {
        return {
          panel: def.panel,
          metric: def.metric,
          status: (def.isThrottleMetric ? "ok" : "no_data") as "ok" | "no_data",
          summary: def.isThrottleMetric ? "No throttling/drops detected ✅" : "No data (NaN)",
          values: { sum: 0, max: 0, nonNanPoints: 0, totalPoints: vals.length },
        };
      }

      const sum = nonNanVals.reduce((a, b) => a + b, 0);
      const max = Math.max(...nonNanVals);

      if (def.isThrottleMetric) {
        if (sum === 0) {
          return {
            panel: def.panel,
            metric: def.metric,
            status: "ok" as const,
            summary: `No throttling/drops (${nonNanVals.length} data points, all zero) ✅`,
            values: { sum: 0, max: 0, nonNanPoints: nonNanVals.length, totalPoints: vals.length },
          };
        }
        return {
          panel: def.panel,
          metric: def.metric,
          status: "warning" as const,
          summary: `⚠️ THROTTLING DETECTED: total=${sum.toLocaleString()}, max=${max.toLocaleString()} (${nonNanVals.length} points)`,
          values: { sum, max, nonNanPoints: nonNanVals.length, totalPoints: vals.length },
        };
      }

      // Volume/limit metrics
      const avg = sum / nonNanVals.length;
      const latest = nonNanVals[nonNanVals.length - 1];
      return {
        panel: def.panel,
        metric: def.metric,
        status: "ok" as const,
        summary: `latest=${latest.toLocaleString(undefined, { maximumFractionDigits: 0 })}, avg=${avg.toLocaleString(undefined, { maximumFractionDigits: 0 })}/min (${nonNanVals.length} points)`,
        values: { sum, max, nonNanPoints: nonNanVals.length, totalPoints: vals.length },
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
  const eventUsage = results.find((r) => r.metric === "ClientAggregatedMetricCount");
  const eventLimit = results.find((r) => r.metric === "ClientAggregatedMetricCountLimit");
  const tsUsage = results.find((r) => r.metric === "MStoreActiveTimeSeriesCount");
  const tsLimit = results.find((r) => r.metric === "MStoreActiveTimeSeriesLimit");

  let utilizationLines: string[] = [];
  if (
    eventUsage?.values && eventLimit?.values &&
    eventUsage.values.nonNanPoints > 0 && eventLimit.values.nonNanPoints > 0
  ) {
    const usageAvg = eventUsage.values.sum / eventUsage.values.nonNanPoints;
    const limitAvg = eventLimit.values.sum / eventLimit.values.nonNanPoints;
    if (limitAvg > 0) {
      const pct = ((usageAvg / limitAvg) * 100).toFixed(1);
      utilizationLines.push(`- **Event Volume Utilization**: ${pct}%`);
    }
  }
  if (
    tsUsage?.values && tsLimit?.values &&
    tsUsage.values.nonNanPoints > 0 && tsLimit.values.nonNanPoints > 0
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
    const icon = r.status === "ok" ? "✅" : r.status === "warning" ? "⚠️" : "ℹ️";
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

  // Per-namespace time series breakdown
  try {
    const nsQuery = `metric("MStoreActiveTimeSeriesCount").dimensions("MetricNamespace").samplingTypes("Sum")`;
    const nsResult = await callKqlm(monitoringAccount, "MdmQos", nsQuery, startTime, endTime);
    if (nsResult && nsResult.Status === "Success" && nsResult.Results && nsResult.Results.length > 0) {
      const nsSummaries: { ns: string; latest: number }[] = [];
      for (const series of nsResult.Results) {
        const ns = series.Dimensions?.MetricNamespace || "unknown";
        const vals = series.SamplingTypes?.Sum;
        if (vals && vals.length > 0) {
          const nonNan = vals.filter((v) => !isNaN(v));
          const latest = nonNan.length > 0 ? nonNan[nonNan.length - 1] : 0;
          nsSummaries.push({ ns, latest });
        }
      }
      if (nsSummaries.length > 0) {
        nsSummaries.sort((a, b) => b.latest - a.latest);
        lines.push(
          "",
          "### Time Series by Namespace",
          "",
          "| Namespace | Active Time Series |",
          "|-----------|-------------------|"
        );
        for (const s of nsSummaries) {
          lines.push(`| \`${s.ns}\` | ${s.latest.toLocaleString()} |`);
        }
      }
    }
  } catch {
    // Non-critical, skip
  }

  // Reason drill-down for drop/throttle metrics that have warnings
  const dropMetricsWithReasons: { metric: string; dimName: string; label: string }[] = [
    { metric: "MStoreDroppedSamplesCount", dimName: "Reason", label: "MStore Dropped Samples" },
    { metric: "DroppedClientMetricCount", dimName: "Reason", label: "Dropped Client Metrics" },
    { metric: "ThrottledClientMetricCount", dimName: "Reason", label: "Throttled Client Metrics" },
    { metric: "ThrottledTimeSeriesCount", dimName: "Reason", label: "Throttled Time Series" },
  ];

  const reasonSections: string[] = [];
  const reasonPromises = dropMetricsWithReasons.map(async (dm) => {
    const metricResult = results.find((r) => r.metric === dm.metric);
    if (!metricResult || metricResult.status !== "warning") return null;

    try {
      const reasonQuery = `metric("${dm.metric}").dimensions("${dm.dimName}").samplingTypes("Sum")`;
      const reasonResult = await callKqlm(monitoringAccount, "MdmQos", reasonQuery, startTime, endTime);
      if (!reasonResult || reasonResult.Status !== "Success" || !reasonResult.Results || reasonResult.Results.length === 0) {
        return null;
      }

      const rows: { reason: string; total: number; latest: number }[] = [];
      for (const series of reasonResult.Results) {
        const reason = series.Dimensions?.[dm.dimName] || "unknown";
        const vals = series.SamplingTypes?.Sum;
        if (!vals) continue;
        const nonNan = vals.filter((v) => !isNaN(v));
        if (nonNan.length === 0) continue;
        const total = nonNan.reduce((a, b) => a + b, 0);
        const latest = nonNan[nonNan.length - 1];
        rows.push({ reason, total, latest });
      }

      if (rows.length === 0) return null;
      rows.sort((a, b) => b.total - a.total);

      const section = [
        `**${dm.label}** by Reason:`,
        "",
        "| Reason | Total (period) | Latest |",
        "|--------|---------------|--------|",
        ...rows.map((r) =>
          `| \`${r.reason}\` | ${r.total.toLocaleString()} | ${r.latest.toLocaleString()} |`
        ),
      ];
      return section.join("\n");
    } catch {
      return null;
    }
  });

  const reasonResults = await Promise.all(reasonPromises);
  for (const section of reasonResults) {
    if (section) reasonSections.push(section);
  }

  if (reasonSections.length > 0) {
    lines.push("", "### Drop/Throttle Reasons", "");
    lines.push(reasonSections.join("\n\n"));
  }

  const anyThrottled = throttleResults.some((r) => r.status === "warning");
  lines.push("");
  if (anyThrottled) {
    lines.push(
      "### ⚠️ ACTION REQUIRED",
      "Throttling or drops detected. Check the [Geneva QoS Dashboard](https://portal.microsoftgeneva.com/dashboard/mac_91c1e6c2-bcdf-4650-9f80-179b245c2533/GenevaQos/%E2%86%90%20MdmQos) for details.",
      "",
      "**Common drop reasons:**",
      "- `Duplicated` — MStore rejected samples as duplicates (same metric+labels+timestamp). Often caused by multiple collectors scraping the same target or HPA rebalancing during scale events",
      "- `NotSupportedDoubleValue` — Metric value is NaN/Inf/subnormal, which MDM cannot store",
      "- `ThrottledByQuota` — Account quota exceeded",
      "- `DimensionLimitExceeded` — Too many dimensions on the metric (limit: 64 for Prometheus accounts)",
      "- `TimeSeriesLimitExceeded` — Too many unique time series in the account",
    );
  } else {
    lines.push("### ✅ No throttling or drops detected.");
  }

  return lines.join("\n");
}

/**
 * Parse Sum and Count arrays from an MDM query result string.
 * Returns arrays of numbers (NaN for "NaN" entries).
 */
function parseSumCount(raw: string): { sums: number[]; counts: number[] } | null {
  const sumMatch = raw.match(/Sum:\s*\[([^\]]+)\]/);
  const countMatch = raw.match(/Count:\s*\[([^\]]+)\]/);
  if (!sumMatch || !countMatch) return null;
  const sums = sumMatch[1].split(",").map((s) => parseFloat(s.trim()));
  const counts = countMatch[1].split(",").map((s) => parseFloat(s.trim()));
  return { sums, counts };
}

/** Analyze parsed up metric data for a single job. */
interface UpAnalysis {
  totalScrapes: number;
  totalUp: number;
  healthy: number;
  degraded: number;
  nanBuckets: number;
  activeBuckets: number;
  successRate: string;
  failureBuckets: { index: number; sum: number; count: number }[];
}

function analyzeUpMetric(parsed: { sums: number[]; counts: number[] }): UpAnalysis {
  let healthy = 0, degraded = 0, nanBuckets = 0, totalScrapes = 0, totalUp = 0;
  const failureBuckets: { index: number; sum: number; count: number }[] = [];

  for (let i = 0; i < parsed.sums.length; i++) {
    const s = parsed.sums[i];
    const c = parsed.counts[i];
    if (isNaN(s) || isNaN(c)) { nanBuckets++; continue; }
    totalScrapes += c;
    totalUp += s;
    if (s === c) {
      healthy++;
    } else {
      degraded++;
      failureBuckets.push({ index: i, sum: s, count: c });
    }
  }

  const activeBuckets = healthy + degraded;
  const successRate = totalScrapes > 0 ? ((totalUp / totalScrapes) * 100).toFixed(2) : "N/A";
  return { totalScrapes, totalUp, healthy, degraded, nanBuckets, activeBuckets, successRate, failureBuckets };
}

/** Compute min/max/avg/latest stats and detect >10% bucket-to-bucket changes. */
function computeSampleStats(parsed: { sums: number[] }) {
  const vals = parsed.sums.filter((v) => !isNaN(v));
  if (vals.length === 0) return null;
  const min = Math.min(...vals);
  const max = Math.max(...vals);
  const avg = vals.reduce((a, b) => a + b, 0) / vals.length;
  const latest = vals[vals.length - 1];
  const deviations: { index: number; value: number; pctChange: number }[] = [];
  for (let i = 1; i < parsed.sums.length; i++) {
    const cur = parsed.sums[i];
    const prev = parsed.sums[i - 1];
    if (isNaN(cur) || isNaN(prev) || prev === 0) continue;
    const pctChange = ((cur - prev) / prev) * 100;
    if (Math.abs(pctChange) > 10) {
      deviations.push({ index: i, value: cur, pctChange });
    }
  }
  return { min, max, avg, latest, count: vals.length, deviations };
}

// Common Prometheus collector scrape jobs to probe in multi-job mode.
const DEFAULT_JOBS = [
  "kube-state-metrics",
  "kubelet",
  "cadvisor",
  "node",
  "kube-proxy",
  "kube-apiserver",
  "controlplane-apiserver",
  "controlplane-etcd",
  "controlplane-kube-scheduler",
  "controlplane-kube-controller-manager",
  "controlplane-cluster-autoscaler",
  "prometheus_collector_health",
  "kube-dns",
  "windows-exporter",
  "kube-proxy-windows",
  "networkobservability-hubble",
  "networkobservability-retina",
  "networkobservability-cilium",
];

/**
 * Query scrape health metrics from Geneva MDM for a cluster.
 *
 * **Single-job mode** (job provided): Queries `up`, `scrape_samples_scraped`,
 * and `scrape_samples_post_metric_relabeling` for the specified job. Shows
 * detailed per-bucket failure analysis and relabeling drop rate.
 *
 * **Multi-job mode** (job omitted/empty): Probes the `up` metric for a set of
 * common scrape jobs in parallel. Returns a per-job summary table showing which
 * targets are healthy, degraded, or missing.
 */
export async function queryScrapeTargetHealth(
  monitoringAccount: string,
  job: string,
  cluster: string,
  timeRangeHours: number = 24
): Promise<string> {
  const serverUp = await checkMdmServerHealth();
  if (!serverUp) {
    return [
      "## Scrape Target Health Check",
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

  if (!job) {
    return queryMultiJobHealth(monitoringAccount, cluster, timeRangeHours, startTime, endTime);
  }
  return querySingleJobHealth(monitoringAccount, job, cluster, timeRangeHours, startTime, endTime);
}

/**
 * Multi-job mode: probe `up` for all default jobs in parallel, return summary.
 */
async function queryMultiJobHealth(
  monitoringAccount: string,
  cluster: string,
  timeRangeHours: number,
  startTime: string,
  endTime: string
): Promise<string> {
  const results = await Promise.all(
    DEFAULT_JOBS.map(async (j) => {
      try {
        const r = await callMdmTool("QueryDimensionMDM", {
          monitoringAccount,
          nameSpace: "customdefault",
          metrics: "up",
          startTime,
          endTime,
          dimensionMapJson: JSON.stringify({ job: [j], cluster: [cluster] }),
        });
        return { job: j, result: r };
      } catch {
        return { job: j, result: null };
      }
    })
  );

  interface JobSummary {
    job: string;
    status: "✅ Healthy" | "⚠️ Degraded" | "❌ Down" | "— No data";
    successRate: string;
    totalScrapes: number;
    failures: number;
    samplesScraped: string;
    samplesPostRelabel: string;
  }

  // For jobs that have up data, also query sample metrics in parallel
  const jobsWithData: string[] = [];
  const upAnalyses: Map<string, UpAnalysis> = new Map();

  for (const { job: j, result: r } of results) {
    if (!r || !r.Success || !r.Result || r.Result.length === 0) continue;
    const parsed = parseSumCount(r.Result.join("\n"));
    if (!parsed) continue;
    const analysis = analyzeUpMetric(parsed);
    if (analysis.totalScrapes > 0) {
      jobsWithData.push(j);
      upAnalyses.set(j, analysis);
    }
  }

  // Query scrape_samples_scraped and scrape_samples_post_metric_relabeling for jobs with data
  const sampleResults = await Promise.all(
    jobsWithData.flatMap((j) => [
      callMdmTool("QueryDimensionMDM", {
        monitoringAccount,
        nameSpace: "customdefault",
        metrics: "scrape_samples_scraped",
        startTime,
        endTime,
        dimensionMapJson: JSON.stringify({ job: [j], cluster: [cluster] }),
      }).then((r) => ({ job: j, metric: "scraped" as const, result: r }))
       .catch(() => ({ job: j, metric: "scraped" as const, result: null as MdmToolResult | null })),
      callMdmTool("QueryDimensionMDM", {
        monitoringAccount,
        nameSpace: "customdefault",
        metrics: "scrape_samples_post_metric_relabeling",
        startTime,
        endTime,
        dimensionMapJson: JSON.stringify({ job: [j], cluster: [cluster] }),
      }).then((r) => ({ job: j, metric: "postRelabel" as const, result: r }))
       .catch(() => ({ job: j, metric: "postRelabel" as const, result: null as MdmToolResult | null })),
    ])
  );

  const scrapedAvgs: Map<string, number> = new Map();
  const postRelabelAvgs: Map<string, number> = new Map();
  for (const { job: j, metric, result: r } of sampleResults) {
    if (!r || !r.Success || !r.Result || r.Result.length === 0) continue;
    const parsed = parseSumCount(r.Result.join("\n"));
    if (!parsed) continue;
    const stats = computeSampleStats(parsed);
    if (!stats) continue;
    if (metric === "scraped") {
      scrapedAvgs.set(j, stats.avg);
    } else {
      postRelabelAvgs.set(j, stats.avg);
    }
  }

  const summaries: JobSummary[] = [];
  for (const { job: j, result: r } of results) {
    if (!r || !r.Success || !r.Result || r.Result.length === 0) {
      // Check if the query errored vs simply no data
      summaries.push({ job: j, status: "— No data", successRate: "—", totalScrapes: 0, failures: 0, samplesScraped: "—", samplesPostRelabel: "—" });
      continue;
    }
    const analysis = upAnalyses.get(j);
    if (!analysis || analysis.totalScrapes === 0) {
      summaries.push({ job: j, status: "— No data", successRate: "—", totalScrapes: 0, failures: 0, samplesScraped: "—", samplesPostRelabel: "—" });
      continue;
    }
    const failures = analysis.totalScrapes - analysis.totalUp;
    let status: JobSummary["status"];
    if (failures === 0) {
      status = "✅ Healthy";
    } else if (parseFloat(analysis.successRate) > 95) {
      status = "⚠️ Degraded";
    } else {
      status = "❌ Down";
    }

    const scraped = scrapedAvgs.has(j) ? scrapedAvgs.get(j)!.toFixed(0) : "—";
    const postRelabel = postRelabelAvgs.has(j) ? postRelabelAvgs.get(j)!.toFixed(0) : "—";

    summaries.push({ job: j, status, successRate: analysis.successRate + "%", totalScrapes: analysis.totalScrapes, failures, samplesScraped: scraped, samplesPostRelabel: postRelabel });
  }

  // Filter to only jobs with data, plus any degraded/down
  const active = summaries.filter((s) => s.status !== "— No data");
  const missing = summaries.filter((s) => s.status === "— No data");

  const lines: string[] = [
    "## Scrape Target Health — All Jobs",
    "",
    "| Field | Value |",
    "|-------|-------|",
    `| **Cluster** | \`${cluster}\` |`,
    `| **MDM Account** | \`${monitoringAccount}\` |`,
    `| **Time Range** | ${timeRangeHours}h (${startTime} → ${endTime}) |`,
    `| **Jobs found** | ${active.length} |`,
    "",
  ];

  if (active.length === 0) {
    lines.push("⚠️ No scrape data found for any default job on this cluster.");
    lines.push("", "Jobs probed: " + DEFAULT_JOBS.map((j) => `\`${j}\``).join(", "));
    return lines.join("\n");
  }

  lines.push(
    "### Per-Job Health Summary",
    "",
    "| Job | Status | Success Rate | Scrapes | Failures | Samples Scraped (avg) | After Relabeling (avg) |",
    "|-----|--------|--------------|---------|----------|-----------------------|------------------------|"
  );

  for (const s of active) {
    lines.push(
      `| \`${s.job}\` | ${s.status} | ${s.successRate} | ${s.totalScrapes} | ${s.failures} | ${s.samplesScraped} | ${s.samplesPostRelabel} |`
    );
  }

  // Highlight issues
  const degraded = active.filter((s) => s.status !== "✅ Healthy");
  if (degraded.length > 0) {
    lines.push(
      "",
      "### ⚠️ Jobs with Issues",
      "",
      "Run `tsg_scrape_health` with a specific `job` parameter for detailed per-bucket failure analysis:",
      ""
    );
    for (const s of degraded) {
      lines.push(`- **\`${s.job}\`** — ${s.status} (${s.successRate}, ${s.failures} failures)`);
    }
  } else {
    lines.push("", "### ✅ All scrape targets are healthy.");
  }

  // Show relabeling summary for jobs that have both metrics
  const relabelRows: string[] = [];
  for (const s of active) {
    const scraped = scrapedAvgs.get(s.job);
    const postRelabel = postRelabelAvgs.get(s.job);
    if (scraped && postRelabel && scraped > 0) {
      const dropped = scraped - postRelabel;
      const dropPct = ((dropped / scraped) * 100).toFixed(1);
      if (dropped > 0) {
        relabelRows.push(`| \`${s.job}\` | ${scraped.toFixed(0)} | ${postRelabel.toFixed(0)} | ${dropped.toFixed(0)} | ${dropPct}% |`);
      }
    }
  }
  if (relabelRows.length > 0) {
    lines.push(
      "",
      "### Metric Relabeling Drop Rates",
      "",
      "| Job | Scraped (avg) | After Relabeling (avg) | Dropped | Drop % |",
      "|-----|---------------|------------------------|---------|--------|",
      ...relabelRows
    );
  }

  if (missing.length > 0 && missing.length < DEFAULT_JOBS.length) {
    lines.push(
      "",
      `<details><summary>${missing.length} jobs with no data (not active on this cluster)</summary>`,
      "",
      missing.map((s) => `\`${s.job}\``).join(", "),
      "",
      "</details>"
    );
  }

  return lines.join("\n");
}

/**
 * Single-job mode: detailed analysis with up + samples_scraped + post_relabeling.
 */
async function querySingleJobHealth(
  monitoringAccount: string,
  job: string,
  cluster: string,
  timeRangeHours: number,
  startTime: string,
  endTime: string
): Promise<string> {
  const dimensionMapJson = JSON.stringify({ job: [job], cluster: [cluster] });
  const baseArgs = { monitoringAccount, nameSpace: "customdefault", startTime, endTime, dimensionMapJson };

  try {
    const [upResult, scrapedResult, postRelabelResult] = await Promise.all([
      callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "up" }),
      callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "scrape_samples_scraped" }),
      callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "scrape_samples_post_metric_relabeling" }),
    ]);

    const noData = (r: MdmToolResult) => !r.Success || !r.Result || r.Result.length === 0;

    if (noData(upResult)) {
      return [
        "## Scrape Target Health Check",
        "",
        `**Job:** \`${job}\``,
        `**Cluster:** \`${cluster}\``,
        `**MDM Account:** \`${monitoringAccount}\``,
        `**Time Range:** ${timeRangeHours}h`,
        "",
        "⚠️ No data returned for the `up` metric. Possible causes:",
        "- The job name doesn't match any scrape target",
        "- The cluster name doesn't match the `cluster` label in MDM",
        "- The monitoring account is incorrect",
        "- The metric hasn't been ingested in this time range",
      ].join("\n");
    }

    const upRaw = upResult.Result!.join("\n");
    const upParsed = parseSumCount(upRaw);
    if (!upParsed) {
      return [
        "## Scrape Target Health Check",
        "",
        `**Job:** \`${job}\` | **Cluster:** \`${cluster}\``,
        "",
        "⚠️ Could not parse Sum/Count from MDM response.",
        "",
        "Raw response (first 500 chars):",
        "```",
        upRaw.substring(0, 500),
        "```",
      ].join("\n");
    }

    const analysis = analyzeUpMetric(upParsed);
    const { totalScrapes, totalUp, healthy, degraded, nanBuckets, activeBuckets, successRate, failureBuckets } = analysis;
    const healthPct = activeBuckets > 0 ? ((healthy / activeBuckets) * 100).toFixed(1) : "N/A";
    const totalMinutes = timeRangeHours * 60;
    const bucketSizeMin = activeBuckets > 0 ? (totalMinutes / (activeBuckets + nanBuckets)).toFixed(1) : "?";

    const lines: string[] = [
      "## Scrape Target Health Check",
      "",
      "| Field | Value |",
      "|-------|-------|",
      `| **Job** | \`${job}\` |`,
      `| **Cluster** | \`${cluster}\` |`,
      `| **MDM Account** | \`${monitoringAccount}\` |`,
      `| **Time Range** | ${timeRangeHours}h (${startTime} → ${endTime}) |`,
      `| **Bucket Resolution** | ~${bucketSizeMin} min |`,
      "",
      "### Scrape Up/Down Status",
      "",
      "| Metric | Value |",
      "|--------|-------|",
      `| Total scrape events | ${totalScrapes} |`,
      `| Successful (up=1) | ${totalUp} |`,
      `| Failed (up=0) | ${totalScrapes - totalUp} |`,
      `| **Scrape success rate** | **${successRate}%** |`,
      `| Healthy buckets (all up=1) | ${healthy} / ${activeBuckets} (${healthPct}%) |`,
      `| Degraded buckets (some up=0) | ${degraded} |`,
      `| No-data buckets (NaN) | ${nanBuckets} |`,
    ];

    if (degraded === 0) {
      lines.push("", `✅ Target \`${job}\` is fully healthy — no scrape failures in ${timeRangeHours}h.`);
    } else {
      lines.push("", `⚠️ Target \`${job}\` has scrape failures`);
      if (failureBuckets.length <= 20) {
        lines.push("", "| Bucket | up=1 | up=0 | Total |", "|--------|------|------|-------|");
        for (const fb of failureBuckets) {
          lines.push(`| ${fb.index} | ${fb.sum} | ${fb.count - fb.sum} | ${fb.count} |`);
        }
      } else {
        const gaps: number[] = [];
        for (let i = 1; i < Math.min(failureBuckets.length, 30); i++) {
          gaps.push(failureBuckets[i].index - failureBuckets[i - 1].index);
        }
        const avgGap = gaps.length > 0 ? (gaps.reduce((a, b) => a + b, 0) / gaps.length).toFixed(0) : "?";
        const isRegular = gaps.length > 3 && gaps.every((g) => Math.abs(g - gaps[0]) <= 1);
        lines.push(
          "",
          `- **${failureBuckets.length}** buckets with failures over ${timeRangeHours}h`,
          `- Failure spacing: ${isRegular ? `regular, every ~${avgGap} buckets (~${(parseFloat(avgGap) * parseFloat(bucketSizeMin)).toFixed(0)} min)` : `irregular (avg ${avgGap} buckets apart)`}`,
          `- Typical failure: Sum=${failureBuckets[0].sum}, Count=${failureBuckets[0].count} (${failureBuckets[0].count - failureBuckets[0].sum} scrapes returned up=0)`
        );
      }
    }

    // --- Scrape Samples Analysis ---
    lines.push("", "### Scrape Samples Analysis");

    const scrapedParsed = noData(scrapedResult) ? null : parseSumCount(scrapedResult.Result!.join("\n"));
    const postRelabelParsed = noData(postRelabelResult) ? null : parseSumCount(postRelabelResult.Result!.join("\n"));

    if (!scrapedParsed && !postRelabelParsed) {
      lines.push("", "⚠️ No data for `scrape_samples_scraped` or `scrape_samples_post_metric_relabeling`.");
    } else {
      const scrapedStats = scrapedParsed ? computeSampleStats(scrapedParsed) : null;
      const postRelabelStats = postRelabelParsed ? computeSampleStats(postRelabelParsed) : null;

      lines.push(
        "",
        "| Metric | Min | Max | Avg | Latest |",
        "|--------|-----|-----|-----|--------|"
      );

      if (scrapedStats) {
        lines.push(
          `| \`scrape_samples_scraped\` | ${scrapedStats.min} | ${scrapedStats.max} | ${scrapedStats.avg.toFixed(0)} | ${scrapedStats.latest} |`
        );
      }
      if (postRelabelStats) {
        lines.push(
          `| \`scrape_samples_post_metric_relabeling\` | ${postRelabelStats.min} | ${postRelabelStats.max} | ${postRelabelStats.avg.toFixed(0)} | ${postRelabelStats.latest} |`
        );
      }

      if (scrapedStats && postRelabelStats) {
        const avgDropped = scrapedStats.avg - postRelabelStats.avg;
        const dropPct = scrapedStats.avg > 0 ? ((avgDropped / scrapedStats.avg) * 100).toFixed(1) : "N/A";
        const latestDropped = scrapedStats.latest - postRelabelStats.latest;
        const latestDropPct = scrapedStats.latest > 0
          ? ((latestDropped / scrapedStats.latest) * 100).toFixed(1)
          : "N/A";

        lines.push(
          "",
          "**Metric Relabeling Drop Rate:**",
          "",
          "| Metric | Value |",
          "|--------|-------|",
          `| Avg samples scraped | ${scrapedStats.avg.toFixed(0)} |`,
          `| Avg samples after relabeling | ${postRelabelStats.avg.toFixed(0)} |`,
          `| Avg dropped by relabeling | ${avgDropped.toFixed(0)} (${dropPct}%) |`,
          `| Latest dropped | ${latestDropped} (${latestDropPct}%) |`
        );

        if (parseFloat(dropPct) > 50) {
          lines.push(
            "",
            "⚠️ **High relabeling drop rate (>" + "50%).** More than half of scraped samples are dropped by `metric_relabel_configs`.",
            "This is expected if keep-list filtering is configured, but may indicate over-aggressive relabeling if not intended."
          );
        }
      }

      const significantChanges: string[] = [];
      if (scrapedStats && scrapedStats.deviations.length > 0) {
        significantChanges.push(
          `\`scrape_samples_scraped\`: ${scrapedStats.deviations.length} bucket(s) with >10% change from previous bucket`
        );
      }
      if (postRelabelStats && postRelabelStats.deviations.length > 0) {
        significantChanges.push(
          `\`scrape_samples_post_metric_relabeling\`: ${postRelabelStats.deviations.length} bucket(s) with >10% change`
        );
      }
      if (significantChanges.length > 0) {
        lines.push("", "**Sample Count Volatility:**", "");
        for (const c of significantChanges) {
          lines.push(`- ${c}`);
        }
      } else if (scrapedStats || postRelabelStats) {
        lines.push("", "✅ Sample counts are stable (no buckets with >10% change).");
      }
    }

    return lines.join("\n");
  } catch (err) {
    return [
      "## Scrape Target Health Check",
      "",
      `❌ Query failed: ${err instanceof Error ? err.message : String(err)}`,
    ].join("\n");
  }
}

/**
 * Query any Prometheus metric from Geneva MDM for a specific cluster.
 *
 * Returns the raw time series data with summary statistics, allowing
 * investigation of whether a specific metric has recent data and what
 * its values look like over time.
 */
export async function queryMdmMetric(
  monitoringAccount: string,
  metric: string,
  cluster: string,
  nameSpace: string = "customdefault",
  dimensions: string = "{}",
  timeRangeHours: number = 24
): Promise<string> {
  const serverUp = await checkMdmServerHealth();
  if (!serverUp) {
    return [
      "## MDM Metric Query",
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

  // Merge the cluster filter into the user-provided dimensions
  let dimMap: Record<string, string[]>;
  try {
    dimMap = JSON.parse(dimensions);
  } catch {
    dimMap = {};
  }
  dimMap["cluster"] = [cluster];
  const dimensionMapJson = JSON.stringify(dimMap);

  const lines: string[] = [
    "## MDM Metric Query",
    "",
    "| Field | Value |",
    "|-------|-------|",
    `| **Metric** | \`${metric}\` |`,
    `| **Namespace** | \`${nameSpace}\` |`,
    `| **Cluster** | \`${cluster}\` |`,
    `| **MDM Account** | \`${monitoringAccount}\` |`,
    `| **Time Range** | ${timeRangeHours}h (${startTime} → ${endTime}) |`,
    `| **Dimension Filter** | \`${dimensionMapJson}\` |`,
  ];

  try {
    const result = await callMdmTool("QueryDimensionMDM", {
      monitoringAccount,
      nameSpace,
      metrics: metric,
      startTime,
      endTime,
      dimensionMapJson,
    });

    if (!result.Success || !result.Result || result.Result.length === 0) {
      lines.push(
        "",
        "⚠️ **No data returned.** Possible causes:",
        "- The metric name doesn't exist in this MDM account",
        "- The cluster name doesn't match the `cluster` dimension in MDM",
        "- No data was ingested for this metric in the queried time range",
        "- The namespace is wrong (try `prometheus` instead of `customdefault`)",
        "- The dimension filter is too restrictive",
      );
      if (result.Error) {
        lines.push("", `**Error:** ${result.Error}`);
      }
      return lines.join("\n");
    }

    const rawText = result.Result.join("\n");

    // Parse Sum/Count arrays
    const parsed = parseSumCount(rawText);

    if (!parsed) {
      // Return raw response if we can't parse it structurally
      lines.push(
        "",
        "✅ **Data exists** — metric returned results but could not parse Sum/Count arrays.",
        "",
        "**Raw response (first 2000 chars):**",
        "```",
        rawText.substring(0, 2000),
        "```",
      );
      return lines.join("\n");
    }

    const { sums, counts } = parsed;

    // Compute stats on the Sum array (primary value for gauge/counter metrics)
    const validSums = sums.filter((v) => !isNaN(v));
    const validCounts = counts.filter((v) => !isNaN(v));
    const nanBuckets = sums.filter((v) => isNaN(v)).length;
    const totalBuckets = sums.length;
    const activeBuckets = totalBuckets - nanBuckets;

    if (validSums.length === 0) {
      lines.push(
        "",
        `⚠️ **All ${totalBuckets} buckets are NaN** — no actual data points in this time range.`,
        "The metric may have existed historically but has no current data.",
      );
      return lines.join("\n");
    }

    const sumMin = Math.min(...validSums);
    const sumMax = Math.max(...validSums);
    const sumAvg = validSums.reduce((a, b) => a + b, 0) / validSums.length;
    const sumLatest = validSums[validSums.length - 1];
    const sumFirst = validSums[0];

    const countTotal = validCounts.reduce((a, b) => a + b, 0);
    const countAvg = validCounts.length > 0 ? countTotal / validCounts.length : 0;
    const countLatest = validCounts.length > 0 ? validCounts[validCounts.length - 1] : 0;

    const totalMinutes = timeRangeHours * 60;
    const bucketSizeMin = totalBuckets > 0 ? (totalMinutes / totalBuckets).toFixed(1) : "?";

    lines.push(
      "",
      `✅ **Data exists** — ${activeBuckets} active buckets out of ${totalBuckets} total (${nanBuckets} NaN).`,
      "",
      "### Sum Series (primary metric values)",
      "",
      "| Stat | Value |",
      "|------|-------|",
      `| First value | ${sumFirst} |`,
      `| Latest value | ${sumLatest} |`,
      `| Min | ${sumMin} |`,
      `| Max | ${sumMax} |`,
      `| Avg | ${sumAvg.toFixed(2)} |`,
      `| Active buckets | ${activeBuckets} / ${totalBuckets} |`,
      `| NaN buckets | ${nanBuckets} |`,
      `| Bucket resolution | ~${bucketSizeMin} min |`,
    );

    lines.push(
      "",
      "### Count Series (number of data points per bucket)",
      "",
      "| Stat | Value |",
      "|------|-------|",
      `| Total data points | ${countTotal} |`,
      `| Avg per bucket | ${countAvg.toFixed(1)} |`,
      `| Latest bucket | ${countLatest} |`,
    );

    // Detect significant changes in Sum values
    const significantChanges: { index: number; from: number; to: number; pctChange: number }[] = [];
    for (let i = 1; i < sums.length; i++) {
      const cur = sums[i];
      const prev = sums[i - 1];
      if (isNaN(cur) || isNaN(prev) || prev === 0) continue;
      const pctChange = ((cur - prev) / prev) * 100;
      if (Math.abs(pctChange) > 20) {
        significantChanges.push({ index: i, from: prev, to: cur, pctChange });
      }
    }

    if (significantChanges.length > 0) {
      lines.push(
        "",
        `### Significant Changes (>${"20"}% bucket-to-bucket)`,
        "",
        `Found **${significantChanges.length}** bucket(s) with >20% change:`,
      );
      const displayChanges = significantChanges.slice(0, 10);
      lines.push("", "| Bucket | From | To | Change |", "|--------|------|----|--------|");
      for (const c of displayChanges) {
        lines.push(`| ${c.index} | ${c.from.toFixed(1)} | ${c.to.toFixed(1)} | ${c.pctChange > 0 ? "+" : ""}${c.pctChange.toFixed(1)}% |`);
      }
      if (significantChanges.length > 10) {
        lines.push(`| ... | ... | ... | (${significantChanges.length - 10} more) |`);
      }
    } else {
      lines.push("", "✅ Sum values are stable (no buckets with >20% change).");
    }

    // Show a compact sparkline-like view of the last N buckets
    const sparkBuckets = Math.min(30, validSums.length);
    const recentSums = validSums.slice(-sparkBuckets);
    const sparkMax = Math.max(...recentSums);
    const sparkMin = Math.min(...recentSums);
    if (sparkMax > sparkMin) {
      const bars = "▁▂▃▄▅▆▇█";
      const spark = recentSums.map((v) => {
        const idx = Math.round(((v - sparkMin) / (sparkMax - sparkMin)) * (bars.length - 1));
        return bars[idx];
      }).join("");
      lines.push(
        "",
        `### Recent Trend (last ${sparkBuckets} buckets)`,
        "",
        `\`${spark}\``,
        `Range: ${sparkMin.toFixed(1)} → ${sparkMax.toFixed(1)}`,
      );
    }

    // Show first 500 chars of raw response for debugging
    lines.push(
      "",
      "<details><summary>Raw MDM response (first 1000 chars)</summary>",
      "",
      "```",
      rawText.substring(0, 1000),
      "```",
      "",
      "</details>",
    );

    return lines.join("\n");
  } catch (err) {
    lines.push(
      "",
      `❌ **Query failed:** ${err instanceof Error ? err.message : String(err)}`,
    );
    return lines.join("\n");
  }
}

// ─── MetricsExtension Internal QoS Diagnostics ─────────────────────────────────

/**
 * MetricsExtension internal QoS metrics live in the customer's MDM account
 * under the "MetricsExtension2" namespace. These track drops, errors, and
 * throttling at the ME agent level — distinct from the MdmQos namespace
 * which tracks drops at the MDM backend level.
 *
 * Source: https://eng.ms/docs/products/geneva/metrics/qos/metricsagentsdk/meinternalmetrics2
 */

interface MeMetricDef {
  panel: string;
  metric: string;
  description: string;
  reasons: string[];
}

const ME_QOS_METRICS: MeMetricDef[] = [
  {
    panel: "Raw Events Dropped",
    metric: "RawEventsDroppedCount",
    description: "Raw metric events dropped before aggregation",
    reasons: [
      "ConfigurationLost",
      "OldConfiguration",
      "Throttled",
      "IngestionLimitExceed",
      "OldDataExpiredReceiveTime",
      "FutureData",
      "BlackListedMetric",
      "TooManyDimensions",
      "MetricNameTooLong",
    ],
  },
  {
    panel: "Metric Aggregates Dropped",
    metric: "MetricAggregatesDroppedCount",
    description: "Pre-aggregated metric batches dropped before publication",
    reasons: [
      "OldData:PublicationCancelled",
      "PublicationFailedThrottled",
      "InvalidMetricMetadata",
      "PublicationQueueExceeded",
      "PublicationDisabled",
    ],
  },
  {
    panel: "Publication Failed",
    metric: "PublicationFailedCount",
    description: "Publication attempts that failed",
    reasons: [
      "Throttled",
      "ServerError",
      "Timeout",
      "AuthFailure",
      "NetworkError",
      "InternalError",
    ],
  },
  {
    panel: "ME Errors",
    metric: "MeErrorsCount",
    description: "MetricsExtension errors by type and reason",
    reasons: [
      "UnknownHttpException",
      "NoValidCertificate",
      "ConfigurationLoadFailed",
      "MaxPublicationBytesPerMinuteExceeded",
      "MaxPublicationMetricsPerMinuteExceeded",
      "MaxPublicationAttemptsPerMinuteExceeded",
      "OversizedHistogram",
      "ScrapingFailed",
      "ScrapingLate",
      "ScrapingEarly",
    ],
  },
];

// Positive-path ME metrics for health overview
const ME_HEALTH_METRICS = [
  { panel: "Metrics Ingested", metric: "MetricsIngestedCount" },
  { panel: "Aggregates Published", metric: "MetricAggregatesPublishedCount" },
  { panel: "Events Published", metric: "MetricEventsPublishedCount" },
  { panel: "Publication Queue Length", metric: "MetricPublicationQueueLength" },
  { panel: "Pipeline Latency (ms)", metric: "MetricsPipelineLatencyInMs" },
  { panel: "Process CPU %", metric: "ProcessCpuUsagePercentage" },
  { panel: "Process Memory (bytes)", metric: "ProcessMemorySizeInBytes" },
];

/**
 * Query MetricsExtension internal QoS metrics from the customer's MDM account.
 *
 * Uses KQLM queries (which correctly handle MetricsExtension2 sampling types)
 * instead of QueryDimensionMDM (which returns all NaN for these metrics).
 * Queries both error/drop metrics AND health/volume metrics.
 * When drops are detected, drills down by Reason dimension.
 */
export async function queryMeInternalMetrics(
  monitoringAccount: string,
  timeRangeHours: number = 6
): Promise<string> {
  const serverUp = await checkMdmServerHealth();
  if (!serverUp) {
    return [
      "## ME Internal Diagnostics",
      "",
      "❌ **Geneva MDM MCP server is not running.**",
      "",
      "Start it with:",
      "```",
      'DOTNET_EnableDiagnostics=0 ASPNETCORE_URLS="http://localhost:5050" ASPNETCORE_ENVIRONMENT=Local \\',
      '  nohup dotnet run --project /tmp/mdm-mcp/src/MdmMcp/GenevaMDM-MCP.csproj -- --urls "http://localhost:5050" &',
      "```",
    ].join("\n");
  }

  const now = new Date();
  const start = new Date(now.getTime() - timeRangeHours * 60 * 60 * 1000);
  const startTime = start.toISOString().replace(/\.\d{3}Z$/, "Z");
  const endTime = now.toISOString().replace(/\.\d{3}Z$/, "Z");

  const lines: string[] = [
    "## MetricsExtension Internal QoS Diagnostics",
    "",
    "| Field | Value |",
    "|-------|-------|",
    `| **MDM Account** | \`${monitoringAccount}\` |`,
    `| **Namespace** | \`MetricsExtension2\` |`,
    `| **Time Range** | ${timeRangeHours}h (${startTime} → ${endTime}) |`,
    "",
  ];

  // Phase 1: Query drop/error metrics via KQLM
  const aggregateResults = await Promise.all(
    ME_QOS_METRICS.map(async (def) => {
      try {
        const kqlmQuery = `metric("${def.metric}").samplingTypes("Sum")`;
        const kqlmResult = await callKqlm(monitoringAccount, "MetricsExtension2", kqlmQuery, startTime, endTime);

        if (!kqlmResult || kqlmResult.Status !== "Success" || !kqlmResult.Results || kqlmResult.Results.length === 0) {
          return { ...def, status: "no_data" as const, summary: "No data", total: 0, max: 0, hasData: false };
        }

        const vals = kqlmResult.Results[0].SamplingTypes?.Sum;
        if (!vals || vals.length === 0) {
          return { ...def, status: "no_data" as const, summary: "No data", total: 0, max: 0, hasData: false };
        }

        const nonNan = vals.filter((v) => !isNaN(v));
        if (nonNan.length === 0) {
          return { ...def, status: "ok" as const, summary: "No drops/errors ✅", total: 0, max: 0, hasData: false };
        }

        const total = nonNan.reduce((a, b) => a + b, 0);
        const max = Math.max(...nonNan);

        if (total === 0) {
          return { ...def, status: "ok" as const, summary: `No drops/errors (${nonNan.length} data points, all zero) ✅`, total: 0, max: 0, hasData: true };
        }

        return {
          ...def,
          status: "warning" as const,
          summary: `⚠️ DROPS DETECTED: total=${total.toLocaleString()}, max=${max.toLocaleString()} (${nonNan.length} points)`,
          total, max, hasData: true,
        };
      } catch (err) {
        return { ...def, status: "error" as const, summary: `Error: ${err instanceof Error ? err.message : String(err)}`, total: 0, max: 0, hasData: false };
      }
    })
  );

  // Summary table
  lines.push("### Drop & Error Summary", "", "| Panel | Metric | Status |", "|-------|--------|--------|");
  for (const r of aggregateResults) {
    const icon = r.status === "ok" ? "✅" : r.status === "warning" ? "⚠️" : r.status === "error" ? "❌" : "ℹ️";
    lines.push(`| ${r.panel} | \`${r.metric}\` | ${icon} ${r.summary} |`);
  }

  // Phase 1b: Query health/volume metrics via KQLM
  const healthResults = await Promise.all(
    ME_HEALTH_METRICS.map(async (def) => {
      try {
        const kqlmQuery = `metric("${def.metric}").samplingTypes("Sum")`;
        const kqlmResult = await callKqlm(monitoringAccount, "MetricsExtension2", kqlmQuery, startTime, endTime);

        if (!kqlmResult || kqlmResult.Status !== "Success" || !kqlmResult.Results || kqlmResult.Results.length === 0) {
          return { panel: def.panel, metric: def.metric, value: "No data" };
        }

        const vals = kqlmResult.Results[0].SamplingTypes?.Sum;
        if (!vals || vals.length === 0) return { panel: def.panel, metric: def.metric, value: "No data" };

        const nonNan = vals.filter((v) => !isNaN(v));
        if (nonNan.length === 0) return { panel: def.panel, metric: def.metric, value: "No data (NaN)" };

        const latest = nonNan[nonNan.length - 1];
        const avg = nonNan.reduce((a, b) => a + b, 0) / nonNan.length;
        return { panel: def.panel, metric: def.metric, value: `latest=${latest.toLocaleString(undefined, { maximumFractionDigits: 0 })}, avg=${avg.toLocaleString(undefined, { maximumFractionDigits: 0 })}` };
      } catch {
        return { panel: def.panel, metric: def.metric, value: "Error" };
      }
    })
  );

  const healthWithData = healthResults.filter((r) => r.value !== "No data" && r.value !== "No data (NaN)" && r.value !== "Error");
  if (healthWithData.length > 0) {
    lines.push("", "### ME Health Metrics", "", "| Panel | Metric | Value |", "|-------|--------|-------|");
    for (const r of healthWithData) {
      lines.push(`| ${r.panel} | \`${r.metric}\` | ${r.value} |`);
    }
  }

  // Phase 2: For any metric with drops > 0, drill down by Reason dimension via KQLM
  const metricsWithDrops = aggregateResults.filter((r) => r.status === "warning" && r.total > 0);

  if (metricsWithDrops.length > 0) {
    lines.push("", "### ⚠️ Drop Breakdown by Reason", "");

    for (const metricDef of metricsWithDrops) {
      lines.push(`#### ${metricDef.panel} (\`${metricDef.metric}\`)`, "");

      // Query with Reason dimension via KQLM
      const reasonQuery = `metric("${metricDef.metric}").dimensions("Reason").samplingTypes("Sum")`;
      try {
        const reasonResult = await callKqlm(monitoringAccount, "MetricsExtension2", reasonQuery, startTime, endTime);

        if (reasonResult && reasonResult.Status === "Success" && reasonResult.Results && reasonResult.Results.length > 0) {
          const reasonSummaries: { reason: string; total: number; max: number }[] = [];
          for (const series of reasonResult.Results) {
            const reason = series.Dimensions?.Reason || "unknown";
            const vals = series.SamplingTypes?.Sum;
            if (!vals) continue;
            const nonNan = vals.filter((v) => !isNaN(v));
            if (nonNan.length === 0) continue;
            const total = nonNan.reduce((a, b) => a + b, 0);
            if (total === 0) continue;
            reasonSummaries.push({ reason, total, max: Math.max(...nonNan) });
          }

          if (reasonSummaries.length > 0) {
            reasonSummaries.sort((a, b) => b.total - a.total);
            lines.push("| Reason | Total Dropped | Max per Bucket |", "|--------|---------------|----------------|");
            for (const r of reasonSummaries) {
              const highlight = [
                "TooManyDimensions", "BlackListedMetric", "Throttled", "IngestionLimitExceed",
                "MaxPublicationMetricsPerMinuteExceeded", "MaxPublicationBytesPerMinuteExceeded",
                "OversizedHistogram",
              ].includes(r.reason) ? " 🔴" : "";
              lines.push(`| \`${r.reason}\`${highlight} | ${r.total.toLocaleString()} | ${r.max.toLocaleString()} |`);
            }
          } else {
            lines.push("No per-reason data available.");
          }
        } else {
          // Fallback: try individual reason queries via QueryDimensionMDM
          lines.push("No per-reason breakdown available (Reason dimension may not be pre-aggregated).");
        }
      } catch {
        lines.push("Error querying per-reason breakdown.");
      }
      lines.push("");
    }

    // Diagnostic guidance
    lines.push("### Diagnostic Guidance", "");
    for (const metricDef of metricsWithDrops) {
      lines.push(`**${metricDef.metric}** — ${metricDef.description}:`, "");
      if (metricDef.metric === "RawEventsDroppedCount") {
        lines.push("- `TooManyDimensions` → Metric has more dimensions than the account limit (64 for Prometheus/AMW). Reduce `metricLabelsAllowlist` or use `metric_relabel_configs` to drop labels");
        lines.push("- `BlackListedMetric` → Metric name is on the account's blocklist");
        lines.push("- `Throttled` → ME is throttling due to backpressure from MDM");
        lines.push("- `IngestionLimitExceed` → Account ingestion rate limit exceeded");
        lines.push("- `MetricNameTooLong` → Metric name exceeds max length");
        lines.push("- `OldDataExpiredReceiveTime` → Data arrived too late to be accepted");
        lines.push("- `FutureData` → Timestamps are in the future");
      }
      if (metricDef.metric === "MetricAggregatesDroppedCount") {
        lines.push("- `PublicationFailedThrottled` → MDM backend rejected publication due to throttling");
        lines.push("- `InvalidMetricMetadata` → Metric schema/metadata is invalid");
        lines.push("- `PublicationQueueExceeded` → ME internal queue is full, dropping oldest aggregates");
        lines.push("- `PublicationDisabled` → Publication is disabled (config issue)");
      }
      if (metricDef.metric === "PublicationFailedCount") {
        lines.push("- `Throttled` → MDM backend is throttling publications");
        lines.push("- `ServerError` → MDM returned a server error");
        lines.push("- `Timeout` → Publication request timed out");
      }
      if (metricDef.metric === "MeErrorsCount") {
        lines.push("- `MaxPublicationMetricsPerMinuteExceeded` → ME config limit `maxPublicationMetricsPerMinute` exceeded");
        lines.push("- `MaxPublicationBytesPerMinuteExceeded` → ME config limit `maxPublicationBytesPerMinute` exceeded");
        lines.push("- `OversizedHistogram` → Histogram metric has too many buckets");
        lines.push("- `NoValidCertificate` → Auth/certificate issue with MDM endpoint");
      }
      lines.push("");
    }
  } else {
    lines.push(
      "",
      "### ✅ No ME-level drops or errors detected.",
      "",
      "MetricsExtension is processing and publishing all metrics without drops.",
      "If metrics are still missing from MDM, the issue is likely at the **MDM backend/MStore level**.",
      "Check `tsg_mdm_throttling` for MDM-level throttling (especially `MStoreDroppedSamplesCount`).",
    );
  }

  return lines.join("\n");
}
