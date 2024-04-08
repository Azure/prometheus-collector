PROMETHEUS_VERSION=$(file < PROMETHEUS_VERSION)
all: otelcollector fluentbitplugin promconfigvalidator targetallocator

.PHONY: otelcollector
otelcollector:
	rm -rf otelcollector
	go get
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o otelcollector .

fluentbitplugin:
	make -C ../fluent-bit/src

promconfigvalidator:
	make -C ../prom-config-validator-builder

targetallocator:
	make -C ../otel-allocator

configurationreader:
	make -C ../configuration-reader-builder

prometheusui:
	make -C ../prometheus-ui