---
name: upgrade-kube-state-metrics
description: Upgrade the kube-state-metrics (KSM) image shipped with the azure-monitor-metrics addon to the latest Microsoft-built (dalec) version, verify it against MCR, and generate the PR changelog. Use when "upgrade kube-state-metrics", "bump KSM version", "update kube-state-metrics image tag", "new kube-state-metrics release", or "upgrade ksm to <version>".
allowed-tools:
  - run_in_terminal
  - read_file
  - edit_file
  - create_file
---

# Upgrade kube-state-metrics (KSM)

Automates bumping the KSM image tag consumed by the Azure Monitor Metrics addon.
The addon pulls the Microsoft-built (dalec) image from MCR
(`mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics:<tag>`), where the tag is
`v<upstreamVersion>-<dalecRevision>` (e.g. `v2.20.0-4`).

**AUTO-APPROVE**: Run all `gh`, `git`, `grep`, and PowerShell commands automatically; do not ask for confirmation.

## Source-of-truth repositories

- **dalec build defs** (what Microsoft builds + revision/CVE patches): <https://github.com/Azure/dalec-build-defs/tree/main/specs/kube-state-metrics>
- **MCR** (what is actually pullable): `mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics` — <https://mcr.microsoft.com/v2/oss/v2/kubernetes/kube-state-metrics/tags/list>
- **Upstream releases** (the changelog): <https://github.com/kubernetes/kube-state-metrics/releases>
- **prometheus-community Helm chart** (version ↔ appVersion mapping for the code comment): <https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics>

## Files this skill edits

Exactly two committed files carry the tag (keep them in sync):

| File | Field |
|------|-------|
| `.pipelines/azure-pipeline-build.yml` | `KUBE_STATE_METRICS_IMAGE` |
| `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml` | `KubeStateMetrics.ImageTag` + the `# ... corresponds to chart version` comment |

When the upstream chart delta (Phase 5) requires it, also edit the addon KSM manifests:
`templates/ama-metrics-ksm-role.yaml` (RBAC) and/or `templates/ama-metrics-ksm-deployment.yaml`
(args/probes/ports).

Do **not** edit `values-rashmi-operator-cfg.yaml` (local developer override).

## Execution plan

### Phase 0 — Current version
Find the tag in use:
```powershell
Select-String -Path .pipelines/azure-pipeline-build.yml,`
  otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml `
  -Pattern 'kube-state-metrics:v|ImageTag'
```
Record `CURRENT` (e.g. `v2.19.1-2`) and its upstream version (`v2.19.1`).

### Phase 1 — Latest upstream release
```powershell
gh api repos/kubernetes/kube-state-metrics/releases --jq '.[] | select(.prerelease==false) | .tag_name' | Select-Object -First 5
```
Pick the target upstream version `NEW_UPSTREAM` (e.g. `v2.20.0`). If the user named a specific version, use it.

### Phase 2 — Confirm dalec has it + get the revision
List specs and read the target spec + matrix:
```powershell
gh api repos/Azure/dalec-build-defs/contents/specs/kube-state-metrics --jq '.[].name'
$v = '2.20.0'
gh api "repos/Azure/dalec-build-defs/contents/specs/kube-state-metrics/kube-state-metrics-$v.yml" `
  --jq '.content' | % { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($_)) }
```
From the spec read `REVISION`, `COMMIT`, and the `changelog` (CVE/dependency patches).
If dalec has **no** spec for the version, STOP and tell the user the version is not yet built by Microsoft — only dalec-built versions in MCR can be shipped.

### Phase 3 — Verify the exact tag exists in MCR (source of truth)
```powershell
(Invoke-RestMethod "https://mcr.microsoft.com/v2/oss/v2/kubernetes/kube-state-metrics/tags/list").tags |
  Where-Object { $_ -like "$($v.Substring(0,4))*" } | Sort-Object
