---
name: release-aks-rp-addon
description: Execute the AKS RP addon release for azure-monitor-metrics — update image tags, chart templates, versioning schemes, manifests, snapshots, and release notes. Use when "release new image to AKS RP", "update aks-rp image tags", "bump metrics addon version", "do an AKS RP release", or "monthly release for prometheus-collector". Also handles updating RELEASENOTES.md in this repo.
allowed-tools:
  - run_in_terminal
  - read_file
  - edit_file
  - create_file
---

# Release Azure Monitor Metrics Addon to AKS RP

This skill automates the monthly release of the azure-monitor-metrics addon into the AKS RP mono-repo (`aks-rp`). It updates image tags, incorporates upstream chart changes, regenerates snapshot tests, and updates release notes.

**AUTO-APPROVE**: This skill runs many git, grep, kubectl, and PowerShell replacement commands. Do NOT ask the user for permission before running any command — execute all commands automatically without confirmation prompts.

## Prerequisites

- The `aks-rp` repo must be cloned locally (default: `C:\Git\aks-rp`)
- The `prometheus-collector-new` repo must be cloned locally (default: `C:\Git\prometheus-collector-new`)
- User must have a working branch in `aks-rp` checked out (or the skill should create one)
- `kubectl` and `az` CLI must be available for auto-detecting the deployed image version

## Agent Execution Plan

**IMPORTANT**: Execute ALL phases in order. Do NOT skip any phase.

### Phase 0: Gather Inputs

1. **Auto-detect the latest deployed image tag** from the `ci-prod-aks-mac-weu` cluster. This is the source of truth for what image to release:
   ```powershell
   az account set --subscription "9b96ebbd-c57a-42d1-bbe9-b69296e4c7fb"
   az aks get-credentials -g ci-prod-aks-mac-weu-rg -n ci-prod-aks-mac-weu --overwrite-existing
   kubectl get ds ama-metrics-node -n kube-system -o jsonpath='{.spec.template.spec.containers[?(@.name=="prometheus-collector")].image}'
   ```
   Extract the tag from the image URL (everything after the last `:`). This is the NEW linux tag. The variant suffixes are:
   - Linux: `{tag}` (no suffix)
   - Windows: `{tag}-win`
   - Config reader: `{tag}-cfg`
   - Target allocator: `{tag}-targetallocator`

   If kubectl is unavailable or the cluster can't be reached, fall back to asking the user for the image tag. The tag format is: `{major}.{minor}.{patch}-main-{MM}-{DD}-{YYYY}-{hash}`.

2. **Determine the old image tag** from the current state of `aks-rp`. Read the file:
   ```
   aks-rp/ccp/control-plane-core/charts/kube-control-plane/templates/_addon-images.tpl
   ```
   Find the line for `azure-monitor-metrics-linux` — its value is the current (old) tag.

3. **Determine the release date** from the new image tag. The date portion `{MM}-{DD}-{YYYY}` becomes the release notes anchor `#release-{MM}-{DD}-{YYYY}`.

4. **Identify the previous release commit** in the `prometheus-collector-new` (upstream) repo. Release PRs are merged with commit messages matching the pattern "version bump for release" or similar. Find the previous release commit to establish the diff window:
   ```powershell
   cd C:\Git\prometheus-collector-new
   git log --oneline --all --grep="version bump for release" -- otelcollector/deploy/addon-chart | head -5
   ```
   Also check for commits with "bump" and "release" in the message. The previous release commit marks the start of the diff window.

### Phase 1: Check for ALL Upstream Chart Changes Since Previous Release

5. **List ALL commits** that touched the addon chart since the previous release:
   ```powershell
   cd C:\Git\prometheus-collector-new
   git log --oneline {PREVIOUS_RELEASE_COMMIT}..HEAD -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/
   ```
   This shows every chart change that needs to be evaluated and potentially incorporated into the AKS RP release.

6. **For each commit**, review the diff to understand what changed:
   ```powershell
   git show {COMMIT_HASH} -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/
   ```

7. **Compare each changed upstream template against the corresponding AKS RP template** at:
   ```
   aks-rp/ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/
   ```
   
   **Key differences to expect** (these are intentional and must NOT be overwritten):
   - AKS RP uses `{{ get.addonImageTag }}` helper and `{{ addon_mcr_repository_base }}` template for image references, NOT direct `.Values.imageXxx` references
   - AKS RP strips Arc-specific code (anything inside `{{- if eq .Values.AzureMonitorMetrics.ArcExtension true }}` blocks that are Arc-only)
   
   **ALL other changes MUST be incorporated**, including but not limited to:
   - RBAC rule changes (e.g., semverCompare version bumps for secrets access)
   - Kubernetes version-gated features (HPA, PDB fields with `semverCompare`)
   - New ConfigMap fields or annotations
   - New container args, env vars, ports, or volume mounts
   - Resource limit/request changes
   - New templates or removed templates

