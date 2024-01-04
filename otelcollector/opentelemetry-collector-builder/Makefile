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

# add this back once golang 1.21 is supported in ado builds
targetallocator:
	make -C ../otel-allocator

configurationreader:
	make -C ../configuration-reader-builder