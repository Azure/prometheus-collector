# Fix Critical Vulnerabilities

## Description
Identify and fix critical/high severity vulnerabilities using Trivy (the repo's configured scanning tool) and Dependabot.

USE FOR: fix critical vulnerability, fix high vulnerability, CVE fix, trivy fix, security vulnerability remediation, patch CVE, fix security scan failure
DO NOT USE FOR: general dependency updates without security motivation, adding new security scanning tools, security architecture review (use security-review skill), low/medium severity unless explicitly requested

## Instructions

### When to Apply
When Trivy CI scans fail, when Dependabot raises security PRs, or when manually remediating known CVEs.

### Step-by-Step Procedure

#### 1. Vulnerability Discovery
This repo uses **Trivy** for vulnerability scanning, configured in:
- `.github/workflows/scan.yml` — manual image scanning
- `.github/workflows/scan-released-image.yml` — scheduled released image scanning
- `.github/workflows/otelcollector-upgrade.yml` — scanning during OTel upgrades

Run Trivy locally:
```bash
trivy fs --severity CRITICAL,HIGH --scanners vuln otelcollector/opentelemetry-collector-builder/
trivy fs --severity CRITICAL,HIGH --scanners vuln otelcollector/fluent-bit/src/
```

#### 2. Vulnerability Triage
- Check `.trivyignore` for already-accepted CVEs with justification
- Classify: direct dependency vs transitive vs OS/base image
- Priority: Direct dependencies (fixable) > transitive (bump parent) > base image (update FROM)

#### 3. Fix Implementation

**Go module vulnerabilities:**
- Update the package: `go get <package>@<fixed-version>` in the affected module directory
- Run `go mod tidy`
- Check all 23+ `go.mod` files for the same dependency
- Verify with `go mod graph` that the old version is gone

**Container base image vulnerabilities:**
- Update `FROM` line in `otelcollector/build/linux/Dockerfile` or `otelcollector/build/windows/Dockerfile`
- Pin to specific version/digest

**npm vulnerabilities:**
- Run `npm audit fix` in `tools/az-prom-rules-converter/`
- For remaining issues, manually update `package.json` and run `npm install`

**Unfixable vulnerabilities:**
- Add to `.trivyignore` with justification comment and date:
  ```
  # No fix available upstream as of YYYY-MM-DD
  CVE-YYYY-NNNNN
  ```

#### 4. Build and Test
- Build: `cd otelcollector/opentelemetry-collector-builder && make all`
- Test: `go test ./...` in affected module directories
- Re-scan: Run Trivy again to verify CVEs are resolved
- Verify no new CRITICAL/HIGH vulnerabilities introduced

#### 5. Commit
- Single CVE: `fix: patch CVE-YYYY-NNNNN in <package>`
- Multiple CVEs: `fix: remediate critical/high vulnerabilities in <component>`

### Files Typically Involved
- `otelcollector/opentelemetry-collector-builder/go.mod` and `go.sum`
- `otelcollector/fluent-bit/src/go.mod` and `go.sum`
- `otelcollector/prom-config-validator-builder/go.mod` and `go.sum`
- `internal/referenceapp/golang/go.mod` and `go.sum`
- `otelcollector/build/linux/Dockerfile`
- `.trivyignore`
- `tools/az-prom-rules-converter/package.json`

### Validation
- Build succeeds for all affected components
- All existing tests pass
- Re-scan shows targeted CVEs resolved
- No new critical/high vulnerabilities introduced
- `.trivyignore` entries have proper justification

## Examples from This Repo
- `7022a7c` — Upgrade ksm for CVE fixes (#1355)
- `5f6c00a` — Add reference app to dependabot for OSS vulnerability remediation (#1201)
- `.trivyignore` — Contains accepted CVEs with comments
