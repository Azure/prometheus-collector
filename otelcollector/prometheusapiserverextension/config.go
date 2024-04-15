// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusapiserverextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/prometheusapiserverextension"

type Config struct {
	PrometheusReceiverName string `mapstructure:"prometheus_receiver_name"`
	Endpoint string `mapstructure:"endpoint"`
}
