# Azure Managed Prometheus Troubleshooting Memory Bank

## Overview
This document serves as a comprehensive knowledge base for AI agents investigating Azure Managed Prometheus cluster issues. It contains proven investigation methodologies, common failure patterns, and diagnostic queries derived from real production troubleshooting sessions.

## Investigation Framework

### 1. Azure MCP Server Setup
- **Primary Tool**: Azure Monitor MCP server for querying Application Insights telemetry
- **Data Source**: Application Insights component (NOT Log Analytics workspace)
- **Resource Path**: `/subscriptions/13d371f9-5a39-46d5-8e1b-60158c49db84/resourceGroups/ContainerInsightsPrometheusCollector-Prod/providers/microsoft.insights/components/ContainerInsightsPrometheusCollector-Prod`
- **Tables**: `customMetrics` and `traces` tables contain Managed Prometheus telemetry

### 1.1 AKS Cluster Telemetry (Kusto)
- **Secondary Tool**: Kusto MCP server for querying AKS cluster telemetry
- **Cluster URI**: `https://akshuba.centralus.kusto.windows.net/`
- **Database**: `AKSprod`
- **Data Source**: Direct AKS cluster telemetry and infrastructure metrics
- **Use Case**: Correlate Managed Prometheus issues with broader AKS cluster health and events
- **Tables**: `BlackboxMonitoringActivity`, `ManagedClusterMonitoring`, and `ManagedClusterSnapshot`

### 2. Cluster-Specific Investigation

#### Cluster Health Deep Dive
```kql
customMetrics 
| where timestamp > ago(72h)
| where tostring(customDimensions.cluster) == "/subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ContainerService/managedClusters/{cluster-name}"
| summarize count() by name, bin(timestamp, 1h)
| render timechart
```

#### Component-Specific Metrics
```kql
customMetrics 
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name in ("prometheus_remote_storage_bytes_total", "otelcol_exporter_sent_metric_points_total", "otelcol_exporter_send_failed_metric_points_total")
| extend controllertype = tostring(customDimensions.controllertype)
| summarize sum(value) by name, controllertype
| order by name, controllertype
```

#### ReplicaSet Pod Count Check (HPA Analysis)
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend podname=tostring(customDimensions.podname)
| summarize replica_pod_count=dcount(podname) by bin(timestamp, 5m)
| summarize max(replica_pod_count)
```
**Note**: The HPA limit is 24 replicas. If max replica count reaches 24, the cluster may be hitting HPA limits, which can contribute to resource pressure and OOM conditions.

### 3. AKS Cluster Telemetry Investigation

**Note**: Use Kusto MCP server with cluster URI: `https://akshuba.centralus.kusto.windows.net/`

#### Discover Available Databases
First, list available databases to identify the appropriate one for your investigation:
```kql
// Use kusto_database_list command to discover available databases
```

#### Sample AKS Investigation Patterns
```kql
// Pod restart correlation (example - actual table names may vary)
// Replace {database} and {table} with actual discovered names
PodEvents
| where timestamp > ago(24h)
| where cluster == "{cluster-name}"
| where reason == "Failed" or reason == "FailedMount" or reason == "BackOff"
| summarize count() by reason, bin(timestamp, 1h)
| render timechart
```

#### Cross-Reference with Prometheus Issues
```kql
// Correlate infrastructure events with metrics pipeline failures
// Use actual table schema from kusto_table_schema command
NodeEvents
| where timestamp > ago(24h) 
| where cluster == "{cluster-name}"
| where reason contains "Network" or reason contains "DNS" or reason contains "Storage"
| project timestamp, node, reason, message
| order by timestamp desc
```

## Common Failure Patterns

### Pattern 1: MetricsExtension Startup Failure
**Symptoms:**
- `Error getting PID for process MetricsExtension: error running exit status 1`
- `TokenConfig.json does not exist`
- `No configuration present for the AKS resource`
- `Metrics Extension is not running (configuration exists)`

**Root Cause:** Authentication configuration issues preventing MetricsExtension initialization

**Impact:** Complete metrics pipeline failure, OTLP export failures

### Pattern 2: OTLP Export Connection Failures
**Symptoms:**
- `dial tcp 127.0.0.1:55680: connect: connection refused`
- `Exporting failed. Dropping data`
- Large numbers of `dropped_items`

**Root Cause:** MetricsExtension not running/listening on port 55680

**Diagnostic Query:**
```kql
traces 
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where message contains "connection refused" or message contains "dropped_items"
| project timestamp, controllertype=tostring(customDimensions.controllertype), message
| order by timestamp desc
```

### Pattern 4: DCR/DCE/AMCS Configuration Errors
**Symptoms:**
- `TokenConfig.json does not exist`
- `No configuration present for the AKS resource`
- `InvalidAccess` errors
- `Data collection endpoint must be used to access configuration over private link`

**Root Cause:** Authentication token, resource configuration missing, or AMPLS (Azure Monitor Private Link Scope) misconfiguration

**Diagnostic Query:**
```kql
traces 
| where timestamp > ago(7d)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where message contains "TokenConfig.json does not exist" or message contains "No configuration present for the AKS resource" or message contains "InvalidAccess" or message contains "Data collection endpoint must be used to access configuration over private link"
| extend controllertype = tostring(customDimensions.controllertype)
| project timestamp, controllertype, message
| order by timestamp desc
```

**Resolution for AMPLS (Private Link) Errors:**
When encountering `Data collection endpoint must be used to access configuration over private link` errors:
1. **Verify AMPLS Configuration**: Ensure Azure Monitor Private Link Scope (AMPLS) is properly configured
2. **Associate DCE with AMPLS**: The Data Collection Endpoint (DCE) must be associated with the AMPLS resource
3. **Check Private DNS Zones**: Verify private DNS zones are correctly configured for the AMPLS endpoints
4. **Validate Network Connectivity**: Ensure the AKS cluster can reach the private endpoints through the configured network path
5. **Review AMPLS Access Mode**: Confirm the AMPLS is configured with the correct access mode (Private Only vs Open)

