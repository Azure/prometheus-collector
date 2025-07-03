# Implementation Plan for New CCP Feature

<!-- CONFIGURATION SECTION - MODIFY THIS VARIABLE TO CUSTOMIZE THE FEATURE NAME -->
**NEW_CCP_FEATURE_NAME=""**

> **Instructions**: Set the `NEW_CCP_FEATURE_NAME` variable above to your desired feature name (e.g., "MyCustomFeature", "ServiceMesh", "LoadBalancer", etc.). 
> If left empty, the default name "GenericCCPFeature" will be used throughout this document.
> 
> **Naming Convention**: Use PascalCase for the feature name (e.g., "NodeAutoProvisioning", "ClusterAutoscaler").

This document outlines the complete implementation plan for adding a new Control Plane Component (CCP) feature to the Azure Monitor Prometheus Collector. The plan is based on the implementation pattern established by the Node Auto Provisioning (NAP) feature in PR #1169.

## ⚠️ IMPORTANT: User Configuration Required

After running the automation script, you **MUST** manually configure the following areas:

### 1. Container Name and Component Details
- **File**: `otelcollector/configmapparser/default-prom-configs/controlplane_${NEW_CCP_FEATURE_NAME}.yml`
- **What to change**: Replace "generic-ccp-feature-container" with your actual container name
- **Example**: For cluster autoscaler, it's "cluster-autoscaler"

### 2. Pod Label Selector
- **File**: `otelcollector/configmapparser/default-prom-configs/controlplane_${NEW_CCP_FEATURE_NAME}.yml`
- **What to change**: Update the `kubernetes_sd_configs` selector to match your component's pods
- **Example**: For cluster autoscaler, it's `app.kubernetes.io/name: cluster-autoscaler`

### 3. Metrics Port
- **File**: `otelcollector/configmapparser/default-prom-configs/controlplane_${NEW_CCP_FEATURE_NAME}.yml`
- **What to change**: Set the correct port number in the scrape config
- **Example**: For cluster autoscaler, it's port 8085

### 4. Metric Keep Lists
- **Files**: 
  - `otelcollector/configmaps/ama-metrics-settings-configmap.yaml`
  - `otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-*.yaml`
- **What to change**: Define which metrics should be collected for your component
- **Example**: Add metrics like `cluster_autoscaler_*` patterns

### 5. Test Data and Expected Outputs
- **Files**: All test files in `otelcollector/test/` directory
- **What to change**: Update test keep lists and expected configurations to match your component's metrics

### 6. Recording Rules (Optional)
- **File**: `otelcollector/configmapparser/default-prom-configs/controlplane_${NEW_CCP_FEATURE_NAME}.yml`
- **What to change**: Add any necessary recording rules for aggregation or derived metrics

## Overview

The new CCP feature (${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}) will follow the same architectural pattern as existing control plane components like `cluster-autoscaler`, `node-auto-provisioning`, `apiserver`, etc. It will be:
- **Disabled by default** for backward compatibility
- **Configurable via ConfigMap** settings
- **Support minimal ingestion profile** with essential metrics only
- **Follow existing naming conventions** and code patterns

## Implementation Phases

### Phase 1: Core Data Structures

#### 1.1 Update ConfigProcessor Structs

**File: `otelcollector/shared/configmap/mp/definitions.go`**
```go
// Add after ControlplaneEtcd field
Controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature} string
```

**File: `otelcollector/shared/configmap/ccp/tomlparser-ccp-default-scrape-settings.go`**
```go
// Add to ConfigProcessor struct
Controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature} string
```

#### 1.2 Update RegexValues Struct

**File: `otelcollector/shared/configmap/ccp/tomlparser-ccp-default-targets-metrics-keep-list.go`**
```go
// Add to RegexValues struct
Controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature} string
```

#### 1.3 Add File Path Constants

**File: `otelcollector/shared/configmap/ccp/prometheus-ccp-config-merger.go`**
```go
// Add after controlplaneEtcdDefaultFile
controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}File = defaultPromConfigPathPrefix + "controlplane_${NEW_CCP_FEATURE_NAME:-generic_ccp_feature}.yml"
```

> **Note**: For the file path constant, convert PascalCase to snake_case (e.g., "NodeAutoProvisioning" → "node_auto_provisioning").

### Phase 2: Configuration Processing Logic

#### 2.1 Environment Variable Processing

