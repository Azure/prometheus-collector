package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	//"github.com/vishiy/influxexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	//privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	//"go.opentelemetry.io/collector/extension/healthcheckextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
)

func components() (component.Factories, error) {
  var err error
	factories := component.Factories{}

	factories.Processors, err = component.MakeProcessorFactoryMap(
		batchprocessor.NewFactory(),
		resourceprocessor.NewFactory(),
	)
	if err != nil {
		return component.Factories{}, err
	}

	factories.Receivers, err = component.MakeReceiverFactoryMap(
		prometheusreceiver.NewFactory(),
	)
	if err != nil {
		return component.Factories{}, err
	}

	factories.Exporters, err = component.MakeExporterFactoryMap(
		loggingexporter.NewFactory(),
		otlpexporter.NewFactory(),
		fileexporter.NewFactory(),
		prometheusexporter.NewFactory(),
	)
	if err != nil {
		return component.Factories{}, err
	}

	factories.Extensions, err = component.MakeExtensionFactoryMap(
		pprofextension.NewFactory(),
		zpagesextension.NewFactory(),
	)
	if err != nil {
		return component.Factories{}, err
	}

	return factories, nil
}