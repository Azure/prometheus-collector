---
name: ccp-update-prodimage
description: Bump the CCP prometheus-collector image tag across toggle file, featureflag readers, and regenerate helm fixtures in aks-rp. Use when "update promcollector image tag", "bump prometheus-collector CCP tag", or "ccp update-prodimage".
allowed-tools:
  - run_in_terminal
  - read_file
---

# Bump CCP Prometheus-Collector Image Tag

Update the CCP prometheus-collector container image tag across all required files in `aks-rp`, then regenerate helm fixtures and chart snapshots.

## Required Inputs

| Input | Format | Example |
|-------|--------|---------|
| New image tag | `<version>-main-<MM-DD-YYYY>-<build-id>-ccp` | `6.24.1-main-11-14-2025-15146744-ccp` |

## Files That Change

1. **Toggle file** (default value): `toggles/global/sigs/containerinsights/ama-metrics-ccp-promcollector-imagetag.yaml`
2. **Primary featureflag reader**: `ccp/control-plane-core/helmvalues/featureflag/ccp_plugins_reader.go`
3. **Synced copy (core-addon-synth)**: `ccp/core-addon-synth/helmvalues/featureflag/ccp_plugins_reader.go`
4. **Synced copy (overlaymgr)**: `overlaymgr/server/helmvalues/featureflag/ccp_plugins_reader.go`

> **Note**: Files 3 and 4 are auto-synced copies of file 2. They carry a header comment saying not to edit directly, but we update all four in one shot to keep the branch green locally. CI sync will produce a no-op diff.

---

## Execution Stages

### Stage 0: Validate Inputs

1. Confirm the new image tag matches the expected format.
2. Identify the current image tag:
   ```bash
   grep 'defaultValue' toggles/global/sigs/containerinsights/ama-metrics-ccp-promcollector-imagetag.yaml
   ```

---

### Stage 1: Update Toggle File

**File**: `toggles/global/sigs/containerinsights/ama-metrics-ccp-promcollector-imagetag.yaml`

Replace the `defaultValue` with the new tag:

```yaml
defaultValue: "<NEW_TAG>"
```

Only the `defaultValue` line changes. Leave all rules/matchers untouched.

---

### Stage 2: Update Featureflag Readers

Update the default value in the `AzureMonitorMetricsCCPPromCollectorImageTag` function across all three copies of `ccp_plugins_reader.go`.

#### 2a. Primary (source of truth)

**File**: `ccp/control-plane-core/helmvalues/featureflag/ccp_plugins_reader.go`

In the function `AzureMonitorMetricsCCPPromCollectorImageTag`, replace the old tag string with the new tag in the `getStringWithContext` call:

```go
return t.getStringWithContext(ctx, "ama-metrics-ccp-promcollector-imagetag", t.newEntity(e), "<NEW_TAG>")
```

#### 2b. Synced copy — core-addon-synth

**File**: `ccp/core-addon-synth/helmvalues/featureflag/ccp_plugins_reader.go`

Same edit as 2a.

#### 2c. Synced copy — overlaymgr

**File**: `overlaymgr/server/helmvalues/featureflag/ccp_plugins_reader.go`

Same edit as 2a.

---

### Stage 3: Regenerate Fixtures and Snapshots

Run from the aks-rp root (`${workspaceFolder:aks-rp}`):

```bash
make generate-helm-fixtures
make render-ccp-plugin-adapter-chart-snapshots
make render-ccp-plugin-chart-snapshots
```

These regenerate golden files that embed the image tag. Without this step, CI snapshot tests will fail.

---

### Stage 4: Verify Changes

1. Run `git diff` to confirm only the expected files changed:
   - The 4 source files from Stages 1–2
   - Generated fixture/snapshot files from Stage 3
2. Verify no unrelated changes crept in.

---

## Quick Reference

| What | Where |
|------|-------|
| Toggle YAML | `toggles/global/sigs/containerinsights/ama-metrics-ccp-promcollector-imagetag.yaml` |
| Primary reader | `ccp/control-plane-core/helmvalues/featureflag/ccp_plugins_reader.go` |
| Synced reader (synth) | `ccp/core-addon-synth/helmvalues/featureflag/ccp_plugins_reader.go` |
| Synced reader (overlaymgr) | `overlaymgr/server/helmvalues/featureflag/ccp_plugins_reader.go` |
| Make targets | `generate-helm-fixtures`, `render-ccp-plugin-adapter-chart-snapshots`, `render-ccp-plugin-chart-snapshots` |
