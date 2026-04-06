# Skill: Validate a New Extension Version

## Purpose

Validate that a new Azure Monitor Metrics extension version (Helm chart + container images) is correct and safe to release. This covers comparing deployed manifests between the old and new versions, verifying version consistency, and checking for unintended changes in RBAC, resources, tolerations, and configuration.

---

## When to Use

- A new build has been produced (new `HELM_SEMVER` / `IMAGE_TAG`) and you need to verify it before promoting to staging or stable release trains.
- Comparing a backdoor-deployed manifest from a test cluster against the previously released manifest.
- Reviewing a PR that modifies Helm chart templates, values, or RBAC rules.

---

## Prerequisites

| Item | Location |
|------|----------|
| Base version file | `otelcollector/VERSION` (e.g. `6.25.0`) |
| Chart templates | `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/Chart-template.yaml` and `values-template.yaml` |
| Deployed manifest snapshots | `otelcollector/deploy/addon-chart/ama-metrics-deployed-latest.yaml` and `ama-metrics-deployed-old.yaml` |
| Packaged chart (if available) | `otelcollector/deploy/addon-chart/ama-metrics-{HELM_SEMVER}/` |
| Build pipeline | `.pipelines/azure-pipeline-build.yml` |
| Release pipeline | `.pipelines/azure-pipeline-release.yml` |

---

## Version Naming Convention

```
VERSION file:       6.25.0
IMAGE_TAG:          6.25.0-main-03-05-2026-701eb75f
HELM_SEMVER:        6.25.0-ext-main-03-05-2026-701eb75f   (note the "-ext-" infix)
Windows image tag:  6.25.0-main-03-05-2026-701eb75f-win
TA image tag:       6.25.0-main-03-05-2026-701eb75f-targetallocator
CfgReader tag:      6.25.0-main-03-05-2026-701eb75f-cfg
```

Pattern: `{VERSION}-{BRANCH}-{MM-DD-YYYY}-{SHORT_SHA}` for images; chart version inserts `-ext-` after VERSION.

---

## Validation Checklist

### 1. Version Consistency

- [ ] `otelcollector/VERSION` matches the major.minor.patch in all image tags and the chart version.
- [ ] `HELM_SEMVER` in the build pipeline (`.pipelines/azure-pipeline-build.yml`, look for `Aks_Arc_Helm_Chart` job) matches the packaged chart directory name.
- [ ] `IMAGE_TAG` in the build pipeline matches the container image tags in the deployed manifest (check `image:` lines in DaemonSets, Deployments).
- [ ] `appVersion` in `Chart.yaml` (generated) matches `IMAGE_TAG`.
- [ ] `version` in `Chart.yaml` (generated) matches `HELM_SEMVER`.
- [ ] Windows image tag = `{IMAGE_TAG}-win`, TA tag = `{IMAGE_TAG}-targetallocator`, CfgReader tag = `{IMAGE_TAG}-cfg`.

### 2. Deployed Manifest Comparison

Compare `ama-metrics-deployed-latest.yaml` against `ama-metrics-deployed-old.yaml`:

#### 2a. Structural Inventory (should match unless intentional)

Count each resource type in both files — they should be identical unless a resource was added/removed:

```bash
# From otelcollector/deploy/addon-chart/
grep "^kind:" ama-metrics-deployed-old.yaml | sort | uniq -c
grep "^kind:" ama-metrics-deployed-latest.yaml | sort | uniq -c
```

Expected resources (as of 6.26.0):
- 2 ClusterRoles, 2 ClusterRoleBindings, 2 CustomResourceDefinitions
- 2 DaemonSets (Linux + Windows), 3 Deployments (RS, KSM, operator-targets)
- 1 HorizontalPodAutoscaler, 1 PodDisruptionBudget
- 2 Services, 2 ServiceAccounts

#### 2b. Image Version Bumps

```bash
grep -oP 'image:\s*\K.*' ama-metrics-deployed-old.yaml | sort -u
grep -oP 'image:\s*\K.*' ama-metrics-deployed-latest.yaml | sort -u
```

Verify:
- [ ] All `ama-metrics` images point to the new `IMAGE_TAG`.
- [ ] KSM image version bump is intentional (check release notes / PR).
- [ ] `addon-token-adapter` image is at the expected version.
- [ ] Windows images use the `-win` suffix.

#### 2c. RBAC Changes

```bash
# Extract rules from ClusterRoles
grep -A 50 "kind: ClusterRole" ama-metrics-deployed-old.yaml | grep -E "resources:|verbs:|apiGroups:|resourceNames:"
grep -A 50 "kind: ClusterRole" ama-metrics-deployed-latest.yaml | grep -E "resources:|verbs:|apiGroups:|resourceNames:"
```

Watch for:
- [ ] No unintended addition/removal of API resources (e.g. `nodes/proxy` removal).
- [ ] `resourceNames` restrictions are preserved on `secrets` rules.
- [ ] `verbs` haven't been broadened beyond what's needed.

#### 2d. Tolerations and Scheduling

```bash
grep -B2 -A5 "tolerations:" ama-metrics-deployed-old.yaml
grep -B2 -A5 "tolerations:" ama-metrics-deployed-latest.yaml
```

Check:
- [ ] New tolerations (e.g. `PreferNoSchedule`) are intentional.
- [ ] `nodeSelector` is present where required (e.g. `kubernetes.io/os: linux` on KSM).
- [ ] No unintended affinity/anti-affinity rule changes.

#### 2e. Environment Variables and Volumes

```bash
# Compare env vars
grep -E "name:|value:" ama-metrics-deployed-old.yaml | head -100
grep -E "name:|value:" ama-metrics-deployed-latest.yaml | head -100
```

