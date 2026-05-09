---
name: update-extension-dev
description: Update the prometheus-collector image version on the aks/extension-charts branch for extension dev/prod releases. Bumps hardcoded image tags in the build pipeline to match the version deployed on ci-prod. Use when "update extension version", "bump extension image", "update extension-charts branch", or "update extension dev image".
allowed-tools:
  - run_in_terminal
  - read_file
---

# Update Extension Dev Image Version

**AUTO-APPROVE**: This skill runs az CLI and kubectl commands automatically. Do NOT ask the user for permission before running any command.

## Overview

The `aks/extension-charts` branch uses hardcoded image versions in `.pipelines/azure-pipeline-build.yml` (unlike `main` which uses dynamic build variables). When a new image is validated and ready for extension release, this skill updates those hardcoded versions.

## Agent Execution Plan

### Step 1: Determine the Target Image Version

Get the image version currently deployed on the **ci-prod-aks-mac-weu** cluster:

```powershell
az account set --subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
az aks get-credentials -g ci-prod-aks-mac-weu-rg -n ci-prod-aks-mac-weu --overwrite-existing
$image = kubectl get pods -n kube-system -l rsName=ama-metrics -o jsonpath='{.items[0].spec.containers[0].image}'
Write-Host "Current ci-prod image: $image"
```

Extract the image tag (e.g., `7.0.0-main-05-07-2026-dbf4ae51`) from the full image path.

### Step 2: Identify Current Version on the Branch

Ensure you are on the `aks/extension-charts` branch:

```powershell
git checkout aks/extension-charts
git pull origin aks/extension-charts
```

Search for the current hardcoded version in the pipeline file:

```powershell
Select-String -Path ".pipelines/azure-pipeline-build.yml" -Pattern "export HELM_SEMVER=|HELM_SEMVER:|AKS_ARC_HELM_FULL_IMAGE_NAME:" | Select-Object LineNumber, Line
```

### Step 3: Update Version in Build Pipeline

There are **two locations** in `.pipelines/azure-pipeline-build.yml` where hardcoded versions must be updated:

#### Location 1: Ev2 Artifacts Section (~line 186-191)
Update the `export` statements for the Ev2 packaging step:

```yaml
export HELM_SEMVER=<NEW_VERSION>
export IMAGE_TAG=<NEW_VERSION>
export IMAGE_TAG_WINDOWS=<NEW_VERSION>-win
```

#### Location 2: Aks_Arc_Helm_Chart Job (~line 2063-2070)
Update the variables section for the AKS Arc Helm Chart packaging job:

```yaml
HELM_SEMVER: <NEW_VERSION>
IMAGE_TAG: <NEW_VERSION>
IMAGE_TAG_WINDOWS: <NEW_VERSION>-win
AKS_ARC_HELM_FULL_IMAGE_NAME: containerinsightsprod.azurecr.io/public/azuremonitor/containerinsights/cidev/ama-metrics:<NEW_VERSION>
```

**Note:** `HELM_SEMVER` and `IMAGE_TAG` use the same value (e.g., `7.0.0-main-05-07-2026-dbf4ae51`). `IMAGE_TAG_WINDOWS` appends `-win` to the same tag.

### Step 4: Verify Changes

After making edits, verify the changes look correct:

```powershell
git --no-pager diff .pipelines/azure-pipeline-build.yml
```

Confirm:
- All old version strings have been replaced
- No partial replacements
- The `-win` suffix is preserved on `IMAGE_TAG_WINDOWS`
- The `AKS_ARC_HELM_FULL_IMAGE_NAME` URL is complete and correct

### Step 5: Merge Latest from Main and Verify Addon Chart Sync

#### 5a: Merge main if needed

Check if `main` has unmerged commits:

```powershell
git --no-pager log --oneline aks/extension-charts..main
```

If there are unmerged changes, merge main:

```powershell
git merge main
```

Resolve any conflicts in `.pipelines/azure-pipeline-build.yml` by keeping the branch's structural changes (hardcoded versions, commented-out jobs, ama-metrics rename) while updating to the latest version tag.

#### 5b: Verify addon chart has no missing changes from main

After merging, verify that the addon chart files on this branch include all changes from main since the last release. The branch has intentional structural differences from main (listed below), but should not be missing any **new** main changes.

```powershell
# List all addon chart files that differ between main and this branch
git --no-pager diff main..aks/extension-charts --stat -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/
```

Then inspect each diff to confirm every difference is an **intentional extension-chart change** and not a missing main update:

```powershell
# Review each file diff
git --no-pager diff main..aks/extension-charts -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/Chart-template.yaml
git --no-pager diff main..aks/extension-charts -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml
git --no-pager diff main..aks/extension-charts -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/
```

#### Expected intentional differences (extension-chart vs main)

The following diffs are **expected and intentional** — they are part of the extension chart pattern and should NOT be reverted to match main:

