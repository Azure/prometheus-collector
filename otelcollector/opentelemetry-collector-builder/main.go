package main

import (
	"log"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/service"
)

func main() {
	factories, err := components()
	if err != nil {
		log.Fatalf("failed to build components: %v", err)
	}
	info := component.BuildInfo{
		Command:     "custom-collector-distro",
		Description: "Custom OpenTelemetry Collector distribution",
		Version:     "0.51.0",
	}

	app := service.NewCommand(service.CollectorSettings{BuildInfo: info, Factories: factories})
	err = app.Execute()
	if err != nil {
		log.Fatal("collector server run finished with error: %w", err)
	}
}