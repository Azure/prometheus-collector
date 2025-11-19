# Prometheus Agent Namespace Migration Plan

## Document Information
- **Date**: November 13, 2025
- **Migration Type**: Namespace Change
- **From**: `kube-system`
- **To**: `azure-managed-prometheus`
- **Status**: Planning Phase

---

## Executive Summary

This document outlines the plan to migrate the Azure Monitor Metrics Prometheus agent (ama-metrics) from the `kube-system` namespace to a dedicated `azure-managed-prometheus` namespace.

### Business Justification

**Security Requirement**: When Istio service mesh is enabled on a cluster, it issues mTLS certificates to pods for secure service-to-service communication. Currently, because the Prometheus agent runs in `kube-system`, Istio issues certificates to ALL pods in the `kube-system` namespace, not just the Prometheus agent. This violates the principle of least privilege and creates unnecessary security exposure.

**Solution**: By moving the Prometheus agent to its own dedicated namespace (`azure-managed-prometheus`), Istio will only issue mTLS certificates to the Prometheus agent components, ensuring proper security isolation.

### Migration Impact

- **Scope**: 45+ YAML files across Helm charts, test fixtures, and configurations
- **Breaking Change**: Yes - requires cluster redeployment
- **Downtime**: Minimal if properly coordinated
- **Customer Impact**: Existing deployments must be updated; monitoring dashboards may need namespace updates

---

## Technical Background

### Istio mTLS Certificate Scoping

When Istio is enabled:
1. Istio's sidecar injector automatically injects Envoy proxies into pods
2. Istio CA issues mTLS certificates to these sidecars for mutual TLS
3. Certificate scoping is namespace-based
4. All pods in a namespace receive certificates when any pod needs them

**Current Problem**:
```
kube-system namespace:
├── Prometheus Agent (needs mTLS for scraping)
├── kube-proxy (gets unnecessary certificates)
├── coredns (gets unnecessary certificates)
├── metrics-server (gets unnecessary certificates)
└── ... other system components (all get certificates)
```

**After Migration**:
```
azure-managed-prometheus namespace:
├── Prometheus Agent (gets mTLS certificates)
└── (only Prometheus components)

kube-system namespace:
├── kube-proxy (no certificates)
├── coredns (no certificates)
└── ... (isolated from Istio certificate distribution)
```

---

## Current Architecture

### Components Deployed in kube-system

| Component | Type | Purpose |
|-----------|------|---------|
| `ama-metrics` | Deployment | Main Prometheus collector (ReplicaSet) |
| `ama-metrics-node` | DaemonSet | Node-level metrics collector (Linux) |
| `ama-metrics-win-node` | DaemonSet | Node-level metrics collector (Windows) |
| `ama-metrics-ksm` | Deployment | Kube-State-Metrics |
| `ama-metrics-operator-targets` | Deployment | Target Allocator (when operator enabled) |
| `ama-metrics-hpa` | HPA | Horizontal Pod Autoscaler for collector |
| `ama-metrics-pdb` | PDB | Pod Disruption Budget |

### Supporting Resources

- **ServiceAccounts**: `ama-metrics-serviceaccount`, `ama-metrics-ksm`
- **ClusterRoles**: `ama-metrics`, `ama-metrics-ksm`, `ama-metrics-ccp-role`
- **ClusterRoleBindings**: Multiple bindings for RBAC
- **ConfigMaps**: 
  - `ama-metrics-settings-configmap`
  - `ama-metrics-prometheus-config`
  - `ama-metrics-prometheus-config-node`
  - `ama-metrics-prometheus-config-node-windows`
- **Secrets**: 
  - `ama-metrics-proxy-config`
  - `ama-metrics-proxy-cert`
  - `ama-metrics-mtls-secret`
  - `ama-metrics-operator-targets-client-tls-secret`
- **Services**:
  - `ama-metrics-ksm` (KSM service)
  - `ama-metrics-operator-targets` (Target Allocator service)

---

## Migration Scope

### Files Requiring Updates

#### Phase 1: Helm Chart Templates (Critical Priority)

**Location**: `prometheus-collector/otelcollector/deploy/addon-chart/azure-monitor-metrics-addon/templates/`