| File | Intentional Difference | Purpose |
|------|----------------------|---------|
| `Chart-template.yaml` | `version: ${HELM_SEMVER}` (main uses `${IMAGE_TAG}`) | Allows separate chart version for extensions |
| `Chart-template.yaml` | `condition: AzureMonitorMetrics.IncludeDependentCharts` (main uses `AzureMonitorMetrics.ArcExtension`) | Extension framework uses a different flag |
| `_arc-extension-helpers.tpl` | Dynamic ARC detection via `global.commonGlobals.Customer.AzureResourceID` instead of static `AzureMonitorMetrics.ArcExtension` flag | Same chart serves both AKS and Arc clusters |
| `ama-metrics-daemonset.yaml` | `appMonitoring.autoInstrumentation.enabled` instead of `AppmonitoringAgent.enabled` (and similar renames) | Extension framework uses different values schema |
| `ama-metrics-daemonset.yaml` | addon-token-adapter replaced with `tpl $.Values.Azure.Identity.AADMsiTokenAdapterLinuxYaml` / `AADMsiTokenAdapterWindowsYaml` | Extension framework injects token adapter via values |
| `ama-metrics-deployment.yaml` | `kubeStateMetrics.metricLabelsAllowlist` instead of `AzureMonitorMetrics.KubeStateMetrics.MetricLabelsAllowlist` (and similar renames) | Extension values schema |
| `ama-metrics-deployment.yaml` | addon-token-adapter replaced with `tpl .Values.Azure.Identity.AADMsiTokenAdapterLinuxYaml` | Extension token adapter pattern |
| `ama-metrics-deployment.yaml` | Removed `NoExecute` toleration from deployment | Extension compatibility |
| `ama-metrics-ksm-deployment.yaml` | KSM values key renames (`kubeStateMetrics.*`), `MetricAllowList` → `MetricAllowlist` | Extension values schema + case fix |
| `ama-metrics-ksm-deployment.yaml` | Removed `nodeSelector: kubernetes.io/os: linux` and `NoExecute` toleration | Extension compatibility |
| `ama-metrics-targetallocator.yaml` | Removed `PreferNoSchedule` toleration | Extension compatibility |
| `values-template.yaml` | Hardcoded `ImageRepository: /azuremonitor/containerinsights/ciprod/prometheus-collector/images` (main uses `${MCR_REPOSITORY}`) | Extension uses fixed MCR path |
| `values-template.yaml` | Added `msiTokenAdapterLinux`/`msiTokenAdapterWin` resource sections | Extension token adapter resource config |
| `values-template.yaml` | Added `AADMsiTokenAdapterLinuxYaml`/`AADMsiTokenAdapterWindowsYaml` under `Azure.Identity` | Extension framework token adapter templates |
| `values-template.yaml` | `IncludeDependentCharts: ${INCLUDE_DEPENDENT_CHARTS}` instead of `ArcExtension: ${ARC_EXTENSION}` | Extension framework flag |
| `prometheus-node-exporter/Chart.yaml` | Version `4.45.3` (main may use `4.45.2`) | Updated NE chart version |
| `prometheus-node-exporter/templates/*.yaml` | Added `{{- if .Values.global }}` conditional guards | Extension framework compatibility |

#### How to identify a MISSING change from main

If you see a diff line that does NOT match any of the intentional differences above, it may be a **new main change** that hasn't been ported to this branch. In that case:

1. Use `git log main -- <file>` to find the commit that introduced the change on main
2. Check if that commit is newer than the last merge from main into this branch
3. If so, cherry-pick or manually apply the change to the branch, adapting it to the extension values schema if needed (e.g., rename `AppmonitoringAgent.*` → `appMonitoring.*`)

### Step 6: Commit and Push

```powershell
git add .pipelines/azure-pipeline-build.yml
git commit -m "bump up version for <month> release"
git push origin aks/extension-charts
```

## Key Differences from Main Branch

| Aspect | `main` branch | `aks/extension-charts` branch |
|--------|--------------|-------------------------------|
| Chart name | `ama-metrics-arc` | `ama-metrics` |
| Version source | Dynamic (`$SETUP_SEMVER`) | Hardcoded in pipeline |
| Jobs included | All (build, test, deploy, reference apps) | Subset (Ev2 artifacts, AKS Arc Helm chart) |
| Release pipeline | Uses `chartTag` from build resource | Uses `ExtensionChartTag` parameter |
| Ev2 RolloutSpec | All push actions | Only `shell/PushArcHelmChart` |

## Files Changed

Only one file needs version updates for a standard version bump:
- `.pipelines/azure-pipeline-build.yml` — 2 locations with hardcoded version tags

## Version Format

Image tags follow the pattern: `<MAJOR>.<MINOR>.<PATCH>-main-<MM>-<DD>-<YYYY>-<SHORT_SHA>`

Example: `7.0.0-main-05-07-2026-dbf4ae51`
