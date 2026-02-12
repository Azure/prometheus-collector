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
COPY ./shared/go.mod ./shared/go.sum ./shared/
WORKDIR /src/prometheusreceiver
RUN go version
RUN go mod download
WORKDIR /src/shared
RUN go mod download
WORKDIR /src/prom-config-validator-builder
RUN go mod download
COPY ./prom-config-validator-builder /src/prom-config-validator-builder
COPY ./prometheusreceiver /src/prometheusreceiver
COPY ./shared /src/shared
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; fi

ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as ccpmain-builder
WORKDIR /
# Create directories
RUN mkdir -p ./shared/configmap/mp/
RUN mkdir -p ./shared/configmap/common/
RUN mkdir -p ./main/
# Copy shared module (includes mp, ccp, common, and helpers)
COPY ./shared/ ./main/shared/
# Copy main go files
COPY ./main/*.go ./main/
COPY ./go.mod ./main/
COPY ./go.sum ./main/
WORKDIR /main
RUN go version
RUN go mod download
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
RUN go build -o ccpmain ./main.go

FROM mcr.microsoft.com/azurelinux/base/core:3.0 as builder
ENV OS_TYPE "linux"
ENV tmpdir /opt
# Needed for ME, see https://github.com/microsoft/cpprestsdk/issues/1481
ENV MALLOC_ARENA_MAX=2
RUN mkdir -p $tmpdir/microsoft/configmapparser/
RUN mkdir -p $tmpdir/microsoft/liveness/
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/ccp-collector-config-default.yml ./opentelemetry-collector-builder/ccp-collector-config-template.yml ./opentelemetry-collector-builder/ccp-collector-config-replicaset.yml ./opentelemetry-collector-builder/PROMETHEUS_VERSION $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/opentelemetry-collector-builder/otelcollector $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/opentelemetry-collector-builder/otelcollector $tmpdir/microsoft/otelcollector/
COPY --from=otelcollector-builder /src/goversion.txt $tmpdir/goversion.txt
COPY --from=prom-config-validator-builder /src/prom-config-validator-builder/promconfigvalidator $tmpdir/

COPY ./scripts/*.sh $tmpdir/
COPY ./metricextension/me.config ./metricextension/me_internal.config ./metricextension/me_ds.config ./metricextension/me_ds_setdim.config ./metricextension/me_ds_internal.config ./metricextension/me_ds_internal_setdim.config /usr/sbin/
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft
COPY ./mdsd/envmdsd $tmpdir/
COPY ./build/linux/rpm-repos/ /etc/yum.repos.d/
COPY --from=ccpmain-builder --chmod=777 /main/ccpmain $tmpdir/main/

ARG TARGETARCH
RUN tdnf clean all
RUN tdnf repolist --refresh
RUN tdnf update -y
# RUN tdnf install -y wget sudo net-tools cronie vim ruby-devel logrotate procps-ng busybox diffutils curl
RUN tdnf install -y wget sudo net-tools cronie vim logrotate procps-ng busybox diffutils curl
RUN mkdir /busybin && busybox --install /busybin
RUN chmod 775 $tmpdir/*.sh;
RUN sync;
RUN $tmpdir/ccpsetup.sh ${TARGETARCH}

FROM mcr.microsoft.com/azurelinux/distroless/base:3.0
LABEL description="Azure Monitor Prometheus metrics collector"
LABEL maintainer="ciprometheus@microsoft.com"
# Needed for ME, see https://github.com/microsoft/cpprestsdk/issues/1481
ENV MALLOC_ARENA_MAX=2
ENV PATH="/busybin:${PATH}"
ENV OS_TYPE "linux"
ENV MDSD_LOG="/opt/microsoft/linuxmonagent"
ENV SKIP_IMDS_LOOKUP_FOR_LEGACY_AUTH="true"
ENV MDSD_FLUENT_SOCKET_PORT="28230"
ENV ENABLE_MCS="true"
ENV MONITORING_USE_GENEVA_CONFIG_SERVICE="false"
ENV MDSD_USE_LOCAL_PERSISTENCY="false"
ENV SSL_CERT_FILE="/etc/pki/tls/certs/ca-bundle.crt"

# files
COPY --from=builder /opt /opt
COPY --from=builder /etc /etc
COPY --from=builder /usr/sbin/me.config /usr/sbin/me_internal.config /usr/sbin/me_ds.config /usr/sbin/me_ds_setdim.config /usr/sbin/me_ds_internal.config /usr/sbin/me_ds_internal_setdim.config /usr/sbin/
COPY --from=builder /var/opt/microsoft /var/opt/microsoft

# executables
COPY --from=builder /usr/sbin/MetricsExtension /usr/sbin/MetricsExtension
# inotifywait dependencies
COPY --from=builder /usr/bin/inotifywait /usr/bin/inotifywait
COPY --from=builder /usr/lib/libinotifytools.so.0 /usr/lib/libstdc++.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/libc.so.6 /usr/lib/libm.so.6 /usr/lib/
# metricsextension dependencies
COPY --from=builder /usr/lib/libboost_filesystem.so.1.83.0 /usr/lib/libcpprest.so.2.10 /usr/lib/libuuid.so.1 /usr/lib/libstdc++.so.6 /usr/lib/libm.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/libc.so.6 /usr/lib/libboost_atomic.so.1.83.0 /usr/lib/libbrotlidec.so.1 /usr/lib/libbrotlienc.so.1 /usr/lib/libz.so.1 /usr/lib/libbrotlicommon.so.1 /usr/lib/
# mdsd dependencies
COPY --from=builder /usr/sbin/mdsd /usr/sbin/mdsd
COPY --from=builder /opt/microsoft/azure-mdsd/lib/libtcmalloc_minimal.so.4 /opt/microsoft/azure-mdsd/lib/
COPY --from=builder /usr/sbin/../lib/libpthread.so.0 /usr/sbin/../lib/libdl.so.2 /usr/sbin/../lib/libsymcrypt.so.103 /usr/sbin/../lib/librt.so.1 /usr/sbin/../lib/libm.so.6 /usr/sbin/../lib/libc.so.6 /usr/sbin/../lib/libstdc++.so.6 /usr/sbin/../lib/libgcc_s.so.1 /usr/sbin/../lib/

# debug image
# COPY --from=builder /busybin /busybin
# COPY --from=builder /var/lib/logrotate /var/lib/logrotate
# COPY --from=builder /var/spool/cron /var/spool/cron
# COPY --from=builder /usr/share/p11-kit /usr/share/p11-kit
# COPY --from=builder /usr/share/pki/ /usr/share/pki
# COPY --from=builder /usr/bin/bash /usr/bin/bash
# COPY --from=builder /usr/sbin/busybox /usr/sbin/busybox
# COPY --from=builder /usr/sbin/crond /usr/sbin/crond
# COPY --from=builder /usr/bin/vim /usr/bin/vim
# COPY --from=builder /usr/share/vim /usr/share/vim
# COPY --from=builder /usr/sbin/logrotate /usr/sbin/logrotate
# COPY --from=builder /usr/bin/gzip /usr/bin/
# COPY --from=builder /usr/bin/update-ca-trust /usr/bin
# COPY --from=builder /bin/sh /bin/sh
# COPY --from=builder /usr/bin/p11-kit /usr/bin
# COPY --from=builder /usr/bin/trust /usr/bin
# COPY --from=builder /lib/libreadline.so.8 /lib/
# COPY --from=builder /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /usr/lib/
# COPY --from=builder /lib/libselinux.so.1 /lib/libpam.so.0 /lib/libc.so.6 /lib/libpcre.so.1 /lib/libaudit.so.1 /lib/libcap-ng.so.0/ /lib/
# COPY --from=builder /lib/libm.so.6 /lib/libtinfo.so.6 /lib/
# COPY --from=builder /lib/libselinux.so.1 /lib/libpopt.so.0 /lib/libpcre.so.1 /lib/
# COPY --from=builder /lib/libreadline.so.8 /lib/libc.so.6 /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /lib/
# COPY --from=builder /usr/lib64/pkcs11 /usr/lib64
# COPY --from=builder /usr/lib/pkcs11 /usr/lib/
# COPY --from=builder /usr/libexec/p11-kit /usr/libexec
# COPY --from=builder /lib/libp11-kit.so.0 /lib/libtasn1.so.6 /lib/libc.so.6 /lib/libffi.so.8 /lib/
# COPY --from=builder /usr/lib/p11-kit-trust.so /usr/lib/p11-kit-proxy.so /usr/lib/libp11-kit.so.0.3.0 /usr/lib/libnssckbi.so /usr/lib/
# COPY --from=builder /usr/lib/pkcs11/p11-kit-trust.so /usr/lib/pkcs11/
# COPY --from=builder /usr/bin/curl /usr/bin/
# COPY --from=builder /lib/libcurl.so.4 /lib/libz.so.1 /lib/libc.so.6 /lib/libnghttp2.so.14 /lib/libssh2.so.1 /lib/libgssapi_krb5.so.2 /lib/libzstd.so.1 /lib/
# COPY --from=builder /usr/lib/libkrb5.so.3 /usr/lib/libk5crypto.so.3 /usr/lib/libcom_err.so.2 /usr/lib/libkrb5support.so.0 /usr/lib/libresolv.so.2 /usr/lib/
# RUN chmod 777 /opt/;

# Expose the port on which the application listens
EXPOSE 8080

# Run as root to access /etc
# USER root

# Run the Go executable, entrypoint
ENTRYPOINT ["./opt/main/ccpmain"]
