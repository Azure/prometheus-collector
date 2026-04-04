# Option 2: Detailed Guide - Custom Namespace Deployment

**⚠️ WARNING: This is for development/testing only. NOT supported for production.**

This guide walks through modifying the Helm chart to deploy ama-metrics to a custom namespace instead of `kube-system`.

## 📋 Prerequisites

- Local clone of prometheus-collector repository
- kubectl access to your cluster
- Helm 3 installed
- Understanding that this is an unsupported configuration

## 🎯 Overview of Changes Needed

You need to modify **28 files** across the Helm chart to:
1. Parameterize the namespace
2. Update service account references
3. Fix secret and configmap cross-namespace access
4. Update RBAC bindings
5. Handle addon-token-adapter secret location

---

## 📝 Step-by-Step Implementation

### Step 1: Add Namespace Parameter to Values

**File**: `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/values-template.yaml`

**Add at the top**:
```yaml
# Custom namespace for ama-metrics deployment
namespace: "monitoring-system"  # Change this to your desired namespace

AzureMonitorMetrics:
  KubeStateMetrics:
    # ... existing config
```

---

### Step 2: Create Namespace Helper Template

**File**: `otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/_helpers.tpl`

**Add this helper** (create file if it doesn't exist):
```yaml
{{/*
Get the namespace for deployment
*/}}
{{- define "ama-metrics.namespace" -}}
{{- if .Values.namespace }}
{{- .Values.namespace }}
{{- else }}
kube-system
{{- end }}
{{- end }}

{{/*
Get addon token secret namespace (always kube-system for AKS)
*/}}
{{- define "ama-metrics.secretNamespace" -}}
kube-system
{{- end }}
```

---

### Step 3: Update All Template Files

Here's the detailed process for each type of file:

#### **A. ServiceAccount**

**File**: `templates/ama-metrics-serviceAccount.yaml`

**Before**:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ama-metrics-serviceaccount
  namespace: kube-system
```

**After**:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ama-metrics-serviceaccount
  namespace: {{ include "ama-metrics.namespace" . }}
```

---

#### **B. Deployments (ReplicaSet)**

**File**: `templates/ama-metrics-deployment.yaml`

**Key Changes**:

1. **Namespace**:
```yaml
metadata:
  name: ama-metrics
  namespace: {{ include "ama-metrics.namespace" . }}
```

2. **ConfigMap reference** (stays in custom namespace):
```yaml
args:
  - --configmap-namespace={{ include "ama-metrics.namespace" . }}
  - --configmap-name=ama-metrics-settings-configmap
```

3. **Secret reference** (CRITICAL - must point to kube-system for addon-token-adapter):
```yaml
args:
  - --secret-namespace={{ include "ama-metrics.secretNamespace" . }}
  - --secret-name=aad-msi-auth-token
```

4. **Volume mounts for secrets**:
```yaml
volumes:
  - name: adapter-token
    secret:
      secretName: aad-msi-auth-token
      # This secret lives in kube-system, need to handle cross-namespace access
```

---

#### **C. DaemonSets**

**Files**: 
- `templates/ama-metrics-daemonset.yaml` (Linux)
- `templates/ama-metrics-daemonset-win.yaml` (Windows)

**Same pattern as Deployment**:

```yaml
metadata:
  name: ama-metrics-node
  namespace: {{ include "ama-metrics.namespace" . }}

# In container args:
args:
  - --secret-namespace={{ include "ama-metrics.secretNamespace" . }}
  - --secret-name=aad-msi-auth-token
  - --configmap-namespace={{ include "ama-metrics.namespace" . }}
```

---

#### **D. KubeStateMetrics Deployment**

**File**: `templates/ama-metrics-ksm-deployment.yaml`

```yaml
metadata:
  name: ama-metrics-ksm
  namespace: {{ include "ama-metrics.namespace" . }}
```

**File**: `templates/ama-metrics-ksm-service.yaml`
```yaml
metadata:
  name: ama-metrics-ksm
  namespace: {{ include "ama-metrics.namespace" . }}
```

**File**: `templates/ama-metrics-ksm-serviceaccount.yaml`
```yaml
metadata:
  name: ama-metrics-ksm
  namespace: {{ include "ama-metrics.namespace" . }}
```

---

#### **E. ClusterRoleBindings**

**File**: `templates/ama-metrics-clusterRoleBinding.yaml`

**CRITICAL**: Update subject namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ama-metrics-clusterRoleBinding
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: {{ include "ama-metrics.namespace" . }}  # Changed from kube-system
roleRef:
  kind: ClusterRole
  name: ama-metrics-clusterRole
  apiGroup: rbac.authorization.k8s.io
```

**File**: `templates/ama-metrics-ksm-clusterrolebinding.yaml`

```yaml
subjects:
  - kind: ServiceAccount
    name: ama-metrics-ksm
    namespace: {{ include "ama-metrics.namespace" . }}  # Changed from kube-system
```

---

#### **F. Services**

**File**: `templates/ama-metrics-targetallocator-service.yaml`

```yaml
metadata:
  name: ama-metrics-operator-targets
  namespace: {{ include "ama-metrics.namespace" . }}
```

---

#### **G. Target Allocator**

**File**: `templates/ama-metrics-targetallocator.yaml`

```yaml
metadata:
  name: ama-metrics-operator-targets
  namespace: {{ include "ama-metrics.namespace" . }}
spec:
  template:
    spec:
      containers:
        - name: config-reader
          args:
            - --secret-namespace={{ include "ama-metrics.secretNamespace" . }}
            - --secret-name=aad-msi-auth-token
            - --configmap-namespace={{ include "ama-metrics.namespace" . }}
```

---

#### **H. ConfigMaps and Secrets**

**File**: `templates/ama-metrics-secret.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ama-metrics-proxy-config
  namespace: {{ include "ama-metrics.namespace" . }}
type: Opaque
---
apiVersion: v1
kind: Secret
metadata:
  name: ama-metrics-proxy-cert
  namespace: {{ include "ama-metrics.namespace" . }}
type: Opaque
```

---

#### **I. HPA (Horizontal Pod Autoscaler)**

**File**: `templates/ama-metrics-collector-hpa.yaml`

```yaml
metadata:
  name: ama-metrics-hpa
  namespace: {{ include "ama-metrics.namespace" . }}
spec:
  scaleTargetRef:
    kind: Deployment
    name: ama-metrics
    # Note: HPA must be in same namespace as target
```

---

#### **J. PodDisruptionBudget**

**File**: `templates/ama-metrics-pod-disruption-budget.yaml`

```yaml
metadata:
  name: ama-metrics-pdb
  namespace: {{ include "ama-metrics.namespace" . }}
```

---

### Step 4: Handle Cross-Namespace Secret Access

**THE CRITICAL ISSUE**: The `aad-msi-auth-token` secret is created by AKS in `kube-system`, but your pods are in a different namespace.

**Solution Options**:

#### **Option 4A: Copy Secret (Easier, Less Secure)**

Create a Job to copy the secret:

**File**: `templates/copy-addon-secret-job.yaml` (NEW FILE)

```yaml
{{- if ne (include "ama-metrics.namespace" .) "kube-system" }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: secret-copier
  namespace: {{ include "ama-metrics.namespace" . }}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: secret-reader-copier
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: secret-copier-binding
subjects:
  - kind: ServiceAccount
    name: secret-copier
    namespace: {{ include "ama-metrics.namespace" . }}
roleRef:
  kind: ClusterRole
  name: secret-reader-copier
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: batch/v1
kind: Job
metadata:
  name: copy-addon-secret
  namespace: {{ include "ama-metrics.namespace" . }}
spec:
  template:
    spec:
      serviceAccountName: secret-copier
      restartPolicy: Never
      containers:
        - name: copier
          image: bitnami/kubectl:latest
          command:
            - /bin/sh
            - -c
            - |
              # Wait for source secret to exist
              until kubectl get secret aad-msi-auth-token -n kube-system; do
                echo "Waiting for aad-msi-auth-token secret..."
                sleep 5
              done
              
              # Copy secret
              kubectl get secret aad-msi-auth-token -n kube-system -o yaml | \
                sed 's/namespace: kube-system/namespace: {{ include "ama-metrics.namespace" . }}/' | \
                kubectl apply -f -
              
              echo "Secret copied successfully"
{{- end }}
```

#### **Option 4B: Use External Secrets Operator (Better, More Complex)**

Use an external secrets operator to sync the secret across namespaces.

#### **Option 4C: Service Account Token Volume Projection (Recommended)**

Modify pods to use projected volumes for cross-namespace access:

```yaml
# In deployment/daemonset spec
volumes:
  - name: adapter-token
    projected:
      sources:
        - serviceAccountToken:
            audience: api
            expirationSeconds: 3600
            path: token
```

But this requires modifying the adapter code to read from this path.

---

### Step 5: Update Extension Identity (Arc Only)

**File**: `templates/ama-metrics-extensionIdentity.yaml`

```yaml
{{- if .Values.AzureMonitorMetrics.ArcExtension }}
apiVersion: clusterconfig.azure.com/v1beta1
kind: ExtensionIdentity
metadata:
  name: ama-metrics-extension-identity
  namespace: {{ include "ama-metrics.namespace" . }}
spec:
  serviceAccountName: ama-metrics-serviceaccount
  serviceAccountNamespace: {{ include "ama-metrics.namespace" . }}
  # Token still expected in azure-arc namespace
  tokenNamespace: azure-arc
{{- end }}
```

---

### Step 6: Create Deployment Script

**File**: `deploy-custom-namespace.sh` (NEW FILE)

```bash
#!/bin/bash

# Configuration
CUSTOM_NAMESPACE="monitoring-system"
CLUSTER_NAME="your-cluster"
RESOURCE_GROUP="your-rg"

echo "=========================================="
echo "Custom Namespace Deployment for ama-metrics"
echo "=========================================="
echo "Target Namespace: $CUSTOM_NAMESPACE"
echo "Cluster: $CLUSTER_NAME"
echo ""

# Step 1: Ensure addon is enabled (to create secret in kube-system)
echo "Step 1: Checking if addon is enabled..."
az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP --query "azureMonitorProfile.metrics.enabled" -o tsv

if [ "$(az aks show -n $CLUSTER_NAME -g $RESOURCE_GROUP --query 'azureMonitorProfile.metrics.enabled' -o tsv)" != "true" ]; then
    echo "Enabling addon to create secret..."
    az aks update --enable-azure-monitor-metrics -n $CLUSTER_NAME -g $RESOURCE_GROUP
    sleep 30
fi

# Step 2: Verify secret exists in kube-system
echo ""
echo "Step 2: Verifying addon secret exists..."
kubectl get secret aad-msi-auth-token -n kube-system || {
    echo "ERROR: addon secret not found. Addon may not be fully deployed."
    exit 1
}

# Step 3: Create custom namespace
echo ""
echo "Step 3: Creating custom namespace..."
kubectl create namespace $CUSTOM_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Step 4: Disable the addon (optional - to prevent conflicts)
echo ""
echo "Step 4: Disabling addon to prevent conflicts (optional)..."
read -p "Disable the AKS addon? This will stop the kube-system deployment (y/n): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    az aks update --disable-azure-monitor-metrics -n $CLUSTER_NAME -g $RESOURCE_GROUP
fi

# Step 5: Deploy ARM template (for DCR/DCE only - with addon section commented out)
echo ""
echo "Step 5: Deploy ARM template for infrastructure..."
echo "Make sure to comment out the addon enablement section (lines ~160-200)"
read -p "Press enter to continue after ARM template is deployed..."

# Step 6: Install Helm chart
echo ""
echo "Step 6: Installing Helm chart to custom namespace..."

cd otelcollector/deploy/addon-chart

# Update values
cat > custom-values.yaml <<EOF
namespace: $CUSTOM_NAMESPACE

AzureMonitorMetrics:
  ImageTag: "6.24.1-main-11-14-2025-15146744"
  ImageTagWin: "6.24.1-main-11-14-2025-15146744-win"
  ImageTagTargetAllocator: "6.24.1-main-11-14-2025-15146744-targetallocator"
  ImageTagCfgReader: "6.24.1-main-11-14-2025-15146744-cfg"

global:
  commonGlobals:
    Region: "eastus"
    Customer:
      AzureResourceID: "/subscriptions/xxx/resourceGroups/xxx/providers/Microsoft.ContainerService/managedClusters/$CLUSTER_NAME"
EOF

helm upgrade --install ama-metrics ./azure-monitor-metrics-addon \
  --namespace $CUSTOM_NAMESPACE \
  --values custom-values.yaml \
  --create-namespace

# Step 7: Copy secret if needed
echo ""
echo "Step 7: Copying addon secret to custom namespace..."
kubectl get secret aad-msi-auth-token -n kube-system -o yaml | \
  sed "s/namespace: kube-system/namespace: $CUSTOM_NAMESPACE/" | \
  kubectl apply -f -

# Step 8: Verify deployment
echo ""
echo "Step 8: Verifying deployment..."
echo ""
kubectl get pods -n $CUSTOM_NAMESPACE

echo ""
echo "=========================================="
echo "Deployment complete!"
echo "=========================================="
echo ""
echo "Check logs with:"
echo "  kubectl logs -n $CUSTOM_NAMESPACE -l rsName=ama-metrics"
echo ""
echo "Remember: This is an unsupported configuration!"
```

---

### Step 7: Verification Checklist

After deployment, verify:

```bash
NAMESPACE="monitoring-system"

# 1. Check all pods are running
kubectl get pods -n $NAMESPACE

# Expected pods:
# - ama-metrics-XXXXX (ReplicaSet, 2 replicas)
# - ama-metrics-node-XXXXX (DaemonSet, 1 per node)
# - ama-metrics-ksm-XXXXX (KSM deployment)
# - ama-metrics-operator-targets-XXXXX (Target Allocator)

# 2. Check secrets exist
kubectl get secrets -n $NAMESPACE | grep -E 'aad-msi-auth-token|ama-metrics'

# 3. Check configmaps
kubectl get configmap -n $NAMESPACE

# 4. Check services
kubectl get svc -n $NAMESPACE

# 5. Check RBAC
kubectl get clusterrolebinding | grep ama-metrics

# 6. Check logs for authentication
kubectl logs -n $NAMESPACE -l rsName=ama-metrics | grep -i "auth\|token\|error"

# 7. Test metric scraping
kubectl logs -n $NAMESPACE -l rsName=ama-metrics | grep -i "scrape"
```

---

## 🚨 Common Issues and Solutions

### Issue 1: Pods Can't Authenticate

**Symptom**: Logs show authentication errors

**Solution**:
```bash
# Ensure secret was copied correctly
kubectl get secret aad-msi-auth-token -n monitoring-system -o yaml
kubectl get secret aad-msi-auth-token -n kube-system -o yaml

# Compare - they should be identical except namespace
```

### Issue 2: RBAC Permission Denied

**Symptom**: Pods can't access Kubernetes API

**Solution**:
```bash
# Verify ClusterRoleBinding points to correct namespace
kubectl get clusterrolebinding ama-metrics-clusterRoleBinding -o yaml

# Should show:
# subjects:
#   - kind: ServiceAccount
#     name: ama-metrics-serviceaccount
#     namespace: monitoring-system  # <-- Your custom namespace
```

### Issue 3: Cross-Namespace Communication

**Symptom**: Services can't communicate

**Solution**: Ensure all services use FQDN:
```yaml
# Instead of: ama-metrics-ksm
# Use: ama-metrics-ksm.monitoring-system.svc.cluster.local
```

### Issue 4: ConfigMap Not Found

**Symptom**: Pods can't find configuration

**Solution**:
```bash
# Create configmap in custom namespace
kubectl create configmap ama-metrics-settings-configmap \
  -n monitoring-system \
  --from-file=prometheus-config.yml

# Or copy from kube-system if it exists
kubectl get configmap -n kube-system ama-metrics-settings-configmap -o yaml | \
  sed 's/namespace: kube-system/namespace: monitoring-system/' | \
  kubectl apply -f -
```

---

## 📊 Complete File Modification Summary

| File | Changes Required |
|------|------------------|
| `values-template.yaml` | Add `namespace` parameter |
| `_helpers.tpl` | Add namespace helper functions |
| `ama-metrics-serviceAccount.yaml` | Parameterize namespace |
| `ama-metrics-deployment.yaml` | Namespace + secret/configmap args |
| `ama-metrics-daemonset.yaml` | Namespace + secret/configmap args |
| `ama-metrics-daemonset-win.yaml` | Namespace + secret/configmap args |
| `ama-metrics-ksm-*.yaml` (5 files) | Parameterize namespace |
| `ama-metrics-clusterRoleBinding.yaml` | Update subject namespace |
| `ama-metrics-ksm-clusterrolebinding.yaml` | Update subject namespace |
| `ama-metrics-targetallocator.yaml` | Namespace + args |
| `ama-metrics-targetallocator-service.yaml` | Parameterize namespace |
| `ama-metrics-secret.yaml` | Parameterize namespace |
| `ama-metrics-collector-hpa.yaml` | Parameterize namespace |
| `ama-metrics-pod-disruption-budget.yaml` | Parameterize namespace |
| `ama-metrics-extensionIdentity.yaml` | Update namespaces |
| `copy-addon-secret-job.yaml` | NEW: Handle secret copy |

**Total**: ~16-18 files need modification, 1-2 new files

---

## ⏱️ Estimated Time

- Initial file modifications: **2-3 hours**
- Testing and debugging: **2-4 hours**
- Documentation: **1 hour**

**Total**: ~5-8 hours for first implementation

---

## ✅ Success Criteria

You've successfully deployed to custom namespace when:

1. ✅ All pods running in custom namespace
2. ✅ Metrics flowing to Azure Monitor Workspace
3. ✅ No authentication errors in logs
4. ✅ KSM scraping cluster-wide resources
5. ✅ Node exporter metrics being collected
6. ✅ Target allocator distributing targets

---

## 🔄 Maintenance Considerations

**Important**: When upstream updates occur:
- You must manually reapply ALL namespace modifications
- Test thoroughly after each update
- Consider creating a patch/script to automate modifications
- Keep detailed documentation of what you changed

**Automation Suggestion**: Create a script that applies namespace patches:

```bash
#!/bin/bash
# apply-namespace-patches.sh

NAMESPACE="monitoring-system"

# Apply sed replacements to all template files
find templates/ -name "*.yaml" -type f -exec sed -i \
  "s/namespace: kube-system/namespace: {{ include \"ama-metrics.namespace\" . }}/g" {} \;

echo "Patches applied. Review changes with: git diff"
```

---

## 🎓 Learning Outcomes

By completing this option, you'll understand:
- Helm chart templating deeply
- Kubernetes RBAC and service accounts
- Cross-namespace secret access patterns
- Pod security and authentication flows
- The complexity of namespace isolation

---

## ⚠️ Final Warning

**This configuration is NOT supported by Microsoft**. Use for:
- ✅ Learning and experimentation
- ✅ Development environments
- ✅ Understanding the architecture
- ❌ NOT for production
- ❌ NOT for supported deployments

For production, use the standard `kube-system` deployment or request this as a feature from Microsoft.