### Pattern 5: MetricsExtension HTTP Publication Failures (Transient - Can Be Ignored)
**Symptoms:**
- `Failed to read HTTP status line`
- `Failed to write request headers`
- `Metrics data publication failed with an HTTP exception`
- `Error in SSL handshake` (intermittent)
- `503 Service Unavailable` responses

**Root Cause:** Transient network connectivity issues to Azure Monitor ingestion endpoints

**Impact:** These are temporary network issues with built-in retry mechanisms. MetricsExtension will automatically retry failed publications, so these errors can be safely ignored unless they persist continuously for extended periods without any successful publications between them.

**Important Note:** SSL handshake errors are often transient and part of normal network behavior. Only investigate SSL handshake errors if they are:
- Occurring continuously for hours without successful publications
- Preventing authentication token retrieval completely
- Accompanied by complete pipeline failures (e.g., MetricsExtension startup failures)

**Diagnostic Query:**
```kql
traces 
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where message contains "MetricsExtension" and (message contains "Failed to read" or message contains "Failed to write")
| project timestamp, controllertype=tostring(customDimensions.controllertype), message
| order by timestamp desc
```

**Note:** Only investigate this pattern if failures persist continuously for hours without successful publications between them. SSL handshake errors are commonly transient and should not be treated as the primary issue unless they prevent core functionality.

### Pattern 6: Disabled Filtering Pattern with Custom Application Overload
**Symptoms:**
- High memory consumption (10+ GB per pod) with healthy OTLP pipeline
- OOM-killed ReplicaSet pods despite successful metric exports
- Massive metrics volume (billions of metrics per day)
- Empty keep-list strings in metric filtering configuration
- High percentage of custom PodMonitor targets (>80% of total targets)

**Root Cause:** Disabled metric filtering combined with high-cardinality custom applications

**Diagnostic Approach:**
1. **Check metric filtering configuration** - Look for empty keep-list strings
2. **Perform pod-level correlation analysis** - Correlate memory usage, metrics scraped, and targets assigned by pod name
3. **Analyze custom target distribution** - Identify percentage of custom vs standard targets
4. **Calculate memory efficiency ratios** - Identify pods with poor memory efficiency per metric
5. **Identify high-cardinality applications** - Find custom applications producing excessive metrics

**Diagnostic Queries:**
- Use Comprehensive Pod-Level Correlation Analysis queries
- Use Custom Target Distribution Analysis query
- Use High-Volume Custom Application Identification query

**Impact:** Memory pressure from unfiltered custom application metrics, leading to OOM kills despite healthy telemetry pipeline

**Resolution Path:**
1. **Enable standard metric filtering** for Kubernetes components
2. **Evaluate custom PodMonitor applications** for metric necessity
3. **Implement custom metric filtering** for high-cardinality applications
4. **Monitor memory reduction** and HPA scaling effectiveness

## Investigation Workflow

### Step 1: Cluster-Specific Analysis
1. Filter to problematic cluster using resource ID
2. Check component metrics (OTLP exports, failures)
3. **Check ReplicaSet replica count**: Verify if cluster has hit HPA limit (24 replicas max)
4. **Check metric filtering configuration**: Verify if default keep lists are disabled (empty strings indicate disabled filtering)
5. **Correlate memory usage with metrics load**: Check if high-memory pods are processing more metrics
6. **Verify target distribution**: Ensure targets are evenly distributed across collectors
7. **Perform comprehensive pod-level correlation analysis**: For OOM investigations, correlate by pod name:
   - Metrics scraped per pod (`meMetricsReceivedCount`)
   - Targets assigned per pod (`target_allocator_opentelemetry_allocator_targets_per_collector`)
   - Memory usage per pod (`otelcollector_memory_rss_095` / `metricsextension_memory_rss_095`)
   - Calculate memory efficiency ratios (Memory GB per million metrics)
   - Identify memory anomalies and processing inefficiencies
8. **Analyze custom target distribution**: For memory pressure investigations:
   - Query cluster-wide custom target breakdown by type (PodMonitor vs Configmap vs ServiceMonitor)
   - Identify high-cardinality custom applications contributing to memory pressure
   - Calculate percentage of custom vs standard targets
9. Analyze time-series patterns
10. **Cross-reference with AKS telemetry**: Query AKS Kusto cluster for broader cluster health context

### Step 2: Error Log Deep Dive
1. Query traces table for error patterns
2. Focus on container logs (prometheus.log.prometheuscollectorcontainer tag)
3. Look for startup sequence failures
4. **Correlate with AKS events**: Check AKS telemetry for pod restarts, resource constraints, or infrastructure issues

### Step 3: Component Dependency Analysis
1. Check MetricsExtension startup sequence
2. Verify OTLP exporter connectivity to MetricsExtension
3. Validate Prometheus scrape pool configurations
4. Examine authentication token availability

### Step 5: Root Cause Determination
Follow the failure chain:
```
Authentication Issues → MetricsExtension Failure → OTLP Export Failure → Data Loss
```

## Dashboard Query Patterns

The `dashboard.json` file contains systematic investigation queries including:

### DCR/DCE/AMCS Configuration Errors
```kql
traces 
| where timestamp > ago(7d)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where message contains "TokenConfig.json does not exist" or message contains "No configuration present for the AKS resource" or message contains "InvalidAccess" or message contains "Data collection endpoint must be used to access configuration over private link"
| extend controllertype = tostring(customDimensions.controllertype)
| project timestamp, controllertype, message
| order by timestamp desc
```

### ReplicaSet HPA Analysis
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "metricsextension_cpu_usage_095" or name == "otelcollector_cpu_usage_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend podname=tostring(customDimensions.podname)
| summarize replica_pod_count=dcount(podname) by bin(timestamp, 5m)
| summarize max(replica_pod_count)
```

### ReplicaSet Memory Usage Analysis (OOM Investigation)
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "otelcollector_memory_rss_095" or name == "metricsextension_memory_rss_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value = value / 1000000000
| extend pod=tostring(customDimensions.podname)
| summarize value=round(percentile(value, 100), 2) by pod, bin(timestamp, 5m)
| summarize value=sum(value) by pod, bin(timestamp, 5m)
```

