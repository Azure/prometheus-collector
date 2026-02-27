# Istio mTLS & ama-metrics Custom Namespace Deployment

## 🎯 The Problem

When **Istio is enabled** with mTLS on an AKS cluster:

1. Istio issues **service certificates based on pod namespace**
2. Default ama-metrics deployment in `kube-system` gets certificates for `kube-system`
3. This creates **security/isolation concerns**:
   - `kube-system` certificates may have elevated privileges
   - Violates namespace isolation principles
   - Istio security policies may not apply correctly
   - Certificate rotation and management complexity

## ✅ The Solution

Deploy ama-metrics to a **dedicated namespace** (e.g., `monitoring` or `istio-system`) so Istio issues appropriate certificates for that namespace.

---

## 🚀 Recommended Approach for Istio Environments

### Architecture Decision

**Best Namespace Options for Istio + ama-metrics:**

1. **`istio-system`** (Recommended if ama-metrics monitors Istio)
   - ✅ Logical grouping with Istio components
   - ✅ Istio certificates scoped to Istio namespace
   - ✅ Easier to apply Istio-specific policies
   - ❌ Requires careful RBAC management

2. **`monitoring`** (Recommended for general monitoring)
   - ✅ Clear separation of concerns
   - ✅ Can include other monitoring tools
   - ✅ Istio certificates scoped to monitoring
   - ✅ Easier to manage monitoring stack together

3. **NOT `kube-system`** (Avoid with Istio)
   - ❌ Istio issues system-level certificates
   - ❌ Security boundary concerns
   - ❌ Harder to apply granular mTLS policies

---

## 📋 Implementation Steps

### Step 1: Enable Istio Sidecar Injection for ama-metrics Namespace

**Choose your namespace** and label it for Istio injection:

```bash
NAMESPACE="monitoring"  # or "istio-system"

# Create namespace
kubectl create namespace $NAMESPACE

# Enable Istio sidecar injection
kubectl label namespace $NAMESPACE istio-injection=enabled

# Verify
kubectl get namespace $NAMESPACE --show-labels
```

---

### Step 2: Configure Istio PeerAuthentication

Create a PeerAuthentication policy for ama-metrics:

**File**: `istio-peer-auth-ama-metrics.yaml`

```yaml
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: ama-metrics-mtls
  namespace: monitoring  # Your chosen namespace
spec:
  mtls:
    mode: STRICT  # Enforce mTLS for all ama-metrics pods
  selector:
    matchLabels:
      rsName: ama-metrics  # Matches ama-metrics pods
```

Apply:
```bash
kubectl apply -f istio-peer-auth-ama-metrics.yaml
```

---

### Step 3: Modify Helm Chart for Custom Namespace

**Key Changes** (see CUSTOM_NAMESPACE_GUIDE.md for full details):

1. **Add namespace parameter to values**
2. **Update all template namespaces**
3. **Handle addon-token-adapter secret** (still in kube-system)
4. **Add Istio sidecar annotations**

**Critical Addition**: Istio sidecar configuration in pod templates:

**File**: `templates/ama-metrics-deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ama-metrics
  namespace: {{ .Values.namespace | default "monitoring" }}
spec:
  template:
    metadata:
      annotations:
        # Istio sidecar configuration
        sidecar.istio.io/inject: "true"
        # Allow traffic to Azure Monitor endpoints
        traffic.sidecar.istio.io/excludeOutboundIPRanges: "169.254.169.254/32"
        # Exclude health check ports from Istio
        traffic.sidecar.istio.io/excludeInboundPorts: "8080,8888"
        # Set Istio proxy resource limits
        sidecar.istio.io/proxyCPU: "100m"
        sidecar.istio.io/proxyMemory: "128Mi"
      labels:
        rsName: ama-metrics
        # Istio version label (recommended)
        version: v1
    spec:
      # ... rest of pod spec
```

