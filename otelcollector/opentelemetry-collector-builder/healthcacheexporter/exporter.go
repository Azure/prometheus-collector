package healthcacheexporter

import (
	"context"
	"strconv"

	metricsreport "prometheus-collector/metricsreport/controller"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const typeStr = "health_cache"

var componentType = component.MustNewType(typeStr)

// upgradeHealthMetrics are always intercepted for built-in health checks.
var builtinHealthMetrics = map[string]bool{
	"kube_node_status_condition":                               true,
	"kube_node_info":                                           true,
	"kube_node_labels":                                         true,
	"kube_pod_info":                                            true,
	"kube_pod_status_phase":                                    true,
	"kube_poddisruptionbudget_status_pod_disruptions_allowed":  true,
}

// Config holds the exporter configuration.
type Config struct{}

// Validate implements component.Config.
func (c *Config) Validate() error { return nil }

// healthCacheExporter writes upgrade-health metrics into a shared MetricsCache.
type healthCacheExporter struct {
	sink            *metricsreport.HealthMetricSink
	upgradeGate     *metricsreport.UpgradeGate
	customMetricNames map[string]bool // metric names from customer rules
}

func (e *healthCacheExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (e *healthCacheExporter) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (e *healthCacheExporter) Shutdown(_ context.Context) error {
	return nil
}

// ConsumeMetrics filters health-relevant metrics and records them in the cache.
func (e *healthCacheExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Reload customer rules periodically to discover new metric names.
	e.refreshCustomMetrics(ctx)

	var samples []metricsreport.HealthMetricSample

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				name := m.Name()

				if builtinHealthMetrics[name] {
					samples = append(samples, processBuiltinMetric(name, m)...)
				}
				// Customer metrics: store raw values for rule evaluation.
				if e.customMetricNames[name] {
					samples = append(samples, processCustomMetric(name, m)...)
				}
			}
		}
	}

	if len(samples) > 0 {
		e.sink.Ingest(samples)
	}
	return nil
}

// refreshCustomMetrics reloads the ConfigMap rules to discover which metric
// names customers reference in their queries. This is best-effort.
func (e *healthCacheExporter) refreshCustomMetrics(ctx context.Context) {
	if e.upgradeGate == nil {
		return
	}
	rules, err := e.upgradeGate.LoadRules(ctx)
	if err != nil || len(rules) == 0 {
		return
	}
	names := make(map[string]bool, len(rules))
	for _, r := range rules {
		// Extract metric name from query — simple heuristic: first word before { or (
		name := extractMetricName(r.Query)
		if name != "" {
			names[name] = true
		}
	}
	e.customMetricNames = names
}

// extractMetricName attempts to get the base metric name from a PromQL query.
// e.g. "sum(rate(http_requests_total{code=~\"5..\"}[5m]))" → "http_requests_total"
func extractMetricName(query string) string {
	// Walk past function names and parens to find a metric name.
	name := []byte{}
	inFunc := false
	for _, c := range []byte(query) {
		switch {
		case c == '(' || c == ')':
			inFunc = true
			name = name[:0]
		case c == '{' || c == '[' || c == ' ' || c == ',':
			if len(name) > 0 {
				return string(name)
			}
		default:
			if !inFunc {
				name = append(name, c)
			}
			inFunc = false
		}
	}
	if len(name) > 0 {
		return string(name)
	}
	return ""
}

// --- Builtin metric processing ---

func processBuiltinMetric(name string, m pmetric.Metric) []metricsreport.HealthMetricSample {
	switch name {
	case "kube_node_status_condition":
		return processNodeStatusCondition(m)
	case "kube_node_info":
		return processNodeInfo(m)
	case "kube_node_labels":
		return processNodeLabels(m)
	case "kube_pod_info":
		return processPodInfo(m)
	case "kube_pod_status_phase":
		return processPodStatusPhase(m)
	case "kube_poddisruptionbudget_status_pod_disruptions_allowed":
		return processPDBStatus(m)
	}
	return nil
}

