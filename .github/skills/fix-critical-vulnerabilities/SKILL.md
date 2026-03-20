# Fix Critical Vulnerabilities

## Description
Identify and fix critical/high severity vulnerabilities using the repo's own scanning tools.

USE FOR: fix critical vulnerability, fix high vulnerability, CVE fix, trivy fix, security vulnerability remediation, patch CVE, fix security scan failure, dependency vulnerability fix
DO NOT USE FOR: general dependency updates without security motivation, adding new scanning tools, security architecture review (use security-review skill), low/medium severity unless explicitly requested

## Instructions

### When to Apply
When Trivy scans report CRITICAL or HIGH vulnerabilities, when Dependabot security PRs need manual intervention, or when CI security scans fail.

### Step-by-Step Procedure

#### 1. Vulnerability Discovery
This repo uses:
- **Trivy**: Container image scanning (`.github/workflows/scan.yml`, `.github/workflows/scan-released-image.yml`)
- **Dependabot**: Go module and GitHub Actions dependency updates (`.github/dependabot.yml`)

Run local scans:
```bash
# Scan Go dependencies
trivy fs --severity CRITICAL,HIGH --scanners vuln otelcollector/opentelemetry-collector-builder/
trivy fs --severity CRITICAL,HIGH --scanners vuln otelcollector/fluent-bit/src/

# Scan container images (if built locally)
trivy image --severity CRITICAL,HIGH <image-name>
```

#### 2. Vulnerability Triage
a. **Direct dependencies**: Package in `go.mod` `require` (not `indirect`) — HIGH priority, directly fixable
b. **Transitive dependencies**: `// indirect` entries — may require bumping parent dependency
c. **Base image vulnerabilities**: Check Dockerfile `FROM` lines for newer MCR images
d. **Already-ignored**: Check `.trivyignore` — if CVE is listed with justification, skip it

#### 3. Fix Implementation

**Go module vulnerabilities:**
- Update: `go get <package>@<fixed-version>` in each affected module
- This repo has 24 `go.mod` files — check ALL affected modules
- Run `go mod tidy` in each affected module
- Verify: `go mod graph | grep <vulnerable-package>` shows no old version

**Container base image vulnerabilities:**
- Check for newer MCR image tags in Dockerfiles at `otelcollector/build/linux/Dockerfile` and `otelcollector/build/windows/Dockerfile`
- Update `FROM` line with new version
- Rebuild and re-scan

**Unfixable vulnerabilities:**
- Add to `.trivyignore` with:
  - CVE ID
  - Date added
  - Reason (e.g., "No fix available upstream as of YYYY-MM-DD")

#### 4. Build and Test
```bash
# Build all components
cd otelcollector/opentelemetry-collector-builder && make all

# Run tests in affected modules
cd otelcollector/opentelemetry-collector-builder && go test ./...

# Re-scan to verify fix
trivy fs --severity CRITICAL,HIGH --scanners vuln otelcollector/opentelemetry-collector-builder/
```

#### 5. Commit and Document
- Single CVE: `fix: patch CVE-YYYY-NNNNN in <package>`
- Multiple CVEs: `fix: remediate critical/high vulnerabilities in <component>`
- Include table of CVEs fixed in PR description

### Files Typically Involved
- `otelcollector/*/go.mod`, `otelcollector/*/go.sum`
- `otelcollector/build/linux/Dockerfile`, `otelcollector/build/windows/Dockerfile`
- `.trivyignore`
- `.github/workflows/scan.yml`

### Validation
- Build succeeds for all affected components
- All existing tests pass
- Re-scan shows targeted CVEs resolved
- No new CRITICAL/HIGH vulnerabilities introduced
- `.trivyignore` entries (if any) have justification