### Metric Filtering Configuration Check (High Memory Investigation)
```kql
customMetrics
| where timestamp > ago(24h)
| order by timestamp
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsProcessedCount"
| take 1
| extend invalidPromConfig=tobool(customDimensions.InvalidCustomPrometheusConfig)
| extend apiserver=tostring(customDimensions.ApiServerKeepListRegex)
| extend cadvisor=tostring(customDimensions.CAdvisorKeepListRegex)
| extend coredns=tostring(customDimensions.CoreDNSKeepListRegex)
| extend kappie=tostring(customDimensions.KappieBasicKeepListRegex)
| extend kubeproxy=tostring(customDimensions.KubeProxyKeepListRegex)
| extend kubestate=tostring(customDimensions.KubeStateKeepListRegex)
| extend kubelet=tostring(customDimensions.KubeletKeepListRegex)
| extend nodeexporter=tostring(customDimensions.NodeExporterKeepListRegex)
| extend windowsexporter=tostring(customDimensions.WinExporterKeepListRegex)
| extend windowskubeproxy=tostring(customDimensions.WinKubeProxyKeepListRegex)
| extend acstorcapacityprovisioner=tostring(customDimensions.AcstorCapacityProvisionerRegex)
| extend acstormetricsexporter=tostring(customDimensions.AcstorMetricsExporterRegex)
| extend NetworkObservabilityCiliumScrape=tostring(customDimensions.NetworkObservabilityCiliumScrapeRegex)
| extend NetworkObservabilityHubbleScrape=tostring(customDimensions.NetworkObservabilityHubbleScrapeRegex)
| extend NetworkObservabilityRetinaScrape=tostring(customDimensions.NetworkObservabilityRetinaScrapeRegex)
| extend Values = pack(
                        "API Server", apiserver,
                        "cAdvisor", cadvisor,
                        "Core DNS", coredns,
                        "Kappie Basic", kappie,
                        "Kube Proxy", kubeproxy,
                        "Kube-State-Metrics", kubestate,
                        "Kubelet", kubelet,
                        "Node Exporter", nodeexporter,
                        "Windows Exporter", windowsexporter,
                        "AcStor Capacity Provisioner", acstorcapacityprovisioner,
                        "AcStor Metrics Exporter", acstormetricsexporter,
                        "Network Observability Cilium", NetworkObservabilityCiliumScrape,
                        "Network Observability Hubbble", NetworkObservabilityHubbleScrape,
                        "Network Observability Retina", NetworkObservabilityRetinaScrape
                    )
| mv-expand kind=array Values
| project Name=tostring(Values[0]), Value=tostring(Values[1])
```

### Metrics Load Per Pod Analysis (Memory Correlation)
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsReceivedCount"
| extend pod=tostring(customDimensions.podname)
| summarize value=max(value) by pod
```

### Target Allocator Distribution Analysis
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "target_allocator_opentelemetry_allocator_targets_per_collector"
| summarize round(targets=avg(value)) by bin(timestamp, 5m), collector=tostring(customDimensions.collector_name)
```

### Prometheus Scrape Job Analysis (Custom Target Investigation)
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend podname=tostring(customDimensions.podname)
| extend type = iff(job_name startswith "serviceMonitor", "ServiceMonitor", iff(job_name startswith "podMonitor", "PodMonitor", "Configmap"))
| summarize count=dcount(job_name) by bin(timestamp, 5m), type
```

**Note**: Uneven target distribution can occur when custom scrape jobs contain single targets with high metric volumes. These cannot be distributed across pods since each target is atomic. Customers should evaluate if all metrics from high-volume targets are needed or if additional filtering can be applied.

### Comprehensive Pod-Level Correlation Analysis (OOM Investigations)

#### Combined Pod Correlation Query
```kql
// Step 1: Get metrics scraped per pod
let metrics_data = customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| where name == "meMetricsReceivedCount"
| extend pod=tostring(customDimensions.podname)
| summarize metrics_scraped=max(value) by pod;
// Step 2: Get targets assigned per pod
let targets_data = customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "target_allocator_opentelemetry_allocator_targets_per_collector"
| extend pod=tostring(customDimensions.collector_name)
| summarize targets_assigned=round(avg(value)) by pod
| where targets_assigned > 0;
// Step 3: Get memory usage per pod
let memory_data = customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "otelcollector_memory_rss_095" or name == "metricsextension_memory_rss_095"
| where tostring(customDimensions.controllertype) == "ReplicaSet"
| extend value_gb = value / 1000000000
| extend pod=tostring(customDimensions.podname)
| summarize memory_gb=round(max(value_gb), 2) by pod;
// Step 4: Combine and calculate efficiency ratios
metrics_data
| join kind=leftouter targets_data on pod
| join kind=leftouter memory_data on pod
| extend metrics_per_target = iff(targets_assigned > 0, round(metrics_scraped / targets_assigned), 0)
| extend memory_per_million_metrics = iff(metrics_scraped > 0, round(memory_gb / (metrics_scraped / 1000000), 2), 0)
| project pod, memory_gb, metrics_scraped, targets_assigned, metrics_per_target, memory_per_million_metrics
| order by memory_gb desc
```

#### Custom Target Distribution Analysis
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "target_allocator_opentelemetry_allocator_targets"
| extend job_name=tostring(customDimensions.job_name)
| extend type = iff(job_name startswith "serviceMonitor", "ServiceMonitor", iff(job_name startswith "podMonitor", "PodMonitor", "Configmap"))
| summarize total_targets=count(), unique_jobs=dcount(job_name) by type
| extend percentage = round(100.0 * total_targets / toscalar(summarize sum(total_targets)), 1)
| order by total_targets desc
```

