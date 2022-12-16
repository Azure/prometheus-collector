/**
 * Prometheus rules file
 */
export interface PrometheusRules {
    groups?: PrometheusRulesGroup[];
}
export interface PrometheusRulesGroup {
    /**
       * The name of the group. Must be unique within a file.
       */
    name: string;
    /**
     * How often rules in the group are evaluated.
     */
    interval?: string;
    /**
     * Limit the number of alerts an alerting rule and series a recording rule can produce. 0 is no limit.
     */
    limit?: number;
    /**
     * Prometheus rules.
     */
    rules?: (RecordingRule | AlertingRule)[];
}
export interface RecordingRule {
    /**
     * The name of the time series to output to. Must be a valid metric name.
     */
    record: string;
    /**
     * The PromQL expression to evaluate. Every evaluation cycle this is evaluated at the current time, and the result recorded as a new set of time series with the metric name as given by 'record'.
     */
    expr: string;
    /**
   * Labels to add or overwrite before storing the result.
   */
    labels?: Labels;
}
export interface AlertingRule {
    /**
     * The name of the alert. Must be a valid metric name.
     */
    alert: string;
    /**
     * The PromQL expression to evaluate. Every evaluation cycle this is evaluated at the current time, and all resultant time series become pending/firing alerts.
     */
    expr: string;
    /**
     * Alerts are considered firing once they have been returned for this long. Alerts which have not yet fired for long enough are considered pending.
     */
    for?: string;
    /**
   * Labels to add or overwrite for each alert.
   */
    labels?: Labels;
    annotations?: Annotations;
}
/**
 * Annotations to add to each alert.
 */
export interface Annotations {
    [k: string]: TemplateString;
}
export interface Labels {
    [k: string]: LabelValue;
}
/**
 * This interface was referenced by `Labels`'s JSON-Schema definition
 * via the `patternProperty` "^[a-zA-Z_][a-zA-Z0-9_]*$".
 *
 * This interface was referenced by `Labels1`'s JSON-Schema definition
 * via the `patternProperty` "^[a-zA-Z_][a-zA-Z0-9_]*$".
 */
export type LabelValue = string;
/**
 * A string which is template-expanded before usage.
 *
 * This interface was referenced by `Annotations`'s JSON-Schema definition
 * via the `patternProperty` "^[a-zA-Z_][a-zA-Z0-9_]*$".
 */
export type TemplateString = string;