**File: `otelcollector/shared/configmap/ccp/prometheus-ccp-config-merger.go`**

Add the following block in `populateDefaultPrometheusConfig()` function:
```go
// Add after the ETCD block
if enabled, exists := os.LookupEnv("AZMON_PROMETHEUS_CONTROLPLANE_${NEW_CCP_FEATURE_NAME:-GENERIC_CCP_FEATURE}_ENABLED"); exists && strings.ToLower(enabled) == "true" && currentControllerType == replicasetControllerType {
    fmt.Println("${NEW_CCP_FEATURE_NAME:-Generic CCP Feature} enabled.")
    controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}KeepListRegex, exists := regexHash["CONTROLPLANE_${NEW_CCP_FEATURE_NAME:-GENERIC_CCP_FEATURE}_KEEP_LIST_REGEX"]
    if exists && controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}KeepListRegex != "" {
        fmt.Printf("Using regex for ${NEW_CCP_FEATURE_NAME:-Generic CCP Feature}: %s\n", controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}KeepListRegex)
        appendMetricRelabelConfig(controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}File, controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}KeepListRegex)
    }
    contents, err := os.ReadFile(controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}File)
    if err == nil {
        contents = []byte(strings.Replace(string(contents), "$$POD_NAMESPACE$$", os.Getenv("POD_NAMESPACE"), -1))
        err = os.WriteFile(controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}File, contents, fs.FileMode(0644))
    }
    defaultConfigs = append(defaultConfigs, controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature}File)
}
```

> **Note**: For environment variable names, convert PascalCase to UPPER_SNAKE_CASE (e.g., "NodeAutoProvisioning" → "NODE_AUTO_PROVISIONING").

#### 2.2 Settings Parsing

**File: `otelcollector/shared/configmap/ccp/tomlparser-ccp-default-scrape-settings.go`**

Add in `PopulateSettingValues` function:
```go
// Add after controlplane-etcd processing
if val, ok := parsedConfig["controlplane-${NEW_CCP_FEATURE_NAME:-generic-ccp-feature}"]; ok && val != "" {
    cp.Controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature} = val
    fmt.Printf("PopulateSettingValues::Using scrape settings for controlplane-${NEW_CCP_FEATURE_NAME:-generic-ccp-feature}: %v\n", cp.Controlplane${NEW_CCP_FEATURE_NAME:-GenericCCPFeature})
}
```

> **Note**: For config keys, convert PascalCase to kebab-case (e.g., "NodeAutoProvisioning" → "node-auto-provisioning").

#### 2.3 Metrics Keep List Processing

**File: `otelcollector/shared/configmap/ccp/tomlparser-ccp-default-targets-metrics-keep-list.go`**

Add minimal metrics constant:
```go
// Add after controlplaneEtcdMinMac
controlplaneGenericCCPFeatureMinMac = "generic_ccp_feature_health_status|generic_ccp_feature_request_total|generic_ccp_feature_error_total|generic_ccp_feature_duration_seconds|process_start_time_seconds"
```

Add to switch statement in `populateSettingValuesFromConfigMap`:
```go
case "controlplane-generic-ccp-feature":
    regexValues.ControlplaneGenericCCPFeature = getStringValue(value)
```

Add validation:
```go
// Add in validation section
if regexValues.ControlplaneGenericCCPFeature != "" && !shared.IsValidRegex(regexValues.ControlplaneGenericCCPFeature) {
    return regexValues, fmt.Errorf("invalid regex for controlplane-generic-ccp-feature: %s", regexValues.ControlplaneGenericCCPFeature)
}
```

Add to minimal ingestion profile:
```go
// Add in getRegexForMinimalIngestionProfile function
regexValues.ControlplaneGenericCCPFeature = controlplaneGenericCCPFeatureMinMac
```

### Phase 3: Default Prometheus Configuration

#### 3.1 Create Default Scrape Configuration

**File: `otelcollector/configmapparser/default-prom-configs/controlplane_generic_ccp_feature.yml`**

**⚠️ IMPORTANT: This template requires extensive user customization based on your specific component deployment.**

Examine existing controlplane configs for reference patterns:
- `controlplane_cluster_autoscaler.yml` (uses cluster-autoscaler container)
- `controlplane_apiserver.yml` (uses endpoints discovery)
- `controlplane_etcd.yml` (uses different TLS config)