#### High-Volume Custom Application Identification
```kql
customMetrics
| where timestamp > ago(24h)
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where name == "target_allocator_opentelemetry_allocator_targets"
| where tostring(customDimensions.podname) == "{specific-pod-name}" // Replace with pod of interest
| extend job_name=tostring(customDimensions.job_name)
| extend target=tostring(customDimensions.target)
| extend type = iff(job_name startswith "serviceMonitor", "ServiceMonitor", iff(job_name startswith "podMonitor", "PodMonitor", "Configmap"))
| summarize target_count=dcount(target) by job_name, type
| order by target_count desc, job_name asc
```

### Collector Error Logs
```kql
traces 
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where tostring(customDimensions.tag) == "prometheus.log.prometheuscollectorcontainer"
| mv-expand message = split(message, "\\n") to typeof(string)
| where (message contains 'error' or message contains 'E!' or message contains 'warning::Custom prometheus config does not exist') and message !contains "\\"filepath\\":\\"/"
| project timestamp, controllertype=tostring(customDimensions.controllertype), osType=tostring(customDimensions.osType), message
| order by timestamp desc
```

## Key Diagnostic Indicators

### Healthy Cluster Indicators
- MetricsExtension PID found successfully
- OTLP exports succeeding with no connection refused errors
- CA certificates accessible for secure scraping
- TokenConfig.json present and valid

### Unhealthy Cluster Indicators
- High volume of OTLP export failures (7000+ dropped items per failure)
- MetricsExtension startup failures
- Persistent CA certificate errors
- Authentication configuration errors
- **ReplicaSet count at HPA limit (24 replicas)** - indicates resource pressure
- **High memory consumption with healthy OTLP pipeline** - suggests disabled filtering or custom application overload
- **Memory efficiency ratio >1.5 GB per million metrics** - indicates processing inefficiency
- **Custom targets >80% of total targets** - suggests custom application-heavy environment requiring specialized filtering
- **Empty keep-list strings in filtering configuration** - indicates disabled metric filtering

## Remediation Guidelines

### For MetricsExtension Failures
1. Check AKS resource configuration in Azure Monitor
2. Verify DCR/DCE assignments
3. Validate managed identity permissions
4. Ensure TokenConfig.json generation process
5. **For Private Link errors**: Configure AMPLS (Azure Monitor Private Link Scope) and associate DCE with AMPLS

### For OTLP Export Failures
1. Verify MetricsExtension is running and healthy
2. Check port 55680 connectivity
3. Enable retry_on_failure configuration
4. Monitor for network timeouts

### For CA Certificate Issues
1. Verify certificate deployment to container
2. Check custom scrape configuration validity
3. Validate TLS/SSL configurations

### For MetricsExtension HTTP Publication Failures
**No action required** - These are transient network errors with automatic retry mechanisms. Only investigate if failures persist continuously for extended periods without any successful publications. SSL handshake errors are commonly intermittent and should be ignored unless they prevent core authentication or pipeline functionality.

## Case Study: aks-sde-eastus Cluster

**Key Findings:**
- 1.7M failed OTLP exports over 24 hours
- MetricsExtension startup failures due to missing TokenConfig.json
- Authentication configuration issues with DCR/DCE/AMCS
- CA certificate problems affecting patroni scrape pools
- Complete metrics pipeline failure

**Root Cause Chain:**
1. Authentication configuration missing
2. MetricsExtension fails to start
3. OTLP exporter cannot connect (port 55680 refused)
4. Massive data loss (7000 metrics per failed export)
5. Additional CA certificate issues compound the problem

**Resolution Path:**
1. Fix authentication configuration and TokenConfig.json generation
2. Ensure proper DCR/DCE/AMCS assignments
3. Verify managed identity permissions
4. Address CA certificate deployment issues

**Customer Symptom:** ReplicaSet pods getting OOM-killed

**Investigation Findings:**

### Primary Issues Identified:
1. **Massive OTLP Export Failures**: Continuous `dropped_items: 7000` errors with `DeadlineExceeded` failures
3. **MetricsExtension HTTP Publication Failures**: Transient network connectivity issues (Pattern 5 - can be ignored)

### Root Cause Analysis:
**The OOM symptoms are NOT caused by memory issues but by the underlying metrics pipeline failures:**

1. **MetricsExtension Not Running**: The MetricsExtension process is not running, so the OTLP exporter cannot connect to port 55680
2. **OTLP Export Failures**: Because MetricsExtension is down, the OTLP exporter continuously fails with `rpc error: code = DeadlineExceeded desc = context deadline exceeded`, dropping 7000 metrics per failure
3. **Metrics Memory Buildup**: Failed exports cause metrics to accumulate in the collector's memory since they cannot be exported to MetricsExtension
4. **OOM Condition**: The accumulated metrics eventually consume all available memory, causing the ReplicaSet pods to be OOM-killed

### Evidence Pattern Matching:
- **Pattern 2**: ✅ OTLP export failures with DeadlineExceeded errors
- **Pattern 4**: ✅  No DCR/DCE/AMCS authentication issues found  
- **Pattern 5**: ✅ MetricsExtension HTTP publication failures (transient, can be ignored)

### Timeline of Failures:
- Continuous OTLP export failures every few minutes throughout the 24-hour period
- 7000 metrics dropped per failed export

### Resolution Path:
1. **Immediate**: Investigate why MetricsExtension is not running and fix the underlying issue
2. **Secondary**: Fix missing CA certificate `/etc/ssl/certs/AME_INFRA_CA_03.crt` deployment
3. **Configure OTLP Retry**: Enable `retry_on_failure` configuration option as suggested in error messages
4. **Verify Custom Scrape Configuration**: Ensure patroni-related scrape pool configurations are valid
5. **Monitor Memory Usage**: The OOM kills should resolve once OTLP exports succeed consistently