**Same for DaemonSet** (`templates/ama-metrics-daemonset.yaml`):

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ama-metrics-node
  namespace: {{ .Values.namespace | default "monitoring" }}
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/inject: "true"
        traffic.sidecar.istio.io/excludeOutboundIPRanges: "169.254.169.254/32"
        traffic.sidecar.istio.io/excludeInboundPorts: "8080,8888"
        # DaemonSet may need higher limits due to node-level metrics
        sidecar.istio.io/proxyCPU: "200m"
        sidecar.istio.io/proxyMemory: "256Mi"
      labels:
        dsName: ama-metrics
        version: v1
```

---

### Step 4: Configure Istio DestinationRule for Azure Monitor

ama-metrics needs to communicate with Azure Monitor endpoints. Configure Istio to allow this:

**File**: `istio-destinationrule-azure-monitor.yaml`

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: azure-monitor-endpoints
  namespace: monitoring
spec:
  host: "*.monitor.azure.com"
  trafficPolicy:
    tls:
      mode: SIMPLE  # Use simple TLS, not Istio mTLS
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: azure-monitor-ingestion
  namespace: monitoring
spec:
  host: "*.ingest.monitor.azure.com"
  trafficPolicy:
    tls:
      mode: SIMPLE
---
apiVersion: networking.istio.io/v1beta1
kind: ServiceEntry
metadata:
  name: azure-monitor-external
  namespace: monitoring
spec:
  hosts:
    - "*.monitor.azure.com"
    - "*.ingest.monitor.azure.com"
  ports:
    - number: 443
      name: https
      protocol: HTTPS
  location: MESH_EXTERNAL
  resolution: DNS
```

Apply:
```bash
kubectl apply -f istio-destinationrule-azure-monitor.yaml
```

---

### Step 5: Handle Cross-Namespace Secret Access

**The Challenge**: addon-token-adapter secret is in `kube-system`, but pods are in `monitoring`.

**Solution**: Use Kubernetes Service Account Token Volume Projection:

**Modify deployment/daemonset pod spec**:

```yaml
spec:
  serviceAccountName: ama-metrics-serviceaccount
  containers:
    - name: prometheus-collector
      # ... existing config
      volumeMounts:
        - name: adapter-token
          mountPath: /var/run/secrets/tokens
          readOnly: true
  volumes:
    - name: adapter-token
      projected:
        sources:
          # Option 1: Use service account token (recommended for Istio)
          - serviceAccountToken:
              path: token
              expirationSeconds: 3600
              audience: api
          # Option 2: Reference secret in kube-system (requires RBAC)
          # - secret:
          #     name: aad-msi-auth-token
          #     items:
          #       - key: token
          #         path: token
```

**If using Option 2** (secret reference), add RBAC:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: read-addon-token
  namespace: kube-system
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    resourceNames: ["aad-msi-auth-token"]
    verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ama-metrics-read-addon-token
  namespace: kube-system
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: monitoring  # Your custom namespace
roleRef:
  kind: Role
  name: read-addon-token
  apiGroup: rbac.authorization.k8s.io
```

---

### Step 6: Configure Istio Authorization Policies

Allow ama-metrics to scrape targets across namespaces:

**File**: `istio-authz-ama-metrics.yaml`

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: ama-metrics-scrape-allow
  namespace: istio-system  # Apply cluster-wide
spec:
  action: ALLOW
  rules:
    - from:
        - source:
            namespaces: ["monitoring"]  # Your ama-metrics namespace
            principals: ["cluster.local/ns/monitoring/sa/ama-metrics-serviceaccount"]
      to:
        - operation:
            paths: ["/metrics", "/stats/prometheus"]
            methods: ["GET"]
      when:
        - key: request.headers[user-agent]
          values: ["Prometheus/*", "OpenTelemetry-Collector/*"]
```

Apply:
```bash
kubectl apply -f istio-authz-ama-metrics.yaml
```

---

### Step 7: Deploy with Helm

**Values file** (`custom-istio-values.yaml`):

