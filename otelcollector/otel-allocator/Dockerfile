# Build the otel-allocator binary
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 as builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH
# Build the Go app
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; fi

######## Start a new stage from scratch #######
FROM mcr.microsoft.com/cbl-mariner/distroless/debug:2.0
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/main .

ENTRYPOINT ["./main"]