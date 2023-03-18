package main

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/service"

	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	//"go.opentelemetry.io/collector/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
)

func components() (service.Factories, error) {
	var err error
	factories := service.Factories{}

	factories.Processors, err = service.MakeProcessorFactoryMap(
		batchprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
	)
	if err != nil {
		return service.Factories{}, err
	}

	factories.Receivers, err = service.MakeReceiverFactoryMap(
		privatepromreceiver.NewFactory(),
	)
	if err != nil {
		return service.Factories{}, err
	}

	factories.Exporters, err = service.MakeExporterFactoryMap(
		loggingexporter.NewFactory(),
		otlpexporter.NewFactory(),
		fileexporter.NewFactory(),
		prometheusexporter.NewFactory(),
	)
	if err != nil {
		return service.Factories{}, err
	}

	factories.Extensions, err = service.MakeExtensionFactoryMap(
		pprofextension.NewFactory(),
		zpagesextension.NewFactory(),
	)
	if err != nil {
		return service.Factories{}, err
	}

	return factories, nil
}
