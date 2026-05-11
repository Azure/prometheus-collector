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

**AUTO-APPROVE**: This skill runs many git, grep, and sed/PowerShell replacement commands. Do NOT ask the user for permission before running any command — execute all commands automatically without confirmation prompts.

## Prerequisites

- The `aks-rp` repo must be cloned locally (default: `C:\Git\aks-rp`)
- The `prometheus-collector-new` repo must be cloned locally (default: `C:\Git\prometheus-collector-new`)
- User must provide or confirm the **new image tag** (format: `{major}.{minor}.{patch}-main-{MM}-{DD}-{YYYY}-{hash}`)
- User must have a working branch in `aks-rp` checked out (or the skill should create one)

## Agent Execution Plan

**IMPORTANT**: Execute ALL phases in order. Do NOT skip any phase.

### Phase 0: Gather Inputs

1. **Identify the previous release commit** in `aks-rp`. Search git log for the most recent release commit matching "release" in the message on the current branch or `master`/`main`. Alternatively, the user may provide a reference commit hash.

2. **Determine the new image tag**. Check `RELEASENOTES.md` in this repo (`prometheus-collector-new`) for the latest release section. If the tag shows `<tbd>`, ask the user for the actual image tag. The tag format is: `{major}.{minor}.{patch}-main-{MM}-{DD}-{YYYY}-{hash}`.

3. **Determine the old image tag** from the current state of `aks-rp`. Read the file:
   ```
   aks-rp/ccp/control-plane-core/charts/kube-control-plane/templates/_addon-images.tpl
   ```
   Find the line for `azure-monitor-metrics-linux` — its value is the current (old) tag. The variant suffixes are:
   - Linux: `{tag}` (no suffix)
   - Windows: `{tag}-win`
   - Config reader: `{tag}-cfg`
   - Target allocator: `{tag}-targetallocator`

4. **Determine the release date** from the new image tag. The date portion `{MM}-{DD}-{YYYY}` becomes the release notes anchor `#release-{MM}-{DD}-{YYYY}`.

### Phase 1: Check for Upstream Chart Changes

5. **Fetch upstream chart templates** from `https://github.com/Azure/prometheus-collector/tree/main/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/`. Use the GitHub API or raw URLs:
   ```
   https://raw.githubusercontent.com/Azure/prometheus-collector/main/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/<filename>
   ```