```yaml
# TODO: This is a template that requires extensive user customization
global:
  scrape_interval: 30s
  evaluation_interval: 30s
  external_labels:
    cluster: $$CLUSTER$$

scrape_configs:
  - job_name: generic-ccp-feature
    # TODO: Configure service discovery based on your deployment
    # Options: 'pod' (most common) or 'endpoints' (for apiserver-style)
    kubernetes_sd_configs:
      - role: pod  # TODO: Change to 'endpoints' if needed
        namespaces:
          names:
            - $$POD_NAMESPACE$$
    
    # TODO: Configure authentication and TLS based on your component
    # Option 1: HTTPS with service account token (most common)
    scheme: https
    tls_config:
      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      insecure_skip_verify: true
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    
    # Option 2: HTTPS with client certificates (like cluster-autoscaler)
    # scheme: https
    # tls_config:
    #   ca_file: /etc/kubernetes/secrets/ca.pem
    #   cert_file: /etc/kubernetes/secrets/client.pem
    #   key_file: /etc/kubernetes/secrets/client-key.pem
    #   insecure_skip_verify: true
    
    # Option 3: HTTP (if your component doesn't use TLS)
    # scheme: http
    
    relabel_configs:
      # TODO: Configure pod/container selection based on your deployment
      # Real examples from existing implementations:
      # - cluster-autoscaler: app=cluster-autoscaler, container=cluster-autoscaler
      # - apiserver: k8s_app=kube-apiserver, container=kube-apiserver  
      # - karpenter: app=karpenter, container=controller
      
      # TODO: Update these label selectors for your specific component
      - source_labels: [__meta_kubernetes_pod_label_app, __meta_kubernetes_pod_container_name]
        action: keep
        regex: 'YOUR-APP-LABEL;YOUR-CONTAINER-NAME'  # TODO: Replace with actual values
      
      # TODO: Configure scraping annotations if your component uses them
      # Some use prometheus.io/scrape, others use aks_prometheus_io_scrape
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      
      # TODO: Configure custom metrics path if not /metrics
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      
      # TODO: Configure port handling based on your setup
      # Option 1: Use annotation-specified port
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: $1:$2
        target_label: __address__
      
      # Option 2: Target specific named port (uncomment and modify)
      # - source_labels: [__meta_kubernetes_pod_container_port_name]
      #   action: keep
      #   regex: 'YOUR-PORT-NAME'  # TODO: Replace with actual port name like 'https-metrics'
      
      # Standard instance labeling
      - source_labels: [__meta_kubernetes_pod_name]
        regex: (.*)
        target_label: instance
        action: replace
      
      # Add cluster label
      - target_label: cluster
        replacement: $$CLUSTER$$
    
    metric_relabel_configs:
      # TODO: Configure metric filtering for your specific component
      # Replace 'generic_ccp_feature_.*' with actual metric name patterns
      # Real examples:
      # - apiserver: 'apiserver_.*'
      # - cluster-autoscaler: 'cluster_autoscaler_.*'  
      # - karpenter: 'karpenter_.*'
      - source_labels: [__name__]
        action: keep
        regex: 'YOUR_COMPONENT_.*|process_start_time_seconds'  # TODO: Replace with actual metric patterns
      
      # TODO: Add any component-specific metric relabeling
      # Some components need host alias generation or label cleanup
      # See existing configs for examples
```

### Phase 4: Configuration Maps Updates

#### 4.1 Main Settings ConfigMap

**File: `otelcollector/configmaps/ama-metrics-settings-configmap.yaml`**

Update the controlplane-metrics section:
```yaml
controlplane-metrics: |-
  default-targets-scrape-enabled: |-
    apiserver = true
    cluster-autoscaler = false
    node-auto-provisioning = false
    generic-ccp-feature = false  # Add this line
    kube-scheduler = false
    kube-controller-manager = false
    etcd = true
  default-targets-metrics-keep-list: |-
    apiserver = ""
    cluster-autoscaler = ""
    node-auto-provisioning = ""
    generic-ccp-feature = ""  # Add this line
    kube-scheduler = ""
    kube-controller-manager = ""
    etcd = ""
```

Update the flat settings section:
```yaml
default-scrape-settings-enabled: |-
  # ... existing settings ...
  controlplane-generic-ccp-feature = false  # Add this line

default-targets-metrics-keep-list: |-
  # ... existing settings ...
  controlplane-generic-ccp-feature = ""  # Add this line
```

