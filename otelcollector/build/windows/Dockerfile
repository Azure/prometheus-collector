FROM mcr.microsoft.com/windows/servercore:ltsc2019
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="omscontainers@microsoft.com"
ENV OS_TYPE "windows"
ENV tmpdir /opt/microsoft/

# Below is for ContainerInsightsPrometheusCollector-Prod AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH "MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy"

COPY ./build/windows/scripts/setup.ps1 $tmpdir/scripts/
COPY ./build/windows/scripts/main.ps1 $tmpdir/scripts/
COPY ./build/windows/scripts/livenessprobe.cmd $tmpdir/scripts/

COPY ../../configmapparser/*.rb $tmpdir/configmapparser/
COPY ../../opentelemetry-collector-builder/otelcollector.exe ../../opentelemetry-collector-builder/collector-config-default.yml ../../opentelemetry-collector-builder/collector-config-template.yml $tmpdir/otelcollector/

COPY ../../metricextension/me.config ../../metricextension/me_ds.config $tmpdir/metricextension/
COPY ../../telegraf/telegraf-prometheus-collector.conf $tmpdir/telegraf/
COPY ../../fluent-bit/fluent-bit.conf $tmpdir/fluent-bit/
COPY ../../fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY ../../fluent-bit/src/out_appinsights.so $tmpdir/fluent-bit/bin/

# Do not split this into multiple RUN!
# Docker creates a layer for every RUN-Statement
RUN powershell -Command "Set-ExecutionPolicy Bypass -Scope Process -Force; iex ((New-Object System.Net.WebClient).DownloadString('https://chocolatey.org/install.ps1'))"
# Fluentd depends on cool.io whose fat gem is only available for Ruby < 2.5, so need to specify --platform ruby when install Ruby > 2.5 and install msys2 to get dev tools
RUN choco install -y ruby --version 2.6.5.1 --params "'/InstallDir:C:\ruby26'" \
&& choco install -y msys2 --version 20210604.0.0 --params "'/NoPath /NoUpdate /InstallDir:C:\ruby26\msys64'" \
&& choco install -y vim

# gangams - optional MSYS2 update via ridk failing in merged docker file so skipping that since we dont need optional update
RUN refreshenv \
&& ridk install 3 \
&& echo gem: --no-document >> C:\ProgramData\gemrc \
&& gem install cool.io -v 1.5.4 --platform ruby \
&& gem install oj -v 3.3.10 \
&& gem install json -v 2.2.0 \
&& gem install fluentd -v 1.12.2 \
&& gem install win32-service -v 1.0.1 \
&& gem install win32-ipc -v 0.7.0 \
&& gem install win32-event -v 0.6.3 \
&& gem install windows-pr -v 1.2.6 \
&& gem install tomlrb -v 1.3.0 \
&& gem install gyoku -v 1.3.1 \
&& gem sources --clear-all

# Remove gem cache and chocolatey
RUN powershell -Command "Remove-Item -Force C:\ruby26\lib\ruby\gems\2.6.0\cache\*.gem; Remove-Item -Recurse -Force 'C:\ProgramData\chocolatey'"

SHELL ["powershell"]
RUN ./opt/microsoft/scripts/setup.ps1


ENTRYPOINT ["powershell", "C:\\opt\\scripts\\main.ps1"]