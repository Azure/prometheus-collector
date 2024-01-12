PROMETHEUS_VERSION=$(file < PROMETHEUS_VERSION)
all: otelcollector fluentbitplugin promconfigvalidator targetallocator

.PHONY: otelcollector
otelcollector:
	rm -rf otelcollector
	go get
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o otelcollector .

prometheusui:
	echo $(PROMETHEUS_VERSION)
	wget https://github.com/prometheus/prometheus/releases/download/v$(PROMETHEUS_VERSION)/prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz
	tar -xvzf prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz
	rm -rf ../react/static
	mv static ../react
	rm prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz

fluentbitplugin:
	make -C ../fluent-bit/src

promconfigvalidator:
	make -C ../prom-config-validator-builder

targetallocator:
	make -C ../otel-allocator

configurationreader:
	make -C ../configuration-reader-builder