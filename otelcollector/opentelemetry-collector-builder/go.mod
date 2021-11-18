module github.com/vishiy/opentelemetry-collector-builder

go 1.14

replace github.com/vishiy/influxexporter => ../influxexporter

replace github.com/gracewehner/prometheusreceiver => ../prometheusreceiver

require (
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/cenkalti/backoff/v4 v4.1.1 // indirect
	github.com/containerd/containerd v1.4.12 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/gracewehner/prometheusreceiver v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.mongodb.org/mongo-driver v1.5.1 // indirect
	//github.com/vishiy/influxexporter v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/collector v0.27.0
	golang.org/x/sys v0.0.0-20210426230700-d19ff857e887 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)
