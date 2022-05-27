FROM ubuntu:20.04
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="omscontainers@microsoft.com"
# ARG IMAGE_TAG="test"
# ENV AGENT_VERSION ${IMAGE_TAG}
ENV OS_TYPE "linux"
ENV DEBIAN_FRONTEND=noninteractive
ENV AZMON_COLLECT_ENV False
ENV tmpdir /opt
# Below is for ContainerInsightsPrometheusCollector-Prod AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy
COPY ./logrotate/prometheus-collector /etc/logrotate.d/
RUN apt-get update && apt-get install -y libc-bin wget openssl curl sudo init-system-helpers net-tools cron vim apt-transport-https locales ruby gnupg logrotate ucf
RUN apt-get upgrade libsystemd0 -y
COPY ./scripts/livenessprobe.sh $tmpdir/microsoft/liveness/livenessprobe.sh
COPY ./configmapparser/*.rb $tmpdir/microsoft/configmapparser/
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/otelcollector ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml $tmpdir/microsoft/otelcollector/
COPY ./prom-config-validator-builder/promconfigvalidator $tmpdir/
COPY ./scripts/setup.sh ./scripts/main.sh $tmpdir/
COPY ./metricextension/me.config ./metricextension/me_internal.config ./metricextension/me_ds.config ./metricextension/me_ds_internal.config /usr/sbin/
COPY ./telegraf/telegraf-prometheus-collector.conf $tmpdir/telegraf/
COPY ./fluent-bit/fluent-bit.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/src/out_appinsights.so $tmpdir/fluent-bit/bin/
COPY ./react /static/react
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft
COPY ./mdsd/envmdsd $tmpdir/

RUN chmod 775 $tmpdir/*.sh; sync; $tmpdir/setup.sh
ENTRYPOINT [ "/opt/main.sh" ]