1. **Workload Resources**:
   - `ama-metrics-deployment.yaml` - Update namespace metadata
   - `ama-metrics-daemonset.yaml` - Update namespace metadata
   - `ama-metrics-ksm-deployment.yaml` - Update namespace metadata
   - `ama-metrics-targetallocator.yaml` - Update namespace metadata
   - `ama-metrics-collector-hpa.yaml` - Update namespace and target reference

2. **RBAC Resources**:
   - `ama-metrics-serviceAccount.yaml` - Update namespace
   - `ama-metrics-clusterRoleBinding.yaml` - Update subjects[].namespace
   - `ama-metrics-ksm-serviceaccount.yaml` - Update namespace
   - `ama-metrics-ksm-clusterrolebinding.yaml` - Update subjects[].namespace

3. **Configuration Resources**:
   - `ama-metrics-secret.yaml` - Update namespace
   - `ama-metrics-extensionIdentity.yaml` - Update namespace references

4. **Service Resources**:
   - `ama-metrics-ksm-service.yaml` - Update namespace
   - `ama-metrics-targetallocator-service.yaml` - Update namespace

5. **Policy Resources**:
   - `ama-metrics-pod-disruption-budget.yaml` - Update namespace

6. **New Resource**:
   - `ama-metrics-namespace.yaml` - **CREATE NEW** to define namespace

#### Phase 2: ConfigMaps (High Priority)

**Location**: `prometheus-collector/otelcollector/configmaps/`

- `ama-metrics-prometheus-config-configmap.yaml`
- `ama-metrics-prometheus-config-node-configmap.yaml`
- `ama-metrics-prometheus-config-node-windows-configmap.yaml`
- `ama-metrics-settings-configmap.yaml`
- `ama-metrics-settings-configmap-v1.yaml`
- `ama-metrics-settings-configmap-v2.yaml`
- `ama-metrics-settings-configmap-otel.yaml`

#### Phase 3: Test Fixtures (Medium Priority)

**Location**: `prometheus-collector/otelcollector/test/test-cluster-yamls/configmaps/`

Update all test ConfigMaps (40+ files):
- `default-config-map/` - All test variants
- `custom-config-map/` - Custom configuration tests
- `custom-config-map-node/` - Node configuration tests
- `custom-config-map-win/` - Windows configuration tests
- `global-settings/` - Global setting tests
- `controlplane/` - Control plane tests

#### Phase 4: Arc Extension & CCP Plugin (High Priority)

**Location**: `prometheus-collector/otelcollector/deploy/addon-chart/ccp-metrics-plugin/templates/`

- `ama-metrics-role.yaml` - Control plane role
- `ama-metrics-roleBinding.yaml` - Control plane binding

#### Phase 5: Retina Integration (Medium Priority)

**Location**: `prometheus-collector/otelcollector/deploy/retina/custom-files/`

- `network-observability-service.yaml` - Network observability service

---

## Implementation Details

### 1. Namespace Resource Creation

Create new file: `ama-metrics-namespace.yaml`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: azure-managed-prometheus
  labels:
    name: azure-managed-prometheus
    kubernetes.azure.com/managedby: aks
    istio-injection: enabled  # Enable Istio sidecar injection
```

### 2. Metadata Updates

**Pattern**: Update namespace field in all resource metadata

**Before**:
```yaml
metadata:
  name: ama-metrics
  namespace: kube-system
```

**After**:
```yaml
metadata:
  name: ama-metrics
  namespace: azure-managed-prometheus
```

### 3. ClusterRoleBinding Updates

**Pattern**: Update subject namespace references

**Before**:
```yaml
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: kube-system
```

**After**:
```yaml
subjects:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: azure-managed-prometheus
```

### 4. Service Discovery Updates

**Pattern**: Update service FQDN references for cross-namespace communication

**Before**:
```yaml
- name: KUBE_STATE_NAME
  value: ama-metrics-ksm
```

**After** (if using FQDN):
```yaml
- name: KUBE_STATE_NAME
  value: ama-metrics-ksm.azure-managed-prometheus.svc.cluster.local
