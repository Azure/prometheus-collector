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

**Root Cause:** Transient network connectivity issues to Azure Monitor ingestion endpoints

**Impact:** These are temporary network issues with built-in retry mechanisms. MetricsExtension will automatically retry failed publications, so these errors can be safely ignored unless they persist for extended periods.

**Diagnostic Query:**
```kql
traces 
| where tostring(customDimensions.cluster) == "{cluster-resource-id}"
| where message contains "MetricsExtension" and (message contains "Failed to read" or message contains "Failed to write")
| project timestamp, controllertype=tostring(customDimensions.controllertype), message
| order by timestamp desc
```

**Note:** Only investigate this pattern if failures persist continuously for hours without successful publications between them.

## Investigation Workflow

### Step 1: Cluster-Specific Analysis
1. Filter to problematic cluster using resource ID
2. Check component metrics (OTLP exports, failures)
3. Analyze time-series patterns
4. **Cross-reference with AKS telemetry**: Query AKS Kusto cluster for broader cluster health context

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
**No action required** - These are transient network errors with automatic retry mechanisms. Only investigate if failures persist continuously for extended periods without any successful publications.

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

## Future Investigation Tips

1. **Use dashboard.json queries** as proven investigation patterns
2. **Follow the component dependency chain** when analyzing failures
3. **Correlate timing** between authentication failures and export failures
4. **Check both customMetrics and traces tables** for complete picture
5. **Monitor startup sequences** to catch initialization failures
6. **Validate authentication configuration** as first step for any cluster issues
7. **Cross-reference AKS telemetry**: Use AKS Kusto cluster to correlate Prometheus issues with infrastructure events
8. **Discover schema first**: Always use `kusto_database_list` and `kusto_table_schema` to understand available data before querying

## Tools and Resources

- **Azure MCP Server**: Primary investigation tool for Application Insights telemetry
- **Kusto MCP Server**: Secondary tool for AKS cluster telemetry (`https://akshuba.centralus.kusto.windows.net/`)
- **Application Insights**: Data source for Managed Prometheus telemetry
- **AKS Cluster Telemetry**: Infrastructure and cluster-level events/metrics
- **KQL Queries**: Structured investigation patterns for both data sources
- **Dashboard Queries**: Proven diagnostic patterns
- **Component Health Checks**: MetricsExtension, OTLP, Prometheus receiver status

This memory bank should be updated with new patterns and findings from future investigations to build a comprehensive troubleshooting knowledge base.
