---
name: ccp-deployto-standalone
description: Build a prometheus-collector CCP test image, deploy it to an AKS standalone, and validate metric scraping and ingestion into Azure Monitor.
allowed-tools:
  - Bash
  - Read
  - Write
  - Grep
  - Glob
---

# Prom-Collector Buddy-Build Standalone

## Overview

This skill verifies that a new CCP component's managed Prometheus addon functions correctly by deploying a **prometheus-collector** test image into an AKS standalone environment. It automates the full end-to-end flow:

1. Building a CI/dev CCP image from a prometheus-collector branch.
2. Creating a standalone environment and test cluster.
3. Patching the `ama-metrics-ccp` deployment with the test image.
4. Validating metric scraping and ingestion into Azure Monitor.

### When to Use

- You have a prometheus-collector PR/branch with CCP changes (new scrape targets, config changes, etc.).
- You need to validate CCP metrics ingestion end-to-end before merging.
- You want to test in an isolated standalone environment rather than staging.

---

## Repository Setup

This skill requires the **prometheus-collector** repo. Ask the user for their local checkout path.

If they don't have it:
```bash
git clone https://github.com/Azure/prometheus-collector.git <destination>
```

The standalone creation steps reference the **aks-rp** repo for `aksdev` binary and `azureconfig.yaml`. Ask the user for their local aks-rp checkout path.

If they don't have it:
```bash
git clone https://msazure.visualstudio.com/DefaultCollection/CloudNativeCompute/_git/aks-rp <destination>
```

## Prerequisites

- Azure CLI (`az`) logged in with ADO permissions
- Azure DevOps PAT or `az devops` authentication
- VPN connection to `MSFT-AzVPN-Manual` (for aksdev operations)
- `aksdev` binary (built or downloaded via the create-standalone skill)
- `kubectl` installed

---

## Inputs

| Input | Description | Example |
|-------|-------------|---------|
| `PROM_COLLECTOR_BRANCH` | The prometheus-collector branch to test | `user/my-ccp-feature` |
| `USER_ALIAS` | Your Microsoft alias | `dakydd` |
| `STANDALONE_NAME` | Name of an existing standalone (if reusing) | `standalone-260216bm47nl` |

---

## Helper Script

The skill includes a helper script at `tools/check_build.py` that checks a prometheus-collector pipeline build for the CCP ORAS push stage status and extracts the image tag.

Usage:
```bash
python3 tools/check_build.py <build-id>
```

---

## Steps

### Step 1: Build a Test Image from Your Branch

