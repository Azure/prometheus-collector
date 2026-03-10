# Infrastructure

## Description
Guide for modifying Helm charts, Kubernetes manifests, Dockerfiles, and IaC templates.

USE FOR: update helm chart, change deployment, modify k8s manifest, update Dockerfile, ARM template change, Bicep change, Terraform change
DO NOT USE FOR: application code changes, CI/CD pipeline changes, dependency updates

## Instructions

### When to Apply
When modifying deployment configurations, Helm chart values/templates, Dockerfiles, or IaC templates.

### Step-by-Step Procedure
1. Identify the infrastructure layer:
   - **Helm charts**: `otelcollector/deploy/` — addon-chart, aks-chart, arc-chart variants
   - **Dockerfiles**: `otelcollector/build/linux/Dockerfile`, `otelcollector/build/windows/Dockerfile`
   - **ARM templates**: `AddonArmTemplate/`, `ArcArmTemplate/`
   - **Bicep templates**: `AddonBicepTemplate/`, `ArcBicepTemplate/`
   - **Terraform**: `AddonTerraformTemplate/`

2. For Helm chart changes:
   - Update values in all chart variants (addon, AKS, Arc) for consistency
   - Validate templates: `helm template <chart-path>`
   - Check RBAC changes against least-privilege principle
   - Verify resource limits are set

3. For Dockerfile changes:
   - Maintain multi-arch support (amd64/arm64)
   - Use pinned base image versions (not `latest`)
   - Keep multi-stage build structure
   - Run non-root where possible

4. For IaC changes:
   - ARM/Bicep: Follow existing parameter/variable patterns
   - Terraform: Update both `variables.tf` and `main.tf`
   - Test template rendering

### Files Typically Involved
- `otelcollector/deploy/*/templates/*.yaml`
- `otelcollector/deploy/*/values.yaml`
- `otelcollector/build/linux/Dockerfile`
- `otelcollector/build/windows/Dockerfile`
- `AddonArmTemplate/`, `ArcArmTemplate/`, `AddonBicepTemplate/`, `ArcBicepTemplate/`, `AddonTerraformTemplate/`

### Validation
- `helm template` renders without errors
- Docker build succeeds for both architectures
- No security regressions (non-root, resource limits, RBAC)
- All chart variants updated consistently

## Examples from This Repo
- Helm chart changes span multiple variants in `otelcollector/deploy/`
- Dockerfile changes at `otelcollector/build/linux/Dockerfile` use multi-stage, multi-arch builds