func processNodeStatusCondition(m pmetric.Metric) []metricsreport.HealthMetricSample {
	var samples []metricsreport.HealthMetricSample
	iterateGauge(m, func(attrs map[string]string, val float64) {
		node := attrs["node"]
		condition := attrs["condition"]
		status := attrs["status"]
		if node == "" || condition == "" {
			return
		}
		switch condition {
		case "Ready":
			if status == "true" {
				samples = append(samples, metricsreport.HealthMetricSample{
					MetricType: metricsreport.MetricNodeReady, Target: node, Labels: attrs, Value: val,
				})
			}
		case "NetworkUnavailable":
			if status == "true" {
				samples = append(samples, metricsreport.HealthMetricSample{
					MetricType: metricsreport.MetricNetworkUnavailable, Target: node, Labels: attrs, Value: val,
				})
			}
		}
	})
	return samples
}

func processNodeInfo(m pmetric.Metric) []metricsreport.HealthMetricSample {
	return []metricsreport.HealthMetricSample{
		{MetricType: metricsreport.MetricClusterNodeCount, Value: float64(countGauge(m))},
	}
}

func processNodeLabels(m pmetric.Metric) []metricsreport.HealthMetricSample {
	var samples []metricsreport.HealthMetricSample
	iterateGauge(m, func(attrs map[string]string, val float64) {
		node := attrs["node"]
		pool := attrs["label_kubernetes_azure_com_agentpool"]
		if node == "" || pool == "" {
			return
		}
		samples = append(samples, metricsreport.HealthMetricSample{
			MetricType: metricsreport.MetricPoolNetwork, Target: pool + "/" + node, Labels: attrs, Value: val,
		})
	})
	return samples
}

func processPodInfo(m pmetric.Metric) []metricsreport.HealthMetricSample {
	nodePods := make(map[string]int)
	iterateGauge(m, func(attrs map[string]string, val float64) {
		if node := attrs["node"]; node != "" {
			nodePods[node]++
		}
	})
	samples := make([]metricsreport.HealthMetricSample, 0, len(nodePods))
	for node, count := range nodePods {
		samples = append(samples, metricsreport.HealthMetricSample{
			MetricType: metricsreport.MetricPodTotal, Target: node, Value: float64(count),
		})
	}
	return samples
}

func processPodStatusPhase(m pmetric.Metric) []metricsreport.HealthMetricSample {
	unhealthy := 0
	iterateGauge(m, func(attrs map[string]string, val float64) {
		if val == 1 && (attrs["phase"] == "Failed" || attrs["phase"] == "Unknown") {
			unhealthy++
		}
	})
	return []metricsreport.HealthMetricSample{
		{MetricType: metricsreport.MetricPodHealth, Value: float64(unhealthy)},
	}
}

func processPDBStatus(m pmetric.Metric) []metricsreport.HealthMetricSample {
	restrictive := 0
	iterateGauge(m, func(attrs map[string]string, val float64) {
		if val == 0 {
			restrictive++
		}
	})
	return []metricsreport.HealthMetricSample{
		{MetricType: metricsreport.MetricPDBRestriction, Value: float64(restrictive)},
	}
}

// processCustomMetric records all data points for a customer-referenced metric.
func processCustomMetric(name string, m pmetric.Metric) []metricsreport.HealthMetricSample {
	var samples []metricsreport.HealthMetricSample
	iterateGauge(m, func(attrs map[string]string, val float64) {
		samples = append(samples, metricsreport.HealthMetricSample{
			MetricType: metricsreport.MetricCustomRule,
			Target:     name,
			Labels:     attrs,
			Value:      val,
		})
	})
	return samples
}

// --- helpers ---

func iterateGauge(m pmetric.Metric, fn func(attrs map[string]string, val float64)) {
	if m.Type() != pmetric.MetricTypeGauge {
		return
	}
	dps := m.Gauge().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		attrs := make(map[string]string)
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})
		fn(attrs, dp.DoubleValue())
	}
}

func countGauge(m pmetric.Metric) int {
	if m.Type() != pmetric.MetricTypeGauge {
		return 0
	}
	return m.Gauge().DataPoints().Len()
}

var _ = strconv.Itoa // keep strconv import for future use
