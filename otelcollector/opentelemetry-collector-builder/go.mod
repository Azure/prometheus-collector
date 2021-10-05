module github.com/vishiy/opentelemetry-collector-builder

go 1.14

replace github.com/vishiy/influxexporter => ../influxexporter

replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

//replace github.com/gracewehner/web => ../web

require (
	github.com/go-kit/log v0.2.0 // indirect
	github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector v0.27.0
)