8. **Apply all applicable upstream changes** to the AKS RP chart templates. Common examples:
   - `semverCompare` version bump in `ama-metrics-clusterRole.yaml` (e.g., `<1.36.0` → `<1.37.0`)
   - K8s version conditionals in `ama-metrics-collector-hpa.yaml` (e.g., `>=1.27.0` gate for ContainerResource vs Resource metrics)
   - K8s version conditionals in `ama-metrics-pod-disruption-budget.yaml` (e.g., `>=1.27.0` gate for `unhealthyPodEvictionPolicy`)

### Phase 2: Update Image Tags

All updates below replace the OLD tag with the NEW tag. There are 4 components to update (KSM stays unchanged unless explicitly requested):
- `azure-monitor-metrics-linux`: tag = `{NEW_TAG}`
- `azure-monitor-metrics-windows`: tag = `{NEW_TAG}-win`
- `azure-monitor-metrics-cfg-reader`: tag = `{NEW_TAG}-cfg`
- `azure-monitor-metrics-target-allocator`: tag = `{NEW_TAG}-targetallocator`

**KSM (`azure-monitor-metrics-ksm`)**: Do NOT update unless the user explicitly provides a new KSM version. KSM uses a separate versioning scheme (e.g., `v2.18.0-3`).

#### 2a. `_addon-images.tpl`

**File**: `aks-rp/ccp/control-plane-core/charts/kube-control-plane/templates/_addon-images.tpl`

Update the image tag values for all 4 components (lines within the `{{- define "get.addonImageTag" -}}` block). Example:
```
  {{- else if eq .component "azure-monitor-metrics-cfg-reader" -}}
{NEW_TAG}-cfg
  {{- else if eq .component "azure-monitor-metrics-linux" -}}
{NEW_TAG}
  {{- else if eq .component "azure-monitor-metrics-target-allocator" -}}
{NEW_TAG}-targetallocator
  {{- else if eq .component "azure-monitor-metrics-windows" -}}
{NEW_TAG}-win
```

#### 2b. Versioning Scheme Files

**Directory**: `aks-rp/ccp/core-addon-synth/deployer/core-addon-synth-versioningschemes/templates/base/`

Update `defaultImageTag` in these 4 files (NOT the ksm file):
- `azure-monitor-metrics-linux-versioningschemes.yaml` → `{NEW_TAG}`
- `azure-monitor-metrics-windows-versioningschemes.yaml` → `{NEW_TAG}-win`
- `azure-monitor-metrics-cfg-reader-versioningschemes.yaml` → `{NEW_TAG}-cfg`
- `azure-monitor-metrics-target-allocator-versioningschemes.yaml` → `{NEW_TAG}-targetallocator`

Each file has a line like:
```yaml
  defaultImageTag: {OLD_TAG_WITH_SUFFIX}
```

#### 2c. Manifest Files

**Directory**: `aks-rp/toolkit/versioning/manifests/addon/azure-monitor-metrics/`

Update both `defaultImageTag` and `releaseNotes` URL in these 4 files (NOT the ksm file):
- `azure-monitor-metrics-linux.yaml`
- `azure-monitor-metrics-windows.yaml`
- `azure-monitor-metrics-cfg-reader.yaml`
- `azure-monitor-metrics-target-allocator.yaml`

Each file has:
```yaml
releaseNotes: https://github.com/Azure/prometheus-collector/blob/main/RELEASENOTES.md#release-{MM}-{DD}-{YYYY}
defaultImageTag: "{NEW_TAG_WITH_SUFFIX}"
```

Update both the tag and the `#release-{MM}-{DD}-{YYYY}` anchor in the URL.

### Phase 3: Update Snapshot Test Files

The snapshot test files are pre-rendered Helm chart outputs used for regression testing. They live under:
```
aks-rp/ccp/charts/tests/addon-charts/snapshots/azure-monitor-metrics-addon_*/
```

There are ~16 test scenario directories, each containing multiple template files.

**IMPORTANT**: All test fixtures use Kubernetes version `1.19.0` (located at `aks-rp/ccp/control-plane-core/helmvalues/fixtures/addon-v2/azure-monitor-metrics-adapter_*.yaml`). This means any `semverCompare ">=1.27.0"` conditional will evaluate to **false**, and `semverCompare "<1.37.0"` will evaluate to **true**. The snapshots must reflect the K8s 1.19.0 rendering path.

