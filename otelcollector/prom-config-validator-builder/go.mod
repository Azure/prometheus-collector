module github.com/rashmy/prom-config-validator-builder

go 1.18

//replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

require (
	//github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.62.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.62.0
	go.opentelemetry.io/collector v0.62.1
)