```

**Note**: In most cases, the short name works within the same namespace. FQDN is only needed for cross-namespace references.

### 5. Addon Token Adapter Updates

**Pattern**: Update secret namespace argument

**Before**:
```yaml
args:
  - --secret-namespace=kube-system
  - --secret-name=aad-msi-auth-token
```

**After**:
```yaml
args:
  - --secret-namespace=azure-managed-prometheus
  - --secret-name=aad-msi-auth-token
```

### 6. Arc Extension Identity Updates

**Pattern**: Update tokenNamespace if applicable

**Before** (if present):
```yaml
spec:
  tokenNamespace: azure-arc
```

**After**: Review and verify Arc extension continues to work across namespaces

### 7. Lookup Function Updates (Helm)

**Pattern**: Update namespace in Helm lookup functions

**Before**:
```yaml
{{- $currentSpec := (lookup "apps/v1" "Deployment" "kube-system" "ama-metrics").spec }}
```

**After**:
```yaml
{{- $currentSpec := (lookup "apps/v1" "Deployment" "azure-managed-prometheus" "ama-metrics").spec }}
```

---

## Implementation Phases

### Phase 1: Helm Chart Core Updates (Week 1)

**Objective**: Update primary Helm chart templates

**Tasks**:
1. Create namespace resource template
2. Update all deployment/daemonset metadata
3. Update ServiceAccount and ClusterRoleBinding namespaces
4. Update ConfigMap and Secret namespaces
5. Update Service resources
6. Update HPA and PDB resources
7. Update Helm lookup functions

**Files**: ~20 template files

**Testing**:
- Helm template rendering validation
- Lint checking
- Local cluster deployment test

### Phase 2: ConfigMap & Settings Updates (Week 1-2)

**Objective**: Update configuration resources

**Tasks**:
1. Update standalone ConfigMap files
2. Verify prometheus scrape configurations
3. Update settings ConfigMaps (all versions)
4. Test configuration parsing

**Files**: ~10 ConfigMap files

**Testing**:
- Configuration validation
- Prometheus config syntax check
- Settings schema validation

### Phase 3: Test Fixtures Updates (Week 2)

**Objective**: Update all test YAML files

**Tasks**:
1. Update test ConfigMaps in test-cluster-yamls/
2. Update test scenarios
3. Update validation scripts
4. Re-run test suite

**Files**: ~40 test files

**Testing**:
- Test suite execution
- Validation script verification
- E2E test scenarios

### Phase 4: Arc Extension & Special Cases (Week 2-3)

**Objective**: Handle Arc-specific configurations

**Tasks**:
1. Update CCP metrics plugin templates
2. Update Retina integration
3. Verify Arc extension compatibility
4. Test with Arc-connected clusters

**Files**: ~5 special case files

**Testing**:
- Arc cluster deployment
- MSI adapter functionality
- Extension identity validation

### Phase 5: Documentation & Validation (Week 3)

**Objective**: Complete documentation and final validation

**Tasks**:
1. Update deployment documentation
2. Update README files
3. Create migration guide for customers
4. Final integration testing
5. Performance validation

**Deliverables**:
- Migration documentation
- Customer communication guide
- Rollback procedures

---

## Validation & Testing

### Pre-Migration Validation

**Checklist**:
- [ ] Backup current cluster configuration
- [ ] Document current metrics collection state
- [ ] Verify Istio version compatibility
- [ ] Check for any custom namespace policies
- [ ] Review network policies that might affect new namespace
- [ ] Identify all dashboards/alerts referencing kube-system

### Post-Migration Validation

**Functionality Tests**:

1. **Pod Status**:
   ```bash
   kubectl get pods -n azure-managed-prometheus
   # Verify all pods are Running
   ```

2. **Service Discovery**:
   ```bash
   kubectl get svc -n azure-managed-prometheus
   # Verify services are accessible
   ```

3. **Metrics Collection**:
   ```bash
   kubectl logs -n azure-managed-prometheus deployment/ama-metrics
   # Check for successful scraping logs
   ```

4. **Target Discovery**:
   ```bash
   kubectl logs -n azure-managed-prometheus deployment/ama-metrics-operator-targets
   # Verify target allocator finds targets
   ```

5. **Istio Certificate Validation**:
   ```bash
   # Verify certificates are only in azure-managed-prometheus
   kubectl get pods -n azure-managed-prometheus -o json | jq '.items[].spec.containers[].name'
   # Should show istio-proxy sidecar
   
   kubectl get pods -n kube-system -o json | jq '.items[].spec.containers[].name'
   # Should NOT show istio-proxy for non-Prometheus pods
   ```

6. **RBAC Verification**:
   ```bash
   kubectl auth can-i list nodes --as=system:serviceaccount:azure-managed-prometheus:ama-metrics-serviceaccount
   # Should return "yes"
   ```

7. **Cross-Namespace Communication**:
   ```bash
   # Test KSM service accessibility
   kubectl run test-pod --image=curlimages/curl -n azure-managed-prometheus --rm -it -- \
     curl http://ama-metrics-ksm:8080/metrics
   ```

**Performance Tests**:
- Compare metrics ingestion rate before/after
- Verify memory and CPU usage is comparable
- Check metrics delay/latency
- Validate scrape success rate

**Security Validation**:
- Confirm Istio certificates are scoped correctly
- Verify no certificate leakage to kube-system
- Validate mTLS between Prometheus and targets works
- Check secret access is properly restricted

### Automated Test Suite

**Unit Tests**:
- Helm template rendering tests
- ConfigMap parsing tests
- RBAC permission tests

**Integration Tests**:
- Full deployment test on test cluster
- Metrics collection validation
- Target discovery verification
- Service mesh integration test

**E2E Tests**:
- Complete scraping workflow
- Alert generation
- Dashboard visualization
- Multi-cluster scenarios

---

## Rollout Strategy

### Recommended Deployment Sequence

**Stage 1: Development Clusters (Week 3)**
- Deploy to internal dev/test clusters
- Validate basic functionality
- Test Istio integration
- Gather performance metrics

**Stage 2: Canary Clusters (Week 4)**
- Deploy to small subset of production clusters
- Monitor for 1 week
- Collect customer feedback
- Identify any edge cases

**Stage 3: Progressive Rollout (Weeks 5-8)**
- 10% of clusters - Week 5
- 25% of clusters - Week 6
- 50% of clusters - Week 7
- 100% of clusters - Week 8

**Stage 4: Legacy Cleanup (Week 9+)**
- Remove old kube-system deployments
- Clean up old documentation
- Update all external references

### Deployment Methods

**Method 1: Helm Upgrade (Recommended)**
```bash
# This will be a breaking change requiring full redeployment
helm upgrade azure-monitor-metrics ./azure-monitor-metrics-addon \
  --namespace azure-managed-prometheus \
  --create-namespace \
  --values values.yaml