### Key Learning:
**OOM symptoms can be secondary effects of metrics pipeline failures** - investigate telemetry export issues first before assuming memory leaks or resource constraints.

## Case Study: falcon-phx-staging Clusters (July 22, 2025)

**Customer Symptom:** Missing metrics and Azure Managed Prometheus pipeline failures

**Investigation Findings:**

### Primary Issues Identified:
1. **Missing TokenConfig.json**: Persistent authentication configuration failures
2. **Complete OTLP Export Failures**: Continuous `dial tcp 127.0.0.1:55680: connect: connection refused` errors
3. **Configuration Pipeline Breakdown**: `No configuration present for the AKS resource` errors

### Root Cause Analysis:
**Authentication configuration system failure preventing MetricsExtension startup:**

1. **Authentication Configuration Missing**: DCR/DCE/AMCS configuration pipeline failed
2. **TokenConfig.json Generation Failed**: Unable to generate authentication tokens
3. **MetricsExtension Cannot Start**: Service dependency on authentication configuration
4. **OTLP Export Complete Failure**: Cannot connect to MetricsExtension on port 55680
5. **Massive Data Loss**: Continuous metrics drops (5-1513 items per failure)

### Evidence Pattern Matching:
- **Pattern 1**: ✅ MetricsExtension startup failures with authentication issues
- **Pattern 2**: ✅ OTLP export connection refused errors (port 55680)
- **Pattern 4**: ✅ DCR/DCE/AMCS configuration errors with missing TokenConfig.json

### Timeline of Failures:
- **July 22, 2025 23:59 UTC**: Extended period of authentication and export failures
- **Duration**: Persistent failures throughout the monitoring period
- **Scope**: Both falcon-phx-staging and falcon-phx resource group clusters affected

### Resolution Path:
1. **Immediate**: Fix authentication configuration and DCR/DCE/AMCS assignments
2. **Secondary**: Regenerate TokenConfig.json through service restart
3. **Configure OTLP Retry**: Enable `retry_on_failure` configuration option
4. **Verify Pipeline**: Ensure complete metrics pipeline health post-recovery
5. **Monitor Authentication**: Set up proactive alerts for TokenConfig.json failures

### Key Learning:
**Authentication configuration failures create cascading effects** - missing TokenConfig.json prevents MetricsExtension startup, causing complete metrics pipeline failure. Always investigate authentication first when seeing OTLP export connection refused errors.

## Case Study: kt0f0850119001 Cluster (August 5-6, 2025)

**Customer Symptom:** Complete metrics pipeline failure around August 5th and 6th

**Investigation Findings:**

### Primary Issues Identified:
1. **Authentication Configuration Failures**: Persistent `TokenConfig.json does not exist` errors
2. **AMPLS/Private Link Misconfiguration**: `Data collection endpoint must be used to access configuration over private link` errors
3. **Complete OTLP Export Failures**: Continuous `dial tcp 127.0.0.1:55680: connect: connection refused` errors
4. **MetricsExtension SSL Handshake Failures**: `Error in SSL handshake` during metrics publication
5. **DNS Resolution Issues**: kube-state-metrics DNS failures

### Root Cause Analysis:
**Multiple simultaneous failures creating complete metrics pipeline breakdown:**

1. **AMPLS Configuration Issues**: DCE not properly associated with AMPLS, causing private link access errors
2. **Authentication Configuration Missing**: Unable to generate TokenConfig.json due to AMPLS/DCE issues
3. **MetricsExtension Startup Failures**: Service cannot start without proper authentication configuration
4. **OTLP Export Complete Failure**: Cannot connect to MetricsExtension on port 55680
5. **SSL/TLS Connectivity Issues**: Network layer problems with Azure Monitor endpoints
6. **Massive Data Loss**: 21-4304 metrics dropped per failure, continuous throughout timeframe

### Evidence Pattern Matching:
- **Pattern 1**: ✅ MetricsExtension startup failures (`Error getting PID for process MetricsExtension`)
- **Pattern 2**: ✅ OTLP export connection refused errors (port 55680)
- **Pattern 4**: ✅ DCR/DCE/AMCS configuration errors + AMPLS private link issues
- **Pattern 5**: ✅ MetricsExtension SSL handshake failures with publication endpoint

### Timeline of Failures:
- **August 5-6, 2025**: Continuous failures across both DaemonSet and ReplicaSet controllers
- **Duration**: Persistent failures throughout the 72-hour monitoring period
- **Scope**: Complete metrics pipeline breakdown affecting all components

### Resolution Path:
1. **Immediate**: Fix AMPLS configuration - properly associate DCE with AMPLS resource
2. **Authentication**: Regenerate TokenConfig.json after AMPLS configuration is fixed
3. **Network**: Verify SSL certificate chains and DNS resolution for Azure Monitor endpoints
4. **OTLP Pipeline**: Restart MetricsExtension services after authentication is restored
5. **Monitoring**: Implement enhanced alerting for AMPLS and authentication failures

### Key Learning:
**AMPLS misconfiguration can trigger cascading authentication failures** - private link access errors prevent proper DCE configuration, blocking TokenConfig.json generation and causing complete metrics pipeline failure. Always check AMPLS configuration first when seeing private link-related errors.

## Case Study: cosmic-dev-d00-002-nam-eastus-aks Cluster (August 8, 2025)

**Customer Symptom:** Complete metrics pipeline failure starting at 21:00 UTC on August 8, 2025

**Investigation Findings:**

### Primary Issues Identified:
1. **SSL Handshake Failures**: `Error in SSL handshake` preventing secure communication
2. **TokenConfig.json Generation Failures**: Cannot retrieve authentication configuration due to SSL issues
3. **MetricsExtension Startup Failures**: `Error getting PID for process MetricsExtension` - service cannot start without authentication
4. **Complete OTLP Export Failures**: `dial tcp 127.0.0.1:55680: connect: connection refused` - MetricsExtension not listening
5. **Configuration Retrieval Failures**: `Could not obtain configuration from https://eastus.handler.control.monitor.azure.com` with ErrorCode:1310977

