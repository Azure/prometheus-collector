# CI/CD Pipeline

## Description
Guide for modifying GitHub Actions workflows and Azure Pipelines in this repository.

USE FOR: update pipeline, CI change, workflow modification, add CI check, fix build pipeline
DO NOT USE FOR: application code changes, infrastructure changes, dependency updates

## Instructions

### When to Apply
When modifying CI/CD workflows, adding new checks, updating pipeline configurations, or fixing build failures.

### Step-by-Step Procedure
1. Identify the pipeline system:
   - **GitHub Actions**: `.github/workflows/` — scan.yml, scan-released-image.yml, otelcollector-upgrade.yml, build-and-release-mixin.yml, build-and-push-dependent-helm-charts.yml, stale.yml, size.yml
   - **Azure Pipelines**: `.pipelines/` — deployment, release, regional testing

2. For GitHub Actions:
   - Use YAML syntax with proper indentation
   - Pin action versions (e.g., `actions/checkout@v4`) — Dependabot tracks these
   - Follow existing job naming conventions

3. For Azure Pipelines:
   - Follow existing template patterns in `.pipelines/`
   - Use variable groups for secrets
   - Maintain multi-stage deployment patterns

4. Test pipeline changes:
   - Verify YAML syntax locally
   - Check that referenced scripts exist
   - Ensure action versions are current

### Files Typically Involved
- `.github/workflows/*.yml`
- `.pipelines/**/*.yml`
- `.github/dependabot.yml` (for action version tracking)

### Validation
- Pipeline YAML is valid
- Referenced scripts and actions exist
- No secrets hardcoded in workflow files

## Examples from This Repo
- `.github/workflows/scan.yml` — Trivy container scanning
- `.github/workflows/otelcollector-upgrade.yml` — Automated OTel dependency updates
