---
name: code-review
description: >
  Code review skill for the Azure Managed Prometheus (prometheus-collector) repository.
  Reviews PRs against team conventions, common pitfalls, and repo-specific patterns.
  USE FOR: code review, PR review, review my changes, check my PR, review diff,
  review code, find bugs, check for issues, validate changes.
  DO NOT USE FOR: writing new features, troubleshooting ICMs, build fixes.
argument-hint: 'Provide the PR number, branch name, or say "review my changes" to review staged/unstaged changes'
---

# Azure Managed Prometheus — Code Review Skill

Review code changes in the `Azure/prometheus-collector` repository against team conventions,
common pitfalls, and repo-specific patterns learned from past PR reviews.

## How to Use

When invoked, determine what to review:
- **"review my changes"** → review staged/unstaged git diff
- **PR number** → fetch PR diff via GitHub MCP tools
- **branch name** → diff against main

Then apply the review checklist below. **Only flag issues that genuinely matter** — bugs, correctness,
security, operational risk. Do NOT comment on style, formatting, or trivial naming preferences.

## Review Checklist

### 1. Correctness & Logic

- **Command-line argument formatting** — MetricsExtension (ME) CLI args use `-FlagName` format (single dash, PascalCase). A common mistake is using `OperationEnvironment` instead of `-OperationEnvironment`. Verify all `exec.Command()` calls in `otelcollector/shared/process_utilities_*.go` pass flags with the leading dash
- **Wrong configmap scope** — cluster-wide scrape jobs MUST go in `ama-metrics-prometheus-config` (ReplicaSet). If placed in `ama-metrics-prometheus-config-node` (DaemonSet), targets get scraped N times (once per node). Check any configmap changes for this pattern
- **Keep list vs drop list** — when adding default scrape configs, prefer `keep` over `drop` for `metric_relabel_configs` to avoid unintentionally letting new metrics through. This is a deliberate team preference to minimize ongoing maintenance risk
- **Helm values defaults** — feature flags in `values-template.yaml` should default to `false` unless the feature is GA. A past review caught a flag incorrectly defaulting to `true`
- **CCP vs addon deployment** — changes to addon Helm chart templates (`deploy/addon-chart/azure-monitor-metrics-addon/`) may also need corresponding changes in the CCP chart (`deploy/addon-chart/ccp-metrics-plugin/`), and vice versa. But NOT always — ask "does this apply to CCP?" if unclear

### 2. Dependency & Compatibility

