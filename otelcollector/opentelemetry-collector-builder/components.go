package main

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
)

func components() (otelcol.Factories, error) {
	promReceiver := prometheusreceiver.NewFactory()
	otlpExporter := otlpexporter.NewFactory()
	promExporter := prometheusexporter.NewFactory()
	batchProcessor := batchprocessor.NewFactory()
	resourceProcessor := resourceprocessor.NewFactory()
	filterProcessor := filterprocessor.NewFactory()

	factories := otelcol.Factories{
		Extensions: map[component.Type]extension.Factory{},
		Receivers: map[component.Type]receiver.Factory{
			promReceiver.Type(): promReceiver,
		},
		Exporters: map[component.Type]exporter.Factory{
			otlpExporter.Type(): otlpExporter,
			promExporter.Type(): promExporter,
		},
		Processors: map[component.Type]processor.Factory{
			batchProcessor.Type():    batchProcessor,
			resourceProcessor.Type(): resourceProcessor,
			filterProcessor.Type():   filterProcessor,
		},
	}

	return factories, nil
}
