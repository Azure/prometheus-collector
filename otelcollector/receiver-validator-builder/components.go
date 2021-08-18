package main

import (
	"go.opentelemetry.io/collector/component"

	//"github.com/vishiy/influxexporter"
	//"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
)

func components() component.ReceiverFactory {
	// var errs []error

	receiver := privatepromreceiver.NewFactory()

	// if err != nil {
	// 	errs = append(errs, err)
	// }

	return receiver
}