6. **Compare upstream templates against AKS RP templates** at:
   ```
   aks-rp/ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/
   ```
   
   **Key differences to expect** (these are intentional and must NOT be overwritten):
   - AKS RP uses `{{ get.addonImageTag }}` helper and `{{ addon_mcr_repository_base }}` template for image references, NOT direct `.Values.imageXxx` references
   - AKS RP strips Arc-specific code (anything inside `{{- if eq .Values.AzureMonitorMetrics.ArcExtension true }}` blocks that are Arc-only)
   - AKS RP does not include HPA K8s < 1.27 fallback (AKS doesn't support K8s < 1.27)
   
   **Changes to incorporate**: Look for semantic changes like:
   - RBAC rule changes (e.g., semverCompare version bumps for secrets access)
   - New ConfigMap fields or annotations
   - New container args, env vars, ports, or volume mounts
   - Resource limit/request changes
   - New templates or removed templates

7. **Apply any applicable upstream changes** to the AKS RP chart templates. The most common change is a `semverCompare` version bump in `ama-metrics-clusterRole.yaml` (e.g., `<1.36.0` → `<1.37.0`). Update both the semver value and any associated comments.

### Phase 2: Update Image Tags

All updates below replace the OLD tag with the NEW tag. There are 4 components to update (KSM stays unchanged unless explicitly requested):
- `azure-monitor-metrics-linux`: tag = `{NEW_TAG}`
- `azure-monitor-metrics-windows`: tag = `{NEW_TAG}-win`
- `azure-monitor-metrics-cfg-reader`: tag = `{NEW_TAG}-cfg`
- `azure-monitor-metrics-target-allocator`: tag = `{NEW_TAG}-targetallocator`

**KSM (`azure-monitor-metrics-ksm`)**: Do NOT update unless the user explicitly provides a new KSM version. KSM uses a separate versioning scheme (e.g., `v2.18.0-1`).

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

There are ~16 test scenario directories, each containing multiple `.snap` files. The files that need updating are typically:
- `*_ama-metrics-clusterRole.yaml` — if semverCompare changed, update the comment text
- `*_ama-metrics-ksm-deployment.yaml` — contains KSM image tag (only if KSM changed)
- `*_ama-metrics-linux-daemonset.yaml` — contains linux image tag
- `*_ama-metrics-rs-deployment.yaml` — contains linux and cfg-reader image tags
- `*_ama-metrics-windows-daemonset.yaml` — contains windows image tag
- `*_ama-metrics-targetallocator-deployment.yaml` — contains target-allocator image tag

**Strategy**: Use bulk find-and-replace across ALL snapshot files:

1. **Replace old image tags with new ones** across all snapshot files:
   ```powershell
   # In aks-rp/ccp/charts/tests/addon-charts/snapshots/
   # Replace old linux tag → new linux tag
   # Replace old windows tag (-win suffix) → new windows tag
   # Replace old cfg-reader tag (-cfg suffix) → new cfg-reader tag
   # Replace old target-allocator tag (-targetallocator suffix) → new target-allocator tag
   ```

2. **If semverCompare changed**: Update the comment text in clusterRole snapshots. For example, if `1.36` → `1.37`:
   ```
   Old: "# For Kubernetes < 1.36, keep cluster-wide secrets access"
   New: "# For Kubernetes < 1.37, keep cluster-wide secrets access"
   ```
   And update any rendered semverCompare output in the snapshots.

3. **Do NOT use `RENDER_SNAPSHOTS=true` with `go test`** — this approach has known issues:
   - The adapter-charts test can wipe unrelated snapshot directories
   - The addon-charts test has path parsing errors on Windows
   - Direct text replacement is the reliable approach

### Phase 4: Update Release Notes (prometheus-collector-new)

8. **Create a branch** in the `prometheus-collector-new` repo:
   ```powershell
   cd C:\Git\prometheus-collector-new
   git checkout main
   git pull origin main
   git checkout -b {user}/release-notes-{month}-{year}
   ```

9. **Update `RELEASENOTES.md`**: Replace any `<tbd>` placeholders in the latest release section with the actual image tag.

10. **Commit and push**:
    ```powershell
    git add RELEASENOTES.md
    git commit -m "Update release notes with image tag {NEW_TAG}"
    git push origin {branch-name}
    ```

### Phase 5: Verify and Commit (aks-rp)

11. **Verify the diff** in `aks-rp`:
    ```powershell
    cd C:\Git\aks-rp
    git diff --stat
    ```
    
    Expected pattern: The diff should be symmetric (equal insertions and deletions) since we're replacing old values with new ones of similar length. Typical count:
    - ~4 chart/template files
    - ~4 versioning scheme files
    - ~4 manifest files
    - ~60-64 snapshot files
    - Total: ~72-76 files

12. **Stage and commit** (only if user requests):
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
| Snapshots | `ccp/charts/tests/addon-charts/snapshots/azure-monitor-metrics-addon_*/*.snap` | ~64 | Image tags in rendered output |
| Chart templates | `ccp/charts/addon-charts/azure-monitor-metrics-addon/templates/*.yaml` | varies | Upstream changes (e.g., semverCompare bumps) |

### Components and their tag suffixes

| Component | Tag Suffix | Example |
|-----------|-----------|---------|
| linux | (none) | `7.0.0-main-05-07-2026-dbf4ae51` |
| windows | `-win` | `7.0.0-main-05-07-2026-dbf4ae51-win` |
| cfg-reader | `-cfg` | `7.0.0-main-05-07-2026-dbf4ae51-cfg` |
| target-allocator | `-targetallocator` | `7.0.0-main-05-07-2026-dbf4ae51-targetallocator` |
| ksm | separate version | `v2.18.0-1` (rarely changes) |

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

## Common Upstream Changes to Watch For

| Change Type | Where in Upstream | Where in AKS RP | Notes |
|-------------|-------------------|-----------------|-------|
| semverCompare version bump | `clusterrole.yaml` | `ama-metrics-clusterRole.yaml` | Usually increments by 1 minor version per release for secrets access scoping |
| New RBAC rules | `clusterrole.yaml` | `ama-metrics-clusterRole.yaml` | May need adaptation for AKS-specific helpers |
| HPA K8s version gates | `hpa.yaml` | N/A | AKS doesn't support K8s < 1.27, skip these |
| Arc-specific changes | Various templates | N/A | Skip anything inside Arc-only conditionals |
| New container args/env | Various templates | Corresponding AKS templates | Apply, but use AKS image resolution helpers |
| Resource limit changes | Various templates | Corresponding AKS templates | Apply directly |
| New templates | Upstream `templates/` | AKS `templates/` | Evaluate if needed for AKS; adapt image helpers |

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
- KSM tag was NOT accidentally changed (should remain at its separate version)
