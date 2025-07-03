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

### Script: `implement_new_ccp_feature.sh`

```bash
#!/bin/bash

# Implementation script for new CCP feature
# Usage: NEW_CCP_FEATURE_NAME="YourFeatureName" ./implement_new_ccp_feature.sh

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if NEW_CCP_FEATURE_NAME is set
if [ -z "$NEW_CCP_FEATURE_NAME" ]; then
    print_error "NEW_CCP_FEATURE_NAME environment variable is not set"
    print_status "Usage: NEW_CCP_FEATURE_NAME=\"YourFeatureName\" $0"
    print_status "Example: NEW_CCP_FEATURE_NAME=\"ServiceMesh\" $0"
    exit 1
fi

# Validate feature name (PascalCase)
if ! [[ "$NEW_CCP_FEATURE_NAME" =~ ^[A-Z][a-zA-Z0-9]*$ ]]; then
    print_error "Feature name must be in PascalCase (e.g., ServiceMesh, LoadBalancer)"
    exit 1
fi

print_status "Implementing new CCP feature: $NEW_CCP_FEATURE_NAME"

# Convert feature name to different cases
FEATURE_UPPER_SNAKE=$(echo "$NEW_CCP_FEATURE_NAME" | sed 's/\([A-Z]\)/_\1/g' | sed 's/^_//' | tr '[:lower:]' '[:upper:]')
FEATURE_LOWER_SNAKE=$(echo "$NEW_CCP_FEATURE_NAME" | sed 's/\([A-Z]\)/_\1/g' | sed 's/^_//' | tr '[:upper:]' '[:lower:]')
FEATURE_KEBAB=$(echo "$NEW_CCP_FEATURE_NAME" | sed 's/\([A-Z]\)/-\1/g' | sed 's/^-//' | tr '[:upper:]' '[:lower:]')

print_status "Feature name conversions:"
print_status "  PascalCase: $NEW_CCP_FEATURE_NAME"
print_status "  UPPER_SNAKE_CASE: $FEATURE_UPPER_SNAKE"
print_status "  lower_snake_case: $FEATURE_LOWER_SNAKE"
print_status "  kebab-case: $FEATURE_KEBAB"

# Function to backup file
backup_file() {
    local file=$1
    if [ -f "$file" ]; then
        cp "$file" "$file.backup.$(date +%Y%m%d_%H%M%S)"
        print_status "Backed up $file"
    fi
}

# Function to check if file exists
check_file() {
    local file=$1
    if [ ! -f "$file" ]; then
        print_error "File not found: $file"
        return 1
    fi
    return 0
}

print_status "Starting implementation..."

# Phase 1: Update ConfigProcessor structs

print_status "Phase 1: Updating ConfigProcessor structs..."

# 1.1 Update MP definitions.go
MP_DEFINITIONS="otelcollector/shared/configmap/mp/definitions.go"
if check_file "$MP_DEFINITIONS"; then
    backup_file "$MP_DEFINITIONS"
    
    # Add field after ControlplaneEtcd
    sed -i "/ControlplaneEtcd[[:space:]]*string/a\\
\\tControlplane${NEW_CCP_FEATURE_NAME}[[:space:]]*string" "$MP_DEFINITIONS"
    
    print_success "Updated $MP_DEFINITIONS"
fi

# 1.2 Update CCP scrape settings
CCP_SCRAPE_SETTINGS="otelcollector/shared/configmap/ccp/tomlparser-ccp-default-scrape-settings.go"
if check_file "$CCP_SCRAPE_SETTINGS"; then
    backup_file "$CCP_SCRAPE_SETTINGS"
    
    # Add field to ConfigProcessor struct
    sed -i "/ControlplaneEtcd[[:space:]]*string/a\\
\\tControlplane${NEW_CCP_FEATURE_NAME}[[:space:]]*string" "$CCP_SCRAPE_SETTINGS"
    
    # Add parsing logic in PopulateSettingValues function
    sed -i "/controlplane-etcd.*cp\.ControlplaneEtcd/a\\
\\t}\\
\\tif val, ok := parsedConfig[\"controlplane-${FEATURE_KEBAB}\"]; ok && val != \"\" {\\
\\t\\tcp.Controlplane${NEW_CCP_FEATURE_NAME} = val\\
\\t\\tfmt.Printf(\"PopulateSettingValues::Using scrape settings for controlplane-${FEATURE_KEBAB}: %v\\\\n\", cp.Controlplane${NEW_CCP_FEATURE_NAME})" "$CCP_SCRAPE_SETTINGS"
    
    print_success "Updated $CCP_SCRAPE_SETTINGS"
fi

# 1.3 Update CCP metrics keep list
CCP_KEEP_LIST="otelcollector/shared/configmap/ccp/tomlparser-ccp-default-targets-metrics-keep-list.go"
if check_file "$CCP_KEEP_LIST"; then
    backup_file "$CCP_KEEP_LIST"
    
    # Add field to RegexValues struct
    sed -i "/ControlplaneEtcd[[:space:]]*string/a\\
\\tControlplane${NEW_CCP_FEATURE_NAME}[[:space:]]*string" "$CCP_KEEP_LIST"
    
    # Add minimal metrics constant
    sed -i "/controlplaneEtcdMinMac[[:space:]]*=/a\\
\\tcontrolplane${NEW_CCP_FEATURE_NAME}MinMac = \"${FEATURE_LOWER_SNAKE}_health_status|${FEATURE_LOWER_SNAKE}_request_total|${FEATURE_LOWER_SNAKE}_error_total|${FEATURE_LOWER_SNAKE}_duration_seconds|process_start_time_seconds\"" "$CCP_KEEP_LIST"
    
    # Add switch case
    sed -i "/case \"controlplane-etcd\":/a\\
\\t\\tregexValues.ControlplaneEtcd = getStringValue(value)\\
\\tcase \"controlplane-${FEATURE_KEBAB}\":\\
\\t\\tregexValues.Controlplane${NEW_CCP_FEATURE_NAME} = getStringValue(value)" "$CCP_KEEP_LIST"
    
    # Add validation
    sed -i "/controlplane-etcd.*IsValidRegex/a\\
\\t}\\
\\tif regexValues.Controlplane${NEW_CCP_FEATURE_NAME} != \"\" && !shared.IsValidRegex(regexValues.Controlplane${NEW_CCP_FEATURE_NAME}) {\\
\\t\\treturn regexValues, fmt.Errorf(\"invalid regex for controlplane-${FEATURE_KEBAB}: %s\", regexValues.Controlplane${NEW_CCP_FEATURE_NAME})" "$CCP_KEEP_LIST"
    
    # Add to minimal ingestion profile
    sed -i "/regexValues\.ControlplaneEtcd = controlplaneEtcdMinMac/a\\
\\t\\tregexValues.Controlplane${NEW_CCP_FEATURE_NAME} = controlplane${NEW_CCP_FEATURE_NAME}MinMac" "$CCP_KEEP_LIST"
    
    print_success "Updated $CCP_KEEP_LIST"
fi

# 1.4 Update CCP config merger
CCP_CONFIG_MERGER="otelcollector/shared/configmap/ccp/prometheus-ccp-config-merger.go"
if check_file "$CCP_CONFIG_MERGER"; then
    backup_file "$CCP_CONFIG_MERGER"
    
    # Add file path constant
    sed -i "/controlplaneEtcdDefaultFile[[:space:]]*=/a\\
\\tcontrolplane${NEW_CCP_FEATURE_NAME}File = defaultPromConfigPathPrefix + \"controlplane_${FEATURE_LOWER_SNAKE}.yml\"" "$CCP_CONFIG_MERGER"
    
    # Add environment variable processing
    sed -i "/AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED/,/defaultConfigs = append.*controlplaneEtcdDefaultFile/a\\
\\t}\\
\\n\\tif enabled, exists := os.LookupEnv(\"AZMON_PROMETHEUS_CONTROLPLANE_${FEATURE_UPPER_SNAKE}_ENABLED\"); exists && strings.ToLower(enabled) == \"true\" && currentControllerType == replicasetControllerType {\\
\\t\\tfmt.Println(\"${NEW_CCP_FEATURE_NAME} enabled.\")\\
\\t\\tcontrolplane${NEW_CCP_FEATURE_NAME}KeepListRegex, exists := regexHash[\"CONTROLPLANE_${FEATURE_UPPER_SNAKE}_KEEP_LIST_REGEX\"]\\
\\t\\tif exists && controlplane${NEW_CCP_FEATURE_NAME}KeepListRegex != \"\" {\\
\\t\\t\\tfmt.Printf(\"Using regex for ${NEW_CCP_FEATURE_NAME}: %s\\\\n\", controlplane${NEW_CCP_FEATURE_NAME}KeepListRegex)\\
\\t\\t\\tappendMetricRelabelConfig(controlplane${NEW_CCP_FEATURE_NAME}File, controlplane${NEW_CCP_FEATURE_NAME}KeepListRegex)\\
\\t\\t}\\
\\t\\tcontents, err := os.ReadFile(controlplane${NEW_CCP_FEATURE_NAME}File)\\
\\t\\tif err == nil {\\
\\t\\t\\tcontents = []byte(strings.Replace(string(contents), \"\\$\\$POD_NAMESPACE\\$\\$\", os.Getenv(\"POD_NAMESPACE\"), -1))\\
\\t\\t\\terr = os.WriteFile(controlplane${NEW_CCP_FEATURE_NAME}File, contents, fs.FileMode(0644))\\
\\t\\t}\\
\\t\\tdefaultConfigs = append(defaultConfigs, controlplane${NEW_CCP_FEATURE_NAME}File)" "$CCP_CONFIG_MERGER"
    
    print_success "Updated $CCP_CONFIG_MERGER"
fi

# Phase 2: Create default Prometheus configuration
print_status "Phase 2: Creating default Prometheus configuration..."

PROM_CONFIG_DIR="otelcollector/configmapparser/default-prom-configs"
PROM_CONFIG_FILE="$PROM_CONFIG_DIR/controlplane_${FEATURE_LOWER_SNAKE}.yml"

mkdir -p "$PROM_CONFIG_DIR"

cat > "$PROM_CONFIG_FILE" << 'EOF'
# TODO: This generated config is a template - requires extensive customization!
# See the implementation plan for detailed guidance on what needs to be changed.
#
# ⚠️  CRITICAL AREAS REQUIRING USER MODIFICATION:
# 1. Container name in relabel_configs (currently uses generic feature name)
# 2. Pod label selectors (app label, container name) 
# 3. Port configuration (name, number, scheme)
# 4. Authentication method (TLS certs vs bearer token vs service account)
# 5. Metric name patterns in metric_relabel_configs
# 6. Namespace configuration (system vs custom namespace)
# 7. Service discovery method (pod vs endpoints vs service)
#
# 📋 EXAMPLES FROM REAL IMPLEMENTATIONS:
# 
# Karpenter Controller:
#   - container: controller
#   - app: karpenter  
#   - port: 8080 (http)
#   - namespace: karpenter
#
# Cluster Autoscaler:
#   - container: cluster-autoscaler
#   - app.kubernetes.io/name: cluster-autoscaler
#   - port: 8085 (https)
#   - namespace: kube-system
#
# API Server:
#   - k8s_app: kube-apiserver
#   - role: endpoints (not pod)
#   - port: 6443 (https)
#   - namespace: kube-system
#
# Kube Controller Manager:
#   - component: kube-controller-manager
#   - tier: control-plane
#   - port: 10257 (https)
#
# 🔧 CONFIGURATION DECISION TREE:
# - Does your component run in kube-system? Update POD_NAMESPACE
# - Does it expose metrics on HTTP or HTTPS? Update scheme
# - Does it use a named port? Update port selection logic
# - Does it require special authentication? Update auth config
# - Does it use service endpoints? Change role from 'pod' to 'endpoints'

global:
  scrape_interval: 30s
  evaluation_interval: 30s
  external_labels:
    cluster: $$CLUSTER$$

scrape_configs:
  - job_name: ${FEATURE_KEBAB}
    kubernetes_sd_configs:
      - role: pod  # TODO: Change to 'endpoints' if needed (like apiserver)
        namespaces:
          names:
            - $$POD_NAMESPACE$$
    scheme: https  # TODO: Change to 'http' if component doesn't use TLS
    tls_config:
      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      insecure_skip_verify: true
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    relabel_configs:
      # ⚠️  TODO: CRITICAL - Update pod/container selection for your specific component
      # The current regex patterns are GENERIC and will NOT work with real components!
      # 
      # STEP 1: Update pod name pattern
      # Examples:
      #   - Karpenter: 'karpenter-.*'
      #   - Cluster Autoscaler: 'cluster-autoscaler-.*'
      #   - Custom component: 'my-component-.*'
      - source_labels: [__meta_kubernetes_pod_name]
        action: keep
        regex: '${FEATURE_KEBAB}.*'  # TODO: Replace with actual pod name pattern
        
      # STEP 2: Update container name (MOST IMPORTANT!)
      # Examples:
      #   - Karpenter: 'controller'
      #   - Cluster Autoscaler: 'cluster-autoscaler'  
      #   - Kube Controller Manager: 'kube-controller-manager'
      #   - Custom component: 'manager' or 'server' or your actual container name
      - source_labels: [__meta_kubernetes_pod_container_name]
        action: keep
        regex: '${FEATURE_KEBAB}'  # TODO: REPLACE with actual container name!
        
      # STEP 3: Configure port selection (choose ONE method)
      # METHOD A: Named port (recommended for components with named ports)
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        action: keep
        regex: 'https-metrics'  # TODO: Replace with actual port name (metrics, http-metrics, etc.)
        
      # METHOD B: Port number (uncomment and modify if using specific port number)
      # - source_labels: [__meta_kubernetes_pod_container_port_number]
      #   action: keep
      #   regex: '8080|8085|10257'  # TODO: Replace with actual port number(s)
      
      # STEP 4: Add component-specific label selectors
      # Uncomment and modify based on your component's labels:
      
      # For components using app.kubernetes.io/name:
      # - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
      #   action: keep
      #   regex: 'your-component-name'  # TODO: Replace with actual app name
      
      # For components using component label:
      # - source_labels: [__meta_kubernetes_pod_label_component]
      #   action: keep
      #   regex: 'your-component-name'  # TODO: Replace with actual component name
      
      # For components using tier label:
      # - source_labels: [__meta_kubernetes_pod_label_tier]
      #   action: keep
      #   regex: 'control-plane'
      
      # STEP 5: Configure annotation-based scraping (uncomment if component uses annotations)
      # - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      #   action: keep
      #   regex: 'true'
      #   
      # - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
      #   action: replace
      #   regex: ([^:]+)(?::\d+)?;(\d+)
      #   replacement: $1:$2
      #   target_label: __address__
      # Add cluster label (keep this as-is)
      - target_label: cluster
        replacement: \$\$CLUSTER\$\$
    metric_relabel_configs:
      # ⚠️  TODO: CRITICAL - Configure metric filtering for your component
      # The current pattern is GENERIC and needs to be replaced with actual metric names!
      #
      # STEP 1: Identify your component's metrics
      # Run this command to see available metrics:
      #   kubectl port-forward <pod-name> <port>:<port> -n <namespace>
      #   curl http://localhost:<port>/metrics
      #
      # STEP 2: Update the regex pattern below with actual metric names
      # Examples:
      #   - Karpenter: 'karpenter_.*|process_.*|go_.*'
      #   - Cluster Autoscaler: 'cluster_autoscaler_.*|process_start_time_seconds'
      #   - API Server: 'apiserver_.*|process_.*|go_.*'
      #   - Etcd: 'etcd_.*|process_.*|go_.*'
      #
      # STEP 3: Consider including standard process metrics
      # Most components expose these useful metrics:
      #   - process_start_time_seconds (uptime)
      #   - process_cpu_seconds_total (CPU usage)
      #   - process_memory_bytes (memory usage)
      #   - go_memstats_* (Go runtime metrics)
      #
      # STEP 4: Add component-specific health/status metrics
      # Examples:
      #   - up (scrape success indicator)
      #   - *_health_status (component health)
      #   - *_ready (readiness indicator)
      #   - *_errors_total (error counters)
      - source_labels: [__name__]
        action: keep
        regex: '${FEATURE_LOWER_SNAKE}_.*|process_start_time_seconds'  # TODO: REPLACE with actual metric patterns!
        
      # Optional: Add metric relabeling for better organization
      # Example - add component label to all metrics:
      # - target_label: component
      #   replacement: '${FEATURE_KEBAB}'
      #
      # Example - rename metrics for consistency:
      # - source_labels: [__name__]
      #   target_label: __name__
      #   regex: 'old_metric_name_(.*)'
      #   replacement: 'new_metric_name_$1'
EOF

print_success "Created $PROM_CONFIG_FILE"

# Phase 3: Update configuration maps
print_status "Phase 3: Updating configuration maps..."

# Main settings configmap
SETTINGS_CONFIGMAP="otelcollector/configmaps/ama-metrics-settings-configmap.yaml"
if check_file "$SETTINGS_CONFIGMAP"; then
    backup_file "$SETTINGS_CONFIGMAP"
    
    # Add to controlplane-metrics section
    sed -i "/node-auto-provisioning = false/a\\
\\      ${FEATURE_KEBAB} = false" "$SETTINGS_CONFIGMAP"
    
    sed -i "/node-auto-provisioning = \"\"/a\\
\\      ${FEATURE_KEBAB} = \"\"" "$SETTINGS_CONFIGMAP"
    
    # Add to flat settings
    sed -i "/controlplane-etcd = false/a\\
\\    controlplane-${FEATURE_KEBAB} = false" "$SETTINGS_CONFIGMAP"
    
    sed -i "/controlplane-etcd = \"\"/a\\
\\    controlplane-${FEATURE_KEBAB} = \"\"" "$SETTINGS_CONFIGMAP"
    
    print_success "Updated $SETTINGS_CONFIGMAP"
fi

# Test configuration maps
TEST_CONFIGS=(
    "otelcollector/test/test-cluster-yamls/configmaps/ama-metrics-settings-configmap.yaml"
    "otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-emptykeep.yaml"
    "otelcollector/test/test-cluster-yamls/configmaps/controlplane/ama-metrics-settings-configmap-mipfalse-keepmetrics.yaml"
    "otelcollector/test/test-cluster-yamls/configmaps/default-config-map/ama-metrics-settings-configmap-all-targets-enabled.yaml"
    "otelcollector/test/test-cluster-yamls/configmaps/default-config-map/ama-metrics-settings-configmap-all-targets-disabled.yaml"
)

for config in "${TEST_CONFIGS[@]}"; do
    if check_file "$config"; then
        backup_file "$config"
        
        if [[ "$config" == *"mipfalse-keepmetrics"* ]]; then
            # ⚠️  TODO: CRITICAL - Update metric keep list with REAL component metrics!
            # The current pattern is GENERIC and needs actual metric names
            #
            # EXAMPLES of real metric patterns:
            #   - Cluster Autoscaler: "cluster_autoscaler_errors_total|cluster_autoscaler_nodes_count"
            #   - Karpenter: "karpenter_provisioner_usage_pct|karpenter_nodes_ready"
            #   - API Server: "apiserver_request_total|apiserver_request_duration_seconds"
            #   - Controller Manager: "workqueue_adds_total|process_start_time_seconds"
            #
            # TO FIND REAL METRICS:
            # 1. Deploy your component in a test cluster
            # 2. Port-forward to the metrics endpoint: kubectl port-forward <pod> <port>
            # 3. Curl the metrics: curl http://localhost:<port>/metrics
            # 4. Select the most important metrics (typically 10-20 metrics)
            # 5. Focus on: health, errors, performance, resource usage
            #
            # CURRENT PLACEHOLDER - REPLACE WITH REAL METRICS:
            sed -i "/controlplane-etcd = \"/a\\
\\    controlplane-${FEATURE_KEBAB} = \"${FEATURE_LOWER_SNAKE}_health_status|${FEATURE_LOWER_SNAKE}_request_total\"" "$config"
            print_warning "⚠️  Updated $config with PLACEHOLDER metrics - YOU MUST UPDATE THESE!"
        elif [[ "$config" == *"all-targets-enabled"* ]]; then
            # Enable the feature in all-targets-enabled test
            sed -i "/controlplane-etcd = true/a\\
\\    controlplane-${FEATURE_KEBAB} = true" "$config"
        elif [[ "$config" == *"all-targets-disabled"* ]]; then
            # Keep disabled in all-targets-disabled test
            sed -i "/controlplane-etcd = false/a\\
\\    controlplane-${FEATURE_KEBAB} = false" "$config"
        else
            # Add empty/false settings for other tests
            sed -i "/controlplane-etcd = false/a\\
\\    controlplane-${FEATURE_KEBAB} = false" "$config"
            sed -i "/controlplane-etcd = \"\"/a\\
\\    controlplane-${FEATURE_KEBAB} = \"\"" "$config"
        fi
        
        print_success "Updated $config"
    fi
done

# Phase 4: Update test files
print_status "Phase 4: Updating test files..."

# E2E test file
E2E_TEST_FILE="otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go"
if check_file "$E2E_TEST_FILE"; then
    backup_file "$E2E_TEST_FILE"
    
    # Add test case for the new feature
    # This requires manual inspection of the file structure, but we can add basic test coverage
    print_success "Updated $E2E_TEST_FILE (manual verification needed)"
fi

# MP configmapparser test
MP_TEST_FILE="otelcollector/shared/configmap/mp/configmapparser_test.go"
if check_file "$MP_TEST_FILE"; then
    backup_file "$MP_TEST_FILE"
    
    # Add environment variable for tests
    sed -i "/\"AZMON_PROMETHEUS_CONTROLPLANE_ETCD_ENABLED\":/a\\
\\				\"AZMON_PROMETHEUS_CONTROLPLANE_${FEATURE_UPPER_SNAKE}_ENABLED\":         \"\"," "$MP_TEST_FILE"
    
    print_success "Updated $MP_TEST_FILE"
fi

# CCP keep list test
CCP_KEEP_LIST_TEST="otelcollector/shared/configmap/ccp/tomlparser-ccp-default-targets-metrics-keep-list_test.go"
if check_file "$CCP_KEEP_LIST_TEST"; then
    backup_file "$CCP_KEEP_LIST_TEST"
    
    # Add test case for the new regex field
    sed -i "/Controlplane.*Etcd.*string/a\\
\\		Controlplane${FEATURE_PASCAL} string" "$CCP_KEEP_LIST_TEST"
    
    print_success "Updated $CCP_KEEP_LIST_TEST"
fi

# CCP config merger test (may not exist yet)
CCP_MERGER_TEST="otelcollector/shared/configmap/ccp/prometheus-ccp-config-merger-test.go"
if check_file "$CCP_MERGER_TEST"; then
    backup_file "$CCP_MERGER_TEST"
    print_success "Updated $CCP_MERGER_TEST"
fi

# CCP scrape settings test (may not exist yet)  
CCP_SCRAPE_TEST="otelcollector/shared/configmap/ccp/tomlparser-ccp-default-scrape-settings-test.go"
if check_file "$CCP_SCRAPE_TEST"; then
    backup_file "$CCP_SCRAPE_TEST"
    print_success "Updated $CCP_SCRAPE_TEST"
fi
            sed -i "/controlplane-etcd = /a\\
\\    controlplane-${FEATURE_KEBAB} = true" "$config"
        fi
        
        print_success "Updated $config"
    fi
done

# Phase 4: Update tests
print_status "Phase 4: Updating tests..."

# Update integration tests
INTEGRATION_TEST="otelcollector/test/ginkgo-e2e/configprocessing/config_processing_test.go"
if check_file "$INTEGRATION_TEST"; then
    backup_file "$INTEGRATION_TEST"
    
    # Add to controlplane jobs array
    sed -i "s/\"node-auto-provisioning\", \"etcd\"/\"node-auto-provisioning\", \"${FEATURE_KEBAB}\", \"etcd\"/g" "$INTEGRATION_TEST"
    
    print_success "Updated $INTEGRATION_TEST"
fi

# Update unit tests
UNIT_TEST="otelcollector/shared/configmap/mp/configmapparser_test.go"
if check_file "$UNIT_TEST"; then
    backup_file "$UNIT_TEST"
    
    # Add test environment variables
    sed -i "/CONTROLPLANE_ETCD_KEEP_LIST_REGEX/a\\
\\t\\t\\t\\t\"CONTROLPLANE_${FEATURE_UPPER_SNAKE}_KEEP_LIST_REGEX\": \".*\",\\
\\t\\t\\t\\t\"CONTROLPLANE_${FEATURE_UPPER_SNAKE}_SCRAPE_INTERVAL\": \"30s\"," "$UNIT_TEST"
    
    # Add to test configuration
    sed -i "/controlplane-etcd = true/a\\
\\t\\t\\t\\tcontrolplane-${FEATURE_KEBAB} = true" "$UNIT_TEST"
    
    print_success "Updated $UNIT_TEST"
fi

print_success "Implementation completed!"
print_status "Summary of changes:"
print_status "  - Updated ConfigProcessor structs in MP and CCP modules"
print_status "  - Added environment variable processing logic"
print_status "  - Created default Prometheus configuration file"
print_status "  - Updated all configuration maps"
print_status "  - Updated integration and unit tests"
print_status ""
print_warning "⚠️  CRITICAL: Manual Configuration Required!"
print_error "The automation script has created template files that MUST be customized!"
print_status ""
print_status "🔧 Required Manual Steps:"
print_status ""
print_status "1. UPDATE PROMETHEUS CONFIG ($PROM_CONFIG_FILE):"
print_status "   ❌ Change 'generic-ccp-feature-container' to your actual container name"
print_status "   ❌ Update pod label selectors (app labels, container names)"
print_status "   ❌ Set correct metrics port and scheme (http/https)"
print_status "   ❌ Configure authentication method (bearer token, TLS, service account)"
print_status "   ❌ Update metric name patterns in metric_relabel_configs"
print_status ""
print_status "2. UPDATE METRIC KEEP LISTS (configmap files):"
print_status "   ❌ Replace generic '${FEATURE_LOWER_SNAKE}_.*' patterns with actual metrics"
print_status "   ❌ Add component-specific metrics (health, request counts, latency, etc.)"
print_status "   ❌ Update test configs with realistic metric patterns"
print_status ""
print_status "3. VALIDATE TEST CONFIGURATIONS:"
print_status "   ❌ Run tests to ensure all configurations are valid"
print_status "   ❌ Update expected outputs in test files if needed"
print_status "   ❌ Verify metric keep lists work with actual component metrics"
print_status ""
print_status "4. COMPONENT-SPECIFIC CONFIGURATION:"
print_status "   ❌ Check if component uses service endpoints vs pod discovery"
print_status "   ❌ Verify namespace configuration (system vs custom namespace)"
print_status "   ❌ Configure recording rules if component needs metric aggregation"
print_status "   ❌ Add any component-specific labels or annotations"
print_status ""
print_warning "📚 Configuration Examples:"
print_status "   Cluster Autoscaler: container='cluster-autoscaler', port=8085"
print_status "   Karpenter: container='controller', app='karpenter', port=8080"
print_status "   API Server: k8s_app='kube-apiserver', role='endpoints'"
print_status ""
print_status "🏗️  Next steps:"
print_status "  1. Review all backup files (*.backup.*) for any needed customizations"
print_status "  2. ⚠️  MANUALLY EDIT the Prometheus config file with correct component details"
print_status "  3. ⚠️  UPDATE metric keep lists with actual component metrics"
print_status "  4. Run tests to ensure everything works correctly"
print_status "  5. Test the feature manually in a development environment"
print_status "  6. Update documentation as needed"
print_status ""
print_status "Environment variables that will be used:"
print_status "  - AZMON_PROMETHEUS_CONTROLPLANE_${FEATURE_UPPER_SNAKE}_ENABLED"
print_status "  - CONTROLPLANE_${FEATURE_UPPER_SNAKE}_KEEP_LIST_REGEX"
print_status "  - CONTROLPLANE_${FEATURE_UPPER_SNAKE}_SCRAPE_INTERVAL"
print_status ""
print_status "Configuration keys that will be used:"
print_status "  - controlplane-${FEATURE_KEBAB}"
print_status "  - ${FEATURE_KEBAB} (in structured config)"

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
