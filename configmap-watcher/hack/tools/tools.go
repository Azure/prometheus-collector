//go:build tools
// +build tools

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	_ "github.com/axw/gocov/gocov"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/goreleaser/goreleaser"
	_ "github.com/jstemmer/go-junit-report"
	_ "github.com/matm/gocov-html"
	_ "github.com/t-yuki/gocover-cobertura"
)