Specifically check for:
- [ ] Addition or removal of env vars (e.g. `ENDPOINT_FQDN` removal).
- [ ] Projected service account token volumes (audience, expirationSeconds).
- [ ] ConfigMap and Secret volume mounts.

#### 2f. Resource Requests and Limits

```bash
grep -B1 -A4 "resources:" ama-metrics-deployed-old.yaml
grep -B1 -A4 "resources:" ama-metrics-deployed-latest.yaml
```

- [ ] CPU/memory requests and limits haven't changed unexpectedly.

#### 2g. Normalized Diff (strip formatting noise)

To ignore quoting, whitespace, comment, and Helm label differences:

```bash
# Normalize both files for meaningful comparison
normalize() {
    sed "s/['\"]//g; s/#.*//; /^$/d; /helm.sh/d; /app.kubernetes.io/d" "$1" \
    | sed 's/[[:space:]]*$//' \
    | sort
}
normalize ama-metrics-deployed-old.yaml > /tmp/old_norm.txt
normalize ama-metrics-deployed-latest.yaml > /tmp/new_norm.txt
diff /tmp/old_norm.txt /tmp/new_norm.txt
```

### 3. Helm Chart Template Validation

```bash
cd otelcollector/deploy/addon-chart/azure-monitor-metrics-addon
helm lint .
helm template ama-metrics . --values values.yaml > /tmp/rendered.yaml
```

- [ ] `helm lint` passes with no errors.
- [ ] `helm template` renders without errors.
- [ ] Rendered output matches expectations for the target environment.

### 4. Values File Cross-Check

If a release values file exists (e.g. `values-feb-2026-release.yaml`):

- [ ] `ImageTag`, `ImageTagWin`, `ImageTagTargetAllocator`, `ImageTagCfgReader` all share the same build (same commit SHA, date, branch).
- [ ] `AddonTokenAdapter.ImageTag` is at the latest from AKS-RP.
- [ ] `KubeStateMetrics.ImageTag` matches what's in the deployed manifest.
- [ ] `Region` and `AzureResourceID` are correct for the target test cluster.
- [ ] `CloudEnvironment` is correct (`azurepubliccloud`, `azuredeloscloud`, etc.).

### 5. Arc Extension Specific Checks

For Arc extension releases:

- [ ] Chart is available at the expected MCR endpoint: `mcr.microsoft.com/azuremonitor/containerinsights/ciprod/ama-metrics-arc`.
- [ ] `CHART_VERSION` in `arcExtensionRelease.sh` parameters file maps to the correct build.
- [ ] Release train is correct for the stage (`pipeline` → `staging` → `stable`).
- [ ] SDP rollout configuration has correct bake times and region batches.

---

## Common Issues

| Issue | Symptom | Resolution |
|-------|---------|------------|
| Version mismatch between chart and images | Pods pull wrong image or `helm upgrade` fails | Ensure `HELM_SEMVER` and `IMAGE_TAG` derive from the same build. Check that `envsubst` ran correctly. |
| RBAC permission errors after upgrade | `ama-metrics-serviceaccount cannot list resource "X"` | Compare ClusterRole rules between old and new manifests. Verify any removed permissions are intentional. |
| Missing nodeSelector on KSM | KSM pods scheduled on Windows nodes, crash | Ensure `nodeSelector: kubernetes.io/os: linux` is present in the KSM Deployment. |
| Projected token volume removed | Authentication failures for addon-token-adapter | Check for `projected` volumes with `audience` and `expirationSeconds` in ServiceAccount token mounts. |
| Hardcoded versions in build pipeline | Build produces stale versions | `Aks_Arc_Helm_Chart` job in `azure-pipeline-build.yml` has hardcoded `HELM_SEMVER` and `IMAGE_TAG` — verify they match the current build. |

---

## Quick Comparison Commands

```powershell
# PowerShell one-liner: side-by-side resource count
$old = Get-Content ama-metrics-deployed-old.yaml | Select-String "^kind:" | Group-Object Line | Sort-Object Name
$new = Get-Content ama-metrics-deployed-latest.yaml | Select-String "^kind:" | Group-Object Line | Sort-Object Name
$old | Format-Table Count, Name -AutoSize
$new | Format-Table Count, Name -AutoSize

# Extract all image references
Select-String -Path ama-metrics-deployed-latest.yaml -Pattern "image:" | ForEach-Object { $_.Line.Trim() } | Sort-Object -Unique

# Diff RBAC rules only
Select-String -Path ama-metrics-deployed-old.yaml -Pattern "resources:|verbs:|apiGroups:|resourceNames:" | ForEach-Object { $_.Line.Trim() }
Select-String -Path ama-metrics-deployed-latest.yaml -Pattern "resources:|verbs:|apiGroups:|resourceNames:" | ForEach-Object { $_.Line.Trim() }
```

---

## Related Files

- [otelcollector/VERSION](../../../otelcollector/VERSION) — base semver
- [Chart-template.yaml](../../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/Chart-template.yaml) — chart template
- [values-template.yaml](../../../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml) — values template
- [azure-pipeline-build.yml](../../../.pipelines/azure-pipeline-build.yml) — build pipeline
- [azure-pipeline-release.yml](../../../.pipelines/azure-pipeline-release.yml) — release pipeline
- [arcExtensionRelease.sh](../../../.pipelines/deployment/arc-extension-release/ServiceGroupRoot/Scripts/arcExtensionRelease.sh) — Arc SDP rollout script
- [internal/docs/ARC.md](../../../internal/docs/ARC.md) — Arc extension lifecycle docs