- **ME version compatibility** — when adding new MetricsExtension command-line arguments, verify the ME version bundled in the container supports them. Check `otelcollector/scripts/setup.sh` for the ME version being used. Don't assume new CLA flags exist in the current bundled version
- **Fluent-bit build flags** — Dockerfile changes to fluent-bit compilation (`otelcollector/build/linux/Dockerfile`) must match the flags used by [dalec-build-defs](https://github.com/Azure/dalec-build-defs). When `DFLB_*` flags change, corresponding library dependencies (e.g. `postgresql-devel`) may need updating
- **OTel collector version** — `otelcollector` upgrades often require updating Go module versions across multiple `go.mod` files. Check that all `go.mod` files in the repo are consistent
- **Removed/renamed files or flags** — when upgrading dependencies (fluent-bit, OTel, ME), check for removed config files, renamed CLI flags, or deprecated APIs. Past reviews caught references to files that no longer exist in new versions

### 3. Operational Safety

- **Helm template helpers** — AKS RP maintains shared Helm helpers (e.g. `should_mount_hostca`). When adding volume mounts or security contexts, check if an existing helper should be used instead of inline logic. We can defer this to extension migration, but note it in review
- **Environment variables in Dockerfile** — verify Dockerfile `ENV` vars are actually referenced in code. Past reviews found env vars set in the Dockerfile that were never read because the same var was set programmatically in Go code (`telemetry.go`). Dead env vars add confusion
- **Instrumentation key exposure** — App Insights instrumentation keys baked into Dockerfiles or configs should use the correct key for the environment (prod vs test). Verify `APPLICATIONINSIGHTS_AUTH` vs `APPLICATIONINSIGHTS_AUTH_PUBLIC` usage
- **Test coverage for new features** — the PR template requires listing test labels. For config processing changes, add Ginkgo E2E tests under `otelcollector/test/ginkgo-e2e/`. Check if new scrape configs need a corresponding TestKube workflow entry in `run-testkube-workflow.sh`
- **Extension migration awareness** — the codebase is migrating from addon to AKS extension. Helm values and templates are being moved to the extension repo (`aks-rp`). Flag if changes need to be coordinated with the extension team (Pradeep/Rutvij)

### 4. Configuration & Scrape Targets

- **Default scrape config changes** — any new default scrape target in `otelcollector/configmapparser/default-prom-configs/` should have:
  - Corresponding entries in `tomlparser-*-default-targets-metrics-keep-list.go` for minimal ingestion profile
  - `metric_relabel_configs` to control cardinality
  - Documentation of which Grafana dashboards the metrics light up
  - E2E test coverage
- **Scrape interval sanity** — default scrape intervals should be 30s-60s for most targets. 15s is aggressive (only cadvisor uses it). Flag any new default target with < 30s interval
- **Label cardinality** — new scrape configs that include high-cardinality labels (pod, container_id, instance) without filtering can cause customer AMW cost spikes. Check for `labeldrop` or `labelkeep` in `metric_relabel_configs`

### 5. Release & Documentation

- **RELEASENOTES.md** — any user-facing change should have a corresponding entry. Check that the image tag, ME version bumps, and behavioral changes are documented
- **PR title** — must follow [conventional commit format](https://conventionalcommits.org/en/v1.0.0/#summary): `feat:`, `fix:`, `docs:`, `chore:`, `build:`, etc.
- **PR description checklist** — the template includes a New Feature Checklist (telemetry, one-pager, perf testing) and Tests Checklist (Ginkgo labels, test labels, API permissions). Verify relevant items are checked or explained

### 6. File-Specific Patterns

| File/Path Pattern | What to Check |
|---|---|
| `otelcollector/shared/process_utilities_*.go` | ME CLI args have leading dash, correct flag names |
| `otelcollector/build/linux/Dockerfile` | Build flags match dalec, deps are needed, env vars are used |
| `deploy/addon-chart/**/values-template.yaml` | Defaults are safe (features off), required by extension team? |
| `deploy/addon-chart/**/templates/*.yaml` | CCP parity needed? Helm helpers used? |
| `otelcollector/configmapparser/default-prom-configs/` | Keep list updated, interval sane, cardinality controlled |
| `otelcollector/shared/configmap/ccp/` | CCP-specific config changes, keep list regex |
| `otelcollector/test/ginkgo-e2e/` | New test labels registered, TestKube workflow updated |
| `RELEASENOTES.md` | Image tag, changes documented |
| `tools/prom-collector-tsg-mcp/` | `dist/` rebuilt if `src/` changed, queries.ts category correct |

## Review Output Format

Present findings as:

```
## Code Review: [PR title or description]

### 🔴 Must Fix (N)
- [file:line] Description of bug/correctness issue

### 🟡 Should Fix (N)
- [file:line] Description of operational risk or convention violation

### 💡 Suggestions (N)
- [file:line] Optional improvement

### ✅ Looks Good
- Summary of what was verified and found correct
```

Only include sections that have items. If everything looks good, just say ✅ with a brief summary.

## Team Context

- **Primary reviewers**: rashmichandrashekar (lead), suyadav1, zanejohnson-azure, davidkydd
- **External contributors**: GreyCat (ME team), deveshdama (Istio), hsubramanianaks (AKS)
- **Review style**: The team asks "why?" questions to understand intent, requests concrete suggestions with `suggestion` blocks, and flags coordination needs with other teams (extension, ME, AKS RP)
- **Common back-and-forth**: Reviewers often ask for links to upstream references (dalec specs, AKS RP helpers), test evidence, and confirmation that changes work in both addon and CCP deployments