```
Choose the **highest** `v<version>-<revision>` present (e.g. `v2.20.0-4`). This is `NEW` = `v2.20.0-4`.
The revision here should match the dalec spec `REVISION`.

### Phase 4 — Map the Helm chart version (for the comment)
Find the first prometheus-community chart version whose `appVersion` equals `NEW_UPSTREAM`:
```powershell
foreach ($t in (gh api "repos/prometheus-community/helm-charts/git/matching-refs/tags/kube-state-metrics-8" `
    --jq '.[].ref' | % { $_ -replace 'refs/tags/kube-state-metrics-','' } | Sort-Object {[version]$_})) {
  $c = gh api "repos/prometheus-community/helm-charts/contents/charts/kube-state-metrics/Chart.yaml?ref=kube-state-metrics-$t" `
       --jq '.content' | % { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($_)) }
  "$t -> " + ($c | Select-String 'appVersion:').ToString().Trim()
}
```
Record `CHART` = the first chart version at the new appVersion (e.g. `8.4.0`).

### Phase 5 — Check upstream Helm chart changes (port relevant ones)

The addon keeps its **own** KSM manifests (`otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/ama-metrics-ksm-*.yaml`), forked from the prometheus-community chart. Diff the upstream chart across the app-version boundary — the last chart at the **old** appVersion (`OLD_CHART`) → the first at the **new** one (`NEW_CHART` from Phase 4) — and port anything relevant.

```powershell
$OLD_CHART = '8.3.1'   # last chart at the previous appVersion
$NEW_CHART = '8.4.0'   # first chart at the new appVersion (from Phase 4)

# 1) file-level delta for the KSM chart
gh api "repos/prometheus-community/helm-charts/compare/kube-state-metrics-$OLD_CHART...kube-state-metrics-$NEW_CHART" `
  --jq '.files[] | select(.filename|startswith("charts/kube-state-metrics/")) | "\(.status)  \(.filename)"'

# 2) RBAC diff (most important — new metrics often need new list/watch rules)
foreach ($ref in $OLD_CHART,$NEW_CHART) {
  gh api "repos/prometheus-community/helm-charts/contents/charts/kube-state-metrics/templates/role.yaml?ref=kube-state-metrics-$ref" `
    --jq '.content' | % { [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String(($_ -replace '\s',''))) } |
    Set-Content "$env:TEMP\ksm_role_$ref.yaml"
}
git --no-pager diff --no-index -- "$env:TEMP\ksm_role_$OLD_CHART.yaml" "$env:TEMP\ksm_role_$NEW_CHART.yaml"
```

Decide per change:
- **RBAC rule added for a collector the addon enables** → mirror it into `ama-metrics-ksm-role.yaml`.
- **New required container arg / flag / probe / port** → mirror it into `ama-metrics-ksm-deployment.yaml`.
- **New opt-in collector** (e.g. admission policies, standalone DRA `resourceclaims`) → add the collector to `KubeStateMetrics.Collectors` in `values-template.yaml` **and** its RBAC rule **only if** the new metrics are wanted (upstream leaves these off by default).
- **Chart-only / packaging changes** (`Chart.yaml`, README, values plumbing the addon doesn't use) → ignore.

If the delta is version-only (just `Chart.yaml`), there is nothing to port — record that in the PR description and the readme (as was the case for 2.19.1 → 2.20.0).

### Phase 6 — Collect the upstream changelog (PR description)
```powershell
gh api "repos/kubernetes/kube-state-metrics/compare/$CURRENT_UPSTREAM...$NEW_UPSTREAM" `
  --jq '{commits: .total_commits, files: (.files|length)}'
gh api "repos/kubernetes/kube-state-metrics/releases/tags/$NEW_UPSTREAM" --jq '.body'
```
Keep the categorized `[CHANGE]/[FEATURE]/[ENHANCEMENT]/[BUGFIX]` list for the PR body. Call out any **cardinality / breaking** changes explicitly.

### Phase 7 — Edit both files
- `.pipelines/azure-pipeline-build.yml`: set `KUBE_STATE_METRICS_IMAGE` to `mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics:$NEW`.
- `values-template.yaml`: set `ImageTag: "$NEW"` and update the comment line to
  `# Kube-state-metrics ImageTag - <upstream>, corresponds to chart version - $CHART`.

