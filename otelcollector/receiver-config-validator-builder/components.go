package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"

	//"github.com/vishiy/influxexporter"
	//"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
)

func components() (component.Factories, error) {
	var errs []error

	// processors, err := component.MakeProcessorFactoryMap(
	// 	batchprocessor.NewFactory(),
	// 	memorylimiter.NewFactory(),
	// 	resourceprocessor.NewFactory(),
	// 	filterprocessor.NewFactory(),
	// )

	// if err != nil {
	// 	errs = append(errs, err)
	// }

	receivers, err := component.MakeReceiverFactoryMap(
		privatepromreceiver.NewFactory(),
	)

	if err != nil {
		errs = append(errs, err)
	}

	// exporters, err := component.MakeExporterFactoryMap(
	// 	loggingexporter.NewFactory(),
	// 	prometheusexporter.NewFactory(),
	// 	fileexporter.NewFactory(),
	// 	otlpexporter.NewFactory(),
	// 	otlphttpexporter.NewFactory(),
	// 	//influxexporter.NewFactory(),
	// )

	// if err != nil {
	// 	errs = append(errs, err)
	// }

	// extensions, err := component.MakeExtensionFactoryMap(
	// 	healthcheckextension.NewFactory(),
	// 	pprofextension.NewFactory(),
	// 	zpagesextension.NewFactory(),
	// )

	// if err != nil {
	// 	errs = append(errs, err)
	// }
	factories := component.Factories{
		// Processors: processors,
		Receivers: receivers,
		// Exporters:  exporters,
		// Extensions: extensions,
	}
	return factories, consumererror.Combine(errs)
}