```

**Method 2: GitOps (Flux/ArgoCD)**
- Update manifest repository
- Let GitOps controllers apply changes
- Monitor sync status

**Method 3: Azure Policy/Extension**
- Update Arc extension definition
- Update AKS addon configuration
- Let platform handle rollout

---

## Rollback Plan

### Rollback Triggers

Rollback if:
- Metrics collection failure > 5%
- Pod crash loop in > 10% deployments
- Istio integration issues
- RBAC permission failures
- Customer-reported service disruption

### Rollback Procedure

**Step 1: Immediate Revert**
```bash
# Revert to previous Helm release
helm rollback azure-monitor-metrics -n azure-managed-prometheus

# Or redeploy to kube-system
helm upgrade azure-monitor-metrics ./azure-monitor-metrics-addon \
  --namespace kube-system \
  --values values-previous.yaml
```

**Step 2: Cleanup**
```bash
# Remove new namespace if needed
kubectl delete namespace azure-managed-prometheus
```

**Step 3: Verification**
- Verify all pods back in kube-system
- Check metrics collection resumed
- Validate dashboards working

**Step 4: Post-Mortem**
- Document what went wrong
- Update migration plan
- Plan remediation

### Rollback Testing

- Test rollback procedure in dev environment
- Document expected rollback time (target: < 15 minutes)
- Ensure rollback playbook is accessible to on-call team

---

## Customer Communication

### Pre-Migration Communication

**Timeline**: 2 weeks before migration

**Content**:
```
Subject: Upcoming Prometheus Agent Namespace Migration

