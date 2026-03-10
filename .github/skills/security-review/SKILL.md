# Security Review

## Description
STRIDE-based security review skill for the prometheus-collector repository.

USE FOR: security review, threat model, STRIDE analysis, credential leak check, secret scan, vulnerability review, security audit
DO NOT USE FOR: performance optimization, functional bug fixes, code style issues, feature implementation

## Instructions

### When to Apply
Apply to every PR that modifies authentication/authorization logic, network-facing code, data handling, infrastructure (Dockerfiles, Helm charts, K8s manifests), or dependency changes.

### Step-by-Step Procedure

#### 1. STRIDE Threat Model Checklist

**Spoofing (Identity)**
- Are K8s RBAC ClusterRoles and RoleBindings least-privilege?
- Are service account tokens properly scoped?
- Is Azure managed identity configured correctly?
- Are scrape targets authenticated where sensitive metrics are exposed?

**Tampering (Data Integrity)**
- Is ConfigMap input validated before use in the collector pipeline?
- Are Helm chart values sanitized (no template injection)?
- Are shell script variables properly quoted?
- Is TLS enabled for remote write to Azure Monitor?

**Repudiation (Auditability)**
- Are security-relevant actions logged via Application Insights?
- Do logs include sufficient context (component, operation, timestamp) without credentials?

**Information Disclosure (Confidentiality)**
- No hardcoded Application Insights keys, Azure credentials, or connection strings in code
- No secrets in log output, error messages, or Helm chart values
- Environment variables used for secrets (not config files committed to repo)
- `.trivyignore` entries have justification comments

**Denial of Service (Availability)**
- Resource limits set in K8s manifests (CPU, memory)
- Scrape intervals bounded (no unbounded collection)
- Goroutine lifecycle properly managed (context cancellation, signal handling)
- Container resource limits in Dockerfiles and Helm values

**Elevation of Privilege (Authorization)**
- Containers run as non-root (`USER` directive in Dockerfile)
- No privileged containers or hostNetwork without documented justification
- K8s SecurityContext set (readOnlyRootFilesystem where possible, drop capabilities)
- RBAC follows least-privilege principle

#### 2. Credential & Secret Leak Detection
Scan changed files for:
- Application Insights keys (base64-encoded strings)
- Azure connection strings (`AccountKey=`, `SharedAccessKey=`)
- K8s service account tokens
- Private keys (`-----BEGIN.*PRIVATE KEY-----`)
- Hardcoded IP addresses or hostnames that should be configurable

#### 3. Weak Security Patterns

**Go:**
- Unchecked `err` returns from security-sensitive functions
- `fmt.Sprintf` for building commands (injection risk)
- `exec.Command` with unsanitized input
- `#nosec` annotations without justification

**Shell/Bash:**
- Unquoted variables in commands
- `chmod 777` or overly permissive permissions
- Secrets passed as command-line arguments
- Missing `set -e` in security-critical scripts
- `curl | bash` patterns

**Infrastructure (Dockerfiles, K8s, Helm):**
- Running as root without justification
- Using `latest` tags (non-reproducible builds)
- Secrets in ENV instead of mounted secrets
- Privileged containers or hostNetwork
- Missing security contexts

#### 4. CI Security Integration
- Trivy: Container scanning configured in `.github/workflows/scan.yml`
- Dependabot: Configured in `.github/dependabot.yml` for 24 Go modules + GitHub Actions
- Secret scanning: GitHub native secret scanning active

### Validation
- Verify `.trivyignore` excludes are justified and have review dates
- Verify `.gitignore` excludes secret file patterns
- Confirm `SECURITY.md` exists (Microsoft MSRC reporting)
- Run `trivy fs --severity CRITICAL,HIGH .` for local scan
