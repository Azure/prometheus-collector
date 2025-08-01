# Stage 1: prom-config-validator-builder
ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as prom-config-validator-builder
WORKDIR /src
RUN apt-get update && apt-get install -y gcc-aarch64-linux-gnu

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
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; \
    else \
        CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o promconfigvalidator . ; \
    fi

# Stage 2: configuration-reader-builder
ARG GOLANG_VERSION
FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as configuration-reader-builder
WORKDIR /src
RUN apt-get update && apt-get install -y gcc-aarch64-linux-gnu

COPY ./configuration-reader-builder/go.mod ./configuration-reader-builder/go.sum ./configuration-reader-builder/
COPY ./configuration-reader-builder/certcreator/go.mod ./configuration-reader-builder/certcreator/
COPY ./configuration-reader-builder/certgenerator/go.mod ./configuration-reader-builder/certgenerator/
COPY ./configuration-reader-builder/certoperator/go.mod ./configuration-reader-builder/certoperator/
RUN go version > goversion.txt

COPY ./shared /src/shared
WORKDIR /src/configuration-reader-builder
RUN go mod download
RUN go mod tidy
COPY ./configuration-reader-builder /src/configuration-reader-builder

ARG TARGETOS TARGETARCH
RUN echo "TARGETOS: $TARGETOS, TARGETARCH: $TARGETARCH"
RUN if [ "$TARGETARCH" = "arm64" ]; then \
        CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o configurationreader . ; \
    else \
        CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o configurationreader . ; \
    fi

# Stage 3: builder
FROM mcr.microsoft.com/azurelinux/base/core:3.0 as builder
LABEL description="Azure Monitor Prometheus metrics collector - configuration reader sidecar"
LABEL maintainer="ciprometheus@microsoft.com"

ENV OS_TYPE="linux" tmpdir="/opt"

COPY ./logrotate/logrotate /etc/cron.daily/logrotate
COPY ./logrotate/crontab /etc/crontab
RUN mkdir -p $tmpdir/microsoft/configmapparser/
COPY ./configmapparser/default-prom-configs/*.yml $tmpdir/microsoft/otelcollector/default-prom-configs/
COPY ./opentelemetry-collector-builder/collector-config-default.yml ./opentelemetry-collector-builder/collector-config-template.yml ./opentelemetry-collector-builder/PROMETHEUS_VERSION $tmpdir/microsoft/otelcollector/
COPY --from=configuration-reader-builder /src/goversion.txt $tmpdir/goversion.txt
COPY --from=prom-config-validator-builder /src/prom-config-validator-builder/promconfigvalidator $tmpdir/
COPY --from=configuration-reader-builder /src/configuration-reader-builder/configurationreader $tmpdir/
COPY ./scripts/*.sh $tmpdir/
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft
COPY ./build/linux/rpm-repos/ /etc/yum.repos.d/

ARG TARGETARCH

RUN tdnf clean all && tdnf repolist --refresh && tdnf update -y && tdnf install -y \
    wget sudo net-tools cronie vim logrotate procps-ng busybox diffutils curl && \
    mkdir /busybin && busybox --install /busybin && \
    chmod 775 /etc/cron.daily/logrotate && \
    chmod 775 $tmpdir/*.sh && \
    sync && \
    $tmpdir/setup-configreader.sh ${TARGETARCH}

# Stage 4: final
FROM mcr.microsoft.com/azurelinux/distroless/base:3.0

ENV PATH="/busybin:${PATH}" OS_TYPE="linux"

COPY --from=builder /opt /opt
COPY --from=builder /etc /etc
COPY --from=builder /busybin /busybin
COPY --from=builder /var/lib/logrotate /var/lib/logrotate
COPY --from=builder /var/spool/cron /var/spool/cron

COPY --from=builder /usr/bin/inotifywait /usr/bin/inotifywait
COPY --from=builder /usr/bin/bash /usr/bin/bash
COPY --from=builder /usr/sbin/busybox /usr/sbin/busybox
COPY --from=builder /usr/sbin/crond /usr/sbin/crond
COPY --from=builder /usr/bin/vim /usr/bin/vim
COPY --from=builder /usr/share/vim /usr/share/vim
COPY --from=builder /usr/sbin/logrotate /usr/sbin/logrotate
COPY --from=builder /usr/bin/gzip /usr/bin/
COPY --from=builder /usr/bin/curl /usr/bin/
COPY --from=builder /bin/sh /bin/sh
COPY --from=builder /opt/microsoft/configmapparser /opt/microsoft/configmapparser

# bash dependencies
COPY --from=builder /usr/lib/libreadline.so.8 /usr/lib/libc.so.6 /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /usr/lib/

# inotifywait dependencies
COPY --from=builder /usr/lib/libinotifytools.so.0 /usr/lib/libstdc++.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/libc.so.6 /usr/lib/libm.so.6 /usr/lib/

# crond dependencies
COPY --from=builder /usr/lib/libselinux.so.1 /usr/lib/libpam.so.0 /usr/lib/libc.so.6 /usr/lib/

# vim dependencies
COPY --from=builder /usr/lib/libm.so.6 /usr/lib/libtinfo.so.6 /usr/lib/libc.so.6 /usr/lib/

# logrotate dependencies
COPY --from=builder /usr/lib/libpopt.so.0 /usr/lib/libc.so.6 /usr/lib/

# curl dependencies
COPY --from=builder /usr/lib/libcurl.so.4 /usr/lib/libz.so.1 /usr/lib/libc.so.6 /usr/lib/libnghttp2.so.14 /usr/lib/libssh2.so.1 /usr/lib/libgssapi_krb5.so.2 /usr/lib/libzstd.so.1 /usr/lib/libkrb5.so.3 /usr/lib/libk5crypto.so.3 /usr/lib/libcom_err.so.2 /usr/lib/libkrb5support.so.0 /usr/lib/libresolv.so.2 /usr/lib/

# sh dependencies
COPY --from=builder /usr/lib/libreadline.so.8 /usr/lib/libc.so.6 /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /usr/lib/

COPY --from=configuration-reader-builder /src/configuration-reader-builder/configurationreader $tmpdir/
COPY --from=builder /opt/*.sh $tmpdir/

RUN [ "/bin/bash", "-c", "chmod 644 /etc/crontab" ]
RUN [ "/bin/bash", "-c", "chown root.root /etc/crontab" ]
RUN [ "/bin/bash", "-c", "chmod 755 /etc/cron.daily/logrotate" ]

ENTRYPOINT ["./opt/configurationreader"]
