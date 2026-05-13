package healthcacheexporter

import (
	"context"

	metricsreport "prometheus-collector/metricsreport/controller"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

// SharedState holds the in-process objects shared between the exporter and the
// HealthSignal controller. Set by the collector main.go before the pipeline starts.
var SharedState struct {
	Cache       *metricsreport.MetricsCache
	UpgradeGate *metricsreport.UpgradeGate
}

// NewFactory creates a factory for the health_cache exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		componentType,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsExporter(
	_ context.Context,
	_ exporter.Settings,
	_ component.Config,
) (exporter.Metrics, error) {
	sink := metricsreport.NewHealthMetricSink(SharedState.Cache)

	return &healthCacheExporter{
		sink:              sink,
		upgradeGate:       SharedState.UpgradeGate,
		customMetricNames: make(map[string]bool),
	}, nil
}
