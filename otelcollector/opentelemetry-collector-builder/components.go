package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/processor/filterprocessor"
	"go.opentelemetry.io/collector/processor/memorylimiter"
	"go.opentelemetry.io/collector/processor/resourceprocessor"
	"go.opentelemetry.io/collector/exporter/loggingexporter"
	"go.opentelemetry.io/collector/exporter/prometheusexporter"
	"go.opentelemetry.io/collector/exporter/fileexporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"github.com/vishiy/influxexporter"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
	"go.opentelemetry.io/collector/extension/healthcheckextension"
	"go.opentelemetry.io/collector/extension/pprofextension"
	"go.opentelemetry.io/collector/extension/zpagesextension"
)

func components() (component.Factories, error) {
	var errs []error

	processors, err := component.MakeProcessorFactoryMap (
		batchprocessor.NewFactory(),
		memorylimiter.NewFactory(),
		resourceprocessor.NewFactory(),
		filterprocessor.NewFactory(),
	)

	if err != nil {
		errs = append(errs,err)
	}

	receivers, err := component.MakeReceiverFactoryMap (
		privatepromreceiver.NewFactory(),
	)

	if err != nil {
		errs = append(errs,err)
	}

	exporters, err := component.MakeExporterFactoryMap (
		loggingexporter.NewFactory(),
		prometheusexporter.NewFactory(),
		fileexporter.NewFactory(),
		otlpexporter.NewFactory(),
		otlphttpexporter.NewFactory(),
		influxexporter.NewFactory(),
	)

	if err != nil {
		errs = append(errs,err)
	}

	extensions, err := component.MakeExtensionFactoryMap (
		healthcheckextension.NewFactory(),
		pprofextension.NewFactory(),
		zpagesextension.NewFactory(),
	)

	if err != nil {
		errs = append(errs,err)
	}
	factories := component.Factories{
		Processors : processors,
		Receivers : receivers,
		Exporters : exporters,
		Extensions : extensions,
	}
	return factories, consumererror.CombineErrors(errs)
}