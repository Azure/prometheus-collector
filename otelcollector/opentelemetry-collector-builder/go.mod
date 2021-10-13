module github.com/vishiy/opentelemetry-collector-builder

go 1.14

replace github.com/vishiy/influxexporter => ../influxexporter

replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

//replace github.com/gracewehner/web => ../web

//replace github.com/gracewehner/web/ui => ../web/ui

require (
	github.com/go-kit/log v0.2.0 // indirect
	//github.com/go-kit/log v0.2.0 // indirect
	//github.com/gracewehner/prometheus/web/ui v0.0.0-20211006212255-4c61f7b7c4cc // indirect
	github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	//github.com/prometheus/statsd_exporter v0.22.1 // indirect
	go.opentelemetry.io/collector v0.27.0
)
