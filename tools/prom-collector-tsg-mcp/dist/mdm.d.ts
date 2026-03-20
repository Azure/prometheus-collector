/**
 * Check if the Geneva MDM MCP server is reachable.
 */
export declare function checkMdmServerHealth(): Promise<boolean>;
/**
 * Query all QoS throttling metrics for a given MDM monitoring account.
 * Returns formatted results for each panel.
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
//# sourceMappingURL=mdm.d.ts.map