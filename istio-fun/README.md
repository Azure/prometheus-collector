# Istio mTLS Configuration Files for ama-metrics

This directory contains ready-to-use configuration files for deploying Azure Monitor metrics (ama-metrics) with Istio mTLS support in a custom namespace.

## 🎯 Why Custom Namespace for Istio?

When Istio is enabled with mTLS, it issues certificates based on the **pod's namespace**. Deploying ama-metrics in `kube-system` causes Istio to issue `kube-system` certificates, which:
- May have elevated privileges
- Violates namespace isolation principles
- Makes it harder to apply granular Istio security policies

**Solution**: Deploy to a dedicated namespace (e.g., `monitoring`) for proper certificate scoping.

## 📁 Files in This Directory

| File | Purpose |
|------|---------|
| `istio-peer-auth-ama-metrics.yaml` | PeerAuthentication policy - enforces STRICT mTLS |
| `istio-destinationrule-azure-monitor.yaml` | DestinationRule & ServiceEntry - allows traffic to Azure Monitor endpoints |
| `istio-authz-ama-metrics.yaml` | AuthorizationPolicy - allows ama-metrics to scrape metrics across namespaces |
| `cross-namespace-secret-rbac.yaml` | RBAC - allows reading addon-token-adapter secret from kube-system |
| `custom-istio-values.yaml` | Helm values - configuration for ama-metrics deployment |
| `deploy-ama-metrics-istio.sh` | Automated deployment script |
| `parameterize-helm-templates.sh` | Script to parameterize Helm templates for custom namespace support |
| `README.md` | This file |

## 🔧 Preparing the Helm Chart (First Time Setup)

**IMPORTANT**: Before deploying, you must first parameterize the Helm chart templates to support custom namespaces.

### Run the Parameterization Script

```bash
# From the istio-fun directory
cd prometheus-collector/istio-fun

# Make script executable
chmod +x parameterize-helm-templates.sh

# Run the script to modify Helm templates
./parameterize-helm-templates.sh
```

**What this script does:**
- Adds helper templates to `_helpers.tpl` for namespace parameterization
- Updates `values-template.yaml` with a `namespace` parameter
- Modifies all 16 Helm template files to use `{{ include "ama-metrics.namespace" . }}`
- Preserves backward compatibility (defaults to `kube-system`)

**Expected output:**
```
✓ Helper functions in _helpers.tpl: 2
✓ Template files using helpers: 16
✓ Remaining hardcoded kube-system: 0 (should be 0)
```

**Note**: This only needs to be run **once**. The script modifies files directly. If you need to restore the original files, use: `git checkout ../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/`

---

## 🚀 Quick Start

### Prerequisites

- AKS cluster with Istio installed
- `kubectl` configured for your cluster
- Helm 3 installed
- Azure CLI (`az`) installed (optional but recommended)
- Cluster admin access
- **Helm chart parameterized** (run `parameterize-helm-templates.sh` first - see above)

### Option 1: Automated Deployment (Recommended)

```bash
# 1. Configure environment variables
export NAMESPACE="monitoring"      # Your desired namespace
export CLUSTER_NAME="your-cluster"
export RESOURCE_GROUP="your-rg"

# 2. Make script executable
chmod +x deploy-ama-metrics-istio.sh

# 3. Run deployment script
./deploy-ama-metrics-istio.sh
```

**What the script does:**
1. Creates namespace and enables Istio injection
2. Updates all YAML configuration files with your chosen namespace
3. Applies Istio configurations (PeerAuthentication, DestinationRule, AuthorizationPolicy, RBAC)
4. Checks/enables Azure Monitor addon to create the `addon-token-adapter` secret
5. Optionally deploys ARM template (creates DCR, DCE, Azure Monitor resources)
6. **Deploys parameterized Helm chart** with your custom namespace via `custom-istio-values.yaml`
7. Verifies deployment and Istio sidecar injection

**How it uses the parameterized Helm chart:**
```bash
helm upgrade --install ama-metrics \
  ../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon \
  --namespace ${NAMESPACE} \
  --values custom-istio-values.yaml \
  --create-namespace
```