Build the CCP image using the [Azure/prometheus-collector pipeline](https://github-private.visualstudio.com/azure/_build?definitionId=440):

1. Navigate to the pipeline and click **Run pipeline**.
2. Select the branch: `$PROM_COLLECTOR_BRANCH`.
3. Run the build and wait for the `ORAS Push Artifacts in /mnt/vss/_work/1/a/linuxccp/` stage inside `Build: linux CCP prometheus-collector image` to succeed.

> **Note:** The pipeline can show as failed overall. You can continue as long as the ORAS push stage succeeded.

**One-line check command** (expects `succeeded`):
```bash
az devops invoke --organization https://github-private.visualstudio.com --area build --resource timeline \
  --route-parameters project=azure buildId=<build-id> --api-version 7.1 \
  --query "records[?name=='ORAS Push Artifacts in /mnt/vss/_work/1/a/linuxccp/'] | [0].result" -o tsv
```

4. Extract the image tag from the completed build:

```
mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:<TAG>-ccp
```

> **Important:** Use `cidev` (not `ciprod`), and the tag must end with `-ccp`.

```bash
export CCP_IMAGE_TAG="<paste-the-tag-here>"
export TEST_IMAGE="mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:${CCP_IMAGE_TAG}"
echo "CCP test image: $TEST_IMAGE"
```

### Step 2: Set Up Your Standalone Environment

Create a standalone cluster (follow the create-standalone skill if needed), then download `azureconfig.yaml` and build or download the `aksdev` binary.

### Step 3: Enable AMA Metrics Addon on the cx-1 Underlay

The standalone's cx-1 underlay is a real AKS cluster, which we leverage to get an MSI token for metric ingestion (see [Why cx-1](#why-do-we-have-to-manually-update-the-cx-1-underlay)).

1. Set environment variables using values from the Portal:

```bash
export AKS_RESOURCE_ID=/subscriptions/<subscription-id>/resourcegroups/<standalone-resource-group>/providers/Microsoft.ContainerService/managedClusters/<standalone-resource-group>-cx-1
export AKS_CLUSTER_NAME=<standalone-resource-group>-cx-1
export RESOURCE_GROUP=<standalone-resource-group>
export SUBSCRIPTION_ID=<subscription-id>
```

2. Enable AMA Metrics addon:

```bash
az account set -s $SUBSCRIPTION_ID
az aks update --enable-azure-monitor-metrics --enable-control-plane-metrics -n $AKS_CLUSTER_NAME -g $RESOURCE_GROUP
# Note: --enable-control-plane-metrics requires the aks-preview CLI extension
az aks get-credentials -n $AKS_CLUSTER_NAME -g $RESOURCE_GROUP -f $AKS_CLUSTER_NAME.kubeconfig
```

### Step 4: Create a Test Cluster with the Addon Enabled

1. Set environment variables:

```bash
export USER_ALIAS=<your-alias>
export WORKFLOW_NAME=buddybuild-standalone
export CX_CLUSTER_NAME=$USER_ALIAS-$WORKFLOW_NAME
export MC_SUB=82acd5bb-4206-47d4-9c12-a65db028483d
export LOCATION=<standalone-location>  # Must match standalone location
```

2. Create the cluster:

```bash
./bin/aksdev cluster create $CX_CLUSTER_NAME --location $LOCATION \
  --managedclustersubscription $MC_SUB --enableManagedIdentity \
  --enable-azure-monitor-metrics \
  --subscription-features AzureMonitorMetricsControlPlanePreview \
  --node-provisioning-mode Auto

./bin/aksdev cluster kubeconfig $CX_CLUSTER_NAME --managedclustersubscription $MC_SUB > $CX_CLUSTER_NAME.kubeconfig
```

### Step 5: Scale Down Reconcilers in the cx-1 Underlay

Scale all reconciler deployments to 0 replicas to prevent them from reverting the deployment patches:

```bash
kubectl scale deploy addonconfigreconciler -n addonconfigreconciler --replicas=0 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
kubectl scale deploy overlaymgr-overlaymanager overlaymgr-overlaymanager-loop -n overlaymgr --replicas=0 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
kubectl scale deploy eno-reconciler -n eno-system --replicas=0 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
```

> **Note:** All four must be scaled down. The `eno-reconciler` manages underlay deployment specs and will scale the other reconcilers back to 1 if left running.

### Step 6: Identify and Annotate the CCP Namespace

1. Find the CCP namespace (starts with `6`):

```bash
kubectl get ns --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
export CCP_NS=<ccp-namespace-id>
```

2. Skip CCP reconciliation for this namespace:

```bash
export SKIP_CCP_RECONCILE_UNTIL=$(date -u -v+7d '+%Y-%m-%dT%H:%M:%SZ')
kubectl annotate namespace $CCP_NS skip-ccp-reconcile-until-this-time="$SKIP_CCP_RECONCILE_UNTIL" --overwrite --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
```

### Step 7: Patch the ama-metrics-ccp Deployment

Make three updates to the `ama-metrics-ccp` deployment:

> **Note:** `kubectl set env` does **not** work for env vars that use `valueFrom` (e.g., `fieldRef`). Use `kubectl patch` with strategic merge and `"valueFrom": null`.

#### 7a. Set the CLUSTER env var to the cx-1 underlay resource ID

```bash
kubectl patch deployment ama-metrics-ccp -n $CCP_NS --type=strategic --kubeconfig $AKS_CLUSTER_NAME.kubeconfig \
  -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"prometheus-collector\",\"env\":[{\"name\":\"CLUSTER\",\"value\":\"$AKS_RESOURCE_ID\",\"valueFrom\":null}]}]}}}}"
```

#### 7b. Replace msi-adapter with addon-token-adapter (if needed)

Check which container is present:

```bash
kubectl get deploy ama-metrics-ccp -n $CCP_NS -o jsonpath='{.spec.template.spec.containers[*].name}' --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
```

If it has `msi-adapter`, export the deployment, replace the container block, and apply:

```bash
kubectl get deployment ama-metrics-ccp -n $CCP_NS -o yaml --kubeconfig $AKS_CLUSTER_NAME.kubeconfig > /tmp/ama-metrics-ccp.yaml
# Edit /tmp/ama-metrics-ccp.yaml — replace the msi-adapter container with addon-token-adapter
kubectl apply -f /tmp/ama-metrics-ccp.yaml --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
```

The `addon-token-adapter` container block to use:

```yaml
- name: addon-token-adapter
  command:
    - /addon-token-adapter
  args:
    - --secret-namespace=kube-system
    - --secret-name=aad-msi-auth-token
    - --token-server-listening-port=7777
    - --health-server-listening-port=9999
    - --restart-pod-waiting-minutes-on-broken-connection=240
  image: mcr.microsoft.com/aks/msi/addon-token-adapter:master.251201.2
  imagePullPolicy: IfNotPresent
  env:
    - name: AZMON_COLLECT_ENV
      value: "false"
  livenessProbe:
    httpGet:
      path: /healthz
      port: 9999
    initialDelaySeconds: 10
    periodSeconds: 60
  resources:
    limits:
      cpu: 500m
      memory: 500Mi
    requests:
      cpu: 20m
      memory: 30Mi
  securityContext:
    capabilities:
      drop:
        - ALL
      add:
        - NET_ADMIN
        - NET_RAW
```

#### 7c. Update the prometheus-collector image to your test image

```bash
kubectl set image deployment/ama-metrics-ccp -n $CCP_NS prometheus-collector=$TEST_IMAGE --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
```

---

## Validation

### V1. Enable the Feature in ConfigMap

Set your CCP component's setting to true in the ConfigMap. For example, for NAP:
```yaml
controlplane-node-auto-provisioning: true
```

### V2. Verify Propagation to Agent

```bash
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --kubeconfig $AKS_CLUSTER_NAME.kubeconfig | tail -50
```

### V3. Check Azure Monitor Workspace Ingestion

Verify that metrics are appearing in the connected Azure Monitor workspace.

### V4. Validate Minimal Ingestion Profile (Default Behavior)

With `minimalingestionprofile` enabled (default), confirm only the metrics in your minimal list are ingested.

### V5. Test KeepList Override Behavior

Add an additional metric to the `keeplist` in the ConfigMap and confirm ingestion.

---

## Switching Between Overlay and Underlay Clusters

```bash
export KUBECONFIG=$CX_CLUSTER_NAME.kubeconfig           # overlay
export KUBECONFIG=$AKS_CLUSTER_NAME.kubeconfig           # underlay
```

---

## Troubleshooting

1. **Check the path + port** the target pod exposes metrics on.
2. **Check TLS requirements** — is the target pod accessible via localhost or TLS?
3. **MinMac regex syntax** — use single parentheses for metrics, e.g., `karpenter_(nodes_created_total|nodes_terminated_total)`.
4. **Compare multiple existing configs** from [default-prom-configs](https://github.com/Azure/prometheus-collector/tree/main/otelcollector/configmapparser/default-prom-configs).
5. **Reconcilers reverting changes** — verify all four reconcilers from Step 5 are scaled to 0.

---

## Cleanup

1. Delete the test cluster:
   ```bash
   ./bin/aksdev cluster delete $CX_CLUSTER_NAME --managedclustersubscription $MC_SUB
   ```

2. Scale reconcilers back up (if the standalone is still needed):
   ```bash
   kubectl scale deploy addonconfigreconciler -n addonconfigreconciler --replicas=1 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
   kubectl scale deploy overlaymgr-overlaymanager overlaymgr-overlaymanager-loop -n overlaymgr --replicas=1 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
   kubectl scale deploy eno-reconciler -n eno-system --replicas=1 --kubeconfig $AKS_CLUSTER_NAME.kubeconfig
   ```

3. Or delete the standalone entirely (auto-deletes after 3 days).

---

## Why do we have to manually update the cx-1 underlay?

In standalone, we use a trick to get the `ama-metrics-ccp` pod an MSI token for ingestion to a real Azure Monitor workspace. The "cluster" created via `aksdev` isn't a real AKS cluster — it only exists within the standalone. There's no MSI token available for it.

However, the standalone underlay itself (the cx-1 cluster) **is** a real AKS cluster. We enable the AMA Metrics addon on cx-1, which gives it permission to ingest to an Azure Monitor workspace. We then configure the `ama-metrics-ccp` pod to use the cx-1 cluster's resource ID via the `CLUSTER` env var, which allows the `addon-token-adapter` to obtain the correct MSI token.

---

## Related Reference

| Document | Location |
|----------|----------|
| Enabling Managed Prometheus for CCP | [ADO Wiki](https://msazure.visualstudio.com/CloudNativeCompute/_wiki/wikis/CloudNativeCompute.wiki) |
| Minimal Prometheus ingestion profile | [Microsoft Learn](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration-minimal) |
| Azure Monitor Metrics enable guide | [Microsoft Learn](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable?tabs=cli) |
| Skip CCP Reconcile | [ADO Wiki](https://msazure.visualstudio.com/CloudNativeCompute/_wiki/wikis/aks-troubleshooting-guide/506628/skip-ccp-reconcile) |