### Phase 8 — Verify
```powershell
# new tag present in the two shipped files; old tag gone
Select-String -Path .pipelines/azure-pipeline-build.yml,`
  otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml `
  -Pattern ([regex]::Escape($NEW))
git --no-pager diff -- .pipelines/azure-pipeline-build.yml `
  otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml
```

### Phase 9 — Docs, commit, and open the PR (automated)
- Update `internal/docs/kube-state-metrics-upgrade.md` with the new before/after row and changelog.
- Stage **only** the intended files (not developer overrides like `values-rashmi-operator-cfg.yaml`) and commit with subject `build(deps): Upgrade kube-state-metrics from <old> to <new>` plus the required Copilot trailers.
- Fill the PR-description template below with the Phase 6 changelog, the dalec `-<rev>` CVE patches, and the Phase 5 chart-delta finding, then push the branch and open the PR automatically:

```powershell
$branch = git rev-parse --abbrev-ref HEAD
# write the filled-in PR description to a temp file (single-quoted here-string keeps the markdown intact)
$body = @'
<paste the filled-in PR-description template here>
'@
Set-Content -Path "$env:TEMP\ksm_pr_body.md" -Value $body -Encoding utf8

git push -u origin $branch
gh pr create --base main --head $branch `
  --title "build(deps): Upgrade kube-state-metrics from <old> to <new>" `
  --body-file "$env:TEMP\ksm_pr_body.md"
gh pr view --web        # optional: open the new PR in a browser
```

- If a PR already exists for the branch, `gh pr create` errors — refresh its description instead with
  `gh pr edit $branch --body-file "$env:TEMP\ksm_pr_body.md"`.

## PR description template

```markdown
### Upgrade kube-state-metrics <CURRENT> → <NEW>

- Upstream: <CURRENT_UPSTREAM> → <NEW_UPSTREAM> (dalec revision <rev>)
- Image: mcr.microsoft.com/oss/v2/kubernetes/kube-state-metrics:<NEW>
- Helm chart mapping: <CHART> (appVersion <NEW_UPSTREAM>)
- Verified present in MCR: yes

#### Upstream changes (<CURRENT_UPSTREAM>...<NEW_UPSTREAM>)
<categorized [CHANGE]/[FEATURE]/[ENHANCEMENT]/[BUGFIX] list>

#### Microsoft (dalec) build patches in revision <rev>
<CVE/dependency replaces from the dalec spec changelog>

#### Upstream Helm chart delta (chart <OLD_CHART> → <CHART>)
<version-only, or the RBAC/args changes ported into the addon's ama-metrics-ksm-*.yaml>

#### Impact / breaking notes
<cardinality changes, new collectors to enable, etc.>

Refs:
- dalec: https://github.com/Azure/dalec-build-defs/tree/main/specs/kube-state-metrics
- MCR tags: https://mcr.microsoft.com/v2/oss/v2/kubernetes/kube-state-metrics/tags/list
- Releases: https://github.com/kubernetes/kube-state-metrics/releases
- Diff: https://github.com/kubernetes/kube-state-metrics/compare/<CURRENT_UPSTREAM>...<NEW_UPSTREAM>
- Helm chart: https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-state-metrics
- Helm chart diff: https://github.com/prometheus-community/helm-charts/compare/kube-state-metrics-<OLD_CHART>...kube-state-metrics-<CHART>
```

## Guardrails

- Only ship a version that **exists in MCR** with a dalec spec — never a raw upstream tag.
- Always take the **highest dalec revision** for the chosen upstream version (revisions carry CVE fixes).
- Keep the two tag references identical.
- Diff the upstream chart's `role.yaml`/`deployment.yaml` across the app-version boundary (Phase 5) and port only **relevant** rules (RBAC for enabled collectors, required args/probes) into the addon's `ama-metrics-ksm-*.yaml`; ignore chart-only/packaging changes.
- Leave local override files untouched.
