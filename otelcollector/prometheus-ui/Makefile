PROMETHEUS_VERSION=$(file < ../opentelemetry-collector-builder/PROMETHEUS_VERSION)
.PHONY: prometheusui
prometheusui:
	rm -rf static
	wget https://github.com/prometheus/prometheus/releases/download/v$(PROMETHEUS_VERSION)/prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz
	tar -xvzf prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz
	rm prometheus-web-ui-$(PROMETHEUS_VERSION).tar.gz
	rm -rf prometheusui
	go get
	go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o prometheusui .