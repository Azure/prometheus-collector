# Supported values of windows version are ltsc2019 or ltsc2022 which are being passed manually or by the build pipeline
ARG WINDOWS_VERSION=
FROM mcr.microsoft.com/windows/servercore:${WINDOWS_VERSION}
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="ciprometheus@microsoft.com"
ENV OS_TYPE "windows"
ENV MONITORING_DATA_DIRECTORY "C:\\opt\\genevamonitoringagent\\datadirectory"
ENV MONITORING_MCS_MODE "1"
# Set unfair semaphore wait for better initial CPU performance
ENV COMPlus_ThreadPool_UnfairSemaphoreSpinLimit 0
ENV tmpdir /opt/

# Below is for ContainerInsightsPrometheusCollector-Prod AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH_PUBLIC MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy
# Below is for ContainerInsightsPrometheusCollector-Fairfax AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH_USGOVERNMENT ZmRjMTE0MmUtY2U0YS1mNTFmLWE4M2EtODBjM2ZjNDYwNGE5
# Below is for ContainerInsightsPrometheusCollector-Mooncake AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH_CHINACLOUD ZTcyY2ZjOTYtNjY3Zi1jZGYwLTkwOWMtNzhiZjAwZjQ0NDg4
# Below is for ContainerInsightsPrometheusCollector-USSec AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH_USSEC ZTg4MzFlZGYtNWQ1ZC0wYjZmLTk3MGUtNDkxNTgyYjliMDFl
# Below is for ContainerInsightsPrometheusCollector-USNat AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH_USNAT ZTliNjRmZmUtZDZlYi0xYjczLThjYWQtNDU2OTFjN2FhNzIw
ENV TELEMETRY_DISABLED false

COPY ./build/windows/scripts/setup.ps1 $tmpdir/scripts/
RUN mkdir "C:\\opt\\microsoft\\configmapparser"
RUN mkdir "C:\\opt\\microsoft\\scripts"
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/otelcollector.exe ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml $tmpdir/microsoft/otelcollector/
COPY ./prom-config-validator-builder/promconfigvalidator.exe $tmpdir/
COPY ./prometheus-ui/prometheusui.exe $tmpdir/microsoft/otelcollector/
COPY ./metricextension/me.config ./metricextension/me_internal.config ./metricextension/me_ds.config ./metricextension/me_ds_win.config ./metricextension/me_ds_internal.config ./metricextension/me_ds_internal_win.config $tmpdir/metricextension/
COPY ./fluent-bit/fluent-bit-windows.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/src/out_appinsights.so $tmpdir/fluent-bit/bin/
COPY ./main/main.exe $tmpdir/microsoft
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft

SHELL ["powershell"]
RUN ./opt/scripts/setup.ps1

# Run the Go executable, entrypoint
ENTRYPOINT ["powershell", "C:\\opt\\microsoft\\main.exe"]