#### 4.2 Test Configuration Maps

**⚠️ IMPORTANT: Test configurations require real metrics from your component.**

**File: `otelcollector/test/test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml`**
```yaml
# Enable for testing
controlplane-generic-ccp-feature = true
```

**File: `otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-emptykeep.yaml`**
```yaml
# Empty keep list for testing all metrics
controlplane-generic-ccp-feature = ""
```

**File: `otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-keepmetrics.yaml`**
```yaml
# TODO: Replace with actual metrics from your component
# Examples of real metrics from existing components:
# - apiserver: "apiserver_request_total"
# - cluster-autoscaler: "rest_client_requests_total"
# - karpenter: "karpenter_nodes_created_total|karpenter_pods_scheduled_total"
controlplane-generic-ccp-feature = "YOUR_COMPONENT_METRIC_1|YOUR_COMPONENT_METRIC_2"  # TODO: Replace with real metric names
```

**File: `otelcollector/test/test-cluster-yamls/configmaps/default-config-map/ama-metrics-settings-configmap-all-targets-enabled.yaml`**
```yaml
# Enable in all-targets-enabled test
controlplane-generic-ccp-feature = true
```

**File: `otelcollector/test/test-cluster-yamls/configmaps/default-config-map/ama-metrics-settings-configmap-all-targets-disabled.yaml`**
```yaml
# Disable in all-targets-disabled test
controlplane-generic-ccp-feature = false
```

### Phase 5: Testing Updates

#### 5.1 Integration Tests

**File: `otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go`**

Update expected job lists:
```go
// Update controlplane jobs array
controlplaneJobs := []string{"apiserver", "cluster-autoscaler", "node-auto-provisioning", "generic-ccp-feature", "etcd"}
```

#### 5.2 Unit Tests

**File: `otelcollector/shared/configmap/mp/configmapparser_test.go`**

Add test environment variables:
```go
// Add to environment setup
"CONTROLPLANE_GENERIC_CCP_FEATURE_KEEP_LIST_REGEX": ".*",
"CONTROLPLANE_GENERIC_CCP_FEATURE_SCRAPE_INTERVAL": "30s",
```

Add to test configurations:
```go
// Add to default settings test
controlplane-generic-ccp-feature = true
```

**File: `otelcollector/shared/configmap/ccp/tomlparser-ccp-default-targets-metrics-keep-list_test.go`**

Add test cases for the new feature:
```go
func TestGenericCCPFeatureRegexValidation(t *testing.T) {
    // Test valid regex
    // Test invalid regex
    // Test empty regex
    // Test minimal ingestion profile
}
```

### Phase 6: Documentation Updates

#### 6.1 Feature Documentation

Create or update documentation explaining:
- Purpose and functionality of GenericCCPFeature
- Configuration options
- Default metrics collected
- How to enable/disable
- Troubleshooting guide

#### 6.2 Release Notes

Add to release notes:
- New GenericCCPFeature support
- Configuration options
- Breaking changes (if any)

### Phase 7: Validation and Testing

#### 7.1 Manual Testing Checklist

- [ ] Feature disabled by default
- [ ] Can be enabled via ConfigMap
- [ ] Minimal ingestion profile works correctly
- [ ] Custom regex validation works
- [ ] Environment variables are processed correctly
- [ ] Prometheus config file is generated correctly
- [ ] Metrics are scraped when enabled
- [ ] No metrics scraped when disabled

#### 7.2 Automated Testing

- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] End-to-end tests pass
- [ ] Performance tests (if applicable)

## Environment Variables

The following environment variables will be used:

| Variable Name | Purpose | Default |
|---------------|---------|---------|
| `AZMON_PROMETHEUS_CONTROLPLANE_GENERIC_CCP_FEATURE_ENABLED` | Enable/disable scraping | `false` |
| `CONTROLPLANE_GENERIC_CCP_FEATURE_KEEP_LIST_REGEX` | Metrics regex filter | minimal set |
| `CONTROLPLANE_GENERIC_CCP_FEATURE_SCRAPE_INTERVAL` | Scrape interval | `30s` |

## Configuration Keys

The following configuration keys will be supported:

| Configuration Key | Section | Purpose | Default |
|------------------|---------|---------|---------|
| `controlplane-generic-ccp-feature` | default-targets-scrape-enabled | Enable scraping | `false` |
| `controlplane-generic-ccp-feature` | default-targets-metrics-keep-list | Metrics filter | `""` |
| `generic-ccp-feature` | controlplane-metrics | Enable in structured config | `false` |

