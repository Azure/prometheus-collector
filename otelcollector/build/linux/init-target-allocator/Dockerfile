# Build the otel-allocator binary
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 as init-ta-builder

WORKDIR /src
RUN apt-get update && apt-get install gcc-aarch64-linux-gnu -y
COPY ./init-targetallocator-builder/go.mod ./init-targetallocator-builder/go.sum ./init-targetallocator-builder/
RUN go version > goversion.txt
WORKDIR /src/init-targetallocator-builder
RUN go mod download
COPY ./init-targetallocator-builder /src/init-targetallocator-builder
ARG TARGETOS TARGETARCH
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o initTa . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now' -o initTa . ; fi

ENTRYPOINT ["./initTa"]


