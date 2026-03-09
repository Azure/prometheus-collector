# Security Review

## Description
STRIDE-based security review skill for the prometheus-collector repository, covering container security, Kubernetes RBAC, dependency vulnerabilities, and credential management.

USE FOR: security review, threat model, STRIDE analysis, credential leak check, secret scan, vulnerability review, security audit, hardening review
DO NOT USE FOR: performance optimization, functional bug fixes, code style issues, feature implementation

## Instructions

### When to Apply
Apply to every PR that modifies authentication logic, network-facing code, Dockerfiles, Helm charts, Kubernetes manifests, or dependency files.

### Step-by-Step Procedure

#### 1. STRIDE Threat Model Checklist

**Spoofing (Identity)**
- Verify service-to-service calls use managed identity or mTLS (Azure SDK uses `azidentity`)
- Check that tokens are validated, not just checked for presence
- Verify IMDS token adapter authentication in `otelcollector/main/`

**Tampering (Data Integrity)**
- Input validation on Prometheus scrape configs (via prom-config-validator)
- Checksums/signatures verified for external data
- File permissions are restrictive in Dockerfiles

**Repudiation (Auditability)**
- Security-relevant actions are logged via `log.Println`/`log.Fatalf`
- Audit context includes operation details without leaking secrets

**Information Disclosure (Confidentiality)**
- No hardcoded secrets, API keys, tokens, or connection strings
- No secrets in log output or error messages
- Environment variables used for secrets (`TELEMETRY_APPLICATIONINSIGHTS_KEY`, etc.)
- `.trivyignore` entries have justification comments

**Denial of Service (Availability)**
- Container resource limits set in Kubernetes manifests
- Timeout configurations present for scrape targets
- No unbounded goroutines in collector components

**Elevation of Privilege (Authorization)**
- Containers run as non-root where possible
- ClusterRole permissions follow least-privilege (verify after `clusterrole` changes)
- Security contexts set in Kubernetes manifests

#### 2. Credential & Secret Leak Detection
- Scan changed files for hardcoded secret patterns (API keys, tokens, connection strings)
- Verify `.gitignore` excludes `*.pem`, `*.key`, `.env`
- Check that env vars reference names only, not values

#### 3. Weak Security Patterns (Go-specific)
- No `#nosec` annotations without justification comments
- Unchecked `err` returns from security-sensitive functions
- No `fmt.Sprintf` for building queries or commands with user input
- No `exec.Command` with unsanitized input

#### 4. Infrastructure Security (Docker/Kubernetes)
- No `latest` tags in container images
- Build flags include PIE and RELRO hardening
- No privileged containers or hostNetwork without justification
- Security contexts present (readOnlyRootFilesystem, runAsNonRoot)
- Exposed ports are intentional and documented

### Validation
- Verify `.trivyignore` entries have proper justification
- Confirm `SECURITY.md` exists and describes responsible disclosure
- Run Trivy scan: severity CRITICAL,HIGH against container images

## References
- `SECURITY.md` — Microsoft security vulnerability reporting policy
- `.github/workflows/scan.yml` — Trivy scanning workflow
- `.trivyignore` — Accepted CVE exceptions
