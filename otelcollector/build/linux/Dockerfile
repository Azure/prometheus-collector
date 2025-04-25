ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as fluent-bit-builder
WORKDIR /src
COPY ./fluent-bit/src/go.mod ./fluent-bit/src/go.sum .
RUN go version
RUN go mod download
COPY ./fluent-bit/src/ .
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=c-shared -ldflags '-extldflags=-Wl,-z,now' -o out_appinsights.so . ; else  CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=c-shared -ldflags '-extldflags=-Wl,-z,now' -o out_appinsights.so . ; fi

ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as prometheus-ui
WORKDIR /src
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
RUN go version > goversion.txt
COPY ./prometheus-ui/go.mod ./prometheus-ui/go.sum ./prometheus-ui/
WORKDIR /src/prometheus-ui
RUN go mod download
COPY ./prometheus-ui /src/prometheus-ui
ARG TARGETOS TARGETARCH PROMETHEUS_VERSION
RUN echo $PROMETHEUS_VERSION
RUN wget https://github.com/prometheus/prometheus/releases/download/v$PROMETHEUS_VERSION/prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz
RUN tar -xvzf prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz
RUN rm -rf prometheus-web-ui-$PROMETHEUS_VERSION.tar.gz
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o prometheusui . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o prometheusui . ; fi

ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as otelcollector-builder
WORKDIR /src
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
RUN go version > goversion.txt
COPY ./opentelemetry-collector-builder/go.mod ./opentelemetry-collector-builder/go.sum ./opentelemetry-collector-builder/
COPY ./prometheusreceiver/go.mod ./prometheusreceiver/go.sum ./prometheusreceiver/
WORKDIR /src/prometheusreceiver
RUN go mod download
WORKDIR /src/opentelemetry-collector-builder
RUN go mod download
COPY ./opentelemetry-collector-builder /src/opentelemetry-collector-builder
COPY ./prometheusreceiver /src/prometheusreceiver
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o otelcollector . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o otelcollector . ; fi

ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as prom-config-validator-builder
WORKDIR /src
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
COPY ./prom-config-validator-builder/go.mod ./prom-config-validator-builder/go.sum ./prom-config-validator-builder/
COPY ./prometheusreceiver/go.mod ./prometheusreceiver/go.sum ./prometheusreceiver/
WORKDIR /src/prometheusreceiver
RUN go version
RUN go mod download
WORKDIR /src/prom-config-validator-builder
RUN go mod download
COPY ./prom-config-validator-builder /src/prom-config-validator-builder
COPY ./prometheusreceiver /src/prometheusreceiver
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; fi

ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as main-builder
WORKDIR /
# Create directories
RUN mkdir -p ./shared/configmap/mp/
RUN mkdir -p ./main/
# Copy shared go files
COPY ../shared/*.go ./main/shared/
COPY ./shared/go.mod ./main/shared/
COPY ./shared/go.sum ./main/shared/
COPY ../shared/configmap/mp/*.go ./main/shared/configmap/mp/
COPY ../shared/configmap/ccp/*.go ./main/shared/configmap/ccp/
COPY ./shared/configmap/mp/go.mod ./main/shared/configmap/mp/
COPY ./shared/configmap/mp/go.sum ./main/shared/configmap/mp/
COPY ./shared/configmap/ccp/go.mod ./main/shared/configmap/ccp/
COPY ./shared/configmap/ccp/go.sum ./main/shared/configmap/ccp/
# Copy main go files
COPY ./main/*.go ./main/
COPY ./go.mod ./main/
COPY ./go.sum ./main/
WORKDIR /main
RUN go version
RUN go mod download
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=exe -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main.exe ./main.go ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=exe -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main.exe ./main.go ; fi

ARG TARGETARCH
FROM mcr.microsoft.com/cbl-mariner/base/core:2.0 AS fluent-bit-binary-builder
WORKDIR /
# Install with the same exact dependencies and code source that Mariner uses
RUN tdnf install wget tar ca-certificates bison cmake cyrus-sasl-devel doxygen flex gcc-c++ \
    gnutls-devel graphviz libpq-devel libyaml-devel luajit-devel make openssl-devel pkgconfig \
    systemd-devel systemd-rpm-macros zlib-devel build-essential -y
ARG FLUENT_BIT_VERSION
RUN wget https://github.com/fluent/fluent-bit/archive/refs/tags/v${FLUENT_BIT_VERSION}.tar.gz
RUN tar -xvf v${FLUENT_BIT_VERSION}.tar.gz
# Add a file with settings to build only the plugins we use
COPY ./fluent-bit/plugins_options.cmake /fluent-bit-${FLUENT_BIT_VERSION}/cmake/plugins_options.cmake
# Make a change that allows Fluent-Bit metrics to flow to our Go output plugin
RUN sed -i '/out->type  = FLB_OUTPUT_PLUGIN_PROXY;/a \ \ \ \ out->event_type = FLB_OUTPUT_LOGS | FLB_OUTPUT_METRICS;' /fluent-bit-${FLUENT_BIT_VERSION}/src/flb_plugin_proxy.c
WORKDIR /fluent-bit-${FLUENT_BIT_VERSION}/build
# Run cmake with the same flags that Mariner uses
RUN cmake \
-DCMAKE_BUILD_TYPE=RelWithDebInfo \
-DFLB_EXAMPLES=Off \
-DFLB_OUT_SLACK=Off \
-DFLB_IN_SYSTEMD=On \
-DFLB_OUT_TD=Off \
-DFLB_OUT_ES=Off \
-DFLB_SHARED_LIB=On \
-DFLB_RELEASE=On \
-DFLB_DEBUG=Off \
-DFLB_TLS=On \
-DFLB_JEMALLOC=On \
-DFLB_PREFER_SYSTEM_LIBS=On \
-DFLB_PROXY_GO=On ../
RUN make
RUN make install

FROM mcr.microsoft.com/cbl-mariner/base/core:2.0 as builder
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="ciprometheus@microsoft.com"
ENV OS_TYPE "linux"
ENV tmpdir /opt
# Below is for ContainerInsightsPrometheusCollector-Prod AppInsights Resource
ENV APPLICATIONINSIGHTS_AUTH MWNkYTMxMTItYWY1Ni00ZmNiLWI4MDQtZjg5NDVhYTFjYjMy
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
# Needed for ME, see https://github.com/microsoft/cpprestsdk/issues/1481
ENV MALLOC_ARENA_MAX=2
COPY ./logrotate/prometheus-collector /etc/logrotate.d/
COPY ./logrotate/logrotate /etc/cron.daily/logrotate
COPY ./logrotate/crontab /etc/crontab
RUN mkdir -p $tmpdir/microsoft/configmapparser/
RUN mkdir -p $tmpdir/microsoft/liveness/
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml ./opentelemetry-collector-builder/collector-config-replicaset.yml ./opentelemetry-collector-builder/PROMETHEUS_VERSION $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/opentelemetry-collector-builder/otelcollector $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/opentelemetry-collector-builder/otelcollector $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/goversion.txt $tmpdir/goversion.txt
COPY --from=prom-config-validator-builder /src/prom-config-validator-builder/promconfigvalidator $tmpdir/
COPY --from=prometheus-ui /src/prometheus-ui/prometheusui $tmpdir/microsoft/otelcollector/
COPY --from=main-builder --chmod=777 /main/main.exe $tmpdir/main

COPY ./scripts/*.sh $tmpdir/
COPY ./metricextension/me.config ./metricextension/me_internal.config ./metricextension/me_ds.config ./metricextension/me_ds_internal.config /usr/sbin/
COPY ./fluent-bit/fluent-bit.yaml ./fluent-bit/fluent-bit-daemonset.yaml ./fluent-bit/fluent-bit-parsers.conf $tmpdir/fluent-bit/
COPY --from=fluent-bit-builder /src/out_appinsights.so $tmpdir/fluent-bit/bin/
COPY --from=fluent-bit-binary-builder /usr/local/bin/fluent-bit /usr/local/bin/fluent-bit
COPY --from=fluent-bit-binary-builder /usr/local/etc/fluent-bit /usr/local/etc/fluent-bit
COPY --from=fluent-bit-binary-builder /usr/local/lib/fluent-bit /usr/local/etc/fluent-bit
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft
COPY ./mdsd/envmdsd $tmpdir/
COPY ./build/linux/rpm-repos/ /etc/yum.repos.d/

ARG TARGETARCH
RUN tdnf clean all
RUN tdnf repolist --refresh
RUN tdnf update -y
RUN tdnf install -y wget sudo net-tools cronie vim logrotate procps-ng busybox diffutils curl
RUN mkdir /busybin && busybox --install /busybin
RUN chmod 775 /etc/cron.daily/logrotate
RUN chmod 775 $tmpdir/*.sh;
RUN sync;
RUN $tmpdir/setup.sh ${TARGETARCH}
# If wanting to run without distroless, uncomment this line and comment everything after
# ENTRYPOINT ["./opt/main"]

FROM mcr.microsoft.com/cbl-mariner/distroless/base:2.0
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
# Set environment variables for mdsd
ENV MDSD_LOG="/opt/microsoft/linuxmonagent"
ENV SKIP_IMDS_LOOKUP_FOR_LEGACY_AUTH="true"
ENV MDSD_FLUENT_SOCKET_PORT="28230"
ENV ENABLE_MCS="true"
ENV MONITORING_USE_GENEVA_CONFIG_SERVICE="false"
ENV MDSD_USE_LOCAL_PERSISTENCY="false"
ENV SSL_CERT_FILE="/etc/pki/tls/certs/ca-bundle.crt"
ENV TELEMETRY_DISABLED false
# Needed for ME, see https://github.com/microsoft/cpprestsdk/issues/1481
ENV MALLOC_ARENA_MAX=2
ENV PATH="/busybin:${PATH}"
ENV OS_TYPE "linux"

# files
COPY --from=builder /opt /opt
COPY --from=builder /etc /etc
COPY --from=builder /busybin /busybin
COPY --from=builder /usr/sbin/me.config /usr/sbin/me_internal.config /usr/sbin/me_ds.config /usr/sbin/me_ds_internal.config /usr/sbin/
COPY --from=builder /var/opt/microsoft /var/opt/microsoft
COPY --from=builder /var/lib/logrotate /var/lib/logrotate
COPY --from=builder /var/spool/cron /var/spool/cron
COPY --from=builder /usr/share/p11-kit /usr/share/p11-kit
COPY --from=builder /usr/share/pki/ /usr/share/pki
COPY --from=builder /opt/microsoft/liveness /opt/microsoft/liveness
COPY --from=builder /opt/microsoft/configmapparser /opt/microsoft/configmapparser

# executables
COPY --from=builder /usr/sbin/MetricsExtension /usr/sbin/MetricsExtension
COPY --from=builder /usr/bin/inotifywait /usr/bin/inotifywait
COPY --from=builder /usr/bin/bash /usr/bin/bash
COPY --from=builder /usr/sbin/busybox /usr/sbin/busybox
COPY --from=builder /usr/sbin/crond /usr/sbin/crond
COPY --from=builder /usr/bin/vim /usr/bin/vim
COPY --from=builder /usr/share/vim /usr/share/vim
COPY --from=builder /usr/sbin/mdsd /usr/sbin/mdsd
COPY --from=builder /usr/sbin/logrotate /usr/sbin/logrotate
COPY --from=builder /usr/bin/gzip /usr/bin/
COPY --from=builder /usr/bin/curl /usr/bin/
COPY --from=builder /usr/bin/update-ca-trust /usr/bin
COPY --from=builder /bin/sh /bin/sh
COPY --from=builder /usr/bin/p11-kit /usr/bin
COPY --from=builder /usr/bin/trust /usr/bin

COPY --from=fluent-bit-binary-builder /usr/local/bin/fluent-bit /usr/local/bin/fluent-bit
COPY --from=fluent-bit-binary-builder /usr/local/etc/fluent-bit /usr/local/etc/fluent-bit
COPY --from=fluent-bit-binary-builder /usr/local/lib/fluent-bit /usr/local/etc/fluent-bit

# bash dependencies
COPY --from=builder /lib/libreadline.so.8 /lib/
COPY --from=builder /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /usr/lib/
# inotifywait dependencies
COPY --from=builder /lib/libinotifytools.so.0 /lib/
# crond dependencies
COPY --from=builder /lib/libselinux.so.1 /lib/libpam.so.0 /lib/libc.so.6 /lib/libpcre.so.1 /lib/libaudit.so.1 /lib/libcap-ng.so.0/ /lib/
# vim dependencies
COPY --from=builder /lib/libm.so.6 /lib/libtinfo.so.6 /lib/
# metricsextension dependencies
# libssl.so.1.1 & libcrypto.so.1.1 are already available with openssl in distroless and copying them over causes FIPS HMAC verification failures
COPY --from=builder /lib/libboost_filesystem.so.1.76.0 /lib/libcpprest.so.2.10  /lib/libstdc++.so.6 /lib/libm.so.6 /lib/libgcc_s.so.1 /lib/libc.so.6 /lib/libbrotlidec.so.1 /lib/libbrotlienc.so.1 /lib/libz.so.1 /lib/libbrotlicommon.so.1 /lib/
COPY --from=builder /lib64/libuuid.so.1 /lib64
# fluent-bit dependencies
# libssl.so.1.1 & libcrypto.so.1.1 are already available with openssl in distroless and copying them over causes FIPS HMAC verification failures
COPY --from=fluent-bit-binary-builder /lib/libluajit-5.1.so.2 /lib/libyaml-0.so.2 /lib/libsystemd.so.0 /lib/libgcc_s.so.1 /lib/libc.so.6 /lib/liblzma.so.5 /lib/libzstd.so.1 /lib/liblz4.so.1 /lib/libcap.so.2 /lib/libgcrypt.so.20 /lib/libgpg-error.so.0 /lib/
# mdsd dependencies
COPY --from=builder /usr/lib/libdl.so.2 /usr/lib/librt.so.1 /usr/lib/libpthread.so.0 /usr/lib/libm.so.6 /usr/lib/libstdc++.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/
# logrotate dependencies
COPY --from=builder /lib/libselinux.so.1 /lib/libpopt.so.0 /lib/libpcre.so.1 /lib/
# curl dependencies
# libssl.so.1.1 & libcrypto.so.1.1 are already available with openssl in distroless and copying them over causes FIPS HMAC verification failures
COPY --from=builder /lib/libcurl.so.4 /lib/libz.so.1 /lib/libc.so.6 /lib/libnghttp2.so.14 /lib/libssh2.so.1   /lib/libgssapi_krb5.so.2 /lib/libzstd.so.1 /lib/
COPY --from=builder /usr/lib/libkrb5.so.3 /usr/lib/libk5crypto.so.3 /usr/lib/libcom_err.so.2 /usr/lib/libkrb5support.so.0 /usr/lib/libresolv.so.2 /usr/lib/
# sh dependencies
COPY --from=builder /lib/libreadline.so.8 /lib/libc.so.6 /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /lib/
# update-ca-trust dependencies
COPY --from=builder /usr/lib64/pkcs11 /usr/lib64
COPY --from=builder /usr/lib/pkcs11 /usr/lib/
COPY --from=builder /usr/libexec/p11-kit /usr/libexec
COPY --from=builder /lib/libp11-kit.so.0 /lib/libtasn1.so.6 /lib/libc.so.6 /lib/libffi.so.8 /lib/
COPY --from=builder /usr/lib/p11-kit-trust.so /usr/lib/p11-kit-proxy.so /usr/lib/libp11-kit.so.0.3.0 /usr/lib/libnssckbi.so /usr/lib/
COPY --from=builder /usr/lib/pkcs11/p11-kit-trust.so /usr/lib/pkcs11/

RUN [ "/bin/bash", "-c", "chmod 644 /etc/crontab" ]
RUN [ "/bin/bash", "-c", "chown root.root /etc/crontab" ]
RUN [ "/bin/bash", "-c", "chmod 755 /etc/cron.daily/logrotate" ]
RUN [ "/bin/bash", "-c", "chmod 644 /etc/logrotate.d/prometheus-collector" ]

# Run the Go executable, entrypoint
ENTRYPOINT ["./opt/main"]