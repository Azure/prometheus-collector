module github.com/rashmy/promreceiver-config-validator-builder

go 1.14

replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

require (
	github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector v0.27.0
)