```yaml
# Custom namespace for Istio compatibility
namespace: "monitoring"

AzureMonitorMetrics:
  ImageTag: "6.24.1-main-11-14-2025-15146744"
  ImageTagWin: "6.24.1-main-11-14-2025-15146744-win"
  ImageTagTargetAllocator: "6.24.1-main-11-14-2025-15146744-targetallocator"
  ImageTagCfgReader: "6.24.1-main-11-14-2025-15146744-cfg"
  
  # Istio-specific annotations (applied via templates)
  podAnnotations:
    sidecar.istio.io/inject: "true"
    traffic.sidecar.istio.io/excludeOutboundIPRanges: "169.254.169.254/32"
    traffic.sidecar.istio.io/excludeInboundPorts: "8080,8888"

global:
  commonGlobals:
    Region: "eastus"
    Customer:
      AzureResourceID: "/subscriptions/.../managedClusters/my-cluster"
```

**Deploy**:
```bash
helm upgrade --install ama-metrics \
  ./azure-monitor-metrics-addon \
  --namespace monitoring \
  --values custom-istio-values.yaml \
  --create-namespace
```

---

## 🔍 Verification

### Check Istio Sidecar Injection

```bash
# Should show 2/2 containers (app + istio-proxy)
kubectl get pods -n monitoring -l rsName=ama-metrics

# Example output:
# NAME                          READY   STATUS
# ama-metrics-xxxxx             2/2     Running
# ama-metrics-node-xxxxx        2/2     Running
```

### Verify mTLS Certificates

```bash
# Check Istio certificates issued to ama-metrics pods
kubectl exec -n monitoring deploy/ama-metrics -c istio-proxy -- \
  openssl s_client -showcerts -connect localhost:15000 </dev/null 2>/dev/null | \
  openssl x509 -noout -text | grep -A 2 "Subject:"

# Should show certificate for monitoring namespace
# Subject: O = monitoring, CN = ...
```

### Test Metrics Scraping with mTLS

```bash
# Check if ama-metrics can scrape targets
kubectl logs -n monitoring -l rsName=ama-metrics | grep -i "scrape\|mtls\|certificate"

# Should NOT see certificate errors
# Should see successful scrapes
```

### Verify Istio Policy Enforcement

```bash
# Check Istio proxy logs
kubectl logs -n monitoring deploy/ama-metrics -c istio-proxy | tail -20

# Look for:
# - mTLS handshakes
# - Authorization decisions
# - No permission denied errors
```

---

## 🔧 Troubleshooting

### Issue 1: mTLS Connection Failures

**Symptom**: Logs show "connection refused" or "certificate verification failed"

**Solution**:
```bash
# Check PeerAuthentication mode
kubectl get peerauthentication -n monitoring -o yaml

# Ensure STRICT mode is set correctly
# Check Istio sidecar is running
kubectl get pods -n monitoring -o jsonpath='{.items[*].spec.containers[*].name}'
```

### Issue 2: Can't Scrape Targets in Other Namespaces

**Symptom**: Metrics from other namespaces not being collected

**Solution**:
```bash
# Verify AuthorizationPolicy allows cross-namespace scraping
kubectl get authorizationpolicy -A

# Check if target pods have Istio sidecars
kubectl get pods -A -o jsonpath='{range .items[*]}{.metadata.namespace}{"\t"}{.metadata.name}{"\t"}{.spec.containers[*].name}{"\n"}{end}' | grep istio-proxy
```

### Issue 3: Azure Monitor Endpoint Connection Issues

**Symptom**: Metrics not reaching Azure Monitor

**Solution**:
```bash
# Check DestinationRule for Azure endpoints
kubectl get destinationrule -n monitoring azure-monitor-endpoints -o yaml

# Verify ServiceEntry exists
kubectl get serviceentry -n monitoring azure-monitor-external -o yaml

# Test connectivity from pod
kubectl exec -n monitoring deploy/ama-metrics -c prometheus-collector -- \
  curl -v https://eastus.ingest.monitor.azure.com
```

### Issue 4: High Istio Sidecar Resource Usage

**Symptom**: Pods using more resources than expected

**Solution**:
```yaml
# Adjust sidecar resource limits in pod annotations
metadata:
  annotations:
    sidecar.istio.io/proxyCPU: "50m"        # Reduce CPU
    sidecar.istio.io/proxyMemory: "64Mi"    # Reduce memory
    sidecar.istio.io/proxyCPULimit: "200m"
    sidecar.istio.io/proxyMemoryLimit: "256Mi"
```