Dear Azure Monitor Users,

We will be migrating the Azure Monitor Metrics Prometheus agent from the 
kube-system namespace to a new dedicated namespace: azure-managed-prometheus.

WHY: This migration improves security isolation when using Istio service mesh 
by ensuring mTLS certificates are only issued to Prometheus components.

WHEN: [Specific date/time range]

IMPACT: 
- Prometheus agent pods will restart in new namespace
- Brief metrics collection interruption possible (< 5 minutes)
- Dashboards/alerts referencing 'kube-system' need updating

ACTION REQUIRED:
1. Update any custom dashboards to use namespace: azure-managed-prometheus
2. Update any alerts referencing the old namespace
3. Review any network policies that might affect the new namespace

For questions, contact: [support contact]
```

### Post-Migration Communication

**Timeline**: Immediately after successful migration

**Content**:
```
Subject: Prometheus Agent Namespace Migration - Complete

The Prometheus agent has been successfully migrated to the 
azure-managed-prometheus namespace.

VERIFICATION:
Run: kubectl get pods -n azure-managed-prometheus

DASHBOARD UPDATES:
Update namespace filters from 'kube-system' to 'azure-managed-prometheus'

SUPPORT:
If you experience any issues, please contact [support contact]
```

---

## Known Issues & Edge Cases

### Issue 1: Custom Network Policies

**Symptom**: Metrics collection fails after migration
**Cause**: Network policies blocking new namespace
**Solution**: Update network policies to allow azure-managed-prometheus

### Issue 2: PodSecurityPolicy/PodSecurityStandards

**Symptom**: Pods fail to start due to security policy violations
**Cause**: Namespace-specific security policies
**Solution**: Apply equivalent security policies to new namespace

### Issue 3: Resource Quotas

**Symptom**: Deployment fails due to resource limits
**Cause**: Namespace has restrictive resource quotas
**Solution**: Apply appropriate resource quotas to new namespace

### Issue 4: Custom ServiceMonitor/PodMonitor

**Symptom**: Custom scrape targets not discovered
**Cause**: ServiceMonitor/PodMonitor namespace selectors
**Solution**: Update custom monitors to include new namespace

### Issue 5: Grafana Dashboard Queries

**Symptom**: Dashboards show no data
**Cause**: PromQL queries filtering on namespace=kube-system
**Solution**: Update dashboard queries to use azure-managed-prometheus

---

## FAQ

**Q: Why not make the namespace configurable?**
A: The Istio security requirement makes dedicated namespace mandatory. Configurability would defeat the security purpose.

**Q: Will this affect clusters without Istio?**
A: No negative impact. The migration improves security posture for all clusters.

**Q: What about existing metrics history?**
A: Historical metrics are preserved. Only namespace label changes in new metrics.

**Q: How long will the migration take per cluster?**
A: Estimated 5-15 minutes depending on cluster size and rollout method.

**Q: Can we run both old and new simultaneously?**
A: Not recommended. Could cause duplicate metric collection and confusion.

**Q: What about Arc-connected clusters?**
A: Arc extension fully supports the new namespace. MSI adapter works across namespaces.

**Q: Will monitoring cost change?**
A: No cost change expected. Same metrics collected, just from different namespace.

**Q: What if a cluster has custom Prometheus configuration?**
A: Custom configs must be updated to reference new namespace for any namespace-specific settings.

---

## References

### Internal Documentation
- [Prometheus Collector Architecture](./prometheus-collector/README.md)
- [Helm Chart Documentation](./prometheus-collector/otelcollector/deploy/addon-chart/README.md)
- [Testing Guide](./prometheus-collector/test/README.md)

### External Resources
- [Istio Security - mTLS](https://istio.io/latest/docs/concepts/security/#mutual-tls-authentication)
- [Kubernetes Namespaces Best Practices](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
- [Azure Monitor Metrics Documentation](https://learn.microsoft.com/azure/azure-monitor/containers/prometheus-metrics-enable)

### Related Work Items
- [Link to ADO work item]
- [Link to security review]
- [Link to architecture decision record]

---

## Appendix A: Complete File List

### Helm Chart Templates (20 files)
1. `ama-metrics-namespace.yaml` (NEW)
2. `ama-metrics-deployment.yaml`
3. `ama-metrics-daemonset.yaml`
4. `ama-metrics-ksm-deployment.yaml`
5. `ama-metrics-targetallocator.yaml`
6. `ama-metrics-collector-hpa.yaml`
7. `ama-metrics-serviceAccount.yaml`
8. `ama-metrics-clusterRoleBinding.yaml`
9. `ama-metrics-ksm-serviceaccount.yaml`
10. `ama-metrics-ksm-clusterrolebinding.yaml`
11. `ama-metrics-secret.yaml`
12. `ama-metrics-extensionIdentity.yaml`
13. `ama-metrics-ksm-service.yaml`
14. `ama-metrics-targetallocator-service.yaml`
15. `ama-metrics-pod-disruption-budget.yaml`
16. `ama-metrics-clusterRole.yaml`
17. `ama-metrics-ksm-role.yaml`
18. `ama-metrics-podmonitor-crd.yaml`
19. `ama-metrics-servicemonitor-crd.yaml`
20. `ama-metrics-scc.yaml`

### ConfigMap Files (10 files)
1. `ama-metrics-prometheus-config-configmap.yaml`
2. `ama-metrics-prometheus-config-node-configmap.yaml`
3. `ama-metrics-prometheus-config-node-windows-configmap.yaml`
4. `ama-metrics-settings-configmap.yaml`
5. `ama-metrics-settings-configmap-v1.yaml`
6. `ama-metrics-settings-configmap-v2.yaml`
7. `ama-metrics-settings-configmap-otel.yaml`

### CCP Plugin Files (2 files)
1. `ccp-metrics-plugin/templates/ama-metrics-role.yaml`
2. `ccp-metrics-plugin/templates/ama-metrics-roleBinding.yaml`

### Retina Files (1 file)
1. `retina/custom-files/network-observability-service.yaml`

### Test ConfigMaps (~40 files)
All files in `test/test-cluster-yamls/configmaps/` subdirectories

---

## Appendix B: Search & Replace Patterns

### Simple Text Replacement

**Pattern 1: Metadata Namespace**
```
FIND: namespace: kube-system
REPLACE: namespace: azure-managed-prometheus
```

**Pattern 2: ClusterRoleBinding Subjects**
```
FIND: 
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: kube-system