The script automatically updates `custom-istio-values.yaml` with `namespace: "${NAMESPACE}"`, and the parameterized Helm templates (created by `parameterize-helm-templates.sh`) read this value to deploy all resources to your custom namespace.

### Option 2: Manual Deployment

#### Step 1: Update Configuration Files

Edit the following files to match your environment:

**All YAML files**: Replace `monitoring` with your desired namespace
**`custom-istio-values.yaml`**: Update:
- `namespace`
- `global.commonGlobals.Region`
- `global.commonGlobals.Customer.AzureResourceID`

#### Step 2: Create Namespace

```bash
NAMESPACE="monitoring"  # Change as needed

kubectl create namespace ${NAMESPACE}
kubectl label namespace ${NAMESPACE} istio-injection=enabled
```

#### Step 3: Apply Istio Configurations

```bash
kubectl apply -f istio-peer-auth-ama-metrics.yaml
kubectl apply -f istio-destinationrule-azure-monitor.yaml
kubectl apply -f istio-authz-ama-metrics.yaml
kubectl apply -f cross-namespace-secret-rbac.yaml
```

#### Step 4: Ensure addon-token-adapter Secret Exists

```bash
# Enable addon to create secret (if not already enabled)
az aks update --enable-azure-monitor-metrics \
  -n ${CLUSTER_NAME} -g ${RESOURCE_GROUP}

# Verify secret
kubectl get secret aad-msi-auth-token -n kube-system
```

#### Step 5: Deploy ARM Template

Deploy the ARM template from `../AddonArmTemplate/` with the **addon section commented out** (lines ~160-200 in `FullAzureMonitorMetricsProfile.json`).

This creates the Data Collection Rule (DCR), Data Collection Endpoint (DCE), and other Azure resources.

#### Step 6: Deploy Helm Chart

```bash
helm upgrade --install ama-metrics \
  ../otelcollector/deploy/addon-chart/azure-monitor-metrics-addon \
  --namespace ${NAMESPACE} \
  --values custom-istio-values.yaml \
  --create-namespace
```

## ✅ Verification

### Check Istio Sidecar Injection

```bash
# Pods should show 2/2 containers (app + istio-proxy)
kubectl get pods -n ${NAMESPACE}

# Verify container names
kubectl get pods -n ${NAMESPACE} \
  -o jsonpath='{.items[*].spec.containers[*].name}'
```

### Verify mTLS Certificates

```bash
# Check certificate subject - should show your custom namespace
kubectl exec -n ${NAMESPACE} deploy/ama-metrics -c istio-proxy -- \
  openssl s_client -showcerts -connect localhost:15000 \
  </dev/null 2>/dev/null | \
  openssl x509 -noout -text | grep "Subject:"
```

### Check Metrics Collection

```bash
# Check ama-metrics logs
kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics \
  -c prometheus-collector | grep -i scrape

# Check Istio proxy logs
kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics \
  -c istio-proxy
```

### Verify Istio Policies

```bash
kubectl get peerauthentication -n ${NAMESPACE}
kubectl get destinationrule -n ${NAMESPACE}
kubectl get authorizationpolicy -n istio-system | grep ama-metrics
```

## 🔧 Customization

### Change Namespace

To use a different namespace (e.g., `istio-system`):

1. Update all `.yaml` files: Replace `monitoring` with your namespace
2. Update `custom-istio-values.yaml`: Change `namespace` value
3. Redeploy

### Adjust Istio Sidecar Resources

In pod annotations (via Helm chart templates):

```yaml
metadata:
  annotations:
    sidecar.istio.io/proxyCPU: "50m"
    sidecar.istio.io/proxyMemory: "64Mi"
    sidecar.istio.io/proxyCPULimit: "200m"
    sidecar.istio.io/proxyMemoryLimit: "256Mi"
```

### Allow Additional Scrape Paths

Edit `istio-authz-ama-metrics.yaml`:

```yaml
spec:
  rules:
    - to:
        - operation:
            paths: 
              - "/metrics"
              - "/stats/prometheus"
              - "/your-custom-path"  # Add here
```

## 🐛 Troubleshooting

### Issue: Pods show 1/2 Ready

**Cause**: Istio sidecar not injecting

