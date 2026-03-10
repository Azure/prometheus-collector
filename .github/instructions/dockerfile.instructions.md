---
applyTo: "**/Dockerfile,**/Dockerfile.*,**/*.dockerfile"
description: "Dockerfile conventions for prometheus-collector container builds."
---

# Dockerfile Conventions

- Use multi-stage builds to minimize final image size.
- Build Go binaries with hardened flags: `-buildmode=pie -ldflags '-linkmode external -extldflags=-Wl,-z,now'`.
- Use Microsoft base images: `mcr.microsoft.com/oss/go/microsoft/golang` for build stages.
- Support multi-architecture builds via `TARGETARCH` and `TARGETOS` build arguments.
- Pin base image versions — do not use `latest` tags.
- Use `mcr.microsoft.com/cbl-mariner/distroless/base` for minimal runtime images where possible.
- Copy only required binaries and assets into the final stage.
- Expose explicit ports for Prometheus metrics endpoints (e.g., `EXPOSE 2112`).
