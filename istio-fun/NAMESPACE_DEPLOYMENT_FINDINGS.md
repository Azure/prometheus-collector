# Custom Namespace Deployment Findings

This document summarizes the findings from testing ama-metrics Helm chart deployment to a custom namespace (`ama-metrics-zane-test`) compared to the default `kube-system` namespace.

## Executive Summary

Deploying to a custom namespace requires additional configuration for TLS certificates. Other errors (TokenConfig, MetricsExtension) are Azure Monitor configuration issues unrelated to namespace selection.

---

## Template Modifications Made

All Helm templates were parameterized to support custom namespaces:

### Changes Applied
- Replaced all hardcoded `namespace: kube-system` with `{{ $.Values.namespace }}`
- Updated 16+ template files
- Fixed additional references:
  - Environment variables in target allocator (`OTELCOL_NAMESPACE`)
  - ServiceAccount references in SCC
  - Lookup functions in helpers and deployments
  - `--secret-namespace` arguments

### values-template.yaml Addition
```yaml
# Custom namespace for deployment (defaults to kube-system for compatibility)
namespace: "kube-system"
```

---

## Pod Status Comparison

| Component | kube-system | Custom Namespace |
|-----------|-------------|------------------|
| **Target Allocator** | ✅ 2/2 Running | ❌ 1/2 CrashLoopBackOff |
| ama-metrics (Deployment) | 1/2 Error | 1/2 CrashLoopBackOff |
| ama-metrics-node (DaemonSet) | 1/2 Error | 1/2 Error/CrashLoopBackOff |
| ama-metrics-ksm | ✅ 1/1 Running | ✅ 1/1 Running |

---

## Error Analysis

### 1. Target Allocator - TLS Certificate Error (Custom Namespace Only)

**Error:**
```
open /etc/operator-targets/server/certs/server.crt: no such file or directory
```

**Root Cause:**
- TLS secrets exist in kube-system but NOT in custom namespace:
  - `ama-metrics-operator-targets-server-tls-secret`
  - `ama-metrics-operator-targets-client-tls-secret`

### 🔍 Secret Creation Discovery (NEW)

**Who Creates the Secrets?**

The **`config-reader` container** (sidecar in the Target Allocator pod) auto-generates the TLS secrets on startup:

```log
2025/12/01 19:12:01 Secret ama-metrics-operator-targets-server-tls-secret created/updated successfully in namespace kube-system
2025/12/01 19:12:01 Generating secret with CA cert
2025/12/01 19:12:01 Secret ama-metrics-operator-targets-client-tls-secret created/updated successfully in namespace kube-system
2025/12/01 19:12:01 TLS certificates and secret generated successfully
```

**How It Works:**
1. `config-reader` container starts first
2. Generates self-signed CA and server/client certificates
3. Creates the secrets in the deployment namespace
4. `targetallocator` container reads certs from volume mounts
5. Pod becomes 2/2 Running

**RBAC Required:**
- Uses `ama-metrics-serviceaccount` 
- Bound to `ama-metrics-reader` ClusterRole via `ama-metrics-clusterrolebinding`
- ClusterRole has `secrets: create` permission

**Why kube-system works:**
- The config-reader successfully creates secrets with proper RBAC
- Secrets are auto-generated on each fresh deployment

**Why custom namespace initially failed:**
- Likely a timing/race condition on first attempt
- The `--secret-namespace` flag needs to point to the correct namespace
- ClusterRoleBinding works across namespaces (cluster-scoped)

**Solutions:**
1. **Wait for auto-creation** (secrets are created by config-reader):
   - Ensure RBAC (ClusterRoleBinding) is properly deployed
   - Verify `--secret-namespace` matches deployment namespace

2. **Copy secrets from kube-system (if already exist):**
   ```bash
   kubectl get secret ama-metrics-operator-targets-server-tls-secret -n kube-system -o yaml | \
     sed 's/namespace: kube-system/namespace: ama-metrics-zane-test/' | kubectl apply -f -
   kubectl get secret ama-metrics-operator-targets-client-tls-secret -n kube-system -o yaml | \
     sed 's/namespace: kube-system/namespace: ama-metrics-zane-test/' | kubectl apply -f -
   ```

3. **Disable HTTPS (not recommended for production):**
   ```yaml
   # In values.yaml
   AzureMonitorMetrics:
     OperatorTargetsHttpsEnabled: false
   ```

4. **Use cert-manager:**
   - Set up cert-manager to generate certificates in the custom namespace

---

### 2. TokenConfig.json Missing (Both Namespaces)

**Error:**
```
TokenConfig.json does not exist
No configuration present for the AKS resource
```

**Root Cause:**
- Azure Monitor for Prometheus is not enabled on this AKS cluster
- Missing Data Collection Rule (DCR) and Data Collection Endpoint (DCE) association

**Solution:**
Enable Azure Monitor for Prometheus through:
- Azure Portal → AKS cluster → Insights → Enable Prometheus
- ARM/Bicep template with proper DCR/DCE configuration

---

### 3. MetricsExtension Connection Refused (Both Namespaces)

**Error:**
```
dial tcp 127.0.0.1:55680: connection refused
Error getting PID for process MetricsExtension
```

**Root Cause:**
- MetricsExtension sidecar container fails to start
- This is a downstream effect of TokenConfig.json missing

**Solution:**
- Fix Azure Monitor configuration (see #2 above)

---

## Target Allocator Purpose

The Target Allocator distributes Prometheus scrape targets across multiple collector replicas:

1. **Prevents duplicate metrics** - Each target is scraped by only ONE collector
2. **Load balances** - Distributes targets evenly across collectors
3. **Dynamic rebalancing** - Handles pod/collector scaling automatically

Without Target Allocator, multiple replicas would all scrape the same targets.

---

## Deploy Script (deploy-simple.sh)

Current configuration:
```bash
NAMESPACE="${NAMESPACE:-ama-metrics-zane-test}"
IMAGE_TAG="6.24.1-main-11-14-2025-15146744"
MCR_REPOSITORY="/azuremonitor/containerinsights/ciprod/prometheus-collector/images"
AKS_REGION="westeurope"
```

Usage:
```bash
# Deploy to default custom namespace
./deploy-simple.sh

# Deploy to specific namespace
NAMESPACE=my-custom-ns ./deploy-simple.sh
```

---

## Recommendations

### For Custom Namespace Deployment

1. **Pre-create TLS secrets** before deploying:
   - Either copy from kube-system
   - Or set up cert-manager
   - Or disable HTTPS (dev/test only)

2. **Ensure Azure Monitor is configured** on the cluster:
   - Enable Prometheus monitoring in Azure Portal
   - Associate DCR/DCE with the cluster

3. **Consider namespace isolation requirements**:
   - RBAC: ClusterRoleBindings already work across namespaces
   - Network policies may need adjustment
   - Secret management for TLS certificates

### For Production

1. Use cert-manager for automatic certificate rotation
2. Enable Istio mTLS if deploying in Istio mesh
3. Configure proper Azure Monitor DCR/DCE association

---

## Files Modified

- `templates/*.yaml` - All 16+ template files updated with `{{ $.Values.namespace }}`
- `values-template.yaml` - Added `namespace` field
- `istio-fun/deploy-simple.sh` - Deployment script with configurable namespace

---

## Test Results Date

December 1, 2025