### Root Cause Analysis:
**SSL/TLS connectivity issues creating cascading authentication and pipeline failures:**

1. **Network/SSL Layer Failures**: SSL handshake errors prevent secure connections to Azure Monitor endpoints
2. **Configuration Service Unreachable**: Cannot fetch cluster configuration from `eastus.handler.control.monitor.azure.com`
3. **TokenConfig.json Generation Blocked**: Authentication token retrieval fails due to connectivity issues
4. **MetricsExtension Cannot Start**: Service dependency on successful authentication configuration
5. **OTLP Export Complete Failure**: Cannot connect to MetricsExtension on port 55680
6. **Massive Data Loss**: 5-7000 metrics dropped per failure, continuous throughout timeframe

### Evidence Pattern Matching:
- **Pattern 1**: ✅ MetricsExtension startup failures due to authentication issues
- **Pattern 2**: ✅ OTLP export connection refused errors (port 55680) 
- **Pattern 4**: ✅ DCR/DCE/AMCS configuration errors with `TokenConfig.json does not exist`
- **Pattern 5**: ✅ SSL handshake failures during configuration retrieval

### Timeline of Failures:
- **Before 21:00 UTC August 8**: Normal operations with 4,000+ metrics/hour
- **21:00 UTC August 8**: Dramatic drop to 1,997 metrics/hour 
- **After 21:00 UTC August 8**: Complete cessation of metrics flow
- **Duration**: Complete pipeline failure persisting through investigation

### Diagnostic Evidence:
```
- "Error in SSL handshake" - Network connectivity/certificate issues
- "TokenConfig.json does not exist" - Authentication configuration missing
- "dial tcp 127.0.0.1:55680: connect: connection refused" - MetricsExtension not running
- "Could not obtain configuration from https://eastus.handler.control.monitor.azure.com" - Service endpoint unreachable
- "dropped_items": 5-7000 per failure - Massive data loss
```

### Resolution Path:
1. **Cluster Owner Investigation Required**: SSL handshake failures indicate underlying network/certificate issues that require cluster-level investigation
2. **Network Connectivity**: Verify outbound connectivity to Azure Monitor endpoints (especially eastus.handler.control.monitor.azure.com)
3. **Certificate Validation**: Check SSL certificate chains, expiration, and trust store configuration
4. **Firewall/NSG Rules**: Ensure no network security changes blocking HTTPS traffic to Azure Monitor endpoints
5. **DNS Resolution**: Verify proper DNS resolution for Azure Monitor service endpoints

### Key Learning:
**SSL/Network connectivity issues can trigger complete metrics pipeline failure** - SSL handshake errors prevent configuration retrieval from Azure Monitor endpoints, blocking TokenConfig.json generation and causing MetricsExtension startup failures. The root cause requires cluster owner investigation of network connectivity, certificates, and firewall rules rather than Azure Managed Prometheus service configuration.

**Investigation Priority**: When seeing SSL handshake errors combined with TokenConfig.json missing and MetricsExtension startup failures, focus on network connectivity and SSL certificate issues first. The cluster owner must investigate underlying network infrastructure before Azure Managed Prometheus components can recover.

## Case Study: aks-fsc-prod-app-eaus-001 Cluster (August 11, 2025) - Re-Investigation with Enhanced Methodology

**Customer Symptom:** ReplicaSet pods getting OOM-killed

**Investigation Findings Using Enhanced Troubleshooting Steps:**

### Step 1: Cluster-Specific Analysis Results

#### 1.3 ReplicaSet Replica Count Check (HPA Analysis)
- **Current Replica Count**: 16 pods
- **HPA Limit**: 24 replicas  
- **Status**: ✅ **NOT hitting HPA limits** - cluster can still scale up 8 more replicas

#### 1.4 Metric Filtering Configuration Check - **ROOT CAUSE IDENTIFIED**
**CRITICAL DISCOVERY**: Nearly all metric filtering (keep lists) are **DISABLED**:
- **API Server**: ❌ Empty string (ALL metrics ingested)
- **cAdvisor**: ❌ Empty string (ALL metrics ingested)
- **Core DNS**: ❌ Empty string (ALL metrics ingested)
- **Kappie Basic**: ❌ Empty string (ALL metrics ingested) 
- **Kube Proxy**: ❌ Empty string (ALL metrics ingested)
- **Kubelet**: ❌ Empty string (ALL metrics ingested)
- **Node Exporter**: ❌ Empty string (ALL metrics ingested)
- **Windows Exporter**: ❌ Empty string (ALL metrics ingested)
- **AcStor Components**: ❌ Empty string (ALL metrics ingested)
- **Network Observability**: ❌ Empty string (ALL metrics ingested)
- **Kube-State-Metrics**: ✅ **ONLY component with filtering enabled**

### Memory Usage Analysis - **CONFIRMS HIGH MEMORY CONSUMPTION**
**Current ReplicaSet Memory Usage (Last Hour):**
- **`ama-metrics-fb679f7bc-ndtqp`**: **11.3 GB** ⚠️ CRITICAL
- **`ama-metrics-fb679f7bc-5bsll`**: **10.92 GB** ⚠️ CRITICAL  
- **`ama-metrics-fb679f7bc-n4r6q`**: **10.65 GB** ⚠️ CRITICAL
- **`ama-metrics-fb679f7bc-z4dz4`**: **4.28 GB** ⚠️ HIGH
- **`ama-metrics-fb679f7bc-6xp85`**: **2.85 GB** ⚠️ ELEVATED

**Memory Pattern**: Several pods consistently consuming 10+ GB memory, which would trigger OOM kills in most Kubernetes environments.

### Component Health Status
#### OTLP Export Pipeline: ✅ **HEALTHY**
- **ReplicaSet OTLP Failed Exports**: 0 failed metric points
- **DaemonSet OTLP Failed Exports**: 0 failed metric points  
- **Pipeline Status**: Fully operational

