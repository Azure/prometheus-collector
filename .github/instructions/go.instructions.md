---
applyTo: "**/*.go"
description: Go code style, design patterns, and best practices for this repository.
---

# Go Code Conventions

1. **Naming**: Use PascalCase for exported identifiers, camelCase for unexported. Struct fields that are exported use PascalCase.
2. **Imports**: Group in three tiers separated by blank lines — stdlib, external packages (e.g., `github.com/`), internal/local modules (e.g., `prometheus-collector/`).
3. **Error handling**: Wrap errors with context using `fmt.Errorf("operation failed: %w", err)`. Return early on errors. Never silently ignore errors.
4. **Logging**: Use the standard `log` package (`log.Println`, `log.Fatalf`). For CCP (control-plane) mode, use `shared.SetupCCPLogging()` for JSON-structured logs.
5. **Configuration**: Read from environment variables using `shared.GetEnv("KEY", "default")` or `os.Getenv`. Never hardcode secrets or connection strings.
6. **Module boundaries**: Each component has its own `go.mod`. Use `replace` directives for local shared modules. Run `go mod tidy` after dependency changes.
7. **Build flags**: Production binaries use `-buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now'` for security hardening.
8. **Signal handling**: Long-running services must handle `SIGTERM` for graceful shutdown (see `otelcollector/main/main.go`).
9. **Kubernetes client**: Use `k8s.io/client-go` with in-cluster config. Follow existing patterns in `otelcollector/shared/` for K8s interactions.
10. **Testing**: Use Ginkgo v2 + Gomega for E2E tests. Test files are `*_test.go` with `suite_test.go` per package. Use labels for selective execution.
