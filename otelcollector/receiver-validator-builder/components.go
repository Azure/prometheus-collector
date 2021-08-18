package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"

	//"github.com/vishiy/influxexporter"
	//"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	privatepromreceiver "github.com/gracewehner/prometheusreceiver"
)

func components() (component.Receiver, error) {
	var errs []error

	receiver, err := privatepromreceiver.NewFactory()

	if err != nil {
		errs = append(errs, err)
	}

	return receiver, consumererror.Combine(errs)
}
