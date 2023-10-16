# Remediator
REPO_ROOT=$(PWD)
IMG_NAME ?= configmap-watcher:latest

# Directories
TOOLS_DIR := $(PWD)/hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
BIN_DIR := bin

# Binaries
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
GORELEASER := $(TOOLS_BIN_DIR)/goreleaser
GO_JUNIT_REPORT := $(TOOLS_BIN_DIR)/go-junit-report
GOCOV := $(TOOLS_BIN_DIR)/gocov
GOCOV_HTML := $(TOOLS_BIN_DIR)/gocov-html
GOCOVER_COBERTURA := $(TOOLS_BIN_DIR)/gocover-cobertura

$(GOLANGCI_LINT): $(TOOLS_DIR)/go.mod # Build golangci-lint from tools folder.
	cd $(TOOLS_DIR); GOBIN=$(TOOLS_BIN_DIR) GO111MODULE=on GOPROXY="https://proxy.golang.org,direct" go install -mod=mod github.com/golangci/golangci-lint/cmd/golangci-lint

$(GORELEASER): $(TOOLS_DIR)/go.mod # Build goreleaser from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/goreleaser github.com/goreleaser/goreleaser

restore: $(GOLANGCI_LINT) $(GORELEASER) coverage-bin
	go mod download

coverage-bin: $(GO_JUNIT_REPORT) $(GOCOV) $(GOCOV_HTML) $(GOCOVER_COBERTURA)

# Packaging
package: $(GORELEASER)
	$(GORELEASER) --snapshot --skip-publish --rm-dist

build: lint-full
	go build -o $(REPO_ROOT)/bin/configmap-watcher main.go

test: coverage-bin lint
	mkdir -p coverage; \
	go test ./... -race -coverprofile cover.out -covermode atomic -v 2>&1 | $(GO_JUNIT_REPORT) --set-exit-code > report.xml; FAILED=$$?;\
	$(GOCOVER_COBERTURA) < cover.out > summary.xml;\
	$(GOCOV) convert cover.out > cover.json;\
   	cat cover.json | $(GOCOV_HTML) > coverage/index.html;\
	exit $$FAILED

# Linting
.PHONY: lint lint-full
lint: $(GOLANGCI_LINT) ## Lint codebase
	$(GOLANGCI_LINT) run -v --timeout 5m --config $(REPO_ROOT)/.golangci.yml

lint-full: $(GOLANGCI_LINT) ## Run slower linters to detect possible issues
	$(GOLANGCI_LINT) run -v --fast=false --config $(REPO_ROOT)/.golangci.yml

docker-build: lint
	gcr_endpoint=aksinfrapublic.azurecr.io envsubst < images/Dockerfile > Dockerfile.tmp
	docker build -f Dockerfile.tmp . -t ${IMG_NAME}

docker-push:
	docker push ${IMG_NAME}
