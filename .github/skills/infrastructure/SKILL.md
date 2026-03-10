# Infrastructure

## Description
Guide for modifying Kubernetes manifests, Helm charts, Bicep templates, Terraform configurations, and Dockerfiles.

USE FOR: update Helm chart, modify Dockerfile, change Kubernetes manifest, update Bicep template, Terraform change, deployment configuration
DO NOT USE FOR: application logic changes, test code, CI/CD pipeline changes

## Instructions

### When to Apply
When modifying deployment artifacts, infrastructure-as-code, container builds, or Kubernetes resources.

### Step-by-Step Procedure

#### Helm Charts
1. Charts are in `otelcollector/deploy/chart/prometheus-collector/` (main) and `otelcollector/deploy/addon-chart/` (AKS add-on).
2. Templates use `-template.yaml` suffix — these are generated at build time.
3. Run `helm lint` and `helm template` to validate changes.
4. Dependent charts (node-exporter, kube-state-metrics) are in `otelcollector/deploy/dependentcharts/`.

#### Dockerfiles
1. Primary build: `otelcollector/build/linux/Dockerfile` (multi-stage, multi-arch).
2. Windows: `otelcollector/build/windows/Dockerfile`.
3. Maintain security-hardened build flags.
4. Pin base image versions — never use `latest`.

#### Bicep/Terraform/ARM
1. Bicep templates: `AddonBicepTemplate/`, `ArcBicepTemplate/`.
2. Terraform: `AddonTerraformTemplate/`.
3. ARM templates: `Azure-ARM-templates/`, `AddonArmTemplate/`, `ArcArmTemplate/`.
4. Keep deployment templates consistent across IaC flavors.

### Files Typically Involved
- `otelcollector/deploy/chart/prometheus-collector/templates/`
- `otelcollector/build/linux/Dockerfile`
- `AddonBicepTemplate/*.bicep`
- `AddonTerraformTemplate/*.tf`
- `Azure-ARM-templates/`

### Validation
- `helm lint` passes for modified charts
- Docker build succeeds for affected platforms
- Bicep/Terraform validates without errors

## Examples from This Repo
- `308d8df` — Added OTel gRPC ports support in extension chart (#1438)
- `fecaefd` — fix: bicep fixes (#1359)
- `cd5b720` — fix: helm lint+dry-run check for PRs + arc fixes (#1326)
