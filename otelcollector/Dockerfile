FROM ubuntu:20.04
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="omscontainers@microsoft.com"
ARG IMAGE_TAG="test"
ENV AGENT_VERSION ${IMAGE_TAG}
ENV DEBIAN_FRONTEND=noninteractive
ENV AZMON_COLLECT_ENV False
ENV tmpdir /opt
# Below is for ContainerInsightsPrometheusCollector-Prod AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy
RUN apt-get update && apt-get install -y libc-bin wget openssl curl sudo init-system-helpers net-tools cron vim apt-transport-https locales ruby-full gnupg
COPY ./scripts/livenessprobe.sh $tmpdir/microsoft/liveness/livenessprobe.sh
COPY ./configmapparser/*.rb $tmpdir/microsoft/configmapparser/
COPY ./opentelemetry-collector-builder/otelcollector ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml $tmpdir/microsoft/otelcollector/
COPY ./scripts/setup.sh ./scripts/main.sh $tmpdir/
COPY ./metricextension/me.config /usr/sbin/
COPY ./telegraf/telegraf-prometheus-collector.conf $tmpdir/telegraf/
COPY ./fluent-bit/fluent-bit.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/src/out_appinsights.so $tmpdir/fluent-bit/bin/
RUN chmod 775 $tmpdir/*.sh; sync; $tmpdir/setup.sh
CMD [ "/opt/main.sh" ]