## Minimal Metrics Set

The minimal ingestion profile will include:
- `generic_ccp_feature_health_status` - Health/status metrics
- `generic_ccp_feature_request_total` - Request count metrics
- `generic_ccp_feature_error_total` - Error count metrics
- `generic_ccp_feature_duration_seconds` - Duration/latency metrics
- `process_start_time_seconds` - Standard process metric

## File Structure Summary

```
otelcollector/
├── configmaps/
│   └── ama-metrics-settings-configmap.yaml                    # Main config (updated)
├── configmapparser/
│   └── default-prom-configs/
│       └── controlplane_generic_ccp_feature.yml               # New file
├── shared/configmap/
│   ├── mp/
│   │   ├── definitions.go                                      # Updated
│   │   └── configmapparser_test.go                            # Updated
│   └── ccp/
│       ├── prometheus-ccp-config-merger.go                    # Updated
│       ├── tomlparser-ccp-default-scrape-settings.go          # Updated
│       └── tomlparser-ccp-default-targets-metrics-keep-list.go # Updated
└── test/
    ├── ginkgo-e2e/configprocessing/
    │   └── config_processing_test.go                           # Updated
    └── test-cluster-yamls/configmaps/
        ├── ama-metrics-settings-configmap.yaml                # Updated
        └── controlplane/
            ├── ama-metrics-settings-configmap-mipfalse-emptykeep.yaml    # Updated
            └── ama-metrics-settings-configmap-mipfalse-keepmetrics.yaml  # Updated
```

## Success Criteria

The implementation will be considered successful when:

1. **Backward Compatibility**: Existing functionality remains unchanged
2. **Default Behavior**: Feature is disabled by default
3. **Configuration**: Can be enabled/configured via ConfigMap
4. **Metrics Collection**: Collects appropriate metrics when enabled
5. **Testing**: All tests pass
6. **Documentation**: Complete documentation available
7. **Performance**: No significant impact on existing performance
8. **Security**: Follows existing security patterns

## Rollback Plan

If issues are discovered:

1. **Immediate**: Disable by default (already planned)
2. **Short-term**: Remove configuration options from ConfigMaps
3. **Long-term**: Remove code changes if necessary

## Dependencies

- No external dependencies
- Follows existing patterns established by NAP implementation
- Compatible with current Prometheus collector architecture
- Requires Kubernetes environment for full functionality

## Automated Implementation Script

To simplify the implementation process, use the provided bash script that automates all the code changes:

### Usage

```bash
# Set your feature name and run the script
export NEW_CCP_FEATURE_NAME="YourFeatureName"
./implement_new_ccp_feature.sh
```

## 📋 Post-Script Configuration Checklist

After running the automation script, use this checklist to ensure proper configuration:

### ✅ Prometheus Configuration (CRITICAL)

**File: `otelcollector/configmapparser/default-prom-configs/controlplane_${FEATURE_LOWER_SNAKE}.yml`**

- [ ] **Container Name Updated**
  - [ ] Changed from `${FEATURE_KEBAB}` to actual container name
  - [ ] Examples: `controller`, `cluster-autoscaler`, `manager`, `server`
  - [ ] Verified name matches container in component's pod spec

- [ ] **Pod Selection Configured**
  - [ ] Updated pod name regex pattern (`${FEATURE_KEBAB}.*` → actual pattern)
  - [ ] Added appropriate label selectors (app, component, tier labels)
  - [ ] Verified selectors match component's pod labels

- [ ] **Port Configuration**
  - [ ] Set correct port number/name for metrics endpoint
  - [ ] Updated scheme (http vs https) based on component's TLS config
  - [ ] Configured authentication method (bearer token, TLS, service account)

- [ ] **Metric Filtering**
  - [ ] Replaced generic `${FEATURE_LOWER_SNAKE}_.*` with actual metric patterns
  - [ ] Added essential health/status metrics
  - [ ] Included standard process metrics (uptime, CPU, memory)
  - [ ] Limited to 20-50 most important metrics for performance

### ✅ Metric Keep Lists Configuration

**Files: `otelcollector/configmaps/ama-metrics-settings-configmap.yaml` and test configs**

