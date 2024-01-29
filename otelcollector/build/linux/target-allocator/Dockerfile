FROM mcr.microsoft.com/cbl-mariner/base/core:2.0 as builder
LABEL description="Azure Monitor Prometheus metrics collector - configuration reader sidecar"
LABEL maintainer="ciprometheus@microsoft.com"
ENV OS_TYPE "linux"
ENV tmpdir /opt

COPY ./scripts/setup-targetallocator.sh $tmpdir/
COPY ./LICENSE $tmpdir/microsoft
COPY ./NOTICE $tmpdir/microsoft
# COPY ./build/linux/rpm-repos/ /etc/yum.repos.d/

ARG TARGETARCH
RUN tdnf clean all
RUN tdnf repolist --refresh
RUN tdnf update -y
RUN tdnf install -y wget sudo vim ruby-devel busybox curl
RUN mkdir /busybin && busybox --install /busybin
# RUN chmod 775 /etc/cron.daily/logrotate
RUN chmod 775 $tmpdir/setup-targetallocator.sh;
RUN sync;
RUN $tmpdir/setup-targetallocator.sh ${TARGETARCH}

# Build the otel-allocator binary
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 as ta-builder

WORKDIR /app
# Copy prometheus-operator repo files
###COPY ../../../otel-allocator/prometheus-operator/go.mod ../../../otel-allocator/prometheus-operator/go.sum ./prometheus-operator/
COPY ./otel-allocator/prometheus-operator/go.mod ./otel-allocator/prometheus-operator/go.sum ./prometheus-operator/
WORKDIR /app
###COPY ../../../otel-allocator/prometheus-operator/pkg/apis/monitoring/go.mod ../../../otel-allocator/prometheus-operator/pkg/apis/monitoring/go.sum ./prometheus-operator/pkg/apis/monitoring/
COPY ./otel-allocator/prometheus-operator/pkg/apis/monitoring/go.mod ./otel-allocator/prometheus-operator/pkg/apis/monitoring/go.sum ./prometheus-operator/pkg/apis/monitoring/
WORKDIR /app/prometheus-operator/pkg/apis/monitoring/
RUN go mod download


WORKDIR /app
# # # COPY ../../../otel-allocator/prometheus-operator/pkg/client/go.mod ../../../otel-allocator/prometheus-operator/pkg/client/go.sum ./prometheus-operator/pkg/client/
COPY ./otel-allocator/prometheus-operator/pkg/client/go.mod ./otel-allocator/prometheus-operator/pkg/client/go.sum ./prometheus-operator/pkg/client/
WORKDIR /app/prometheus-operator/pkg/client/
RUN go mod download

WORKDIR /app/prometheus-operator/
RUN go mod download

WORKDIR /app

# # # COPY ../../../otel-allocator/prometheus-operator /app/prometheus-operator
COPY ./otel-allocator/prometheus-operator /app/prometheus-operator


# Copy go mod and sum files
COPY ./otel-allocator/go.mod ./otel-allocator/go.sum ./
# # # COPY ../../../otel-allocator/go.mod ../../../otel-allocator/go.sum ./

RUN go mod download

# # # COPY ../../../otel-allocator .
COPY ./otel-allocator .


ARG TARGETOS TARGETARCH
# Build the Go app
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o targetallocator . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -a -installsuffix -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o targetallocator . ; fi


# # # FROM --platform=$BUILDPLATFORM mcr.microsoft.com/oss/go/microsoft/golang:1.21 as targetallocator-wrapper-builder
FROM --platform=linux/amd64 mcr.microsoft.com/oss/go/microsoft/golang:1.21 as targetallocator-wrapper-builder

WORKDIR /src
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
# COPY --from=ta-builder /app/targetallocator ./targetallocator-wrapper-builder/
COPY ./targetallocator-wrapper-builder/go.mod ./targetallocator-wrapper-builder/go.sum ./targetallocator-wrapper-builder/
RUN go version > goversion.txt
WORKDIR /src/targetallocator-wrapper-builder
RUN go mod download
COPY ./targetallocator-wrapper-builder /src/targetallocator-wrapper-builder
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main-targetallocator . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main-targetallocator . ; fi

