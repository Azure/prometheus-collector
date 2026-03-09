# CI/CD Pipeline

## Description
Guide for modifying GitHub Actions workflows and Azure Pipelines configurations in this repository.

USE FOR: modify CI, update pipeline, fix workflow, add CI step, update GitHub Actions, pipeline configuration
DO NOT USE FOR: application code changes, dependency updates, test code changes

## Instructions

### When to Apply
When modifying build, test, release, or scanning workflows in GitHub Actions or Azure Pipelines.

### Step-by-Step Procedure
1. **Identify the workflow**: GitHub Actions are in `.github/workflows/`, Azure Pipelines in `.pipelines/`.
2. **Key workflows**:
   - `otelcollector-upgrade.yml` — Automated OTel Collector version upgrades (daily schedule)
   - `build-and-release-mixin.yml` — Prometheus mixin builds and releases
   - `build-and-push-dependent-helm-charts.yml` — Helm chart publishing
   - `scan.yml` / `scan-released-image.yml` — Trivy security scanning
   - `stale.yml` — Stale issue/PR management
3. **Test locally** where possible (e.g., `act` for GitHub Actions).
4. **Version-pin actions**: Use specific versions, not `@master` or `@latest`.
5. **For Azure Pipelines**: The main build pipeline is `.pipelines/azure-pipeline-build.yml`.

### Files Typically Involved
- `.github/workflows/*.yml`
- `.pipelines/azure-pipeline-build.yml`
- `.pipelines/azure-pipeline-release.yml`

### Validation
- Workflow YAML is valid (use `actionlint` or similar)
- Action versions are pinned
- Secrets use GitHub secrets or Azure Pipeline variables, never hardcoded

## Examples from This Repo
- `b03f03f` — test: Testkube workflow migration (#1392)
- `690de4b` — Update azure-pipeline-release.yml for Azure Pipelines (#1314)
- `a348bc7` — release: separate pipeline for arc release (#1212)
