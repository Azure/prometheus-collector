BASE_DIR := $(subst /build/linux,,$(PWD))

all: otelcollector fluentbitplugin

.PHONY: otelcollector
otelcollector:
	rm -rf otelcollector
	go get
	go build -o otelcollector .

fluentbitplugin:
	make -C ../fluent-bit/src