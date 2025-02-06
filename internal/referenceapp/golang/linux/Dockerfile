
FROM mcr.microsoft.com/oss/go/microsoft/golang:${GOLANG_VERSION} as builder

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
#RUN tdnf install -y golang-${GOLANG_VERSION} ca-certificates
RUN go mod download

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

FROM mcr.microsoft.com/cbl-mariner/distroless/base:2.0

# Copy the binary from the builder stage
COPY --from=builder /dist/main /dist/main

# Export necessary ports
EXPOSE 2112 2113

# Command to run when starting the container
CMD ["/dist/main"]
