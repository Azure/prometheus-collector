// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"context"
	// "errors"

	promconfig "github.com/prometheus/prometheus/config"
	_ "github.com/prometheus/prometheus/discovery/install" // init() of this package registers service discovery impl.

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	// "go.uber.org/zap"
	// "github.com/gracewehner/prometheusreceiver/internal"
)

// This file implements config for Prometheus receiver.

const (
	typeStr = "prometheus"
)

// var (
// 	errNilScrapeConfig = errors.New("Rashmi-prom-collector-logs ----------please provide some scrape configs--------------------")
// )

// NewFactory creates a new Prometheus receiver factory.
func NewFactory() component.ReceiverFactory {
	return receiverhelper.NewFactory(
		typeStr,
		// createDefaultConfig(params),
		createDefaultConfig,
		receiverhelper.WithMetrics(createMetricsReceiver))
}

func createDefaultConfig() config.Receiver {
	// func createDefaultConfig(params component.ReceiverCreateParams) config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		// logger : params.Logger,
	}
}

func createCustomConfig(cfg *promconfig.Config) config.Receiver {
	// func createDefaultConfig(params component.ReceiverCreateParams) config.Receiver {
	return &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewID(typeStr)),
		PrometheusConfig: cfg,
		// logger : params.Logger,
	}
}

func createMetricsReceiver(
	_ context.Context,
	params component.ReceiverCreateParams,
	cfg config.Receiver,
	nextConsumer consumer.Metrics,
) (component.MetricsReceiver, error) {
	return newPrometheusReceiver(params.Logger, cfg.(*Config), nextConsumer), nil
}
