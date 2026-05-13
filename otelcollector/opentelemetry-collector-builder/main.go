package main

import (
	"context"
	"log"
	"os"
	"time"

	metricsreport "prometheus-collector/metricsreport/controller"

	"github.com/vishiy/opentelemetry-collector-builder/healthcacheexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/provider/envprovider"
	"go.opentelemetry.io/collector/confmap/provider/fileprovider"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	// Initialize the shared MetricsCache and UpgradeGate before the collector
	// pipeline starts. The health_cache exporter and the HealthSignal controller
	// both reference these via healthcacheexporter.SharedState.
	metricsCache := metricsreport.NewMetricsCache(1*time.Hour, 15*time.Second)
	healthcacheexporter.SharedState.Cache = metricsCache

	// Start the HealthSignal controller in a background goroutine.
	// It watches HealthCheckRequest CRs and writes HealthSignal CRs using
	// the shared cache (populated by the health_cache exporter in the pipeline).
	if os.Getenv("OS_TYPE") != "windows" {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := metricsreport.StartHealthSignalControllerWithCache(ctx, metricsCache); err != nil {
			log.Printf("Warning: Failed to start HealthSignal controller: %v\n", err)
		} else {
			// Set the UpgradeGate so the exporter can load customer rules.
			healthcacheexporter.SharedState.UpgradeGate = metricsreport.GetUpgradeGate()
		}
	}

	info := component.BuildInfo{
		Command:     "custom-collector-distro",
		Description: "Custom OpenTelemetry Collector distribution",
		Version:     "0.150.0",
	}

	set := otelcol.CollectorSettings{
		BuildInfo: info,
		Factories: components,
		ConfigProviderSettings: otelcol.ConfigProviderSettings{
			ResolverSettings: confmap.ResolverSettings{
				ProviderFactories: []confmap.ProviderFactory{
					envprovider.NewFactory(),
					fileprovider.NewFactory(),
				},
			},
		},
	}

	app := otelcol.NewCommand(set)
	err := app.Execute()
	if err != nil {
		log.Fatal("collector server run finished with error: %w", err)
	}
}
