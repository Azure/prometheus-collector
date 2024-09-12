# Create directories
New-Item -Path "./shared/configmap/mp/" -ItemType Directory -Force
New-Item -Path "./shared/configmap/ccp/" -ItemType Directory -Force
# New-Item -Path "./main/" -ItemType Directory -Force

# Copy shared Go files
Copy-Item -Path "../shared/*.go" -Destination "./shared/"
Copy-Item -Path "../shared/go.mod" -Destination "./shared/"
Copy-Item -Path "../shared/go.sum" -Destination "./shared/"
Copy-Item -Path "../shared/configmap/mp/*.go" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/mp/go.mod" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/mp/go.sum" -Destination "./shared/configmap/mp/"
Copy-Item -Path "../shared/configmap/ccp/*.go" -Destination "./shared/configmap/ccp/"
Copy-Item -Path "../shared/configmap/ccp/go.mod" -Destination "./shared/configmap/ccp/"
Copy-Item -Path "../shared/configmap/ccp/go.sum" -Destination "./shared/configmap/ccp/"


go version
go mod tidy
go build -buildmode=pie -o "main.exe" "./main.go"
