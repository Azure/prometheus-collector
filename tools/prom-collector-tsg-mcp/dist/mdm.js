// Geneva MDM QoS throttling checks via the Geneva MDM MCP server (HTTP)
// Queries the same metrics as the Geneva QoS dashboard: mac_91c1e6c2-bcdf-4650-9f80-179b245c2533
const MDM_MCP_URL = process.env.MDM_MCP_URL || "http://localhost:5050/mcp";
/**
 * Call a tool on the Geneva MDM MCP server via JSON-RPC over HTTP.
 */
async function callMdmTool(toolName, args) {
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
        throw new Error(`MDM MCP server returned ${response.status}: ${await response.text()}`);
    }
    const body = await response.json();
    if (body.error) {
        throw new Error(`MDM MCP error: ${body.error.message}`);
    }
    const text = body.result?.content?.[0]?.text;
    if (!text) {
        throw new Error("No content in MDM MCP response");
    }
    return JSON.parse(text);
}
/**
 * Parse a Sum series string like "[0,0,NaN,5,10]" into stats.
 */
function parseSumSeries(raw) {
    const match = raw.match(/Sum:\s*\[([^\]]+)\]/);
    if (!match)
        return { sum: 0, max: 0, nonNanPoints: 0, totalPoints: 0 };
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
export async function checkMdmServerHealth() {
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
    }
    catch {
        return false;
    }
}
/**
 * Query all QoS throttling metrics for a given MDM monitoring account.
 * Returns formatted results for each panel.
 */
export async function queryMdmThrottling(monitoringAccount, timeRangeHours = 6) {
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
    const results = [];
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
                const errMsg = result.Result?.[0] || result.Error || "No data returned";
                return {
                    panel: def.panel,
                    metric: def.metric,
                    status: "no_data",
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
                    status: (def.isThrottleMetric ? "ok" : "no_data"),
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
                        status: "ok",
                        summary: `No throttling/drops (${stats.nonNanPoints} data points, all zero) ✅`,
                        values: stats,
                    };
                }
                return {
                    panel: def.panel,
                    metric: def.metric,
                    status: "warning",
                    summary: `⚠️ THROTTLING DETECTED: total=${stats.sum.toLocaleString()}, max=${stats.max.toLocaleString()} (${stats.nonNanPoints} points)`,
                    values: stats,
                };
            }
            // Volume/limit metrics
            const avg = stats.sum / stats.nonNanPoints;
            return {
                panel: def.panel,
                metric: def.metric,
                status: "ok",
                summary: `avg=${avg.toLocaleString(undefined, {
                    maximumFractionDigits: 0,
                })}/min, total=${stats.sum.toLocaleString(undefined, {
                    maximumFractionDigits: 0,
                })} (${stats.nonNanPoints} points)`,
                values: stats,
            };
        }
        catch (err) {
            return {
                panel: def.panel,
                metric: def.metric,
                status: "error",
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
    let utilizationLines = [];
    if (eventUsage?.values &&
        eventLimit?.values &&
        eventUsage.values.nonNanPoints > 0 &&
        eventLimit.values.nonNanPoints > 0) {
        const usageAvg = eventUsage.values.sum / eventUsage.values.nonNanPoints;
        const limitAvg = eventLimit.values.sum / eventLimit.values.nonNanPoints;
        if (limitAvg > 0) {
            const pct = ((usageAvg / limitAvg) * 100).toFixed(1);
            utilizationLines.push(`- **Event Volume Utilization**: ${pct}%`);
        }
    }
    if (tsUsage?.values &&
        tsLimit?.values &&
        tsUsage.values.nonNanPoints > 0 &&
        tsLimit.values.nonNanPoints > 0) {
        const usageAvg = tsUsage.values.sum / tsUsage.values.nonNanPoints;
        const limitAvg = tsLimit.values.sum / tsLimit.values.nonNanPoints;
        if (limitAvg > 0) {
            const pct = ((usageAvg / limitAvg) * 100).toFixed(1);
            utilizationLines.push(`- **MStore Time Series Utilization**: ${pct}%`);
        }
    }
    // Format output
    const lines = [
        `## MDM QoS Throttling Check`,
        `Account: **${monitoringAccount}** | Namespace: **MdmQos** | Window: last ${timeRangeHours}h`,
        "",
    ];
    // Throttle/Drop metrics first
    const throttleResults = results.filter((r) => QOS_METRICS.find((m) => m.metric === r.metric)?.isThrottleMetric);
    const volumeResults = results.filter((r) => !QOS_METRICS.find((m) => m.metric === r.metric)?.isThrottleMetric);
    lines.push("### Throttling & Drops");
    lines.push("| Panel | Metric | Status |", "| --- | --- | --- |");
    for (const r of throttleResults) {
        const icon = r.status === "ok" ? "✅" : r.status === "warning" ? "⚠️" : "ℹ️";
        lines.push(`| ${r.panel} | \`${r.metric}\` | ${icon} ${r.summary} |`);
    }
    lines.push("", "### Volume & Limits");
    lines.push("| Panel | Metric | Status |", "| --- | --- | --- |");
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
        lines.push("### ⚠️ ACTION REQUIRED", "Throttling or drops detected. Check the [Geneva QoS Dashboard](https://portal.microsoftgeneva.com/dashboard/mac_91c1e6c2-bcdf-4650-9f80-179b245c2533/GenevaQos/%E2%86%90%20MdmQos) for details.", "Common causes: high cardinality metrics, metric explosion, exceeded account limits.");
    }
    else {
        lines.push("### ✅ No throttling or drops detected.");
    }
    return lines.join("\n");
}
/**
 * Parse Sum and Count arrays from an MDM query result string.
 * Returns arrays of numbers (NaN for "NaN" entries).
 */
