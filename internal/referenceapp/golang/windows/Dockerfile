FROM mcr.microsoft.com/oss/go/microsoft/golang:1.21.9-1-nanoserver-ltsc2022-amd64

# Set necessary environmet variables needed for our image
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=windows \
    GOARCH=amd64

# Move to working directory /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod .
COPY go.sum .
#RUN go mod download

# Copy the code into the container
COPY . .

# Build the application
RUN go build -o /dist/main.exe .

# Move to /dist directory as the place for resulting binary folder
#WORKDIR /dist

# Copy binary from build to main folder
#RUN copy "/build/main.exe" "/dist/main.exe"

# Export necessary ports
EXPOSE 2112 2113

# Command to run when starting the container
CMD ["/dist/main.exe"]
