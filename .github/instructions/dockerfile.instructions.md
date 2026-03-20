---
applyTo: "**/Dockerfile,**/Dockerfile.*"
description: Dockerfile conventions and best practices for this repository.
---

# Dockerfile Conventions

1. **Multi-stage builds**: All Dockerfiles use multi-stage builds. Each stage compiles a specific component (fluent-bit plugin, prometheus-ui, otelcollector, config-reader, etc.).
2. **Multi-arch**: Support both `amd64` and `arm64` via `$BUILDPLATFORM`, `$TARGETOS`, `$TARGETARCH` build args. Use cross-compilation with appropriate `CC` settings for ARM64.
3. **Base images**: Use Microsoft's official Go images (`mcr.microsoft.com/oss/go/microsoft/golang`) and CBL-Mariner for runtime.
4. **Security**: Build with PIE (`-buildmode=pie`) and hardening flags. Final images run as non-root where possible.
5. **Build args**: Use `ARG GOLANG_VERSION` and `ARG FLUENTBIT_GOLANG_VERSION` for version pinning — never use `latest` tags.
6. **Package manager**: Use `tdnf` (Mariner Linux) for runtime package installation, not `apt-get`.