REPLACE:
  - kind: ServiceAccount
    name: ama-metrics-serviceaccount
    namespace: azure-managed-prometheus
```

**Pattern 3: Helm Lookup Function**
```
FIND: (lookup "apps/v1" "Deployment" "kube-system"
REPLACE: (lookup "apps/v1" "Deployment" "azure-managed-prometheus"
```

**Pattern 4: Addon Token Adapter Args**
```
FIND: --secret-namespace=kube-system
REPLACE: --secret-namespace=azure-managed-prometheus
```

### Regex Patterns (for automated tooling)

```regex
# Find all namespace: kube-system in YAML files
namespace:\s+kube-system

# Find namespace in subject sections
subjects:\s*\n\s*-\s*kind:\s*ServiceAccount\s*\n\s*name:\s*ama-metrics.*\n\s*namespace:\s*kube-system

# Find Helm lookup with kube-system
lookup\s+"[^"]+"\s+"[^"]+"\s+"kube-system"
```

---

## Sign-off

### Planning Phase
- [ ] Architecture Review: _________________ Date: _______
- [ ] Security Review: _________________ Date: _______
- [ ] Engineering Lead: _________________ Date: _______

### Implementation Phase
- [ ] Code Review Complete: _________________ Date: _______
- [ ] Testing Complete: _________________ Date: _______
- [ ] Documentation Complete: _________________ Date: _______

### Deployment Phase
- [ ] Dev Deployment: _________________ Date: _______
- [ ] Canary Deployment: _________________ Date: _______
- [ ] Production Rollout: _________________ Date: _______

---

**Document Version**: 1.0  
**Last Updated**: November 13, 2025  
**Owner**: Azure Monitor Metrics Team  
**Contact**: [Team contact information]