- [ ] **Main ConfigMap Updated**
  - [ ] Verified `controlplane-${FEATURE_KEBAB} = false` (disabled by default)
  - [ ] Verified empty keep list `controlplane-${FEATURE_KEBAB} = ""`

- [ ] **Test ConfigMaps Updated**
  - [ ] `mipfalse-keepmetrics.yaml`: Added realistic metric patterns
  - [ ] `all-targets-enabled.yaml`: Feature enabled for comprehensive testing
  - [ ] `all-targets-disabled.yaml`: Feature disabled as expected
  - [ ] Other test configs: Appropriate default settings

### ✅ Component-Specific Validation

- [ ] **Namespace Configuration**
  - [ ] Confirmed correct namespace (kube-system, custom namespace, etc.)
  - [ ] Updated `$$POD_NAMESPACE$$` placeholder if needed

- [ ] **Service Discovery Method**
  - [ ] Verified `role: pod` vs `role: endpoints` based on component architecture
  - [ ] For API server-like components: Use `role: endpoints`
  - [ ] For regular deployments: Use `role: pod`

- [ ] **Authentication & Security**
  - [ ] TLS configuration matches component's security setup
  - [ ] Bearer token authentication configured correctly
  - [ ] CA certificate path is appropriate for cluster setup

- [ ] **Recording Rules (Optional)**
  - [ ] Added any necessary aggregation rules
  - [ ] Configured alert rules if component requires monitoring alerts

### ✅ Testing & Validation

- [ ] **Syntax Validation**
  - [ ] All YAML files have valid syntax
  - [ ] Go code compiles without errors
  - [ ] No template variables left unreplaced

- [ ] **Unit Tests**
  - [ ] All existing tests pass
  - [ ] New test cases added for the feature
  - [ ] Mock configurations work as expected

- [ ] **Integration Tests**
  - [ ] E2E tests pass with new feature
  - [ ] ConfigMap processing works correctly
  - [ ] Environment variable handling functions properly

- [ ] **Manual Testing**
  - [ ] Deployed in test environment
  - [ ] Metrics are successfully scraped
  - [ ] Keep list filtering works as expected
  - [ ] Feature can be enabled/disabled via configuration

### ✅ Documentation & Code Quality

- [ ] **Code Comments**
  - [ ] Added descriptive comments for new structs/functions
  - [ ] Updated existing comments where logic changed
  - [ ] Documented any special configuration requirements

- [ ] **Error Handling**
  - [ ] Proper error messages for configuration issues
  - [ ] Graceful handling of missing/invalid settings
  - [ ] Logging includes sufficient detail for troubleshooting

- [ ] **Naming Consistency**
  - [ ] All variable names follow established patterns
  - [ ] Configuration keys use consistent naming convention
  - [ ] File names match established conventions

### ⚠️ Common Issues to Check

- [ ] **Metric Name Patterns**
  - [ ] No typos in regex patterns
  - [ ] Patterns match actual component metrics (test with `curl /metrics`)
  - [ ] Special characters properly escaped in regex

- [ ] **Label Selectors**
  - [ ] Pod labels exactly match component's actual labels
  - [ ] No spaces or special characters in label values
  - [ ] Case sensitivity handled correctly

- [ ] **Port Configuration**
  - [ ] Port number matches component's actual metrics port
  - [ ] Named ports match exact names in component's service/pod spec
  - [ ] HTTP/HTTPS scheme matches component's configuration

- [ ] **Environment Variables**
  - [ ] Variable names follow AZMON_PROMETHEUS_CONTROLPLANE_* pattern
  - [ ] UPPER_SNAKE_CASE conversion is correct
  - [ ] No conflicts with existing environment variables

### 🧪 Final Validation Commands

```bash
# Validate YAML syntax
yamllint otelcollector/configmaps/ama-metrics-settings-configmap.yaml

# Check Go code compilation
cd otelcollector && go build ./...

# Run unit tests
cd otelcollector && go test ./...

# Validate Prometheus config
promtool check config otelcollector/configmapparser/default-prom-configs/controlplane_${FEATURE_LOWER_SNAKE}.yml

# Test metric scraping (in test environment)
kubectl port-forward <component-pod> <port>:<port> -n <namespace>
curl http://localhost:<port>/metrics | grep -E "your_metric_pattern"
```

> **Remember**: The automation script creates templates that MUST be customized. A generic implementation will not work with real components. Always test thoroughly in a development environment before deploying to production.
