# CCP Health Metrics Validation Workflow

## Overview

This workflow validates that the `ama-metrics-ccp` deployment correctly exposes health metrics when running in CCP (Control-Plane) mode. It automates the full end-to-end flow:

1. Building a CI/dev CCP image from a prometheus-collector branch.
2. Creating (or reusing) a standalone environment and test cluster.
3. Patching the `ama-metrics-ccp` deployment with the test image.
4. Validating health metric scraping and ingestion via port-forward.

### When to Use

- You have a prometheus-collector PR/branch with CCP health metric changes.
- You need to validate CCP health metrics end-to-end before merging.
- You want to test in an isolated standalone environment.

### How to Run

This workflow is designed to be **run interactively in a VS Code chat session** with a recent model (Claude Opus 4.6 recommended). Copy or reference this document in the chat and the agent will walk you through each step, running commands on your behalf.

> **Why interactive?** CCP pods live on standalone underlay clusters, not CI clusters. The ginkgo-e2e/TestKube pipeline cannot reach them. This workflow uses the standalone environment directly via kubeconfig and port-forward.

---

## Prerequisites

- Azure CLI (`az`) logged in with ADO permissions
- Azure DevOps PAT or `az devops` authentication
- VPN connection to `MSFT-AzVPN-Manual` (for aksdev operations)
- `kubectl` installed
- `jq` and `unzip` installed
- The `aks-rp` repo checked out (for `aksdev` binary)

---

## Inputs

| Input | Description | Example |
|-------|-------------|---------|
| `PROM_COLLECTOR_BRANCH` | The prometheus-collector branch to test | `user/my-ccp-feature` |
| `USER_ALIAS` | Your Microsoft alias | `dakydd` |
| `STANDALONE_BUILD_ID` | ADO Build ID of an existing standalone (if reusing) | `146391809` |

---

## Part 1: Create or Reuse a Standalone Environment

### Step 1.1: Check for an Existing Standalone

Before creating a new standalone, check if one already exists with sufficient TTL (>1 day):

```bash
# Configure ADO defaults
az devops configure --defaults organization=https://dev.azure.com/msazure project=CloudNativeCompute

# Check recent Dev AKS Deploy pipeline runs
CURRENT_USER=$(az account show --query user.name -o tsv)
az pipelines runs list --pipeline-id 68881 --top 50 -o json | \
  jq -r --arg user "$CURRENT_USER" '.[] | select(.requestedBy.uniqueName == $user and .result == "succeeded") | "\(.id) | \(.queueTime) | \(.result)"'
```

### Step 1.2: Create a New Standalone (if needed)

