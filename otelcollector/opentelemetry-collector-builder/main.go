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

	info := component.ApplicationStartInfo{
		ExeName:  "custom-collector-distro",
		LongName: "Custom OpenTelemetry Collector distribution",
		Version:  "1.0.0",
	}

	app, err := service.New(service.Parameters{ApplicationStartInfo: info, Factories: factories})
	if err != nil {
		log.Fatal("failed to construct the application: %w", err)
	}

	err = app.Run()
	if err != nil {
		log.Fatal("application run finished with error: %w", err)
	}
}