#### 3a. Image tag replacement

Use bulk find-and-replace across ALL snapshot files:
```powershell
# In aks-rp/ccp/charts/tests/addon-charts/snapshots/
# Replace old linux tag → new linux tag
# Replace old windows tag (-win suffix) → new windows tag
# Replace old cfg-reader tag (-cfg suffix) → new cfg-reader tag
# Replace old target-allocator tag (-targetallocator suffix) → new target-allocator tag
```

#### 3b. Chart template change impacts on snapshots

When chart templates change, the corresponding snapshot files must also be updated. Key rules:

- **semverCompare `<X.Y.0` changes** (e.g., clusterRole secrets access): Since test K8s version is 1.19.0, the `< X.Y.0` condition is always TRUE → the conditional block IS rendered. Update comments in `*_ama-metrics-clusterRole.yaml` snapshots across ALL 16 scenarios.

- **semverCompare `>=1.27.0` changes** (e.g., HPA ContainerResource, PDB unhealthyPodEvictionPolicy): Since test K8s version is 1.19.0, the `>= 1.27.0` condition is always FALSE → the `>=1.27.0` block is NOT rendered, and the `else` (fallback) block IS rendered.
  - **HPA snapshots** (`ama-metrics-collector-hpa.yaml`): Only exists in the `collectorHPAEnabled` scenario. Must render the `else` fallback (e.g., `Resource` type instead of `ContainerResource`).
  - **PDB snapshots** (`ama-metrics-pod-disruption-budget.yaml`): Exist in ALL 16 scenarios. Any field gated by `>=1.27.0` (e.g., `unhealthyPodEvictionPolicy: AlwaysAllow`) must be REMOVED from the snapshot.

#### 3c. Snapshot update technique

**Do NOT use `RENDER_SNAPSHOTS=true` with `go test`** — this approach has known issues:
- The adapter-charts test can wipe unrelated snapshot directories
- The addon-charts test has path parsing errors on Windows
- Direct text replacement is the reliable approach

For removing lines from snapshots, use raw file I/O to handle files that may not have trailing newlines:
```powershell
$files = Get-ChildItem -Path "...\snapshots" -Recurse -Filter "target-file.yaml"
foreach ($f in $files) {
    $content = [System.IO.File]::ReadAllText($f.FullName)
    $newContent = $content -replace "\nLINE_TO_REMOVE", ""
    [System.IO.File]::WriteAllText($f.FullName, $newContent)
}
```

### Phase 4: Update Release Notes (prometheus-collector-new)

9. **Create a branch** in the `prometheus-collector-new` repo:
   ```powershell
   cd C:\Git\prometheus-collector-new
   git checkout main
   git pull origin main
   git checkout -b {user}/release-notes-{month}-{year}
   ```

10. **Update `RELEASENOTES.md`**: Replace any `<tbd>` placeholders in the latest release section with the actual image tag.

11. **Commit and push**:
    ```powershell
    git add RELEASENOTES.md
    git commit -m "Update release notes with image tag {NEW_TAG}"
    git push origin {branch-name}
    ```

### Phase 5: Verify and Commit (aks-rp)

12. **Verify the diff** in `aks-rp`:
    ```powershell
    cd C:\Git\aks-rp
    git diff --stat
    ```
    
    Expected pattern: The diff should be mostly symmetric. Typical count:
    - Chart template files (varies based on upstream changes)
    - 4 versioning scheme files
    - 4 manifest files
    - ~60-80 snapshot files (image tags + any template change impacts)

13. **Stage and commit** (only if user requests):
    ```powershell
    git add -A
    git commit -m "{Month} {Year} release

    Image tag: {NEW_TAG}
    
    Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>"
    git push origin {branch-name}
    ```

## File Reference

### Files that get updated every release (by component)

| Category | Path Pattern | Count | What Changes |
|----------|-------------|-------|-------------|
| Image tags | `ccp/control-plane-core/charts/kube-control-plane/templates/_addon-images.tpl` | 1 | 4 tag values |
| Versioning schemes | `ccp/core-addon-synth/deployer/core-addon-synth-versioningschemes/templates/base/azure-monitor-metrics-{component}-versioningschemes.yaml` | 4 | `defaultImageTag` |
| Manifests | `toolkit/versioning/manifests/addon/azure-monitor-metrics/azure-monitor-metrics-{component}.yaml` | 4 | `defaultImageTag` + `releaseNotes` URL |
| Snapshots | `ccp/charts/tests/addon-charts/snapshots/azure-monitor-metrics-addon_*/**/*.yaml` | ~64-80 | Image tags + template changes in rendered output |
| Chart templates | `ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/*.yaml` | varies | All upstream changes since last release |