#### Metrics Processing Status: ⚠️ **MASSIVE VOLUME DUE TO DISABLED FILTERING**
- **ReplicaSet Metrics Received**: 15,075,849,228 (15+ billion metrics in 24h)
- **ReplicaSet Metrics Processed**: 15,082,808,271 
- **ReplicaSet Metrics Dropped**: 0
- **DaemonSet Metrics Dropped**: 113,145 (minimal compared to ReplicaSet)

### Root Cause Analysis: **METRIC FILTERING DISABLED CAUSING EXCESSIVE MEMORY CONSUMPTION**

**The OOM symptoms are caused by disabled metric filtering configuration:**

1. **Disabled Keep Lists**: Almost all keep list regex patterns are empty strings, meaning the cluster ingests ALL metrics from all Prometheus targets instead of only the essential ones
2. **Massive Metrics Volume**: ReplicaSet processing 15+ billion metrics in 24 hours due to lack of filtering
3. **Memory Accumulation**: Unfiltered metrics create massive memory pressure (10+ GB per pod)
4. **OOM Condition**: Memory consumption exceeds container limits, causing Kubernetes to OOM-kill pods
5. **Healthy Pipeline Paradox**: OTLP pipeline works perfectly but processes excessive unfiltered data

### Evidence Pattern Matching (Updated):
- **Pattern 1**: ❌ No MetricsExtension startup failures (authentication working)
- **Pattern 2**: ❌ No OTLP export connection failures (pipeline healthy)
- **Pattern 4**: ❌ No authentication/token configuration issues
- **Pattern 5**: ✅ Transient SSL handshake failures (can be ignored as per Pattern 5 guidance)
- **NEW: Disabled Filtering Pattern**: ✅ **CRITICAL ROOT CAUSE** - Keep lists disabled causing excessive metric ingestion

### Timeline and Scope:
- **Current Status**: Ongoing high memory consumption with 3 pods over 10GB memory usage
- **Volume Scale**: 15+ billion metrics processed by ReplicaSet in 24 hours
- **Affected Components**: All Prometheus targets except Kube-State-Metrics
- **OOM Risk**: Continuous due to unfiltered metric collection

### Resolution Path (Updated Priority):
1. **IMMEDIATE - Enable Metric Filtering**: Configure proper keep list regex patterns for all disabled targets:
   - API Server keep list regex
   - cAdvisor keep list regex  
   - Core DNS keep list regex
   - Kubelet keep list regex
   - Node Exporter keep list regex
   - All other disabled keep lists

2. **HIGH PRIORITY - Evaluate Custom PodMonitor Applications**: **590K+ PodMonitor targets identified**:
   - **Streaming Rules Engine** (6 instances): High-volume rule processing metrics
   - **SOAR Services** (7 services): Security automation metrics  
   - **Asset Management** (5 services): Asset tracking and orchestration metrics
   - **Network Flow Services** (3 services): Network analysis metrics
   - **Task Scheduler** (6 regional instances): Job scheduling metrics
   - **20+ additional custom services**: Various application-specific metrics

3. **Secondary - Analyze Custom Scrape Jobs**: Use Prometheus Scrape Job Analysis query to identify high-volume custom targets and evaluate:
   - Which custom metrics are essential vs. optional for each application
   - Whether additional filtering can be applied to high-volume single targets
   - If ServiceMonitor/PodMonitor configurations can be optimized
   - Consider implementing custom metric filtering for high-cardinality applications

4. **Tertiary - Monitor Memory Reduction**: After filtering is enabled, memory usage should drop significantly
5. **Quaternary - Verify HPA Scaling**: Ensure HPA can scale down pods as memory pressure reduces
6. **Monitoring - Track Metrics Volume**: Monitor `meMetricsProcessedCount` to confirm filtering effectiveness

**Critical Note**: The **590K PodMonitor targets represent 87% of all targets** in the cluster, indicating this is a custom application-heavy environment where standard Kubernetes filtering alone will not resolve the memory pressure.

### Key Learning (Updated):
**Disabled metric filtering is the primary cause of OOM conditions in Azure Managed Prometheus** - empty keep list strings cause ingestion of ALL metrics from Prometheus targets instead of only essential metrics. Always check metric filtering configuration when investigating memory issues. The OTLP pipeline can be completely healthy while still causing OOM due to excessive unfiltered data volume.

**Investigation Priority (Updated)**: When seeing high memory usage with healthy OTLP exports, immediately check metric filtering configuration. Disabled filtering (empty keep list strings) is a common root cause of OOM symptoms that presents with healthy telemetry pipelines but massive memory consumption.

**Corrected Analysis**: The previous case study incorrectly focused on transient SSL errors. The actual root cause is **disabled metric filtering configuration** causing excessive memory consumption from unfiltered Prometheus metric ingestion.

### Enhanced Correlation Analysis

**Final Investigation Results:**
- **Root Cause**: Disabled metric filtering (empty keep-list strings)
- **Memory Impact**: 10+ GB per pod (5x normal consumption)
- **Metrics Volume**: 15+ billion metrics in 24 hours
- **Pod Distribution**: 48 total pods with uneven target allocation

**Comprehensive Pod-Level Correlation Analysis:**

**Top Memory Consumers vs. Metrics vs. Targets:**

| Pod | Memory (GB) | Metrics Scraped | Targets | Metrics/Target | Memory/Million Metrics |
|-----|-------------|-----------------|---------|----------------|------------------------|
| `q8qvm` | **13.83** | 8.7M | 10 | 872K | 1.59 GB |
| `hg8sj` | **13.72** | **10.8M** | 35 | 308K | 1.27 GB |
| `kn7zf` | **13.34** | **9.4M** | 12 | 786K | 1.41 GB |
| `6xp85` | **12.57** | **9.2M** | **116** | 79K | 1.37 GB |
| `5bsll` | **12.26** | **11.4M** | **112** | 102K | 1.08 GB |
| `9mwdv` | **11.64** | **9.1M** | **101** | 91K | 1.28 GB |
| `ndtqp` | **11.30** | 8.9M | **112** | 79K | 1.27 GB |
| `n4r6q` | **11.20** | 8.6M | **104** | 83K | 1.31 GB |
| `z4dz4` | 9.35 | **9.5M** | **128** | 74K | 0.98 GB |