function parseSumCount(raw) {
    const sumMatch = raw.match(/Sum:\s*\[([^\]]+)\]/);
    const countMatch = raw.match(/Count:\s*\[([^\]]+)\]/);
    if (!sumMatch || !countMatch)
        return null;
    const sums = sumMatch[1].split(",").map((s) => parseFloat(s.trim()));
    const counts = countMatch[1].split(",").map((s) => parseFloat(s.trim()));
    return { sums, counts };
}
function analyzeUpMetric(parsed) {
    let healthy = 0, degraded = 0, nanBuckets = 0, totalScrapes = 0, totalUp = 0;
    const failureBuckets = [];
    for (let i = 0; i < parsed.sums.length; i++) {
        const s = parsed.sums[i];
        const c = parsed.counts[i];
        if (isNaN(s) || isNaN(c)) {
            nanBuckets++;
            continue;
        }
        totalScrapes += c;
        totalUp += s;
        if (s === c) {
            healthy++;
        }
        else {
            degraded++;
            failureBuckets.push({ index: i, sum: s, count: c });
        }
    }
    const activeBuckets = healthy + degraded;
    const successRate = totalScrapes > 0 ? ((totalUp / totalScrapes) * 100).toFixed(2) : "N/A";
    return { totalScrapes, totalUp, healthy, degraded, nanBuckets, activeBuckets, successRate, failureBuckets };
}
/** Compute min/max/avg/latest stats and detect >10% bucket-to-bucket changes. */
function computeSampleStats(parsed) {
    const vals = parsed.sums.filter((v) => !isNaN(v));
    if (vals.length === 0)
        return null;
    const min = Math.min(...vals);
    const max = Math.max(...vals);
    const avg = vals.reduce((a, b) => a + b, 0) / vals.length;
    const latest = vals[vals.length - 1];
    const deviations = [];
    for (let i = 1; i < parsed.sums.length; i++) {
        const cur = parsed.sums[i];
        const prev = parsed.sums[i - 1];
        if (isNaN(cur) || isNaN(prev) || prev === 0)
            continue;
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
export async function queryScrapeTargetHealth(monitoringAccount, job, cluster, timeRangeHours = 24) {
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
async function queryMultiJobHealth(monitoringAccount, cluster, timeRangeHours, startTime, endTime) {
    const results = await Promise.all(DEFAULT_JOBS.map(async (j) => {
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
        }
        catch {
            return { job: j, result: null };
        }
    }));
    // For jobs that have up data, also query sample metrics in parallel
    const jobsWithData = [];
    const upAnalyses = new Map();
    for (const { job: j, result: r } of results) {
        if (!r || !r.Success || !r.Result || r.Result.length === 0)
            continue;
        const parsed = parseSumCount(r.Result.join("\n"));
        if (!parsed)
            continue;
        const analysis = analyzeUpMetric(parsed);
        if (analysis.totalScrapes > 0) {
            jobsWithData.push(j);
            upAnalyses.set(j, analysis);
        }
    }
    // Query scrape_samples_scraped and scrape_samples_post_metric_relabeling for jobs with data
    const sampleResults = await Promise.all(jobsWithData.flatMap((j) => [
        callMdmTool("QueryDimensionMDM", {
            monitoringAccount,
            nameSpace: "customdefault",
            metrics: "scrape_samples_scraped",
            startTime,
            endTime,
            dimensionMapJson: JSON.stringify({ job: [j], cluster: [cluster] }),
        }).then((r) => ({ job: j, metric: "scraped", result: r }))
            .catch(() => ({ job: j, metric: "scraped", result: null })),
        callMdmTool("QueryDimensionMDM", {
            monitoringAccount,
            nameSpace: "customdefault",
            metrics: "scrape_samples_post_metric_relabeling",
            startTime,
            endTime,
            dimensionMapJson: JSON.stringify({ job: [j], cluster: [cluster] }),
        }).then((r) => ({ job: j, metric: "postRelabel", result: r }))
            .catch(() => ({ job: j, metric: "postRelabel", result: null })),
    ]));
    const scrapedAvgs = new Map();
    const postRelabelAvgs = new Map();
    for (const { job: j, metric, result: r } of sampleResults) {
        if (!r || !r.Success || !r.Result || r.Result.length === 0)
            continue;
        const parsed = parseSumCount(r.Result.join("\n"));
        if (!parsed)
            continue;
        const stats = computeSampleStats(parsed);
        if (!stats)
            continue;
        if (metric === "scraped") {
            scrapedAvgs.set(j, stats.avg);
        }
        else {
            postRelabelAvgs.set(j, stats.avg);
        }
    }
    const summaries = [];
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
        let status;
        if (failures === 0) {
            status = "✅ Healthy";
        }
        else if (parseFloat(analysis.successRate) > 95) {
            status = "⚠️ Degraded";
        }
        else {
            status = "❌ Down";
        }
        const scraped = scrapedAvgs.has(j) ? scrapedAvgs.get(j).toFixed(0) : "—";
        const postRelabel = postRelabelAvgs.has(j) ? postRelabelAvgs.get(j).toFixed(0) : "—";
        summaries.push({ job: j, status, successRate: analysis.successRate + "%", totalScrapes: analysis.totalScrapes, failures, samplesScraped: scraped, samplesPostRelabel: postRelabel });
    }
    // Filter to only jobs with data, plus any degraded/down
    const active = summaries.filter((s) => s.status !== "— No data");
    const missing = summaries.filter((s) => s.status === "— No data");
    const lines = [
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
    lines.push("### Per-Job Health Summary", "", "| Job | Status | Success Rate | Scrapes | Failures | Samples Scraped (avg) | After Relabeling (avg) |", "|-----|--------|--------------|---------|----------|-----------------------|------------------------|");
    for (const s of active) {
        lines.push(`| \`${s.job}\` | ${s.status} | ${s.successRate} | ${s.totalScrapes} | ${s.failures} | ${s.samplesScraped} | ${s.samplesPostRelabel} |`);
    }
    // Highlight issues
    const degraded = active.filter((s) => s.status !== "✅ Healthy");
    if (degraded.length > 0) {
        lines.push("", "### ⚠️ Jobs with Issues", "", "Run `tsg_scrape_health` with a specific `job` parameter for detailed per-bucket failure analysis:", "");
        for (const s of degraded) {
            lines.push(`- **\`${s.job}\`** — ${s.status} (${s.successRate}, ${s.failures} failures)`);
        }
    }
    else {
        lines.push("", "### ✅ All scrape targets are healthy.");
    }
    // Show relabeling summary for jobs that have both metrics
    const relabelRows = [];
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
        lines.push("", "### Metric Relabeling Drop Rates", "", "| Job | Scraped (avg) | After Relabeling (avg) | Dropped | Drop % |", "|-----|---------------|------------------------|---------|--------|", ...relabelRows);
    }
    if (missing.length > 0 && missing.length < DEFAULT_JOBS.length) {
        lines.push("", `<details><summary>${missing.length} jobs with no data (not active on this cluster)</summary>`, "", missing.map((s) => `\`${s.job}\``).join(", "), "", "</details>");
    }
    return lines.join("\n");
}
/**
 * Single-job mode: detailed analysis with up + samples_scraped + post_relabeling.
 */
async function querySingleJobHealth(monitoringAccount, job, cluster, timeRangeHours, startTime, endTime) {
    const dimensionMapJson = JSON.stringify({ job: [job], cluster: [cluster] });
    const baseArgs = { monitoringAccount, nameSpace: "customdefault", startTime, endTime, dimensionMapJson };
    try {
        const [upResult, scrapedResult, postRelabelResult] = await Promise.all([
            callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "up" }),
            callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "scrape_samples_scraped" }),
            callMdmTool("QueryDimensionMDM", { ...baseArgs, metrics: "scrape_samples_post_metric_relabeling" }),
        ]);
        const noData = (r) => !r.Success || !r.Result || r.Result.length === 0;
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
        const upRaw = upResult.Result.join("\n");
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
        const lines = [
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
        }
        else {
            lines.push("", `⚠️ Target \`${job}\` has scrape failures`);
            if (failureBuckets.length <= 20) {
                lines.push("", "| Bucket | up=1 | up=0 | Total |", "|--------|------|------|-------|");
                for (const fb of failureBuckets) {
                    lines.push(`| ${fb.index} | ${fb.sum} | ${fb.count - fb.sum} | ${fb.count} |`);
                }
            }
            else {
                const gaps = [];
                for (let i = 1; i < Math.min(failureBuckets.length, 30); i++) {
                    gaps.push(failureBuckets[i].index - failureBuckets[i - 1].index);
                }
                const avgGap = gaps.length > 0 ? (gaps.reduce((a, b) => a + b, 0) / gaps.length).toFixed(0) : "?";
                const isRegular = gaps.length > 3 && gaps.every((g) => Math.abs(g - gaps[0]) <= 1);
                lines.push("", `- **${failureBuckets.length}** buckets with failures over ${timeRangeHours}h`, `- Failure spacing: ${isRegular ? `regular, every ~${avgGap} buckets (~${(parseFloat(avgGap) * parseFloat(bucketSizeMin)).toFixed(0)} min)` : `irregular (avg ${avgGap} buckets apart)`}`, `- Typical failure: Sum=${failureBuckets[0].sum}, Count=${failureBuckets[0].count} (${failureBuckets[0].count - failureBuckets[0].sum} scrapes returned up=0)`);
            }
        }
        // --- Scrape Samples Analysis ---
        lines.push("", "### Scrape Samples Analysis");
        const scrapedParsed = noData(scrapedResult) ? null : parseSumCount(scrapedResult.Result.join("\n"));
        const postRelabelParsed = noData(postRelabelResult) ? null : parseSumCount(postRelabelResult.Result.join("\n"));
        if (!scrapedParsed && !postRelabelParsed) {
            lines.push("", "⚠️ No data for `scrape_samples_scraped` or `scrape_samples_post_metric_relabeling`.");
        }
        else {
            const scrapedStats = scrapedParsed ? computeSampleStats(scrapedParsed) : null;
            const postRelabelStats = postRelabelParsed ? computeSampleStats(postRelabelParsed) : null;
            lines.push("", "| Metric | Min | Max | Avg | Latest |", "|--------|-----|-----|-----|--------|");
            if (scrapedStats) {
                lines.push(`| \`scrape_samples_scraped\` | ${scrapedStats.min} | ${scrapedStats.max} | ${scrapedStats.avg.toFixed(0)} | ${scrapedStats.latest} |`);
            }
            if (postRelabelStats) {
                lines.push(`| \`scrape_samples_post_metric_relabeling\` | ${postRelabelStats.min} | ${postRelabelStats.max} | ${postRelabelStats.avg.toFixed(0)} | ${postRelabelStats.latest} |`);
            }
            if (scrapedStats && postRelabelStats) {
                const avgDropped = scrapedStats.avg - postRelabelStats.avg;
                const dropPct = scrapedStats.avg > 0 ? ((avgDropped / scrapedStats.avg) * 100).toFixed(1) : "N/A";
                const latestDropped = scrapedStats.latest - postRelabelStats.latest;
                const latestDropPct = scrapedStats.latest > 0
                    ? ((latestDropped / scrapedStats.latest) * 100).toFixed(1)
                    : "N/A";
                lines.push("", "**Metric Relabeling Drop Rate:**", "", "| Metric | Value |", "|--------|-------|", `| Avg samples scraped | ${scrapedStats.avg.toFixed(0)} |`, `| Avg samples after relabeling | ${postRelabelStats.avg.toFixed(0)} |`, `| Avg dropped by relabeling | ${avgDropped.toFixed(0)} (${dropPct}%) |`, `| Latest dropped | ${latestDropped} (${latestDropPct}%) |`);
                if (parseFloat(dropPct) > 50) {
                    lines.push("", "⚠️ **High relabeling drop rate (>" + "50%).** More than half of scraped samples are dropped by `metric_relabel_configs`.", "This is expected if keep-list filtering is configured, but may indicate over-aggressive relabeling if not intended.");
                }
            }
            const significantChanges = [];
            if (scrapedStats && scrapedStats.deviations.length > 0) {
                significantChanges.push(`\`scrape_samples_scraped\`: ${scrapedStats.deviations.length} bucket(s) with >10% change from previous bucket`);
            }
            if (postRelabelStats && postRelabelStats.deviations.length > 0) {
                significantChanges.push(`\`scrape_samples_post_metric_relabeling\`: ${postRelabelStats.deviations.length} bucket(s) with >10% change`);
            }
            if (significantChanges.length > 0) {
                lines.push("", "**Sample Count Volatility:**", "");
                for (const c of significantChanges) {
                    lines.push(`- ${c}`);
                }
            }
            else if (scrapedStats || postRelabelStats) {
                lines.push("", "✅ Sample counts are stable (no buckets with >10% change).");
            }
        }
        return lines.join("\n");
    }
    catch (err) {
        return [
            "## Scrape Target Health Check",
            "",
            `❌ Query failed: ${err instanceof Error ? err.message : String(err)}`,
        ].join("\n");
    }
}
//# sourceMappingURL=mdm.js.map