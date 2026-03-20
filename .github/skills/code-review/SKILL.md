---
name: code-review
description: >
  Code review skill for the Azure Managed Prometheus (prometheus-collector) repository.
  Reviews PRs against team conventions, common pitfalls, and repo-specific patterns
  learned from 26 reviewed PRs spanning 2023-2026.
  USE FOR: code review, PR review, review my changes, check my PR, review diff,
  review code, find bugs, check for issues, validate changes.
  DO NOT USE FOR: writing new features, troubleshooting ICMs, build fixes.
argument-hint: 'Provide the PR number, branch name, or say "review my changes" to review staged/unstaged changes'
---

# Azure Managed Prometheus — Code Review Skill

Review code changes in the `Azure/prometheus-collector` repository against team conventions,
common pitfalls, and repo-specific patterns learned from analysis of 26 PRs with substantive
review comments (out of 43 PRs analyzed, from a pool of 508 reviewed PRs).

## How to Use

When invoked, determine what to review:
- **"review my changes"** → review staged/unstaged git diff
- **PR number** → fetch PR diff via GitHub MCP tools
- **branch name** → diff against main

Then apply the review checklist below. **Only flag issues that genuinely matter** — bugs, correctness,
security, operational risk. Do NOT comment on style, formatting, or trivial naming preferences.

## Review Checklist

### 1. Correctness & Logic

