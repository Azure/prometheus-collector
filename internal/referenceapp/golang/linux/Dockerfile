
ARG GOLANG_VERSION=1.26.9
FROM --platform=${BUILDPLATFORM} mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as builder

ARG TARGETARCH
# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=${TARGETARCH} \
    GOEXPERIMENT=ms_nocgo_opensslcrypto

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
#RUN tdnf install -y golang-${GOLANG_VERSION} ca-certificates
RUN case "$TARGETARCH" in amd64|arm64) ;; *) echo "Unsupported TARGETARCH: $TARGETARCH" >&2; exit 1 ;; esac \
    && go mod download

# COPY client-cert.pem /etc/prometheus/certs/
# COPY client-key.pem /etc/prometheus/certs/

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o main .

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/main .

FROM mcr.microsoft.com/azurelinux/distroless/base:3.0

# Copy the binary from the builder stage
COPY --from=builder /dist/main /dist/main

# Export necessary ports
EXPOSE 2112 2113 2115

# Command to run when starting the container
CMD ["/dist/main"]