# with sleep
COPY ./targetallocator-wrapper-builder/main-with-sleep /src/targetallocator-wrapper-builder/main-with-sleep
WORKDIR /src/targetallocator-wrapper-builder/main-with-sleep
RUN go mod download
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main-targetallocator-with-sleep . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o main-targetallocator-with-sleep . ; fi


# Build final exe
FROM mcr.microsoft.com/cbl-mariner/distroless/base:2.0
ENV PATH="/busybin:${PATH}"
ENV OS_TYPE "linux"

# files
COPY --from=builder /opt /opt
# COPY --from=builder /etc /etc
COPY --from=builder /busybin /busybin
# COPY --from=builder /var/lib/logrotate /var/lib/logrotate
# COPY --from=builder /var/spool/cron /var/spool/cron

# executables
COPY --from=builder /usr/bin/inotifywait /usr/bin/inotifywait
COPY --from=builder /usr/bin/bash /usr/bin/bash
COPY --from=builder /usr/sbin/busybox /usr/sbin/busybox
COPY --from=builder /usr/bin/vim /usr/bin/vim
COPY --from=builder /usr/share/vim /usr/share/vim
COPY --from=builder /usr/bin/curl /usr/bin/
COPY --from=builder /bin/sh /bin/sh

# bash dependencies
COPY --from=builder /lib/libreadline.so.8 /lib/
COPY --from=builder /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /usr/lib/
# inotifywait dependencies
COPY --from=builder /lib/libinotifytools.so.0 /lib/
# crond dependencies
# COPY --from=builder /lib/libselinux.so.1 /lib/libpam.so.0 /lib/libc.so.6 /lib/libpcre.so.1 /lib/libaudit.so.1 /lib/libcap-ng.so.0/ /lib/
# vim dependencies
COPY --from=builder /lib/libm.so.6 /lib/libtinfo.so.6 /lib/
# ruby dependencies
# COPY --from=builder /usr/lib/libruby.so.3.1 /usr/lib/libz.so.1 /usr/lib/libgmp.so.10 /usr/lib/libcrypt.so.1 /usr/lib/libm.so.6 /usr/lib/
# ruby re2 dependencies
# COPY --from=builder /usr/lib/libre2.so.0a /usr/lib/libstdc++.so.6 /usr/lib/libgcc_s.so.1 /usr/lib/libz.so.1 /usr/lib/libgmp.so.10 /usr/lib/libcrypt.so.1 /usr/lib/libm.so.6 /usr/lib/
# logrotate dependencies
# COPY --from=builder /lib/libselinux.so.1 /lib/libpopt.so.0 /lib/libpcre.so.1 /lib/
# curl dependencies
# COPY --from=builder /lib/libcurl.so.4 /lib/libz.so.1 /lib/libc.so.6 /lib/libnghttp2.so.14 /lib/libssh2.so.1 /lib/libssl.so.1.1   /lib/libcrypto.so.1.1  /lib/libgssapi_krb5.so.2 /lib/libzstd.so.1 /lib/
# COPY --from=builder /usr/lib/libkrb5.so.3 /usr/lib/libk5crypto.so.3 /usr/lib/libcom_err.so.2 /usr/lib/libkrb5support.so.0 /usr/lib/libresolv.so.2 /usr/lib/
# sh dependencies
COPY --from=builder /lib/libreadline.so.8 /lib/libc.so.6 /usr/lib/libncursesw.so.6 /usr/lib/libtinfo.so.6 /lib/

# RUN [ "/bin/bash", "-c", "chmod 644 /etc/crontab" ]
# RUN [ "/bin/bash", "-c", "chown root.root /etc/crontab" ]
# RUN [ "/bin/bash", "-c", "chmod 755 /etc/cron.daily/logrotate" ]


WORKDIR /opt/

# Copy the pre-built binary file from the previous stage
COPY --from=targetallocator-wrapper-builder /src/targetallocator-wrapper-builder/main-targetallocator .
COPY --from=targetallocator-wrapper-builder /src/targetallocator-wrapper-builder/main-with-sleep/main-targetallocator-with-sleep .
COPY --from=ta-builder /app/targetallocator .


# Expose the port on which the application listens
EXPOSE 8080

#ENTRYPOINT ["./main-targetallocator-with-sleep"]

ENTRYPOINT ["./main-targetallocator"]

# ENTRYPOINT [ "/bin/bash" ]
# CMD [ "/opt/main-configreader.sh" ]
