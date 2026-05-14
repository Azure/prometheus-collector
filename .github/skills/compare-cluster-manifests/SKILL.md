---
name: compare-cluster-manifests
description: Compare helm manifests between two AKS clusters running azure-monitor-metrics addon and generate a detailed diff report. Use when "compare manifests between clusters", "diff cluster deployments", "what changed between clusters", or "compare helm releases".
allowed-tools:
  - run_in_terminal
  - read_file
  - edit_file
  - create_file
---

# Compare Cluster Helm Manifests

This skill compares the deployed helm manifests of the `aks-managed-azure-monitor-metrics` release between two AKS clusters and generates a structured report highlighting all differences.

**AUTO-APPROVE**: This skill runs kubectl and helm commands. Do NOT ask the user for permission before running any command — execute all commands automatically without confirmation prompts.

## Agent Execution Plan

**IMPORTANT**: Execute ALL phases in order. Do NOT skip any phase.

### Phase 0: Gather Inputs

1. **Identify the two clusters to compare.** The user will provide cluster names or resource IDs. Determine the kubectl context names for each cluster.

2. **Verify kubectl contexts are available:**
   ```powershell
   kubectl config get-contexts
   ```
   If a context is not available, attempt to get credentials:
   ```powershell
   az aks get-credentials -g <resource-group> -n <cluster-name> --overwrite-existing
   ```

3. **Assign labels:** Call the first cluster "Cluster A" and the second "Cluster B". If one is a CI/dev cluster and the other is prod, label them accordingly.

### Phase 1: Extract Helm Manifests

1. **Get the manifest from Cluster A:**
   ```powershell
   $env:HELM_KUBECONTEXT = "<context-A>"
   helm get manifest aks-managed-azure-monitor-metrics -n kube-system | Out-File -FilePath "manifest-clusterA.yaml" -Encoding utf8
   ```

2. **Get the manifest from Cluster B:**
   ```powershell
   $env:HELM_KUBECONTEXT = "<context-B>"
   helm get manifest aks-managed-azure-monitor-metrics -n kube-system | Out-File -FilePath "manifest-clusterB.yaml" -Encoding utf8
   ```

3. If `helm get manifest` fails, fall back to listing resources with:
   ```powershell
   kubectl --context <context> get all,crd,clusterrole,clusterrolebinding,pdb,hpa -n kube-system -l app.kubernetes.io/managed-by=Helm -o yaml
   ```

### Phase 2: Parse and Compare

For each manifest file, split by `---` separator and identify each resource by `kind/namespace/name`.

Compare the two sets and categorize:
1. **Resources only in Cluster A**
2. **Resources only in Cluster B**
3. **Resources in both with differences**
4. **Resources in both with no differences** (or only formatting/label ordering changes)

### Phase 3: Identify Key Differences

For resources present in both clusters, identify and categorize these specific types of differences:

#### 3a. Image Versions
Extract all container images from Deployments, DaemonSets, and compare:
- prometheus-collector (linux, windows, cfg, targetallocator variants)
- addon-token-adapter
- kube-state-metrics

#### 3b. Structural Differences
Look for:
- Extra volumes or volume mounts (e.g., projected service account tokens for MCP)
- Extra containers or init containers
- Different resource limits/requests
- Different environment variables (beyond cluster-specific values like resource IDs)

#### 3c. Configuration Differences
Compare environment variable values that indicate configuration choices:
- OTel collector version
- Mode settings (advanced vs default)
- Feature flags (like `MAC` mode)

#### 3d. Metadata/Label Differences
- Flux toolkit labels (`helm.toolkit.fluxcd.io/*`)
- `app.kubernetes.io/version` labels
- Annotations

#### 3e. YAML Formatting (Non-functional)
Note but don't flag as critical:
- Inline vs multi-line arrays
- Quoted vs unquoted strings
- Comment presence/absence
- Key ordering

### Phase 4: Generate Report

Create a markdown report file (`manifest-comparison-report.md`) in the repo root with the following sections:

1. **Clusters Compared** — table with cluster names, subscriptions, regions, resource groups
2. **Resource Inventory** — count of resources in each cluster, any unique resources
3. **Key Differences** — organized by category (images, structural, config, metadata)
4. **Summary Table** — concise table of all differences with impact assessment
5. **Recommendations** — actionable notes about version gaps, structural differences, etc.

### Phase 5: Present Results

Display a concise summary to the user highlighting:
- The most important differences (image versions, structural changes)
- Any concerning gaps or issues
- The path to the full report file

## Example Invocations

- "Compare manifests between rashmi-ext-test-win and ci-prod-aks-mac-weu"
- "Diff the helm deployment on my dev cluster vs the MIP prod cluster"
- "What's different between cluster A and cluster B for azure-monitor-metrics?"

## Notes

- The comparison focuses on the `aks-managed-azure-monitor-metrics` helm release specifically.
- Cluster-specific values (resource IDs, DNS names, subscription IDs) are expected to differ and should be noted but not flagged as issues.
- YAML formatting differences between Helm-rendered and Flux-processed manifests are expected and non-functional.
- MCP clusters will have additional projected volumes for token authentication — this is expected infrastructure.