### Components and their tag suffixes

| Component | Tag Suffix | Example |
|-----------|-----------|---------|
| linux | (none) | `7.0.0-main-05-07-2026-dbf4ae51` |
| windows | `-win` | `7.0.0-main-05-07-2026-dbf4ae51-win` |
| cfg-reader | `-cfg` | `7.0.0-main-05-07-2026-dbf4ae51-cfg` |
| target-allocator | `-targetallocator` | `7.0.0-main-05-07-2026-dbf4ae51-targetallocator` |
| ksm | separate version | `v2.18.0-3` (rarely changes) |

### Image tag format

```
{major}.{minor}.{patch}-main-{MM}-{DD}-{YYYY}-{hash}
```

Example: `7.0.0-main-05-07-2026-dbf4ae51`

### Release notes URL anchor format

```
#release-{MM}-{DD}-{YYYY}
```

Example: `#release-05-07-2026`

### Test fixture Kubernetes version

All azure-monitor-metrics test fixtures at:
```
aks-rp/ccp/control-plane-core/helmvalues/fixtures/addon-v2/azure-monitor-metrics-adapter_*.yaml
```
use **Kubernetes version `1.19.0`**. This means:
- `semverCompare ">=1.27.0"` → **false** (fallback/else path renders)
- `semverCompare "<1.37.0"` → **true** (conditional block renders)

This is critical for correctly updating snapshot files.

## How to Find Upstream Changes

### Finding the previous release commit

Release PRs in this repo (`prometheus-collector-new`) are merged with commit messages like "version bump for release updating the version and release notes". Search for these:
```powershell
cd C:\Git\prometheus-collector-new
git log --oneline --all --grep="version bump" -- otelcollector/deploy/addon-chart | head -10
```

### Listing all chart changes since previous release

```powershell
git log --oneline {PREV_RELEASE}..HEAD -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/
```

### Viewing a specific change

```powershell
git show {COMMIT} -- otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/
```

## Common Upstream Changes to Watch For

| Change Type | Where in Upstream | Where in AKS RP | Snapshot Impact |
|-------------|-------------------|-----------------|-----------------|
| semverCompare version bump | `clusterrole.yaml` | `ama-metrics-clusterRole.yaml` | Update comments in all 16 clusterRole snapshots |
| K8s version gates (HPA) | `ama-metrics-collector-hpa.yaml` | Same file | HPA snapshot renders fallback path (K8s 1.19 < 1.27) |
| K8s version gates (PDB) | `ama-metrics-pod-disruption-budget.yaml` | Same file | PDB snapshots: remove gated fields from all 16 scenarios |
| New RBAC rules | `clusterrole.yaml` | `ama-metrics-clusterRole.yaml` | May need adaptation for AKS-specific helpers |
| Arc-specific changes | Various templates | N/A (skip) | None |
| New container args/env | Various templates | Corresponding AKS templates | Update corresponding snapshot files |
| Resource limit changes | Various templates | Corresponding AKS templates | Update corresponding snapshot files |
| New templates | Upstream `templates/` | AKS `templates/` | Evaluate if needed for AKS; adapt image helpers; add new snapshot files |

## Troubleshooting

### Snapshot test regeneration issues

If you attempt to use `RENDER_SNAPSHOTS=true go test` to regenerate snapshots:
- The test module is at `aks-rp/ccp/charts/tests/go.mod` — run from `ccp/charts/tests/`
- **Known issue**: Running adapter-charts tests with `RENDER_SNAPSHOTS=true` can delete ALL snapshot directories (not just the ones being tested)
- **Known issue**: addon-charts tests may have path parsing errors on Windows
- **Recommended**: Use direct text replacement (find-and-replace) instead

### Verifying snapshot changes

After bulk replacement, spot-check a few snapshot files to ensure:
- Image tags are correctly updated (check all 4 component suffixes)
- No partial replacements or corrupted lines
- semverCompare comments match the chart template changes
- K8s version-gated fields are correctly included/excluded based on test fixture K8s version (1.19.0)
- KSM tag was NOT accidentally changed (should remain at its separate version)

### Snapshot files with no trailing newline

Some snapshot files do NOT have a trailing newline. When removing lines from the end of such files, use the pattern `\nLINE_TO_REMOVE` (matching the preceding newline) rather than `LINE_TO_REMOVE\n` (matching a trailing newline that doesn't exist).