- **ME CLI argument formatting** — MetricsExtension CLI args use `-FlagName` format (single dash, PascalCase). A common mistake is `OperationEnvironment` instead of `-OperationEnvironment`. Verify all `exec.Command()` calls in `otelcollector/shared/process_utilities_*.go` (PR #1403)
- **Wrong configmap scope** — cluster-wide scrape jobs MUST go in `ama-metrics-prometheus-config` (ReplicaSet). If placed in `ama-metrics-prometheus-config-node` (DaemonSet), targets get scraped N times per node
- **Keep list vs drop list** — prefer `keep` over `drop` for `metric_relabel_configs` to avoid unintentionally letting new metrics through. Team preference to minimize ongoing maintenance risk (PR #1393)
- **Helm values defaults** — feature flags in `values-template.yaml` must default to `false` unless the feature is GA. Scrape targets that use pod discovery (e.g. `dcgm_exporter`) can default to `true` because if pods aren't running they won't be discovered — but ask for explicit justification (PR #1452, #1391, #976)
- **CCP vs addon parity** — changes to addon chart (`deploy/addon-chart/azure-monitor-metrics-addon/`) may need corresponding CCP chart changes (`deploy/addon-chart/ccp-metrics-plugin/`). Ask "does this apply to CCP?" if unclear (PR #1393, #1391)
- **relabel_configs vs metric_relabel_configs** — `relabel_configs` operates on target labels from service discovery (before scrape). `metric_relabel_configs` operates on metric labels (after scrape). Labels like `cluster`, `instance`, `gpu`, `job` are metric labels and MUST be in `metric_relabel_configs`. Past reviews caught labelkeep actions in the wrong section (PR #1417)
- **Divide-by-zero protection** — when using values from external libraries (e.g. `gopsutil`), guard against zero returns that could cause out-of-bounds errors (PR #1030)
- **String vs bool configmap parsing** — Go configmap parsing treats unquoted `true`/`false` in YAML differently from quoted `"true"`/`"false"`. When changing configmap parsing, verify both quoted and unquoted values work correctly (PR #1133)
- **Schema version defaults** — `AZMON_AGENT_CFG_SCHEMA_VERSION` defaults to `v1` even when no configmap exists. Code that checks schema version should handle empty/invalid values gracefully (PR #1217)

### 2. Configmap & Scrape Target Changes

- **v1 + v2 configmap parity** — changes to `ama-metrics-settings-configmap.yaml` (v1) MUST also be made in `ama-metrics-settings-configmap-v1.yaml` and `ama-metrics-settings-configmap-v2.yaml`. Both schemas are actively supported (PR #1056, #1169, #1292)
- **Default scrape config requirements** — any new target in `otelcollector/configmapparser/default-prom-configs/` needs:
  - Entries in `tomlparser-*-default-targets-metrics-keep-list.go` for minimal ingestion profile
  - `metric_relabel_configs` with `labelkeep`/`labeldrop` to control cardinality
  - Documentation of which Grafana dashboards the metrics light up
  - Ingestion volume screenshots (MIP on vs MIP off) in the PR description
  - E2E test and TestKube workflow entry (PR #1393, #1224, #1391)
- **Scrape interval — use `$$SCRAPE_INTERVAL$$`** — new default targets should use the `$$SCRAPE_INTERVAL$$` placeholder (replaced at runtime) instead of hardcoded values like `10s`. The default is 30s, which balances data freshness with resource cost. Only cadvisor uses 15s. Reviewers flag any < 30s interval (PR #976)
- **Label cardinality** — check for high-cardinality labels (`pod`, `container_id`, `instance`) in new scrape configs without filtering. These cause customer AMW cost spikes. Verify `labeldrop`/`labelkeep` is present. If a label was previously dropped (e.g. `cluster`), adding it back needs explicit justification (PR #1417)
- **Histogram metric awareness** — histogram metrics produce `_sum`, `_count`, and `_bucket` series. When adding histogram metrics to keep lists, account for all three suffixes in cardinality estimates (PR #1224)
- **Namespace filtering for pod discoveries** — if a target runs in a specific namespace, filter service discovery to that namespace only to reduce unnecessary discovery overhead (PR #976)
- **Document metrics collected** — PR description should list what metrics are collected by default, with their labels, when enabling a new default target (PR #1391)
- **Preserve aka.ms links** — existing aka.ms links pointing to configmap files must not be broken by renames. Check that linked files still exist at the same paths (PR #1056)

### 3. Telemetry & Observability

- **Add telemetry for new features** — new configmap settings need corresponding telemetry:
  - Set environment variables for enabled/disabled state in Go configmap parsing code
  - Emit values via `process_stats.go` / `telemetry.go` in fluent-bit
  - Add scrape intervals, keep list regex, and enabled/disabled state to telemetry
  - Follow patterns from existing targets like ztunnel/waypointProxy (PR #976, #1003, #1292, #1320, #1391)
- **v1/v2 schema tracking** — add a telemetry dimension to track which configmap schema version customers use (PR #1056)
- **Configmap presence telemetry** — track whether settings configmaps are deployed at all (not just what's in them), to understand adoption (PR #1217)
- **Monitoring alerts** — major new features should include alert rules or Grafana dashboard updates so the team can monitor adoption and issues (PR #1133)

### 4. Build, Docker & Multi-Arch

- **Pin versions in setup scripts** — `otelcollector/scripts/setup.sh` and `otelcollector/scripts/ccpsetup.sh` must pin dependency versions explicitly. Unpinned versions cause non-reproducible builds (PR #1014)
- **Update ALL Dockerfiles** — when changing base images, build tools, or runtime dependencies, update ALL Dockerfiles: main Linux, configuration-reader, target-allocator, AND Windows. Past reviews found Dockerfiles missed during upgrades (PR #1014, #1232)
- **Dockerfile consistency** — when fixing a build flag or dependency in one Dockerfile, check if other Dockerfiles (e.g. `configuration-reader/Dockerfile`) need the same fix. Use `grep` across all Dockerfiles to find inconsistencies (PR #1232)
- **Runtime .so verification** — after build changes, verify that shared libraries (`.so` files) needed at runtime are actually present in the final container image. Build-time dependencies don't always carry to runtime (PR #1014, #1397)
- **Multi-arch manifest issues** — on ARM64 hosts, `docker manifest inspect` may return architecture-specific results instead of the manifest list. Be aware of this when verifying multi-arch image builds (PR #1390)
- **Don't disable security features** — Docker provenance (`--provenance=false`), attestation, or other security features should never be disabled without explicit team discussion and documented justification (PR #1390)
- **Fluent-bit build flags → dalec parity** — Dockerfile changes to fluent-bit compilation must match [dalec-build-defs](https://github.com/Azure/dalec-build-defs). When `DFLB_*` flags change, library dependencies may need updating (PR #1397)
- **Clean up build artifacts** — remove commented-out code, debug commands (`cat`, `echo` of payloads), and temporary workarounds before merging. "We have git history" is the accepted response when authors want to keep commented code (PR #1030, #1407, #976, #1169)

### 5. Testing & Pipeline

- **Test labels must match pipeline tasks** — test labels in Ginkgo test files must match the task labels in `azure-pipeline-config-tests.yml`. Mismatched labels mean tests won't run in CI even though they pass locally (PR #1305)
- **TestKube workflow entries** — new scrape targets need a corresponding entry in `run-testkube-workflow.sh` and appropriate TestKube test CR files. Don't just add the config — add the test (PR #1393)
- **Clean up configmaps between test runs** — config processing tests must clean up (delete) test configmaps before applying new ones. Leftover configmaps from previous runs cause false positives (PR #1305)
- **Verify MIP actually blocks metrics** — when testing Minimal Ingestion Profile, verify that metrics are actually dropped (not just that no errors are logged). Check metric counts before and after (PR #1305)
- **Prometheus version in tests** — when upgrading Prometheus, update the version in E2E test files too. Tests may pass with old versions because they use different `config.Load()` signatures (PR #1115)
- **Install dependencies once, not per-test** — when TestKube or other dependencies need installing, do it once at pipeline start, not inside each test script (PR #1392)
- **Test with overridden branches** — verify that TestKube test CRs can run from non-main branches (the branch parameter must be configurable) (PR #1392)

### 6. Helm Chart & Deployment

- **Arc vs AKS conditional logic** — Helm template conditionals for Arc clusters have different requirements than AKS. When removing/changing tolerations, affinity, or RBAC, verify the change doesn't break Arc deployments (PR #1298)
- **Helm template helpers** — AKS RP maintains shared Helm helpers (e.g. `should_mount_hostca`). When adding volume mounts or security contexts, check if an existing helper should be used (PR #1408)
- **Don't add unused ports** — don't expose gRPC or other ports in pod specs until the feature is fully supported and tested. Unused ports may conflict with other services (PR #1298)
- **Extension migration coordination** — the codebase is migrating from addon to AKS extension. Helm values and templates need to be coordinated with the extension team. Flag changes that touch shared extension boundaries (PR #1408)
- **Chart.yaml / values.yaml reverts** — don't commit changes to `Chart.yaml` or `values.yaml` version bumps if they're not part of the PR's intent. These files are auto-managed by the release process (PR #1417, #653)
- **Values-template.yaml vs inline** — prefer adding configurable values in `values-template.yaml` over hardcoding in Helm templates. This allows extension-level overrides (PR #1292)

### 7. Naming & Documentation

- **User-friendly scrape target names** — scrape target names visible in configmaps should be descriptive and unambiguous. Avoid names that could be confused with other concepts (e.g. "control plane" in data-plane context) (PR #1056)
- **Naming prefix consistency** — new scrape targets should follow established naming conventions (e.g. `acstor` prefix for all Azure Container Storage targets). Don't mix naming styles within a feature area (PR #1224)
- **RELEASENOTES.md** — any user-facing change needs a release note entry. Image tag bumps, ME version changes, and behavioral changes must be documented
- **PR title — conventional commits** — `feat:`, `fix:`, `docs:`, `chore:`, `build:`, etc.
- **Metrics documentation** — PR description for new default targets should list all collected metrics and their labels. Link to upstream exporters when applicable (PR #1391)
- **External docs links** — when adding configmap options for users, consider linking to upstream documentation (e.g. kube-state-metrics docs for KSM resources) (PR #1292)

### 8. Control Plane (CCP) Specific

- **CCP scrape configs** — CCP targets use different keep lists (`tomlparser-ccp-default-targets-metrics-keep-list.go`) from addon targets. Ensure new CCP targets have appropriate entries (PR #1169, #1320)
- **Variable placement for CCP telemetry** — CCP telemetry vars must be added to the correct section in `process_stats.go` for proper emission (PR #1320)
- **CCP configmap naming** — CCP configmaps use separate naming from addon. Don't reuse addon configmap names for CCP entries
- **Verify metrics against actual exports** — for CCP targets, verify metric names against the actual running component (e.g. self-hosted karpenter), not just documentation. Metric names may differ from docs (PR #1169)
- **rest_client_requests_total** — not all control plane components expose this metric. Don't include it in keep lists unless the component actually emits it (PR #1169)

## File-Specific Quick Reference

| File/Path Pattern | What to Check |
|---|---|
| `process_utilities_*.go` | ME CLI args have leading dash, flag names correct |
| `build/linux/Dockerfile` | dalec parity, deps needed at runtime, env vars used |
| `build/linux/configuration-reader/Dockerfile` | Same fixes as main Dockerfile |
| `values-template.yaml` | Defaults safe (features off), extension team aware |
| `templates/ama-metrics-*.yaml` | CCP parity, Arc conditionals, Helm helpers used |
| `default-prom-configs/*.yml` | Keep list updated, `$$SCRAPE_INTERVAL$$`, cardinality controlled, relabel in correct section |
| `tomlparser-default-scrape-settings.go` | Defaults match configmap defaults (false for new features) |
| `tomlparser-*-keep-list.go` | v1+v2 parity, CCP list too, regex correct |
| `tomlparser-scrape-interval.go` | Interval + regex overrides added for new targets |
| `configmaps/ama-metrics-settings-configmap*.yaml` | v1 + v2 both updated, doc links present |
| `fluent-bit/src/process_stats.go` / `telemetry.go` | New env vars emitted for telemetry |
| `scripts/setup.sh` / `scripts/ccpsetup.sh` | Versions pinned, deps installed |
| `prometheus-config-merger.go` | New default file arrays updated (both RS and DS arrays) |
| `test/ginkgo-e2e/` | Test labels match pipeline, version strings updated |
| `test/testkube/` | TestKube CRs + workflow entries for new tests |
| `.pipelines/azure-pipeline-*.yml` | Test labels, multi-arch, clean up debug code |
| `RELEASENOTES.md` | Image tag, changes documented |
| `tools/prom-collector-tsg-mcp/` | `dist/` rebuilt if `src/` changed |

## PR Template Checklist Verification

Every PR should be checked against the [PR template](/.github/pull_request_template.md). When reviewing, verify:

### New Feature Checklist (for feature PRs)
- [ ] **Telemetry listed** — PR description should list what telemetry was added (env vars, AppInsights events, process_stats dimensions)
- [ ] **One-pager linked** — significant features should link to a design one-pager or spec
- [ ] **Release tasks listed** — 3P docs updates, AKS RP chart changes, extension team coordination, customer-facing docs
- [ ] **Scale/perf results attached** — performance testing results (ingestion volume, resource usage) should be in the PR description or linked

### Tests Checklist (for code changes)
- [ ] **Ginkgo E2E tests run** — author should confirm which labels were tested:
  - `operator`, `windows`, `arm64`, `arc-extension`, `fips`
  - Not all labels apply to every PR — but the relevant ones should be checked
- [ ] **New tests added** — features need feature tests, fixes need regression tests
- [ ] **New scrape job** → added to `otelcollector/test/test-cluster-yamls/` in correct configmap or as CR
- [ ] **New test label** → all four places updated:
  1. String constant in `otelcollector/test/utils/constants.go`
  2. Label + description in `otelcollector/test/README.md`
  3. Added to the PR checklist template itself (`.github/pull_request_template.md`)
  4. Added to `otelcollector/test/testkube/testkube-test-crs.yaml`
- [ ] **API permissions** — new tests needing API server access must update `otelcollector/test/testkube/api-server-permissions.yaml`
- [ ] **New test suite** (new folder under `/tests`) → included in `testkube-test-crs.yaml`

### PR Metadata
- [ ] **Title follows conventional commits** — `feat:`, `fix:`, `docs:`, `chore:`, `build:`, `ci:`, `refactor:`, `test:`
- [ ] **Description is substantive** — not just the template with empty checkboxes. For new default targets, the description should include collected metrics, labels, and ingestion volume screenshots

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

**Important: Do NOT attribute conventions to specific people** (e.g. "per Rashmi's feedback" or "rashmichandrashekar always asks for…"). Present findings as team conventions or repo standards. Reference PR numbers for traceability if needed, but never names.

## Team Context

- **Review style**: The team asks "why?" questions to understand intent, requests test evidence, asks for telemetry additions, and flags coordination needs with other teams (extension, ME, AKS RP, CCP). Non-blocking suggestions are often prefixed with "nit:" or "Non-blocking, but…"
- **Common review patterns**:
  - Nearly every new feature gets a telemetry request
  - Files changed accidentally (Chart.yaml, values.yaml, pipeline files) should be reverted
  - Addon changes are checked for CCP equivalents
  - No commented-out code in merge — "we have git history"
  - Manual testing evidence is expected for significant changes
  - Reviewers link to existing code patterns as examples (e.g. "similar to ztunnel target")