**Key Correlation Findings:**

1. **Memory-Metrics Anomaly**: 
   - Pod `q8qvm`: **Highest memory (13.83 GB)** but only 8.7M metrics and 10 targets
   - **Memory efficiency ratio: 1.59 GB per million metrics** (worst efficiency)
   - Suggests memory leak or processing inefficiency beyond just metric volume

2. **Target Load Distribution Patterns**:
   - **High Target Pods** (`z4dz4`: 128 targets, `6xp85`: 116 targets) show better memory efficiency
   - **Low Target Pods** (`q8qvm`: 10 targets, `kn7zf`: 12 targets) show poor memory efficiency
   - **Pattern**: Low target count correlates with higher memory per metric ratio

3. **Metrics Volume vs. Memory Correlation**:
   - Pod `5bsll`: **Highest metrics (11.4M)** but moderate memory (12.26 GB)
   - Pod `hg8sj`: **Second highest metrics (10.8M)** with high memory (13.72 GB)
   - **Clear correlation**: Higher metrics volume generally leads to higher memory usage

4. **Target Distribution Analysis**:
   - Uneven distribution: Some pods handle 100+ targets, many handle 0
   - Active pods: `z4dz4` (128 targets), `6xp85` (116 targets), `5bsll` (112 targets)
   - Inactive pods: 30+ pods with 0 targets but still consuming memory
   - **Distribution Cause**: Custom scrape jobs may contain single targets with high metric volumes that cannot be split across pods since targets are atomic units
   - **Efficiency Pattern**: Pods with more targets tend to have better memory efficiency per metric

5. **System Impact**:
   - Total cluster load: 48 pods processing unfiltered metrics
   - Resource waste: Empty pods still consuming resources
   - HPA unable to scale down due to consistent high memory pressure
   - **Custom Target Impact**: High-volume single targets create uneven load distribution requiring per-target metric filtering evaluation
   - **Memory Inefficiency**: Some pods show 60% worse memory efficiency than others processing similar volumes

**Critical Discovery**: Pod `q8qvm` shows anomalous memory consumption (13.83 GB) with relatively low metrics volume (8.7M), suggesting potential memory leak or processing inefficiency beyond the disabled filtering issue.

**Custom Target Analysis - Major Finding:**

**Cluster-Wide Custom Target Distribution:**
- **PodMonitor Jobs**: 590,414 total targets (47 unique jobs) - **87% of all targets**
- **Configmap Jobs**: 75,372 total targets (6 unique jobs) - **11% of all targets**
- **ServiceMonitor Jobs**: 12,562 total targets (1 unique job) - **2% of all targets**

**Pod `q8qvm` Custom Target Breakdown:**
- **47 unique PodMonitor jobs** (1 target each) - These are high-cardinality custom application metrics
- **6 Configmap jobs** (1 target each) - Standard Kubernetes/Istio metrics
- **Total**: 10 targets but extremely high metric density per target

**Root Cause Analysis - Enhanced:**

1. **Primary Issue**: Disabled metric filtering (empty keep-list strings) affects ALL metric sources
2. **Amplifying Factor**: **Custom PodMonitor applications producing extremely high metric volumes**
   - 47 custom PodMonitor jobs per pod (streaming-rules-engine, soar-*, asset-*, flows-*, etc.)
   - Each custom application likely producing thousands of metrics per scrape
   - **590K+ PodMonitor targets across cluster** vs. only **88K standard targets**
3. **Memory Inefficiency Pattern**: Pods with high custom PodMonitor density show poor memory efficiency
4. **Target Allocation Issue**: Custom applications cannot be filtered by standard keep-lists since they're not covered by default filtering

**Custom Application Metrics Identified:**
- `streaming-rules-engine-service` (6 instances)
- `soar-*` services (7 SOAR automation services)
- `asset-*` services (5 asset management services)  
- `flows-*` services (3 network flow services)
- `task-scheduler` (6 regional instances)
- Plus 20+ additional custom services

**Resolution Validation**: Enable metric filtering to reduce ingestion volume by 80-90%, allowing proper memory management and HPA scaling. Additionally investigate pod `q8qvm` for potential memory leak.

## Future Investigation Tips

1. **Use dashboard.json queries** as proven investigation patterns
2. **Follow the component dependency chain** when analyzing failures
3. **Correlate timing** between authentication failures and export failures
4. **Check both customMetrics and traces tables** for complete picture
5. **Monitor startup sequences** to catch initialization failures
6. **Validate authentication configuration** as first step for any cluster issues
7. **Cross-reference AKS telemetry**: Use AKS Kusto cluster to correlate Prometheus issues with infrastructure events
8. **Distinguish transient from persistent SSL failures**: Intermittent SSL handshake errors are normal network behavior - only investigate if they prevent core functionality or persist continuously
9. **Discover schema first**: Always use `kusto_database_list` and `kusto_table_schema` to understand available data before querying

## Tools and Resources

- **Azure MCP Server**: Primary investigation tool for Application Insights telemetry
- **Kusto MCP Server**: Secondary tool for AKS cluster telemetry (`https://akshuba.centralus.kusto.windows.net/`)
- **Application Insights**: Data source for Managed Prometheus telemetry
- **AKS Cluster Telemetry**: Infrastructure and cluster-level events/metrics
- **KQL Queries**: Structured investigation patterns for both data sources
- **Dashboard Queries**: Proven diagnostic patterns
- **Component Health Checks**: MetricsExtension, OTLP, Prometheus receiver status

This memory bank should be updated with new patterns and findings from future investigations to build a comprehensive troubleshooting knowledge base.
