/**
 * Check if the Geneva MDM MCP server is reachable.
 */
export declare function checkMdmServerHealth(): Promise<boolean>;
/**
 * Query all QoS throttling metrics for a given MDM monitoring account.
 * Uses KQLM queries which correctly handle MdmQos sampling types.
 * Also includes per-namespace time series breakdown.
 */
export declare function queryMdmThrottling(monitoringAccount: string, timeRangeHours?: number): Promise<string>;
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
export declare function queryScrapeTargetHealth(monitoringAccount: string, job: string, cluster: string, timeRangeHours?: number): Promise<string>;
/**
 * Query any Prometheus metric from Geneva MDM for a specific cluster.
 *
 * Returns the raw time series data with summary statistics, allowing
 * investigation of whether a specific metric has recent data and what
 * its values look like over time.
 */
export declare function queryMdmMetric(monitoringAccount: string, metric: string, cluster: string, nameSpace?: string, dimensions?: string, timeRangeHours?: number): Promise<string>;
/**
 * Query MetricsExtension internal QoS metrics from the customer's MDM account.
 *
 * Uses KQLM queries (which correctly handle MetricsExtension2 sampling types)
 * instead of QueryDimensionMDM (which returns all NaN for these metrics).
 * Queries both error/drop metrics AND health/volume metrics.
 * When drops are detected, drills down by Reason dimension.
 */
export declare function queryMeInternalMetrics(monitoringAccount: string, timeRangeHours?: number): Promise<string>;
//# sourceMappingURL=mdm.d.ts.map