**Solution**:
```bash
# Verify namespace label
kubectl get namespace ${NAMESPACE} --show-labels

# Should show: istio-injection=enabled
# If not:
kubectl label namespace ${NAMESPACE} istio-injection=enabled
```

### Issue: mTLS Connection Failures

**Cause**: PeerAuthentication not applied correctly

**Solution**:
```bash
# Check PeerAuthentication
kubectl get peerauthentication -n ${NAMESPACE} -o yaml

# Reapply if needed
kubectl apply -f istio-peer-auth-ama-metrics.yaml
```

### Issue: Can't Reach Azure Monitor

**Cause**: DestinationRule or ServiceEntry missing

**Solution**:
```bash
# Check configurations exist
kubectl get destinationrule -n ${NAMESPACE}
kubectl get serviceentry -n ${NAMESPACE}

# Test connectivity from pod
kubectl exec -n ${NAMESPACE} deploy/ama-metrics \
  -c prometheus-collector -- \
  curl -v https://eastus.ingest.monitor.azure.com
```

### Issue: Authentication Errors

**Cause**: Can't access addon-token-adapter secret

**Solution**:
```bash
# Verify RBAC is applied
kubectl get role read-addon-token -n kube-system
kubectl get rolebinding ama-metrics-read-addon-token -n kube-system

# Reapply if needed
kubectl apply -f cross-namespace-secret-rbac.yaml
```

## 📊 Architecture

```
┌─────────────────────────────────────────────────────────┐
│         AKS Cluster with Istio                          │
│                                                         │
│  ┌──────────────────────────────────────────────────┐  │
│  │  Namespace: monitoring (Istio-injected)          │  │
│  │                                                  │  │
│  │  ┌────────────────┐    ┌────────────────┐      │  │
│  │  │ ama-metrics    │    │ ama-metrics    │      │  │
│  │  │ ReplicaSet     │    │ DaemonSet      │      │  │
│  │  │ [app + proxy]  │    │ [app + proxy]  │      │  │
│  │  └────────┬───────┘    └───────┬────────┘      │  │
│  │           │                     │               │  │
│  │           └──────────┬──────────┘               │  │
│  │                      │                          │  │
│  │              ┌───────▼────────┐                 │  │
│  │              │ Istio mTLS     │                 │  │
│  │              │ (monitoring    │                 │  │
│  │              │  namespace)    │                 │  │
│  │              └───────┬────────┘                 │  │
│  └──────────────────────┼──────────────────────────┘  │
│                         │                             │
│  ┌──────────────────────┼──────────────────────────┐  │
│  │  Namespace: kube-system                         │  │
│  │                      │                          │  │
│  │       ┌──────────────▼────────────┐             │  │
│  │       │ addon-token-adapter       │             │  │
│  │       │ secret (cross-ns access)  │             │  │
│  │       └───────────────────────────┘             │  │
│  └──────────────────────────────────────────────────┘  │
│                         │                             │
└─────────────────────────┼─────────────────────────────┘
                          │
                ┌─────────▼──────────┐
                │ Azure Monitor      │
                │ Workspace          │
                └────────────────────┘
```

## 📚 Additional Resources

- [Main Istio mTLS Guide](../ISTIO_MTLS_NAMESPACE_GUIDE.md) - Detailed explanation
- [Custom Namespace Guide](../CUSTOM_NAMESPACE_GUIDE.md) - General namespace modifications
- [Istio Security](https://istio.io/latest/docs/concepts/security/)
- [Azure Monitor Metrics](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-overview)

## ⚠️ Important Notes

1. **This is a custom configuration** - Not officially supported by Microsoft for production use
2. **Test thoroughly** in non-production environments first
3. **Monitor closely** after deployment for any issues
4. **Keep configurations in sync** - If you update namespace, update ALL files
5. **Certificate scoping** - Main benefit is proper Istio certificate scoping to your namespace

## 🆘 Support

For issues:
1. Check troubleshooting section above
2. Review logs: `kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics`
3. Check Istio proxy: `kubectl logs -n ${NAMESPACE} -l rsName=ama-metrics -c istio-proxy`
4. Refer to main guide: `../ISTIO_MTLS_NAMESPACE_GUIDE.md`

---

**Last Updated**: November 2025
