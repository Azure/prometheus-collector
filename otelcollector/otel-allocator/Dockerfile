# Build the otel-allocator binary
FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21 as builder

WORKDIR /app
# Copy prometheus-operator repo files
# COPY ./prometheus-operator/go.mod ./prometheus-operator/go.sum ./prometheus-operator/

# WORKDIR /app
# COPY ./prometheus-operator/pkg/apis/monitoring/go.mod ./prometheus-operator/pkg/apis/monitoring/go.sum ./prometheus-operator/pkg/apis/monitoring/
# WORKDIR /app/prometheus-operator/pkg/apis/monitoring/
# RUN go mod download


# WORKDIR /app
# COPY ./prometheus-operator/pkg/client/go.mod ./prometheus-operator/pkg/client/go.sum ./prometheus-operator/pkg/client/
# WORKDIR /app/prometheus-operator/pkg/client/
# RUN go mod download

# WORKDIR /app/prometheus-operator/
# RUN go mod download

# WORKDIR /app

# COPY ./prometheus-operator /app/prometheus-operator

# Copy go mod and sum files
COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH
# Build the Go app
RUN if [ "$TARGETARCH" = "arm64" ] ; then CC=aarch64-linux-gnu-gcc CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; else CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now -s -X github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring.GroupName=azmonitoring.coreos.com' -o main . ; fi

######## Start a new stage from scratch #######
# FROM mcr.microsoft.com/cbl-mariner/distroless/debug:2.0
FROM mcr.microsoft.com/cbl-mariner/distroless/base:2.0
WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/main .

ENTRYPOINT ["./main"]