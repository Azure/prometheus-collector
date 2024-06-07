package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol"
)

func main() {
	info := component.BuildInfo{
		Command:     "custom-collector-distro",
		Description: "Custom OpenTelemetry Collector distribution",
		Version:     "0.99.0",
	}

	app := otelcol.NewCommand(otelcol.CollectorSettings{BuildInfo: info, Factories: components})
	err := app.Execute()
	if err != nil {
		log.Fatal("collector server run finished with error: %w", err)
	}
}