---

## 📊 Istio-Specific Configuration Summary

| Component | Namespace | Istio Sidecar | mTLS Mode | Notes |
|-----------|-----------|---------------|-----------|-------|
| ama-metrics ReplicaSet | `monitoring` | ✅ Yes | STRICT | Main collector |
| ama-metrics DaemonSet | `monitoring` | ✅ Yes | STRICT | Node-level metrics |
| ama-metrics-ksm | `monitoring` | ✅ Yes | STRICT | Kube-state-metrics |
| Target Allocator | `monitoring` | ✅ Yes | STRICT | Target distribution |
| addon-token-adapter secret | `kube-system` | ❌ N/A | N/A | Cross-namespace access needed |

---

## 🎓 Why This Works with Istio

1. **Namespace Isolation**: 
   - Istio issues certificates scoped to `monitoring` namespace
   - Prevents `kube-system` certificate privilege escalation

2. **mTLS Policy Control**:
   - Can apply specific PeerAuthentication to ama-metrics
   - Granular AuthorizationPolicy for scraping

3. **Service Mesh Benefits**:
   - Traffic encryption between ama-metrics components
   - Observability via Istio telemetry
   - Circuit breaking and retry policies

4. **Security Boundaries**:
   - Clear separation from system components
   - Easier to audit and comply with security policies

---

## ✅ Final Checklist

Before deploying ama-metrics in Istio environment:

- [ ] Choose appropriate namespace (`monitoring` or `istio-system`)
- [ ] Enable Istio injection on namespace
- [ ] Modify Helm templates for custom namespace
- [ ] Add Istio sidecar annotations to pod specs
- [ ] Create PeerAuthentication policy
- [ ] Create DestinationRule for Azure Monitor
- [ ] Create ServiceEntry for external Azure endpoints
- [ ] Create AuthorizationPolicy for cross-namespace scraping
- [ ] Handle addon-token-adapter secret access
- [ ] Update ClusterRoleBindings with correct namespace
- [ ] Test metrics collection
- [ ] Verify mTLS certificates
- [ ] Check Azure Monitor data ingestion

---

## 📚 Additional Resources

- [Istio Security - mTLS](https://istio.io/latest/docs/concepts/security/#mutual-tls-authentication)
- [Istio Authorization Policies](https://istio.io/latest/docs/reference/config/security/authorization-policy/)
- [Azure Monitor Metrics Documentation](https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/prometheus-metrics-overview)
- [CUSTOM_NAMESPACE_GUIDE.md](./CUSTOM_NAMESPACE_GUIDE.md) - Full namespace modification guide

---

## 🚀 Quick Start Script

```bash
#!/bin/bash
# deploy-ama-metrics-istio.sh

NAMESPACE="monitoring"
CLUSTER_NAME="your-cluster"
RESOURCE_GROUP="your-rg"

echo "Deploying ama-metrics with Istio mTLS support..."

# 1. Create and label namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -
kubectl label namespace $NAMESPACE istio-injection=enabled --overwrite

# 2. Apply Istio configurations
kubectl apply -f istio-peer-auth-ama-metrics.yaml
kubectl apply -f istio-destinationrule-azure-monitor.yaml
kubectl apply -f istio-authz-ama-metrics.yaml

# 3. Deploy ARM template for Azure infrastructure
echo "Deploy ARM template with addon section commented out"
read -p "Press enter when ARM template is deployed..."

# 4. Install Helm chart
helm upgrade --install ama-metrics ./azure-monitor-metrics-addon \
  --namespace $NAMESPACE \
  --values custom-istio-values.yaml \
  --create-namespace

# 5. Verify
echo "Verifying deployment..."
kubectl get pods -n $NAMESPACE
kubectl get peerauthentication -n $NAMESPACE
kubectl get destinationrule -n $NAMESPACE

echo "Deployment complete! Check logs:"
echo "kubectl logs -n $NAMESPACE -l rsName=ama-metrics"
```

---

**This configuration allows ama-metrics to work seamlessly with Istio mTLS while maintaining proper namespace isolation and certificate scoping.**
