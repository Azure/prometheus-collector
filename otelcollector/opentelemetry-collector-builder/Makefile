all: otelcollector promconfigvalidator targetallocator configurationreader

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