If no suitable standalone exists, trigger the [Dev AKS Deploy pipeline](https://dev.azure.com/msazure/CloudNativeCompute/_build?definitionId=68881&_a=summary):

```bash
PIPELINE_RESULT=$(az pipelines run --id 68881 --branch master -o json)
BUILD_ID=$(echo "$PIPELINE_RESULT" | jq -r '.id')
echo "Build ID: $BUILD_ID"
```

> ⏱️ Wait ~20-30 minutes for completion.

### Step 1.3: Verify Pipeline Completion

```bash
ADO_TOKEN=$(az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv)

BUILD_STATUS=$(curl -s -H "Authorization: Bearer ${ADO_TOKEN}" \
  "https://dev.azure.com/msazure/CloudNativeCompute/_apis/build/builds/${BUILD_ID}?api-version=7.0" \
  | jq -r '.status')

echo "Build status: $BUILD_STATUS"
# Must be "completed" before proceeding
```

### Step 1.4: Download Artifacts and Configure kubectl

```bash
AKS_RP_DIR=~/code/go/src/go.goms.io/aks-rp
cd $AKS_RP_DIR

ADO_TOKEN=$(az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798 --query accessToken -o tsv)

# Download azureconfig.yaml
AZURECONFIG_URL=$(curl -s -H "Authorization: Bearer ${ADO_TOKEN}" \
  "https://dev.azure.com/msazure/CloudNativeCompute/_apis/build/builds/${BUILD_ID}/artifacts?api-version=7.0" \
  | jq -r '.value[] | select(.name == "azureconfig.yaml") | .resource.downloadUrl')
curl -sL -H "Authorization: Bearer ${ADO_TOKEN}" "${AZURECONFIG_URL}" -o /tmp/azureconfig_artifact.zip
rm -rf /tmp/azureconfig_extract && unzip -o /tmp/azureconfig_artifact.zip -d /tmp/azureconfig_extract
[ -f azureconfig.yaml ] && mv azureconfig.yaml azureconfig.yaml.bak
cp /tmp/azureconfig_extract/azureconfig.yaml/azureconfig.yaml ./azureconfig.yaml

# Download kubeconfigs
KUBECONFIG_URL=$(curl -s -H "Authorization: Bearer ${ADO_TOKEN}" \
  "https://dev.azure.com/msazure/CloudNativeCompute/_apis/build/builds/${BUILD_ID}/artifacts?api-version=7.0" \
  | jq -r '.value[] | select(.name == "e2e-underlay-kubeconfig") | .resource.downloadUrl')
curl -sL -H "Authorization: Bearer ${ADO_TOKEN}" "${KUBECONFIG_URL}" -o /tmp/kubeconfigs_artifact.zip
rm -rf /tmp/kubeconfigs_extract && unzip -o /tmp/kubeconfigs_artifact.zip -d /tmp/kubeconfigs_extract

# Extract underlay name and configure kubectl
STANDALONE_UNDERLAY=$(cat /tmp/kubeconfigs_extract/e2e-underlay-kubeconfig/meta | jq -r '.underlay_clusters["cx-1"].managed_cluster.name')
echo "Standalone underlay: ${STANDALONE_UNDERLAY}"
cp /tmp/kubeconfigs_extract/e2e-underlay-kubeconfig/kubeconfig-cx-1 ${STANDALONE_UNDERLAY}.kubeconfig

# Verify connectivity
KUBECONFIG=${STANDALONE_UNDERLAY}.kubeconfig kubectl get nodes
```

### Step 1.5: Build aksdev (if needed)

```bash
cd $AKS_RP_DIR
[ ! -f bin/aksdev ] && go build -o bin/aksdev ./test/e2e/cmd/aksdev && chmod +x bin/aksdev
```

---

## Part 2: Build a Test Image from Your Branch

Build the CCP image using the [Azure/prometheus-collector pipeline](https://github-private.visualstudio.com/azure/_build?definitionId=440):

1. Navigate to the pipeline and click **Run pipeline**.
2. Select the branch: `$PROM_COLLECTOR_BRANCH`.
3. Wait for the `ORAS Push Artifacts in /mnt/vss/_work/1/a/linuxccp/` stage inside `Build: linux CCP prometheus-collector image` to succeed.

> **Note:** The pipeline can show as failed overall. You can continue as long as the ORAS push stage succeeded.

**Check build status programmatically:**

```bash
# Replace <build-id> with the actual build ID
python3 otelcollector/test/ccp/tools/check_build.py <build-id>
```

4. Extract the image tag from the build output. The full image will be:

```
mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:<TAG>-ccp
```

```bash
export CCP_IMAGE_TAG="<paste-the-tag-here>"
export TEST_IMAGE="mcr.microsoft.com/azuremonitor/containerinsights/cidev/prometheus-collector/images:${CCP_IMAGE_TAG}"
echo "CCP test image: $TEST_IMAGE"
```

---

## Part 3: Create a Test Cluster with Monitoring Enabled

### Step 3.1: Enable AMA Metrics Addon on the cx-1 Underlay

The standalone cx-1 underlay is a real AKS cluster. We enable the AMA Metrics addon on it to get MSI token access for metric ingestion. See [Why do we use the cx-1 underlay?](#appendix-why-do-we-use-the-cx-1-underlay).

```bash
# Get values from the Azure Portal for the standalone resource group
export AKS_RESOURCE_ID=/subscriptions/<subscription-id>/resourcegroups/<standalone-resource-group>/providers/Microsoft.ContainerService/managedClusters/<standalone-resource-group>-cx-1
export AKS_CLUSTER_NAME=<standalone-resource-group>-cx-1
export RESOURCE_GROUP=<standalone-resource-group>
export SUBSCRIPTION_ID=<subscription-id>
export UNDERLAY_KUBECONFIG=${AKS_RP_DIR}/${STANDALONE_UNDERLAY}.kubeconfig

az account set -s $SUBSCRIPTION_ID
az aks update --enable-azure-monitor-metrics -n $AKS_CLUSTER_NAME -g $RESOURCE_GROUP
```

### Step 3.2: Create the Test Cluster

```bash
export CX_CLUSTER_NAME=${USER_ALIAS}-ccptest
export MC_SUB=82acd5bb-4206-47d4-9c12-a65db028483d
export LOCATION=<standalone-location>  # Must match standalone location

cd $AKS_RP_DIR
./bin/aksdev cluster create $CX_CLUSTER_NAME --location $LOCATION \
  --managedclustersubscription $MC_SUB --enableManagedIdentity \
  --enable-azure-monitor-metrics \
  --subscription-features AzureMonitorMetricsControlPlanePreview \
  --node-provisioning-mode Auto

./bin/aksdev cluster kubeconfig $CX_CLUSTER_NAME --managedclustersubscription $MC_SUB 2>/dev/null > ${CX_CLUSTER_NAME}.kubeconfig
```

---

## Part 4: Prepare the CCP Namespace

### Step 4.1: Scale Down Reconcilers

Scale all reconciler deployments to 0 replicas to prevent them from reverting patches:

```bash
kubectl scale deploy addonconfigreconciler -n addonconfigreconciler --replicas=0 --kubeconfig $UNDERLAY_KUBECONFIG
kubectl scale deploy overlaymgr-overlaymanager overlaymgr-overlaymanager-loop -n overlaymgr --replicas=0 --kubeconfig $UNDERLAY_KUBECONFIG
kubectl scale deploy eno-reconciler -n eno-system --replicas=0 --kubeconfig $UNDERLAY_KUBECONFIG
```

> **All four must be scaled down.** The `eno-reconciler` manages underlay deployment specs and will scale the other reconcilers back to 1 if left running.

### Step 4.2: Identify and Annotate the CCP Namespace

```bash
# CCP namespace starts with '6'
kubectl get ns --kubeconfig $UNDERLAY_KUBECONFIG | grep ^6
export CCP_NS=<ccp-namespace-id>

# Skip reconciliation for ~7 days
export SKIP_CCP_RECONCILE_UNTIL=$(date -u -v+7d '+%Y-%m-%dT%H:%M:%SZ')
kubectl annotate namespace $CCP_NS skip-ccp-reconcile-until-this-time="$SKIP_CCP_RECONCILE_UNTIL" --overwrite --kubeconfig $UNDERLAY_KUBECONFIG
```

### Step 4.3: Patch the ama-metrics-ccp Deployment

#### 4.3a. Set CLUSTER env var to the cx-1 underlay resource ID

```bash
kubectl patch deployment ama-metrics-ccp -n $CCP_NS --type=strategic --kubeconfig $UNDERLAY_KUBECONFIG \
  -p "{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"prometheus-collector\",\"env\":[{\"name\":\"CLUSTER\",\"value\":\"$AKS_RESOURCE_ID\",\"valueFrom\":null}]}]}}}}"
```

#### 4.3b. Replace msi-adapter with addon-token-adapter (if needed)

```bash
# Check which container is present
kubectl get deploy ama-metrics-ccp -n $CCP_NS -o jsonpath='{.spec.template.spec.containers[*].name}' --kubeconfig $UNDERLAY_KUBECONFIG
```

If `msi-adapter` is present, export the deployment, replace the container block with `addon-token-adapter`:

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

#### 4.3c. Update the prometheus-collector image

```bash
kubectl set image deployment/ama-metrics-ccp -n $CCP_NS prometheus-collector=$TEST_IMAGE --kubeconfig $UNDERLAY_KUBECONFIG
```

---

## Part 5: Validate Health Metrics

After the pod restarts with the new image (~2-3 minutes), validate health metrics.

### Step 5.1: Wait for Pod Ready

```bash
kubectl rollout status deployment/ama-metrics-ccp -n $CCP_NS --timeout=300s --kubeconfig $UNDERLAY_KUBECONFIG
```

### Step 5.2: Port-Forward and Fetch Metrics

```bash
POD=$(kubectl get pod -n $CCP_NS -l rsName=ama-metrics-ccp -o jsonpath='{.items[0].metadata.name}' --kubeconfig $UNDERLAY_KUBECONFIG)
echo "Pod: $POD"

# Port-forward to health metrics port (2234)
kubectl port-forward pod/$POD 12234:2234 -n $CCP_NS --kubeconfig $UNDERLAY_KUBECONFIG &
PF_PID=$!
sleep 3

# Fetch metrics
curl -s http://127.0.0.1:12234/metrics > /tmp/ccp_health_metrics.txt
kill $PF_PID 2>/dev/null
```

### Step 5.3: Validate All 8 Health Metrics

The following metrics should all be present:

| Metric | Source | Expected Value |
|--------|--------|----------------|
| `timeseries_received_per_minute` | ME logs | > 0 |
| `timeseries_sent_per_minute` | ME logs | > 0 |
| `bytes_sent_per_minute` | ME logs | > 0 |
| `invalid_custom_prometheus_config` | Status | 0 |
| `exporting_metrics_failed` | Status | 0 |
| `otelcol_receiver_accepted_metric_points` | Otelcol diagnostic | >= 0 |
| `otelcol_exporter_sent_metric_points` | Otelcol diagnostic | >= 0 |
| `otelcol_exporter_send_failed_metric_points` | Otelcol diagnostic | >= 0 |

**Validation commands:**

```bash
# Check all 8 metrics are present
grep -cE "timeseries_received_per_minute|timeseries_sent_per_minute|bytes_sent_per_minute|invalid_custom_prometheus_config|exporting_metrics_failed|otelcol_receiver_accepted_metric_points|otelcol_exporter_sent_metric_points|otelcol_exporter_send_failed_metric_points" /tmp/ccp_health_metrics.txt

# Check ME-based metrics are > 0 (primary ingestion indicators)
grep -E "^(timeseries_received|timeseries_sent|bytes_sent)" /tmp/ccp_health_metrics.txt

# Check status metrics are 0 (healthy state)
grep -E "^(invalid_custom_prometheus_config|exporting_metrics_failed)" /tmp/ccp_health_metrics.txt

# Check labels include computer, release, controller_type
grep "timeseries_received_per_minute" /tmp/ccp_health_metrics.txt | head -1
```

### Step 5.4: Verify CCP Mode in Logs

Since CCP containers are distroless (no shell), use the logs API:

```bash
# Confirm CCP mode
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --kubeconfig $UNDERLAY_KUBECONFIG --tail=200 | grep -i "controlplane\|CCP\|CLUSTER_TYPE"

# Confirm fluent-bit is NOT running (CCP mode exposes metrics directly)
kubectl logs deploy/ama-metrics-ccp -c prometheus-collector -n $CCP_NS --kubeconfig $UNDERLAY_KUBECONFIG --tail=200 | grep -i "fluent-bit"
```

---

## Cleanup

When testing is complete:

```bash
# Delete the test cluster
cd $AKS_RP_DIR
./bin/aksdev cluster delete $CX_CLUSTER_NAME --managedclustersubscription $MC_SUB

# Scale reconcilers back up (if standalone is still needed)
kubectl scale deploy addonconfigreconciler -n addonconfigreconciler --replicas=1 --kubeconfig $UNDERLAY_KUBECONFIG
kubectl scale deploy overlaymgr-overlaymanager overlaymgr-overlaymanager-loop -n overlaymgr --replicas=1 --kubeconfig $UNDERLAY_KUBECONFIG
kubectl scale deploy eno-reconciler -n eno-system --replicas=1 --kubeconfig $UNDERLAY_KUBECONFIG

# Standalone auto-deletes after 3 days
```

---

## Troubleshooting

1. **503 from API server proxy** — The Kubernetes API server proxy is blocked by network policies in CCP namespaces. Use `kubectl port-forward` instead.
2. **Distroless containers** — CCP containers have no shell or `curl`. Use `kubectl logs` and `kubectl port-forward` + local `curl` to interact.
3. **Reconcilers reverting patches** — Verify all 4 reconcilers from Step 4.1 are scaled to 0: `addonconfigreconciler`, `overlaymgr-overlaymanager`, `overlaymgr-overlaymanager-loop`, `eno-reconciler`.
4. **Metrics all zero** — Allow 3-5 minutes after pod startup for ME to begin processing and publishing metrics. ME logs are parsed every 60 seconds.
5. **Port-forward drops** — CCP namespace port-forwards may time out. Re-establish with `kubectl port-forward`.
6. **Pod crashlooping** — Check `kubectl describe pod` and logs for the `prometheus-collector` and `addon-token-adapter` containers.

---

## Appendix: Why do we use the cx-1 underlay?

In standalone, the `ama-metrics-ccp` pod needs an MSI token to ingest metrics to Azure Monitor. The test cluster created via `aksdev` only exists within the standalone — there's no real ARM resource with an MSI.

However, the cx-1 underlay **is** a real AKS cluster. Enabling the AMA Metrics addon on cx-1 gives it permission to ingest to an Azure Monitor workspace. We configure the `ama-metrics-ccp` pod to use the cx-1 resource ID via the `CLUSTER` env var, which lets `addon-token-adapter` obtain the correct MSI token.

---

## Related Documents

| Document | Location |
|----------|----------|
| Test README | [test/README.md](../README.md) |
| Minimal Prometheus ingestion profile | [Microsoft Learn](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-configuration-minimal) |
| Azure Monitor Metrics enable guide | [Microsoft Learn](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/kubernetes-monitoring-enable?tabs=cli) |
| Skip CCP Reconcile | [ADO Wiki](https://msazure.visualstudio.com/CloudNativeCompute/_wiki/wikis/aks-troubleshooting-guide/506628/skip-ccp-reconcile) |
