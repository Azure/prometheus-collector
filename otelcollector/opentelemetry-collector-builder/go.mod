module github.com/vishiy/opentelemetry-collector-builder

go 1.17

replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

require (
	github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.73.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.73.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.73.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.73.0
	go.opentelemetry.io/collector v0.73.0
	go.opentelemetry.io/collector/component v0.73.0
	go.opentelemetry.io/collector/exporter/loggingexporter v0.73.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.73.0
	go.opentelemetry.io/collector/extension/zpagesextension v0.73.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.73.0
)
