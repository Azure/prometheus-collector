PROMETHEUS_VERSION=$(file < PROMETHEUS_VERSION)
all: otelcollector fluentbitplugin promconfigvalidator targetallocator configurationreader prometheusui

.PHONY: otelcollector
otelcollector:
	rm -rf otelcollector
	go get
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o otelcollector .

fluentbitplugin:
	make -C ../fluent-bit/src

promconfigvalidator:
	make -C ../prom-config-validator-builder

prometheusui:
	make -C ../prometheus-ui

targetallocator:
	make -C ../otel-allocator

configurationreader:
	make -C ../configuration-reader-builder