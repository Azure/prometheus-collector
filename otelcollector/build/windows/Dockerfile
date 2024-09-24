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
COPY ./build/windows/scripts/main.ps1 $tmpdir/scripts/
COPY ./build/windows/scripts/filesystemwatcher.ps1 $tmpdir/scripts/
COPY ./build/windows/scripts/livenessprobe.cmd $tmpdir/microsoft/scripts/
COPY ./configmapparser/*.rb $tmpdir/microsoft/configmapparser/
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/otelcollector.exe ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml $tmpdir/microsoft/otelcollector/
COPY ./prom-config-validator-builder/promconfigvalidator.exe $tmpdir/
COPY ./metricextension/me.config ./metricextension/me_internal.config ./metricextension/me_ds.config ./metricextension/me_ds_win.config ./metricextension/me_ds_internal.config ./metricextension/me_ds_internal_win.config $tmpdir/metricextension/
COPY ./telegraf/telegraf-prometheus-collector-windows.conf $tmpdir/telegraf/
COPY ./fluent-bit/fluent-bit-windows.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY ./fluent-bit/src/out_appinsights.so $tmpdir/fluent-bit/bin/
COPY ./main/main.exe $tmpdir/microsoft
COPY ./react /Users/ContainerAdministrator/Documents/static/react
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft

# Do not split this into multiple RUN!
# Docker creates a layer for every RUN-Statement
ENV chocolateyVersion "1.4.0"
RUN powershell -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))"
# Fluentd depends on cool.io whose fat gem is only available for Ruby < 2.5, so need to specify --platform ruby when install Ruby > 2.5 and install msys2 to get dev tools
# RUN choco install -y ruby --version 2.6.5.1 --params "'/InstallDir:C:\ruby26'" \
# && choco install -y msys2 --version 20211130.0.0 --params "'/NoPath /NoUpdate /InstallDir:C:\ruby26\msys64'" \
# && choco install -y vim

# # gangams - optional MSYS2 update via ridk failing in merged docker file so skipping that since we dont need optional update
# RUN refreshenv \
# && ridk install 3 \
# && echo gem: --no-document >> C:\ProgramData\gemrc \
# && gem install cool.io -v 1.5.4 --platform ruby \
# && gem install oj -v 3.3.10 \
# && gem install json -v 2.2.0 \
# && gem install fluentd -v 1.12.2 \
# && gem install win32-service -v 1.0.1 \
# && gem install win32-ipc -v 0.7.0 \
# && gem install win32-event -v 0.6.3 \
# && gem install windows-pr -v 1.2.6 \
# && gem install tomlrb -v 1.3.0 \
# && gem install deep_merge -v 1.2.1\
# && gem install colorize\
# && gem sources --clear-all

# # Remove gem cache and chocolatey
# RUN powershell -Command "Remove-Item -Force C:\ruby26\lib\ruby\gems\2.6.0\cache\*.gem; Remove-Item -Recurse -Force 'C:\ProgramData\chocolatey'"

SHELL ["powershell"]
RUN ./opt/scripts/setup.ps1

# Run the Go executable, entrypoint
ENTRYPOINT ["powershell", "C:\\opt\\microsoft\\main.exe"]
# ENTRYPOINT ["powershell", "C:\\opt\\scripts\\main.ps1"]

